package shipping

import (
	"context"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestBuiltInShippingProvidersRequireOperationIdentityLookupAndCompensation(t *testing.T) {
	registry := NewDefaultProviderRegistry()
	for _, providerID := range []string{"dummy-ground", "dummy-pickup"} {
		t.Run(providerID, func(t *testing.T) {
			provider, err := registry.Provider(providerID)
			require.NoError(t, err)
			request := BuyLabelRequest{Provider: providerID, OrderID: 1, SnapshotID: 2, Rate: models.ShipmentRate{ID: 3}}
			_, err = provider.BuyLabel(context.Background(), request)
			require.ErrorContains(t, err, "idempotency key")

			request.IdempotencyKey = "label-1"
			request.OperationKey = "op-label-1"
			shipment, err := provider.BuyLabel(context.Background(), request)
			require.NoError(t, err)

			outcome, err := provider.GetOutcomeByOperationKey(context.Background(), request.OperationKey)
			require.NoError(t, err)
			require.Equal(t, models.ProviderOutcomeSucceeded, outcome.Outcome)

			compensation, err := provider.CancelLabel(context.Background(), CancelLabelRequest{
				Provider: providerID, ProviderShipmentID: shipment.ProviderShipmentID,
				IdempotencyKey: "cancel-1", OperationKey: "op-cancel-1",
			})
			require.NoError(t, err)
			require.Equal(t, models.ProviderOutcomeSucceeded, compensation.Outcome)
			require.Equal(t, shipment.ProviderShipmentID, compensation.ProviderShipmentID)
		})
	}
}
