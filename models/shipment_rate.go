package models

import "time"

type ShipmentRate struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	OrderID        uint   `gorm:"not null;index"`
	SnapshotID     uint   `gorm:"not null;index:idx_shipment_rates_snapshot_provider_rate,unique"`
	Provider       string `gorm:"not null;index:idx_shipment_rates_snapshot_provider_rate,unique"`
	ProviderRateID string `gorm:"not null;index:idx_shipment_rates_snapshot_provider_rate,unique"`
	ServiceCode    string `gorm:"not null"`
	ServiceName    string `gorm:"not null"`
	Amount         Money  `gorm:"type:numeric(12,2);not null"`
	Currency       string `gorm:"not null;size:3"`
	Selected       bool   `gorm:"not null;default:false;index"`
	ExpiresAt      *time.Time
	ShipmentID     *uint `gorm:"index"`
}
