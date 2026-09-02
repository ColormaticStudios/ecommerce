package discounts

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newDiscountPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	baseDSN := os.Getenv("MIGRATIONS_TEST_POSTGRES_DSN")
	if baseDSN == "" {
		t.Skip("set MIGRATIONS_TEST_POSTGRES_DSN to run Postgres discount concurrency tests")
	}
	bootstrap, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("discount_concurrency_%d", time.Now().UTC().UnixNano())
	require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	parsed, err := url.Parse(baseDSN)
	require.NoError(t, err)
	require.NotEmpty(t, parsed.Scheme, "MIGRATIONS_TEST_POSTGRES_DSN must be URL-formatted")
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error })
	require.NoError(t, db.AutoMigrate(&models.DiscountCampaign{}, &models.DiscountRedemption{}))
	return db
}

func TestConcurrentUsageCapAllowsOneRedemptionPostgres(t *testing.T) {
	db := newDiscountPostgresTestDB(t)
	now := time.Now().UTC()
	capValue := 1
	campaign := models.DiscountCampaign{
		Name: "One redemption", Type: models.DiscountCampaignTypeProductDiscount, Status: models.DiscountCampaignStatusActive,
		StartsAt: now.Add(-time.Hour), DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(5), GlobalUsageCap: &capValue,
	}
	require.NoError(t, db.Create(&campaign).Error)
	result := EvaluationResult{Lines: []EvaluatedLine{{
		CartLine:         CartLine{ProductID: 1, ProductVariantID: 1, Quantity: 1},
		AppliedCampaigns: []AppliedCampaign{{ID: campaign.ID, DiscountAmount: models.MoneyFromFloat(5)}},
	}}}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, orderID := range []uint{101, 102} {
		wg.Add(1)
		go func(orderID uint) {
			defer wg.Done()
			<-start
			errs <- db.Transaction(func(tx *gorm.DB) error {
				if err := VerifyUsageCaps(tx, result, nil); err != nil {
					return err
				}
				return RecordRedemptions(tx, orderID, nil, result, now)
			})
		}(orderID)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, capped int
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrUsageCapExceeded) {
			capped++
		} else {
			t.Errorf("unexpected redemption error: %v", err)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, capped)
	var count int64
	require.NoError(t, db.Model(&models.DiscountRedemption{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestConcurrentPerCustomerUsageCapAllowsOneRedemptionPostgres(t *testing.T) {
	db := newDiscountPostgresTestDB(t)
	now := time.Now().UTC()
	capValue := 1
	userID := uint(42)
	campaign := models.DiscountCampaign{
		Name: "One redemption per customer", Type: models.DiscountCampaignTypeProductDiscount, Status: models.DiscountCampaignStatusActive,
		StartsAt: now.Add(-time.Hour), DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(5), PerCustomerUsageCap: &capValue,
	}
	require.NoError(t, db.Create(&campaign).Error)
	result := EvaluationResult{Lines: []EvaluatedLine{{
		CartLine:         CartLine{ProductID: 1, ProductVariantID: 1, Quantity: 1},
		AppliedCampaigns: []AppliedCampaign{{ID: campaign.ID, DiscountAmount: models.MoneyFromFloat(5)}},
	}}}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, orderID := range []uint{201, 202} {
		wg.Add(1)
		go func(orderID uint) {
			defer wg.Done()
			<-start
			errs <- db.Transaction(func(tx *gorm.DB) error {
				if err := VerifyUsageCaps(tx, result, &userID); err != nil {
					return err
				}
				return RecordRedemptions(tx, orderID, &userID, result, now)
			})
		}(orderID)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, capped int
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrUsageCapExceeded) {
			capped++
		} else {
			t.Errorf("unexpected redemption error: %v", err)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, capped)
	var count int64
	require.NoError(t, db.Model(&models.DiscountRedemption{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
