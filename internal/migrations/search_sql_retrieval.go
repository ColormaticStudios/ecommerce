package migrations

import (
	"ecommerce/internal/searchprojection"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode"
)

const searchSQLRetrievalVersion = "2026101003_search_sql_retrieval"

// Frozen legacy document shape: migration replay never selects future fields.
type searchSQLLegacyDocument struct {
	ID              uint
	EntityID        uint
	PayloadJSON     string
	SearchableText  string
	CategoryTokens  string
	BrandSlug       string
	SourceCreatedAt time.Time
	Active          bool
}

func migrateSearchSQLRetrieval(tx *gorm.DB) error {
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_search_documents_eligibility ON search_documents(entity_type, active, entity_id)").Error; err != nil {
		return err
	}
	if err := tx.AutoMigrate(&searchprojection.Rank{}, &searchprojection.Category{}, &searchprojection.Attribute{}, &searchprojection.Term{}); err != nil {
		return err
	}
	for _, sql := range []string{
		"CREATE INDEX IF NOT EXISTS idx_search_categories_lookup ON search_product_categories (slug, product_id)",
		"CREATE INDEX IF NOT EXISTS idx_search_attributes_lookup ON search_product_attributes (slug, value, product_id)",
		"CREATE INDEX IF NOT EXISTS idx_search_terms_lookup ON search_product_terms (term, product_id)",
	} {
		if err := tx.Exec(sql).Error; err != nil {
			return err
		}
	}
	var cursor uint
	for {
		var rows []searchSQLLegacyDocument
		if err := tx.Table("search_documents").Where("entity_type = ? AND id > ?", "product", cursor).Order("id ASC").Limit(250).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			cursor = row.ID
			if normalized := normalizeSearchSQLKey(row.BrandSlug); normalized != row.BrandSlug {
				if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_documents").Where("id = ?", row.ID).Update("brand_slug", normalized).Error; err != nil {
					return err
				}
			}
			if err := searchprojection.Replace(tx, searchprojection.Input{ProductID: row.EntityID, Payload: row.PayloadJSON, Searchable: row.SearchableText, Categories: row.CategoryTokens, Created: row.SourceCreatedAt, Active: row.Active}); err != nil {
				return err
			}
		}
	}
}

// Frozen normalization keeps legacy filter keys consistent with normalized
// postings without invoking current search services during migration replay.
func normalizeSearchSQLKey(value string) string {
	var normalized strings.Builder
	space := true
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			normalized.WriteRune(character)
			space = false
		} else if !space {
			normalized.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(normalized.String())
}
