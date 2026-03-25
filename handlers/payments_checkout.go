package handlers

import (
	checkoutservice "ecommerce/internal/services/checkout"
	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"
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

func buildSnapshotItemsFromCart(cart *models.Cart) []paymentservice.SnapshotItemInput {
	if cart == nil {
		return nil
	}
	items := make([]paymentservice.SnapshotItemInput, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, paymentservice.SnapshotItemInput{
			ProductVariantID: item.ProductVariantID,
			VariantSKU:       item.ProductVariant.SKU,
			VariantTitle:     item.ProductVariant.Title,
			Quantity:         item.Quantity,
			Price:            item.ProductVariant.Price,
		})
	}
	return items
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
