package models

import "time"

const (
	CheckoutSessionStatusActive    = "ACTIVE"
	CheckoutSessionStatusConverted = "CONVERTED"
	CheckoutSessionStatusExpired   = "EXPIRED"
)

type CheckoutSession struct {
	BaseModel
	PublicToken string `json:"-" gorm:"uniqueIndex;not null"`
	UserID      *uint  `json:"user_id"`
	User        *User  `json:"-" gorm:"foreignKey:UserID"`
	GuestEmail  *string
	Status      string    `json:"status" gorm:"not null;default:ACTIVE"`
	ExpiresAt   time.Time `json:"expires_at" gorm:"not null"`
	LastSeenAt  time.Time `json:"last_seen_at" gorm:"not null"`
}
