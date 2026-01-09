package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	SKU         string   `json:"sku" gorm:"unique;not null"`
	Name        string   `json:"name" gorm:"not null"`
	Description string   `json:"description"`
	Price       float64  `json:"price" gorm:"not null"`
	Stock       int      `json:"stock" gorm:"default:0"`
	Images      []string `json:"images" gorm:"type:text[]"`
	// Self-referential relationship for related products
	Related []Product `json:"related_products" gorm:"many2many:product_related;"`
}
