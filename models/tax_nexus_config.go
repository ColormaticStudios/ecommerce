package models

import "time"

type TaxNexusConfig struct {
	ID               uint `gorm:"primaryKey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Provider         string `gorm:"not null;index:idx_tax_nexus_provider_region,unique"`
	Country          string `gorm:"not null;size:2;index:idx_tax_nexus_provider_region,unique"`
	State            string `gorm:"not null;default:'';index:idx_tax_nexus_provider_region,unique"`
	ExemptionCode    string `gorm:"not null;default:''"`
	InclusivePricing bool   `gorm:"not null;default:false"`
	Active           bool   `gorm:"not null;default:true"`
}
