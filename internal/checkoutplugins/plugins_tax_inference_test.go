package checkoutplugins

import (
	"testing"

	"ecommerce/models"
)

func TestQuoteInfersTaxStateFromPickupShippingData(t *testing.T) {
	manager := NewDefaultManager()

	quote := manager.Quote(QuoteRequest{
		Subtotal:   models.MoneyFromFloat(100),
		PaymentID:  "dummy-card",
		ShippingID: "dummy-pickup",
		TaxID:      "dummy-us-tax",
		PaymentData: map[string]string{
			"cardholder_name": "Alex Tester",
			"card_number":     "4242424242424242",
			"exp_month":       "12",
			"exp_year":        "2099",
		},
		ShippingData: map[string]string{
			"pickup_location": "downtown",
			"pickup_contact":  "Alex Tester",
			"state":           "CA",
		},
	})

	if !quote.Valid {
		t.Fatalf("expected quote to be valid, got invalid with tax states: %#v", quote.TaxStates)
	}
	if quote.Tax <= 0 {
		t.Fatalf("expected positive tax for CA, got %s", quote.Tax.String())
	}
}

func TestDummyPickupRequiresState(t *testing.T) {
	manager := NewDefaultManager()

	quote := manager.Quote(QuoteRequest{
		Subtotal:   models.MoneyFromFloat(100),
		PaymentID:  "dummy-card",
		ShippingID: "dummy-pickup",
		TaxID:      "dummy-us-tax",
		PaymentData: map[string]string{
			"cardholder_name": "Alex Tester",
			"card_number":     "4242424242424242",
			"exp_month":       "12",
			"exp_year":        "2099",
		},
		ShippingData: map[string]string{
			"pickup_location": "downtown",
			"pickup_contact":  "Alex Tester",
		},
	})

	if quote.Valid {
		t.Fatalf("expected quote to be invalid when pickup state is missing")
	}
}

func TestQuoteIgnoresClientProvidedTaxProviderID(t *testing.T) {
	manager := NewDefaultManager()

	quote := manager.Quote(QuoteRequest{
		Subtotal:   models.MoneyFromFloat(100),
		PaymentID:  "dummy-card",
		ShippingID: "dummy-ground",
		TaxID:      "client-selected-tax-provider-should-be-ignored",
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
		TaxData: map[string]string{
			"state": "CA",
		},
	})

	if !quote.Valid {
		t.Fatalf("expected quote to be valid, got invalid with tax states: %#v", quote.TaxStates)
	}
	if quote.Tax <= 0 {
		t.Fatalf("expected positive tax, got %s", quote.Tax.String())
	}
	for _, state := range quote.TaxStates {
		if state.Code == "tax_invalid" {
			t.Fatalf("expected no tax_invalid state when client provides tax provider id")
		}
	}
}
