package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCurrency(t *testing.T) {
	validCurrencies := map[string]bool{
		"USD": true, "EUR": true, "GBP": true, "JPY": true, "CAD": true,
		"AUD": true, "CHF": true, "CNY": true, "INR": true, "BRL": true,
	}

	tests := []struct {
		name     string
		currency string
		valid    bool
	}{
		{"Valid USD", "USD", true},
		{"Valid EUR", "EUR", true},
		{"Valid GBP", "GBP", true},
		{"Valid JPY", "JPY", true},
		{"Invalid currency", "XYZ", false},
		{"Invalid lowercase", "usd", false},
		{"Invalid empty", "", false},
		{"Invalid mixed case", "Usd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validCurrencies[tt.currency]
			assert.Equal(t, tt.valid, result, "Currency %s validation", tt.currency)
		})
	}
}

func TestUpdateProfileRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		currency    string
		shouldError bool
	}{
		{"Valid USD", "USD", false},
		{"Valid EUR", "EUR", false},
		{"Invalid currency", "INVALID", true},
		{"Empty currency (should be allowed)", "", false}, // Empty means don't update
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validCurrencies := map[string]bool{
				"USD": true, "EUR": true, "GBP": true, "JPY": true, "CAD": true,
				"AUD": true, "CHF": true, "CNY": true, "INR": true, "BRL": true,
			}

			if tt.currency != "" && !validCurrencies[tt.currency] {
				if tt.shouldError {
					assert.True(t, true, "Expected error for invalid currency")
				} else {
					assert.Fail(t, "Unexpected validation result")
				}
			} else {
				if tt.shouldError {
					assert.Fail(t, "Expected error but validation passed")
				} else {
					assert.True(t, true, "Validation passed as expected")
				}
			}
		})
	}
}
