package commands

import (
	"errors"
	"fmt"
	"log"

	"ecommerce/config"
	"ecommerce/internal/migrations"

	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewMigrateCmd() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
		Long:  "Run or validate schema migrations against the configured database.",
	}

	migrateCmd.AddCommand(newMigrateUpCmd())
	migrateCmd.AddCommand(newMigrateCheckCmd())

	return migrateCmd
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			if err := migrations.Run(db); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
			fmt.Printf("✓ Migrations up-to-date at %s\n", migrations.LatestVersion())
		},
	}
}

func newMigrateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Fail if pending migrations exist",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			pending, err := migrations.Pending(db)
			if err != nil {
				log.Fatalf("Migration check failed: %v", err)
			}
			if len(pending) == 0 {
				fmt.Printf("✓ Migrations up-to-date at %s\n", migrations.LatestVersion())
				return
			}
			for _, migration := range pending {
				fmt.Printf("pending migration: %s %s\n", migration.Version, migration.Name)
			}
			log.Fatal(errors.New("database is not at latest migration"))
		},
	}
}

func openMigrateDB() *gorm.DB {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	return db
}
