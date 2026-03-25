package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"ecommerce/models"
)

var ErrUnknownPaymentProvider = fmt.Errorf("unknown payment provider")
var ErrInvalidWebhookSignature = fmt.Errorf("invalid webhook signature")

type PaymentProvider interface {
	Authorize(ctx context.Context, req AuthorizeRequest) (ProviderOperationResult, error)
	Capture(ctx context.Context, req CaptureRequest) (ProviderOperationResult, error)
	Void(ctx context.Context, req VoidRequest) (ProviderOperationResult, error)
	Refund(ctx context.Context, req RefundRequest) (ProviderOperationResult, error)
	VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (VerifiedWebhookEvent, error)
}

type TransactionLookupProvider interface {
	GetTransaction(ctx context.Context, providerTxnID string) (ProviderTransaction, error)
}

type StoredWebhookParser interface {
	ParseStoredWebhook(ctx context.Context, body []byte) (VerifiedWebhookEvent, error)
}

type ProviderRegistry interface {
	Provider(providerID string) (PaymentProvider, error)
}

type AuthorizeRequest struct {
	OrderID              uint
	SnapshotID           uint
	Amount               models.Money
	Currency             string
	Provider             string
	IdempotencyKey       string
	CorrelationID        string
	PaymentMethodDisplay string
	PaymentData          map[string]string
}

type CaptureRequest struct {
	OrderID          uint
	IntentID         uint
	Amount           models.Money
	Currency         string
	Provider         string
	IdempotencyKey   string
	CorrelationID    string
	ProviderTxnIDRef string
}

type VoidRequest struct {
	OrderID          uint
	IntentID         uint
	Amount           models.Money
	Currency         string
	Provider         string
	IdempotencyKey   string
	CorrelationID    string
	ProviderTxnIDRef string
}

type RefundRequest struct {
	OrderID          uint
	IntentID         uint
	Amount           models.Money
	Currency         string
	Provider         string
	IdempotencyKey   string
	CorrelationID    string
	ProviderTxnIDRef string
}

type ProviderOperationResult struct {
	ProviderTxnID       string
	RawResponseRedacted string
}

type ProviderTransaction struct {
	ProviderTxnID string
	Operation     string
	Amount        models.Money
	Currency      string
	Status        string
}

type VerifiedWebhookEvent struct {
	Provider        string
	ProviderEventID string
	EventType       string
	ProviderTxnID   string
}

type DefaultProviderRegistry struct {
	providers map[string]PaymentProvider
}

func NewDefaultProviderRegistry() *DefaultProviderRegistry {
	dummy := dummyPaymentProvider{}
	return &DefaultProviderRegistry{
		providers: map[string]PaymentProvider{
			"dummy-card":   dummy,
			"dummy-wallet": dummy,
		},
	}
}

func (r *DefaultProviderRegistry) Provider(providerID string) (PaymentProvider, error) {
	if r == nil {
		return nil, ErrUnknownPaymentProvider
	}
	provider, ok := r.providers[strings.TrimSpace(providerID)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPaymentProvider, providerID)
	}
	return provider, nil
}

type dummyPaymentProvider struct{}

func (dummyPaymentProvider) Authorize(_ context.Context, req AuthorizeRequest) (ProviderOperationResult, error) {
	return ProviderOperationResult{
		ProviderTxnID: providerTxnID(req.Provider, "authorize", req.OrderID, req.SnapshotID, req.Currency, req.Amount, req.IdempotencyKey),
		RawResponseRedacted: marshalProviderResponse(map[string]any{
			"operation":              models.PaymentTransactionOperationAuthorize,
			"provider":               req.Provider,
			"order_id":               req.OrderID,
			"snapshot_id":            req.SnapshotID,
			"amount":                 req.Amount,
			"currency":               req.Currency,
			"payment_method_display": req.PaymentMethodDisplay,
			"correlation_id":         req.CorrelationID,
		}),
	}, nil
}

func (dummyPaymentProvider) Capture(_ context.Context, req CaptureRequest) (ProviderOperationResult, error) {
	return ProviderOperationResult{
		ProviderTxnID: providerTxnID(req.Provider, "capture", req.OrderID, req.IntentID, req.Currency, req.Amount, req.IdempotencyKey),
		RawResponseRedacted: marshalProviderResponse(map[string]any{
			"operation":           models.PaymentTransactionOperationCapture,
			"provider":            req.Provider,
			"order_id":            req.OrderID,
			"intent_id":           req.IntentID,
			"amount":              req.Amount,
			"currency":            req.Currency,
			"provider_txn_id_ref": req.ProviderTxnIDRef,
			"correlation_id":      req.CorrelationID,
		}),
	}, nil
}

func (dummyPaymentProvider) Void(_ context.Context, req VoidRequest) (ProviderOperationResult, error) {
	return ProviderOperationResult{
		ProviderTxnID: providerTxnID(req.Provider, "void", req.OrderID, req.IntentID, req.Currency, req.Amount, req.IdempotencyKey),
		RawResponseRedacted: marshalProviderResponse(map[string]any{
			"operation":           models.PaymentTransactionOperationVoid,
			"provider":            req.Provider,
			"order_id":            req.OrderID,
			"intent_id":           req.IntentID,
			"amount":              req.Amount,
			"currency":            req.Currency,
			"provider_txn_id_ref": req.ProviderTxnIDRef,
			"correlation_id":      req.CorrelationID,
		}),
	}, nil
}

func (dummyPaymentProvider) Refund(_ context.Context, req RefundRequest) (ProviderOperationResult, error) {
	return ProviderOperationResult{
		ProviderTxnID: providerTxnID(req.Provider, "refund", req.OrderID, req.IntentID, req.Currency, req.Amount, req.IdempotencyKey),
		RawResponseRedacted: marshalProviderResponse(map[string]any{
			"operation":           models.PaymentTransactionOperationRefund,
			"provider":            req.Provider,
			"order_id":            req.OrderID,
			"intent_id":           req.IntentID,
			"amount":              req.Amount,
			"currency":            req.Currency,
			"provider_txn_id_ref": req.ProviderTxnIDRef,
			"correlation_id":      req.CorrelationID,
		}),
	}, nil
}

func (dummyPaymentProvider) GetTransaction(_ context.Context, providerTxnID string) (ProviderTransaction, error) {
	parts := strings.Split(providerTxnID, "|")
	if len(parts) != 7 {
		return ProviderTransaction{}, fmt.Errorf("invalid provider transaction id")
	}

	operation := strings.ToUpper(strings.TrimSpace(parts[1]))
	status := models.PaymentTransactionStatusSucceeded
	if operation == "AUTHORIZE" {
		operation = models.PaymentTransactionOperationAuthorize
	}
	parsedAmount, err := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	if err != nil {
		return ProviderTransaction{}, err
	}
	return ProviderTransaction{
		ProviderTxnID: providerTxnID,
		Operation:     operation,
		Amount:        models.MoneyFromFloat(parsedAmount),
		Currency:      strings.ToUpper(strings.TrimSpace(parts[4])),
		Status:        status,
	}, nil
}

func (dummyPaymentProvider) VerifyWebhook(_ context.Context, headers map[string]string, body []byte) (VerifiedWebhookEvent, error) {
	if strings.TrimSpace(headers["X-Dummy-Signature"]) != "valid" {
		return VerifiedWebhookEvent{}, ErrInvalidWebhookSignature
	}
	return parseWebhookPayload(body)
}

func (dummyPaymentProvider) ParseStoredWebhook(_ context.Context, body []byte) (VerifiedWebhookEvent, error) {
	return parseWebhookPayload(body)
}

func parseWebhookPayload(body []byte) (VerifiedWebhookEvent, error) {
	var payload struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			ProviderTxnID string `json:"provider_txn_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return VerifiedWebhookEvent{}, err
	}
	if strings.TrimSpace(payload.ID) == "" || strings.TrimSpace(payload.Type) == "" {
		return VerifiedWebhookEvent{}, fmt.Errorf("webhook id and type are required")
	}
	providerID := strings.TrimSpace(payload.Data.ProviderTxnID)
	if idx := strings.Index(providerID, "|"); idx >= 0 {
		providerID = providerID[:idx]
	}
	return VerifiedWebhookEvent{
		Provider:        providerID,
		ProviderEventID: strings.TrimSpace(payload.ID),
		EventType:       strings.TrimSpace(payload.Type),
		ProviderTxnID:   strings.TrimSpace(payload.Data.ProviderTxnID),
	}, nil
}

func providerTxnID(providerID, operation string, entityID uint, secondaryID uint, currency string, amount models.Money, idempotencyKey string) string {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		key = "nokey"
	}
	key = strings.ReplaceAll(key, " ", "_")
	return fmt.Sprintf(
		"%s|%s|%d|%d|%s|%s|%s",
		providerID,
		strings.ToUpper(strings.TrimSpace(operation)),
		entityID,
		secondaryID,
		strings.ToUpper(strings.TrimSpace(currency)),
		amount.String(),
		key,
	)
}

func marshalProviderResponse(payload map[string]any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
