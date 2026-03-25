package models

import "time"

const (
	TaxLineTypeItem     = "ITEM"
	TaxLineTypeShipping = "SHIPPING"
)

type OrderTaxLine struct {
	ID                 uint `gorm:"primaryKey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	OrderID            uint   `gorm:"not null;index"`
	SnapshotID         uint   `gorm:"not null;index"`
	SnapshotItemID     *uint  `gorm:"index"`
	LineType           string `gorm:"not null;index"`
	TaxProviderID      string `gorm:"not null;index"`
	ProductVariantID   *uint
	Jurisdiction       string    `gorm:"not null;default:''"`
	TaxCode            string    `gorm:"not null;default:''"`
	TaxName            string    `gorm:"not null;default:''"`
	Quantity           int       `gorm:"not null;default:0"`
	TaxableAmount      Money     `gorm:"type:numeric(12,2);not null"`
	TaxAmount          Money     `gorm:"type:numeric(12,2);not null"`
	TaxRateBasisPoints int       `gorm:"not null;default:0"`
	Inclusive          bool      `gorm:"not null;default:false"`
	FinalizedAt        time.Time `gorm:"not null;index"`
}
