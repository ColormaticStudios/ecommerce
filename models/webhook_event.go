package models

import "time"

type WebhookEvent struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Provider        string     `gorm:"not null;index:idx_webhook_events_provider_event,unique"`
	ProviderEventID string     `gorm:"not null;index:idx_webhook_events_provider_event,unique"`
	EventType       string     `gorm:"not null;index"`
	SignatureValid  bool       `gorm:"not null"`
	Payload         string     `gorm:"type:text;not null"`
	ReceivedAt      time.Time  `gorm:"not null;index"`
	ProcessedAt     *time.Time `gorm:"index"`
	AttemptCount    int        `gorm:"not null;default:0"`
	LastError       string     `gorm:"type:text;not null;default:''"`
}
