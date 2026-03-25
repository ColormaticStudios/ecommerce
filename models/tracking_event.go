package models

import "time"

type TrackingEvent struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ShipmentID      uint      `gorm:"not null;index:idx_tracking_events_shipment_provider_event,unique"`
	Provider        string    `gorm:"not null;index:idx_tracking_events_shipment_provider_event,unique"`
	ProviderEventID string    `gorm:"not null;index:idx_tracking_events_shipment_provider_event,unique"`
	Status          string    `gorm:"not null;index"`
	TrackingNumber  string    `gorm:"not null;default:''"`
	Location        string    `gorm:"type:text;not null;default:''"`
	Description     string    `gorm:"type:text;not null;default:''"`
	OccurredAt      time.Time `gorm:"not null;index"`
	RawPayload      string    `gorm:"type:text;not null;default:''"`
}
