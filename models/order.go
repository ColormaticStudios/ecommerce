package models

import "gorm.io/gorm"

const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"
	StatusFailed  = "FAILED"
)

type Order struct {
	gorm.Model
	UserID uint        `json:"user_id"`
	User   User        `json:"user" gorm:"foreignKey:UserID"`
	Total  float64     `json:"total"`
	Status string      `json:"status"` // PENDING, PAID, FAILED
	Items  []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

type OrderItem struct {
	gorm.Model
	OrderID   uint    `json:"order_id"`
	Order     Order   `json:"-" gorm:"foreignKey:OrderID"`
	ProductID uint    `json:"product_id"`
	Product   Product `json:"product" gorm:"foreignKey:ProductID"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"` // Price at time of order (snapshot)
}
