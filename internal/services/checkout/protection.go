package checkout

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

const idempotencyRetention = 24 * time.Hour

var (
	ErrIdempotencyConflict   = errors.New("idempotency key has already been used for a different request")
	ErrIdempotencyInProgress = errors.New("idempotency key is already processing")
)

type IdempotencyResult struct {
	Record *models.IdempotencyKey
	Replay bool
}

func (s *Service) BeginIdempotency(ctx context.Context, sessionID uint, scope, key string, request any, correlationID string) (IdempotencyResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return IdempotencyResult{}, nil
	}
	hash, err := idempotencyHash(request)
	if err != nil {
		return IdempotencyResult{}, err
	}
	var existing models.IdempotencyKey
	err = s.db.WithContext(ctx).Where("scope = ? AND key = ? AND checkout_session_id = ?", scope, key, sessionID).First(&existing).Error
	if err == nil {
		if existing.RequestHash != hash {
			return IdempotencyResult{}, ErrIdempotencyConflict
		}
		if existing.ResponseCode == 0 || strings.TrimSpace(existing.ResponseBody) == "" {
			return IdempotencyResult{}, ErrIdempotencyInProgress
		}
		return IdempotencyResult{Record: &existing, Replay: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return IdempotencyResult{}, err
	}
	record := models.IdempotencyKey{Scope: scope, Key: key, RequestHash: hash, Status: "processing", CorrelationID: correlationID, CheckoutSessionID: sessionID, ExpiresAt: time.Now().UTC().Add(idempotencyRetention)}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueError(err) {
			return s.BeginIdempotency(ctx, sessionID, scope, key, request, correlationID)
		}
		return IdempotencyResult{}, err
	}
	return IdempotencyResult{Record: &record}, nil
}

func (s *Service) CompleteIdempotency(ctx context.Context, record *models.IdempotencyKey, status int, response any) error {
	if record == nil {
		return nil
	}
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&models.IdempotencyKey{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status": "completed", "response_code": status, "response_body": string(body),
	}).Error
}

func (s *Service) ReleaseIdempotency(ctx context.Context, record *models.IdempotencyKey) error {
	if record == nil {
		return nil
	}
	return s.db.WithContext(ctx).Delete(&models.IdempotencyKey{}, record.ID).Error
}

func idempotencyHash(request any) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func isUniqueError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key value")
}
