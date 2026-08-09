package providerops

import (
	"context"
	"encoding/json"
	"sync"

	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"
)

// OperationLifecycle contains the process callbacks that are safe to resume for
// a durable operation. Call and Finalize are intentionally not inferred from
// domain metadata: domain owners must register those callbacks explicitly.
type OperationLifecycle struct {
	Call     func(context.Context, models.ProviderOperation) (ProviderCallResult, error)
	Lookup   func(context.Context, string) (ProviderCallResult, error)
	Finalize func(context.Context, models.ProviderOperation, json.RawMessage) error
}

// LifecycleResolver resolves only callbacks and providers known to be safe for
// a persisted operation. Returning false leaves the operation untouched.
type LifecycleResolver interface {
	Resolve(models.ProviderOperation) (OperationLifecycle, bool)
}

type LifecycleRegistry struct {
	mu        sync.RWMutex
	callbacks map[string]OperationLifecycle
}

func NewLifecycleRegistry() *LifecycleRegistry {
	return &LifecycleRegistry{callbacks: make(map[string]OperationLifecycle)}
}

func (r *LifecycleRegistry) Register(operationKey string, lifecycle OperationLifecycle) {
	if r == nil || operationKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks[operationKey] = lifecycle
}

func (r *LifecycleRegistry) Resolve(operation models.ProviderOperation) (OperationLifecycle, bool) {
	if r == nil {
		return OperationLifecycle{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	lifecycle, ok := r.callbacks[operation.OperationKey]
	return lifecycle, ok
}

type RuntimeLifecycleResolver struct {
	registered *LifecycleRegistry
	payments   paymentservice.ProviderRegistry
	shipping   shippingservice.ProviderRegistry
	tax        taxservice.ProviderRegistry
}

func NewRuntimeLifecycleResolver(
	registered *LifecycleRegistry,
	payments paymentservice.ProviderRegistry,
	shipping shippingservice.ProviderRegistry,
	tax taxservice.ProviderRegistry,
) *RuntimeLifecycleResolver {
	return &RuntimeLifecycleResolver{registered: registered, payments: payments, shipping: shipping, tax: tax}
}

func (r *RuntimeLifecycleResolver) Resolve(operation models.ProviderOperation) (OperationLifecycle, bool) {
	lifecycle, registered := r.registered.Resolve(operation)
	if lifecycle.Lookup != nil {
		return lifecycle, true
	}

	switch operation.ProviderType {
	case models.ProviderTypePayment:
		if r.payments == nil {
			return lifecycle, registered
		}
		provider, err := r.payments.Provider(operation.ProviderID)
		if err != nil {
			return lifecycle, registered
		}
		lifecycle.Lookup = func(ctx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(ctx, operationKey)
			return ProviderCallResult{
				Outcome:           outcome.Outcome,
				ProviderReference: outcome.ProviderTxnID,
				Result: paymentservice.ProviderOperationResult{
					ProviderTxnID: outcome.ProviderTxnID, RawResponseRedacted: outcome.RawResponseRedacted,
				},
			}, lookupErr
		}
	case models.ProviderTypeShipping:
		if r.shipping == nil {
			return lifecycle, registered
		}
		provider, err := r.shipping.Provider(operation.ProviderID)
		if err != nil {
			return lifecycle, registered
		}
		lifecycle.Lookup = func(ctx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(ctx, operationKey)
			return ProviderCallResult{
				Outcome:           outcome.Outcome,
				ProviderReference: outcome.ProviderShipmentID,
				Result: shippingservice.ProviderShipment{
					ProviderShipmentID: outcome.ProviderShipmentID,
				},
			}, lookupErr
		}
	case models.ProviderTypeTax:
		if r.tax == nil {
			return lifecycle, registered
		}
		provider, err := r.tax.Provider(operation.ProviderID)
		if err != nil {
			return lifecycle, registered
		}
		lifecycle.Lookup = func(ctx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(ctx, operationKey)
			return ProviderCallResult{
				Outcome:           outcome.Outcome,
				ProviderReference: outcome.ProviderReference,
				Result: taxservice.TaxFinalized{
					Provider: operation.ProviderID, ProviderReference: outcome.ProviderReference,
				},
			}, lookupErr
		}
	default:
		return lifecycle, registered
	}
	return lifecycle, true
}
