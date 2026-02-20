package models

import "time"

type Product struct {
	BaseModel
	SKU            string     `json:"sku" gorm:"unique;not null"`
	Name           string     `json:"name" gorm:"not null"`
	Description    string     `json:"description"`
	Price          Money      `json:"price" gorm:"type:numeric(12,2);not null"`
	Stock          int        `json:"stock" gorm:"default:0"`
	Images         []string   `json:"images" gorm:"type:text[]"`
	IsPublished    bool       `json:"is_published" gorm:"not null;default:true;index"`
	DraftData      string     `json:"-" gorm:"type:text"`
	DraftUpdatedAt *time.Time `json:"draft_updated_at"`
	CoverImage     *string    `json:"cover_image,omitempty" gorm:"-"`
	// Self-referential relationship for related products
	Related []Product `json:"related_products" gorm:"many2many:product_related;"`
}
