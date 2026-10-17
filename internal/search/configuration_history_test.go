package search

import (
	"context"
	"ecommerce/models"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfigurationHistoryRetainsActorsSnapshotsAndDeletedRows(t *testing.T) {
	db := searchTestDB(t)
	s := NewService(db, nil, nil)
	ctx := context.Background()
	actor := uint(7)
	input := SynonymSetInput{Name: "Footwear", Direction: "bi", Terms: []string{"shoe", "boot"}, IsActive: true, UpdatedBy: &actor}
	value, err := s.CreateSynonymSet(ctx, input)
	require.NoError(t, err)
	input.Terms = []string{"shoe", "trainer"}
	_, err = s.UpdateSynonymSet(ctx, value.ID, input)
	require.NoError(t, err)
	require.NoError(t, s.DeleteSynonymSet(ctx, value.ID, &actor))
	rows, total, err := s.ListConfigurationHistory(ctx, "synonym", value.ID, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, rows, 2)
	require.Equal(t, "delete", rows[0].Operation)
	require.Equal(t, &actor, rows[0].ActorID)
	require.Nil(t, rows[0].AfterJSON)
	require.Equal(t, "update", rows[1].Operation)
	var before, after SynonymSet
	require.NoError(t, json.Unmarshal([]byte(*rows[1].BeforeJSON), &before))
	require.NoError(t, json.Unmarshal([]byte(*rows[1].AfterJSON), &after))
	require.Equal(t, []string{"shoe", "boot"}, before.Terms)
	require.Equal(t, []string{"shoe", "trainer"}, after.Terms)
	_, _, err = s.ListConfigurationHistory(ctx, "unknown", 0, 1, 20)
	require.Error(t, err)
	_, _, err = s.ListConfigurationHistory(ctx, "", value.ID, 1, 20)
	require.Error(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.SearchConfigurationAudit{}))
	_, err = s.CreateSynonymSet(ctx, input)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&models.SearchSynonymSet{}).Count(&count).Error)
	require.Zero(t, count, "audit failure must roll back its configuration")
}
func TestConfigurationHistoryIncludesImplicitDefaultAndActiveChanges(t *testing.T) {
	db := searchTestDB(t)
	s := NewService(db, nil, nil)
	ctx := context.Background()
	actor := uint(7)
	var old models.SearchRankingProfile
	require.NoError(t, db.Where("is_default = ?", true).First(&old).Error)
	next, err := s.CreateRankingProfile(ctx, RankingProfileInput{Name: "new default", IsDefault: true, Weights: DefaultRankingWeights()}, &actor)
	require.NoError(t, err)
	rows, _, err := s.ListConfigurationHistory(ctx, "ranking_profile", old.ID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, "update", rows[0].Operation)
	var before, after RankingProfile
	require.NoError(t, json.Unmarshal([]byte(*rows[0].BeforeJSON), &before))
	require.NoError(t, json.Unmarshal([]byte(*rows[0].AfterJSON), &after))
	require.True(t, before.IsDefault)
	require.False(t, after.IsDefault)
	require.Equal(t, before.Version+1, after.Version)
	rows, _, err = s.ListConfigurationHistory(ctx, "ranking_profile", next.ID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, "create", rows[0].Operation)
	nextTypo, err := s.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{Name: "new typo", MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8, IsActive: true, UpdatedBy: &actor})
	require.NoError(t, err)
	rows, total, err := s.ListConfigurationHistory(ctx, "typo_profile", 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	ids := map[uint]string{}
	for _, row := range rows {
		ids[row.EntityID] = row.Operation
		require.Equal(t, &actor, row.ActorID)
	}
	require.Equal(t, "create", ids[nextTypo.ID])
	require.Equal(t, "update", ids[1])
}
