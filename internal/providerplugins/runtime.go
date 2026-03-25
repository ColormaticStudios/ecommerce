package providerplugins

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/providercontext"
	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"
)

const (
	defaultExternalProviderTimeout = 5 * time.Second
	maxExternalProviderTimeout     = 30 * time.Second
)

type Manifest struct {
	ID           string                            `json:"id"`
	Type         checkoutplugins.ProviderType      `json:"type"`
	Name         string                            `json:"name"`
	Description  string                            `json:"description"`
	Status       string                            `json:"status"`
	Fields       []checkoutplugins.FieldDefinition `json:"fields"`
	States       []checkoutplugins.State           `json:"states"`
	Command      string                            `json:"command"`
	Args         []string                          `json:"args"`
	TimeoutMS    int                               `json:"timeout_ms"`
	Capabilities Capabilities                      `json:"capabilities"`
	baseDir      string
}

type Capabilities struct {
	Payment  PaymentCapabilities  `json:"payment"`
	Shipping ShippingCapabilities `json:"shipping"`
}

type PaymentCapabilities struct {
	LookupTransaction bool `json:"lookup_transaction"`
	ParseWebhook      bool `json:"parse_webhook"`
}

type ShippingCapabilities struct {
	LookupShipment bool `json:"lookup_shipment"`
	ParseWebhook   bool `json:"parse_webhook"`
}

type LoadedRegistries struct {
	LoadedCount       int
	PaymentProviders  paymentservice.ProviderRegistry
	ShippingProviders shippingservice.ProviderRegistry
	TaxProviders      taxservice.ProviderRegistry
}

type mergedPaymentRegistry struct {
	base      paymentservice.ProviderRegistry
	providers map[string]paymentservice.PaymentProvider
}

type mergedShippingRegistry struct {
	base      shippingservice.ProviderRegistry
	providers map[string]shippingservice.ShippingProvider
}

type mergedTaxRegistry struct {
	base      taxservice.ProviderRegistry
	providers map[string]taxservice.TaxProvider
}

type externalRunner struct {
	providerID   string
	providerType checkoutplugins.ProviderType
	command      string
	args         []string
	timeout      time.Duration
	workDir      string
}

type requestEnvelope struct {
	Action       string            `json:"action"`
	ProviderID   string            `json:"provider_id"`
	ProviderType string            `json:"provider_type"`
	Environment  string            `json:"environment,omitempty"`
	Credentials  map[string]string `json:"credentials,omitempty"`
	Input        any               `json:"input,omitempty"`
}

type paymentWebhookInput struct {
	Headers    map[string]string `json:"headers,omitempty"`
	BodyBase64 string            `json:"body_base64"`
}

type shippingWebhookInput struct {
	Headers    map[string]string `json:"headers,omitempty"`
	BodyBase64 string            `json:"body_base64"`
}

type paymentProvider struct {
	runner externalRunner
}

type paymentLookupProvider struct {
	paymentProvider
}

type paymentWebhookParser struct {
	paymentProvider
}

type paymentLookupWebhookParser struct {
	paymentProvider
}

type shippingProvider struct {
	runner externalRunner
}

type shippingLookupProvider struct {
	shippingProvider
}

type shippingWebhookParser struct {
	shippingProvider
}

type shippingLookupWebhookParser struct {
	shippingProvider
}

type taxProvider struct {
	runner externalRunner
}

func LoadRegistriesFromDir(
	dir string,
	basePayments paymentservice.ProviderRegistry,
	baseShippings shippingservice.ProviderRegistry,
	baseTaxes taxservice.ProviderRegistry,
) (LoadedRegistries, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return LoadedRegistries{
			PaymentProviders:  basePayments,
			ShippingProviders: baseShippings,
			TaxProviders:      baseTaxes,
		}, nil
	}

	manifests, err := ReadManifestsFromDir(trimmedDir)
	if err != nil {
		return LoadedRegistries{}, err
	}

	payments := make(map[string]paymentservice.PaymentProvider)
	shippings := make(map[string]shippingservice.ShippingProvider)
	taxes := make(map[string]taxservice.TaxProvider)

	for _, manifest := range manifests {
		runner := externalRunnerFromManifest(manifest)
		switch manifest.Type {
		case checkoutplugins.ProviderTypePayment:
			if err := ensurePaymentProviderAvailable(basePayments, payments, manifest.ID); err != nil {
				return LoadedRegistries{}, err
			}
			payments[manifest.ID] = buildPaymentProvider(runner, manifest.Capabilities.Payment)
		case checkoutplugins.ProviderTypeShipping:
			if err := ensureShippingProviderAvailable(baseShippings, shippings, manifest.ID); err != nil {
				return LoadedRegistries{}, err
			}
			shippings[manifest.ID] = buildShippingProvider(runner, manifest.Capabilities.Shipping)
		case checkoutplugins.ProviderTypeTax:
			if err := ensureTaxProviderAvailable(baseTaxes, taxes, manifest.ID); err != nil {
				return LoadedRegistries{}, err
			}
			taxes[manifest.ID] = taxProvider{runner: runner}
		default:
			return LoadedRegistries{}, fmt.Errorf("unsupported provider type %q", manifest.Type)
		}
	}

	return LoadedRegistries{
		LoadedCount:       len(manifests),
		PaymentProviders:  mergedPaymentRegistry{base: basePayments, providers: payments},
		ShippingProviders: mergedShippingRegistry{base: baseShippings, providers: shippings},
		TaxProviders:      mergedTaxRegistry{base: baseTaxes, providers: taxes},
	}, nil
}

func ReadManifestsFromDir(dir string) ([]Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read provider plugin manifest dir: %w", err)
	}

	manifests := make([]Manifest, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		manifest, err := readManifest(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read provider plugin manifest %s: %w", path, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse provider plugin manifest %s: %w", path, err)
	}

	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Status = strings.TrimSpace(manifest.Status)
	manifest.Command = strings.TrimSpace(manifest.Command)
	if manifest.Status == "" {
		manifest.Status = "available"
	}
	if manifest.ID == "" || manifest.Name == "" || manifest.Command == "" {
		return Manifest{}, fmt.Errorf("invalid provider plugin manifest %s: id, name, and command are required", path)
	}
	if manifest.Type != checkoutplugins.ProviderTypePayment &&
		manifest.Type != checkoutplugins.ProviderTypeShipping &&
		manifest.Type != checkoutplugins.ProviderTypeTax {
		return Manifest{}, fmt.Errorf("invalid provider plugin manifest %s: unsupported type %q", path, manifest.Type)
	}

	manifest.baseDir = filepath.Dir(path)
	if !filepath.IsAbs(manifest.Command) && strings.Contains(manifest.Command, string(os.PathSeparator)) {
		manifest.Command = filepath.Clean(filepath.Join(manifest.baseDir, manifest.Command))
	}
	for i, arg := range manifest.Args {
		trimmedArg := strings.TrimSpace(arg)
		if trimmedArg == "" {
			continue
		}
		if filepath.IsAbs(trimmedArg) {
			manifest.Args[i] = filepath.Clean(trimmedArg)
			continue
		}
		if strings.HasPrefix(trimmedArg, "-") {
			continue
		}
		if strings.Contains(trimmedArg, string(os.PathSeparator)) {
			manifest.Args[i] = filepath.Clean(filepath.Join(manifest.baseDir, trimmedArg))
		}
	}

	return manifest, nil
}

func externalRunnerFromManifest(manifest Manifest) externalRunner {
	timeout := defaultExternalProviderTimeout
	if manifest.TimeoutMS > 0 {
		timeout = time.Duration(manifest.TimeoutMS) * time.Millisecond
	}
	if timeout > maxExternalProviderTimeout {
		timeout = maxExternalProviderTimeout
	}
	return externalRunner{
		providerID:   manifest.ID,
		providerType: manifest.Type,
		command:      manifest.Command,
		args:         manifest.Args,
		timeout:      timeout,
		workDir:      manifest.baseDir,
	}
}

func ensurePaymentProviderAvailable(base paymentservice.ProviderRegistry, extras map[string]paymentservice.PaymentProvider, providerID string) error {
	if _, exists := extras[providerID]; exists {
		return fmt.Errorf("external payment provider %q is duplicated", providerID)
	}
	if base == nil {
		return nil
	}
	if _, err := base.Provider(providerID); err == nil {
		return fmt.Errorf("external payment provider %q conflicts with built-in provider", providerID)
	} else if !errors.Is(err, paymentservice.ErrUnknownPaymentProvider) {
		return err
	}
	return nil
}

func ensureShippingProviderAvailable(base shippingservice.ProviderRegistry, extras map[string]shippingservice.ShippingProvider, providerID string) error {
	if _, exists := extras[providerID]; exists {
		return fmt.Errorf("external shipping provider %q is duplicated", providerID)
	}
	if base == nil {
		return nil
	}
	if _, err := base.Provider(providerID); err == nil {
		return fmt.Errorf("external shipping provider %q conflicts with built-in provider", providerID)
	} else if !errors.Is(err, shippingservice.ErrUnknownShippingProvider) {
		return err
	}
	return nil
}

func ensureTaxProviderAvailable(base taxservice.ProviderRegistry, extras map[string]taxservice.TaxProvider, providerID string) error {
	if _, exists := extras[providerID]; exists {
		return fmt.Errorf("external tax provider %q is duplicated", providerID)
	}
	if base == nil {
		return nil
	}
	if _, err := base.Provider(providerID); err == nil {
		return fmt.Errorf("external tax provider %q conflicts with built-in provider", providerID)
	} else if !errors.Is(err, taxservice.ErrUnknownTaxProvider) {
		return err
	}
	return nil
}

func buildPaymentProvider(runner externalRunner, capabilities PaymentCapabilities) paymentservice.PaymentProvider {
	base := paymentProvider{runner: runner}
	switch {
	case capabilities.LookupTransaction && capabilities.ParseWebhook:
		return paymentLookupWebhookParser{paymentProvider: base}
	case capabilities.LookupTransaction:
		return paymentLookupProvider{paymentProvider: base}
	case capabilities.ParseWebhook:
		return paymentWebhookParser{paymentProvider: base}
	default:
		return base
	}
}

func buildShippingProvider(runner externalRunner, capabilities ShippingCapabilities) shippingservice.ShippingProvider {
	base := shippingProvider{runner: runner}
	switch {
	case capabilities.LookupShipment && capabilities.ParseWebhook:
		return shippingLookupWebhookParser{shippingProvider: base}
	case capabilities.LookupShipment:
		return shippingLookupProvider{shippingProvider: base}
	case capabilities.ParseWebhook:
		return shippingWebhookParser{shippingProvider: base}
	default:
		return base
	}
}

func (r mergedPaymentRegistry) Provider(providerID string) (paymentservice.PaymentProvider, error) {
	if provider, ok := r.providers[strings.TrimSpace(providerID)]; ok {
		return provider, nil
	}
	if r.base == nil {
		return nil, paymentservice.ErrUnknownPaymentProvider
	}
	return r.base.Provider(providerID)
}

func (r mergedShippingRegistry) Provider(providerID string) (shippingservice.ShippingProvider, error) {
	if provider, ok := r.providers[strings.TrimSpace(providerID)]; ok {
		return provider, nil
	}
	if r.base == nil {
		return nil, shippingservice.ErrUnknownShippingProvider
	}
	return r.base.Provider(providerID)
}

func (r mergedTaxRegistry) Provider(providerID string) (taxservice.TaxProvider, error) {
	if provider, ok := r.providers[strings.TrimSpace(providerID)]; ok {
		return provider, nil
	}
	if r.base == nil {
		return nil, taxservice.ErrUnknownTaxProvider
	}
	return r.base.Provider(providerID)
}

func (p paymentProvider) Authorize(ctx context.Context, req paymentservice.AuthorizeRequest) (paymentservice.ProviderOperationResult, error) {
	var response paymentservice.ProviderOperationResult
	err := p.runner.run(ctx, "payment.authorize", req, &response)
	return response, err
}

func (p paymentProvider) Capture(ctx context.Context, req paymentservice.CaptureRequest) (paymentservice.ProviderOperationResult, error) {
	var response paymentservice.ProviderOperationResult
	err := p.runner.run(ctx, "payment.capture", req, &response)
	return response, err
}

func (p paymentProvider) Void(ctx context.Context, req paymentservice.VoidRequest) (paymentservice.ProviderOperationResult, error) {
	var response paymentservice.ProviderOperationResult
	err := p.runner.run(ctx, "payment.void", req, &response)
	return response, err
}

func (p paymentProvider) Refund(ctx context.Context, req paymentservice.RefundRequest) (paymentservice.ProviderOperationResult, error) {
	var response paymentservice.ProviderOperationResult
	err := p.runner.run(ctx, "payment.refund", req, &response)
	return response, err
}

func (p paymentProvider) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (paymentservice.VerifiedWebhookEvent, error) {
	var response paymentservice.VerifiedWebhookEvent
	err := p.runner.run(ctx, "payment.verify_webhook", paymentWebhookInput{
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}, &response)
	if response.Provider == "" {
		response.Provider = p.runner.providerID
	}
	return response, err
}

func (p paymentLookupProvider) GetTransaction(ctx context.Context, providerTxnID string) (paymentservice.ProviderTransaction, error) {
	var response paymentservice.ProviderTransaction
	err := p.runner.run(ctx, "payment.get_transaction", map[string]string{"provider_txn_id": providerTxnID}, &response)
	return response, err
}

func (p paymentWebhookParser) ParseStoredWebhook(ctx context.Context, body []byte) (paymentservice.VerifiedWebhookEvent, error) {
	var response paymentservice.VerifiedWebhookEvent
	err := p.runner.run(ctx, "payment.parse_webhook", paymentWebhookInput{
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}, &response)
	if response.Provider == "" {
		response.Provider = p.runner.providerID
	}
	return response, err
}

func (p paymentLookupWebhookParser) GetTransaction(ctx context.Context, providerTxnID string) (paymentservice.ProviderTransaction, error) {
	return paymentLookupProvider{paymentProvider: p.paymentProvider}.GetTransaction(ctx, providerTxnID)
}

func (p paymentLookupWebhookParser) ParseStoredWebhook(ctx context.Context, body []byte) (paymentservice.VerifiedWebhookEvent, error) {
	return paymentWebhookParser{paymentProvider: p.paymentProvider}.ParseStoredWebhook(ctx, body)
}

func (p shippingProvider) QuoteRates(ctx context.Context, req shippingservice.QuoteRatesRequest) ([]shippingservice.QuotedRate, error) {
	var response []shippingservice.QuotedRate
	err := p.runner.run(ctx, "shipping.quote_rates", req, &response)
	return response, err
}

func (p shippingProvider) BuyLabel(ctx context.Context, req shippingservice.BuyLabelRequest) (shippingservice.ProviderShipment, error) {
	var response shippingservice.ProviderShipment
	err := p.runner.run(ctx, "shipping.buy_label", req, &response)
	return response, err
}

func (p shippingProvider) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (shippingservice.TrackingWebhookEvent, error) {
	var response shippingservice.TrackingWebhookEvent
	err := p.runner.run(ctx, "shipping.verify_webhook", shippingWebhookInput{
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}, &response)
	if response.Provider == "" {
		response.Provider = p.runner.providerID
	}
	return response, err
}

func (p shippingLookupProvider) GetShipment(ctx context.Context, providerShipmentID string) (shippingservice.ProviderShipmentState, error) {
	var response shippingservice.ProviderShipmentState
	err := p.runner.run(ctx, "shipping.get_shipment", map[string]string{"provider_shipment_id": providerShipmentID}, &response)
	return response, err
}

func (p shippingWebhookParser) ParseStoredWebhook(ctx context.Context, body []byte) (shippingservice.TrackingWebhookEvent, error) {
	var response shippingservice.TrackingWebhookEvent
	err := p.runner.run(ctx, "shipping.parse_webhook", shippingWebhookInput{
		BodyBase64: base64.StdEncoding.EncodeToString(body),
	}, &response)
	if response.Provider == "" {
		response.Provider = p.runner.providerID
	}
	return response, err
}

func (p shippingLookupWebhookParser) GetShipment(ctx context.Context, providerShipmentID string) (shippingservice.ProviderShipmentState, error) {
	return shippingLookupProvider{shippingProvider: p.shippingProvider}.GetShipment(ctx, providerShipmentID)
}

func (p shippingLookupWebhookParser) ParseStoredWebhook(ctx context.Context, body []byte) (shippingservice.TrackingWebhookEvent, error) {
	return shippingWebhookParser{shippingProvider: p.shippingProvider}.ParseStoredWebhook(ctx, body)
}

func (p taxProvider) QuoteTax(ctx context.Context, req taxservice.QuoteTaxRequest) (models.Money, error) {
	var response models.Money
	err := p.runner.run(ctx, "tax.quote_tax", req, &response)
	return response, err
}

func (p taxProvider) FinalizeTax(ctx context.Context, req taxservice.FinalizeTaxRequest) (taxservice.TaxFinalized, error) {
	var response taxservice.TaxFinalized
	err := p.runner.run(ctx, "tax.finalize_tax", req, &response)
	return response, err
}

func (p taxProvider) ExportReport(ctx context.Context, req taxservice.ExportReportRequest) (io.ReadCloser, error) {
	var response struct {
		Content string `json:"content"`
	}
	if err := p.runner.run(ctx, "tax.export_report", req, &response); err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(response.Content)), nil
}

func (r externalRunner) run(ctx context.Context, action string, input any, into any) error {
	runtimeData, _ := providercontext.RuntimeDataFromContext(ctx)
	payload, err := json.Marshal(requestEnvelope{
		Action:       action,
		ProviderID:   r.providerID,
		ProviderType: string(r.providerType),
		Environment:  runtimeData.Environment,
		Credentials:  runtimeData.Credentials,
		Input:        input,
	})
	if err != nil {
		return err
	}

	callCtx := ctx
	if callCtx == nil {
		callCtx = context.Background()
	}
	var cancel context.CancelFunc
	callCtx, cancel = context.WithTimeout(callCtx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, r.command, r.args...)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("external provider %s timed out", r.providerID)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("external provider %s failed: %s", r.providerID, message)
	}

	if err := json.Unmarshal(stdout.Bytes(), into); err != nil {
		return fmt.Errorf("external provider %s returned invalid json: %w", r.providerID, err)
	}
	return nil
}
