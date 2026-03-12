package checkout

import (
	"errors"
	"fmt"
	"strings"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"

	"gorm.io/gorm"
)

// ProviderSelection captures checkout plugin selections and field data.
type ProviderSelection struct {
	PaymentProviderID  string
	ShippingProviderID string
	TaxProviderID      string
	PaymentData        map[string]string
	ShippingData       map[string]string
	TaxData            map[string]string
}

// ProviderResolutionResult is the resolved provider output used by order submission.
type ProviderResolutionResult struct {
	Total           models.Money
	PaymentDisplay  string
	ShippingAddress string
}

func HasProviderSelection(selection ProviderSelection) bool {
	return strings.TrimSpace(selection.PaymentProviderID) != "" ||
		strings.TrimSpace(selection.ShippingProviderID) != "" ||
		strings.TrimSpace(selection.TaxProviderID) != ""
}

// ResolveProviderSelection validates provider IDs/data and resolves derived checkout details.
func ResolveProviderSelection(
	manager *checkoutplugins.Manager,
	subtotal models.Money,
	selection ProviderSelection,
) (ProviderResolutionResult, error) {
	if manager == nil {
		return ProviderResolutionResult{}, fmt.Errorf("checkout plugins unavailable")
	}

	quoteReq := checkoutplugins.QuoteRequest{
		Subtotal:     subtotal,
		PaymentID:    strings.TrimSpace(selection.PaymentProviderID),
		ShippingID:   strings.TrimSpace(selection.ShippingProviderID),
		TaxID:        strings.TrimSpace(selection.TaxProviderID),
		PaymentData:  selection.PaymentData,
		ShippingData: selection.ShippingData,
		TaxData:      selection.TaxData,
	}

	quote := manager.Quote(quoteReq)
	if !quote.Valid {
		return ProviderResolutionResult{}, fmt.Errorf("selected providers are invalid")
	}

	details, err := manager.ResolveCheckoutDetails(quoteReq)
	if err != nil {
		return ProviderResolutionResult{}, err
	}

	return ProviderResolutionResult{
		Total:           quote.Total,
		PaymentDisplay:  details.PaymentDisplay,
		ShippingAddress: details.ShippingAddress,
	}, nil
}

// ClearOrderedItemsFromCart removes only the quantities consumed by the order from the checkout session cart.
func ClearOrderedItemsFromCart(tx *gorm.DB, checkoutSessionID uint, orderItems []models.OrderItem) error {
	var cart models.Cart
	if err := tx.Where("checkout_session_id = ?", checkoutSessionID).
		First(&cart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("load cart: %w", err)
	}

	orderedQtyByVariant := make(map[uint]int, len(orderItems))
	for _, item := range orderItems {
		if item.Quantity <= 0 {
			continue
		}
		orderedQtyByVariant[item.ProductVariantID] += item.Quantity
	}
	if len(orderedQtyByVariant) == 0 {
		return nil
	}

	variantIDs := make([]uint, 0, len(orderedQtyByVariant))
	for variantID := range orderedQtyByVariant {
		variantIDs = append(variantIDs, variantID)
	}

	var cartItems []models.CartItem
	if err := tx.Where("cart_id = ? AND product_variant_id IN ?", cart.ID, variantIDs).Find(&cartItems).Error; err != nil {
		return fmt.Errorf("load cart items: %w", err)
	}

	for _, cartItem := range cartItems {
		remaining := cartItem.Quantity - orderedQtyByVariant[cartItem.ProductVariantID]
		if remaining <= 0 {
			if err := tx.Delete(&cartItem).Error; err != nil {
				return fmt.Errorf("clear cart item: %w", err)
			}
			continue
		}

		if err := tx.Model(&cartItem).Update("quantity", remaining).Error; err != nil {
			return fmt.Errorf("update cart item quantity: %w", err)
		}
	}

	return nil
}
