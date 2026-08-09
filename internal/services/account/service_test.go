package account

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

func newAccountTestDB(t *testing.T, values ...any) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(values...))
	return db
}

func TestProfileAndAdminUserOperations(t *testing.T) {
	db := newAccountTestDB(t, &models.User{})
	user := models.User{Subject: "subject-1", Username: "ada", Email: "ada@example.com", Role: "customer", Currency: "USD"}
	require.NoError(t, db.Create(&user).Error)
	service := NewService(db, nil)

	currency := "EUR"
	name := "Ada Lovelace"
	updated, err := service.UpdateProfile(context.Background(), user.Subject, UpdateProfileInput{Name: &name, Currency: &currency})
	require.NoError(t, err)
	assert.Equal(t, name, updated.Name)
	assert.Equal(t, currency, updated.Currency)

	invalidCurrency := "ZZZ"
	_, err = service.UpdateProfile(context.Background(), user.Subject, UpdateProfileInput{Currency: &invalidCurrency})
	require.ErrorIs(t, err, ErrInvalidCurrency)

	page, err := service.ListUsers(context.Background(), ListUsersInput{Page: 1, Limit: 10, Query: "ada@example"})
	require.NoError(t, err)
	require.Len(t, page.Users, 1)

	promoted, err := service.UpdateUserRole(context.Background(), user.ID, "admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", promoted.Role)
	_, err = service.UpdateUserRole(context.Background(), user.ID, "owner")
	require.ErrorIs(t, err, ErrInvalidRole)
}

func TestServiceUsesCallerContext(t *testing.T) {
	db := newAccountTestDB(t, &models.User{})
	service := NewService(db, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.ListUsers(ctx, ListUsersInput{Page: 1, Limit: 10})
	require.ErrorIs(t, err, context.Canceled)
}

func TestWebsiteSettingsDefaultsAndRejectsUnconfiguredSecretEncryption(t *testing.T) {
	db := newAccountTestDB(t, &models.WebsiteSettings{})
	service := NewService(db, nil)
	settings, err := service.GetWebsiteSettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Ecommerce", settings.SiteTitle)
	assert.True(t, settings.AllowGuestCheckout)

	_, err = service.UpdateWebsiteSettings(context.Background(), WebsiteSettingsInput{SiteTitle: "Shop", OIDCClientSecret: "secret"})
	require.ErrorIs(t, err, ErrCredentialServiceUnconfigured)
}
