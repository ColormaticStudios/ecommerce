package commands

import (
	"ecommerce/config"
	"ecommerce/internal/migrations"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func getConfig() config.Config {
	if err := requireLocalMode("local config access"); err != nil {
		log.Fatal(err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

func getDB() *gorm.DB {
	return getDBWithConfig(getConfig())
}

func getDBWithConfig(cfg config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)
	db = db.Session(&gorm.Session{Logger: gormLogger})

	if err := migrations.EnsureReady(db, cfg.AutoApplyMigrations); err != nil {
		log.Fatalf("Database migration readiness check failed: %v", err)
	}

	return db
}

func closeDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}
