package providerplugins

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ecommerce/internal/migrations"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExternalProvidersSupportRuntimeOperations(t *testing.T) {
	db := newProviderPluginTestDB(t)
	dir := writeRuntimeProviderFixtures(t)
	runtime := newExternalRuntime(t, db, dir)

	paymentProvider, err := runtime.PaymentProviders.Provider("ext-pay")
	require.NoError(t, err)

	authorizeResult, err := paymentProvider.Authorize(context.Background(), paymentservice.AuthorizeRequest{
		OrderID:              10,
		SnapshotID:           20,
		Amount:               models.MoneyFromFloat(15),
		Currency:             "USD",
		Provider:             "ext-pay",
		IdempotencyKey:       "auth-1",
		CorrelationID:        "corr-auth",
		PaymentMethodDisplay: "External Pay",
		PaymentData:          map[string]string{"token": "tok_test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ext-pay-auth", authorizeResult.ProviderTxnID)

	lookupProvider, ok := paymentProvider.(paymentservice.TransactionLookupProvider)
	require.True(t, ok)
	txn, err := lookupProvider.GetTransaction(context.Background(), "ext-pay-auth")
	require.NoError(t, err)
	assert.Equal(t, models.PaymentTransactionOperationAuthorize, txn.Operation)
	assert.Equal(t, models.PaymentTransactionStatusSucceeded, txn.Status)

	parser, ok := paymentProvider.(paymentservice.StoredWebhookParser)
	require.True(t, ok)
	paymentEvent, err := parser.ParseStoredWebhook(context.Background(), []byte(`{"id":"evt-ext-payment"}`))
	require.NoError(t, err)
	assert.Equal(t, "ext-pay", paymentEvent.Provider)
	assert.Equal(t, "payment.captured", paymentEvent.EventType)

	shippingProvider, err := runtime.ShippingProviders.Provider("ext-ship")
	require.NoError(t, err)

	rates, err := shippingProvider.QuoteRates(context.Background(), shippingservice.QuoteRatesRequest{
		OrderID:               30,
		SnapshotID:            40,
		Currency:              "USD",
		ShippingAddressPretty: "123 Test St, Portland, US",
		ShippingAmount:        models.MoneyFromFloat(7.25),
		ShippingData:          map[string]string{"line1": "123 Test St"},
	})
	require.NoError(t, err)
	require.Len(t, rates, 1)
	assert.Equal(t, "ext-rate-1", rates[0].ProviderRateID)

	shipment, err := shippingProvider.BuyLabel(context.Background(), shippingservice.BuyLabelRequest{
		OrderID:    30,
		SnapshotID: 40,
		Provider:   "ext-ship",
		Rate: models.ShipmentRate{
			ID:             77,
			Provider:       "ext-ship",
			ProviderRateID: "ext-rate-1",
			ServiceCode:    "standard",
			ServiceName:    "Standard",
			Amount:         models.MoneyFromFloat(7.25),
			Currency:       "USD",
		},
		ShippingAddressPretty: "123 Test St, Portland, US",
		Package: shippingservice.PackageInput{
			Reference:   "pkg-1",
			WeightGrams: 1000,
			LengthCM:    10,
			WidthCM:     10,
			HeightCM:    10,
		},
		IdempotencyKey: "ship-1",
		CorrelationID:  "corr-ship",
	})
	require.NoError(t, err)
	assert.Equal(t, "ext-shipment-1", shipment.ProviderShipmentID)

	shipmentLookup, ok := shippingProvider.(shippingservice.ShipmentLookupProvider)
	require.True(t, ok)
	shipmentState, err := shipmentLookup.GetShipment(context.Background(), "ext-shipment-1")
	require.NoError(t, err)
	assert.Equal(t, "DELIVERED", shipmentState.Status)

	shippingParser, ok := shippingProvider.(shippingservice.StoredWebhookParser)
	require.True(t, ok)
	trackingEvent, err := shippingParser.ParseStoredWebhook(context.Background(), []byte(`{"id":"evt-ext-shipping"}`))
	require.NoError(t, err)
	assert.Equal(t, "ext-ship", trackingEvent.Provider)
	assert.Equal(t, "tracking.delivered", trackingEvent.EventType)

	taxProvider, err := runtime.TaxProviders.Provider("ext-tax")
	require.NoError(t, err)

	taxAmount, err := taxProvider.QuoteTax(context.Background(), taxservice.QuoteTaxRequest{
		Provider: "ext-tax",
		Data:     map[string]string{"state": "OR"},
		Base:     models.MoneyFromFloat(100),
	})
	require.NoError(t, err)
	assert.Equal(t, models.MoneyFromFloat(5.25), taxAmount)

	finalized, err := taxProvider.FinalizeTax(context.Background(), taxservice.FinalizeTaxRequest{
		Provider:          "ext-tax",
		Currency:          "USD",
		Data:              map[string]string{"state": "OR"},
		Items:             []taxservice.LineInput{{LineType: models.TaxLineTypeItem, Quantity: 1, Amount: models.MoneyFromFloat(100)}},
		ShippingAmount:    models.MoneyFromFloat(0),
		ExpectedTaxAmount: models.MoneyFromFloat(5.25),
		InclusivePricing:  false,
	})
	require.NoError(t, err)
	assert.Equal(t, models.MoneyFromFloat(5.25), finalized.TotalTax)
	require.Len(t, finalized.Lines, 1)

	report, err := taxProvider.ExportReport(context.Background(), taxservice.ExportReportRequest{
		Provider: "ext-tax",
		Lines: []models.OrderTaxLine{{
			OrderID:       1,
			SnapshotID:    2,
			LineType:      models.TaxLineTypeItem,
			Jurisdiction:  "OR",
			TaxName:       "External Tax",
			TaxAmount:     models.MoneyFromFloat(5.25),
			TaxableAmount: models.MoneyFromFloat(100),
			Inclusive:     false,
			TaxProviderID: "ext-tax",
			FinalizedAt:   time.Now().UTC(),
		}},
	})
	require.NoError(t, err)
	defer report.Close()

	reportBody, err := io.ReadAll(report)
	require.NoError(t, err)
	assert.Contains(t, string(reportBody), "External Tax")
}

func TestWebhookServiceProcessesExternalPaymentEvents(t *testing.T) {
	db := newProviderPluginTestDB(t)
	dir := writeRuntimeProviderFixtures(t)
	runtime := newExternalRuntime(t, db, dir)

	service := webhookservice.NewService(db, runtime.PaymentProviders, runtime.ShippingProviders, nil)
	service.StartProcessor()
	t.Cleanup(func() {
		close(service.Queue)
	})

	session := models.CheckoutSession{
		PublicToken: "ext-webhook-session",
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		LastSeenAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Create(&session).Error)

	order := models.Order{
		CheckoutSessionID: session.ID,
		Total:             models.MoneyFromFloat(15),
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&order).Error)

	intent := models.PaymentIntent{
		OrderID:          order.ID,
		SnapshotID:       1,
		Provider:         "ext-pay",
		Status:           models.PaymentIntentStatusAuthorized,
		AuthorizedAmount: models.MoneyFromFloat(15),
		CapturedAmount:   0,
		Currency:         "USD",
		Version:          1,
	}
	require.NoError(t, db.Create(&intent).Error)
	require.NoError(t, db.Create(&models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       "ext-pay-auth",
		IdempotencyKey:      "webhook-ext",
		Amount:              models.MoneyFromFloat(15),
		Status:              models.PaymentTransactionStatusPending,
		RawResponseRedacted: "{}",
	}).Error)

	event, duplicate, err := service.ReceiveWebhook(
		context.Background(),
		"ext-pay",
		map[string]string{"X-Ext-Signature": "valid"},
		[]byte(`{"id":"evt-ext-payment","type":"payment.captured"}`),
	)
	require.NoError(t, err)
	assert.False(t, duplicate)

	waitForWebhookProcessed(t, db, event.ID)

	var txn models.PaymentTransaction
	require.NoError(t, db.Where("provider_txn_id = ?", "ext-pay-auth").First(&txn).Error)
	assert.Equal(t, models.PaymentTransactionStatusSucceeded, txn.Status)

	var refreshedOrder models.Order
	require.NoError(t, db.First(&refreshedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, refreshedOrder.Status)
}

func newProviderPluginTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrations.RunWithoutContract(db))
	return db
}

func writeRuntimeProviderFixtures(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "provider.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
payload="$(cat)"

case "$payload" in
  *'"provider_id":"ext-pay"'*)
    [[ "$payload" == *'"api_key":"test-secret"'* ]] || { echo "missing payment credential" >&2; exit 1; }
    ;;
  *'"provider_id":"ext-ship"'*)
    [[ "$payload" == *'"ship_key":"ship-secret"'* ]] || { echo "missing shipping credential" >&2; exit 1; }
    ;;
  *'"provider_id":"ext-tax"'*)
    [[ "$payload" == *'"tax_key":"tax-secret"'* ]] || { echo "missing tax credential" >&2; exit 1; }
    ;;
esac

case "$payload" in
  *'"action":"payment.authorize"'*)
    printf '%s\n' '{"ProviderTxnID":"ext-pay-auth","RawResponseRedacted":"{\"status\":\"authorized\"}"}'
    ;;
  *'"action":"payment.capture"'*)
    printf '%s\n' '{"ProviderTxnID":"ext-pay-capture","RawResponseRedacted":"{\"status\":\"captured\"}"}'
    ;;
  *'"action":"payment.void"'*)
    printf '%s\n' '{"ProviderTxnID":"ext-pay-void","RawResponseRedacted":"{\"status\":\"voided\"}"}'
    ;;
  *'"action":"payment.refund"'*)
    printf '%s\n' '{"ProviderTxnID":"ext-pay-refund","RawResponseRedacted":"{\"status\":\"refunded\"}"}'
    ;;
  *'"action":"payment.get_transaction"'*)
    printf '%s\n' '{"ProviderTxnID":"ext-pay-auth","Operation":"AUTHORIZE","Amount":15,"Currency":"USD","Status":"SUCCEEDED"}'
    ;;
  *'"action":"payment.verify_webhook"'*)
    [[ "$payload" == *'"X-Ext-Signature":"valid"'* ]] || { echo "invalid payment signature" >&2; exit 1; }
    printf '%s\n' '{"Provider":"ext-pay","ProviderEventID":"evt-ext-payment","EventType":"payment.captured","ProviderTxnID":"ext-pay-auth"}'
    ;;
  *'"action":"payment.parse_webhook"'*)
    printf '%s\n' '{"Provider":"ext-pay","ProviderEventID":"evt-ext-payment","EventType":"payment.captured","ProviderTxnID":"ext-pay-auth"}'
    ;;
  *'"action":"shipping.quote_rates"'*)
    printf '%s\n' '[{"ProviderRateID":"ext-rate-1","ServiceCode":"standard","ServiceName":"Standard","Amount":7.25,"Currency":"USD","ExpiresAt":"2030-01-01T00:00:00Z"}]'
    ;;
  *'"action":"shipping.buy_label"'*)
    printf '%s\n' '{"ProviderShipmentID":"ext-shipment-1","TrackingNumber":"EXT123","TrackingURL":"https://tracking.example/ext","LabelURL":"https://labels.example/ext.pdf","ServiceCode":"standard","ServiceName":"Standard"}'
    ;;
  *'"action":"shipping.get_shipment"'*)
    printf '%s\n' '{"ProviderShipmentID":"ext-shipment-1","TrackingNumber":"EXT123","Status":"DELIVERED","ServiceCode":"standard","ServiceName":"Standard"}'
    ;;
  *'"action":"shipping.verify_webhook"'*)
    printf '%s\n' '{"Provider":"ext-ship","ProviderEventID":"evt-ext-shipping","EventType":"tracking.delivered","ProviderShipmentID":"ext-shipment-1","TrackingNumber":"EXT123","Status":"DELIVERED","Location":"Warehouse","Description":"Delivered","OccurredAt":"2030-01-01T00:00:00Z","RawPayload":"{}"}'
    ;;
  *'"action":"shipping.parse_webhook"'*)
    printf '%s\n' '{"Provider":"ext-ship","ProviderEventID":"evt-ext-shipping","EventType":"tracking.delivered","ProviderShipmentID":"ext-shipment-1","TrackingNumber":"EXT123","Status":"DELIVERED","Location":"Warehouse","Description":"Delivered","OccurredAt":"2030-01-01T00:00:00Z","RawPayload":"{}"}'
    ;;
  *'"action":"tax.quote_tax"'*)
    printf '%s\n' '5.25'
    ;;
  *'"action":"tax.finalize_tax"'*)
    printf '%s\n' '{"Provider":"ext-tax","Currency":"USD","InclusivePricing":false,"TotalTax":5.25,"Lines":[{"LineType":"item","Quantity":1,"Jurisdiction":"OR","TaxCode":"external_goods","TaxName":"External Tax","TaxableAmount":100,"TaxAmount":5.25,"TaxRateBasisPoints":525,"Inclusive":false}]}'
    ;;
  *'"action":"tax.export_report"'*)
    printf '%s\n' '{"content":"order_id,snapshot_id,line_type,jurisdiction,tax_name,tax_amount,taxable_amount,inclusive\n1,2,item,OR,External Tax,5.25,100.00,false\n"}'
    ;;
  *)
    echo "unsupported action" >&2
    exit 1
    ;;
esac
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	manifests := map[string]string{
		"payment.json": `{
  "id":"ext-pay",
  "type":"payment",
  "name":"External Payment",
  "description":"External payment provider",
  "command":"bash",
  "args":["./provider.sh"],
  "timeout_ms":4000,
  "capabilities":{"payment":{"lookup_transaction":true,"parse_webhook":true}}
}`,
		"shipping.json": `{
  "id":"ext-ship",
  "type":"shipping",
  "name":"External Shipping",
  "description":"External shipping provider",
  "command":"bash",
  "args":["./provider.sh"],
  "timeout_ms":4000,
  "capabilities":{"shipping":{"lookup_shipment":true,"parse_webhook":true}}
}`,
		"tax.json": `{
  "id":"ext-tax",
  "type":"tax",
  "name":"External Tax",
  "description":"External tax provider",
  "command":"bash",
  "args":["./provider.sh"],
  "timeout_ms":4000
}`,
	}
	for name, manifest := range manifests {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(manifest), 0o644))
	}

	return dir
}

func newExternalRuntime(t *testing.T, db *gorm.DB, dir string) *providerops.Runtime {
	t.Helper()

	keyring, err := providerops.ParseKeyringConfig("v1:MDEyMzQ1Njc4OWFiY2RlZg==")
	require.NoError(t, err)

	credentials, err := providerops.NewCredentialService(keyring, "v1")
	require.NoError(t, err)

	_, err = credentials.Store(context.Background(), db, providerops.StoreCredentialInput{
		ProviderType: models.ProviderTypePayment,
		ProviderID:   "ext-pay",
		Environment:  models.ProviderEnvironmentSandbox,
		Label:        "Ext Pay",
		SecretData:   map[string]string{"api_key": "test-secret"},
	})
	require.NoError(t, err)

	_, err = credentials.Store(context.Background(), db, providerops.StoreCredentialInput{
		ProviderType: models.ProviderTypeShipping,
		ProviderID:   "ext-ship",
		Environment:  models.ProviderEnvironmentSandbox,
		Label:        "Ext Ship",
		SecretData:   map[string]string{"ship_key": "ship-secret"},
	})
	require.NoError(t, err)

	_, err = credentials.Store(context.Background(), db, providerops.StoreCredentialInput{
		ProviderType: models.ProviderTypeTax,
		ProviderID:   "ext-tax",
		Environment:  models.ProviderEnvironmentSandbox,
		Label:        "Ext Tax",
		SecretData:   map[string]string{"tax_key": "tax-secret"},
	})
	require.NoError(t, err)

	loaded, err := LoadRegistriesFromDir(
		dir,
		paymentservice.NewDefaultProviderRegistry(),
		shippingservice.NewDefaultProviderRegistry(),
		taxservice.NewDefaultProviderRegistry(),
	)
	require.NoError(t, err)

	return providerops.NewRuntime(db, providerops.RuntimeConfig{
		Environment:       models.ProviderEnvironmentSandbox,
		Credentials:       credentials,
		PaymentProviders:  loaded.PaymentProviders,
		ShippingProviders: loaded.ShippingProviders,
		TaxProviders:      loaded.TaxProviders,
	})
}

func waitForWebhookProcessed(t *testing.T, db *gorm.DB, eventID uint) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var event models.WebhookEvent
		if err := db.First(&event, eventID).Error; err == nil && event.ProcessedAt != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	var event models.WebhookEvent
	require.NoError(t, db.First(&event, eventID).Error)
	require.NotNil(t, event.ProcessedAt)
}
