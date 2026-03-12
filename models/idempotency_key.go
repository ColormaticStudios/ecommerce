package models

import "time"

type IdempotencyKey struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Scope             string    `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	Key               string    `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	RequestHash       string    `gorm:"not null"`
	ResponseCode      int       `gorm:"not null;default:0"`
	ResponseBody      string    `gorm:"type:text;not null;default:''"`
	CheckoutSessionID uint      `gorm:"not null;index:idx_idempotency_scope_session_key,unique"`
	ExpiresAt         time.Time `gorm:"not null;index"`
}
