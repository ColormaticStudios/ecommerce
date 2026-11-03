package migrations

import (
	"crypto/sha256"
	"ecommerce/internal/migrations/ops"
	"encoding/hex"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const searchProjectionRevisionsVersion = "2026101001_search_projection_revisions"

type searchProductRevisionSchema struct {
	ProductID       uint       `gorm:"primaryKey;autoIncrement:false"`
	Revision        int64      `gorm:"not null"`
	IndexedRevision int64      `gorm:"not null;default:0"`
	ChangedAt       time.Time  `gorm:"not null"`
	PendingSince    *time.Time `gorm:"index"`
}

func (searchProductRevisionSchema) TableName() string { return "search_product_revisions" }

func migrateSearchProjectionRevisions(tx *gorm.DB) error {
	if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchProductRevisionSchema{}); err != nil {
		return err
	}
	now := time.Now().UTC()
	// Earlier versions missed related-entity changes. Force one known convergence
	// baseline without inventing a successful projection for existing products.
	if err := tx.Exec(`INSERT INTO search_product_revisions (product_id, revision, indexed_revision, changed_at, pending_since)
 SELECT sources.product_id, CASE WHEN COALESCE(doc.version, 0) >= ? THEN doc.version + 1 ELSE ? END, 0, ?, ?
 FROM (SELECT id AS product_id FROM products UNION SELECT entity_id AS product_id FROM search_documents WHERE entity_type = 'product') sources
 LEFT JOIN search_documents doc ON doc.entity_type = 'product' AND doc.entity_id = sources.product_id
 WHERE 1 = 1 ON CONFLICT (product_id) DO NOTHING`, now.UnixNano(), now.UnixNano(), now, now).Error; err != nil {
		return err
	}
	payload := `{"version":1,"request_id":"search-projection-revisions-2026101001"}`
	sum := sha256.Sum256([]byte(payload))
	row := map[string]any{"id": uuid.NewString(), "created_at": now, "updated_at": now, "job_type": "search.full_reindex", "payload_json": payload, "payload_fingerprint": hex.EncodeToString(sum[:]), "status": "pending", "run_at": now, "idempotency_key": "full:search-projection-revisions-2026101001", "correlation_id": "", "attempt_count": 0, "max_attempts": 5, "lease_owner": "", "last_error": ""}
	return tx.Session(&gorm.Session{NewDB: true}).Table("job_queue").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "job_type"}, {Name: "idempotency_key"}}, DoNothing: true}).Create(row).Error
}
