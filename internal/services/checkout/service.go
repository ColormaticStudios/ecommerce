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

// ClearOrderedItemsFromCart removes only the quantities consumed by the order from the cart.
func ClearOrderedItemsFromCart(tx *gorm.DB, userID uint, orderItems []models.OrderItem) error {
	var cart models.Cart
	if err := tx.Where("user_id = ?", userID).First(&cart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("load cart: %w", err)
	}

	orderedQtyByProduct := make(map[uint]int, len(orderItems))
	for _, item := range orderItems {
		if item.Quantity <= 0 {
			continue
		}
		orderedQtyByProduct[item.ProductID] += item.Quantity
	}
	if len(orderedQtyByProduct) == 0 {
		return nil
	}

	productIDs := make([]uint, 0, len(orderedQtyByProduct))
	for productID := range orderedQtyByProduct {
		productIDs = append(productIDs, productID)
	}

	var cartItems []models.CartItem
	if err := tx.Where("cart_id = ? AND product_id IN ?", cart.ID, productIDs).Find(&cartItems).Error; err != nil {
		return fmt.Errorf("load cart items: %w", err)
	}

	for _, cartItem := range cartItems {
		remaining := cartItem.Quantity - orderedQtyByProduct[cartItem.ProductID]
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
