package models

import "time"

type TaxExport struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Provider    string    `gorm:"not null;index"`
	Format      string    `gorm:"not null;default:'csv'"`
	FiltersJSON string    `gorm:"type:text;not null;default:''"`
	RowCount    int       `gorm:"not null;default:0"`
	Contents    string    `gorm:"type:text;not null;default:''"`
	ExportedAt  time.Time `gorm:"not null;index"`
}
