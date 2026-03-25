package providerops

import (
	"context"
	"testing"
	"time"

	shippingservice "ecommerce/internal/services/shipping"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProviderOpsTestDB(t *testing.T, modelsToMigrate ...any) *gorm.DB {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(modelsToMigrate...))
	return db
}

func TestCredentialStoreResolveRotateWithoutPlaintextLeak(t *testing.T) {
	db := newProviderOpsTestDB(t, &models.ProviderCredential{})
	ctx := context.Background()

	serviceV1, err := NewCredentialService(map[string][]byte{
		"v1": []byte("0123456789abcdef0123456789abcdef"),
	}, "v1")
	require.NoError(t, err)

	stored, err := serviceV1.Store(ctx, db, StoreCredentialInput{
		ProviderType: models.ProviderTypePayment,
		ProviderID:   "dummy-card",
		Environment:  models.ProviderEnvironmentSandbox,
		Label:        "sandbox card",
		SecretData: map[string]string{
			"api_key": "super-secret-api-key",
		},
		Metadata: CredentialMetadata{
			SupportedCurrencies: []string{"usd"},
			SettlementCurrency:  "usd",
			FXMode:              models.ProviderFXModeSameCurrencyOnly,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "v1", stored.Record.KeyVersion)
	require.NotContains(t, stored.Record.SecretEnvelopeJSON, "super-secret-api-key")

	resolved, err := serviceV1.Resolve(ctx, db, models.ProviderTypePayment, "dummy-card", models.ProviderEnvironmentSandbox)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, "super-secret-api-key", resolved.SecretData["api_key"])
	require.NoError(t, serviceV1.ValidateCurrency("USD", resolved))
	require.ErrorIs(t, serviceV1.ValidateCurrency("EUR", resolved), ErrUnsupportedProviderCurrency)

	serviceV2, err := NewCredentialService(map[string][]byte{
		"v1": []byte("0123456789abcdef0123456789abcdef"),
		"v2": []byte("abcdef0123456789abcdef0123456789"),
	}, "v2")
	require.NoError(t, err)

	rotated, err := serviceV2.Rotate(ctx, db, stored.Record.ID)
	require.NoError(t, err)
	require.Equal(t, "v2", rotated.Record.KeyVersion)
	require.WithinDuration(t, time.Now().UTC(), rotated.Record.LastRotatedAt, 5*time.Second)

	resolvedAfterRotate, err := serviceV2.Resolve(ctx, db, models.ProviderTypePayment, "dummy-card", models.ProviderEnvironmentSandbox)
	require.NoError(t, err)
	require.Equal(t, "super-secret-api-key", resolvedAfterRotate.SecretData["api_key"])
}

func TestReconciliationDetectsPaymentShippingAndTaxDrift(t *testing.T) {
	db := newProviderOpsTestDB(
		t,
		&models.PaymentIntent{},
		&models.PaymentTransaction{},
		&models.Shipment{},
		&models.OrderCheckoutSnapshot{},
		&models.OrderCheckoutSnapshotItem{},
		&models.OrderTaxLine{},
		&models.ProviderCallAudit{},
		&models.ProviderReconciliationRun{},
		&models.ProviderReconciliationDrift{},
	)

	runtime := NewRuntime(db, RuntimeConfig{
		Environment: models.ProviderEnvironmentSandbox,
	})

	intent := models.PaymentIntent{
		OrderID:          1,
		SnapshotID:       1,
		Provider:         "dummy-card",
		Status:           models.PaymentIntentStatusAuthorized,
		AuthorizedAmount: models.MoneyFromFloat(20),
		CapturedAmount:   0,
		Currency:         "USD",
		Version:          1,
	}
	require.NoError(t, db.Create(&intent).Error)
	require.NoError(t, db.Create(&models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       "dummy-card|CAPTURE|1|1|USD|10.00|reconcile",
		IdempotencyKey:      "reconcile",
		Amount:              models.MoneyFromFloat(20),
		Status:              models.PaymentTransactionStatusSucceeded,
		RawResponseRedacted: "{}",
	}).Error)

	require.NoError(t, db.Create(&models.Shipment{
		OrderID:            1,
		SnapshotID:         2,
		Provider:           "dummy-ground",
		ShipmentRateID:     42,
		ProviderShipmentID: "ship-ground-1-42-test",
		Status:             models.ShipmentStatusException,
		Currency:           "USD",
		ServiceCode:        "standard",
		ServiceName:        "Standard",
		Amount:             models.MoneyFromFloat(5.99),
		TrackingNumber:     "WRONG",
	}).Error)

	snapshotOrderID := uint(7)
	snapshot := models.OrderCheckoutSnapshot{
		CheckoutSessionID:  11,
		OrderID:            &snapshotOrderID,
		Currency:           "USD",
		Subtotal:           models.MoneyFromFloat(10),
		ShippingAmount:     0,
		TaxAmount:          models.MoneyFromFloat(0.63),
		Total:              models.MoneyFromFloat(10.63),
		PaymentProviderID:  "dummy-card",
		ShippingProviderID: "dummy-ground",
		TaxProviderID:      "dummy-us-tax",
		ShippingDataJSON:   `{"state":"TX","country":"US"}`,
		TaxDataJSON:        `{"state":"TX"}`,
		ExpiresAt:          time.Now().UTC().Add(15 * time.Minute),
	}
	require.NoError(t, db.Create(&snapshot).Error)
	require.NoError(t, db.Create(&models.OrderCheckoutSnapshotItem{
		SnapshotID:       snapshot.ID,
		ProductVariantID: 99,
		VariantSKU:       "sku-99",
		VariantTitle:     "Variant 99",
		Quantity:         1,
		Price:            models.MoneyFromFloat(10),
	}).Error)
	require.NoError(t, db.Create(&models.OrderTaxLine{
		OrderID:            snapshotOrderID,
		SnapshotID:         snapshot.ID,
		LineType:           models.TaxLineTypeItem,
		TaxProviderID:      "dummy-us-tax",
		ProductVariantID:   uintPtr(99),
		Quantity:           1,
		Jurisdiction:       "TX",
		TaxCode:            "sales_goods",
		TaxName:            "Sales Tax",
		TaxableAmount:      models.MoneyFromFloat(10),
		TaxAmount:          models.MoneyFromFloat(0.50),
		TaxRateBasisPoints: 625,
		Inclusive:          false,
		FinalizedAt:        time.Now().UTC(),
	}).Error)

	ctx := context.Background()

	paymentRun, paymentDrifts, err := runtime.Reconciliation.Run(ctx, ReconciliationRunInput{
		ProviderType: models.ProviderTypePayment,
		ProviderID:   "dummy-card",
		Trigger:      models.ProviderReconciliationTriggerManual,
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderReconciliationStatusSucceeded, paymentRun.Status)
	require.NotEmpty(t, paymentDrifts)

	shippingRun, shippingDrifts, err := runtime.Reconciliation.Run(ctx, ReconciliationRunInput{
		ProviderType: models.ProviderTypeShipping,
		ProviderID:   "dummy-ground",
		Trigger:      models.ProviderReconciliationTriggerManual,
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderReconciliationStatusSucceeded, shippingRun.Status)
	require.NotEmpty(t, shippingDrifts)

	taxRun, taxDrifts, err := runtime.Reconciliation.Run(ctx, ReconciliationRunInput{
		ProviderType: models.ProviderTypeTax,
		ProviderID:   "dummy-us-tax",
		Trigger:      models.ProviderReconciliationTriggerManual,
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderReconciliationStatusSucceeded, taxRun.Status)
	require.NotEmpty(t, taxDrifts)
}

func TestShippingQuotePersistsAuditWithinSQLiteTransaction(t *testing.T) {
	db := newProviderOpsTestDB(
		t,
		&models.ProviderCallAudit{},
		&models.ShipmentRate{},
		&models.Shipment{},
	)

	runtime := NewRuntime(db, RuntimeConfig{
		Environment: models.ProviderEnvironmentSandbox,
	})

	order := models.Order{BaseModel: models.BaseModel{ID: 42}}
	snapshot := models.OrderCheckoutSnapshot{
		ID:                    77,
		Currency:              "USD",
		ShippingAmount:        models.MoneyFromFloat(5.99),
		ShippingProviderID:    "dummy-ground",
		ShippingAddressPretty: "1 Audit Way",
		ShippingDataJSON:      `{"country":"US","service_level":"standard"}`,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		rates, err := shippingservice.QuoteAndPersistRates(context.Background(), tx, runtime.ShippingProviders, order, snapshot, time.Now().UTC())
		require.NoError(t, err)
		require.NotEmpty(t, rates)
		return nil
	})
	require.NoError(t, err)

	var audits []models.ProviderCallAudit
	require.NoError(t, db.Order("id ASC").Find(&audits).Error)
	require.Len(t, audits, 1)
	assert.Equal(t, models.ProviderTypeShipping, audits[0].ProviderType)
	assert.Equal(t, "dummy-ground", audits[0].ProviderID)
	assert.Equal(t, "quote_rates", audits[0].Operation)
	assert.Equal(t, models.ProviderCallStatusSucceeded, audits[0].Status)
}

func uintPtr(value uint) *uint {
	return &value
}
