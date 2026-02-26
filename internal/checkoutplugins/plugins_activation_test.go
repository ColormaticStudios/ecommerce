package checkoutplugins

import (
	"testing"

	"ecommerce/models"
)

func TestListIncludesOnlyEnabledProvidersForCheckout(t *testing.T) {
	manager := NewDefaultManager()

	payments, shippings, taxes := manager.List()
	if len(payments) != 2 {
		t.Fatalf("expected 2 enabled payment providers, got %d", len(payments))
	}
	if len(shippings) != 2 {
		t.Fatalf("expected 2 enabled shipping providers, got %d", len(shippings))
	}
	if len(taxes) != 1 {
		t.Fatalf("expected exactly 1 enabled tax provider, got %d", len(taxes))
	}
}

func TestTaxActivationIsExclusive(t *testing.T) {
	manager := NewDefaultManager()

	beforePayments, beforeShippings, beforeTaxes := manager.ListForAdmin()
	_ = beforePayments
	_ = beforeShippings

	activeCount := 0
	var targetID string
	for _, tax := range beforeTaxes {
		if tax.Enabled {
			activeCount++
		} else {
			targetID = tax.ID
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected one active tax provider before update, got %d", activeCount)
	}
	if targetID == "" {
		t.Fatalf("expected at least one disabled tax provider for activation test")
	}

	if err := manager.SetProviderEnabled(ProviderTypeTax, targetID, true); err != nil {
		t.Fatalf("enable tax provider: %v", err)
	}

	_, _, taxes := manager.ListForAdmin()
	activeCount = 0
	for _, tax := range taxes {
		if tax.Enabled {
			activeCount++
			if tax.ID != targetID {
				t.Fatalf("expected %s to be active, got %s", targetID, tax.ID)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected one active tax provider after update, got %d", activeCount)
	}
}

func TestCannotDisableActiveTaxWithoutReplacement(t *testing.T) {
	manager := NewDefaultManager()
	_, _, taxes := manager.ListForAdmin()

	var activeID string
	for _, tax := range taxes {
		if tax.Enabled {
			activeID = tax.ID
			break
		}
	}
	if activeID == "" {
		t.Fatalf("expected active tax provider")
	}

	if err := manager.SetProviderEnabled(ProviderTypeTax, activeID, false); err == nil {
		t.Fatalf("expected error when disabling active tax provider")
	}
}

func TestDisabledProviderCannotBeUsedInQuote(t *testing.T) {
	manager := NewDefaultManager()
	if err := manager.SetProviderEnabled(ProviderTypePayment, "dummy-card", false); err != nil {
		t.Fatalf("disable payment provider: %v", err)
	}

	quote := manager.Quote(QuoteRequest{
		Subtotal:   models.MoneyFromFloat(100),
		PaymentID:  "dummy-card",
		ShippingID: "dummy-ground",
		PaymentData: map[string]string{
			"cardholder_name": "Alex Tester",
			"card_number":     "4242424242424242",
			"exp_month":       "12",
			"exp_year":        "2099",
		},
		ShippingData: map[string]string{
			"full_name":     "Alex Tester",
			"line1":         "123 Market St",
			"city":          "San Francisco",
			"state":         "CA",
			"postal_code":   "94105",
			"country":       "US",
			"service_level": "standard",
		},
		TaxData: map[string]string{"state": "CA"},
	})

	if quote.Valid {
		t.Fatalf("expected quote to be invalid when selected provider is disabled")
	}
}
