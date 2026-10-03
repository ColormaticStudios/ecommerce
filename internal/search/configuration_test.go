package search

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

func newConfigurationTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}))
	return NewService(db, nil, nil), db
}

func TestSynonymSetLifecycleNormalizesAndHardDeletes(t *testing.T) {
	service, _ := newConfigurationTestService(t)
	ctx := context.Background()
	actorID := uint(42)

	created, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "  Apparel  ", Direction: SynonymDirectionUnidirectional,
		Terms: []string{" T-Shirt ", "Tee"}, IsActive: false, UpdatedBy: &actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Apparel", created.Name)
	assert.Equal(t, []string{"t shirt", "tee"}, created.Terms)
	assert.False(t, created.IsActive)
	assert.Equal(t, actorID, *created.UpdatedBy)
	assert.False(t, created.CreatedAt.IsZero())

	updated, err := service.UpdateSynonymSet(ctx, created.ID, SynonymSetInput{
		Name: "Apparel", Direction: SynonymDirectionBidirectional,
		Terms: []string{"shirt", "top"}, IsActive: true, UpdatedBy: &actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, SynonymDirectionBidirectional, updated.Direction)
	assert.Equal(t, []string{"shirt", "top"}, updated.Terms)
	assert.True(t, updated.IsActive)

	sets, err := service.ListSynonymSets(ctx)
	require.NoError(t, err)
	require.Len(t, sets, 1)
	assert.Equal(t, updated.ID, sets[0].ID)

	require.NoError(t, service.DeleteSynonymSet(ctx, created.ID))
	_, err = service.GetSynonymSet(ctx, created.ID)
	require.ErrorIs(t, err, ErrConfigurationNotFound)

	recreated, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "Apparel", Direction: SynonymDirectionBidirectional,
		Terms: []string{"shirt", "top"}, IsActive: true,
	})
	require.NoError(t, err, "hard deletion must allow reuse of a unique configuration name")
	assert.NotEqual(t, created.ID, recreated.ID)
}

func TestSynonymSetValidationAndNameConflict(t *testing.T) {
	service, _ := newConfigurationTestService(t)
	ctx := context.Background()

	_, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "invalid", Direction: "both", Terms: []string{"one", "two"}, IsActive: true,
	})
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	_, err = service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "invalid", Direction: SynonymDirectionBidirectional, Terms: []string{"Tee", "tee!"}, IsActive: true,
	})
	require.ErrorIs(t, err, ErrConfigurationInvalid)

	input := SynonymSetInput{
		Name: "sizes", Direction: SynonymDirectionBidirectional,
		Terms: []string{"extra large", "xl"}, IsActive: true,
	}
	_, err = service.CreateSynonymSet(ctx, input)
	require.NoError(t, err)
	_, err = service.CreateSynonymSet(ctx, input)
	require.ErrorIs(t, err, ErrConfigurationConflict)
}

func TestTypoToleranceProfileActivationMaintainsExactlyOneActive(t *testing.T) {
	service, db := newConfigurationTestService(t)
	ctx := context.Background()
	actorID := uint(7)

	first, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 8, StrictMode: false, IsActive: false,
	})
	require.NoError(t, err)
	assert.True(t, first.IsActive, "the first profile is activated even if an inactive profile was requested")

	second, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "strict", MinimumTokenLength: 5, OneEditMinimumLength: 5,
		TwoEditMinimumLength: 9, StrictMode: true, IsActive: false,
	})
	require.NoError(t, err)
	assert.False(t, second.IsActive)

	_, err = service.UpdateTypoToleranceProfile(ctx, first.ID, TypoToleranceProfileInput{
		Name: first.Name, MinimumTokenLength: first.MinimumTokenLength,
		OneEditMinimumLength: first.OneEditMinimumLength, TwoEditMinimumLength: first.TwoEditMinimumLength,
		StrictMode: first.StrictMode, IsActive: false,
	})
	require.ErrorIs(t, err, ErrActiveTypoProfileRequired)
	require.ErrorIs(t, service.DeleteTypoToleranceProfile(ctx, first.ID), ErrActiveTypoProfileRequired)

	activated, err := service.ActivateTypoToleranceProfile(ctx, second.ID, &actorID)
	require.NoError(t, err)
	assert.True(t, activated.IsActive)
	assert.Equal(t, actorID, *activated.UpdatedBy)

	var activeCount int64
	require.NoError(t, db.Model(&models.SearchTypoToleranceProfile{}).Where("is_active = ?", true).Count(&activeCount).Error)
	assert.EqualValues(t, 1, activeCount)
	reloadedFirst, err := service.GetTypoToleranceProfile(ctx, first.ID)
	require.NoError(t, err)
	assert.False(t, reloadedFirst.IsActive)

	_, err = service.ActivateTypoToleranceProfile(ctx, 99999, &actorID)
	require.ErrorIs(t, err, ErrConfigurationNotFound)
	active, err := service.ActiveTypoToleranceProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, second.ID, active.ID, "failed activation must not deactivate the current profile")

	require.NoError(t, service.DeleteTypoToleranceProfile(ctx, first.ID))
	recreated, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 8, StrictMode: false, IsActive: false,
	})
	require.NoError(t, err, "hard deletion must allow reuse of a unique configuration name")
	assert.False(t, recreated.IsActive)
}

func TestTypoToleranceProfileUpdateAndValidation(t *testing.T) {
	service, _ := newConfigurationTestService(t)
	ctx := context.Background()

	_, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "bad", MinimumTokenLength: 4, OneEditMinimumLength: 3,
		TwoEditMinimumLength: 8,
	})
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	_, err = service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "bad", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 4,
	})
	require.ErrorIs(t, err, ErrConfigurationInvalid)

	active, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 8, IsActive: true,
	})
	require.NoError(t, err)
	inactive, err := service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "lenient", MinimumTokenLength: 3, OneEditMinimumLength: 3,
		TwoEditMinimumLength: 7, IsActive: false,
	})
	require.NoError(t, err)

	updated, err := service.UpdateTypoToleranceProfile(ctx, inactive.ID, TypoToleranceProfileInput{
		Name: "lenient", MinimumTokenLength: 2, OneEditMinimumLength: 3,
		TwoEditMinimumLength: 6, StrictMode: true, IsActive: true,
	})
	require.NoError(t, err)
	assert.True(t, updated.IsActive)
	assert.True(t, updated.StrictMode)
	assert.Equal(t, 2, updated.MinimumTokenLength)
	reloadedActive, err := service.GetTypoToleranceProfile(ctx, active.ID)
	require.NoError(t, err)
	assert.False(t, reloadedActive.IsActive)

	_, err = service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "lenient", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 8,
	})
	require.ErrorIs(t, err, ErrConfigurationConflict)
}
