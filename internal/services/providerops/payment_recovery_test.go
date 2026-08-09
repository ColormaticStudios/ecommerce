package providerops

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type recoveryPaymentRegistry struct {
	provider paymentservice.PaymentProvider
}

func (r recoveryPaymentRegistry) Provider(string) (paymentservice.PaymentProvider, error) {
	return r.provider, nil
}

type recoveryPaymentProvider struct {
	authorizeCalls atomic.Int32
	captureCalls   atomic.Int32
	voidCalls      atomic.Int32
	refundCalls    atomic.Int32
	authorizeErr   error
}

func (p *recoveryPaymentProvider) Authorize(context.Context, paymentservice.AuthorizeRequest) (paymentservice.ProviderOperationResult, error) {
	p.authorizeCalls.Add(1)
	if p.authorizeErr != nil {
		return paymentservice.ProviderOperationResult{}, p.authorizeErr
	}
	return paymentservice.ProviderOperationResult{ProviderTxnID: "authorize-reference", RawResponseRedacted: `{}`}, nil
}

func (p *recoveryPaymentProvider) Capture(context.Context, paymentservice.CaptureRequest) (paymentservice.ProviderOperationResult, error) {
	p.captureCalls.Add(1)
	return paymentservice.ProviderOperationResult{ProviderTxnID: "capture-reference", RawResponseRedacted: `{}`}, nil
}

func (p *recoveryPaymentProvider) Void(context.Context, paymentservice.VoidRequest) (paymentservice.ProviderOperationResult, error) {
	p.voidCalls.Add(1)
	return paymentservice.ProviderOperationResult{ProviderTxnID: "void-reference", RawResponseRedacted: `{}`}, nil
}

func (p *recoveryPaymentProvider) Refund(context.Context, paymentservice.RefundRequest) (paymentservice.ProviderOperationResult, error) {
	p.refundCalls.Add(1)
	return paymentservice.ProviderOperationResult{ProviderTxnID: "refund-reference", RawResponseRedacted: `{}`}, nil
}

func (*recoveryPaymentProvider) GetOutcomeByOperationKey(_ context.Context, operationKey string) (paymentservice.ProviderOperationOutcome, error) {
	return paymentservice.ProviderOperationOutcome{OperationKey: operationKey, Outcome: models.ProviderOutcomeUnknown}, nil
}

func (*recoveryPaymentProvider) VerifyWebhook(context.Context, map[string]string, []byte) (paymentservice.VerifiedWebhookEvent, error) {
	return paymentservice.VerifiedWebhookEvent{}, nil
}

func newPaymentRecoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentIntent{},
		&models.PaymentTransaction{},
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
		&models.ProviderCallAudit{},
	))
	return db
}

func TestPaymentRequestFingerprintIgnoresTransportIdentity(t *testing.T) {
	first := paymentservice.CaptureRequest{OrderID: 1, IntentID: 2, Amount: 100, Currency: "USD", Provider: "test", CorrelationID: "request-one", IdempotencyKey: "first", OperationKey: "operation-one"}
	second := first
	second.CorrelationID, second.IdempotencyKey, second.OperationKey = "request-two", "second", "operation-two"

	firstFingerprint, err := PaymentRequestFingerprint(first)
	require.NoError(t, err)
	secondFingerprint, err := PaymentRequestFingerprint(second)
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, secondFingerprint)
}

func TestPaymentFinalizationRetriesWithoutRepeatingProviderCall(t *testing.T) {
	db := newPaymentRecoveryDB(t)
	provider := &recoveryPaymentProvider{}
	executor := NewOperationExecutor(db)
	var finalizeCalls atomic.Int32
	request := paymentservice.AuthorizeRequest{OrderID: 1, SnapshotID: 1, Amount: models.MoneyFromFloat(10), Currency: "USD", Provider: "test"}

	operation, err := executor.ExecutePaymentAuthorize(context.Background(), PaymentMutationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "payment-retry-finalize", ProviderType: models.ProviderTypePayment, ProviderID: "test",
			Environment: models.ProviderEnvironmentSandbox, Operation: "authorize", IdempotencyKey: "retry-finalize",
			Request: request, EntityType: "payment_intent", EntityID: 1,
		},
		Registry: recoveryPaymentRegistry{provider: provider}, Request: request,
		Finalize: func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error {
			if finalizeCalls.Add(1) == 1 {
				return errors.New("transient database failure")
			}
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompleted, operation.Status)
	require.EqualValues(t, 1, provider.authorizeCalls.Load())
	require.EqualValues(t, 2, finalizeCalls.Load())
}

func TestBoundRuntimePreservesRecoverableUnknownPaymentOutcome(t *testing.T) {
	db := newPaymentRecoveryDB(t)
	provider := &recoveryPaymentProvider{authorizeErr: context.DeadlineExceeded}
	runtime := NewRuntime(nil, RuntimeConfig{PaymentProviders: recoveryPaymentRegistry{provider: provider}})
	runtime.BindDatabase(db)
	request := paymentservice.AuthorizeRequest{OrderID: 1, SnapshotID: 1, Amount: models.MoneyFromFloat(10), Currency: "USD", Provider: "test"}

	operation, err := runtime.Executor.ExecutePaymentAuthorize(context.Background(), PaymentMutationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "payment-runtime-unknown", ProviderType: models.ProviderTypePayment, ProviderID: "test",
			Environment: models.ProviderEnvironmentSandbox, Operation: "authorize", IdempotencyKey: "runtime-unknown",
			Request: request, EntityType: "payment_intent", EntityID: 1,
		},
		Registry: runtime.PaymentProviders, Request: request,
	})

	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
	require.EqualValues(t, 1, provider.authorizeCalls.Load())
}

func TestPaymentAuthorizationCompensatesWithDurableVoid(t *testing.T) {
	db := newPaymentRecoveryDB(t)
	provider := &recoveryPaymentProvider{}
	executor := NewOperationExecutor(db)
	intent, transaction := seedRecoveryPaymentIntent(t, db, models.PaymentIntentStatusRequiresAction, models.PaymentTransactionOperationAuthorize)
	request := paymentservice.AuthorizeRequest{OrderID: intent.OrderID, SnapshotID: intent.SnapshotID, Amount: intent.AuthorizedAmount, Currency: intent.Currency, Provider: intent.Provider}

	operation, err := executor.ExecutePaymentAuthorize(context.Background(), PaymentMutationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "payment-authorize-compensate", ProviderType: models.ProviderTypePayment, ProviderID: intent.Provider,
			Environment: models.ProviderEnvironmentSandbox, Operation: "authorize", IdempotencyKey: "authorize-compensate",
			Request: request, EntityType: "payment_intent", EntityID: intent.ID,
		},
		Registry: recoveryPaymentRegistry{provider: provider}, Request: request, DomainTransactionID: transaction.ID,
		Finalize: func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error {
			return errors.New("persistent order finalization failure")
		},
	})

	require.ErrorIs(t, err, ErrPaymentEffectCompensated)
	require.Equal(t, models.ProviderOperationStatusFinalizeRetry, operation.Status)
	require.EqualValues(t, 1, provider.authorizeCalls.Load())
	require.EqualValues(t, 1, provider.voidCalls.Load())

	var refreshed models.PaymentIntent
	require.NoError(t, db.Preload("Transactions").First(&refreshed, intent.ID).Error)
	require.Equal(t, models.PaymentIntentStatusVoided, refreshed.Status)
	require.Len(t, refreshed.Transactions, 2)
	var compensation models.ProviderOperation
	require.NoError(t, db.Where("parent_operation_id = ?", operation.ID).First(&compensation).Error)
	require.Equal(t, models.ProviderOperationStatusCompleted, compensation.Status)
}

func TestPaymentCaptureCompensatesWithDurableRefund(t *testing.T) {
	db := newPaymentRecoveryDB(t)
	provider := &recoveryPaymentProvider{}
	executor := NewOperationExecutor(db)
	intent, capture := seedRecoveryPaymentIntent(t, db, models.PaymentIntentStatusAuthorized, models.PaymentTransactionOperationCapture)
	authorize := models.PaymentTransaction{
		PaymentIntentID: intent.ID, Operation: models.PaymentTransactionOperationAuthorize, ProviderTxnID: "authorize-reference",
		IdempotencyKey: "authorize", Amount: intent.AuthorizedAmount, Status: models.PaymentTransactionStatusSucceeded,
	}
	require.NoError(t, db.Create(&authorize).Error)
	request := paymentservice.CaptureRequest{OrderID: intent.OrderID, IntentID: intent.ID, Amount: capture.Amount, Currency: intent.Currency, Provider: intent.Provider, ProviderTxnIDRef: authorize.ProviderTxnID}

	operation, err := executor.ExecutePaymentCapture(context.Background(), PaymentMutationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "payment-capture-compensate", ProviderType: models.ProviderTypePayment, ProviderID: intent.Provider,
			Environment: models.ProviderEnvironmentSandbox, Operation: "capture", IdempotencyKey: "capture-compensate",
			Request: request, EntityType: "payment_intent", EntityID: intent.ID,
		},
		Registry: recoveryPaymentRegistry{provider: provider}, Request: request, DomainTransactionID: capture.ID,
		Finalize: func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error {
			return errors.New("persistent order finalization failure")
		},
	})

	require.ErrorIs(t, err, ErrPaymentEffectCompensated)
	require.Equal(t, models.ProviderOperationStatusFinalizeRetry, operation.Status)
	require.EqualValues(t, 1, provider.captureCalls.Load())
	require.EqualValues(t, 1, provider.refundCalls.Load())

	var refreshed models.PaymentIntent
	require.NoError(t, db.Preload("Transactions").First(&refreshed, intent.ID).Error)
	require.Equal(t, models.PaymentIntentStatusRefunded, refreshed.Status)
	require.Len(t, refreshed.Transactions, 3)
}

func TestPaymentVoidFinalizationEscalatesToReconciliation(t *testing.T) {
	db := newPaymentRecoveryDB(t)
	provider := &recoveryPaymentProvider{}
	executor := NewOperationExecutor(db)
	intent, transaction := seedRecoveryPaymentIntent(t, db, models.PaymentIntentStatusAuthorized, models.PaymentTransactionOperationVoid)
	request := paymentservice.VoidRequest{OrderID: intent.OrderID, IntentID: intent.ID, Amount: transaction.Amount, Currency: intent.Currency, Provider: intent.Provider, ProviderTxnIDRef: "authorize-reference"}

	operation, err := executor.ExecutePaymentVoid(context.Background(), PaymentMutationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "payment-void-reconcile", ProviderType: models.ProviderTypePayment, ProviderID: intent.Provider,
			Environment: models.ProviderEnvironmentSandbox, Operation: "void", IdempotencyKey: "void-reconcile",
			Request: request, EntityType: "payment_intent", EntityID: intent.ID,
		},
		Registry: recoveryPaymentRegistry{provider: provider}, Request: request, DomainTransactionID: transaction.ID,
		Finalize: func(context.Context, models.ProviderOperation, paymentservice.ProviderOperationResult) error {
			return errors.New("persistent order finalization failure")
		},
	})

	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
	require.EqualValues(t, 1, provider.voidCalls.Load())
	var reconciliationCase models.ProviderReconciliationCase
	require.NoError(t, db.Where("provider_operation_id = ?", operation.ID).First(&reconciliationCase).Error)
	require.Equal(t, models.ProviderReconciliationCaseStatusOpen, reconciliationCase.Status)
}

func seedRecoveryPaymentIntent(t *testing.T, db *gorm.DB, status, operation string) (models.PaymentIntent, models.PaymentTransaction) {
	t.Helper()
	intent := models.PaymentIntent{
		OrderID: 1, SnapshotID: 1, Provider: "test", Status: status,
		AuthorizedAmount: models.MoneyFromFloat(10), Currency: "USD", Version: 1,
	}
	require.NoError(t, db.Create(&intent).Error)
	transaction := models.PaymentTransaction{
		PaymentIntentID: intent.ID, Operation: operation, IdempotencyKey: operation + "-key",
		Amount: intent.AuthorizedAmount, Status: models.PaymentTransactionStatusPending,
	}
	require.NoError(t, db.Create(&transaction).Error)
	return intent, transaction
}
