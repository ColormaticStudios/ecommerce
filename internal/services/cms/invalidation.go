package cms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

func StartInvalidationWorker(ctx context.Context, db *gorm.DB, webhookURL string, interval time.Duration, logger *log.Logger) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := deliverInvalidationEvents(ctx, db, webhookURL); err != nil && logger != nil {
				logger.Printf("[ERROR] CMS invalidation webhook delivery failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func deliverInvalidationEvents(ctx context.Context, db *gorm.DB, fallbackWebhookURL string) error {
	webhookURL, err := resolveInvalidationWebhookURL(db, fallbackWebhookURL)
	if err != nil {
		return err
	}
	if webhookURL == "" {
		return nil
	}
	var events []models.CMSInvalidationEvent
	if err := db.Where("status = ?", "pending").Order("created_at ASC, id ASC").Limit(50).Find(&events).Error; err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, event := range events {
		body, err := json.Marshal(event)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CMS-Event", event.Reason)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("webhook returned %s", response.Status)
		}
		now := time.Now().UTC()
		if err := db.Model(&models.CMSInvalidationEvent{}).Where("id = ? AND status = ?", event.ID, "pending").Updates(map[string]any{"status": "sent", "sent_at": now}).Error; err != nil {
			return err
		}
	}
	return nil
}

func resolveInvalidationWebhookURL(db *gorm.DB, fallbackWebhookURL string) (string, error) {
	var settings models.CMSSettings
	err := db.First(&settings, 1).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if strings.TrimSpace(settings.InvalidationWebhookURL) != "" {
		return strings.TrimSpace(settings.InvalidationWebhookURL), nil
	}
	return strings.TrimSpace(fallbackWebhookURL), nil
}
