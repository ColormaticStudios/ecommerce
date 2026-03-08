package models

import "time"

type Product struct {
	BaseModel
	SKU              string                  `json:"sku" gorm:"unique;not null"`
	Name             string                  `json:"name" gorm:"not null"`
	Subtitle         *string                 `json:"subtitle,omitempty"`
	Description      string                  `json:"description"`
	Price            Money                   `json:"price" gorm:"type:numeric(12,2);not null"`
	Stock            int                     `json:"stock" gorm:"default:0"`
	Images           []string                `json:"images" gorm:"type:text[]"`
	BrandID          *uint                   `json:"brand_id,omitempty" gorm:"index"`
	DefaultVariantID *uint                   `json:"default_variant_id,omitempty" gorm:"index"`
	IsPublished      bool                    `json:"is_published" gorm:"not null;default:true;index"`
	DraftUpdatedAt   *time.Time              `json:"draft_updated_at"`
	CoverImage       *string                 `json:"cover_image,omitempty" gorm:"-"`
	Brand            *Brand                  `json:"brand,omitempty"`
	DefaultVariant   *ProductVariant         `json:"default_variant,omitempty" gorm:"foreignKey:DefaultVariantID"`
	Options          []ProductOption         `json:"options,omitempty"`
	Variants         []ProductVariant        `json:"variants,omitempty"`
	AttributeValues  []ProductAttributeValue `json:"attribute_values,omitempty"`
	// Self-referential relationship for related products
	Related []Product `json:"related_products" gorm:"many2many:product_related;"`
}
