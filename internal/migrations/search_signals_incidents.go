package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"ecommerce/internal/migrations/ops"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const searchSignalsIncidentsVersion = "2026100801_search_signals_incidents"

// Frozen schema definitions keep historical migration replay independent of
// future application model fields.
type searchConversionSchema struct {
	ProductID         uint      `gorm:"primaryKey;autoIncrement:false"`
	Impressions30Days int64     `gorm:"not null;default:0"`
	Conversions30Days int64     `gorm:"not null;default:0"`
	AsOf              time.Time `gorm:"not null;index"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (searchConversionSchema) TableName() string { return "search_conversion_signals" }

type searchIncidentSchema struct {
	ID             uint       `gorm:"primaryKey"`
	IndexName      string     `gorm:"not null;size:64;index"`
	Reason         string     `gorm:"not null;size:32"`
	OpenedAt       time.Time  `gorm:"not null;index"`
	DetectedAt     time.Time  `gorm:"not null"`
	LastObservedAt time.Time  `gorm:"not null"`
	RecoveredAt    *time.Time `gorm:"index"`
	MaxLagSeconds  int64      `gorm:"not null;default:0"`
	MaxPendingJobs int64      `gorm:"not null;default:0"`
	DocumentCount  int64      `gorm:"not null;default:0"`
}

func (searchIncidentSchema) TableName() string { return "search_index_incidents" }

type searchObservationSchema struct {
	IndexName            string `gorm:"primaryKey;size:64"`
	MissingBaselineSince *time.Time
	LastObservedAt       time.Time `gorm:"not null"`
}

func (searchObservationSchema) TableName() string { return "search_freshness_observations" }

func migrateSearchSignalsIncidentsSchema(tx *gorm.DB) error {
	for _, column := range []struct{ table, name, definition string }{
		{"product_variants", "unit_cost", "NUMERIC(12,2)"},
		{"product_variant_drafts", "unit_cost", "NUMERIC(12,2)"},
		{"search_documents", "margin_rate", "REAL NOT NULL DEFAULT 0"},
		{"search_index_states", "last_conversion_refresh_at", "TIMESTAMP"},
	} {
		if err := ops.AddColumnIfNotExists(tx.Session(&gorm.Session{NewDB: true}), column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	for _, model := range []any{&searchConversionSchema{}, &searchIncidentSchema{}, &searchObservationSchema{}} {
		if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), model); err != nil {
			return err
		}
	}
	return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_search_index_incidents_open ON search_index_incidents(index_name) WHERE recovered_at IS NULL").Error
}

func migrateSearchSignalsIncidents(tx *gorm.DB) error {
	if err := migrateSearchSignalsIncidentsSchema(tx); err != nil {
		return err
	}
	var profiles []struct {
		ID          uint
		WeightsJSON string
	}
	if err := tx.Table("search_ranking_profiles").Select("id, weights_json").Scan(&profiles).Error; err != nil {
		return err
	}
	for _, profile := range profiles {
		var weights map[string]json.RawMessage
		if err := json.Unmarshal([]byte(profile.WeightsJSON), &weights); err != nil {
			return err
		}
		if weights == nil {
			weights = map[string]json.RawMessage{}
		}
		changed := false
		for _, name := range []string{"margin", "conversion"} {
			if _, exists := weights[name]; !exists {
				weights[name] = json.RawMessage("0")
				changed = true
			}
		}
		if !changed {
			continue
		}
		encoded, err := json.Marshal(weights)
		if err != nil {
			return err
		}
		if err := tx.Table("search_ranking_profiles").Where("id = ?", profile.ID).Updates(map[string]any{"weights_json": string(encoded), "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
	}
	// A deterministic versioned job rebuilds existing private margin projections.
	// Use frozen queue columns rather than the current JobQueue model.
	now := time.Now().UTC()
	payload := `{"version":1,"request_id":"search-signals-2026100801"}`
	sum := sha256.Sum256([]byte(payload))
	row := map[string]any{"id": uuid.NewString(), "created_at": now, "updated_at": now, "job_type": "search.full_reindex", "payload_json": payload, "payload_fingerprint": hex.EncodeToString(sum[:]), "status": "pending", "run_at": now, "idempotency_key": "full:search-signals-2026100801", "correlation_id": "", "attempt_count": 0, "max_attempts": 5, "lease_owner": "", "last_error": ""}
	if err := tx.Session(&gorm.Session{NewDB: true}).Table("job_queue").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "job_type"}, {Name: "idempotency_key"}}, DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	// Repair source entries on databases bootstrapped by an earlier P4 preview.
	// Existing translations and keys remain unchanged.
	keys := append(append([]localizationP1BaselineKey{}, searchAnalyticsP4CatalogKeys...), searchSignalsCatalogKeys...)
	for _, key := range keys {
		row := map[string]any{"namespace": key.Namespace, "key": key.Key, "source_text": key.SourceText, "description": "Search ranking and private variant cost", "owner_domain": key.OwnerDomain, "is_deprecated": false, "created_at": now, "updated_at": now}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("translation_keys").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "namespace"}, {Name: "key"}}, DoNothing: true}).Create(row).Error; err != nil {
			return err
		}
	}
	return publishLocalizationDefaultSourceCatalog(tx)
}
