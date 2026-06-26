package migrations

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/internal/migrations/ops"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
const guestCheckoutP0Version = "2026031001_guest_checkout_p0"
const guestCheckoutP1Version = "2026031002_guest_checkout_p1"
const guestCheckoutP3Version = "2026031101_guest_checkout_p3"
const providersP0Version = "2026031701_providers_p0_payment_foundation"
const providersP2Version = "2026032001_providers_p2_webhook_events"
const providersP3Version = "2026032002_providers_p3_shipping_tax"
const providersP4Version = "2026032003_providers_p4_security_ops"
const inventoryDisciplineP0Version = "2026032401_inventory_discipline_p0"
const inventoryDisciplineP1Version = "2026032402_inventory_discipline_p1_reservations"
const inventoryDisciplineP2Version = "2026032403_inventory_discipline_p2_alerts"
const inventoryDisciplineP3Version = "2026032404_inventory_discipline_p3_purchase_orders"
const inventoryDisciplineP4Version = "2026032405_inventory_discipline_p4_adjustments"
const websiteSettingsVersion = "2026042701_website_settings"
const websiteCouponSettingsVersion = "2026042702_website_coupon_settings"
const productCategoriesP0Version = "2026050701_product_categories_p0"
const productCategoriesP1Version = "2026050702_product_categories_p1_assignment"
const productCategoriesP3Version = "2026050703_product_categories_p3_hardening"
const productAttributeEnumsVersion = "2026051801_product_attribute_enums"
const discountsPromotionsP0Version = "2026051901_discounts_promotions_p0"
const discountsPromotionsP2Version = "2026051902_discounts_promotions_p2_scheduling"
const discountsPromotionsP3Version = "2026051903_discounts_promotions_p3_templates_controls"
const discountsPromotionsP4Version = "2026051904_discounts_promotions_p4_operations"
const ecommerceCMSP0Version = "2026061701_ecommerce_cms_p0"
const ecommerceCMSP2Version = "2026061702_ecommerce_cms_p2_navigation_global"
const ecommerceCMSP4Version = "2026062101_ecommerce_cms_p4_delivery"
const ecommerceCMSMediaIDsVersion = "2026062102_ecommerce_cms_media_ids"
const ecommerceCMSP5Version = "2026062103_ecommerce_cms_p5_seo_redirects"
const ecommerceCMSP6Version = "2026062104_ecommerce_cms_p6_localization_governance"
const ecommerceCMSCompletionVersion = "2026062105_ecommerce_cms_governance_operations"
const ecommerceCMSLegacyRemovalVersion = "2026062106_remove_legacy_storefront"
const ecommerceCMSFooterBackfillVersion = "2026062107_cms_footer_empty_backfill"
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
				&legacyWebsiteSettings{},
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
	{
		Version:         guestCheckoutP0Version,
		Name:            "introduce checkout sessions and guest checkout settings",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "checkout_sessions_backfilled",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.CheckoutSession{}) {
						return fmt.Errorf("missing checkout_sessions table")
					}
					if !tx.Migrator().HasColumn("carts", "checkout_session_id") {
						return fmt.Errorf("missing carts.checkout_session_id")
					}
					if tx.Migrator().HasColumn("carts", "user_id") {
						return fmt.Errorf("post-check failed: carts.user_id still exists")
					}
					var count int64
					if err := tx.Table("carts").
						Where("checkout_session_id IS NULL OR checkout_session_id = 0").
						Count(&count).Error; err != nil {
						return err
					}
					if count != 0 {
						return fmt.Errorf("post-check failed: found %d carts without checkout_session_id", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.CheckoutSession{}); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "carts", "checkout_session_id", "BIGINT"); err != nil {
				return err
			}
			if err := backfillLegacyCartCheckoutSessions(tx); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.Cart{}, "idx_carts_checkout_session_id"); err != nil {
				return err
			}
			if err := tx.Exec(`DROP INDEX IF EXISTS "idx_carts_user_id"`).Error; err != nil {
				return err
			}
			ops.AddRowsTouched(tx, 1)
			if tx.Migrator().HasColumn("carts", "user_id") {
				if err := tx.Exec(`ALTER TABLE "carts" DROP COLUMN "user_id"`).Error; err != nil {
					return err
				}
				ops.AddRowsTouched(tx, 1)
			}
			if err := backfillStorefrontCheckoutDefaults(tx); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version:         guestCheckoutP1Version,
		Name:            "move orders to checkout sessions with guest fields",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "checkout", "orders"},
		PostChecks: []PostCheck{
			{
				Name: "orders_checkout_session_backfilled",
				Check: func(tx *gorm.DB) error {
					for _, column := range []string{"checkout_session_id", "guest_email", "confirmation_token"} {
						if !tx.Migrator().HasColumn("orders", column) {
							return fmt.Errorf("missing orders.%s", column)
						}
					}
					var count int64
					if err := tx.Model(&models.Order{}).
						Where("checkout_session_id IS NULL OR checkout_session_id = 0").
						Count(&count).Error; err != nil {
						return err
					}
					if count != 0 {
						return fmt.Errorf("post-check failed: found %d orders without checkout_session_id", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "orders", "checkout_session_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "orders", "guest_email", "TEXT"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "orders", "confirmation_token", "TEXT"); err != nil {
				return err
			}
			if err := backfillLegacyOrderCheckoutSessions(tx); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.Order{}, "idx_orders_checkout_session_id"); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.Order{}, "idx_orders_confirmation_token"); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version:         guestCheckoutP3Version,
		Name:            "add guest order claim metadata and idempotency records",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "checkout", "orders", "hardening"},
		PostChecks: []PostCheck{
			{
				Name: "guest_checkout_p3_structures_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasColumn("orders", "claimed_at") {
						return fmt.Errorf("missing orders.claimed_at")
					}
					if !tx.Migrator().HasTable(&models.IdempotencyKey{}) {
						return fmt.Errorf("missing idempotency_keys table")
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "orders", "claimed_at", "TIMESTAMPTZ"); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.IdempotencyKey{}); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.IdempotencyKey{}, "idx_idempotency_scope_session_key"); err != nil {
				return err
			}
			if err := ops.CreateIndexIfNotExists(tx, &models.IdempotencyKey{}, "idx_idempotency_keys_expires_at"); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version:         providersP0Version,
		Name:            "add provider payment foundation structures",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "checkout", "payments", "providers"},
		PostChecks: []PostCheck{
			{
				Name: "providers_p0_structures_exist",
				Check: func(tx *gorm.DB) error {
					for _, column := range []string{"status", "correlation_id", "payment_intent_id"} {
						if !tx.Migrator().HasColumn("idempotency_keys", column) {
							return fmt.Errorf("missing idempotency_keys.%s", column)
						}
					}
					for _, model := range []any{
						&models.OrderCheckoutSnapshot{},
						&models.OrderCheckoutSnapshotItem{},
						&models.PaymentIntent{},
						&models.PaymentTransaction{},
						&models.OrderStatusHistory{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "idempotency_keys", "status", "TEXT NOT NULL DEFAULT 'processing'"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "idempotency_keys", "correlation_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := ops.AddColumnIfNotExists(tx, "idempotency_keys", "payment_intent_id", "BIGINT"); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.OrderCheckoutSnapshot{}); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.OrderCheckoutSnapshotItem{}); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.PaymentIntent{}); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.PaymentTransaction{}); err != nil {
				return err
			}
			if err := ops.CreateTableIfNotExists(tx, &models.OrderStatusHistory{}); err != nil {
				return err
			}
			for _, index := range []struct {
				model any
				name  string
			}{
				{model: &models.IdempotencyKey{}, name: "idx_idempotency_keys_correlation_id"},
				{model: &models.IdempotencyKey{}, name: "idx_idempotency_keys_payment_intent_id"},
				{model: &models.PaymentTransaction{}, name: "idx_payment_txn_intent_operation_key"},
			} {
				if err := ops.CreateIndexIfNotExists(tx, index.model, index.name); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         providersP2Version,
		Name:            "add provider webhook event store",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "providers", "payments", "webhooks"},
		PostChecks: []PostCheck{
			{
				Name: "providers_p2_webhook_events_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.WebhookEvent{}) {
						return fmt.Errorf("missing table for %T", &models.WebhookEvent{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.WebhookEvent{}); err != nil {
				return err
			}
			for _, index := range []string{
				"idx_webhook_events_provider_event",
				"idx_webhook_events_received_at",
				"idx_webhook_events_processed_at",
			} {
				if err := ops.CreateIndexIfNotExists(tx, &models.WebhookEvent{}, index); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         providersP3Version,
		Name:            "add provider shipping and tax structures",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "providers", "shipping", "tax"},
		PostChecks: []PostCheck{
			{
				Name: "providers_p3_shipping_tax_structures_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.Shipment{},
						&models.ShipmentRate{},
						&models.ShipmentPackage{},
						&models.TrackingEvent{},
						&models.OrderTaxLine{},
						&models.TaxNexusConfig{},
						&models.TaxExport{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.Shipment{},
				&models.ShipmentRate{},
				&models.ShipmentPackage{},
				&models.TrackingEvent{},
				&models.OrderTaxLine{},
				&models.TaxNexusConfig{},
				&models.TaxExport{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, index := range []struct {
				model any
				name  string
			}{
				{model: &models.Shipment{}, name: "idx_shipments_finalized_at"},
				{model: &models.Shipment{}, name: "idx_shipments_provider"},
				{model: &models.Shipment{}, name: "idx_shipments_shipment_rate_id"},
				{model: &models.ShipmentRate{}, name: "idx_shipment_rates_snapshot_provider_rate"},
				{model: &models.TrackingEvent{}, name: "idx_tracking_events_shipment_provider_event"},
				{model: &models.TaxNexusConfig{}, name: "idx_tax_nexus_provider_region"},
			} {
				if err := ops.CreateIndexIfNotExists(tx, index.model, index.name); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         providersP4Version,
		Name:            "add provider security and ops structures",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "providers", "security", "ops"},
		PostChecks: []PostCheck{
			{
				Name: "providers_p4_security_ops_structures_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.ProviderCredential{},
						&models.ProviderCallAudit{},
						&models.ProviderReconciliationRun{},
						&models.ProviderReconciliationDrift{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.ProviderCredential{},
				&models.ProviderCallAudit{},
				&models.ProviderReconciliationRun{},
				&models.ProviderReconciliationDrift{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, index := range []struct {
				model any
				name  string
			}{
				{model: &models.ProviderCredential{}, name: "idx_provider_credentials_scope"},
			} {
				if err := ops.CreateIndexIfNotExists(tx, index.model, index.name); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         inventoryDisciplineP0Version,
		Name:            "add inventory quantity model and movement ledger",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "inventory"},
		PostChecks: []PostCheck{
			{
				Name: "inventory_p0_structures_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.InventoryItem{},
						&models.InventoryLevel{},
						&models.InventoryMovement{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.InventoryItem{},
				&models.InventoryLevel{},
				&models.InventoryMovement{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, index := range []struct {
				model any
				name  string
			}{
				{model: &models.InventoryItem{}, name: "idx_inventory_items_product_variant_id"},
				{model: &models.InventoryLevel{}, name: "idx_inventory_levels_inventory_item_id"},
				{model: &models.InventoryMovement{}, name: "idx_inventory_movements_inventory_item_id"},
				{model: &models.InventoryMovement{}, name: "idx_inventory_movements_movement_type"},
				{model: &models.InventoryMovement{}, name: "idx_inventory_movements_reference_type"},
				{model: &models.InventoryMovement{}, name: "idx_inventory_movements_reference_id"},
			} {
				if err := ops.CreateIndexIfNotExists(tx, index.model, index.name); err != nil {
					return err
				}
			}
			return backfillInventoryItemsFromVariants(tx)
		},
	},
	{
		Version:         inventoryDisciplineP1Version,
		Name:            "add inventory reservations",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "inventory", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "inventory_p1_reservations_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.InventoryReservation{}) {
						return fmt.Errorf("missing table for %T", &models.InventoryReservation{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.InventoryReservation{}); err != nil {
				return err
			}
			for _, index := range []string{
				"idx_inventory_reservations_inventory_item_id",
				"idx_inventory_reservations_product_variant_id",
				"idx_inventory_reservations_status",
				"idx_inventory_reservations_expires_at",
				"idx_inventory_reservations_owner_type",
				"idx_inventory_reservations_owner_id",
				"idx_inventory_reservations_checkout_session_id",
				"idx_inventory_reservations_order_id",
				"idx_inventory_reservations_idempotency_key",
			} {
				if err := ops.CreateIndexIfNotExists(tx, &models.InventoryReservation{}, index); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         inventoryDisciplineP2Version,
		Name:            "add inventory thresholds and alerts",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "inventory"},
		PostChecks: []PostCheck{
			{
				Name: "inventory_p2_alert_tables_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.InventoryThreshold{},
						&models.InventoryAlert{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.InventoryThreshold{},
				&models.InventoryAlert{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, index := range []struct {
				model any
				name  string
			}{
				{model: &models.InventoryThreshold{}, name: "idx_inventory_thresholds_product_variant_id"},
				{model: &models.InventoryAlert{}, name: "idx_inventory_alerts_inventory_item_id"},
				{model: &models.InventoryAlert{}, name: "idx_inventory_alerts_product_variant_id"},
				{model: &models.InventoryAlert{}, name: "idx_inventory_alerts_alert_type"},
				{model: &models.InventoryAlert{}, name: "idx_inventory_alerts_status"},
				{model: &models.InventoryAlert{}, name: "idx_inventory_alerts_opened_at"},
			} {
				if err := ops.CreateIndexIfNotExists(tx, index.model, index.name); err != nil {
					return err
				}
			}
			return tx.FirstOrCreate(&models.InventoryThreshold{}, models.InventoryThreshold{LowStockQuantity: 5}).Error
		},
	},
	{
		Version:         inventoryDisciplineP3Version,
		Name:            "add purchase orders and receiving",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "inventory"},
		PostChecks: []PostCheck{
			{
				Name: "inventory_p3_purchase_order_tables_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.Supplier{},
						&models.PurchaseOrder{},
						&models.PurchaseOrderItem{},
						&models.InventoryReceipt{},
						&models.InventoryReceiptItem{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.Supplier{},
				&models.PurchaseOrder{},
				&models.PurchaseOrderItem{},
				&models.InventoryReceipt{},
				&models.InventoryReceiptItem{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         inventoryDisciplineP4Version,
		Name:            "add inventory adjustments",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "inventory"},
		PostChecks: []PostCheck{
			{
				Name: "inventory_p4_adjustments_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.InventoryAdjustment{}) {
						return fmt.Errorf("missing table for %T", &models.InventoryAdjustment{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.InventoryAdjustment{}); err != nil {
				return err
			}
			for _, index := range []string{
				"idx_inventory_adjustments_inventory_item_id",
				"idx_inventory_adjustments_product_variant_id",
				"idx_inventory_adjustments_reason_code",
			} {
				if err := ops.CreateIndexIfNotExists(tx, &models.InventoryAdjustment{}, index); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         websiteSettingsVersion,
		Name:            "move website level settings out of storefront and environment",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "settings", "auth", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "website_settings_singleton_exists",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&legacyWebsiteSettings{}) {
						return fmt.Errorf("missing website_settings table")
					}
					var count int64
					if err := tx.Model(&legacyWebsiteSettings{}).
						Where("id = ?", models.WebsiteSettingsSingletonID).
						Count(&count).Error; err != nil {
						return err
					}
					if count != 1 {
						return fmt.Errorf("post-check failed: expected website_settings singleton, found %d", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &legacyWebsiteSettings{}); err != nil {
				return err
			}
			allowGuestCheckout, err := legacyAllowGuestCheckout(tx)
			if err != nil {
				return err
			}
			settings := legacyWebsiteSettings{
				ID:                 models.WebsiteSettingsSingletonID,
				AllowGuestCheckout: allowGuestCheckout,
			}
			if err := tx.Select("*").Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"allow_guest_checkout": allowGuestCheckout,
				}),
			}).Create(&settings).Error; err != nil {
				return err
			}
			return stripStorefrontCheckoutSettings(tx)
		},
	},
	{
		Version:         websiteCouponSettingsVersion,
		Name:            "add website coupon code setting",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "settings", "checkout", "discounts"},
		PostChecks: []PostCheck{
			{
				Name: "website_coupon_codes_enabled_exists",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasColumn(&models.WebsiteSettings{}, "coupon_codes_enabled") {
						return fmt.Errorf("website_settings.coupon_codes_enabled column missing")
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			return ops.AddColumnIfNotExists(tx, "website_settings", "coupon_codes_enabled", "BOOLEAN NOT NULL DEFAULT TRUE")
		},
	},
	{
		Version:         productCategoriesP0Version,
		Name:            "add product category hierarchy",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog"},
		PostChecks: []PostCheck{
			{
				Name: "categories_table_exists",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.Category{}) {
						return fmt.Errorf("missing migrated table for %T", &models.Category{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			return ops.CreateTableIfNotExists(tx, &models.Category{})
		},
	},
	{
		Version:         productCategoriesP1Version,
		Name:            "add product category assignments",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog"},
		PostChecks: []PostCheck{
			{
				Name: "product_category_assignment_tables_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.ProductCategory{}) {
						return fmt.Errorf("missing table for %T", &models.ProductCategory{})
					}
					if !tx.Migrator().HasTable(&models.ProductCategoryDraft{}) {
						return fmt.Errorf("missing table for %T", &models.ProductCategoryDraft{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.ProductCategory{},
				&models.ProductCategoryDraft{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         productCategoriesP3Version,
		Name:            "add product category integrity indexes",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog"},
		PostChecks: []PostCheck{
			{
				Name: "product_category_lookup_indexes_exist",
				Check: func(tx *gorm.DB) error {
					for _, index := range []string{
						"idx_product_categories_product_category",
						"idx_product_categories_category_product",
					} {
						if !tx.Migrator().HasIndex("product_categories", index) {
							return fmt.Errorf("missing product_categories index %s", index)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_categories_product_category ON product_categories (product_id, category_id)`,
				`CREATE INDEX IF NOT EXISTS idx_product_categories_category_product ON product_categories (category_id, product_id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         productAttributeEnumsVersion,
		Name:            "add enum values to product attributes",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "catalog"},
		PostChecks: []PostCheck{
			{
				Name: "product_attributes_enum_values_exists",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasColumn(&models.ProductAttribute{}, "enum_values") {
						return errors.New("product_attributes.enum_values column missing")
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.AddColumnIfNotExists(tx, "product_attributes", "enum_values", "TEXT"); err != nil {
				return err
			}
			return backfillProductAttributeEnumValues(tx)
		},
	},
	{
		Version:         discountsPromotionsP0Version,
		Name:            "add discounts and promotions core tables",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "discounts", "checkout"},
		PostChecks: []PostCheck{
			{
				Name: "discounts_promotions_tables_exist",
				Check: func(tx *gorm.DB) error {
					required := []any{
						&models.DiscountCampaign{},
						&models.DiscountRule{},
						&models.DiscountLevel{},
						&models.DiscountTarget{},
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
			for _, model := range []any{
				&models.DiscountCampaign{},
				&models.DiscountRule{},
				&models.DiscountLevel{},
				&models.DiscountTarget{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE INDEX IF NOT EXISTS idx_discount_campaigns_active_window ON discount_campaigns (type, status, is_archived, starts_at, ends_at)`,
				`CREATE INDEX IF NOT EXISTS idx_discount_targets_product_lookup ON discount_targets (target_type, target_id, campaign_id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         discountsPromotionsP2Version,
		Name:            "add discount scheduling and lifecycle history",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "discounts", "scheduling"},
		PostChecks: []PostCheck{
			{
				Name: "discount_scheduling_tables_exist",
				Check: func(tx *gorm.DB) error {
					required := []any{
						&models.DiscountSchedule{},
						&models.DiscountStateHistory{},
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
			for _, model := range []any{
				&models.DiscountSchedule{},
				&models.DiscountStateHistory{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE INDEX IF NOT EXISTS idx_discount_schedules_next_run ON discount_schedules (next_run_at, schedule_type)`,
				`CREATE INDEX IF NOT EXISTS idx_discount_state_history_campaign_changed ON discount_state_histories (campaign_id, changed_at)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         discountsPromotionsP3Version,
		Name:            "add promotion templates and advanced controls",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "discounts", "templates"},
		PostChecks: []PostCheck{
			{
				Name: "discount_templates_and_redemptions_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{&models.PromotionTemplate{}, &models.DiscountRedemption{}} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing migrated table for %T", model)
						}
					}
					for _, column := range []string{"metadata_json", "coupon_code", "channels_json", "customer_segment", "global_usage_cap", "per_customer_usage_cap"} {
						if !tx.Migrator().HasColumn(&models.DiscountCampaign{}, column) {
							return fmt.Errorf("discount_campaigns.%s column missing", column)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			columns := []struct {
				name string
				sql  string
			}{
				{"metadata_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"coupon_code", "TEXT"},
				{"channels_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"customer_segment", "TEXT NOT NULL DEFAULT ''"},
				{"global_usage_cap", "BIGINT"},
				{"per_customer_usage_cap", "BIGINT"},
			}
			for _, column := range columns {
				if err := ops.AddColumnIfNotExists(tx, "discount_campaigns", column.name, column.sql); err != nil {
					return err
				}
			}
			for _, model := range []any{
				&models.PromotionTemplate{},
				&models.DiscountRedemption{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_discount_campaigns_coupon_code ON discount_campaigns (coupon_code) WHERE coupon_code IS NOT NULL AND coupon_code <> ''`,
				`CREATE INDEX IF NOT EXISTS idx_discount_redemptions_campaign_customer ON discount_redemptions (campaign_id, customer_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_discount_redemptions_campaign_order ON discount_redemptions (campaign_id, order_id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         discountsPromotionsP4Version,
		Name:            "add discount operational audit and lookup indexes",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "discounts", "operations"},
		PostChecks: []PostCheck{
			{
				Name: "discount_campaign_audits_exist",
				Check: func(tx *gorm.DB) error {
					if !tx.Migrator().HasTable(&models.DiscountCampaignAudit{}) {
						return fmt.Errorf("missing migrated table for %T", &models.DiscountCampaignAudit{})
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.DiscountCampaignAudit{}); err != nil {
				return err
			}
			statements := []string{
				`CREATE INDEX IF NOT EXISTS idx_discount_campaign_audits_campaign_changed ON discount_campaign_audits (campaign_id, changed_at)`,
				`CREATE INDEX IF NOT EXISTS idx_discount_campaigns_runtime_lookup ON discount_campaigns (status, is_archived, starts_at, ends_at, priority, id)`,
				`CREATE INDEX IF NOT EXISTS idx_discount_targets_category_lookup ON discount_targets (target_type, target_id, campaign_id, level_id)`,
				`CREATE INDEX IF NOT EXISTS idx_discount_targets_level_lookup ON discount_targets (level_id, target_type, target_id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSP0Version,
		Name:            "add ecommerce cms foundation tables",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "content"},
		PostChecks: []PostCheck{
			{
				Name: "cms_foundation_tables_exist",
				Check: func(tx *gorm.DB) error {
					required := []any{
						&models.CMSEntry{},
						&models.CMSEntryVersion{},
						&models.CMSPublication{},
						&models.CMSPage{},
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
			for _, model := range []any{
				&models.CMSEntry{},
				&models.CMSEntryVersion{},
				&models.CMSPublication{},
				&models.CMSPage{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_entries_type_key_live ON cms_entries (entry_type, key) WHERE deleted_at IS NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_versions_entry_number_unique ON cms_entry_versions (entry_id, version_number)`,
				`CREATE INDEX IF NOT EXISTS idx_cms_publications_entry_published ON cms_publications (entry_id, published_at DESC, id DESC)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_pages_path_live ON cms_pages (path) WHERE deleted_at IS NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_pages_homepage_live ON cms_pages (is_homepage) WHERE is_homepage = true AND deleted_at IS NULL`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSP2Version,
		Name:            "add ecommerce cms navigation and global content",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "content", "navigation"},
		PostChecks: []PostCheck{
			{
				Name: "cms_navigation_global_tables_exist",
				Check: func(tx *gorm.DB) error {
					required := []any{
						&models.CMSNavigationMenu{},
						&models.CMSNavigationItem{},
						&models.CMSGlobalRegion{},
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
			for _, model := range []any{
				&models.CMSNavigationMenu{},
				&models.CMSNavigationItem{},
				&models.CMSGlobalRegion{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_navigation_menus_key_live ON cms_navigation_menus (key) WHERE deleted_at IS NULL`,
				`CREATE INDEX IF NOT EXISTS idx_cms_navigation_menus_location ON cms_navigation_menus (location, key)`,
				`CREATE INDEX IF NOT EXISTS idx_cms_navigation_items_menu_order ON cms_navigation_items (menu_id, parent_id, sort_order, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_global_regions_key_live ON cms_global_regions (key) WHERE deleted_at IS NULL`,
				`CREATE INDEX IF NOT EXISTS idx_cms_global_regions_region ON cms_global_regions (region, key)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSP4Version,
		Name:            "add ecommerce cms scheduling targeting and experiments",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "content", "experimentation"},
		PostChecks: []PostCheck{
			{
				Name: "cms_delivery_tables_exist",
				Check: func(tx *gorm.DB) error {
					for _, model := range []any{
						&models.CMSSchedule{},
						&models.CMSTargetingRule{},
						&models.CMSExperiment{},
						&models.CMSExperimentVariant{},
						&models.CMSExposureEvent{},
					} {
						if !tx.Migrator().HasTable(model) {
							return fmt.Errorf("missing migrated table for %T", model)
						}
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{
				&models.CMSSchedule{},
				&models.CMSTargetingRule{},
				&models.CMSExperiment{},
				&models.CMSExperimentVariant{},
				&models.CMSExposureEvent{},
			} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE INDEX IF NOT EXISTS idx_cms_schedules_due ON cms_schedules (status, publish_at, unpublish_at)`,
				`CREATE INDEX IF NOT EXISTS idx_cms_targeting_rules_entry_priority ON cms_targeting_rules (entry_id, is_enabled, priority, id)`,
				`CREATE INDEX IF NOT EXISTS idx_cms_experiments_runtime ON cms_experiments (entry_id, status, starts_at, ends_at)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_experiment_variants_name ON cms_experiment_variants (experiment_id, name)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_exposure_event_dedupe ON cms_exposure_events (correlation_id, event_type, content_version_id)`,
			}
			for _, statement := range statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSMediaIDsVersion,
		Name:            "store ecommerce cms media references by id",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"backfill", "cms", "media"},
		Up: func(tx *gorm.DB) error {
			return migrateCMSMediaIDs(tx)
		},
	},
	{
		Version:         ecommerceCMSP5Version,
		Name:            "add ecommerce cms seo metadata and redirects",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "seo", "redirects"},
		PostChecks: []PostCheck{{
			Name: "cms_redirects_and_seo_fields_exist",
			Check: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable(&models.CMSRedirectRule{}) {
					return errors.New("missing cms redirect rules table")
				}
				for _, field := range []string{"Robots", "OGTitle", "OGDescription", "TwitterCard", "TwitterTitle", "TwitterDescription", "TwitterImageMediaID", "JSONLD"} {
					if !tx.Migrator().HasColumn(&models.SEOMetadata{}, field) {
						return fmt.Errorf("missing seo metadata column %s", field)
					}
				}
				return nil
			},
		}},
		Up: func(tx *gorm.DB) error {
			if err := ops.CreateTableIfNotExists(tx, &models.CMSRedirectRule{}); err != nil {
				return err
			}
			for _, field := range []string{"Robots", "OGTitle", "OGDescription", "TwitterCard", "TwitterTitle", "TwitterDescription", "TwitterImageMediaID", "JSONLD"} {
				if !tx.Migrator().HasColumn(&models.SEOMetadata{}, field) {
					if err := tx.Migrator().AddColumn(&models.SEOMetadata{}, field); err != nil {
						return err
					}
				}
			}
			for _, statement := range []string{
				`CREATE INDEX IF NOT EXISTS idx_cms_redirect_rules_match ON cms_redirect_rules (is_enabled, match_type, priority, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_redirect_rules_source_live ON cms_redirect_rules (source_pattern, match_type) WHERE deleted_at IS NULL`,
			} {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSP6Version,
		Name:            "add ecommerce cms localization variants governance and audit",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "localization", "governance"},
		PostChecks: []PostCheck{{
			Name: "cms_p6_tables_and_default_locale_exist",
			Check: func(tx *gorm.DB) error {
				for _, model := range []any{&models.CMSLocale{}, &models.CMSPageVariant{}, &models.CMSAuditEvent{}, &models.CMSChangeComment{}, &models.CMSRoleAssignment{}, &models.CMSInvalidationEvent{}} {
					if !tx.Migrator().HasTable(model) {
						return fmt.Errorf("missing migrated table for %T", model)
					}
				}
				var count int64
				if err := tx.Model(&models.CMSLocale{}).Where("is_default = ?", true).Count(&count).Error; err != nil {
					return err
				}
				if count != 1 {
					return fmt.Errorf("expected exactly one default CMS locale, found %d", count)
				}
				return nil
			},
		}},
		Up: func(tx *gorm.DB) error {
			for _, model := range []any{&models.CMSLocale{}, &models.CMSPageVariant{}, &models.CMSAuditEvent{}, &models.CMSChangeComment{}, &models.CMSRoleAssignment{}, &models.CMSInvalidationEvent{}} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, statement := range []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_page_variant_scope_live ON cms_page_variants (page_id, locale, market) WHERE deleted_at IS NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_page_variant_path_live ON cms_page_variants (path, locale, market) WHERE deleted_at IS NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_locale_default ON cms_locales (is_default) WHERE is_default = TRUE AND deleted_at IS NULL`,
				`CREATE INDEX IF NOT EXISTS idx_cms_audit_entry_created ON cms_audit_events (entry_id, created_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS idx_cms_invalidation_pending ON cms_invalidation_events (status, created_at, id)`,
			} {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			locale := models.CMSLocale{Code: "en-US", Name: "English (United States)", Enabled: true, IsDefault: true}
			return tx.Where("code = ?", locale.Code).FirstOrCreate(&locale).Error
		},
	},
	{
		Version:         ecommerceCMSCompletionVersion,
		Name:            "complete ecommerce cms governance localization and operations",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "cms", "governance", "operations"},
		Up: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&models.WebsiteSettings{}, "SiteTitle") {
				if err := tx.Migrator().AddColumn(&models.WebsiteSettings{}, "SiteTitle"); err != nil {
					return err
				}
			}
			for _, model := range []any{&models.CMSEntryWorkflow{}, &models.CMSContentVariant{}, &models.CMSSettings{}} {
				if err := ops.CreateTableIfNotExists(tx, model); err != nil {
					return err
				}
			}
			for _, field := range []string{"ResolvedBy", "ResolvedAt"} {
				if !tx.Migrator().HasColumn(&models.CMSChangeComment{}, field) {
					if err := tx.Migrator().AddColumn(&models.CMSChangeComment{}, field); err != nil {
						return err
					}
				}
			}
			for _, field := range []string{"Attempts", "LastError"} {
				if !tx.Migrator().HasColumn(&models.CMSInvalidationEvent{}, field) {
					if err := tx.Migrator().AddColumn(&models.CMSInvalidationEvent{}, field); err != nil {
						return err
					}
				}
			}
			for _, statement := range []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_cms_content_variant_scope_live ON cms_content_variants (entry_id, locale, market) WHERE deleted_at IS NULL`,
				`CREATE INDEX IF NOT EXISTS idx_cms_entry_workflow_status ON cms_entry_workflows (status, updated_at, id)`,
			} {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			settings := models.CMSSettings{ID: 1, ApprovalRequired: true}
			return tx.Where("id = ?", settings.ID).FirstOrCreate(&settings).Error
		},
	},
	{
		Version:          ecommerceCMSLegacyRemovalVersion,
		Name:             "remove legacy storefront settings after cms cutover",
		TransactionMode:  TransactionModeRequired,
		Tags:             []string{"contract", "cms", "storefront"},
		ContractBlockers: []string{"allow_contract_migrations"},
		PostChecks: []PostCheck{{
			Name: "legacy_storefront_removed_and_homepage_available",
			Check: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("storefront_settings") {
					return errors.New("legacy storefront_settings table still exists")
				}
				var count int64
				if err := tx.Model(&models.CMSPage{}).Where("is_homepage = ?", true).Count(&count).Error; err != nil {
					return err
				}
				if count != 1 {
					return fmt.Errorf("expected one CMS homepage, found %d", count)
				}
				return nil
			},
		}},
		Up: func(tx *gorm.DB) error {
			if err := backfillLegacyStorefrontIntoCMS(tx); err != nil {
				return err
			}
			if tx.Migrator().HasTable("storefront_settings") {
				return tx.Exec(`DROP TABLE storefront_settings`).Error
			}
			return nil
		},
	},
	{
		Version:         ecommerceCMSFooterBackfillVersion,
		Name:            "backfill empty cms footer global region",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"backfill", "cms", "storefront"},
		PostChecks: []PostCheck{{
			Name: "published_footer_has_renderable_blocks",
			Check: func(tx *gorm.DB) error {
				var rows []struct {
					PayloadJSON string
				}
				if err := tx.Session(&gorm.Session{NewDB: true}).Table("cms_global_regions").
					Select("cms_entry_versions.payload_json").
					Joins("JOIN cms_entries ON cms_entries.id = cms_global_regions.entry_id").
					Joins("JOIN cms_entry_versions ON cms_entry_versions.id = cms_entries.published_version_id").
					Where("cms_global_regions.region = ?", "footer").
					Find(&rows).Error; err != nil {
					return err
				}
				for _, row := range rows {
					if cmsPayloadBlocksEmpty(row.PayloadJSON) {
						return errors.New("published footer CMS region has no renderable blocks")
					}
				}
				return nil
			},
		}},
		Up: backfillEmptyCMSFooterRegion,
	},
}

type productAttributeEnumBackfillRow struct {
	ProductAttributeID uint
	EnumValue          string
}

func migrateCMSMediaIDs(tx *gorm.DB) error {
	var versions []models.CMSEntryVersion
	if err := tx.Find(&versions).Error; err != nil {
		return err
	}
	for _, version := range versions {
		payload, changed, err := migrateCMSPayloadMediaIDs(version.PayloadJSON)
		if err != nil {
			return fmt.Errorf("migrate cms version %d media: %w", version.ID, err)
		}
		if changed {
			if err := tx.Model(&models.CMSEntryVersion{}).Where("id = ?", version.ID).Update("payload_json", payload).Error; err != nil {
				return err
			}
		}
	}
	var entries []models.CMSEntry
	if err := tx.Find(&entries).Error; err != nil {
		return err
	}
	for _, entry := range entries {
		for _, current := range []struct {
			versionID *uint
			role      string
		}{
			{entry.CurrentVersionID, media.RoleCMSDraftContent},
			{entry.PublishedVersionID, media.RoleCMSContent},
		} {
			if current.versionID == nil {
				continue
			}
			var version models.CMSEntryVersion
			if err := tx.First(&version, *current.versionID).Error; err != nil {
				return err
			}
			ids, err := cmsPayloadMediaIDs(version.PayloadJSON)
			if err != nil {
				return err
			}
			for position, mediaID := range ids {
				var count int64
				if err := tx.Model(&models.MediaObject{}).Where("id = ?", mediaID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					continue
				}
				ref := models.MediaReference{MediaID: mediaID, OwnerType: media.OwnerTypeCMSEntry, OwnerID: entry.ID, Role: current.role, Position: position}
				if err := tx.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?", mediaID, ref.OwnerType, ref.OwnerID, ref.Role).FirstOrCreate(&ref).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func migrateCMSPayloadMediaIDs(raw string) (string, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false, err
	}
	blocks, _ := payload["blocks"].([]any)
	changed := false
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			continue
		}
		if id := mediaIDFromCMSURL(block["image_url"]); id != "" {
			block["image_media_id"] = id
			delete(block, "image_url")
			changed = true
		}
		if id := mediaIDFromCMSURL(block["url"]); id != "" && block["type"] == "image" {
			block["media_id"] = id
			delete(block, "url")
			changed = true
		}
		if images, ok := block["images"].([]any); ok {
			for _, rawImage := range images {
				if image, ok := rawImage.(map[string]any); ok {
					if id := mediaIDFromCMSURL(image["url"]); id != "" {
						image["media_id"] = id
						delete(image, "url")
						changed = true
					}
				}
			}
		}
		if images, ok := block["category_images"].(map[string]any); ok {
			mediaIDs := map[string]any{}
			for slug, rawURL := range images {
				if id := mediaIDFromCMSURL(rawURL); id != "" {
					mediaIDs[slug] = id
				}
			}
			block["category_media_ids"] = mediaIDs
			delete(block, "category_images")
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	encoded, err := json.Marshal(payload)
	return string(encoded), true, err
}

func cmsPayloadMediaIDs(raw string) ([]string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	ids := []string{}
	add := func(value any) {
		id, ok := value.(string)
		if ok && strings.TrimSpace(id) != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	blocks, _ := payload["blocks"].([]any)
	for _, rawBlock := range blocks {
		block, _ := rawBlock.(map[string]any)
		add(block["image_media_id"])
		add(block["media_id"])
		if images, ok := block["images"].([]any); ok {
			for _, rawImage := range images {
				image, _ := rawImage.(map[string]any)
				add(image["media_id"])
			}
		}
		if images, ok := block["category_media_ids"].(map[string]any); ok {
			for _, mediaID := range images {
				add(mediaID)
			}
		}
	}
	return ids, nil
}

func mediaIDFromCMSURL(value any) string {
	raw, ok := value.(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "media" || parts[2] != "original.webp" {
		return ""
	}
	id, err := url.PathUnescape(parts[1])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(id)
}

func backfillLegacyStorefrontIntoCMS(tx *gorm.DB) error {
	db := tx.Session(&gorm.Session{NewDB: true})
	var legacy struct {
		ConfigJSON string
	}
	if db.Migrator().HasTable("storefront_settings") {
		result := db.Table("storefront_settings").Select("config_json").Order("id ASC").Limit(1).Scan(&legacy)
		if result.Error != nil {
			return result.Error
		}
	}
	var config map[string]any
	if strings.TrimSpace(legacy.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(legacy.ConfigJSON), &config); err != nil {
			return fmt.Errorf("decode legacy storefront for CMS cutover: %w", err)
		}
	}
	if config == nil {
		config = map[string]any{}
	}
	if siteTitle, ok := config["site_title"].(string); ok && strings.TrimSpace(siteTitle) != "" {
		if err := db.Model(&models.WebsiteSettings{}).Where("id = ?", models.WebsiteSettingsSingletonID).Update("site_title", strings.TrimSpace(siteTitle)).Error; err != nil {
			return err
		}
	}

	var homepageCount int64
	if err := db.Model(&models.CMSPage{}).Where("is_homepage = ?", true).Count(&homepageCount).Error; err != nil {
		return err
	}
	if homepageCount == 0 {
		blocks := legacyHomepageBlocks(config)
		if len(blocks) == 0 {
			blocks = []any{map[string]any{"type": "hero", "title": "Welcome", "subtitle": ""}}
		}
		payload, err := json.Marshal(map[string]any{"blocks": blocks})
		if err != nil {
			return err
		}
		if err := backfillLegacyHomepageIntoCMS(db, payload); err != nil {
			return err
		}
	}
	return backfillLegacyFooterIntoCMS(db, config)
}

// backfillLegacyHomepageIntoCMS ensures a published CMS homepage exists at "/",
// reusing an existing page entry when one is already present (for example a
// draft created during development) so the cutover does not collide with the
// unique (entry_type, key) index on cms_entries.
func backfillLegacyHomepageIntoCMS(db *gorm.DB, legacyPayload []byte) error {
	var entry models.CMSEntry
	err := db.Where("entry_type = ? AND key = ?", models.CMSEntryTypePage, "/").First(&entry).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return createLegacyHomepageEntry(db, legacyPayload)
	case err != nil:
		return err
	default:
		return adoptExistingHomepageEntry(db, &entry, legacyPayload)
	}
}

func createLegacyHomepageEntry(db *gorm.DB, legacyPayload []byte) error {
	entry := models.CMSEntry{EntryType: models.CMSEntryTypePage, Key: "/", Status: models.CMSEntryStatusPublished}
	if err := db.Create(&entry).Error; err != nil {
		return err
	}
	version := models.CMSEntryVersion{EntryID: entry.ID, VersionNumber: 1, SchemaVersion: 1, PayloadJSON: string(legacyPayload), ChangeSummary: "Migrated from legacy storefront", CreatedAt: time.Now().UTC()}
	if err := db.Create(&version).Error; err != nil {
		return err
	}
	entry.CurrentVersionID, entry.PublishedVersionID = &version.ID, &version.ID
	if err := db.Save(&entry).Error; err != nil {
		return err
	}
	page := models.CMSPage{EntryID: entry.ID, Path: "/", Slug: "home", Title: "Home", TemplateKey: "default", Visibility: models.CMSPageVisibilityPublic, IsHomepage: true}
	if err := db.Select("*").Create(&page).Error; err != nil {
		return err
	}
	return db.Create(&models.CMSPublication{EntryID: entry.ID, VersionID: version.ID, PublishedAt: time.Now().UTC(), Notes: "Legacy storefront cutover"}).Error
}

// adoptExistingHomepageEntry publishes a pre-existing cms_entries page at "/"
// as the homepage, backfilling the legacy storefront content as a new
// published version so the prior draft content is retained as history.
func adoptExistingHomepageEntry(db *gorm.DB, entry *models.CMSEntry, legacyPayload []byte) error {
	now := time.Now().UTC()

	var maxVersion uint
	row := db.Model(&models.CMSEntryVersion{}).
		Where("entry_id = ?", entry.ID).
		Select("COALESCE(MAX(version_number), 0)").
		Row()
	if err := row.Scan(&maxVersion); err != nil {
		return fmt.Errorf("homepage cutover: determine next version number: %w", err)
	}

	version := models.CMSEntryVersion{EntryID: entry.ID, VersionNumber: maxVersion + 1, SchemaVersion: 1, PayloadJSON: string(legacyPayload), ChangeSummary: "Migrated from legacy storefront", CreatedAt: now}
	if err := db.Create(&version).Error; err != nil {
		return err
	}
	entry.CurrentVersionID, entry.PublishedVersionID = &version.ID, &version.ID
	entry.Status = models.CMSEntryStatusPublished
	if err := db.Model(&models.CMSEntry{}).Where("id = ?", entry.ID).Updates(map[string]any{
		"status":               entry.Status,
		"current_version_id":   entry.CurrentVersionID,
		"published_version_id": entry.PublishedVersionID,
	}).Error; err != nil {
		return err
	}

	var pubCount int64
	if err := db.Model(&models.CMSPublication{}).Where("entry_id = ? AND version_id = ?", entry.ID, version.ID).Count(&pubCount).Error; err != nil {
		return err
	}
	if pubCount == 0 {
		if err := db.Create(&models.CMSPublication{EntryID: entry.ID, VersionID: version.ID, PublishedAt: now, Notes: "Legacy storefront cutover"}).Error; err != nil {
			return err
		}
	}

	var page models.CMSPage
	if err := db.Where("entry_id = ?", entry.ID).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			page = models.CMSPage{EntryID: entry.ID, Path: "/", Slug: "home", Title: "Home", TemplateKey: "default", Visibility: models.CMSPageVisibilityPublic, IsHomepage: true}
			return db.Select("*").Create(&page).Error
		}
		return err
	}
	updates := map[string]any{"is_homepage": true, "path": "/"}
	if strings.TrimSpace(page.Slug) == "" {
		updates["slug"] = "home"
	}
	if strings.TrimSpace(page.Title) == "" {
		updates["title"] = "Home"
	}
	if strings.TrimSpace(page.TemplateKey) == "" {
		updates["template_key"] = "default"
	}
	if strings.TrimSpace(string(page.Visibility)) == "" {
		updates["visibility"] = models.CMSPageVisibilityPublic
	}
	return db.Model(&page).Updates(updates).Error
}

func legacyHomepageBlocks(config map[string]any) []any {
	sections, _ := config["homepage_sections"].([]any)
	blocks := make([]any, 0, len(sections))
	for _, raw := range sections {
		section, _ := raw.(map[string]any)
		if enabled, exists := section["enabled"].(bool); exists && !enabled {
			continue
		}
		switch section["type"] {
		case "hero":
			hero, _ := section["hero"].(map[string]any)
			block := map[string]any{"type": "hero", "title": stringFromMap(hero, "title"), "subtitle": stringFromMap(hero, "subtitle")}
			if mediaID := stringFromMap(hero, "background_image_media_id"); mediaID != "" {
				block["image_media_id"] = mediaID
			}
			if cta, ok := hero["primary_cta"].(map[string]any); ok {
				block["primary_cta"] = map[string]any{"label": stringFromMap(cta, "label"), "url": stringFromMap(cta, "url")}
			}
			blocks = append(blocks, block)
		case "products":
			product, _ := section["product_section"].(map[string]any)
			block := map[string]any{"type": "product_rail", "title": stringFromMap(product, "title"), "subtitle": stringFromMap(product, "subtitle"), "source": stringFromMap(product, "source"), "limit": numberFromMap(product, "limit", 8)}
			for _, key := range []string{"query", "category_slug", "sort", "order", "image_aspect"} {
				if value := stringFromMap(product, key); value != "" {
					block[key] = value
				}
			}
			if ids, ok := product["product_ids"].([]any); ok {
				block["product_ids"] = ids
			}
			blocks = append(blocks, block)
		case "promo_cards":
			cards, _ := section["promo_cards"].([]any)
			for _, rawCard := range cards {
				card, _ := rawCard.(map[string]any)
				block := map[string]any{"type": "promo_banner", "title": stringFromMap(card, "title"), "body": stringFromMap(card, "description")}
				if link, ok := card["link"].(map[string]any); ok {
					block["link"] = map[string]any{"label": stringFromMap(link, "label"), "url": stringFromMap(link, "url")}
				}
				blocks = append(blocks, block)
			}
		case "badges":
			badges, _ := section["badges"].([]any)
			values := make([]string, 0, len(badges))
			for _, badge := range badges {
				if value, ok := badge.(string); ok && value != "" {
					values = append(values, value)
				}
			}
			if len(values) > 0 {
				blocks = append(blocks, map[string]any{"type": "rich_text", "body": strings.Join(values, " · ")})
			}
		}
	}
	return blocks
}

func backfillLegacyFooterIntoCMS(tx *gorm.DB, config map[string]any) error {
	var count int64
	if err := tx.Model(&models.CMSGlobalRegion{}).Where("region = ?", "footer").Count(&count).Error; err != nil || count > 0 {
		return err
	}
	payload, err := json.Marshal(map[string]any{"blocks": legacyFooterBlocks(config)})
	if err != nil {
		return err
	}
	entry := models.CMSEntry{EntryType: models.CMSEntryTypeGlobal, Key: "global:footer", Status: models.CMSEntryStatusPublished}
	if err := tx.Create(&entry).Error; err != nil {
		return err
	}
	version := models.CMSEntryVersion{EntryID: entry.ID, VersionNumber: 1, SchemaVersion: 1, PayloadJSON: string(payload), ChangeSummary: "Migrated legacy footer", CreatedAt: time.Now().UTC()}
	if err := tx.Create(&version).Error; err != nil {
		return err
	}
	entry.CurrentVersionID, entry.PublishedVersionID = &version.ID, &version.ID
	if err := tx.Save(&entry).Error; err != nil {
		return err
	}
	region := models.CMSGlobalRegion{EntryID: entry.ID, Key: "footer", Title: "Footer", Region: "footer"}
	if err := tx.Create(&region).Error; err != nil {
		return err
	}
	return tx.Create(&models.CMSPublication{EntryID: entry.ID, VersionID: version.ID, PublishedAt: time.Now().UTC(), Notes: "Legacy storefront cutover"}).Error
}

func backfillEmptyCMSFooterRegion(tx *gorm.DB) error {
	payload, err := json.Marshal(map[string]any{"blocks": defaultStructuredFooterBlocks()})
	if err != nil {
		return err
	}
	var versionIDs []uint
	if err := tx.Session(&gorm.Session{NewDB: true}).Raw(`
		SELECT DISTINCT version_id
		FROM (
			SELECT cms_entries.current_version_id AS version_id
			FROM cms_global_regions
			JOIN cms_entries ON cms_entries.id = cms_global_regions.entry_id
			WHERE cms_global_regions.region = ? AND cms_entries.current_version_id IS NOT NULL
			UNION
			SELECT cms_entries.published_version_id AS version_id
			FROM cms_global_regions
			JOIN cms_entries ON cms_entries.id = cms_global_regions.entry_id
			WHERE cms_global_regions.region = ? AND cms_entries.published_version_id IS NOT NULL
		) footer_versions
	`, "footer", "footer").Scan(&versionIDs).Error; err != nil {
		return err
	}
	if len(versionIDs) == 0 {
		return nil
	}
	var versions []models.CMSEntryVersion
	if err := tx.Session(&gorm.Session{NewDB: true}).Where("id IN ?", versionIDs).Find(&versions).Error; err != nil {
		return err
	}
	for _, version := range versions {
		if !cmsPayloadBlocksEmpty(version.PayloadJSON) {
			continue
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Model(&models.CMSEntryVersion{}).
			Where("id = ?", version.ID).
			Update("payload_json", string(payload)).Error; err != nil {
			return err
		}
	}
	return nil
}

func legacyFooterBlocks(config map[string]any) []any {
	footer, _ := config["footer"].(map[string]any)
	lines := []string{stringFromMap(footer, "tagline"), stringFromMap(footer, "bottom_notice"), stringFromMap(footer, "copyright")}
	blocks := make([]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			blocks = append(blocks, map[string]any{"type": "rich_text", "body": line})
		}
	}
	if len(blocks) > 0 {
		return blocks
	}
	return defaultStructuredFooterBlocks()
}

func defaultStructuredFooterBlocks() []any {
	year := time.Now().UTC().Year()
	return []any{map[string]any{
		"type":       "footer",
		"brand_name": "Store",
		"tagline":    "Thoughtfully selected products for everyday use.",
		"columns": []any{
			map[string]any{
				"title": "Shop",
				"links": []any{
					map[string]any{"label": "All products", "url": "/search"},
					map[string]any{"label": "New arrivals", "url": "/search?sort=created_at"},
				},
			},
			map[string]any{
				"title": "Help",
				"links": []any{
					map[string]any{"label": "Shipping", "url": "/shipping"},
					map[string]any{"label": "Returns", "url": "/returns"},
				},
			},
		},
		"social_links": []any{},
		"copyright":    fmt.Sprintf("\u00a9 %d Store", year),
		"layout":       "columns",
		"theme":        "light",
	}}
}

func cmsPayloadBlocksEmpty(raw string) bool {
	var payload struct {
		Blocks []any `json:"blocks"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	return len(payload.Blocks) == 0
}

func stringFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func numberFromMap(values map[string]any, key string, fallback float64) float64 {
	value, ok := values[key].(float64)
	if !ok || value <= 0 {
		return fallback
	}
	return value
}

func backfillProductAttributeEnumValues(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []productAttributeEnumBackfillRow
	if err := queryDB.Raw(`
		SELECT product_attribute_id, enum_value
		FROM product_attribute_values
		WHERE enum_value IS NOT NULL AND TRIM(enum_value) <> ''
		UNION ALL
		SELECT product_attribute_id, enum_value
		FROM product_attribute_value_drafts
		WHERE enum_value IS NOT NULL AND TRIM(enum_value) <> ''
	`).Scan(&rows).Error; err != nil {
		return fmt.Errorf("backfill product attribute enum values: %w", err)
	}

	valuesByAttributeID := map[uint][]string{}
	seenByAttributeID := map[uint]map[string]struct{}{}
	for _, row := range rows {
		value := strings.TrimSpace(row.EnumValue)
		if row.ProductAttributeID == 0 || value == "" {
			continue
		}
		if _, exists := seenByAttributeID[row.ProductAttributeID]; !exists {
			seenByAttributeID[row.ProductAttributeID] = map[string]struct{}{}
		}
		lookup := strings.ToLower(value)
		if _, exists := seenByAttributeID[row.ProductAttributeID][lookup]; exists {
			continue
		}
		seenByAttributeID[row.ProductAttributeID][lookup] = struct{}{}
		valuesByAttributeID[row.ProductAttributeID] = append(valuesByAttributeID[row.ProductAttributeID], value)
	}

	for attributeID, values := range valuesByAttributeID {
		sort.Strings(values)
		encoded, err := models.StringArray(values).Value()
		if err != nil {
			return fmt.Errorf("encode enum values for product attribute %d: %w", attributeID, err)
		}
		result := queryDB.Table("product_attributes").
			Where("id = ? AND type = ?", attributeID, "enum").
			Update("enum_values", encoded)
		if result.Error != nil {
			return fmt.Errorf("backfill enum values for product attribute %d: %w", attributeID, result.Error)
		}
		ops.AddRowsTouched(tx, result.RowsAffected)
	}

	return nil
}

func backfillInventoryItemsFromVariants(tx *gorm.DB) error {
	db := tx.Session(&gorm.Session{NewDB: true})
	type variantInventoryBackfillRow struct {
		ID    uint
		Stock int
	}

	var variants []variantInventoryBackfillRow
	if err := db.Table("product_variants").Select("id", "stock").Where("deleted_at IS NULL").Find(&variants).Error; err != nil {
		return err
	}

	for _, variant := range variants {
		var item models.InventoryItem
		err := db.Where("product_variant_id = ?", variant.ID).First(&item).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			item = models.InventoryItem{ProductVariantID: variant.ID}
			if err := db.Create(&item).Error; err != nil {
				return err
			}
		}

		var count int64
		if err := db.Model(&models.InventoryLevel{}).Where("inventory_item_id = ?", item.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			level := models.InventoryLevel{
				InventoryItemID: item.ID,
				OnHand:          variant.Stock,
				Reserved:        0,
				Available:       variant.Stock,
			}
			if err := db.Create(&level).Error; err != nil {
				return err
			}
		}
	}
	return nil
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

type legacyCartSessionBackfillRow struct {
	ID        uint
	UserID    uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

type legacyOrderSessionBackfillRow struct {
	ID                uint
	UserID            *uint
	GuestEmail        *string
	CheckoutSessionID *uint
	CreatedAt         time.Time
	UpdatedAt         time.Time
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

func backfillLegacyCartCheckoutSessions(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyCartSessionBackfillRow
	if err := queryDB.Table("carts").
		Select("id", "user_id", "created_at", "updated_at").
		Where("checkout_session_id IS NULL OR checkout_session_id = 0").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		lastSeenAt := row.UpdatedAt.UTC()
		if lastSeenAt.IsZero() {
			lastSeenAt = row.CreatedAt.UTC()
		}
		if lastSeenAt.IsZero() {
			lastSeenAt = time.Now().UTC()
		}

		session := models.CheckoutSession{
			PublicToken: uuid.NewString(),
			Status:      models.CheckoutSessionStatusActive,
			ExpiresAt:   lastSeenAt.Add(30 * 24 * time.Hour),
			LastSeenAt:  lastSeenAt,
		}
		if row.UserID != 0 {
			session.UserID = &row.UserID
		}
		if err := queryDB.Create(&session).Error; err != nil {
			return fmt.Errorf("create checkout session for cart %d: %w", row.ID, err)
		}
		if err := queryDB.Table("carts").Where("id = ?", row.ID).Update("checkout_session_id", session.ID).Error; err != nil {
			return fmt.Errorf("backfill carts.id=%d checkout_session_id: %w", row.ID, err)
		}
	}

	return nil
}

func backfillLegacyOrderCheckoutSessions(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	var rows []legacyOrderSessionBackfillRow
	if err := queryDB.Table("orders").
		Select("id", "user_id", "guest_email", "checkout_session_id", "created_at", "updated_at").
		Where("checkout_session_id IS NULL OR checkout_session_id = 0").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		lastSeenAt := row.UpdatedAt.UTC()
		if lastSeenAt.IsZero() {
			lastSeenAt = row.CreatedAt.UTC()
		}
		if lastSeenAt.IsZero() {
			lastSeenAt = time.Now().UTC()
		}

		session := models.CheckoutSession{
			PublicToken: uuid.NewString(),
			GuestEmail:  row.GuestEmail,
			Status:      models.CheckoutSessionStatusConverted,
			ExpiresAt:   lastSeenAt,
			LastSeenAt:  lastSeenAt,
		}
		if row.UserID != nil && *row.UserID != 0 {
			session.UserID = row.UserID
		}
		if err := queryDB.Create(&session).Error; err != nil {
			return fmt.Errorf("create checkout session for order %d: %w", row.ID, err)
		}
		if err := queryDB.Table("orders").Where("id = ?", row.ID).Update("checkout_session_id", session.ID).Error; err != nil {
			return fmt.Errorf("backfill orders.id=%d checkout_session_id: %w", row.ID, err)
		}
	}

	return nil
}

func backfillStorefrontCheckoutDefaults(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})

	type storefrontRow struct {
		ID              uint
		ConfigJSON      string
		DraftConfigJSON *string
	}

	var rows []storefrontRow
	if err := queryDB.Table("storefront_settings").
		Select("id", "config_json", "draft_config_json").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		updates := map[string]any{}

		configJSON, changed, err := ensureStorefrontCheckoutConfig(row.ConfigJSON)
		if err != nil {
			return fmt.Errorf("backfill storefront_settings.id=%d config_json: %w", row.ID, err)
		}
		if changed {
			updates["config_json"] = configJSON
		}

		if row.DraftConfigJSON != nil {
			draftJSON, draftChanged, err := ensureStorefrontCheckoutConfig(*row.DraftConfigJSON)
			if err != nil {
				return fmt.Errorf("backfill storefront_settings.id=%d draft_config_json: %w", row.ID, err)
			}
			if draftChanged {
				updates["draft_config_json"] = draftJSON
			}
		}

		if len(updates) == 0 {
			continue
		}
		if err := queryDB.Table("storefront_settings").Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update storefront_settings.id=%d: %w", row.ID, err)
		}
	}

	return nil
}

func ensureStorefrontCheckoutConfig(raw string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw, false, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", false, err
	}

	checkoutPayload, ok := payload["checkout"].(map[string]any)
	if !ok || checkoutPayload == nil {
		checkoutPayload = map[string]any{}
	}
	if _, exists := checkoutPayload["allow_guest_checkout"]; exists {
		return raw, false, nil
	}

	checkoutPayload["allow_guest_checkout"] = true
	payload["checkout"] = checkoutPayload
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	return string(encoded), true, nil
}

func legacyAllowGuestCheckout(tx *gorm.DB) (bool, error) {
	queryDB := tx.Session(&gorm.Session{NewDB: true})
	var row struct {
		ConfigJSON string
	}
	result := queryDB.Table("storefront_settings").
		Select("config_json").
		Where("id = ?", 1).
		Limit(1).
		Scan(&row)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return true, nil
	}
	return decodeLegacyAllowGuestCheckout(row.ConfigJSON)
}

func decodeLegacyAllowGuestCheckout(raw string) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return true, nil
	}
	var payload struct {
		Checkout *struct {
			AllowGuestCheckout *bool `json:"allow_guest_checkout"`
		} `json:"checkout"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, err
	}
	if payload.Checkout == nil || payload.Checkout.AllowGuestCheckout == nil {
		return true, nil
	}
	return *payload.Checkout.AllowGuestCheckout, nil
}

func stripStorefrontCheckoutSettings(tx *gorm.DB) error {
	queryDB := tx.Session(&gorm.Session{NewDB: true})
	type storefrontRow struct {
		ID              uint
		ConfigJSON      string
		DraftConfigJSON *string
	}
	var rows []storefrontRow
	if err := queryDB.Table("storefront_settings").
		Select("id", "config_json", "draft_config_json").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		updates := map[string]any{}
		configJSON, changed, err := stripStorefrontCheckoutConfig(row.ConfigJSON)
		if err != nil {
			return fmt.Errorf("strip storefront_settings.id=%d config_json: %w", row.ID, err)
		}
		if changed {
			updates["config_json"] = configJSON
		}
		if row.DraftConfigJSON != nil {
			draftJSON, draftChanged, err := stripStorefrontCheckoutConfig(*row.DraftConfigJSON)
			if err != nil {
				return fmt.Errorf("strip storefront_settings.id=%d draft_config_json: %w", row.ID, err)
			}
			if draftChanged {
				updates["draft_config_json"] = draftJSON
			}
		}
		if len(updates) == 0 {
			continue
		}
		if err := queryDB.Table("storefront_settings").Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update storefront_settings.id=%d: %w", row.ID, err)
		}
	}
	return nil
}

func stripStorefrontCheckoutConfig(raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false, err
	}
	if _, exists := payload["checkout"]; !exists {
		return raw, false, nil
	}
	delete(payload, "checkout")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	return string(encoded), true, nil
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
