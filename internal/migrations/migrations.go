package migrations

import (
	"fmt"
	"sort"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// SchemaMigration tracks applied migration versions.
type SchemaMigration struct {
	Version   string    `gorm:"primaryKey;size:64"`
	AppliedAt time.Time `gorm:"not null"`
}

type Migration struct {
	Version string
	Name    string
	Up      func(tx *gorm.DB) error
}

var orderedMigrations = []Migration{
	{
		Version: "2026022601_initial_schema",
		Name:    "create core schema and backfill publish state",
		Up: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(
				&models.User{},
				&models.Product{},
				&models.Order{},
				&models.OrderItem{},
				&models.Cart{},
				&models.CartItem{},
				&models.MediaObject{},
				&models.MediaVariant{},
				&models.MediaReference{},
				&models.SavedPaymentMethod{},
				&models.SavedAddress{},
				&models.StorefrontSettings{},
				&models.CheckoutProviderSetting{},
			); err != nil {
				return err
			}

			return tx.Model(&models.Product{}).
				Where("is_published = ? AND (draft_data IS NULL OR draft_data = '')", false).
				Update("is_published", true).Error
		},
	},
}

func ensureTable(db *gorm.DB) error {
	return db.AutoMigrate(&SchemaMigration{})
}

func AppliedVersions(db *gorm.DB) (map[string]struct{}, error) {
	if err := ensureTable(db); err != nil {
		return nil, err
	}

	var rows []SchemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}

	applied := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		applied[row.Version] = struct{}{}
	}
	return applied, nil
}

func Pending(db *gorm.DB) ([]Migration, error) {
	applied, err := AppliedVersions(db)
	if err != nil {
		return nil, err
	}

	pending := make([]Migration, 0)
	for _, migration := range orderedMigrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		pending = append(pending, migration)
	}
	return pending, nil
}

func Run(db *gorm.DB) error {
	pending, err := Pending(db)
	if err != nil {
		return err
	}

	for _, migration := range pending {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}
			return tx.Create(&SchemaMigration{Version: migration.Version, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, err)
		}
	}
	return nil
}

func LatestVersion() string {
	if len(orderedMigrations) == 0 {
		return ""
	}
	return orderedMigrations[len(orderedMigrations)-1].Version
}

func Versions() []string {
	versions := make([]string, 0, len(orderedMigrations))
	for _, migration := range orderedMigrations {
		versions = append(versions, migration.Version)
	}
	sort.Strings(versions)
	return versions
}
