package models

import "gorm.io/gorm"

type Cart struct {
	gorm.Model
	UserID uint       `json:"user_id"`
	User   User       `json:"-" gorm:"foreignKey:UserID"`
	Items  []CartItem `json:"items" gorm:"foreignKey:CartID"`
}

type CartItem struct {
	gorm.Model
	CartID    uint    `json:"cart_id"`
	Cart      Cart    `json:"-" gorm:"foreignKey:CartID"`
	ProductID uint    `json:"product_id"`
	Product   Product `json:"product" gorm:"foreignKey:ProductID"`
	Quantity  int     `json:"quantity" gorm:"default:1"`
}
