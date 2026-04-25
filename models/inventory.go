package models

import "time"

type InventoryItem struct {
	BaseModel
	ProductVariantID uint           `json:"product_variant_id" gorm:"not null;uniqueIndex"`
	ProductVariant   ProductVariant `json:"-" gorm:"foreignKey:ProductVariantID"`
}

type InventoryLevel struct {
	BaseModel
	InventoryItemID uint          `json:"inventory_item_id" gorm:"not null;uniqueIndex"`
	InventoryItem   InventoryItem `json:"-" gorm:"foreignKey:InventoryItemID"`
	OnHand          int           `json:"on_hand" gorm:"not null;default:0"`
	Reserved        int           `json:"reserved" gorm:"not null;default:0"`
	Available       int           `json:"available" gorm:"not null;default:0"`
}

type InventoryMovement struct {
	BaseModel
	InventoryItemID uint          `json:"inventory_item_id" gorm:"not null;index"`
	InventoryItem   InventoryItem `json:"-" gorm:"foreignKey:InventoryItemID"`
	MovementType    string        `json:"movement_type" gorm:"not null;size:64;index"`
	QuantityDelta   int           `json:"quantity_delta" gorm:"not null"`
	ReferenceType   string        `json:"reference_type" gorm:"not null;size:64;default:'';index"`
	ReferenceID     *uint         `json:"reference_id,omitempty" gorm:"index"`
	ReasonCode      string        `json:"reason_code" gorm:"not null;size:64;default:''"`
	ActorType       string        `json:"actor_type" gorm:"not null;size:64;default:''"`
	ActorID         *uint         `json:"actor_id,omitempty"`
}

const (
	InventoryAdjustmentReasonCycleCountGain = "CYCLE_COUNT_GAIN"
	InventoryAdjustmentReasonCycleCountLoss = "CYCLE_COUNT_LOSS"
	InventoryAdjustmentReasonDamage         = "DAMAGE"
	InventoryAdjustmentReasonShrinkage      = "SHRINKAGE"
	InventoryAdjustmentReasonReturnRestock  = "RETURN_RESTOCK"
	InventoryAdjustmentReasonCorrection     = "CORRECTION"
)

type InventoryAdjustment struct {
	BaseModel
	InventoryItemID  uint           `json:"inventory_item_id" gorm:"not null;index"`
	InventoryItem    InventoryItem  `json:"-" gorm:"foreignKey:InventoryItemID"`
	ProductVariantID uint           `json:"product_variant_id" gorm:"not null;index"`
	ProductVariant   ProductVariant `json:"-" gorm:"foreignKey:ProductVariantID"`
	QuantityDelta    int            `json:"quantity_delta" gorm:"not null"`
	ReasonCode       string         `json:"reason_code" gorm:"not null;size:64;index"`
	Notes            string         `json:"notes" gorm:"type:text;not null;default:''"`
	ActorType        string         `json:"actor_type" gorm:"not null;size:64"`
	ActorID          *uint          `json:"actor_id,omitempty"`
	ApprovedByType   string         `json:"approved_by_type" gorm:"not null;size:64;default:''"`
	ApprovedByID     *uint          `json:"approved_by_id,omitempty"`
	ApprovedAt       *time.Time     `json:"approved_at,omitempty"`
}

const (
	InventoryReservationStatusActive   = "ACTIVE"
	InventoryReservationStatusConsumed = "CONSUMED"
	InventoryReservationStatusReleased = "RELEASED"
	InventoryReservationStatusExpired  = "EXPIRED"
)

type InventoryReservation struct {
	BaseModel
	InventoryItemID   uint          `json:"inventory_item_id" gorm:"not null;index"`
	InventoryItem     InventoryItem `json:"-" gorm:"foreignKey:InventoryItemID"`
	ProductVariantID  uint          `json:"product_variant_id" gorm:"not null;index"`
	Quantity          int           `json:"quantity" gorm:"not null"`
	Status            string        `json:"status" gorm:"not null;size:32;index"`
	ExpiresAt         time.Time     `json:"expires_at" gorm:"not null;index"`
	OwnerType         string        `json:"owner_type" gorm:"not null;size:64;default:'';index"`
	OwnerID           *uint         `json:"owner_id,omitempty" gorm:"index"`
	CheckoutSessionID *uint         `json:"checkout_session_id,omitempty" gorm:"index"`
	OrderID           *uint         `json:"order_id,omitempty" gorm:"index"`
	IdempotencyKey    string        `json:"idempotency_key" gorm:"not null;size:255;uniqueIndex"`
	ConsumedAt        *time.Time    `json:"consumed_at,omitempty"`
	ReleasedAt        *time.Time    `json:"released_at,omitempty"`
	ExpiredAt         *time.Time    `json:"expired_at,omitempty"`
}

type InventoryThreshold struct {
	BaseModel
	ProductVariantID *uint          `json:"product_variant_id,omitempty" gorm:"uniqueIndex"`
	ProductVariant   ProductVariant `json:"-" gorm:"foreignKey:ProductVariantID"`
	LowStockQuantity int            `json:"low_stock_quantity" gorm:"not null;default:5"`
}

const (
	InventoryAlertTypeLowStock   = "LOW_STOCK"
	InventoryAlertTypeOutOfStock = "OUT_OF_STOCK"
	InventoryAlertTypeRecovery   = "RECOVERY"

	InventoryAlertStatusOpen     = "OPEN"
	InventoryAlertStatusAcked    = "ACKED"
	InventoryAlertStatusResolved = "RESOLVED"
)

type InventoryAlert struct {
	BaseModel
	InventoryItemID  uint          `json:"inventory_item_id" gorm:"not null;index"`
	InventoryItem    InventoryItem `json:"-" gorm:"foreignKey:InventoryItemID"`
	ProductVariantID uint          `json:"product_variant_id" gorm:"not null;index"`
	AlertType        string        `json:"alert_type" gorm:"not null;size:32;index"`
	Status           string        `json:"status" gorm:"not null;size:32;index"`
	Available        int           `json:"available" gorm:"not null"`
	Threshold        int           `json:"threshold" gorm:"not null"`
	OpenedAt         time.Time     `json:"opened_at" gorm:"not null;index"`
	AckedAt          *time.Time    `json:"acked_at,omitempty"`
	AckedByType      string        `json:"acked_by_type" gorm:"not null;size:64;default:''"`
	AckedByID        *uint         `json:"acked_by_id,omitempty"`
	ResolvedAt       *time.Time    `json:"resolved_at,omitempty"`
	ResolvedByType   string        `json:"resolved_by_type" gorm:"not null;size:64;default:''"`
	ResolvedByID     *uint         `json:"resolved_by_id,omitempty"`
}

type Supplier struct {
	BaseModel
	Name  string `json:"name" gorm:"not null;size:255;uniqueIndex"`
	Email string `json:"email" gorm:"not null;size:255;default:''"`
	Notes string `json:"notes" gorm:"type:text;not null;default:''"`
}

const (
	PurchaseOrderStatusDraft             = "DRAFT"
	PurchaseOrderStatusIssued            = "ISSUED"
	PurchaseOrderStatusPartiallyReceived = "PARTIALLY_RECEIVED"
	PurchaseOrderStatusReceived          = "RECEIVED"
	PurchaseOrderStatusCancelled         = "CANCELLED"
)

type PurchaseOrder struct {
	BaseModel
	SupplierID  *uint               `json:"supplier_id,omitempty" gorm:"index"`
	Supplier    *Supplier           `json:"supplier,omitempty" gorm:"foreignKey:SupplierID"`
	Status      string              `json:"status" gorm:"not null;size:32;index"`
	Notes       string              `json:"notes" gorm:"type:text;not null;default:''"`
	IssuedAt    *time.Time          `json:"issued_at,omitempty"`
	ReceivedAt  *time.Time          `json:"received_at,omitempty"`
	CancelledAt *time.Time          `json:"cancelled_at,omitempty"`
	Items       []PurchaseOrderItem `json:"items,omitempty"`
}

type PurchaseOrderItem struct {
	BaseModel
	PurchaseOrderID  uint           `json:"purchase_order_id" gorm:"not null;index"`
	PurchaseOrder    PurchaseOrder  `json:"-" gorm:"foreignKey:PurchaseOrderID"`
	ProductVariantID uint           `json:"product_variant_id" gorm:"not null;index"`
	ProductVariant   ProductVariant `json:"-" gorm:"foreignKey:ProductVariantID"`
	QuantityOrdered  int            `json:"quantity_ordered" gorm:"not null"`
	QuantityReceived int            `json:"quantity_received" gorm:"not null;default:0"`
	UnitCost         Money          `json:"unit_cost" gorm:"type:numeric(12,2);not null;default:0"`
}

type InventoryReceipt struct {
	BaseModel
	PurchaseOrderID uint                   `json:"purchase_order_id" gorm:"not null;index"`
	PurchaseOrder   PurchaseOrder          `json:"-" gorm:"foreignKey:PurchaseOrderID"`
	ReceivedAt      time.Time              `json:"received_at" gorm:"not null;index"`
	ActorType       string                 `json:"actor_type" gorm:"not null;size:64;default:''"`
	ActorID         *uint                  `json:"actor_id,omitempty"`
	Notes           string                 `json:"notes" gorm:"type:text;not null;default:''"`
	Items           []InventoryReceiptItem `json:"items,omitempty"`
}

type InventoryReceiptItem struct {
	BaseModel
	InventoryReceiptID  uint              `json:"inventory_receipt_id" gorm:"not null;index"`
	InventoryReceipt    InventoryReceipt  `json:"-" gorm:"foreignKey:InventoryReceiptID"`
	PurchaseOrderItemID uint              `json:"purchase_order_item_id" gorm:"not null;index"`
	PurchaseOrderItem   PurchaseOrderItem `json:"-" gorm:"foreignKey:PurchaseOrderItemID"`
	ProductVariantID    uint              `json:"product_variant_id" gorm:"not null;index"`
	ProductVariant      ProductVariant    `json:"-" gorm:"foreignKey:ProductVariantID"`
	QuantityReceived    int               `json:"quantity_received" gorm:"not null"`
}
