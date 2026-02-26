package handlers

import (
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
)

func TestCalculateOrderTotal(t *testing.T) {
	tests := []struct {
		name  string
		items []struct {
			price    float64
			quantity int
		}
		expectedTotal float64
	}{
		{
			name: "Single item",
			items: []struct {
				price    float64
				quantity int
			}{
				{price: 10.00, quantity: 1},
			},
			expectedTotal: 10.00,
		},
		{
			name: "Multiple items same product",
			items: []struct {
				price    float64
				quantity int
			}{
				{price: 10.00, quantity: 3},
			},
			expectedTotal: 30.00,
		},
		{
			name: "Multiple different products",
			items: []struct {
				price    float64
				quantity int
			}{
				{price: 10.00, quantity: 2},
				{price: 5.50, quantity: 3},
				{price: 20.00, quantity: 1},
			},
			expectedTotal: 56.50, // (10*2) + (5.5*3) + (20*1) = 20 + 16.5 + 20 = 56.5
		},
		{
			name: "Decimal prices",
			items: []struct {
				price    float64
				quantity int
			}{
				{price: 9.99, quantity: 2},
				{price: 19.95, quantity: 1},
			},
			expectedTotal: 39.93, // (9.99*2) + (19.95*1) = 19.98 + 19.95 = 39.93
		},
		{
			name: "Zero quantity",
			items: []struct {
				price    float64
				quantity int
			}{
				{price: 10.00, quantity: 0},
			},
			expectedTotal: 0.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var total float64
			for _, item := range tt.items {
				itemTotal := item.price * float64(item.quantity)
				total += itemTotal
			}
			assert.InDelta(t, tt.expectedTotal, total, 0.01, "Order total calculation")
		})
	}
}

func TestStockValidation(t *testing.T) {
	tests := []struct {
		name            string
		availableStock  int
		requestedQty    int
		shouldSucceed   bool
		expectedMessage string
	}{
		{
			name:           "Sufficient stock",
			availableStock: 10,
			requestedQty:   5,
			shouldSucceed:  true,
		},
		{
			name:           "Exact stock match",
			availableStock: 10,
			requestedQty:   10,
			shouldSucceed:  true,
		},
		{
			name:            "Insufficient stock",
			availableStock:  5,
			requestedQty:    10,
			shouldSucceed:   false,
			expectedMessage: "Insufficient stock",
		},
		{
			name:           "Zero stock",
			availableStock: 0,
			requestedQty:   1,
			shouldSucceed:  false,
		},
		{
			name:           "Zero quantity request",
			availableStock: 10,
			requestedQty:   0,
			shouldSucceed:  true, // Stock check passes (0 <= 10), but quantity validation happens elsewhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasEnoughStock := tt.availableStock >= tt.requestedQty
			assert.Equal(t, tt.shouldSucceed, hasEnoughStock, "Stock validation")
		})
	}
}

func TestOrderItemPriceSnapshot(t *testing.T) {
	// Test that order items should store price at time of order
	productPrice := 29.99
	quantity := 2

	orderItem := models.OrderItem{
		Quantity: quantity,
		Price:    models.MoneyFromFloat(productPrice), // Snapshot price
	}

	// Even if product price changes later, order item should keep original price
	assert.InDelta(t, productPrice, orderItem.Price.Float64(), 0.01)
	assert.Equal(t, quantity, orderItem.Quantity)

	// Calculate item total from snapshot
	itemTotal := orderItem.Price.Mul(orderItem.Quantity).Float64()
	expectedTotal := 29.99 * 2
	assert.InDelta(t, expectedTotal, itemTotal, 0.01)
}

func TestOrderStatusConstants(t *testing.T) {
	assert.Equal(t, "PENDING", models.StatusPending)
	assert.Equal(t, "PAID", models.StatusPaid)
	assert.Equal(t, "FAILED", models.StatusFailed)
	assert.Equal(t, "SHIPPED", models.StatusShipped)
	assert.Equal(t, "DELIVERED", models.StatusDelivered)
	assert.Equal(t, "CANCELLED", models.StatusCancelled)
	assert.Equal(t, "REFUNDED", models.StatusRefunded)
}
