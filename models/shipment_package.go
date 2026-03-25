package models

import "time"

type ShipmentPackage struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ShipmentID  uint   `gorm:"not null;index"`
	Reference   string `gorm:"not null;default:''"`
	WeightGrams int    `gorm:"not null;default:0"`
	LengthCM    int    `gorm:"not null;default:0"`
	WidthCM     int    `gorm:"not null;default:0"`
	HeightCM    int    `gorm:"not null;default:0"`
}
