package models

type Cart struct {
	BaseModel
	CheckoutSessionID uint            `json:"checkout_session_id" gorm:"uniqueIndex"`
	CheckoutSession   CheckoutSession `json:"-" gorm:"foreignKey:CheckoutSessionID"`
	Items             []CartItem      `json:"items" gorm:"foreignKey:CartID"`
}

type CartItem struct {
	BaseModel
	CartID           uint           `json:"cart_id"`
	Cart             Cart           `json:"-" gorm:"foreignKey:CartID"`
	ProductVariantID uint           `json:"product_variant_id"`
	ProductVariant   ProductVariant `json:"product_variant" gorm:"foreignKey:ProductVariantID"`
	Quantity         int            `json:"quantity" gorm:"default:1"`
}
