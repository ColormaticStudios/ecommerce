package providerops

import (
	"context"
	"encoding/json"
	"fmt"

	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"
)

type PaymentMutationInput struct {
	Prepare             PrepareOperationInput
	Registry            paymentservice.ProviderRegistry
	Request             any
	DomainTransactionID uint
	Finalize            func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error
}

func (e *OperationExecutor) ExecutePaymentCapture(ctx context.Context, input PaymentMutationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	request, ok := input.Request.(paymentservice.CaptureRequest)
	if !ok {
		return models.ProviderOperation{}, fmt.Errorf("payment capture request is required")
	}
	return e.executePayment(ctx, input, request.Provider, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		request.IdempotencyKey = input.Prepare.IdempotencyKey
		request.OperationKey = input.Prepare.OperationKey
		return provider.Capture(callCtx, request)
	})
}

func (e *OperationExecutor) ExecutePaymentVoid(ctx context.Context, input PaymentMutationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	request, ok := input.Request.(paymentservice.VoidRequest)
	if !ok {
		return models.ProviderOperation{}, fmt.Errorf("payment void request is required")
	}
	return e.executePayment(ctx, input, request.Provider, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		request.IdempotencyKey = input.Prepare.IdempotencyKey
		request.OperationKey = input.Prepare.OperationKey
		return provider.Void(callCtx, request)
	})
}

func (e *OperationExecutor) ExecutePaymentRefund(ctx context.Context, input PaymentMutationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	request, ok := input.Request.(paymentservice.RefundRequest)
	if !ok {
		return models.ProviderOperation{}, fmt.Errorf("payment refund request is required")
	}
	return e.executePayment(ctx, input, request.Provider, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		request.IdempotencyKey = input.Prepare.IdempotencyKey
		request.OperationKey = input.Prepare.OperationKey
		return provider.Refund(callCtx, request)
	})
}

func (e *OperationExecutor) executePayment(
	ctx context.Context,
	input PaymentMutationInput,
	providerID string,
	call func(context.Context, paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error),
) (models.ProviderOperation, error) {
	provider, err := input.Registry.Provider(providerID)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	input.Finalize = retryPaymentFinalization(input.Finalize, PaymentFinalizeAttempts)
	operation, executeErr := e.Execute(ctx, ExecuteOperationInput{
		Prepare: input.Prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			result, callErr := call(callCtx, provider)
			if callErr != nil {
				return ProviderCallResult{}, fmt.Errorf("%w: %v", ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: result.ProviderTxnID, Result: result}, nil
		},
		Lookup: func(callCtx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, operationKey)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderTxnID, Result: paymentservice.ProviderOperationResult{
				ProviderTxnID: outcome.ProviderTxnID, RawResponseRedacted: outcome.RawResponseRedacted,
			}}, lookupErr
		},
		Finalize: func(finalizeCtx context.Context, operation models.ProviderOperation, raw json.RawMessage) error {
			if input.Finalize == nil {
				return nil
			}
			var result paymentservice.ProviderOperationResult
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			return input.Finalize(finalizeCtx, operation, result)
		},
	})
	if executeErr == nil || operation.Status != models.ProviderOperationStatusFinalizeRetry {
		return operation, executeErr
	}
	return e.recoverPaymentFinalization(ctx, operation, input, executeErr)
}

type ShippingLabelOperationInput struct {
	Prepare  PrepareOperationInput
	Registry shippingservice.ProviderRegistry
	Request  shippingservice.BuyLabelRequest
	Finalize func(context.Context, models.ProviderOperation, shippingservice.ProviderShipment) error
}

func (e *OperationExecutor) ExecuteShippingLabel(ctx context.Context, input ShippingLabelOperationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	provider, err := input.Registry.Provider(input.Request.Provider)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	input.Request.IdempotencyKey = input.Prepare.IdempotencyKey
	input.Request.OperationKey = input.Prepare.OperationKey
	return e.Execute(ctx, ExecuteOperationInput{
		Prepare: input.Prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			result, callErr := provider.BuyLabel(callCtx, input.Request)
			if callErr != nil {
				return ProviderCallResult{}, fmt.Errorf("%w: %v", ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: result.ProviderShipmentID, Result: result}, nil
		},
		Lookup: func(callCtx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, operationKey)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderShipmentID, Result: shippingservice.ProviderShipment{
				ProviderShipmentID: outcome.ProviderShipmentID,
			}}, lookupErr
		},
		Finalize: func(finalizeCtx context.Context, operation models.ProviderOperation, raw json.RawMessage) error {
			if input.Finalize == nil {
				return nil
			}
			var result shippingservice.ProviderShipment
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			return input.Finalize(finalizeCtx, operation, result)
		},
	})
}

func requirePublicOperationIdentity(input PrepareOperationInput) error {
	if input.IdempotencyKey == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if input.OperationKey == "" {
		return fmt.Errorf("operation key is required")
	}
	return nil
}

type TaxFinalizationOperationInput struct {
	Prepare  PrepareOperationInput
	Registry taxservice.ProviderRegistry
	Request  taxservice.FinalizeTaxRequest
	Finalize func(context.Context, models.ProviderOperation, taxservice.TaxFinalized) error
}

func (e *OperationExecutor) ExecuteTaxFinalization(ctx context.Context, input TaxFinalizationOperationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	provider, err := input.Registry.Provider(input.Request.Provider)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	input.Request.IdempotencyKey = input.Prepare.IdempotencyKey
	input.Request.OperationKey = input.Prepare.OperationKey
	return e.Execute(ctx, ExecuteOperationInput{
		Prepare: input.Prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			result, callErr := provider.FinalizeTax(callCtx, input.Request)
			if callErr != nil {
				return ProviderCallResult{}, fmt.Errorf("%w: %v", ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: result.ProviderReference, Result: result}, nil
		},
		Lookup: func(callCtx context.Context, operationKey string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, operationKey)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderReference, Result: taxservice.TaxFinalized{
				Provider: input.Request.Provider, ProviderReference: outcome.ProviderReference, Currency: input.Request.Currency,
			}}, lookupErr
		},
		Finalize: func(finalizeCtx context.Context, operation models.ProviderOperation, raw json.RawMessage) error {
			if input.Finalize == nil {
				return nil
			}
			var result taxservice.TaxFinalized
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			return input.Finalize(finalizeCtx, operation, result)
		},
	})
}
