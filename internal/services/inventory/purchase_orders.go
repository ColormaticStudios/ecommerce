package inventory

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MovementTypeRestockReceipt = "RESTOCK_RECEIPT"
	ReferenceTypeReceipt       = "INVENTORY_RECEIPT"
)

type SupplierInput struct {
	Name  string
	Email string
	Notes string
}

type PurchaseOrderItemInput struct {
	ProductVariantID uint
	QuantityOrdered  int
	UnitCost         float64
}

type PurchaseOrderInput struct {
	SupplierID *uint
	Supplier   *SupplierInput
	Notes      string
	Items      []PurchaseOrderItemInput
}

type ReceiveItemInput struct {
	PurchaseOrderItemID uint
	QuantityReceived    int
}

type ReceivePurchaseOrderInput struct {
	Items     []ReceiveItemInput
	Notes     string
	ActorType string
	ActorID   *uint
}

func ListPurchaseOrders(db *gorm.DB, limit int) ([]models.PurchaseOrder, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var orders []models.PurchaseOrder
	err := db.Preload("Supplier").
		Preload("Items").
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func CreatePurchaseOrder(db *gorm.DB, input PurchaseOrderInput) (models.PurchaseOrder, error) {
	if len(input.Items) == 0 {
		return models.PurchaseOrder{}, fmt.Errorf("purchase order requires at least one item")
	}
	var po models.PurchaseOrder
	err := db.Transaction(func(tx *gorm.DB) error {
		supplierID := input.SupplierID
		if supplierID == nil && input.Supplier != nil && strings.TrimSpace(input.Supplier.Name) != "" {
			supplier := models.Supplier{
				Name:  strings.TrimSpace(input.Supplier.Name),
				Email: strings.TrimSpace(input.Supplier.Email),
				Notes: strings.TrimSpace(input.Supplier.Notes),
			}
			if err := tx.Where("name = ?", supplier.Name).FirstOrCreate(&supplier).Error; err != nil {
				return err
			}
			supplierID = &supplier.ID
		}
		po = models.PurchaseOrder{
			SupplierID: supplierID,
			Status:     models.PurchaseOrderStatusDraft,
			Notes:      strings.TrimSpace(input.Notes),
		}
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		for _, item := range input.Items {
			if item.ProductVariantID == 0 || item.QuantityOrdered < 1 {
				return fmt.Errorf("purchase order items require variant and positive quantity")
			}
			row := models.PurchaseOrderItem{
				PurchaseOrderID:  po.ID,
				ProductVariantID: item.ProductVariantID,
				QuantityOrdered:  item.QuantityOrdered,
				UnitCost:         models.MoneyFromFloat(item.UnitCost),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return tx.Preload("Supplier").Preload("Items").First(&po, po.ID).Error
	})
	return po, err
}

func IssuePurchaseOrder(db *gorm.DB, purchaseOrderID uint) (models.PurchaseOrder, error) {
	return updatePurchaseOrderStatus(db, purchaseOrderID, models.PurchaseOrderStatusIssued)
}

func CancelPurchaseOrder(db *gorm.DB, purchaseOrderID uint) (models.PurchaseOrder, error) {
	return updatePurchaseOrderStatus(db, purchaseOrderID, models.PurchaseOrderStatusCancelled)
}

func updatePurchaseOrderStatus(db *gorm.DB, purchaseOrderID uint, status string) (models.PurchaseOrder, error) {
	var po models.PurchaseOrder
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&po, purchaseOrderID).Error; err != nil {
			return err
		}
		if status == models.PurchaseOrderStatusIssued {
			if po.Status != models.PurchaseOrderStatusDraft {
				return fmt.Errorf("only draft purchase orders can be issued")
			}
			now := time.Now().UTC()
			po.Status = status
			po.IssuedAt = &now
		} else if status == models.PurchaseOrderStatusCancelled {
			if po.Status == models.PurchaseOrderStatusReceived {
				return fmt.Errorf("received purchase orders cannot be cancelled")
			}
			now := time.Now().UTC()
			po.Status = status
			po.CancelledAt = &now
		}
		if err := tx.Save(&po).Error; err != nil {
			return err
		}
		return tx.Preload("Supplier").Preload("Items").First(&po, po.ID).Error
	})
	return po, err
}

func ReceivePurchaseOrder(db *gorm.DB, purchaseOrderID uint, input ReceivePurchaseOrderInput) (models.InventoryReceipt, models.PurchaseOrder, error) {
	if len(input.Items) == 0 {
		return models.InventoryReceipt{}, models.PurchaseOrder{}, fmt.Errorf("receipt requires at least one item")
	}
	var receipt models.InventoryReceipt
	var po models.PurchaseOrder
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&po, purchaseOrderID).Error; err != nil {
			return err
		}
		if po.Status != models.PurchaseOrderStatusIssued && po.Status != models.PurchaseOrderStatusPartiallyReceived {
			return fmt.Errorf("purchase order is not receivable")
		}
		itemsByID := map[uint]*models.PurchaseOrderItem{}
		for i := range po.Items {
			itemsByID[po.Items[i].ID] = &po.Items[i]
		}
		now := time.Now().UTC()
		receipt = models.InventoryReceipt{
			PurchaseOrderID: po.ID,
			ReceivedAt:      now,
			ActorType:       strings.TrimSpace(input.ActorType),
			ActorID:         input.ActorID,
			Notes:           strings.TrimSpace(input.Notes),
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		for _, received := range input.Items {
			item := itemsByID[received.PurchaseOrderItemID]
			if item == nil {
				return fmt.Errorf("purchase order item not found")
			}
			if received.QuantityReceived < 1 {
				return fmt.Errorf("received quantity must be positive")
			}
			open := item.QuantityOrdered - item.QuantityReceived
			if received.QuantityReceived > open {
				return fmt.Errorf("received quantity exceeds open quantity")
			}
			receiptItem := models.InventoryReceiptItem{
				InventoryReceiptID:  receipt.ID,
				PurchaseOrderItemID: item.ID,
				ProductVariantID:    item.ProductVariantID,
				QuantityReceived:    received.QuantityReceived,
			}
			if err := tx.Create(&receiptItem).Error; err != nil {
				return err
			}
			item.QuantityReceived += received.QuantityReceived
			if err := tx.Model(&models.PurchaseOrderItem{}).Where("id = ?", item.ID).Update("quantity_received", item.QuantityReceived).Error; err != nil {
				return err
			}
			referenceID := receipt.ID
			if _, err := ApplyMovement(tx, MovementInput{
				ProductVariantID: item.ProductVariantID,
				MovementType:     MovementTypeRestockReceipt,
				QuantityDelta:    received.QuantityReceived,
				ReferenceType:    ReferenceTypeReceipt,
				ReferenceID:      &referenceID,
				ReasonCode:       "purchase_order_receipt",
				ActorType:        strings.TrimSpace(input.ActorType),
				ActorID:          input.ActorID,
			}); err != nil {
				return err
			}
		}
		allReceived := true
		anyReceived := false
		for _, item := range itemsByID {
			if item.QuantityReceived > 0 {
				anyReceived = true
			}
			if item.QuantityReceived < item.QuantityOrdered {
				allReceived = false
			}
		}
		if allReceived {
			po.Status = models.PurchaseOrderStatusReceived
			po.ReceivedAt = &now
		} else if anyReceived {
			po.Status = models.PurchaseOrderStatusPartiallyReceived
		}
		if err := tx.Save(&po).Error; err != nil {
			return err
		}
		if err := tx.Preload("Items").First(&receipt, receipt.ID).Error; err != nil {
			return err
		}
		return tx.Preload("Supplier").Preload("Items").First(&po, po.ID).Error
	})
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return receipt, po, err
	}
	return receipt, po, err
}
