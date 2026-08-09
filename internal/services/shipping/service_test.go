package shipping

import (
	"context"
	"errors"
	"testing"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type quoteBoundaryRegistry struct{ provider ShippingProvider }

func (r quoteBoundaryRegistry) Provider(string) (ShippingProvider, error) { return r.provider, nil }

type quoteBoundaryProvider struct {
	t             *testing.T
	transactional bool
}

func (p *quoteBoundaryProvider) QuoteRates(ctx context.Context, req QuoteRatesRequest) ([]QuotedRate, error) {
	p.transactional = dbcontext.GetDB(ctx) != nil
	return []QuotedRate{{ProviderRateID: "new-rate", ServiceCode: "standard", ServiceName: "Standard", Amount: 500, Currency: req.Currency}}, nil
}
func (*quoteBoundaryProvider) BuyLabel(context.Context, BuyLabelRequest) (ProviderShipment, error) {
	return ProviderShipment{}, nil
}
func (*quoteBoundaryProvider) CancelLabel(context.Context, CancelLabelRequest) (ProviderOperationOutcome, error) {
	return ProviderOperationOutcome{}, nil
}
func (*quoteBoundaryProvider) GetOutcomeByOperationKey(context.Context, string) (ProviderOperationOutcome, error) {
	return ProviderOperationOutcome{}, nil
}
func (*quoteBoundaryProvider) VerifyWebhook(context.Context, map[string]string, []byte) (TrackingWebhookEvent, error) {
	return TrackingWebhookEvent{}, nil
}

func newShippingServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Shipment{}, &models.ShipmentRate{}, &models.ShipmentPackage{}, &models.TrackingEvent{}))
	return db
}

func TestQuoteRatesExecutesOutsideTransactionAndPersistsAtomically(t *testing.T) {
	db := newShippingServiceTestDB(t)
	order := models.Order{}
	order.ID = 10
	snapshot := models.OrderCheckoutSnapshot{ID: 20, ShippingProviderID: "ship", Currency: "USD", ShippingAmount: 500}
	plan, err := PrepareRateQuote(db, order, snapshot)
	require.NoError(t, err)
	provider := &quoteBoundaryProvider{t: t}
	quoted, err := QuotePreparedRates(context.Background(), quoteBoundaryRegistry{provider}, plan)
	require.NoError(t, err)
	require.False(t, provider.transactional)

	var stored []models.ShipmentRate
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var persistErr error
		stored, persistErr = PersistQuotedRates(tx, plan, quoted)
		return persistErr
	}))
	require.Len(t, stored, 1)
	require.NotZero(t, stored[0].ID)
}

func TestPersistQuotedRatesRollsBackReplacementOnFailure(t *testing.T) {
	db := newShippingServiceTestDB(t)
	oldRate := models.ShipmentRate{OrderID: 10, SnapshotID: 20, Provider: "ship", ProviderRateID: "old", ServiceCode: "old", Currency: "USD"}
	require.NoError(t, db.Create(&oldRate).Error)
	callback := "test:fail-rate-replacement"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "shipment_rates" {
			tx.AddError(errors.New("injected rate insert failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })

	order := models.Order{}
	order.ID = 10
	plan := RateQuotePlan{Order: order, Snapshot: models.OrderCheckoutSnapshot{ID: 20, ShippingProviderID: "ship", Currency: "USD"}, ShippingData: map[string]string{}}
	err := db.Transaction(func(tx *gorm.DB) error {
		_, persistErr := PersistQuotedRates(tx, plan, []QuotedRate{{ProviderRateID: "new", ServiceCode: "new", Currency: "USD", ExpiresAt: timePointer(time.Now().UTC())}})
		return persistErr
	})
	require.ErrorContains(t, err, "injected rate insert failure")
	var rates []models.ShipmentRate
	require.NoError(t, db.Order("id").Find(&rates).Error)
	require.Len(t, rates, 1)
	require.Equal(t, "old", rates[0].ProviderRateID)
}

func timePointer(value time.Time) *time.Time { return &value }
