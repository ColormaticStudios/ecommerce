package webhooks

import (
	"context"
	"errors"

	"ecommerce/models"
	"gorm.io/gorm"
)

func (s *Service) ListEvents(ctx context.Context, provider, status string, page, limit int) ([]models.WebhookEvent, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, errors.New("webhook service is not configured")
	}
	return ListEvents(s.DB.WithContext(ctx), provider, status, page, limit, s.MaxAttempts)
}

// Replay requeues a verified, unprocessed event without re-verifying a
// reconstructed payload. Stored-webhook parsing remains the provider boundary.
func (s *Service) Replay(ctx context.Context, eventID uint) (models.WebhookEvent, error) {
	if s == nil || s.DB == nil {
		return models.WebhookEvent{}, errors.New("webhook service is not configured")
	}
	var event models.WebhookEvent
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&event, eventID).Error; err != nil {
			return err
		}
		if !event.SignatureValid {
			return errors.New("rejected webhook events cannot be replayed")
		}
		if event.ProcessedAt != nil {
			return nil
		}
		event.AttemptCount, event.LastError = 0, ""
		return tx.Model(&event).Updates(map[string]any{"attempt_count": 0, "last_error": ""}).Error
	})
	if err != nil {
		return models.WebhookEvent{}, err
	}
	s.Enqueue(event.ID)
	return event, nil
}
