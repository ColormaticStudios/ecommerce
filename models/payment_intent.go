package models

import "time"

const (
	PaymentIntentStatusRequiresAction    = "REQUIRES_ACTION"
	PaymentIntentStatusAuthorized        = "AUTHORIZED"
	PaymentIntentStatusPartiallyCaptured = "PARTIALLY_CAPTURED"
	PaymentIntentStatusCaptured          = "CAPTURED"
	PaymentIntentStatusVoided            = "VOIDED"
	PaymentIntentStatusRefunded          = "REFUNDED"
	PaymentIntentStatusFailed            = "FAILED"
)

func IsActivePaymentIntentStatus(status string) bool {
	switch status {
	case PaymentIntentStatusRequiresAction, PaymentIntentStatusAuthorized, PaymentIntentStatusPartiallyCaptured:
		return true
	default:
		return false
	}
}

type PaymentIntent struct {
	ID               uint `gorm:"primaryKey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	OrderID          uint                 `gorm:"not null;index"`
	SnapshotID       uint                 `gorm:"not null;index"`
	Provider         string               `gorm:"not null"`
	Status           string               `gorm:"not null;index"`
	AuthorizedAmount Money                `gorm:"type:numeric(12,2);not null"`
	CapturedAmount   Money                `gorm:"type:numeric(12,2);not null"`
	Currency         string               `gorm:"not null;size:3"`
	Version          int                  `gorm:"not null;default:1"`
	Transactions     []PaymentTransaction `gorm:"foreignKey:PaymentIntentID"`
}
