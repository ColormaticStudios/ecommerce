package migrations

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// These schemas capture P0 and P2 explicitly; runtime model changes must not
// change the historical setup or assertions in these replay tests.
type frozenSearchP0RankingProfile struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Name        string         `gorm:"not null;size:120;uniqueIndex"`
	WeightsJSON string         `gorm:"type:text;not null"`
	IsDefault   bool           `gorm:"not null;default:false;index"`
	UpdatedBy   *uint          `gorm:"index"`
}

func (frozenSearchP0RankingProfile) TableName() string { return "search_ranking_profiles" }

type frozenSearchP2RankingProfile struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Name        string         `gorm:"not null;size:120;uniqueIndex"`
	WeightsJSON string         `gorm:"type:text;not null"`
	IsDefault   bool           `gorm:"not null;default:false;index"`
	UpdatedBy   *uint          `gorm:"index"`
	Version     int            `gorm:"not null;default:1"`
}

func (frozenSearchP2RankingProfile) TableName() string { return "search_ranking_profiles" }

type frozenSearchP2SalesSignal struct {
	ProductID   uint      `gorm:"primaryKey;autoIncrement:false"`
	Units30Days int64     `gorm:"not null;default:0"`
	AsOf        time.Time `gorm:"not null;index"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (frozenSearchP2SalesSignal) TableName() string { return "search_sales_signals" }

func TestSearchRankingP2ReplaysLegacyProfilesAndPreservesConfiguration(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		profiles    []frozenSearchP0RankingProfile
		defaultName string
	}{
		{name: "empty", defaultName: "default"},
		{name: "inactive", profiles: []frozenSearchP0RankingProfile{{Name: "custom", WeightsJSON: `{"name":3}`}}, defaultName: "default"},
		{name: "existing_default", profiles: []frozenSearchP0RankingProfile{{Name: "custom", WeightsJSON: `{"name":3}`, IsDefault: true}}, defaultName: "custom"},
		{name: "duplicate_defaults", profiles: []frozenSearchP0RankingProfile{{Name: "first", WeightsJSON: `{"name":3}`, IsDefault: true}, {Name: "second", WeightsJSON: `{"name":5}`, IsDefault: true}}, defaultName: "first"},
		{name: "deleted_default", profiles: []frozenSearchP0RankingProfile{{Name: "archived", WeightsJSON: `{"name":3}`, IsDefault: true, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}}, defaultName: "default"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := newTestDB(t)
			require.NoError(t, db.AutoMigrate(&frozenSearchP0RankingProfile{}))
			require.False(t, db.Migrator().HasColumn(&frozenSearchP0RankingProfile{}, "version"))
			for i := range scenario.profiles {
				require.NoError(t, db.Create(&scenario.profiles[i]).Error)
			}
			require.NoError(t, db.Transaction(migrateSearchRankingP2))
			require.NoError(t, db.Transaction(migrateSearchRankingP2))
			var profiles []frozenSearchP2RankingProfile
			require.NoError(t, db.Order("id").Find(&profiles).Error)
			defaults := 0
			byName := map[string]frozenSearchP2RankingProfile{}
			for _, profile := range profiles {
				byName[profile.Name] = profile
				require.Equal(t, 1, profile.Version)
				if profile.IsDefault {
					defaults++
					require.Equal(t, scenario.defaultName, profile.Name)
				}
			}
			require.Equal(t, 1, defaults)
			require.Contains(t, byName, "default")
			require.Contains(t, byName, "new_arrivals")
			for _, legacy := range scenario.profiles {
				if !legacy.DeletedAt.Valid {
					require.Equal(t, legacy.WeightsJSON, byName[legacy.Name].WeightsJSON)
					require.Equal(t, legacy.ID, byName[legacy.Name].ID)
				}
			}
			for _, name := range []string{"default", "new_arrivals"} {
				var weights map[string]float64
				require.NoError(t, json.Unmarshal([]byte(byName[name].WeightsJSON), &weights))
				require.Len(t, weights, 8)
				require.Positive(t, weights["token_coverage"])
			}
			require.NoError(t, db.Create(&frozenSearchP2SalesSignal{ProductID: 7, Units30Days: 12, AsOf: time.Now(), UpdatedAt: time.Now()}).Error)
			require.NoError(t, db.Transaction(migrateSearchRankingP2))
			var signal frozenSearchP2SalesSignal
			require.NoError(t, db.First(&signal, "product_id = ?", 7).Error)
			require.EqualValues(t, 12, signal.Units30Days)
			require.Error(t, db.Exec("UPDATE search_ranking_profiles SET is_default = true WHERE name = ?", "new_arrivals").Error)
			// Replaying the migration must not reset runtime profile versions.
			require.NoError(t, db.Exec("UPDATE search_ranking_profiles SET version = 7 WHERE name = ?", "new_arrivals").Error)
			require.NoError(t, db.Transaction(migrateSearchRankingP2))
			var preserved frozenSearchP2RankingProfile
			require.NoError(t, db.First(&preserved, "name = ?", "new_arrivals").Error)
			require.Equal(t, 7, preserved.Version)
		})
	}
}

func TestSearchRankingP2RevivesDeletedReservedSeeds(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&frozenSearchP0RankingProfile{}))
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	actor := uint(17)
	for _, name := range []string{"default", "new_arrivals"} {
		require.NoError(t, db.Create(&frozenSearchP0RankingProfile{Name: name, WeightsJSON: `{}`, CreatedAt: createdAt, UpdatedBy: &actor, IsDefault: true, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}).Error)
	}
	require.NoError(t, db.Transaction(migrateSearchRankingP2))
	require.NoError(t, db.Transaction(migrateSearchRankingP2))
	var profiles []frozenSearchP2RankingProfile
	require.NoError(t, db.Order("id").Find(&profiles).Error)
	require.Len(t, profiles, 2)
	for index, profile := range profiles {
		require.EqualValues(t, index+1, profile.ID)
		require.Equal(t, createdAt, profile.CreatedAt.UTC())
		require.NotNil(t, profile.UpdatedBy)
		require.Equal(t, actor, *profile.UpdatedBy)
		require.False(t, profile.DeletedAt.Valid)
		require.Equal(t, 1, profile.Version)
		var weights map[string]float64
		require.NoError(t, json.Unmarshal([]byte(profile.WeightsJSON), &weights))
		require.Len(t, weights, 8)
		require.Equal(t, profile.Name == "default", profile.IsDefault)
	}
	require.Error(t, db.Exec("UPDATE search_ranking_profiles SET is_default = true WHERE name = ?", "new_arrivals").Error)
}

func TestSearchRankingP2MigrationTransactionRollsBack(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&frozenSearchP0RankingProfile{}))
	failure := errors.New("abort after backfill")
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := migrateSearchRankingP2(tx); err != nil {
			return err
		}
		return failure
	}), failure)
	require.False(t, db.Migrator().HasColumn(&frozenSearchP0RankingProfile{}, "version"))
	require.False(t, db.Migrator().HasTable(&frozenSearchP2SalesSignal{}))
	var count int64
	require.NoError(t, db.Unscoped().Model(&frozenSearchP0RankingProfile{}).Count(&count).Error)
	require.Zero(t, count)
}
