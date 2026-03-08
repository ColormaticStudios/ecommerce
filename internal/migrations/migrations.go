package migrations

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/migrations/ops"
	"ecommerce/models"

	"gorm.io/gorm"
)

// SchemaMigration tracks applied migration versions.
type SchemaMigration struct {
	Version       string    `gorm:"primaryKey;size:64"`
	Name          string    `gorm:"not null;size:255;default:''"`
	Checksum      string    `gorm:"not null;size:64;default:''"`
	AppliedAt     time.Time `gorm:"not null"`
	DurationMs    int64     `gorm:"not null;default:0"`
	ExecutionMeta string    `gorm:"type:text;not null;default:''"`
}

type TransactionMode string

const (
	TransactionModeRequired TransactionMode = "required"
	TransactionModeNone     TransactionMode = "none"
)

type PostCheck struct {
	Name  string
	Check func(tx *gorm.DB) error
}

type Migration struct {
	Version          string
	Name             string
	Up               func(tx *gorm.DB) error
	TransactionMode  TransactionMode
	PostChecks       []PostCheck
	Tags             []string
	ContractBlockers []string
}

type Status struct {
	LatestKnownVersion   string
	LatestAppliedVersion string
	PendingCount         int
}

const advisoryLockKey int64 = 2172384190179656700
const migrationContractTag = "contract"
const contractGuardEnvVar = "MIGRATIONS_ALLOW_CONTRACT"
const migrationChecksumCutoverVersion = productPublishBackfillVersion
const defaultSchemaSnapshotPath = "internal/migrations/schema_snapshot.sql"
const initialSchemaVersion = "2026022601_initial_schema"
const productPublishBackfillVersion = "2026030501_backfill_product_publish_state"
const productCatalogDepthP0Version = "2026030601_catalog_depth_p0"
const productCatalogDepthP2Version = "2026030602_catalog_depth_p2_variant_checkout"
const productCatalogDepthP2ProductBackfillVersion = "2026030603_catalog_depth_p2_backfill_missing_variants"
const productCatalogDepthP4Version = "2026030701_catalog_depth_p4_hardening"
const migrationStepAlertThresholdEnvVar = "MIGRATIONS_STEP_ALERT_THRESHOLD_MS"

var versionPattern = regexp.MustCompile(`^\d{10}_[a-z0-9_]+$`)
var tagPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

var migrationChecksumCompatibilityBackfills = map[string][]string{
	// 2026030602 was corrected to remove a direct AutoMigrate call after it had
	// already been applied in development databases. Accept the prior checksum
	// once so those databases can backfill to the fixed definition.
	productCatalogDepthP2Version: {
		"5a483908a1331a23cfcfa2ab5b4992a5f63fd50e7cef4f732013328f23ca4329",
	},
}

var acquireMigrationLock = acquireMigrationLockForDB
var migrationSourcePath = "internal/migrations/migrations.go"

//go:embed migrations.go
var embeddedMigrationSource []byte

var orderedMigrations = []Migration{
	{
		Version:         initialSchemaVersion,
		Name:            "create core schema",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "baseline"},
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&legacyUser{},
				&legacyProduct{},
				&legacyProductRelated{},
				&legacyOrder{},
				&legacyOrderItem{},
				&legacyCart{},
				&legacyCartItem{},
				&legacyMediaObject{},
				&legacyMediaVariant{},
				&legacyMediaReference{},
				&legacySavedPaymentMethod{},
				&legacySavedAddress{},
				&legacyStorefrontSettings{},
				&legacyCheckoutProviderSetting{},
			)
		},
	},
	{
		Version:         productPublishBackfillVersion,
		Name:            "backfill publish state for products with empty draft payload",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"backfill"},
		PostChecks: []PostCheck{
			{
				Name: "products_publish_state_backfill_applied",
				Check: func(tx *gorm.DB) error {
					var count int64
					if err := tx.Model(&models.Product{}).
						Where("is_published = ? AND (draft_data IS NULL OR draft_data = '')", false).
						Count(&count).Error; err != nil {
						return err
					}
					if count > 0 {
						return fmt.Errorf("post-check failed: found %d unpublished products with empty draft_data", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			_, err := ops.BatchedBackfillByID(
				tx,
				"products",
				"id",
				250,
				func(tx *gorm.DB, ids []int64) (int64, error) {
					result := tx.Model(&models.Product{}).
						Where("id IN ?", ids).
						Where("is_published = ? AND (draft_data IS NULL OR draft_data = '')", false).
						Update("is_published", true)
					return result.RowsAffected, result.Error
				},
				log.Printf,
			)
			return err
		},
	},
	{
		Version:         productCatalogDepthP0Version,
		Name:            "add catalog depth tables and normalized product drafts",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog"},
		PostChecks: []PostCheck{
			{
				Name: "catalog_depth_tables_exist",
				Check: func(tx *gorm.DB) error {
					required := []any{
						&models.Brand{},
						&models.ProductOption{},
						&models.ProductOptionValue{},
						&models.ProductVariant{},
						&models.ProductVariantOptionValue{},
						&models.ProductAttribute{},
						&models.ProductAttributeValue{},
						&models.SEOMetadata{},
						&models.ProductDraft{},
						&models.ProductOptionDraft{},
						&models.ProductOptionValueDraft{},
						&models.ProductVariantDraft{},
						&models.ProductVariantOptionValueDraft{},
						&models.ProductAttributeValueDraft{},
						&models.ProductRelatedDraft{},
					}
					for _, model := range required {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing migrated table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "products", "subtitle", "TEXT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "products", "brand_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "products", "default_variant_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.Product{}, "idx_products_brand_id"); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.Product{}, "idx_products_default_variant_id"); err != nil {
				return err
			}

			for _, model := range []any{
				&models.Brand{},
				&models.ProductOption{},
				&models.ProductOptionValue{},
				&models.ProductVariant{},
				&models.ProductVariantOptionValue{},
				&models.ProductAttribute{},
				&models.ProductAttributeValue{},
				&models.SEOMetadata{},
				&models.ProductDraft{},
				&models.ProductOptionDraft{},
				&models.ProductOptionValueDraft{},
				&models.ProductVariantDraft{},
				&models.ProductVariantOptionValueDraft{},
				&models.ProductAttributeValueDraft{},
				&models.ProductRelatedDraft{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         productCatalogDepthP2Version,
		Name:            "migrate cart and order items to product variants",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "variant_checkout_columns_populated",
				Check: func(tx *gorm.DB) error {
					for _, query := range []string{
						`SELECT COUNT(*) FROM cart_items WHERE product_variant_id IS NULL OR product_variant_id = 0`,
						`SELECT COUNT(*) FROM order_items WHERE product_variant_id IS NULL OR product_variant_id = 0`,
						`SELECT COUNT(*) FROM order_items WHERE TRIM(COALESCE(variant_sku, '')) = ''`,
						`SELECT COUNT(*) FROM order_items WHERE TRIM(COALESCE(variant_title, '')) = ''`,
					} {
						var count int64
						if err := tx.Raw(query).Scan(&count).Error; err != nil {
							return err
						}
						if count != 0 {
							return fmt.Errorf("post-check failed: %s returned %d rows", query, count)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "cart_items", "product_variant_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "order_items", "product_variant_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "order_items", "variant_sku", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "order_items", "variant_title", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := backfillCartItemVariants(tx); err != nil {
				return err
			}
			if err := backfillOrderItemVariants(tx); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version:         productCatalogDepthP2ProductBackfillVersion,
		Name:            "backfill default variants for legacy products",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"backfill", "catalog", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "all_products_have_default_variant",
				Check: func(tx *gorm.DB) error {
					var count int64
					if err := tx.Table("products").
						Where("default_variant_id IS NULL OR default_variant_id = 0").
						Count(&count).Error; err != nil {
						return err
					}
					if count != 0 {
						return fmt.Errorf("post-check failed: %d products still missing default_variant_id", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			return ensureAllProductsHaveDefaultVariants(tx)
		},
	},
	{
		Version:         productCatalogDepthP4Version,
		Name:            "harden catalog indexes and remove legacy draft blob",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"contract", "catalog", "hardening"},
		ContractBlockers: []string{
			"allow_contract_migrations",
		},
		PostChecks: []PostCheck{
			{
				Name: "legacy_product_draft_blob_removed",
				Check: func(tx *gorm.DB) error {
					if tx.Migrator().HasColumn("products", "draft_data") {
						return fmt.Errorf("post-check failed: products.draft_data still exists")
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn("products", "draft_data") {
				if err := backfillLegacyProductDraftBlobs(tx); err != nil {
					return err
				}
			}
			for _, statement := range catalogHardeningIndexStatements() {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
				ops.AddRowsTouched(tx, 1)
			}
			if tx.Migrator().HasColumn("products", "draft_data") {
				if err := tx.Exec(`ALTER TABLE "products" DROP COLUMN "draft_data"`).Error; err != nil {
					return err
				}
				ops.AddRowsTouched(tx, 1)
			}
			return nil
		},
	},
}

type legacyCartItemVariantBackfillRow struct {
	ID        uint
	ProductID uint
}

type legacyOrderItemVariantBackfillRow struct {
	ID        uint
	ProductID uint
}

type legacyProductVariantBackfillRow struct {
	ID uint
}

type legacyProductDraftBlobRow struct {
	ID             uint
	SKU            string
	Name           string
	Subtitle       *string
	Description    string
	Price          models.Money
	Stock          int
	Images         []string
	BrandID        *uint
	DraftData      string
	DraftUpdatedAt *time.Time
}

type legacyProductDraftPayload struct {
	SKU         string   `json:"sku"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Images      []string `json:"images"`
	RelatedIDs  []uint   `json:"related_ids"`
}

func backfillCartItemVariants(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyCartItemVariantBackfillRow
	if err := queryDB.Table("cart_items").
		Select("id", "product_id").
		Where("product_variant_id IS NULL OR product_variant_id = 0").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		variantID, _, _, err := resolveVariantBackfillTarget(tx, row.ProductID)
		if err != nil {
			return fmt.Errorf("backfill cart_items.id=%d: %w", row.ID, err)
		}
		if err := queryDB.Table("cart_items").Where("id = ?", row.ID).Update("product_variant_id", variantID).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureAllProductsHaveDefaultVariants(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyProductVariantBackfillRow
	if err := queryDB.Table("products").
		Select("id").
		Where("default_variant_id IS NULL OR default_variant_id = 0").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		if _, _, _, err := resolveVariantBackfillTarget(tx, row.ID); err != nil {
			return fmt.Errorf("backfill products.id=%d: %w", row.ID, err)
		}
	}

	return nil
}

func backfillLegacyProductDraftBlobs(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyProductDraftBlobRow
	if err := queryDB.Table("products").
		Select("id", "sku", "name", "subtitle", "description", "price", "stock", "images", "brand_id", "draft_data", "draft_updated_at").
		Where("TRIM(COALESCE(draft_data, '')) <> ''").
		Where("NOT EXISTS (SELECT 1 FROM product_drafts WHERE product_drafts.product_id = products.id)").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		var payload legacyProductDraftPayload
		if err := json.Unmarshal([]byte(strings.TrimSpace(row.DraftData)), &payload); err != nil {
			return fmt.Errorf("backfill product %d legacy draft blob: %w", row.ID, err)
		}

		sku := firstNonEmpty(payload.SKU, row.SKU)
		name := firstNonEmpty(payload.Name, row.Name)
		description := firstNonEmpty(payload.Description, row.Description)
		price := row.Price
		if payload.Price > 0 {
			price = models.MoneyFromFloat(payload.Price)
		}
		stock := row.Stock
		if payload.Stock > 0 || row.Stock == 0 {
			stock = payload.Stock
		}
		images := append([]string(nil), row.Images...)
		if len(payload.Images) > 0 {
			images = dedupeStrings(payload.Images)
		}
		imagesJSON, err := json.Marshal(images)
		if err != nil {
			return err
		}

		draftUpdatedAt := time.Now().UTC()
		if row.DraftUpdatedAt != nil && !row.DraftUpdatedAt.IsZero() {
			draftUpdatedAt = row.DraftUpdatedAt.UTC()
		}

		record := models.ProductDraft{
			ProductID:         row.ID,
			Version:           1,
			SKU:               sku,
			DefaultVariantSKU: sku,
			Name:              name,
			Subtitle:          row.Subtitle,
			Description:       description,
			Price:             price,
			Stock:             stock,
			ImagesJSON:        string(imagesJSON),
			BrandID:           row.BrandID,
		}
		if err := queryDB.Create(&record).Error; err != nil {
			return fmt.Errorf("backfill product %d draft header: %w", row.ID, err)
		}

		variant := models.ProductVariantDraft{
			ProductDraftID: record.ID,
			SKU:            sku,
			Title:          name,
			Price:          price,
			Stock:          stock,
			Position:       1,
			IsPublished:    true,
		}
		if err := queryDB.Create(&variant).Error; err != nil {
			return fmt.Errorf("backfill product %d default variant draft: %w", row.ID, err)
		}

		for index, relatedID := range dedupeUint(payload.RelatedIDs) {
			if err := queryDB.Create(&models.ProductRelatedDraft{
				ProductDraftID:   record.ID,
				RelatedProductID: relatedID,
				Position:         index + 1,
			}).Error; err != nil {
				return fmt.Errorf("backfill product %d related draft %d: %w", row.ID, relatedID, err)
			}
		}

		if row.DraftUpdatedAt == nil || row.DraftUpdatedAt.IsZero() {
			if err := queryDB.Table("products").Where("id = ?", row.ID).Update("draft_updated_at", draftUpdatedAt).Error; err != nil {
				return fmt.Errorf("backfill product %d draft timestamp: %w", row.ID, err)
			}
		}
	}

	return nil
}

func catalogHardeningIndexStatements() []string {
	return []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_variants_sku_unique ON product_variants (sku)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_options_product_name_unique ON product_options (product_id, name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_option_values_option_value_unique ON product_option_values (product_option_id, value)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_variant_option_values_variant_value_unique ON product_variant_option_values (product_variant_id, product_option_value_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_attribute_values_product_attribute_unique ON product_attribute_values (product_id, product_attribute_id)`,
		`CREATE INDEX IF NOT EXISTS idx_products_brand_published_created_at ON products (brand_id, is_published, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_product_variants_product_published_price ON product_variants (product_id, is_published, price)`,
		`CREATE INDEX IF NOT EXISTS idx_product_variants_product_published_stock ON product_variants (product_id, is_published, stock)`,
		`CREATE INDEX IF NOT EXISTS idx_product_attribute_values_text_lookup ON product_attribute_values (product_attribute_id, text_value, product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_product_attribute_values_enum_lookup ON product_attribute_values (product_attribute_id, enum_value, product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_product_attribute_values_number_lookup ON product_attribute_values (product_attribute_id, number_value, product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_product_attribute_values_boolean_lookup ON product_attribute_values (product_attribute_id, boolean_value, product_id)`,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func dedupeUint(values []uint) []uint {
	result := make([]uint, 0, len(values))
	seen := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func backfillOrderItemVariants(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyOrderItemVariantBackfillRow
	if err := queryDB.Table("order_items").
		Select("id", "product_id").
		Where("product_variant_id IS NULL OR product_variant_id = 0 OR TRIM(COALESCE(variant_sku, '')) = '' OR TRIM(COALESCE(variant_title, '')) = ''").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		variantID, sku, title, err := resolveVariantBackfillTarget(tx, row.ProductID)
		if err != nil {
			return fmt.Errorf("backfill order_items.id=%d: %w", row.ID, err)
		}
		if err := queryDB.Table("order_items").Where("id = ?", row.ID).Updates(map[string]any{
			"product_variant_id": variantID,
			"variant_sku":        sku,
			"variant_title":      title,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func resolveVariantBackfillTarget(tx *gorm.DB, productID uint) (uint, string, string, error) {
	if productID == 0 {
		return 0, "", "", fmt.Errorf("missing product_id")
	}

	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var product struct {
		ID               uint
		SKU              string
		Name             string
		Price            models.Money
		Stock            int
		IsPublished      bool
		DefaultVariantID *uint
	}
	if err := queryDB.Table("products").
		Select("id", "sku", "name", "price", "stock", "is_published", "default_variant_id").
		Where("id = ?", productID).
		Take(&product).Error; err != nil {
		return 0, "", "", fmt.Errorf("load product %d: %w", productID, err)
	}

	var variant struct {
		ID    uint
		SKU   string
		Title string
	}
	query := queryDB.Table("product_variants").Select("id", "sku", "title")
	if product.DefaultVariantID != nil && *product.DefaultVariantID != 0 {
		if err := query.Where("id = ? AND product_id = ?", *product.DefaultVariantID, productID).Take(&variant).Error; err == nil {
			return variant.ID, variant.SKU, variant.Title, nil
		}
	}

	if err := queryDB.Table("product_variants").
		Select("id", "sku", "title").
		Where("product_id = ?", productID).
		Order("position ASC").
		Order("id ASC").
		Take(&variant).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return 0, "", "", fmt.Errorf("resolve default variant for product %d: %w", productID, err)
		}

		createdVariant := models.ProductVariant{
			ProductID:   product.ID,
			SKU:         product.SKU,
			Title:       product.Name,
			Price:       product.Price,
			Stock:       product.Stock,
			Position:    1,
			IsPublished: product.IsPublished,
		}
		if err := queryDB.Create(&createdVariant).Error; err != nil {
			return 0, "", "", fmt.Errorf("create fallback variant for product %d: %w", productID, err)
		}
		if err := queryDB.Table("products").
			Where("id = ?", productID).
			Update("default_variant_id", createdVariant.ID).Error; err != nil {
			return 0, "", "", fmt.Errorf("set default variant for product %d: %w", productID, err)
		}
		return createdVariant.ID, createdVariant.SKU, createdVariant.Title, nil
	}

	return variant.ID, variant.SKU, variant.Title, nil
}

func ensureTable(db *gorm.DB) error {
	return db.AutoMigrate(&SchemaMigration{})
}

func AppliedVersions(db *gorm.DB) (map[string]SchemaMigration, error) {
	return appliedVersionsWithMigrations(db, orderedMigrations)
}

func appliedVersionsWithMigrations(db *gorm.DB, definitions []Migration) (map[string]SchemaMigration, error) {
	if err := validateMigrations(definitions); err != nil {
		return nil, err
	}

	if err := ensureTable(db); err != nil {
		return nil, err
	}

	var rows []SchemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}

	if err := validateAndBackfillAppliedChecksums(db, definitions, rows); err != nil {
		return nil, err
	}

	applied := make(map[string]SchemaMigration, len(rows))
	for _, row := range rows {
		applied[row.Version] = row
	}
	return applied, nil
}

func Pending(db *gorm.DB) ([]Migration, error) {
	return pendingWithMigrations(db, orderedMigrations)
}

func Run(db *gorm.DB) error {
	return runWithMigrations(db, orderedMigrations)
}

// RunWithoutContract applies all known non-contract migrations without
// acknowledging contract steps.
func RunWithoutContract(db *gorm.DB) error {
	definitions := make([]Migration, 0, len(orderedMigrations))
	for _, migration := range orderedMigrations {
		if hasTag(migration.Tags, migrationContractTag) {
			continue
		}
		definitions = append(definitions, migration)
	}
	return runWithMigrations(db, definitions)
}

func StatusReport(db *gorm.DB) (Status, error) {
	return statusForMigrations(db, orderedMigrations)
}

func runWithMigrations(db *gorm.DB, definitions []Migration) (runErr error) {
	if err := validateMigrations(definitions); err != nil {
		return err
	}

	unlock, err := acquireMigrationLock(db)
	if err != nil {
		return err
	}
	defer func() {
		if err := unlock(); err != nil {
			if runErr != nil {
				runErr = errors.Join(runErr, err)
				return
			}
			runErr = err
		}
	}()

	pending, err := pendingWithMigrations(db, definitions)
	if err != nil {
		return err
	}

	if err := guardPendingMigrations(db, pending, true); err != nil {
		return err
	}

	for _, migration := range pending {
		start := time.Now().UTC()
		log.Printf("migration_step_start version=%s name=%q transaction_mode=%s tags=%s", migration.Version, migration.Name, normalizeTransactionMode(migration.TransactionMode), strings.Join(migration.Tags, ","))

		durationMs := int64(0)
		rowsTouched := int64(0)
		checkResult := "ok"

		runErr := executeMigration(db, migration, &rowsTouched, &checkResult, &durationMs)
		if runErr != nil {
			log.Printf("migration_step_failed version=%s name=%q duration_ms=%d rows_touched=%d check_result=%s error=%q", migration.Version, migration.Name, durationMs, rowsTouched, checkResult, runErr.Error())
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, runErr)
		}

		meta, metaErr := buildExecutionMeta(migration, rowsTouched, checkResult)
		if metaErr != nil {
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, metaErr)
		}
		checksum := migrationChecksum(migration)

		if err := db.Create(&SchemaMigration{
			Version:       migration.Version,
			Name:          migration.Name,
			Checksum:      checksum,
			AppliedAt:     start,
			DurationMs:    durationMs,
			ExecutionMeta: meta,
		}).Error; err != nil {
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, err)
		}

		log.Printf("migration_step_complete version=%s name=%q duration_ms=%d rows_touched=%d check_result=%s", migration.Version, migration.Name, durationMs, rowsTouched, checkResult)
		alertThreshold := migrationStepAlertThresholdMs()
		if durationMs > alertThreshold {
			log.Printf("migration_step_alert version=%s name=%q duration_ms=%d threshold_ms=%d", migration.Version, migration.Name, durationMs, alertThreshold)
		}
	}
	return runErr
}

func executeMigration(db *gorm.DB, migration Migration, rowsTouched *int64, checkResult *string, durationMs *int64) error {
	mode := normalizeTransactionMode(migration.TransactionMode)
	start := time.Now().UTC()

	if mode == TransactionModeNone {
		runDB, counter := ops.AttachRowsCounter(db)
		if err := migration.Up(runDB); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		if err := runPostChecks(runDB, migration.PostChecks); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		*durationMs = time.Since(start).Milliseconds()
		*rowsTouched = ops.ReadRowsCounter(counter)
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		runTx, counter := ops.AttachRowsCounter(tx)
		if err := migration.Up(runTx); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		if err := runPostChecks(runTx, migration.PostChecks); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		*durationMs = time.Since(start).Milliseconds()
		*rowsTouched = ops.ReadRowsCounter(counter)
		return nil
	})
}

func runPostChecks(tx *gorm.DB, checks []PostCheck) error {
	for _, check := range checks {
		if err := check.Check(tx); err != nil {
			return fmt.Errorf("post-check %q failed: %w", check.Name, err)
		}
	}
	return nil
}

func buildExecutionMeta(migration Migration, rowsTouched int64, checkResult string) (string, error) {
	meta := map[string]any{
		"transaction_mode": normalizeTransactionMode(migration.TransactionMode),
		"rows_touched":     rowsTouched,
		"check_result":     checkResult,
	}
	if len(migration.Tags) > 0 {
		meta["tags"] = migration.Tags
	}
	if len(migration.ContractBlockers) > 0 {
		meta["contract_blockers"] = migration.ContractBlockers
	}

	encoded, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to encode execution metadata: %w", err)
	}
	return string(encoded), nil
}

func pendingWithMigrations(db *gorm.DB, definitions []Migration) ([]Migration, error) {
	if err := validateMigrations(definitions); err != nil {
		return nil, err
	}

	applied, err := appliedVersionsWithMigrations(db, definitions)
	if err != nil {
		return nil, err
	}

	pending := make([]Migration, 0)
	for _, migration := range definitions {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		pending = append(pending, migration)
	}
	return pending, nil
}

func statusForMigrations(db *gorm.DB, definitions []Migration) (Status, error) {
	if err := validateMigrations(definitions); err != nil {
		return Status{}, err
	}

	applied, err := appliedVersionsWithMigrations(db, definitions)
	if err != nil {
		return Status{}, err
	}

	latestAppliedVersion := ""
	for version := range applied {
		if version > latestAppliedVersion {
			latestAppliedVersion = version
		}
	}

	pendingCount := 0
	for _, migration := range definitions {
		if _, ok := applied[migration.Version]; !ok {
			pendingCount++
		}
	}

	return Status{
		LatestKnownVersion:   latestVersionFor(definitions),
		LatestAppliedVersion: latestAppliedVersion,
		PendingCount:         pendingCount,
	}, nil
}

func validateMigrations(definitions []Migration) error {
	seen := make(map[string]struct{}, len(definitions))
	prevVersion := ""
	for idx, migration := range definitions {
		if migration.Version == "" {
			return fmt.Errorf("migration at index %d has empty version", idx)
		}
		if !versionPattern.MatchString(migration.Version) {
			return fmt.Errorf("migration %q has invalid version format (expected YYYYMMDDNN_slug)", migration.Version)
		}
		if migration.Name == "" {
			return fmt.Errorf("migration %q has empty name", migration.Version)
		}
		if migration.Up == nil {
			return fmt.Errorf("migration %q has nil Up function", migration.Version)
		}
		if migration.TransactionMode != "" &&
			migration.TransactionMode != TransactionModeRequired &&
			migration.TransactionMode != TransactionModeNone {
			return fmt.Errorf("migration %q has invalid transaction mode %q", migration.Version, migration.TransactionMode)
		}
		for tagIdx, tag := range migration.Tags {
			if tag == "" {
				return fmt.Errorf("migration %q has empty tag at index %d", migration.Version, tagIdx)
			}
			if !tagPattern.MatchString(tag) {
				return fmt.Errorf("migration %q has invalid tag %q", migration.Version, tag)
			}
		}
		for checkIdx, check := range migration.PostChecks {
			if check.Name == "" {
				return fmt.Errorf("migration %q has empty post-check name at index %d", migration.Version, checkIdx)
			}
			if check.Check == nil {
				return fmt.Errorf("migration %q has nil post-check handler for %q", migration.Version, check.Name)
			}
		}
		for blockerIdx, blocker := range migration.ContractBlockers {
			if blocker == "" {
				return fmt.Errorf("migration %q has empty contract blocker at index %d", migration.Version, blockerIdx)
			}
		}
		if hasTag(migration.Tags, migrationContractTag) && len(migration.ContractBlockers) == 0 {
			return fmt.Errorf("migration %q is tagged contract and must declare at least one contract blocker", migration.Version)
		}
		if _, exists := seen[migration.Version]; exists {
			return fmt.Errorf("duplicate migration version %q", migration.Version)
		}
		if prevVersion != "" && migration.Version <= prevVersion {
			return fmt.Errorf("migration %q is out of order (must be strictly increasing)", migration.Version)
		}
		seen[migration.Version] = struct{}{}
		prevVersion = migration.Version
	}
	return nil
}

func acquireMigrationLockForDB(db *gorm.DB) (func() error, error) {
	if db.Dialector.Name() != "postgres" {
		return func() error { return nil }, nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql db for migration advisory lock: %w", err)
	}

	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to open migration advisory lock session: %w", err)
	}

	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}

	return func() (releaseErr error) {
		defer func() {
			closeErr := conn.Close()
			if closeErr != nil {
				if releaseErr != nil {
					releaseErr = errors.Join(releaseErr, fmt.Errorf("failed to close migration advisory lock session: %w", closeErr))
				} else {
					releaseErr = fmt.Errorf("failed to close migration advisory lock session: %w", closeErr)
				}
			}
		}()

		var unlocked bool
		if err := conn.QueryRowContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey).Scan(&unlocked); err != nil {
			releaseErr = fmt.Errorf("failed to release migration advisory lock: %w", err)
			return
		}
		if !unlocked {
			releaseErr = errors.New("failed to release migration advisory lock: lock was not held on migration session")
			return
		}
		return
	}, nil
}

func latestVersionFor(definitions []Migration) string {
	if len(definitions) == 0 {
		return ""
	}
	return definitions[len(definitions)-1].Version
}

func normalizeTransactionMode(mode TransactionMode) TransactionMode {
	if mode == "" {
		return TransactionModeRequired
	}
	return mode
}

func hasTag(tags []string, expected string) bool {
	for _, tag := range tags {
		if tag == expected {
			return true
		}
	}
	return false
}

func latestVersionOrUnknown(version string) string {
	if version == "" {
		return "none"
	}
	return version
}

func printStatusLines(status Status) []string {
	return []string{
		fmt.Sprintf("latest_known_version=%s", latestVersionOrUnknown(status.LatestKnownVersion)),
		fmt.Sprintf("latest_applied_version=%s", latestVersionOrUnknown(status.LatestAppliedVersion)),
		fmt.Sprintf("pending_count=%d", status.PendingCount),
	}
}

func StatusLines(db *gorm.DB) ([]string, error) {
	status, err := StatusReport(db)
	if err != nil {
		return nil, err
	}
	return printStatusLines(status), nil
}

func printPlanLines(pending []Migration) []string {
	lines := []string{
		fmt.Sprintf("pending_count=%d", len(pending)),
	}
	for idx, migration := range pending {
		lines = append(lines,
			fmt.Sprintf("pending_%02d_version=%s", idx+1, migration.Version),
			fmt.Sprintf("pending_%02d_name=%s", idx+1, migration.Name),
		)
	}
	return lines
}

func PlanLines(db *gorm.DB) ([]string, error) {
	pending, err := Pending(db)
	if err != nil {
		return nil, err
	}
	return printPlanLines(pending), nil
}

func Check(db *gorm.DB) error {
	status, err := StatusReport(db)
	if err != nil {
		return err
	}
	if status.PendingCount > 0 {
		return errors.New("database is not at latest migration")
	}
	return nil
}

func EnsureReady(db *gorm.DB, autoApply bool) error {
	if autoApply {
		return Run(db)
	}

	status, err := StatusReport(db)
	if err != nil {
		return err
	}
	if status.PendingCount > 0 {
		return fmt.Errorf(
			"database has %d pending migrations (latest_applied=%s latest_known=%s); run `make migrate` or set AUTO_APPLY_MIGRATIONS=true",
			status.PendingCount,
			latestVersionOrUnknown(status.LatestAppliedVersion),
			latestVersionOrUnknown(status.LatestKnownVersion),
		)
	}
	return nil
}

func LatestVersion() string {
	return latestVersionFor(orderedMigrations)
}

func Versions() []string {
	versions := make([]string, 0, len(orderedMigrations))
	for _, migration := range orderedMigrations {
		versions = append(versions, migration.Version)
	}
	sort.Strings(versions)
	return versions
}

func validateAndBackfillAppliedChecksums(db *gorm.DB, definitions []Migration, rows []SchemaMigration) error {
	definitionsByVersion := make(map[string]Migration, len(definitions))
	for _, definition := range definitions {
		definitionsByVersion[definition.Version] = definition
	}

	for idx := range rows {
		row := &rows[idx]
		definition, ok := definitionsByVersion[row.Version]
		if !ok {
			return fmt.Errorf("applied migration %s is unknown to current binary", row.Version)
		}

		expectedChecksum := migrationChecksum(definition)
		trimmedChecksum := strings.TrimSpace(row.Checksum)
		if trimmedChecksum == "" {
			if err := backfillAppliedChecksum(db, row, expectedChecksum); err != nil {
				return err
			}
			continue
		}
		if trimmedChecksum == expectedChecksum {
			continue
		}
		if row.Version <= migrationChecksumCutoverVersion {
			if err := backfillAppliedChecksum(db, row, expectedChecksum); err != nil {
				return err
			}
			continue
		}
		if slices.Contains(migrationChecksumCompatibilityBackfills[row.Version], trimmedChecksum) {
			if err := backfillAppliedChecksum(db, row, expectedChecksum); err != nil {
				return err
			}
			continue
		}
		if row.Checksum != expectedChecksum {
			return fmt.Errorf(
				"applied migration %s checksum mismatch (stored=%s current=%s)",
				row.Version,
				row.Checksum,
				expectedChecksum,
			)
		}
	}
	return nil
}

func backfillAppliedChecksum(db *gorm.DB, row *SchemaMigration, checksum string) error {
	if err := db.Model(&SchemaMigration{}).
		Where("version = ?", row.Version).
		Update("checksum", checksum).Error; err != nil {
		return fmt.Errorf("failed to backfill checksum for applied migration %s: %w", row.Version, err)
	}
	row.Checksum = checksum
	return nil
}

func migrationChecksum(migration Migration) string {
	tags := append([]string(nil), migration.Tags...)
	sort.Strings(tags)
	contractBlockers := append([]string(nil), migration.ContractBlockers...)
	sort.Strings(contractBlockers)

	fingerprint := map[string]any{
		"version":           migration.Version,
		"name":              migration.Name,
		"transaction_mode":  normalizeTransactionMode(migration.TransactionMode),
		"tags":              tags,
		"contract_blockers": contractBlockers,
		"source":            migrationChecksumSource(migration.Version),
		"post_checks_count": len(migration.PostChecks),
	}
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum)
}

func migrationChecksumSource(version string) string {
	content := embeddedMigrationSource
	if migrationSourcePath != "internal/migrations/migrations.go" {
		loaded, err := os.ReadFile(migrationSourcePath)
		if err != nil {
			return ""
		}
		content = loaded
	}

	source := migrationSourceByVersion(content, version)
	if source == "" {
		return ""
	}
	return source
}

func SourcePath() string {
	return migrationSourcePath
}

func DefaultSchemaSnapshotPath() string {
	return defaultSchemaSnapshotPath
}

func allowContractMigrations() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(contractGuardEnvVar)), "true")
}

func migrationStepAlertThresholdMs() int64 {
	raw := strings.TrimSpace(os.Getenv(migrationStepAlertThresholdEnvVar))
	if raw == "" {
		return 30_000
	}
	parsed, err := time.ParseDuration(raw + "ms")
	if err != nil {
		return 30_000
	}
	return parsed.Milliseconds()
}
