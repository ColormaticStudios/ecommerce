package handlers

import (
	checkoutservice "ecommerce/internal/services/checkout"
	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"

	"gorm.io/gorm"
)

type AuthorizeCheckoutOrderPaymentRequest struct {
	SnapshotID uint `json:"snapshot_id" binding:"required"`
}

func checkoutProviderSelectionFromPaymentRequest(req ProcessPaymentRequest) checkoutservice.ProviderSelection {
	return checkoutservice.ProviderSelection{
		PaymentProviderID:  req.PaymentProviderID,
		ShippingProviderID: req.ShippingProviderID,
		TaxProviderID:      req.TaxProviderID,
		PaymentData:        req.PaymentData,
		ShippingData:       req.ShippingData,
		TaxData:            req.TaxData,
	}
}

func buildSnapshotItemsFromCart(db *gorm.DB, cart *models.Cart) ([]paymentservice.SnapshotItemInput, error) {
	if cart == nil {
		return nil, nil
	}
	discounts, err := evaluateCartDiscounts(db, cart)
	if err != nil {
		return nil, err
	}
	priceByVariant := make(map[uint]models.Money, len(discounts.Lines))
	for _, line := range discounts.Lines {
		priceByVariant[line.ProductVariantID] = line.FinalPrice
	}
	items := make([]paymentservice.SnapshotItemInput, 0, len(cart.Items))
	for _, item := range cart.Items {
		price := priceByVariant[item.ProductVariantID]
		items = append(items, paymentservice.SnapshotItemInput{
			ProductVariantID: item.ProductVariantID,
			VariantSKU:       item.ProductVariant.SKU,
			VariantTitle:     item.ProductVariant.Title,
			Quantity:         item.Quantity,
			Price:            price,
		})
	}
	return items, nil
}

func buildSnapshotItemsFromOrder(order *models.Order) []paymentservice.SnapshotItemInput {
	if order == nil {
		return nil
	}
	items := make([]paymentservice.SnapshotItemInput, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, paymentservice.SnapshotItemInput{
			ProductVariantID: item.ProductVariantID,
			VariantSKU:       item.VariantSKU,
			VariantTitle:     item.VariantTitle,
			Quantity:         item.Quantity,
			Price:            item.Price,
		})
	}
	return items
}
