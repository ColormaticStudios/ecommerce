package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	searchservice "ecommerce/internal/search"
	checkoutservice "ecommerce/internal/services/checkout"
	"ecommerce/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSearchAttributionFollowsCartAndPaidOrder(t *testing.T) {
	db := newOrdersTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WebsiteSettings{}, &models.Cart{}, &models.CartItem{}, &models.SearchQueryEvent{}, &models.SearchClickEvent{}, &models.SearchCartAttribution{}, &models.SearchOrderAttribution{}, &models.SearchRevokedSession{}, &models.SearchIndexState{}, &models.SearchConversionSignal{}))
	variant := seedVariant(t, db, "attributed-product", 5)
	userID := uint(42)
	ctx := context.Background()
	checkout := checkoutservice.NewService(db)
	cart, err := checkout.Cart(ctx, userID)
	require.NoError(t, err)
	// The ingestion tests cover snapshot verification. This test starts with an
	// accepted click and exercises the actual commerce service hooks.
	raw := make([]byte, 32)
	raw[0] = 1
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	impression := models.SearchQueryEvent{ImpressionID: uuid.NewString(), SessionHash: hex.EncodeToString(hash[:]), Query: "product", NormalizedQuery: "product", FiltersJSON: "{}", ProductsJSON: "[]", ResultCount: 1, CreatedAt: time.Now().UTC()}
	require.NoError(t, db.Create(&impression).Error)
	click := models.SearchClickEvent{EventID: uuid.NewString(), ImpressionID: impression.ImpressionID, SessionHash: impression.SessionHash, QueryEventID: impression.ID, ProductID: variant.ProductID, Position: 1, ClickedAt: time.Now().UTC()}
	require.NoError(t, db.Create(&click).Error)
	cart, err = checkout.AddCartItemWithSearchAttribution(ctx, userID, variant.ID, 1, token, click.EventID)
	require.NoError(t, err)
	var cartAttribution models.SearchCartAttribution
	require.NoError(t, db.First(&cartAttribution, "cart_item_id = ?", cart.Items[0].ID).Error)
	require.Equal(t, click.EventID, cartAttribution.ClickID)
	orders := NewService(db)
	order, err := orders.Create(ctx, cart.CheckoutSessionID, &userID, nil, []CreateItemInput{{ProductVariantID: variant.ID, Quantity: 1}})
	require.NoError(t, err)
	var attribution models.SearchOrderAttribution
	require.NoError(t, db.First(&attribution, "order_id = ?", order.ID).Error)
	require.Nil(t, attribution.PaidAt)
	_, err = orders.UpdateStatus(ctx, order.ID, models.StatusPaid)
	require.NoError(t, err)
	require.NoError(t, db.First(&attribution, attribution.ID).Error)
	require.NotNil(t, attribution.PaidAt)
	paidAt := *attribution.PaidAt
	_, err = orders.UpdateStatus(ctx, order.ID, models.StatusPaid)
	require.NoError(t, err)
	require.NoError(t, db.First(&attribution, attribution.ID).Error)
	require.Equal(t, paidAt, *attribution.PaidAt)
	// Withdrawal removes the linkage and prevents subsequent cart attribution.
	require.NoError(t, searchservice.NewService(db, nil, nil).RevokeConsent(ctx, token))
	cart, err = checkout.AddCartItemWithSearchAttribution(ctx, userID, variant.ID, 1, token, click.EventID)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SearchCartAttribution{}).Count(&count).Error)
	require.Zero(t, count)
}
