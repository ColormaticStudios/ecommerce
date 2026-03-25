package shipping

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ecommerce/models"
)

var ErrUnknownShippingProvider = fmt.Errorf("unknown shipping provider")
var ErrInvalidShippingWebhookSignature = fmt.Errorf("invalid shipping webhook signature")

type ShippingProvider interface {
	QuoteRates(ctx context.Context, req QuoteRatesRequest) ([]QuotedRate, error)
	BuyLabel(ctx context.Context, req BuyLabelRequest) (ProviderShipment, error)
	VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (TrackingWebhookEvent, error)
}

type ShipmentLookupProvider interface {
	GetShipment(ctx context.Context, providerShipmentID string) (ProviderShipmentState, error)
}

type StoredWebhookParser interface {
	ParseStoredWebhook(ctx context.Context, body []byte) (TrackingWebhookEvent, error)
}

type ProviderRegistry interface {
	Provider(providerID string) (ShippingProvider, error)
}

type QuoteRatesRequest struct {
	OrderID               uint
	SnapshotID            uint
	Currency              string
	ShippingAddressPretty string
	ShippingAmount        models.Money
	ShippingData          map[string]string
}

type QuotedRate struct {
	ProviderRateID string
	ServiceCode    string
	ServiceName    string
	Amount         models.Money
	Currency       string
	ExpiresAt      *time.Time
}

type PackageInput struct {
	Reference   string
	WeightGrams int
	LengthCM    int
	WidthCM     int
	HeightCM    int
}

type BuyLabelRequest struct {
	OrderID               uint
	SnapshotID            uint
	Provider              string
	Rate                  models.ShipmentRate
	ShippingAddressPretty string
	Package               PackageInput
	IdempotencyKey        string
	CorrelationID         string
}

type ProviderShipment struct {
	ProviderShipmentID string
	TrackingNumber     string
	TrackingURL        string
	LabelURL           string
	ServiceCode        string
	ServiceName        string
}

type ProviderShipmentState struct {
	ProviderShipmentID string
	TrackingNumber     string
	Status             string
	ServiceCode        string
	ServiceName        string
}

type TrackingWebhookEvent struct {
	Provider           string
	ProviderEventID    string
	EventType          string
	ProviderShipmentID string
	TrackingNumber     string
	Status             string
	Location           string
	Description        string
	OccurredAt         time.Time
	RawPayload         string
}

type DefaultProviderRegistry struct {
	providers map[string]ShippingProvider
}

func NewDefaultProviderRegistry() *DefaultProviderRegistry {
	ground := dummyGroundProvider{}
	pickup := dummyPickupProvider{}
	return &DefaultProviderRegistry{
		providers: map[string]ShippingProvider{
			"dummy-ground": ground,
			"dummy-pickup": pickup,
		},
	}
}

func (r *DefaultProviderRegistry) Provider(providerID string) (ShippingProvider, error) {
	if r == nil {
		return nil, ErrUnknownShippingProvider
	}
	provider, ok := r.providers[strings.TrimSpace(providerID)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownShippingProvider, providerID)
	}
	return provider, nil
}

type dummyGroundProvider struct{}

func (dummyGroundProvider) QuoteRates(_ context.Context, req QuoteRatesRequest) ([]QuotedRate, error) {
	country := strings.ToUpper(strings.TrimSpace(req.ShippingData["country"]))
	now := time.Now().UTC()
	standard := models.MoneyFromFloat(5.99)
	express := models.MoneyFromFloat(15.99)
	if country != "" && country != "US" {
		standard += models.MoneyFromFloat(12.50)
		express += models.MoneyFromFloat(12.50)
	}
	expiresAt := now.Add(15 * time.Minute)
	return []QuotedRate{
		{
			ProviderRateID: fmt.Sprintf("dummy-ground-standard-%d", req.SnapshotID),
			ServiceCode:    "standard",
			ServiceName:    "Standard",
			Amount:         standard,
			Currency:       req.Currency,
			ExpiresAt:      &expiresAt,
		},
		{
			ProviderRateID: fmt.Sprintf("dummy-ground-express-%d", req.SnapshotID),
			ServiceCode:    "express",
			ServiceName:    "Express",
			Amount:         express,
			Currency:       req.Currency,
			ExpiresAt:      &expiresAt,
		},
	}, nil
}

func (dummyGroundProvider) BuyLabel(_ context.Context, req BuyLabelRequest) (ProviderShipment, error) {
	suffix := sanitizeKey(req.IdempotencyKey)
	if suffix == "" {
		suffix = "nolabelkey"
	}
	return ProviderShipment{
		ProviderShipmentID: fmt.Sprintf("ship-ground-%d-%d-%s", req.OrderID, req.Rate.ID, suffix),
		TrackingNumber:     fmt.Sprintf("DUM%06d", req.Rate.ID),
		TrackingURL:        fmt.Sprintf("https://tracking.example.test/%d", req.Rate.ID),
		LabelURL:           fmt.Sprintf("https://labels.example.test/%d.pdf", req.Rate.ID),
		ServiceCode:        req.Rate.ServiceCode,
		ServiceName:        req.Rate.ServiceName,
	}, nil
}

func (dummyGroundProvider) VerifyWebhook(_ context.Context, headers map[string]string, body []byte) (TrackingWebhookEvent, error) {
	if strings.TrimSpace(headers["X-Dummy-Signature"]) != "valid" {
		return TrackingWebhookEvent{}, ErrInvalidShippingWebhookSignature
	}
	return parseTrackingWebhookEvent("dummy-ground", body)
}

func (dummyGroundProvider) ParseStoredWebhook(_ context.Context, body []byte) (TrackingWebhookEvent, error) {
	return parseTrackingWebhookEvent("dummy-ground", body)
}

func (dummyGroundProvider) GetShipment(_ context.Context, providerShipmentID string) (ProviderShipmentState, error) {
	return dummyShipmentState(providerShipmentID), nil
}

type dummyPickupProvider struct{}

func (dummyPickupProvider) QuoteRates(_ context.Context, req QuoteRatesRequest) ([]QuotedRate, error) {
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	return []QuotedRate{
		{
			ProviderRateID: fmt.Sprintf("dummy-pickup-pickup-%d", req.SnapshotID),
			ServiceCode:    "pickup",
			ServiceName:    "In-Store Pickup",
			Amount:         0,
			Currency:       req.Currency,
			ExpiresAt:      &expiresAt,
		},
	}, nil
}

func (dummyPickupProvider) BuyLabel(_ context.Context, req BuyLabelRequest) (ProviderShipment, error) {
	suffix := sanitizeKey(req.IdempotencyKey)
	if suffix == "" {
		suffix = "nopickupkey"
	}
	return ProviderShipment{
		ProviderShipmentID: fmt.Sprintf("ship-pickup-%d-%d-%s", req.OrderID, req.Rate.ID, suffix),
		TrackingNumber:     fmt.Sprintf("PICKUP-%06d", req.Rate.ID),
		TrackingURL:        fmt.Sprintf("https://pickup.example.test/%d", req.Rate.ID),
		LabelURL:           fmt.Sprintf("https://pickup.example.test/%d.txt", req.Rate.ID),
		ServiceCode:        req.Rate.ServiceCode,
		ServiceName:        req.Rate.ServiceName,
	}, nil
}

func (dummyPickupProvider) VerifyWebhook(_ context.Context, headers map[string]string, body []byte) (TrackingWebhookEvent, error) {
	if strings.TrimSpace(headers["X-Dummy-Signature"]) != "valid" {
		return TrackingWebhookEvent{}, ErrInvalidShippingWebhookSignature
	}
	return parseTrackingWebhookEvent("dummy-pickup", body)
}

func (dummyPickupProvider) ParseStoredWebhook(_ context.Context, body []byte) (TrackingWebhookEvent, error) {
	return parseTrackingWebhookEvent("dummy-pickup", body)
}

func (dummyPickupProvider) GetShipment(_ context.Context, providerShipmentID string) (ProviderShipmentState, error) {
	return dummyShipmentState(providerShipmentID), nil
}

func ParseStoredWebhookEvent(provider string, payload string) (TrackingWebhookEvent, error) {
	return parseTrackingWebhookEvent(strings.TrimSpace(provider), []byte(payload))
}

func parseTrackingWebhookEvent(provider string, body []byte) (TrackingWebhookEvent, error) {
	var payload struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			ProviderShipmentID string `json:"provider_shipment_id"`
			TrackingNumber     string `json:"tracking_number"`
			Status             string `json:"status"`
			Location           string `json:"location"`
			Description        string `json:"description"`
			OccurredAt         string `json:"occurred_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TrackingWebhookEvent{}, err
	}
	if strings.TrimSpace(payload.ID) == "" {
		return TrackingWebhookEvent{}, fmt.Errorf("webhook id is required")
	}
	if strings.TrimSpace(payload.Data.ProviderShipmentID) == "" {
		return TrackingWebhookEvent{}, fmt.Errorf("provider shipment id is required")
	}
	status := strings.ToUpper(strings.TrimSpace(payload.Data.Status))
	if status == "" {
		status = trackingStatusFromEventType(payload.Type)
	}
	if status == "" {
		return TrackingWebhookEvent{}, fmt.Errorf("tracking status is required")
	}
	occurredAt := time.Now().UTC()
	if rawOccurredAt := strings.TrimSpace(payload.Data.OccurredAt); rawOccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, rawOccurredAt)
		if err != nil {
			return TrackingWebhookEvent{}, err
		}
		occurredAt = parsed.UTC()
	}
	return TrackingWebhookEvent{
		Provider:           provider,
		ProviderEventID:    strings.TrimSpace(payload.ID),
		EventType:          strings.TrimSpace(payload.Type),
		ProviderShipmentID: strings.TrimSpace(payload.Data.ProviderShipmentID),
		TrackingNumber:     strings.TrimSpace(payload.Data.TrackingNumber),
		Status:             status,
		Location:           strings.TrimSpace(payload.Data.Location),
		Description:        strings.TrimSpace(payload.Data.Description),
		OccurredAt:         occurredAt,
		RawPayload:         string(body),
	}, nil
}

func trackingStatusFromEventType(eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "tracking.in_transit", "shipment.in_transit":
		return models.ShipmentStatusInTransit
	case "tracking.delivered", "shipment.delivered":
		return models.ShipmentStatusDelivered
	case "tracking.exception", "shipment.exception":
		return models.ShipmentStatusException
	default:
		return ""
	}
}

func sanitizeKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func dummyShipmentState(providerShipmentID string) ProviderShipmentState {
	status := models.ShipmentStatusLabelPurchased
	if strings.Contains(strings.ToLower(providerShipmentID), "delivered") {
		status = models.ShipmentStatusDelivered
	}
	serviceCode := "standard"
	serviceName := "Standard"
	trackingNumber := sanitizeKey(providerShipmentID)
	parts := strings.Split(providerShipmentID, "-")
	if len(parts) >= 4 {
		rateID := strings.TrimSpace(parts[3])
		if _, err := strconv.ParseUint(rateID, 10, 64); err == nil {
			trackingNumber = fmt.Sprintf("DUM%06s", rateID)
		}
	}
	if strings.Contains(strings.ToLower(providerShipmentID), "pickup") {
		serviceCode = "pickup"
		serviceName = "In-Store Pickup"
		if len(parts) >= 4 {
			rateID := strings.TrimSpace(parts[3])
			if _, err := strconv.ParseUint(rateID, 10, 64); err == nil {
				trackingNumber = fmt.Sprintf("PICKUP-%06s", rateID)
			}
		}
	}
	return ProviderShipmentState{
		ProviderShipmentID: providerShipmentID,
		TrackingNumber:     trackingNumber,
		Status:             status,
		ServiceCode:        serviceCode,
		ServiceName:        serviceName,
	}
}
