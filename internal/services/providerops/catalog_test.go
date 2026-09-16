package providerops

import (
	"context"
	"testing"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestUpsertCheckoutProviderSettingPersistsExplicitlyDisabledStateOnFirstInsert(t *testing.T) {
	db := newProviderOpsTestDB(t, &models.CheckoutProviderSetting{})
	ctx := context.Background()

	setting := checkoutplugins.ProviderSetting{Type: checkoutplugins.ProviderTypePayment, ID: "disabled-provider", Enabled: false}
	require.NoError(t, upsertCheckoutProviderSetting(ctx, db, setting))

	var record models.CheckoutProviderSetting
	require.NoError(t, db.Where("provider_type = ? AND provider_id = ?", string(setting.Type), setting.ID).First(&record).Error)
	require.False(t, record.Enabled, "a provider disabled on first insert must persist as disabled")

	// Re-applying the same disabled setting through the upsert (conflict) path must
	// also remain disabled.
	require.NoError(t, upsertCheckoutProviderSetting(ctx, db, setting))
	require.NoError(t, db.Where("provider_type = ? AND provider_id = ?", string(setting.Type), setting.ID).First(&record).Error)
	require.False(t, record.Enabled)

	// Flipping it enabled and back to disabled through the upsert path must also persist correctly.
	enabled := setting
	enabled.Enabled = true
	require.NoError(t, upsertCheckoutProviderSetting(ctx, db, enabled))
	require.NoError(t, db.Where("provider_type = ? AND provider_id = ?", string(setting.Type), setting.ID).First(&record).Error)
	require.True(t, record.Enabled)

	require.NoError(t, upsertCheckoutProviderSetting(ctx, db, setting))
	require.NoError(t, db.Where("provider_type = ? AND provider_id = ?", string(setting.Type), setting.ID).First(&record).Error)
	require.False(t, record.Enabled)
}

func TestUpsertDisabledProviderNeverInsertsEnabledState(t *testing.T) {
	db := newProviderOpsTestDB(t, &models.CheckoutProviderSetting{})
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_enabled_provider BEFORE INSERT ON checkout_provider_settings WHEN NEW.enabled = 1 BEGIN SELECT RAISE(ABORT, 'unexpected enabled insert'); END`).Error)
	setting := checkoutplugins.ProviderSetting{Type: checkoutplugins.ProviderTypePayment, ID: "disabled-provider", Enabled: false}
	require.NoError(t, upsertCheckoutProviderSetting(context.Background(), db, setting))
	require.NoError(t, upsertCheckoutProviderSetting(context.Background(), db, setting))
	var record models.CheckoutProviderSetting
	require.NoError(t, db.First(&record).Error)
	require.False(t, record.Enabled)
	require.False(t, record.CreatedAt.IsZero())
	require.False(t, record.UpdatedAt.IsZero())
}
