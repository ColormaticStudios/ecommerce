package providerops

import (
	"context"
	"testing"
	"time"

	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerObserverRecorder struct {
	providerType string
	operation    string
	outcome      string
	duration     time.Duration
	events       []string
}

func (r *providerObserverRecorder) ProviderCallCompleted(providerType, operation, outcome string, duration time.Duration) {
	r.providerType = providerType
	r.operation = operation
	r.outcome = outcome
	r.duration = duration
	r.events = append(r.events, providerType+"/"+operation+"/"+outcome)
}

func TestRecoveryAndCompensationCallsNotifyObserver(t *testing.T) {
	observer := &providerObserverRecorder{}
	runtime := NewRuntime(nil, RuntimeConfig{Observer: observer})
	ctx := context.Background()

	payment, err := runtime.PaymentProviders.Provider("dummy-card")
	require.NoError(t, err)
	_, err = payment.GetOutcomeByOperationKey(ctx, "payment-operation")
	require.NoError(t, err)

	shipping, err := runtime.ShippingProviders.Provider("dummy-ground")
	require.NoError(t, err)
	_, err = shipping.CancelLabel(ctx, shippingservice.CancelLabelRequest{Provider: "dummy-ground", ProviderShipmentID: "shipment-1", OperationKey: "shipping-cancel", IdempotencyKey: "shipping-idempotency"})
	require.NoError(t, err)
	_, err = shipping.GetOutcomeByOperationKey(ctx, "shipping-operation")
	require.NoError(t, err)

	tax, err := runtime.TaxProviders.Provider("dummy-us-tax")
	require.NoError(t, err)
	_, err = tax.CancelFinalization(ctx, taxservice.CancelFinalizationRequest{Provider: "dummy-us-tax", ProviderReference: "tax-1", OperationKey: "tax-cancel", IdempotencyKey: "tax-idempotency"})
	require.NoError(t, err)
	_, err = tax.GetOutcomeByOperationKey(ctx, "tax-operation")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"payment/get_operation_outcome/succeeded",
		"shipping/cancel_label/succeeded",
		"shipping/get_operation_outcome/succeeded",
		"tax/cancel_finalization/succeeded",
		"tax/get_operation_outcome/succeeded",
	}, observer.events)
}

func TestAuditObserverRecordsProviderOutcomeWithoutDatabase(t *testing.T) {
	observer := &providerObserverRecorder{}
	audit := NewAuditService(nil, observer)
	err := audit.Record(context.Background(), AuditRecord{
		ProviderType: " payment ", Operation: " authorize ", Status: " failed ", Latency: 750 * time.Millisecond,
	})
	require.NoError(t, err)
	assert.Equal(t, "payment", observer.providerType)
	assert.Equal(t, "authorize", observer.operation)
	assert.Equal(t, "failed", observer.outcome)
	assert.Equal(t, 750*time.Millisecond, observer.duration)
}
