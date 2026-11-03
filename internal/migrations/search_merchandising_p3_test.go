package migrations

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Historical setup and post-migration assertions use frozen types, never
// runtime models, so future search schema changes cannot drift replay tests.
type frozenSearchP0MerchandisingRule struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	Name          string         `gorm:"not null;size:120"`
	RuleType      string         `gorm:"not null;size:32;index"`
	PredicateJSON string         `gorm:"type:text;not null"`
	ActionJSON    string         `gorm:"type:text;not null"`
	Priority      int            `gorm:"not null;default:0;index"`
	StartsAt      *time.Time     `gorm:"index"`
	EndsAt        *time.Time     `gorm:"index"`
	IsActive      bool           `gorm:"not null;default:true;index"`
	UpdatedBy     *uint          `gorm:"index"`
}

func (frozenSearchP0MerchandisingRule) TableName() string { return "search_merchandising_rules" }

type frozenSearchP3MerchandisingRule struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	Name          string         `gorm:"not null;size:120"`
	RuleType      string         `gorm:"not null;size:32;index"`
	PredicateJSON string         `gorm:"type:text;not null"`
	ActionJSON    string         `gorm:"type:text;not null"`
	Priority      int            `gorm:"not null;default:0;index"`
	StartsAt      *time.Time     `gorm:"index"`
	EndsAt        *time.Time     `gorm:"index"`
	IsActive      bool           `gorm:"not null;default:true;index"`
	UpdatedBy     *uint          `gorm:"index"`

	Version int `gorm:"not null;default:1"`
}

func (frozenSearchP3MerchandisingRule) TableName() string { return "search_merchandising_rules" }

type frozenSearchP3MerchandisingAudit struct {
	ID         uint      `gorm:"primaryKey"`
	RuleID     uint      `gorm:"not null;index"`
	Operation  string    `gorm:"not null;size:16"`
	ActorID    *uint     `gorm:"index"`
	BeforeJSON *string   `gorm:"type:text"`
	AfterJSON  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

func (frozenSearchP3MerchandisingAudit) TableName() string { return "search_merchandising_audits" }

func TestSearchMerchandisingP3PreservesLegacyRulesAndVersions(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&frozenSearchP0MerchandisingRule{}))
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	actor := uint(19)
	end := now.AddDate(0, 1, 0)
	legacy := frozenSearchP0MerchandisingRule{ID: 7, CreatedAt: now, UpdatedAt: now, Name: "legacy", RuleType: "reserved", PredicateJSON: "invalid reserved JSON", ActionJSON: `{"legacy":true}`, Priority: 12, StartsAt: &now, EndsAt: &end, UpdatedBy: &actor}
	require.NoError(t, db.Create(&legacy).Error)
	require.NoError(t, db.Exec("UPDATE search_merchandising_rules SET is_active = false WHERE id = ?", legacy.ID).Error)
	deleted := frozenSearchP0MerchandisingRule{ID: 8, Name: "archived", RuleType: "reserved", PredicateJSON: `{}`, ActionJSON: `{}`, DeletedAt: gorm.DeletedAt{Time: now, Valid: true}}
	require.NoError(t, db.Create(&deleted).Error)
	require.False(t, db.Migrator().HasColumn(&frozenSearchP0MerchandisingRule{}, "version"))
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3Schema))
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3Schema))
	var rules []frozenSearchP3MerchandisingRule
	require.NoError(t, db.Unscoped().Order("id ASC").Find(&rules).Error)
	require.Len(t, rules, 2, "no example or fallback rules should be seeded")
	require.Equal(t, legacy.ID, rules[0].ID)
	require.Equal(t, legacy.Name, rules[0].Name)
	require.Equal(t, legacy.RuleType, rules[0].RuleType)
	require.Equal(t, legacy.PredicateJSON, rules[0].PredicateJSON)
	require.Equal(t, legacy.ActionJSON, rules[0].ActionJSON)
	require.Equal(t, legacy.Priority, rules[0].Priority)
	require.Equal(t, now, rules[0].CreatedAt.UTC())
	require.Equal(t, now, rules[0].UpdatedAt.UTC())
	require.True(t, now.Equal(*rules[0].StartsAt))
	require.True(t, end.Equal(*rules[0].EndsAt))
	require.Equal(t, &actor, rules[0].UpdatedBy)
	require.False(t, rules[0].IsActive)
	require.Equal(t, 1, rules[0].Version)
	require.True(t, rules[1].DeletedAt.Valid)
	require.Equal(t, 1, rules[1].Version)
	require.NoError(t, db.Exec("UPDATE search_merchandising_rules SET version = 5 WHERE id = ?", legacy.ID).Error)
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3Schema))
	var preserved frozenSearchP3MerchandisingRule
	require.NoError(t, db.First(&preserved, legacy.ID).Error)
	require.Equal(t, 5, preserved.Version)
	var auditCount int64
	require.NoError(t, db.Model(&frozenSearchP3MerchandisingAudit{}).Count(&auditCount).Error)
	require.Zero(t, auditCount, "schema migration must not invent historical audit events")
}

func TestSearchMerchandisingP3AuditSurvivesRuleDeletionAndReplay(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&frozenSearchP0MerchandisingRule{}))
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3Schema))
	rule := frozenSearchP3MerchandisingRule{Name: "temporary", RuleType: "hide", PredicateJSON: `{}`, ActionJSON: `{}`}
	require.NoError(t, db.Create(&rule).Error)
	actor := uint(999)
	before := `{"id":1,"name":"temporary","version":1}`
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	event := frozenSearchP3MerchandisingAudit{RuleID: rule.ID, Operation: "delete", ActorID: &actor, BeforeJSON: &before, CreatedAt: now}
	require.NoError(t, db.Create(&event).Error)
	require.NoError(t, db.Unscoped().Delete(&rule).Error)
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3Schema))
	var retained frozenSearchP3MerchandisingAudit
	require.NoError(t, db.First(&retained, event.ID).Error)
	require.Equal(t, event.RuleID, retained.RuleID)
	require.Equal(t, event.Operation, retained.Operation)
	require.Equal(t, event.ActorID, retained.ActorID)
	require.Equal(t, event.BeforeJSON, retained.BeforeJSON)
	require.Nil(t, retained.AfterJSON)
	require.Equal(t, now, retained.CreatedAt.UTC())
	var foreignKeys []struct{ Table string }
	require.NoError(t, db.Raw("PRAGMA foreign_key_list(search_merchandising_audits)").Scan(&foreignKeys).Error)
	require.Empty(t, foreignKeys, "neither historical rule nor actor IDs may have a cascading foreign key")
	for _, index := range []string{"idx_search_merchandising_audits_rule_id", "idx_search_merchandising_audits_actor_id", "idx_search_merchandising_audits_created_at"} {
		require.True(t, db.Migrator().HasIndex(&frozenSearchP3MerchandisingAudit{}, index))
	}
}

func TestSearchMerchandisingP3MigrationRollsBackAtomically(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&frozenSearchP0MerchandisingRule{}))
	failure := errors.New("abort merchandising migration")
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := migrateSearchMerchandisingP3Schema(tx); err != nil {
			return err
		}
		return failure
	}), failure)
	require.False(t, db.Migrator().HasColumn(&frozenSearchP0MerchandisingRule{}, "version"))
	require.False(t, db.Migrator().HasTable(&frozenSearchP3MerchandisingAudit{}))
}

func TestSearchMerchandisingP3OrderedMigrationReplay(t *testing.T) {
	db := newTestDB(t)
	index := slices.IndexFunc(orderedMigrations, func(m Migration) bool { return m.Version == searchMerchandisingP3Version })
	require.Positive(t, index)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:index]))
	require.False(t, db.Migrator().HasColumn(&frozenSearchP0MerchandisingRule{}, "version"))
	require.False(t, db.Migrator().HasTable(&frozenSearchP3MerchandisingAudit{}))
	require.NoError(t, Run(db))
	require.NoError(t, Run(db))
	require.NoError(t, Check(db))
	require.True(t, db.Migrator().HasColumn(&frozenSearchP3MerchandisingRule{}, "version"))
	require.True(t, db.Migrator().HasTable(&frozenSearchP3MerchandisingAudit{}))
	var released int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM translation_keys AS keys
 JOIN translation_values AS values_ ON values_.translation_key_id = keys.id
 JOIN translation_release_entries AS entries ON entries.translation_value_id = values_.id
 JOIN translation_releases AS releases ON releases.id = entries.release_id
 JOIN locales ON locales.id = values_.locale_id
 WHERE keys.namespace = ? AND keys.key = ? AND keys.source_text = ?
 AND values_.value = ? AND values_.state = ? AND releases.status = ? AND locales.is_default = true`,
		"admin", "navigation.search_merchandising", "Search merchandising", "Search merchandising", "published", "active").Scan(&released).Error)
	require.EqualValues(t, 1, released, "new navigation source must be in the active default-locale release")
	require.NoError(t, db.Transaction(migrateSearchMerchandisingP3))
	var keys int64
	require.NoError(t, db.Table("translation_keys").Where("namespace = ? AND key = ?", "admin", "navigation.search_merchandising").Count(&keys).Error)
	require.EqualValues(t, 1, keys)
}
