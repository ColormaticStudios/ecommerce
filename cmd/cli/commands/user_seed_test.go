package commands

import (
	"testing"

	accountdataservice "ecommerce/internal/services/accountdata"
	"ecommerce/models"
)

func TestCreateSavedAddressValidatesAndSetsDefault(t *testing.T) {
	db := newTestDB(t, &models.SavedAddress{})
	service := accountdataservice.NewService(db)

	if _, err := service.CreateSavedAddress(1, accountdataservice.CreateSavedAddressInput{
		FullName:   "",
		Line1:      "123 Main St",
		City:       "Portland",
		PostalCode: "97201",
		Country:    "USA",
	}); err == nil {
		t.Fatal("expected invalid address error")
	}

	first, err := service.CreateSavedAddress(1, accountdataservice.CreateSavedAddressInput{
		Label:      "Home",
		FullName:   "Ada Lovelace",
		Line1:      "123 Main St",
		City:       "Portland",
		PostalCode: "97201",
		Country:    "US",
	})
	if err != nil {
		t.Fatalf("create first address: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("expected first address to be default")
	}

	second, err := service.CreateSavedAddress(1, accountdataservice.CreateSavedAddressInput{
		Label:      "Office",
		FullName:   "Ada Lovelace",
		Line1:      "500 Market St",
		City:       "San Francisco",
		PostalCode: "94105",
		Country:    "US",
		SetDefault: true,
	})
	if err != nil {
		t.Fatalf("create second address: %v", err)
	}
	if !second.IsDefault {
		t.Fatal("expected second address to be default")
	}

	var refreshedFirst models.SavedAddress
	if err := db.First(&refreshedFirst, first.ID).Error; err != nil {
		t.Fatalf("reload first address: %v", err)
	}
	if refreshedFirst.IsDefault {
		t.Fatal("expected first address default flag to be cleared")
	}
}

func TestCreateSavedPaymentMethodValidatesAndSetsDefault(t *testing.T) {
	db := newTestDB(t, &models.SavedPaymentMethod{})
	service := accountdataservice.NewService(db)

	if _, err := service.CreateSavedPaymentMethod(1, accountdataservice.CreateSavedPaymentMethodInput{
		CardholderName: "Ada Lovelace",
		CardNumber:     "1234",
		ExpMonth:       12,
		ExpYear:        2030,
	}); err == nil {
		t.Fatal("expected invalid card number error")
	}

	first, err := service.CreateSavedPaymentMethod(1, accountdataservice.CreateSavedPaymentMethodInput{
		CardholderName: "Ada Lovelace",
		CardNumber:     "4111 1111 1111 1111",
		ExpMonth:       12,
		ExpYear:        2030,
	})
	if err != nil {
		t.Fatalf("create first card: %v", err)
	}
	if first.Brand != "Visa" {
		t.Fatalf("expected Visa brand, got %q", first.Brand)
	}
	if !first.IsDefault {
		t.Fatal("expected first card to be default")
	}

	second, err := service.CreateSavedPaymentMethod(1, accountdataservice.CreateSavedPaymentMethodInput{
		CardholderName: "Ada Lovelace",
		CardNumber:     "5555 5555 5555 4444",
		ExpMonth:       1,
		ExpYear:        2032,
		SetDefault:     true,
	})
	if err != nil {
		t.Fatalf("create second card: %v", err)
	}
	if !second.IsDefault {
		t.Fatal("expected second card to be default")
	}

	var refreshedFirst models.SavedPaymentMethod
	if err := db.First(&refreshedFirst, first.ID).Error; err != nil {
		t.Fatalf("reload first card: %v", err)
	}
	if refreshedFirst.IsDefault {
		t.Fatal("expected first card default flag to be cleared")
	}
}
