package models

const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"
	StatusFailed  = "FAILED"
)

type Order struct {
	BaseModel
	UserID                uint        `json:"user_id"`
	User                  User        `json:"user" gorm:"foreignKey:UserID"`
	Total                 float64     `json:"total"`
	Status                string      `json:"status"` // PENDING, PAID, FAILED
	PaymentMethodDisplay  string      `json:"payment_method_display"`
	ShippingAddressPretty string      `json:"shipping_address_pretty"`
	Items                 []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

type OrderItem struct {
	BaseModel
	OrderID   uint    `json:"order_id"`
	Order     Order   `json:"-" gorm:"foreignKey:OrderID"`
	ProductID uint    `json:"product_id"`
	Product   Product `json:"product" gorm:"foreignKey:ProductID"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"` // Price at time of order (snapshot)
}
