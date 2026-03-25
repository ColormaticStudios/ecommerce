package models

import "time"

type IdempotencyKey struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Scope             string    `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	Key               string    `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	RequestHash       string    `gorm:"not null"`
	Status            string    `gorm:"not null;default:processing"`
	ResponseCode      int       `gorm:"not null;default:0"`
	ResponseBody      string    `gorm:"type:text;not null;default:''"`
	CorrelationID     string    `gorm:"not null;default:'';index"`
	CheckoutSessionID uint      `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	PaymentIntentID   *uint     `gorm:"index"`
	ExpiresAt         time.Time `gorm:"not null;index"`
}
