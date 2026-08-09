package providerops

import (
	"context"
	"errors"
	"time"

	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"
)

const (
	defaultProviderMutationTimeout = 15 * time.Second
	defaultLocalFinalizeAttempts   = 3
)

type DurableShippingLabelInput struct {
	ShippingLabelOperationInput
	Timeout          time.Duration
	FinalizeAttempts int
}

func (e *OperationExecutor) ExecuteDurableShippingLabel(ctx context.Context, input DurableShippingLabelInput) (models.ProviderOperation, error) {
	input.Registry = shippingLookupSafetyRegistry{ProviderRegistry: input.Registry}
	input.Finalize = retryShippingFinalization(input.Finalize, input.FinalizeAttempts)
	callCtx, cancel := providerMutationContext(ctx, input.Timeout)
	operation, err := e.ExecuteShippingLabel(callCtx, input.ShippingLabelOperationInput)
	cancel()
	if err == nil && unresolvedProviderOperation(operation) {
		err = ErrOutcomeUnknown
	}
	if err == nil || operation.Status != models.ProviderOperationStatusFinalizeRetry {
		return operation, err
	}

	compensationErr := e.compensateShippingLabel(ctx, operation, input)
	return operation, errors.Join(err, compensationErr)
}

func (e *OperationExecutor) compensateShippingLabel(ctx context.Context, parent models.ProviderOperation, input DurableShippingLabelInput) error {
	provider, err := input.Registry.Provider(input.Request.Provider)
	if err != nil {
		return err
	}
	request := shippingservice.CancelLabelRequest{
		Provider: input.Request.Provider, ProviderShipmentID: parent.ProviderReference,
		IdempotencyKey: parent.IdempotencyKey + ":cancel", OperationKey: parent.OperationKey + ":cancel",
		CorrelationID: input.Request.CorrelationID,
	}
	prepare := PrepareOperationInput{
		OperationKey: request.OperationKey, ProviderType: models.ProviderTypeShipping, ProviderID: input.Request.Provider,
		Environment: parent.Environment, Operation: "cancel_label", IdempotencyKey: request.IdempotencyKey,
		Request: request, CorrelationID: parent.CorrelationID, EntityType: parent.EntityType, EntityID: parent.EntityID,
	}
	prepared, err := e.Store().PrepareCompensation(context.Background(), parent.OperationKey, prepare)
	if err != nil {
		return err
	}
	prepare.ParentOperationID = prepared.ParentOperationID
	prepare.InitialStatus = models.ProviderOperationStatusCompensationPrepared
	callCtx, cancel := providerMutationContext(ctx, input.Timeout)
	compensation, err := e.Execute(callCtx, ExecuteOperationInput{
		Prepare: prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			outcome, callErr := provider.CancelLabel(callCtx, request)
			if callErr != nil {
				return ProviderCallResult{}, errors.Join(ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderShipmentID, Result: outcome}, nil
		},
		Lookup: func(callCtx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, operationKey)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderShipmentID, Result: outcome}, lookupErr
		},
	})
	cancel()
	if err == nil && unresolvedProviderOperation(compensation) {
		return ErrOutcomeUnknown
	}
	return err
}

type DurableTaxFinalizationInput struct {
	TaxFinalizationOperationInput
	Timeout          time.Duration
	FinalizeAttempts int
}

func (e *OperationExecutor) ExecuteDurableTaxFinalization(ctx context.Context, input DurableTaxFinalizationInput) (models.ProviderOperation, error) {
	input.Registry = taxLookupSafetyRegistry{ProviderRegistry: input.Registry}
	input.Finalize = retryTaxFinalization(input.Finalize, input.FinalizeAttempts)
	callCtx, cancel := providerMutationContext(ctx, input.Timeout)
	operation, err := e.ExecuteTaxFinalization(callCtx, input.TaxFinalizationOperationInput)
	cancel()
	if err == nil && unresolvedProviderOperation(operation) {
		err = ErrOutcomeUnknown
	}
	if err == nil || operation.Status != models.ProviderOperationStatusFinalizeRetry {
		return operation, err
	}

	compensationErr := e.compensateTaxFinalization(ctx, operation, input)
	return operation, errors.Join(err, compensationErr)
}

func (e *OperationExecutor) compensateTaxFinalization(ctx context.Context, parent models.ProviderOperation, input DurableTaxFinalizationInput) error {
	provider, err := input.Registry.Provider(input.Request.Provider)
	if err != nil {
		return err
	}
	request := taxservice.CancelFinalizationRequest{
		Provider: input.Request.Provider, ProviderReference: parent.ProviderReference,
		IdempotencyKey: parent.IdempotencyKey + ":cancel", OperationKey: parent.OperationKey + ":cancel",
		CorrelationID: input.Request.CorrelationID,
	}
	prepare := PrepareOperationInput{
		OperationKey: request.OperationKey, ProviderType: models.ProviderTypeTax, ProviderID: input.Request.Provider,
		Environment: parent.Environment, Operation: "cancel_finalization", IdempotencyKey: request.IdempotencyKey,
		Request: request, CorrelationID: parent.CorrelationID, EntityType: parent.EntityType, EntityID: parent.EntityID,
	}
	prepared, err := e.Store().PrepareCompensation(context.Background(), parent.OperationKey, prepare)
	if err != nil {
		return err
	}
	prepare.ParentOperationID = prepared.ParentOperationID
	prepare.InitialStatus = models.ProviderOperationStatusCompensationPrepared
	callCtx, cancel := providerMutationContext(ctx, input.Timeout)
	compensation, err := e.Execute(callCtx, ExecuteOperationInput{
		Prepare: prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			outcome, callErr := provider.CancelFinalization(callCtx, request)
			if callErr != nil {
				return ProviderCallResult{}, errors.Join(ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderReference, Result: outcome}, nil
		},
		Lookup: func(callCtx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, operationKey)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderReference, Result: outcome}, lookupErr
		},
	})
	cancel()
	if err == nil && unresolvedProviderOperation(compensation) {
		return ErrOutcomeUnknown
	}
	return err
}

func retryShippingFinalization(finalize func(context.Context, models.ProviderOperation, shippingservice.ProviderShipment) error, attempts int) func(context.Context, models.ProviderOperation, shippingservice.ProviderShipment) error {
	if finalize == nil {
		return nil
	}
	if attempts <= 0 {
		attempts = defaultLocalFinalizeAttempts
	}
	return func(ctx context.Context, operation models.ProviderOperation, result shippingservice.ProviderShipment) error {
		var err error
		for range attempts {
			if err = finalize(ctx, operation, result); err == nil {
				return nil
			}
		}
		return err
	}
}

func retryTaxFinalization(finalize func(context.Context, models.ProviderOperation, taxservice.TaxFinalized) error, attempts int) func(context.Context, models.ProviderOperation, taxservice.TaxFinalized) error {
	if finalize == nil {
		return nil
	}
	if attempts <= 0 {
		attempts = defaultLocalFinalizeAttempts
	}
	return func(ctx context.Context, operation models.ProviderOperation, result taxservice.TaxFinalized) error {
		var err error
		for range attempts {
			if err = finalize(ctx, operation, result); err == nil {
				return nil
			}
		}
		return err
	}
}

type shippingLookupSafetyRegistry struct {
	shippingservice.ProviderRegistry
}

func (r shippingLookupSafetyRegistry) Provider(providerID string) (shippingservice.ShippingProvider, error) {
	provider, err := r.ProviderRegistry.Provider(providerID)
	if err != nil {
		return nil, err
	}
	return shippingLookupSafetyProvider{ShippingProvider: provider}, nil
}

type shippingLookupSafetyProvider struct {
	shippingservice.ShippingProvider
}

func (p shippingLookupSafetyProvider) GetOutcomeByOperationKey(ctx context.Context, operationKey string) (shippingservice.ProviderOperationOutcome, error) {
	outcome, err := p.ShippingProvider.GetOutcomeByOperationKey(ctx, operationKey)
	if err == nil && outcome.Outcome == models.ProviderOutcomeSucceeded {
		outcome.Outcome = models.ProviderOutcomeUnknown
	}
	return outcome, err
}

type taxLookupSafetyRegistry struct{ taxservice.ProviderRegistry }

func (r taxLookupSafetyRegistry) Provider(providerID string) (taxservice.TaxProvider, error) {
	provider, err := r.ProviderRegistry.Provider(providerID)
	if err != nil {
		return nil, err
	}
	return taxLookupSafetyProvider{TaxProvider: provider}, nil
}

type taxLookupSafetyProvider struct{ taxservice.TaxProvider }

func (p taxLookupSafetyProvider) GetOutcomeByOperationKey(ctx context.Context, operationKey string) (taxservice.ProviderOperationOutcome, error) {
	outcome, err := p.TaxProvider.GetOutcomeByOperationKey(ctx, operationKey)
	if err == nil && outcome.Outcome == models.ProviderOutcomeSucceeded {
		outcome.Outcome = models.ProviderOutcomeUnknown
	}
	return outcome, err
}

func unresolvedProviderOperation(operation models.ProviderOperation) bool {
	return operation.Status == models.ProviderOperationStatusOutcomeUnknown || operation.Status == models.ProviderOperationStatusReconciliationRequired || operation.Status == models.ProviderOperationStatusReconciling
}

func providerMutationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultProviderMutationTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
