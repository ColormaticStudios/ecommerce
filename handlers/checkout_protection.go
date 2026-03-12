package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	checkoutRateLimitedCode     = "checkout_rate_limited"
	idempotencyConflictCode     = "idempotency_key_conflict"
	idempotencyInProgressCode   = "idempotency_in_progress"
	defaultIdempotencyRetention = 24 * time.Hour
)

var checkoutSubmissionRateLimit = struct {
	Limit  int
	Window time.Duration
}{
	Limit:  6,
	Window: time.Minute,
}

var checkoutSubmissionLimiter = newCheckoutSubmissionLimiter()

type checkoutSubmissionLimiterStore struct {
	mu      sync.Mutex
	history map[string][]time.Time
}

func newCheckoutSubmissionLimiter() *checkoutSubmissionLimiterStore {
	return &checkoutSubmissionLimiterStore{
		history: make(map[string][]time.Time),
	}
}

func (l *checkoutSubmissionLimiterStore) allow(key string, now time.Time, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := l.history[key][:0]
	cutoff := now.Add(-window)
	for _, ts := range l.history[key] {
		if ts.After(cutoff) {
			entries = append(entries, ts)
		}
	}
	if len(entries) >= limit {
		l.history[key] = entries
		return false
	}
	l.history[key] = append(entries, now)
	return true
}

func (l *checkoutSubmissionLimiterStore) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.history = make(map[string][]time.Time)
}

func enforceCheckoutSubmissionRateLimit(
	c *gin.Context,
	session *models.CheckoutSession,
	scope string,
) bool {
	if checkoutSubmissionRateLimit.Limit <= 0 || checkoutSubmissionRateLimit.Window <= 0 {
		return true
	}

	key := strings.Join([]string{
		scope,
		c.ClientIP(),
		session.PublicToken,
	}, "|")
	if checkoutSubmissionLimiter.allow(
		key,
		time.Now().UTC(),
		checkoutSubmissionRateLimit.Limit,
		checkoutSubmissionRateLimit.Window,
	) {
		return true
	}

	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": "Too many checkout attempts. Please wait and try again.",
		"code":  checkoutRateLimitedCode,
	})
	return false
}

func beginCheckoutIdempotency(
	db *gorm.DB,
	c *gin.Context,
	session *models.CheckoutSession,
	scope string,
	request any,
) (*models.IdempotencyKey, bool, error) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		return nil, false, nil
	}

	requestHash, err := hashCheckoutIdempotencyRequest(request)
	if err != nil {
		return nil, false, err
	}

	existing, found, err := lookupCheckoutIdempotencyRecord(db, session.ID, scope, key)
	if err != nil {
		return nil, false, err
	}
	if found {
		return nil, handleExistingCheckoutIdempotency(c, existing, requestHash), nil
	}

	record := models.IdempotencyKey{
		Scope:             scope,
		Key:               key,
		RequestHash:       requestHash,
		CheckoutSessionID: session.ID,
		ExpiresAt:         time.Now().UTC().Add(defaultIdempotencyRetention),
	}
	if err := db.Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return beginCheckoutIdempotency(db, c, session, scope, request)
		}
		return nil, false, err
	}
	return &record, false, nil
}

func replayCheckoutIdempotency(
	db *gorm.DB,
	c *gin.Context,
	session *models.CheckoutSession,
	scope string,
	request any,
) (bool, error) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		return false, nil
	}

	requestHash, err := hashCheckoutIdempotencyRequest(request)
	if err != nil {
		return false, err
	}

	existing, found, err := lookupCheckoutIdempotencyRecord(db, session.ID, scope, key)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	return handleExistingCheckoutIdempotency(c, existing, requestHash), nil
}

func hashCheckoutIdempotencyRequest(request any) (string, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func lookupCheckoutIdempotencyRecord(
	db *gorm.DB,
	sessionID uint,
	scope, key string,
) (*models.IdempotencyKey, bool, error) {
	var existing models.IdempotencyKey
	err := db.Where("scope = ? AND key = ? AND checkout_session_id = ?", scope, key, sessionID).
		First(&existing).Error
	if err == nil {
		return &existing, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

func handleExistingCheckoutIdempotency(c *gin.Context, existing *models.IdempotencyKey, requestHash string) bool {
	if existing.RequestHash != requestHash {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Idempotency key has already been used for a different request.",
			"code":  idempotencyConflictCode,
		})
		return true
	}
	if existing.ResponseCode != 0 && strings.TrimSpace(existing.ResponseBody) != "" {
		c.Data(existing.ResponseCode, "application/json", []byte(existing.ResponseBody))
		return true
	}
	c.JSON(http.StatusConflict, gin.H{
		"error": "Idempotency key is already processing.",
		"code":  idempotencyInProgressCode,
	})
	return true
}

func writeCheckoutJSON(
	db *gorm.DB,
	c *gin.Context,
	record *models.IdempotencyKey,
	status int,
	payload any,
) {
	raw, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode response"})
		return
	}

	if record != nil && status < 500 {
		if err := db.Model(&models.IdempotencyKey{}).
			Where("id = ?", record.ID).
			Updates(map[string]any{
				"response_code": status,
				"response_body": string(raw),
			}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist idempotent response"})
			return
		}
	}
	if record != nil && status >= 500 {
		if err := db.Delete(&models.IdempotencyKey{}, record.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to release idempotency key"})
			return
		}
	}

	c.Data(status, "application/json", raw)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key value")
}
