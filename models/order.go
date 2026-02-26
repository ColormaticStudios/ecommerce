package models

const (
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusFailed    = "FAILED"
	StatusShipped   = "SHIPPED"
	StatusDelivered = "DELIVERED"
	StatusCancelled = "CANCELLED"
	StatusRefunded  = "REFUNDED"
)

var validOrderStatuses = map[string]struct{}{
	StatusPending:   {},
	StatusPaid:      {},
	StatusFailed:    {},
	StatusShipped:   {},
	StatusDelivered: {},
	StatusCancelled: {},
	StatusRefunded:  {},
}

var stockCommittedOrderStatuses = map[string]struct{}{
	StatusPaid:      {},
	StatusShipped:   {},
	StatusDelivered: {},
}

var userCancelableOrderStatuses = map[string]struct{}{
	StatusPending: {},
	StatusPaid:    {},
}

func IsValidOrderStatus(status string) bool {
	_, ok := validOrderStatuses[status]
	return ok
}

func IsStockCommittedOrderStatus(status string) bool {
	_, ok := stockCommittedOrderStatuses[status]
	return ok
}

func IsUserCancelableOrderStatus(status string) bool {
	_, ok := userCancelableOrderStatuses[status]
	return ok
}

type Order struct {
	BaseModel
	UserID                uint        `json:"user_id"`
	User                  User        `json:"user" gorm:"foreignKey:UserID"`
	Total                 Money       `json:"total" gorm:"type:numeric(12,2);not null"`
	Status                string      `json:"status"` // PENDING, PAID, FAILED, SHIPPED, DELIVERED, CANCELLED, REFUNDED
	CanCancel             bool        `json:"can_cancel" gorm:"-"`
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
	Price     Money   `json:"price" gorm:"type:numeric(12,2);not null"` // Price at time of order (snapshot)
}
