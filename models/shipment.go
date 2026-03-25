package models

import "time"

const (
	ShipmentStatusQuoted         = "QUOTED"
	ShipmentStatusLabelPurchased = "LABEL_PURCHASED"
	ShipmentStatusInTransit      = "IN_TRANSIT"
	ShipmentStatusDelivered      = "DELIVERED"
	ShipmentStatusException      = "EXCEPTION"
)

type Shipment struct {
	ID                    uint `gorm:"primaryKey"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	OrderID               uint   `gorm:"not null;index"`
	SnapshotID            uint   `gorm:"not null;index"`
	Provider              string `gorm:"not null;index"`
	ShipmentRateID        uint   `gorm:"not null;uniqueIndex"`
	ProviderShipmentID    string `gorm:"not null;index"`
	Status                string `gorm:"not null;index"`
	Currency              string `gorm:"not null;size:3"`
	ServiceCode           string `gorm:"not null"`
	ServiceName           string `gorm:"not null"`
	Amount                Money  `gorm:"type:numeric(12,2);not null"`
	ShippingAddressPretty string `gorm:"type:text;not null;default:''"`
	TrackingNumber        string `gorm:"not null;default:''"`
	TrackingURL           string `gorm:"type:text;not null;default:''"`
	LabelURL              string `gorm:"type:text;not null;default:''"`
	PurchasedAt           *time.Time
	FinalizedAt           *time.Time `gorm:"index"`
	DeliveredAt           *time.Time
	Rates                 []ShipmentRate    `gorm:"foreignKey:ShipmentID"`
	Packages              []ShipmentPackage `gorm:"foreignKey:ShipmentID"`
	TrackingEvents        []TrackingEvent   `gorm:"foreignKey:ShipmentID"`
}
