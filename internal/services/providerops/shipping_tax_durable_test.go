package providerops

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type durableShippingRegistry struct {
	provider shippingservice.ShippingProvider
}

func (r durableShippingRegistry) Provider(string) (shippingservice.ShippingProvider, error) {
	return r.provider, nil
}

type durableShippingProvider struct {
	buyCalls    atomic.Int32
	cancelCalls atomic.Int32
	buyErr      error
	lookup      shippingservice.ProviderOperationOutcome
}

func (p *durableShippingProvider) QuoteRates(context.Context, shippingservice.QuoteRatesRequest) ([]shippingservice.QuotedRate, error) {
	return nil, nil
}
func (p *durableShippingProvider) BuyLabel(context.Context, shippingservice.BuyLabelRequest) (shippingservice.ProviderShipment, error) {
	p.buyCalls.Add(1)
	if p.buyErr != nil {
		return shippingservice.ProviderShipment{}, p.buyErr
	}
	return shippingservice.ProviderShipment{ProviderShipmentID: "shipment-1", LabelURL: "label"}, nil
}
func (p *durableShippingProvider) CancelLabel(_ context.Context, req shippingservice.CancelLabelRequest) (shippingservice.ProviderOperationOutcome, error) {
	p.cancelCalls.Add(1)
	return shippingservice.ProviderOperationOutcome{OperationKey: req.OperationKey, Outcome: models.ProviderOutcomeSucceeded, ProviderShipmentID: req.ProviderShipmentID}, nil
}
func (p *durableShippingProvider) GetOutcomeByOperationKey(_ context.Context, operationKey string) (shippingservice.ProviderOperationOutcome, error) {
	outcome := p.lookup
	outcome.OperationKey = operationKey
	return outcome, nil
}
func (p *durableShippingProvider) VerifyWebhook(context.Context, map[string]string, []byte) (shippingservice.TrackingWebhookEvent, error) {
	return shippingservice.TrackingWebhookEvent{}, nil
}

type durableTaxRegistry struct{ provider taxservice.TaxProvider }

func (r durableTaxRegistry) Provider(string) (taxservice.TaxProvider, error) { return r.provider, nil }

type durableTaxProvider struct {
	finalizeCalls atomic.Int32
	cancelCalls   atomic.Int32
	finalizeErr   error
	lookup        taxservice.ProviderOperationOutcome
}

func (p *durableTaxProvider) QuoteTax(context.Context, taxservice.QuoteTaxRequest) (models.Money, error) {
	return 0, nil
}
func (p *durableTaxProvider) FinalizeTax(_ context.Context, req taxservice.FinalizeTaxRequest) (taxservice.TaxFinalized, error) {
	p.finalizeCalls.Add(1)
	if p.finalizeErr != nil {
		return taxservice.TaxFinalized{}, p.finalizeErr
	}
	return taxservice.TaxFinalized{Provider: req.Provider, ProviderReference: "tax-1", Currency: req.Currency}, nil
}
func (p *durableTaxProvider) CancelFinalization(_ context.Context, req taxservice.CancelFinalizationRequest) (taxservice.ProviderOperationOutcome, error) {
	p.cancelCalls.Add(1)
	return taxservice.ProviderOperationOutcome{OperationKey: req.OperationKey, Outcome: models.ProviderOutcomeSucceeded, ProviderReference: req.ProviderReference}, nil
}
func (p *durableTaxProvider) GetOutcomeByOperationKey(_ context.Context, operationKey string) (taxservice.ProviderOperationOutcome, error) {
	if p.lookup.Outcome != "" {
		outcome := p.lookup
		outcome.OperationKey = operationKey
		return outcome, nil
	}
	return taxservice.ProviderOperationOutcome{OperationKey: operationKey, Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "tax-1"}, nil
}
func (p *durableTaxProvider) ExportReport(context.Context, taxservice.ExportReportRequest) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func newDurableShippingTaxDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProviderOperation{}, &models.ProviderOperationAttempt{}, &models.ProviderReconciliationCase{}))
	return db
}

func TestExecuteDurableShippingLabelSuppressesDuplicateProviderMutation(t *testing.T) {
	db := newDurableShippingTaxDB(t)
	provider := &durableShippingProvider{}
	executor := NewOperationExecutor(db)
	finalizeCalls := 0
	input := DurableShippingLabelInput{ShippingLabelOperationInput: ShippingLabelOperationInput{
		Prepare:  PrepareOperationInput{OperationKey: "shipping-duplicate", ProviderType: models.ProviderTypeShipping, ProviderID: "ship", Environment: models.ProviderEnvironmentSandbox, Operation: "buy_label", IdempotencyKey: "same-key", Request: map[string]any{"rate": 1}},
		Registry: durableShippingRegistry{provider}, Request: shippingservice.BuyLabelRequest{Provider: "ship"},
		Finalize: func(context.Context, models.ProviderOperation, shippingservice.ProviderShipment) error {
			finalizeCalls++
			return nil
		},
	}}
	first, err := executor.ExecuteDurableShippingLabel(context.Background(), input)
	require.NoError(t, err)
	second, err := executor.ExecuteDurableShippingLabel(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompleted, first.Status)
	require.Equal(t, first.ID, second.ID)
	require.EqualValues(t, 1, provider.buyCalls.Load())
	require.Equal(t, 1, finalizeCalls)
}

func TestExecuteDurableShippingLabelRecordsUnresolvedTimeout(t *testing.T) {
	db := newDurableShippingTaxDB(t)
	provider := &durableShippingProvider{buyErr: context.DeadlineExceeded, lookup: shippingservice.ProviderOperationOutcome{Outcome: models.ProviderOutcomeSucceeded, ProviderShipmentID: "shipment-ambiguous"}}
	executor := NewOperationExecutor(db)
	operation, err := executor.ExecuteDurableShippingLabel(context.Background(), DurableShippingLabelInput{ShippingLabelOperationInput: ShippingLabelOperationInput{
		Prepare:  PrepareOperationInput{OperationKey: "shipping-timeout", ProviderType: models.ProviderTypeShipping, ProviderID: "ship", Environment: models.ProviderEnvironmentSandbox, Operation: "buy_label", IdempotencyKey: "timeout-key", Request: map[string]any{"rate": 1}},
		Registry: durableShippingRegistry{provider}, Request: shippingservice.BuyLabelRequest{Provider: "ship"},
	}})
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
	require.Len(t, operation.ReconciliationCases, 1)
	require.EqualValues(t, 1, provider.buyCalls.Load())
}

func TestExecuteDurableShippingLabelCompensatesAfterFinalizeAttemptsExhausted(t *testing.T) {
	db := newDurableShippingTaxDB(t)
	provider := &durableShippingProvider{}
	executor := NewOperationExecutor(db)
	finalizeCalls := 0
	operation, err := executor.ExecuteDurableShippingLabel(context.Background(), DurableShippingLabelInput{
		ShippingLabelOperationInput: ShippingLabelOperationInput{
			Prepare:  PrepareOperationInput{OperationKey: "shipping-compensate", ProviderType: models.ProviderTypeShipping, ProviderID: "ship", Environment: models.ProviderEnvironmentSandbox, Operation: "buy_label", IdempotencyKey: "compensate-key", Request: map[string]any{"rate": 1}},
			Registry: durableShippingRegistry{provider}, Request: shippingservice.BuyLabelRequest{Provider: "ship"},
			Finalize: func(context.Context, models.ProviderOperation, shippingservice.ProviderShipment) error {
				finalizeCalls++
				return errors.New("database unavailable")
			},
		}, FinalizeAttempts: 2,
	})
	require.ErrorContains(t, err, "database unavailable")
	require.Equal(t, models.ProviderOperationStatusFinalizeRetry, operation.Status)
	require.Equal(t, 2, finalizeCalls)
	require.EqualValues(t, 1, provider.buyCalls.Load())
	require.EqualValues(t, 1, provider.cancelCalls.Load())
	var compensation models.ProviderOperation
	require.NoError(t, db.Where("parent_operation_id = ?", operation.ID).First(&compensation).Error)
	require.Equal(t, models.ProviderOperationStatusCompleted, compensation.Status)
}

func TestExecuteDurableTaxFinalizationRecordsUnresolvedTimeout(t *testing.T) {
	db := newDurableShippingTaxDB(t)
	provider := &durableTaxProvider{finalizeErr: context.DeadlineExceeded, lookup: taxservice.ProviderOperationOutcome{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "tax-ambiguous"}}
	executor := NewOperationExecutor(db)
	operation, err := executor.ExecuteDurableTaxFinalization(context.Background(), DurableTaxFinalizationInput{TaxFinalizationOperationInput: TaxFinalizationOperationInput{
		Prepare:  PrepareOperationInput{OperationKey: "tax-timeout", ProviderType: models.ProviderTypeTax, ProviderID: "tax", Environment: models.ProviderEnvironmentSandbox, Operation: "finalize_tax", IdempotencyKey: "tax-timeout-key", Request: map[string]any{"order": 1}},
		Registry: durableTaxRegistry{provider}, Request: taxservice.FinalizeTaxRequest{Provider: "tax", Currency: "USD"},
	}})
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
	require.Len(t, operation.ReconciliationCases, 1)
	require.EqualValues(t, 1, provider.finalizeCalls.Load())
}

func TestExecuteDurableTaxFinalizationCompensatesAfterFinalizeAttemptsExhausted(t *testing.T) {
	db := newDurableShippingTaxDB(t)
	provider := &durableTaxProvider{}
	executor := NewOperationExecutor(db)
	finalizeCalls := 0
	operation, err := executor.ExecuteDurableTaxFinalization(context.Background(), DurableTaxFinalizationInput{
		TaxFinalizationOperationInput: TaxFinalizationOperationInput{
			Prepare:  PrepareOperationInput{OperationKey: "tax-compensate", ProviderType: models.ProviderTypeTax, ProviderID: "tax", Environment: models.ProviderEnvironmentSandbox, Operation: "finalize_tax", IdempotencyKey: "tax-key", Request: map[string]any{"order": 1}},
			Registry: durableTaxRegistry{provider}, Request: taxservice.FinalizeTaxRequest{Provider: "tax", Currency: "USD"},
			Finalize: func(context.Context, models.ProviderOperation, taxservice.TaxFinalized) error {
				finalizeCalls++
				return errors.New("tax rows unavailable")
			},
		}, FinalizeAttempts: 2,
	})
	require.ErrorContains(t, err, "tax rows unavailable")
	require.Equal(t, models.ProviderOperationStatusFinalizeRetry, operation.Status)
	require.Equal(t, 2, finalizeCalls)
	require.EqualValues(t, 1, provider.finalizeCalls.Load())
	require.EqualValues(t, 1, provider.cancelCalls.Load())
}
