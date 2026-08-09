package providerops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const PaymentFinalizeAttempts = 3

var ErrPaymentEffectCompensated = errors.New("payment provider effect was compensated after local finalization failed")

type paymentOperationEnvironmentProvider interface {
	PaymentOperationEnvironment() string
}

func (r paymentRegistryWrapper) PaymentOperationEnvironment() string {
	return r.environment
}

func PaymentOperationEnvironment(registry paymentservice.ProviderRegistry) string {
	if provider, ok := registry.(paymentOperationEnvironmentProvider); ok {
		if environment := strings.TrimSpace(provider.PaymentOperationEnvironment()); environment != "" {
			return environment
		}
	}
	return models.ProviderEnvironmentSandbox
}

func PaymentOperationKey(endpointScope, idempotencyKey string) string {
	scope := strings.TrimSpace(endpointScope)
	sum := sha256.Sum256([]byte(scope + "\x00" + strings.TrimSpace(idempotencyKey)))
	return "payment:" + hex.EncodeToString(sum[:])
}

// PaymentRequestFingerprint hashes semantic mutation input only. Correlation and
// provider operation identities vary by transport attempt or are represented by
// dedicated ledger columns, so they must not turn a valid replay into a conflict.
func PaymentRequestFingerprint(request any) (string, error) {
	switch value := request.(type) {
	case paymentservice.AuthorizeRequest:
		value.CorrelationID, value.IdempotencyKey, value.OperationKey = "", "", ""
		request = value
	case paymentservice.CaptureRequest:
		value.CorrelationID, value.IdempotencyKey, value.OperationKey = "", "", ""
		request = value
	case paymentservice.VoidRequest:
		value.CorrelationID, value.IdempotencyKey, value.OperationKey = "", "", ""
		request = value
	case paymentservice.RefundRequest:
		value.CorrelationID, value.IdempotencyKey, value.OperationKey = "", "", ""
		request = value
	}
	return RequestFingerprint(request)
}

func (e *OperationExecutor) ExecutePaymentAuthorize(ctx context.Context, input PaymentMutationInput) (models.ProviderOperation, error) {
	if err := requirePublicOperationIdentity(input.Prepare); err != nil {
		return models.ProviderOperation{}, err
	}
	request, ok := input.Request.(paymentservice.AuthorizeRequest)
	if !ok {
		return models.ProviderOperation{}, fmt.Errorf("payment authorize request is required")
	}
	return e.executePayment(ctx, input, request.Provider, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		request.IdempotencyKey = input.Prepare.IdempotencyKey
		request.OperationKey = input.Prepare.OperationKey
		return provider.Authorize(callCtx, request)
	})
}

func retryPaymentFinalization(finalize func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error, attempts int) func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error {
	if finalize == nil {
		return nil
	}
	if attempts <= 0 {
		attempts = PaymentFinalizeAttempts
	}
	return func(ctx context.Context, operation models.ProviderOperation, result paymentservice.ProviderOperationResult) error {
		var err error
		for range attempts {
			if err = finalize(ctx, operation, result); err == nil {
				return nil
			}
		}
		return err
	}
}

func (e *OperationExecutor) recoverPaymentFinalization(ctx context.Context, parent models.ProviderOperation, input PaymentMutationInput, finalizeErr error) (models.ProviderOperation, error) {
	switch request := input.Request.(type) {
	case paymentservice.AuthorizeRequest:
		compensationErr := e.compensateAuthorization(ctx, parent, input, request)
		return parent, errors.Join(ErrPaymentEffectCompensated, finalizeErr, compensationErr)
	case paymentservice.CaptureRequest:
		compensationErr := e.compensateCapture(ctx, parent, input, request)
		return parent, errors.Join(ErrPaymentEffectCompensated, finalizeErr, compensationErr)
	case paymentservice.VoidRequest, paymentservice.RefundRequest:
		escalated, err := e.EscalatePaymentFinalization(ctx, parent.OperationKey, finalizeErr.Error())
		return escalated, err
	default:
		return parent, finalizeErr
	}
}

func (e *OperationExecutor) compensateAuthorization(ctx context.Context, parent models.ProviderOperation, input PaymentMutationInput, request paymentservice.AuthorizeRequest) error {
	var authorizeResult paymentservice.ProviderOperationResult
	if err := json.Unmarshal([]byte(parent.ResultJSON), &authorizeResult); err != nil {
		return err
	}
	var transaction models.PaymentTransaction
	var compensationRequest paymentservice.VoidRequest
	err := e.store.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		transaction, compensationRequest, prepareErr = paymentservice.PrepareAuthorizationVoidCompensation(
			tx, input.Prepare.EntityID, parent.IdempotencyKey+":compensate:void", authorizeResult.ProviderTxnID, request.CorrelationID,
		)
		return prepareErr
	})
	if err != nil {
		return err
	}
	compensationRequest.OperationKey = parent.OperationKey + ":compensate:void"
	compensationRequest.IdempotencyKey = parent.IdempotencyKey + ":compensate:void"
	return e.executePaymentCompensation(ctx, parent, input.Registry, compensationRequest.Provider, "void", compensationRequest.IdempotencyKey, compensationRequest.OperationKey, compensationRequest, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		return provider.Void(callCtx, compensationRequest)
	}, func(tx *gorm.DB, compensationResult paymentservice.ProviderOperationResult) error {
		_, _, finalizeCompensationErr := paymentservice.FinalizeAuthorizationVoidCompensation(tx, input.Prepare.EntityID, transaction.ID, authorizeResult, compensationResult)
		return finalizeCompensationErr
	})
}

func (e *OperationExecutor) compensateCapture(ctx context.Context, parent models.ProviderOperation, input PaymentMutationInput, request paymentservice.CaptureRequest) error {
	var captureResult paymentservice.ProviderOperationResult
	if err := json.Unmarshal([]byte(parent.ResultJSON), &captureResult); err != nil {
		return err
	}
	var transaction models.PaymentTransaction
	var compensationRequest paymentservice.RefundRequest
	err := e.store.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		transaction, compensationRequest, prepareErr = paymentservice.PrepareCaptureRefundCompensation(
			tx, request.IntentID, input.DomainTransactionID, parent.IdempotencyKey+":compensate:refund", captureResult.ProviderTxnID, request.CorrelationID,
		)
		return prepareErr
	})
	if err != nil {
		return err
	}
	compensationRequest.OperationKey = parent.OperationKey + ":compensate:refund"
	compensationRequest.IdempotencyKey = parent.IdempotencyKey + ":compensate:refund"
	return e.executePaymentCompensation(ctx, parent, input.Registry, compensationRequest.Provider, "refund", compensationRequest.IdempotencyKey, compensationRequest.OperationKey, compensationRequest, func(callCtx context.Context, provider paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error) {
		return provider.Refund(callCtx, compensationRequest)
	}, func(tx *gorm.DB, compensationResult paymentservice.ProviderOperationResult) error {
		_, _, finalizeCompensationErr := paymentservice.FinalizeCaptureRefundCompensation(tx, request.IntentID, input.DomainTransactionID, transaction.ID, captureResult, compensationResult)
		return finalizeCompensationErr
	})
}

func (e *OperationExecutor) executePaymentCompensation(
	ctx context.Context,
	parent models.ProviderOperation,
	registry paymentservice.ProviderRegistry,
	providerID, operation, idempotencyKey, operationKey string,
	request any,
	call func(context.Context, paymentservice.PaymentProvider) (paymentservice.ProviderOperationResult, error),
	finalize func(*gorm.DB, paymentservice.ProviderOperationResult) error,
) error {
	provider, err := registry.Provider(providerID)
	if err != nil {
		return err
	}
	prepare := PrepareOperationInput{
		OperationKey: operationKey, ProviderType: models.ProviderTypePayment, ProviderID: providerID,
		Environment: parent.Environment, Operation: operation, IdempotencyKey: idempotencyKey, Request: request,
		CorrelationID: parent.CorrelationID, EntityType: parent.EntityType, EntityID: parent.EntityID,
	}
	prepared, err := e.store.PrepareCompensation(ctx, parent.OperationKey, prepare)
	if err != nil {
		return err
	}
	prepare.ParentOperationID = prepared.ParentOperationID
	prepare.InitialStatus = models.ProviderOperationStatusCompensationPrepared
	compensation, err := e.Execute(ctx, ExecuteOperationInput{
		Prepare: prepare,
		Call: func(callCtx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			result, callErr := call(callCtx, provider)
			if callErr != nil {
				return ProviderCallResult{}, errors.Join(ErrOutcomeUnknown, callErr)
			}
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: result.ProviderTxnID, Result: result}, nil
		},
		Lookup: func(callCtx context.Context, key string) (ProviderCallResult, error) {
			outcome, lookupErr := provider.GetOutcomeByOperationKey(callCtx, key)
			return ProviderCallResult{Outcome: outcome.Outcome, ProviderReference: outcome.ProviderTxnID, Result: paymentservice.ProviderOperationResult{ProviderTxnID: outcome.ProviderTxnID, RawResponseRedacted: outcome.RawResponseRedacted}}, lookupErr
		},
		Finalize: func(finalizeCtx context.Context, _ models.ProviderOperation, raw json.RawMessage) error {
			var result paymentservice.ProviderOperationResult
			if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
				return unmarshalErr
			}
			return e.store.db.WithContext(finalizeCtx).Transaction(func(tx *gorm.DB) error { return finalize(tx, result) })
		},
	})
	if err == nil && PaymentOperationRecoverable(compensation) {
		return ErrOutcomeUnknown
	}
	return err
}

func (e *OperationExecutor) MakePaymentFinalizationRetryReady(ctx context.Context, operationKey string) error {
	if e == nil || e.store == nil || e.store.db == nil {
		return errors.New("provider operation executor is not configured")
	}
	result := e.store.db.WithContext(nonNilContext(ctx)).Model(&models.ProviderOperation{}).
		Where("operation_key = ? AND status = ?", strings.TrimSpace(operationKey), models.ProviderOperationStatusFinalizeRetry).
		Updates(map[string]any{"next_attempt_at": nil, "lease_owner": "", "lease_expires_at": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		operation, err := e.store.GetOperationByKey(ctx, operationKey)
		if err != nil {
			return err
		}
		if operation.Status != models.ProviderOperationStatusCompleted {
			return ErrOperationNotClaimable
		}
	}
	return nil
}

func (e *OperationExecutor) EscalatePaymentFinalization(ctx context.Context, operationKey, reason string) (models.ProviderOperation, error) {
	if e == nil || e.store == nil || e.store.db == nil {
		return models.ProviderOperation{}, errors.New("provider operation executor is not configured")
	}
	var operation models.ProviderOperation
	err := e.store.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("operation_key = ?", strings.TrimSpace(operationKey)).First(&operation).Error; err != nil {
			return err
		}
		if operation.Status == models.ProviderOperationStatusReconciliationRequired {
			return nil
		}
		if operation.Status != models.ProviderOperationStatusFinalizeRetry && operation.Status != models.ProviderOperationStatusFinalizing {
			return ErrInvalidReconciliationCaseState
		}
		if err := tx.Model(&operation).Updates(map[string]any{
			"status":           models.ProviderOperationStatusReconciliationRequired,
			"last_error":       strings.TrimSpace(reason),
			"next_attempt_at":  nil,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"version":          gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		openKey := "open"
		now := time.Now().UTC()
		caseRecord := models.ProviderReconciliationCase{
			ProviderOperationID: operation.ID,
			OpenKey:             &openKey,
			Status:              models.ProviderReconciliationCaseStatusOpen,
			Reason:              strings.TrimSpace(reason),
			CaseType:            "payment_finalization",
			ProviderOutcome:     operation.ProviderOutcome,
			OperationKey:        operation.OperationKey,
			OpenedAt:            now,
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&caseRecord).Error
	})
	if err != nil {
		return models.ProviderOperation{}, err
	}
	return e.store.GetOperationByKey(ctx, operationKey)
}

func PaymentOperationRecoverable(operation models.ProviderOperation) bool {
	switch operation.Status {
	case models.ProviderOperationStatusPrepared,
		models.ProviderOperationStatusExecuting,
		models.ProviderOperationStatusOutcomeUnknown,
		models.ProviderOperationStatusProviderSucceeded,
		models.ProviderOperationStatusFinalizing,
		models.ProviderOperationStatusFinalizeRetry,
		models.ProviderOperationStatusReconciliationRequired,
		models.ProviderOperationStatusReconciling,
		models.ProviderOperationStatusCompensationPrepared,
		models.ProviderOperationStatusCompensating,
		models.ProviderOperationStatusCompensationSucceeded,
		models.ProviderOperationStatusCompensationRetry:
		return true
	default:
		return false
	}
}
