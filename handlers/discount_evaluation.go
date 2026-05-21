package handlers

import (
	"time"

	discountservice "ecommerce/internal/services/discounts"
	"ecommerce/models"

	"gorm.io/gorm"
)

func evaluateCartDiscounts(db *gorm.DB, cart *models.Cart) (discountservice.EvaluationResult, error) {
	lines, err := discountCartLines(db, cart)
	if err != nil {
		return discountservice.EvaluationResult{}, err
	}
	return discountservice.EvaluateCart(db, lines, time.Now().UTC())
}

func discountCartLines(db *gorm.DB, cart *models.Cart) ([]discountservice.CartLine, error) {
	if cart == nil {
		return nil, nil
	}
	productIDs := make([]uint, 0, len(cart.Items))
	for _, item := range cart.Items {
		productIDs = append(productIDs, item.ProductVariant.ProductID)
	}
	categoriesByProduct, err := productCategoryIDsByProduct(db, productIDs)
	if err != nil {
		return nil, err
	}
	lines := make([]discountservice.CartLine, 0, len(cart.Items))
	for _, item := range cart.Items {
		product := item.ProductVariant.Product
		lines = append(lines, discountservice.CartLine{
			ProductID:        item.ProductVariant.ProductID,
			ProductVariantID: item.ProductVariantID,
			BrandID:          product.BrandID,
			CategoryIDs:      categoriesByProduct[item.ProductVariant.ProductID],
			SKU:              item.ProductVariant.SKU,
			Quantity:         item.Quantity,
			UnitPrice:        item.ProductVariant.Price,
		})
	}
	return lines, nil
}

func productCategoryIDsByProduct(db *gorm.DB, productIDs []uint) (map[uint][]uint, error) {
	result := make(map[uint][]uint, len(productIDs))
	if len(productIDs) == 0 || !db.Migrator().HasTable(&models.ProductCategory{}) {
		return result, nil
	}
	var rows []models.ProductCategory
	if err := db.Where("product_id IN ?", productIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ProductID] = append(result[row.ProductID], row.CategoryID)
	}
	return result, nil
}
