package checkout

import (
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

type CleanupSummary struct {
	ExpiredSessions        int64
	DeletedIdempotencyKeys int64
}

func CleanupExpiredState(db *gorm.DB, now time.Time) (CleanupSummary, error) {
	summary := CleanupSummary{}
	err := db.Transaction(func(tx *gorm.DB) error {
		sessionResult := tx.Model(&models.CheckoutSession{}).
			Where("status = ? AND expires_at <= ?", models.CheckoutSessionStatusActive, now).
			Updates(map[string]any{
				"status":       models.CheckoutSessionStatusExpired,
				"last_seen_at": now,
			})
		if sessionResult.Error != nil {
			return sessionResult.Error
		}
		summary.ExpiredSessions = sessionResult.RowsAffected

		keyResult := tx.Where("expires_at <= ?", now).Delete(&models.IdempotencyKey{})
		if keyResult.Error != nil {
			return keyResult.Error
		}
		summary.DeletedIdempotencyKeys = keyResult.RowsAffected

		return nil
	})
	return summary, err
}
