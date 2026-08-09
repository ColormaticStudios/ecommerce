package tax

import (
	"context"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestBuiltInTaxProvidersRequireOperationIdentityLookupAndCompensation(t *testing.T) {
	registry := NewDefaultProviderRegistry()
	for _, providerID := range []string{"dummy-us-tax", "dummy-vat"} {
		t.Run(providerID, func(t *testing.T) {
			provider, err := registry.Provider(providerID)
			require.NoError(t, err)
			request := FinalizeTaxRequest{Provider: providerID, Currency: "USD"}
			_, err = provider.FinalizeTax(context.Background(), request)
			require.ErrorContains(t, err, "idempotency key")

			request.IdempotencyKey = "tax-1"
			request.OperationKey = "op-tax-1"
			result, err := provider.FinalizeTax(context.Background(), request)
			require.NoError(t, err)
			require.NotEmpty(t, result.ProviderReference)

			outcome, err := provider.GetOutcomeByOperationKey(context.Background(), request.OperationKey)
			require.NoError(t, err)
			require.Equal(t, models.ProviderOutcomeSucceeded, outcome.Outcome)

			compensation, err := provider.CancelFinalization(context.Background(), CancelFinalizationRequest{
				Provider: providerID, ProviderReference: result.ProviderReference,
				IdempotencyKey: "cancel-tax-1", OperationKey: "op-cancel-tax-1",
			})
			require.NoError(t, err)
			require.Equal(t, models.ProviderOutcomeSucceeded, compensation.Outcome)
			require.Equal(t, result.ProviderReference, compensation.ProviderReference)
		})
	}
}
