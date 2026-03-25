package models

import "time"

type OrderCheckoutSnapshot struct {
	ID                    uint `gorm:"primaryKey"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CheckoutSessionID     uint   `gorm:"not null;index"`
	OrderID               *uint  `gorm:"index"`
	Currency              string `gorm:"not null;size:3"`
	Subtotal              Money  `gorm:"type:numeric(12,2);not null"`
	ShippingAmount        Money  `gorm:"type:numeric(12,2);not null"`
	TaxAmount             Money  `gorm:"type:numeric(12,2);not null"`
	Total                 Money  `gorm:"type:numeric(12,2);not null"`
	PaymentProviderID     string `gorm:"not null"`
	ShippingProviderID    string `gorm:"not null"`
	TaxProviderID         string
	PaymentDataJSON       string    `gorm:"type:text;not null;default:''"`
	ShippingDataJSON      string    `gorm:"type:text;not null;default:''"`
	TaxDataJSON           string    `gorm:"type:text;not null;default:''"`
	PaymentMethodDisplay  string    `gorm:"type:text;not null;default:''"`
	ShippingAddressPretty string    `gorm:"type:text;not null;default:''"`
	ExpiresAt             time.Time `gorm:"not null;index"`
	AuthorizedAt          *time.Time
	Items                 []OrderCheckoutSnapshotItem `gorm:"foreignKey:SnapshotID"`
}

type OrderCheckoutSnapshotItem struct {
	ID               uint `gorm:"primaryKey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SnapshotID       uint   `gorm:"not null;index"`
	ProductVariantID uint   `gorm:"not null"`
	VariantSKU       string `gorm:"not null"`
	VariantTitle     string `gorm:"not null"`
	Quantity         int    `gorm:"not null"`
	Price            Money  `gorm:"type:numeric(12,2);not null"`
}
