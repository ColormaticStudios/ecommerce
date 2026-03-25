package webhooks

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DefaultMaxAttempts    = 3
	DefaultInitialBackoff = 5 * time.Millisecond

	EventStatusPending    = "PENDING"
	EventStatusProcessed  = "PROCESSED"
	EventStatusDeadLetter = "DEAD_LETTER"
	EventStatusRejected   = "REJECTED"
)

type Service struct {
	DB                *gorm.DB
	Providers         paymentservice.ProviderRegistry
	ShippingProviders shippingservice.ProviderRegistry
	Logger            *log.Logger
	Queue             chan uint
	MaxAttempts       int
	InitialBackoff    time.Duration
}

func NewService(
	db *gorm.DB,
	providers paymentservice.ProviderRegistry,
	shippingProviders shippingservice.ProviderRegistry,
	logger *log.Logger,
) *Service {
	if providers == nil {
		providers = paymentservice.NewDefaultProviderRegistry()
	}
	if shippingProviders == nil {
		shippingProviders = shippingservice.NewDefaultProviderRegistry()
	}
	if logger == nil {
		logger = log.Default()
	}

	return &Service{
		DB:                db,
		Providers:         providers,
		ShippingProviders: shippingProviders,
		Logger:            logger,
		Queue:             make(chan uint, 100),
		MaxAttempts:       DefaultMaxAttempts,
		InitialBackoff:    DefaultInitialBackoff,
	}
}

func (s *Service) StartProcessor() {
	go func() {
		for eventID := range s.Queue {
			s.processWithRetry(context.Background(), eventID)
		}
	}()
}

func (s *Service) Enqueue(eventID uint) {
	if s == nil || s.Queue == nil || eventID == 0 {
		return
	}
	select {
	case s.Queue <- eventID:
	default:
		go s.processWithRetry(context.Background(), eventID)
	}
}

func (s *Service) ReceiveWebhook(
	ctx context.Context,
	provider string,
	headers map[string]string,
	body []byte,
) (models.WebhookEvent, bool, error) {
	if s == nil || s.DB == nil {
		return models.WebhookEvent{}, false, fmt.Errorf("webhook service is not configured")
	}

	provider = strings.TrimSpace(provider)

	var (
		providerEventID string
		eventType       string
	)
	resolvedProvider, err := s.Providers.Provider(provider)
	switch {
	case err == nil:
		verified, verifyErr := resolvedProvider.VerifyWebhook(ctx, headers, body)
		if verifyErr != nil {
			if recordErr := s.recordRejectedEvent(ctx, provider, body, verifyErr); recordErr != nil {
				return models.WebhookEvent{}, false, fmt.Errorf("%w: %v", verifyErr, recordErr)
			}
			return models.WebhookEvent{}, false, verifyErr
		}
		providerEventID = verified.ProviderEventID
		eventType = verified.EventType
	case errors.Is(err, paymentservice.ErrUnknownPaymentProvider):
		shippingProvider, shippingErr := s.ShippingProviders.Provider(provider)
		if shippingErr != nil {
			return models.WebhookEvent{}, false, err
		}
		verified, verifyErr := shippingProvider.VerifyWebhook(ctx, headers, body)
		if verifyErr != nil {
			if recordErr := s.recordRejectedEvent(ctx, provider, body, verifyErr); recordErr != nil {
				return models.WebhookEvent{}, false, fmt.Errorf("%w: %v", verifyErr, recordErr)
			}
			return models.WebhookEvent{}, false, verifyErr
		}
		providerEventID = verified.ProviderEventID
		eventType = verified.EventType
	default:
		return models.WebhookEvent{}, false, err
	}

	now := time.Now().UTC()
	event := models.WebhookEvent{
		Provider:        provider,
		ProviderEventID: providerEventID,
		EventType:       eventType,
		SignatureValid:  true,
		Payload:         string(body),
		ReceivedAt:      now,
	}
	if err := s.DB.Create(&event).Error; err != nil {
		if isUniqueConstraintError(err) {
			var existing models.WebhookEvent
			if lookupErr := s.DB.Where("provider = ? AND provider_event_id = ?", provider, providerEventID).First(&existing).Error; lookupErr != nil {
				return models.WebhookEvent{}, false, lookupErr
			}
			if existing.ProcessedAt == nil {
				if EventStatus(&existing, s.MaxAttempts) == EventStatusDeadLetter {
					if resetErr := s.DB.Model(&models.WebhookEvent{}).
						Where("id = ? AND processed_at IS NULL", existing.ID).
						Updates(map[string]any{
							"attempt_count": 0,
							"last_error":    "",
						}).Error; resetErr != nil {
						return models.WebhookEvent{}, false, resetErr
					}
					existing.AttemptCount = 0
					existing.LastError = ""
				}
				s.Enqueue(existing.ID)
			}
			return existing, true, nil
		}
		return models.WebhookEvent{}, false, err
	}

	s.Enqueue(event.ID)
	return event, false, nil
}

func (s *Service) recordRejectedEvent(ctx context.Context, provider string, body []byte, err error) error {
	if !isInvalidSignatureError(err) {
		return nil
	}

	now := time.Now().UTC()
	event := models.WebhookEvent{
		Provider:        provider,
		ProviderEventID: rejectedDeliveryEventID(body, now),
		EventType:       "signature.invalid",
		SignatureValid:  false,
		Payload:         string(body),
		ReceivedAt:      now,
		AttemptCount:    1,
		LastError:       err.Error(),
	}
	return s.DB.WithContext(ctx).Create(&event).Error
}

func (s *Service) processWithRetry(ctx context.Context, eventID uint) {
	backoff := s.InitialBackoff
	if backoff <= 0 {
		backoff = DefaultInitialBackoff
	}

	for attempt := 0; attempt < max(1, s.MaxAttempts); attempt++ {
		err := s.ProcessStoredEvent(ctx, eventID)
		if err == nil {
			return
		}
		s.Logger.Printf("webhook_event_process result=retry event_id=%d attempt=%d reason=%q", eventID, attempt+1, err.Error())
		time.Sleep(backoff)
		backoff *= 2
	}
}

func (s *Service) ProcessStoredEvent(ctx context.Context, eventID uint) error {
	var processErr error
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var event models.WebhookEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, eventID).Error; err != nil {
			return err
		}
		if event.ProcessedAt != nil || EventStatus(&event, s.MaxAttempts) == EventStatusDeadLetter {
			return nil
		}

		event.AttemptCount++
		if err := tx.Model(&models.WebhookEvent{}).
			Where("id = ?", event.ID).
			Update("attempt_count", event.AttemptCount).Error; err != nil {
			return err
		}

		switch {
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.EventType)), "payment."):
			parsed, err := parseStoredPaymentEventPayload(event.Provider, event.Payload)
			if err != nil {
				processErr = err
				return markWebhookFailure(tx, &event, err)
			}
			if _, _, _, err := paymentservice.ApplyWebhookPaymentEvent(tx, parsed, fmt.Sprintf("webhook:%d", event.ID)); err != nil {
				processErr = err
				return markWebhookFailure(tx, &event, err)
			}
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.EventType)), "tracking."),
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.EventType)), "shipment."):
			parsed, err := shippingservice.ParseStoredWebhookEvent(event.Provider, event.Payload)
			if err != nil {
				processErr = err
				return markWebhookFailure(tx, &event, err)
			}
			if _, _, err := shippingservice.ApplyTrackingEvent(tx, parsed, fmt.Sprintf("webhook:%d", event.ID)); err != nil {
				processErr = err
				return markWebhookFailure(tx, &event, err)
			}
		default:
			processErr = fmt.Errorf("unsupported webhook event type: %s", event.EventType)
			return markWebhookFailure(tx, &event, processErr)
		}

		now := time.Now().UTC()
		return tx.Model(&models.WebhookEvent{}).
			Where("id = ?", event.ID).
			Updates(map[string]any{
				"processed_at": now,
				"last_error":   "",
			}).Error
	})
	if err != nil {
		return err
	}
	return processErr
}

func ListEvents(
	db *gorm.DB,
	provider string,
	status string,
	page int,
	limit int,
	maxAttempts int,
) ([]models.WebhookEvent, int64, error) {
	query := db.Model(&models.WebhookEvent{})
	if strings.TrimSpace(provider) != "" {
		query = query.Where("provider = ?", strings.TrimSpace(provider))
	}

	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	switch normalizedStatus {
	case EventStatusProcessed:
		query = query.Where("signature_valid = ? AND processed_at IS NOT NULL", true)
	case EventStatusPending:
		query = query.Where("signature_valid = ? AND processed_at IS NULL AND attempt_count < ?", true, maxAttempts)
	case EventStatusDeadLetter:
		query = query.Where("signature_valid = ? AND processed_at IS NULL AND attempt_count >= ?", true, maxAttempts)
	case EventStatusRejected:
		query = query.Where("signature_valid = ?", false)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var events []models.WebhookEvent
	if err := query.Order("received_at DESC, id DESC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func EventStatus(event *models.WebhookEvent, maxAttempts int) string {
	if event == nil {
		return EventStatusPending
	}
	if !event.SignatureValid {
		return EventStatusRejected
	}
	if event.ProcessedAt != nil {
		return EventStatusProcessed
	}
	if event.AttemptCount >= max(1, maxAttempts) {
		return EventStatusDeadLetter
	}
	return EventStatusPending
}

func isInvalidSignatureError(err error) bool {
	return errors.Is(err, paymentservice.ErrInvalidWebhookSignature) ||
		errors.Is(err, shippingservice.ErrInvalidShippingWebhookSignature)
}

func parseStoredPaymentEventPayload(provider string, payload string) (paymentservice.VerifiedWebhookEvent, error) {
	switch strings.TrimSpace(provider) {
	case "dummy-card", "dummy-wallet":
		var body struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				ProviderTxnID string `json:"provider_txn_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return paymentservice.VerifiedWebhookEvent{}, err
		}
		return paymentservice.VerifiedWebhookEvent{
			Provider:        provider,
			ProviderEventID: strings.TrimSpace(body.ID),
			EventType:       strings.TrimSpace(body.Type),
			ProviderTxnID:   strings.TrimSpace(body.Data.ProviderTxnID),
		}, nil
	default:
		return paymentservice.VerifiedWebhookEvent{}, fmt.Errorf("unsupported webhook provider: %s", provider)
	}
}

func markWebhookFailure(tx *gorm.DB, event *models.WebhookEvent, err error) error {
	if event == nil {
		return nil
	}
	if updateErr := tx.Model(&models.WebhookEvent{}).
		Where("id = ?", event.ID).
		Update("last_error", err.Error()).Error; updateErr != nil {
		return updateErr
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key value")
}

func rejectedDeliveryEventID(body []byte, now time.Time) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("signature.invalid.%d.%x", now.UnixNano(), sum[:4])
}
