package payments

import (
	"context"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestBuiltInPaymentProvidersRequireOperationIdentityAndSupportLookup(t *testing.T) {
	registry := NewDefaultProviderRegistry()
	for _, providerID := range []string{"dummy-card", "dummy-wallet"} {
		t.Run(providerID, func(t *testing.T) {
			provider, err := registry.Provider(providerID)
			require.NoError(t, err)
			request := CaptureRequest{Provider: providerID, OrderID: 1, IntentID: 2, Amount: 100, Currency: "USD"}
			_, err = provider.Capture(context.Background(), request)
			require.ErrorContains(t, err, "idempotency key")

			request.IdempotencyKey = "capture-1"
			_, err = provider.Capture(context.Background(), request)
			require.ErrorContains(t, err, "operation key")

			request.OperationKey = "op-capture-1"
			result, err := provider.Capture(context.Background(), request)
			require.NoError(t, err)
			require.NotEmpty(t, result.ProviderTxnID)

			outcome, err := provider.GetOutcomeByOperationKey(context.Background(), request.OperationKey)
			require.NoError(t, err)
			require.Equal(t, models.ProviderOutcomeSucceeded, outcome.Outcome)
			require.Equal(t, request.OperationKey, outcome.OperationKey)
		})
	}
}
