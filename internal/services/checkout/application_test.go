package checkout

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func applicationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WebsiteSettings{}, &models.CheckoutSession{}, &models.Cart{}, &models.CartItem{}, &models.IdempotencyKey{}, &models.Product{}, &models.ProductVariant{}))
	return db
}

func TestCartCreatesUserScopedSessionAndRejectsInvalidQuantity(t *testing.T) {
	db := applicationTestDB(t)
	service := NewService(db)
	cart, err := service.Cart(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, uint(42), *cart.CheckoutSession.UserID)
	_, err = service.AddCartItem(context.Background(), 42, 1, 0)
	assert.ErrorIs(t, err, ErrInvalidQuantity)
}

func TestCartHonorsCanceledContext(t *testing.T) {
	db := applicationTestDB(t)
	service := NewService(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Cart(ctx, 1)
	assert.Error(t, err)
}

func TestResolveSessionRotatesInvalidTokenAndPrefersLinkedSession(t *testing.T) {
	db := applicationTestDB(t)
	service := NewService(db)

	guest, err := service.ResolveSession(context.Background(), ResolveSessionInput{Token: "invalid", Create: true})
	require.NoError(t, err)
	require.NotNil(t, guest.Session)
	assert.True(t, guest.SetCookie)
	assert.NotEqual(t, "invalid", guest.Session.PublicToken)

	userID := uint(42)
	linked := models.CheckoutSession{PublicToken: "linked", UserID: &userID, Status: models.CheckoutSessionStatusActive, ExpiresAt: time.Now().UTC().Add(time.Hour), LastSeenAt: time.Now().UTC()}
	require.NoError(t, db.Create(&linked).Error)
	resolved, err := service.ResolveSession(context.Background(), ResolveSessionInput{UserID: userID, Token: guest.Session.PublicToken, Create: true})
	require.NoError(t, err)
	require.NotNil(t, resolved.Session)
	assert.Equal(t, linked.ID, resolved.Session.ID)
	assert.True(t, resolved.SetCookie)
}

func TestIdempotencyReplayAndPayloadMismatch(t *testing.T) {
	db := applicationTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	first, err := service.BeginIdempotency(ctx, 7, "create", "same-key", map[string]string{"email": "one@example.com"}, "correlation")
	require.NoError(t, err)
	require.NotNil(t, first.Record)
	require.NoError(t, service.CompleteIdempotency(ctx, first.Record, 201, map[string]int{"id": 9}))

	replay, err := service.BeginIdempotency(ctx, 7, "create", "same-key", map[string]string{"email": "one@example.com"}, "correlation")
	require.NoError(t, err)
	assert.True(t, replay.Replay)
	assert.Equal(t, 201, replay.Record.ResponseCode)

	_, err = service.BeginIdempotency(ctx, 7, "create", "same-key", map[string]string{"email": "two@example.com"}, "correlation")
	assert.ErrorIs(t, err, ErrIdempotencyConflict)
}

func TestSearchAttributionFailureDoesNotInterruptCart(t *testing.T) {
	db := applicationTestDB(t)
	product := models.Product{Name: "Test product", SKU: "search-attribution-cart", IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{ProductID: product.ID, SKU: "search-attribution-variant", IsPublished: true, Stock: 5}
	require.NoError(t, db.Create(&variant).Error)
	service := NewService(db)
	// This schema deliberately excludes analytics tables, simulating a telemetry failure.
	token := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cart, err := service.AddCartItemWithSearchAttribution(context.Background(), 42, variant.ID, 2, token, "no-such-click")
	require.NoError(t, err)
	require.Len(t, cart.Items, 1)
	require.Equal(t, 2, cart.Items[0].Quantity)
	cart, err = service.AddCartItemWithSearchAttribution(context.Background(), 42, variant.ID, 1, "invalid", "invalid")
	require.NoError(t, err)
	require.Equal(t, 3, cart.Items[0].Quantity)
	_, err = service.AddCartItemWithSearchAttribution(context.Background(), 42, variant.ID, 3, token, "invalid")
	require.ErrorIs(t, err, ErrInvalidQuantity)
	cart, err = service.Cart(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 3, cart.Items[0].Quantity)
}
