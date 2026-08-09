package accountdata

import (
	"context"
	"fmt"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAccountDataTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SavedAddress{}, &models.SavedPaymentMethod{}))
	return db
}

func TestContextAddressLifecycle(t *testing.T) {
	service := NewService(newAccountDataTestDB(t))
	_, err := service.CreateSavedAddress(context.Background(), 1, CreateSavedAddressInput{Country: "US"})
	require.ErrorIs(t, err, ErrInvalidAddress)

	address, err := service.CreateSavedAddress(context.Background(), 1, CreateSavedAddressInput{FullName: "Ada", Line1: "123 Main", City: "Portland", PostalCode: "97201", Country: "us"})
	require.NoError(t, err)
	assert.True(t, address.IsDefault)
	assert.Equal(t, "US", address.Country)

	addresses, err := service.ListSavedAddresses(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, addresses, 1)
	require.NoError(t, service.DeleteSavedAddress(context.Background(), 1, address.ID))
	require.ErrorIs(t, service.DeleteSavedAddress(context.Background(), 1, address.ID), ErrAddressNotFound)
}

func TestContextPaymentMethodValidationAndOwnership(t *testing.T) {
	service := NewService(newAccountDataTestDB(t))
	_, err := service.CreateSavedPaymentMethod(context.Background(), 1, CreateSavedPaymentMethodInput{CardholderName: "Ada", CardNumber: "123", ExpMonth: 12, ExpYear: 2030})
	require.ErrorIs(t, err, ErrInvalidCardNumber)

	method, err := service.CreateSavedPaymentMethod(context.Background(), 1, CreateSavedPaymentMethodInput{CardholderName: "Ada", CardNumber: "4111 1111 1111 1111", ExpMonth: 12, ExpYear: 2030})
	require.NoError(t, err)
	assert.Equal(t, "Visa", method.Brand)
	require.ErrorIs(t, service.DeleteSavedPaymentMethod(context.Background(), 2, method.ID), ErrPaymentMethodNotFound)
}

func TestAccountDataServiceUsesCallerContext(t *testing.T) {
	service := NewService(newAccountDataTestDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.ListSavedAddresses(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
}
