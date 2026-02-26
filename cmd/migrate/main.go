package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"ecommerce/config"
	"ecommerce/internal/migrations"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	mode := "up"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	switch mode {
	case "up":
		if err := migrations.Run(db); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		fmt.Printf("migrations up-to-date at %s\n", migrations.LatestVersion())
	case "check":
		pending, err := migrations.Pending(db)
		if err != nil {
			log.Fatalf("migration check failed: %v", err)
		}
		if len(pending) == 0 {
			fmt.Printf("migrations up-to-date at %s\n", migrations.LatestVersion())
			return
		}
		for _, migration := range pending {
			fmt.Printf("pending migration: %s %s\n", migration.Version, migration.Name)
		}
		log.Fatal(errors.New("database is not at latest migration"))
	default:
		log.Fatalf("unknown mode %q (expected: up, check)", mode)
	}
}
