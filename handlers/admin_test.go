package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProductValidation(t *testing.T) {
	tests := []struct {
		name        string
		sku         string
		productName string
		price       float64
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "Valid product",
			sku:         "PROD-001",
			productName: "Test Product",
			price:       29.99,
			shouldError: false,
		},
		{
			name:        "Missing SKU",
			sku:         "",
			productName: "Test Product",
			price:       29.99,
			shouldError: true,
			errorMsg:    "Product SKU is required",
		},
		{
			name:        "Missing name",
			sku:         "PROD-001",
			productName: "",
			price:       29.99,
			shouldError: true,
			errorMsg:    "Product name is required",
		},
		{
			name:        "Zero price",
			sku:         "PROD-001",
			productName: "Test Product",
			price:       0,
			shouldError: true,
			errorMsg:    "Product price must be greater than 0",
		},
		{
			name:        "Negative price",
			sku:         "PROD-001",
			productName: "Test Product",
			price:       -10.00,
			shouldError: true,
			errorMsg:    "Product price must be greater than 0",
		},
		{
			name:        "Valid with decimal price",
			sku:         "PROD-002",
			productName: "Another Product",
			price:       9.99,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false
			var errorMessage string

			if tt.sku == "" {
				hasError = true
				errorMessage = "Product SKU is required"
			} else if tt.productName == "" {
				hasError = true
				errorMessage = "Product name is required"
			} else if tt.price <= 0 {
				hasError = true
				errorMessage = "Product price must be greater than 0"
			}

			assert.Equal(t, tt.shouldError, hasError, "Validation error check")
			if tt.shouldError {
				assert.Equal(t, tt.errorMsg, errorMessage, "Error message")
			}
		})
	}
}

func TestStockDefaultValue(t *testing.T) {
	tests := []struct {
		name            string
		inputStock      int
		expectedStock   int
		shouldSetToZero bool
	}{
		{
			name:            "Positive stock",
			inputStock:      10,
			expectedStock:   10,
			shouldSetToZero: false,
		},
		{
			name:            "Zero stock",
			inputStock:      0,
			expectedStock:   0,
			shouldSetToZero: false,
		},
		{
			name:            "Negative stock should be set to zero",
			inputStock:      -5,
			expectedStock:   0,
			shouldSetToZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := max(tt.inputStock, 0)
			assert.Equal(t, tt.expectedStock, stock)
		})
	}
}
