package providerops

import (
	"context"
	"errors"
	"io"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/internal/providercontext"
	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"

	"gorm.io/gorm"
)

type RuntimeConfig struct {
	Environment       string
	Credentials       *CredentialService
	PaymentProviders  paymentservice.ProviderRegistry
	ShippingProviders shippingservice.ProviderRegistry
	TaxProviders      taxservice.ProviderRegistry
}

type Runtime struct {
	Environment       string
	Credentials       *CredentialService
	Audit             *AuditService
	PaymentProviders  paymentservice.ProviderRegistry
	ShippingProviders shippingservice.ProviderRegistry
	TaxProviders      taxservice.ProviderRegistry
	Reconciliation    *ReconciliationService
}

func NewRuntime(db *gorm.DB, cfg RuntimeConfig) *Runtime {
	environment := models.ProviderEnvironmentSandbox
	if normalized, err := normalizeProviderEnvironment(defaultIfEmpty(cfg.Environment, environment)); err == nil {
		environment = normalized
	}

	credentials := cfg.Credentials
	if credentials == nil {
		credentials = &CredentialService{}
	}
	audit := NewAuditService(db)

	basePayment := cfg.PaymentProviders
	if basePayment == nil {
		basePayment = paymentservice.NewDefaultProviderRegistry()
	}
	baseShipping := cfg.ShippingProviders
	if baseShipping == nil {
		baseShipping = shippingservice.NewDefaultProviderRegistry()
	}
	baseTax := cfg.TaxProviders
	if baseTax == nil {
		baseTax = taxservice.NewDefaultProviderRegistry()
	}

	runtime := &Runtime{
		Environment: environment,
		Credentials: credentials,
		Audit:       audit,
	}
	runtime.PaymentProviders = paymentRegistryWrapper{
		base:        basePayment,
		environment: environment,
		credentials: credentials,
		audit:       audit,
	}
	runtime.ShippingProviders = shippingRegistryWrapper{
		base:        baseShipping,
		environment: environment,
		credentials: credentials,
		audit:       audit,
	}
	runtime.TaxProviders = taxRegistryWrapper{
		base:        baseTax,
		environment: environment,
		credentials: credentials,
		audit:       audit,
	}
	runtime.Reconciliation = NewReconciliationService(
		db,
		environment,
		runtime.PaymentProviders,
		runtime.ShippingProviders,
		runtime.TaxProviders,
	)
	return runtime
}

type paymentRegistryWrapper struct {
	base        paymentservice.ProviderRegistry
	environment string
	credentials *CredentialService
	audit       *AuditService
}

func (r paymentRegistryWrapper) Provider(providerID string) (paymentservice.PaymentProvider, error) {
	provider, err := r.base.Provider(providerID)
	if err != nil {
		return nil, err
	}
	return paymentProviderWrapper{
		providerID:      providerID,
		environment:     r.environment,
		credentials:     r.credentials,
		audit:           r.audit,
		PaymentProvider: provider,
	}, nil
}

type paymentProviderWrapper struct {
	providerID  string
	environment string
	credentials *CredentialService
	audit       *AuditService
	paymentservice.PaymentProvider
}

func (w paymentProviderWrapper) Authorize(ctx context.Context, req paymentservice.AuthorizeRequest) (paymentservice.ProviderOperationResult, error) {
	return recordPaymentCall(ctx, w, "authorize", req.CorrelationID, req.IdempotencyKey, req, func() (paymentservice.ProviderOperationResult, error) {
		callCtx, err := w.prepareContext(ctx, req.Provider, req.Currency)
		if err != nil {
			return paymentservice.ProviderOperationResult{}, err
		}
		return w.PaymentProvider.Authorize(callCtx, req)
	})
}

func (w paymentProviderWrapper) Capture(ctx context.Context, req paymentservice.CaptureRequest) (paymentservice.ProviderOperationResult, error) {
	return recordPaymentCall(ctx, w, "capture", req.CorrelationID, req.IdempotencyKey, req, func() (paymentservice.ProviderOperationResult, error) {
		callCtx, err := w.prepareContext(ctx, req.Provider, req.Currency)
		if err != nil {
			return paymentservice.ProviderOperationResult{}, err
		}
		return w.PaymentProvider.Capture(callCtx, req)
	})
}

func (w paymentProviderWrapper) Void(ctx context.Context, req paymentservice.VoidRequest) (paymentservice.ProviderOperationResult, error) {
	return recordPaymentCall(ctx, w, "void", req.CorrelationID, req.IdempotencyKey, req, func() (paymentservice.ProviderOperationResult, error) {
		callCtx, err := w.prepareContext(ctx, req.Provider, req.Currency)
		if err != nil {
			return paymentservice.ProviderOperationResult{}, err
		}
		return w.PaymentProvider.Void(callCtx, req)
	})
}

func (w paymentProviderWrapper) Refund(ctx context.Context, req paymentservice.RefundRequest) (paymentservice.ProviderOperationResult, error) {
	return recordPaymentCall(ctx, w, "refund", req.CorrelationID, req.IdempotencyKey, req, func() (paymentservice.ProviderOperationResult, error) {
		callCtx, err := w.prepareContext(ctx, req.Provider, req.Currency)
		if err != nil {
			return paymentservice.ProviderOperationResult{}, err
		}
		return w.PaymentProvider.Refund(callCtx, req)
	})
}

func (w paymentProviderWrapper) GetTransaction(ctx context.Context, providerTxnID string) (paymentservice.ProviderTransaction, error) {
	lookupProvider, ok := w.PaymentProvider.(paymentservice.TransactionLookupProvider)
	if !ok {
		return paymentservice.ProviderTransaction{}, errors.New("payment provider transaction lookup is unsupported")
	}
	return recordPaymentLookup(ctx, w, "get_transaction", providerTxnID, func() (paymentservice.ProviderTransaction, error) {
		callCtx, err := w.prepareContext(ctx, w.providerID, "")
		if err != nil {
			return paymentservice.ProviderTransaction{}, err
		}
		return lookupProvider.GetTransaction(callCtx, providerTxnID)
	})
}

func (w paymentProviderWrapper) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (paymentservice.VerifiedWebhookEvent, error) {
	callCtx, err := w.prepareContext(ctx, w.providerID, "")
	if err != nil {
		return paymentservice.VerifiedWebhookEvent{}, err
	}
	return w.PaymentProvider.VerifyWebhook(callCtx, headers, body)
}

func (w paymentProviderWrapper) ParseStoredWebhook(ctx context.Context, body []byte) (paymentservice.VerifiedWebhookEvent, error) {
	parser, ok := w.PaymentProvider.(paymentservice.StoredWebhookParser)
	if !ok {
		return paymentservice.VerifiedWebhookEvent{}, errors.New("payment provider stored webhook parsing is unsupported")
	}
	callCtx, err := w.prepareContext(ctx, w.providerID, "")
	if err != nil {
		return paymentservice.VerifiedWebhookEvent{}, err
	}
	return parser.ParseStoredWebhook(callCtx, body)
}

func (w paymentProviderWrapper) prepareContext(ctx context.Context, providerID, currency string) (context.Context, error) {
	credential, err := resolveProviderCredential(
		ctx,
		w.audit,
		w.credentials,
		models.ProviderTypePayment,
		providerID,
		w.environment,
	)
	if err != nil {
		return ctx, err
	}
	if credential == nil {
		return ctx, nil
	}
	if currency != "" {
		if err := w.credentials.ValidateCurrency(currency, credential); err != nil {
			return ctx, err
		}
	}
	return providercontext.WithRuntimeData(ctx, providercontext.RuntimeData{
		Environment: w.environment,
		Credentials: credential.SecretData,
	}), nil
}

func recordPaymentCall(
	ctx context.Context,
	w paymentProviderWrapper,
	operation string,
	correlationID string,
	idempotencyKey string,
	request any,
	call func() (paymentservice.ProviderOperationResult, error),
) (paymentservice.ProviderOperationResult, error) {
	start := time.Now()
	response, err := call()
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypePayment,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       operation,
		CorrelationID:   correlationID,
		IdempotencyKey:  idempotencyKey,
		Status:          status,
		RequestPayload:  request,
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func recordPaymentLookup(
	ctx context.Context,
	w paymentProviderWrapper,
	operation string,
	providerTxnID string,
	call func() (paymentservice.ProviderTransaction, error),
) (paymentservice.ProviderTransaction, error) {
	start := time.Now()
	response, err := call()
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypePayment,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       operation,
		Status:          status,
		RequestPayload:  map[string]string{"provider_txn_id": providerTxnID},
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

type shippingRegistryWrapper struct {
	base        shippingservice.ProviderRegistry
	environment string
	credentials *CredentialService
	audit       *AuditService
}

func (r shippingRegistryWrapper) Provider(providerID string) (shippingservice.ShippingProvider, error) {
	provider, err := r.base.Provider(providerID)
	if err != nil {
		return nil, err
	}
	return shippingProviderWrapper{
		providerID:       providerID,
		environment:      r.environment,
		credentials:      r.credentials,
		audit:            r.audit,
		ShippingProvider: provider,
	}, nil
}

type shippingProviderWrapper struct {
	providerID  string
	environment string
	credentials *CredentialService
	audit       *AuditService
	shippingservice.ShippingProvider
}

func (w shippingProviderWrapper) QuoteRates(ctx context.Context, req shippingservice.QuoteRatesRequest) ([]shippingservice.QuotedRate, error) {
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, req.Currency)
	if err != nil {
		_ = w.audit.Record(ctx, AuditRecord{
			ProviderType:   models.ProviderTypeShipping,
			ProviderID:     w.providerID,
			Environment:    w.environment,
			Operation:      "quote_rates",
			Status:         models.ProviderCallStatusFailed,
			RequestPayload: req,
			ErrorMessage:   errorMessage(err),
			Latency:        time.Since(start),
		})
		return nil, err
	}
	response, err := w.ShippingProvider.QuoteRates(callCtx, req)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypeShipping,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       "quote_rates",
		Status:          status,
		RequestPayload:  req,
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func (w shippingProviderWrapper) BuyLabel(ctx context.Context, req shippingservice.BuyLabelRequest) (shippingservice.ProviderShipment, error) {
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, req.Rate.Currency)
	if err != nil {
		_ = w.audit.Record(ctx, AuditRecord{
			ProviderType:   models.ProviderTypeShipping,
			ProviderID:     w.providerID,
			Environment:    w.environment,
			Operation:      "buy_label",
			CorrelationID:  req.CorrelationID,
			IdempotencyKey: req.IdempotencyKey,
			Status:         models.ProviderCallStatusFailed,
			RequestPayload: req,
			ErrorMessage:   errorMessage(err),
			Latency:        time.Since(start),
		})
		return shippingservice.ProviderShipment{}, err
	}
	response, err := w.ShippingProvider.BuyLabel(callCtx, req)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypeShipping,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       "buy_label",
		CorrelationID:   req.CorrelationID,
		IdempotencyKey:  req.IdempotencyKey,
		Status:          status,
		RequestPayload:  req,
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func (w shippingProviderWrapper) GetShipment(ctx context.Context, providerShipmentID string) (shippingservice.ProviderShipmentState, error) {
	lookupProvider, ok := w.ShippingProvider.(shippingservice.ShipmentLookupProvider)
	if !ok {
		return shippingservice.ProviderShipmentState{}, errors.New("shipping provider shipment lookup is unsupported")
	}
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, "")
	if err != nil {
		return shippingservice.ProviderShipmentState{}, err
	}
	response, err := lookupProvider.GetShipment(callCtx, providerShipmentID)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypeShipping,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       "get_shipment",
		Status:          status,
		RequestPayload:  map[string]string{"provider_shipment_id": providerShipmentID},
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func (w shippingProviderWrapper) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (shippingservice.TrackingWebhookEvent, error) {
	callCtx, err := w.prepareContext(ctx, "")
	if err != nil {
		return shippingservice.TrackingWebhookEvent{}, err
	}
	return w.ShippingProvider.VerifyWebhook(callCtx, headers, body)
}

func (w shippingProviderWrapper) ParseStoredWebhook(ctx context.Context, body []byte) (shippingservice.TrackingWebhookEvent, error) {
	parser, ok := w.ShippingProvider.(shippingservice.StoredWebhookParser)
	if !ok {
		return shippingservice.TrackingWebhookEvent{}, errors.New("shipping provider stored webhook parsing is unsupported")
	}
	callCtx, err := w.prepareContext(ctx, "")
	if err != nil {
		return shippingservice.TrackingWebhookEvent{}, err
	}
	return parser.ParseStoredWebhook(callCtx, body)
}

func (w shippingProviderWrapper) prepareContext(ctx context.Context, currency string) (context.Context, error) {
	credential, err := resolveProviderCredential(
		ctx,
		w.audit,
		w.credentials,
		models.ProviderTypeShipping,
		w.providerID,
		w.environment,
	)
	if err != nil {
		return ctx, err
	}
	if credential == nil {
		return ctx, nil
	}
	if currency != "" {
		if err := w.credentials.ValidateCurrency(currency, credential); err != nil {
			return ctx, err
		}
	}
	return providercontext.WithRuntimeData(ctx, providercontext.RuntimeData{
		Environment: w.environment,
		Credentials: credential.SecretData,
	}), nil
}

type taxRegistryWrapper struct {
	base        taxservice.ProviderRegistry
	environment string
	credentials *CredentialService
	audit       *AuditService
}

func (r taxRegistryWrapper) Provider(providerID string) (taxservice.TaxProvider, error) {
	provider, err := r.base.Provider(providerID)
	if err != nil {
		return nil, err
	}
	return taxProviderWrapper{
		providerID:  providerID,
		environment: r.environment,
		credentials: r.credentials,
		audit:       r.audit,
		TaxProvider: provider,
	}, nil
}

type taxProviderWrapper struct {
	providerID  string
	environment string
	credentials *CredentialService
	audit       *AuditService
	taxservice.TaxProvider
}

func (w taxProviderWrapper) QuoteTax(ctx context.Context, req taxservice.QuoteTaxRequest) (models.Money, error) {
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, "")
	if err != nil {
		return 0, err
	}
	response, err := w.TaxProvider.QuoteTax(callCtx, req)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypeTax,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       "quote_tax",
		Status:          status,
		RequestPayload:  req,
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func (w taxProviderWrapper) FinalizeTax(ctx context.Context, req taxservice.FinalizeTaxRequest) (taxservice.TaxFinalized, error) {
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, req.Currency)
	if err != nil {
		_ = w.audit.Record(ctx, AuditRecord{
			ProviderType:   models.ProviderTypeTax,
			ProviderID:     w.providerID,
			Environment:    w.environment,
			Operation:      "finalize_tax",
			Status:         models.ProviderCallStatusFailed,
			RequestPayload: req,
			ErrorMessage:   errorMessage(err),
			Latency:        time.Since(start),
		})
		return taxservice.TaxFinalized{}, err
	}
	response, err := w.TaxProvider.FinalizeTax(callCtx, req)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:    models.ProviderTypeTax,
		ProviderID:      w.providerID,
		Environment:     w.environment,
		Operation:       "finalize_tax",
		Status:          status,
		RequestPayload:  req,
		ResponsePayload: response,
		ErrorMessage:    errorMessage(err),
		Latency:         time.Since(start),
	})
	return response, err
}

func (w taxProviderWrapper) ExportReport(ctx context.Context, req taxservice.ExportReportRequest) (io.ReadCloser, error) {
	start := time.Now()
	callCtx, err := w.prepareContext(ctx, "")
	if err != nil {
		return nil, err
	}
	response, err := w.TaxProvider.ExportReport(callCtx, req)
	status := models.ProviderCallStatusSucceeded
	if err != nil {
		status = models.ProviderCallStatusFailed
	}
	_ = w.audit.Record(ctx, AuditRecord{
		ProviderType:   models.ProviderTypeTax,
		ProviderID:     w.providerID,
		Environment:    w.environment,
		Operation:      "export_report",
		Status:         status,
		RequestPayload: map[string]any{"line_count": len(req.Lines), "provider": req.Provider},
		ErrorMessage:   errorMessage(err),
		Latency:        time.Since(start),
	})
	return response, err
}

func (w taxProviderWrapper) prepareContext(ctx context.Context, currency string) (context.Context, error) {
	credential, err := resolveProviderCredential(
		ctx,
		w.audit,
		w.credentials,
		models.ProviderTypeTax,
		w.providerID,
		w.environment,
	)
	if err != nil {
		return ctx, err
	}
	if credential == nil {
		return ctx, nil
	}
	if currency != "" {
		if err := w.credentials.ValidateCurrency(currency, credential); err != nil {
			return ctx, err
		}
	}
	return providercontext.WithRuntimeData(ctx, providercontext.RuntimeData{
		Environment: w.environment,
		Credentials: credential.SecretData,
	}), nil
}

func resolveProviderCredential(
	ctx context.Context,
	audit *AuditService,
	credentials *CredentialService,
	providerType string,
	providerID string,
	environment string,
) (*ResolvedCredential, error) {
	if credentials == nil || !credentials.Enabled() {
		return nil, nil
	}
	db := dbcontext.GetDB(ctx)
	if db == nil {
		db = audit.db
	}
	return credentials.Resolve(dbcontext.OrBackground(ctx), db, providerType, providerID, environment)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func defaultIfEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
