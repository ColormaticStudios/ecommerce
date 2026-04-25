package orders

import (
	"errors"
	"fmt"

	inventoryservice "ecommerce/internal/services/inventory"
	"ecommerce/models"

	"gorm.io/gorm"
)

// InsufficientStockError describes stock validation failures during stock transitions.
type InsufficientStockError struct {
	ProductVariantID uint
	ProductName      string
	Requested        int
	Available        int
}

func (e *InsufficientStockError) Error() string {
	return "insufficient stock"
}

// DeductStockForItems decrements inventory for every order item.
func DeductStockForItems(tx *gorm.DB, orderID uint, items []models.OrderItem) error {
	for _, item := range items {
		var variant models.ProductVariant
		if err := tx.Preload("Product").First(&variant, item.ProductVariantID).Error; err != nil {
			return err
		}

		availability, err := inventoryservice.ApplyMovement(tx, inventoryservice.MovementInput{
			ProductVariantID: item.ProductVariantID,
			MovementType:     inventoryservice.MovementTypeOrderCommit,
			QuantityDelta:    -item.Quantity,
			ReferenceType:    inventoryservice.ReferenceTypeOrder,
			ReferenceID:      &orderID,
			ReasonCode:       "order_stock_commit",
		})
		if err != nil {
			var availabilityErr *inventoryservice.InsufficientAvailabilityError
			if errors.As(err, &availabilityErr) {
				return &InsufficientStockError{
					ProductVariantID: item.ProductVariantID,
					ProductName:      variant.Product.Name,
					Requested:        item.Quantity,
					Available:        availabilityErr.Available,
				}
			}
			return err
		}
		if availability.Available < 0 {
			return &InsufficientStockError{
				ProductVariantID: item.ProductVariantID,
				ProductName:      variant.Product.Name,
				Requested:        item.Quantity,
				Available:        availability.Available,
			}
		}
	}
	return nil
}

// ReplenishStockForItems restores inventory for every order item.
func ReplenishStockForItems(tx *gorm.DB, orderID uint, items []models.OrderItem) error {
	for _, item := range items {
		if _, err := inventoryservice.ApplyMovement(tx, inventoryservice.MovementInput{
			ProductVariantID: item.ProductVariantID,
			MovementType:     inventoryservice.MovementTypeOrderRelease,
			QuantityDelta:    item.Quantity,
			ReferenceType:    inventoryservice.ReferenceTypeOrder,
			ReferenceID:      &orderID,
			ReasonCode:       "order_stock_release",
		}); err != nil {
			return err
		}
	}
	return nil
}

// ApplyStatusTransition updates an order status and manages stock commitment transitions.
func ApplyStatusTransition(tx *gorm.DB, order *models.Order, newStatus string) error {
	if order == nil {
		return fmt.Errorf("order is required")
	}
	if !models.IsValidOrderStatus(newStatus) {
		return fmt.Errorf("invalid order status")
	}

	var items []models.OrderItem
	if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return err
	}

	wasStockCommitted := models.IsStockCommittedOrderStatus(order.Status)
	willCommitStock := models.IsStockCommittedOrderStatus(newStatus)

	if willCommitStock && !wasStockCommitted {
		consumed, err := inventoryservice.ConsumeReservationsForOrder(tx, order.ID, fmt.Sprintf("order-status:%d:%s", order.ID, newStatus))
		if err != nil {
			return err
		}
		if !consumed {
			if err := DeductStockForItems(tx, order.ID, items); err != nil {
				return err
			}
		}
	} else if !willCommitStock && wasStockCommitted {
		if err := ReplenishStockForItems(tx, order.ID, items); err != nil {
			return err
		}
	} else if !willCommitStock && order.Status != newStatus {
		if err := inventoryservice.ReleaseReservationsForOrder(tx, order.ID, fmt.Sprintf("order-status:%d:%s", order.ID, newStatus)); err != nil {
			return err
		}
	}

	order.Status = newStatus
	return tx.Save(order).Error
}
