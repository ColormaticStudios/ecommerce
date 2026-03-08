package models

type Cart struct {
	BaseModel
	UserID uint       `json:"user_id" gorm:"uniqueIndex"`
	User   User       `json:"-" gorm:"foreignKey:UserID"`
	Items  []CartItem `json:"items" gorm:"foreignKey:CartID"`
}

type CartItem struct {
	BaseModel
	CartID           uint           `json:"cart_id"`
	Cart             Cart           `json:"-" gorm:"foreignKey:CartID"`
	ProductVariantID uint           `json:"product_variant_id"`
	ProductVariant   ProductVariant `json:"product_variant" gorm:"foreignKey:ProductVariantID"`
	Quantity         int            `json:"quantity" gorm:"default:1"`
}
