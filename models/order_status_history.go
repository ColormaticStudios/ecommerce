package models

import "time"

type OrderStatusHistory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	OrderID       uint   `gorm:"not null;index"`
	FromStatus    string `gorm:"not null"`
	ToStatus      string `gorm:"not null"`
	Reason        string `gorm:"not null"`
	Source        string `gorm:"not null"`
	Actor         string `gorm:"not null"`
	CorrelationID string `gorm:"not null;default:'';index"`
}
