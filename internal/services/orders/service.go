package orders

import (
	"fmt"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InsufficientStockError describes stock validation failures during stock transitions.
type InsufficientStockError struct {
	ProductID   uint
	ProductName string
	Requested   int
	Available   int
}

func (e *InsufficientStockError) Error() string {
	return "insufficient stock"
}

// DeductStockForItems decrements stock for every order item while holding row locks.
func DeductStockForItems(tx *gorm.DB, items []models.OrderItem) error {
	for _, item := range items {
		var product models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, item.ProductID).Error; err != nil {
			return err
		}

		if product.Stock < item.Quantity {
			return &InsufficientStockError{
				ProductID:   item.ProductID,
				ProductName: product.Name,
				Requested:   item.Quantity,
				Available:   product.Stock,
			}
		}

		if err := tx.Model(&models.Product{}).
			Where("id = ? AND stock >= ?", item.ProductID, item.Quantity).
			Update("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
			return err
		}
	}
	return nil
}

// ReplenishStockForItems restores stock for every order item while holding row locks.
func ReplenishStockForItems(tx *gorm.DB, items []models.OrderItem) error {
	for _, item := range items {
		var product models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, item.ProductID).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.Product{}).
			Where("id = ?", item.ProductID).
			Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
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
		if err := DeductStockForItems(tx, items); err != nil {
			return err
		}
	} else if !willCommitStock && wasStockCommitted {
		if err := ReplenishStockForItems(tx, items); err != nil {
			return err
		}
	}

	order.Status = newStatus
	return tx.Save(order).Error
}
