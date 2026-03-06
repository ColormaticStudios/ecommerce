package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

	switch mode {
	case "up":
		db := openDB()
		if err := migrations.Run(db); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		fmt.Printf("migrations up-to-date at %s\n", migrations.LatestVersion())
	case "lint":
		lines, err := migrations.LintLines()
		for _, line := range lines {
			fmt.Println(line)
		}
		if err != nil {
			log.Fatalf("migration lint failed: %v", err)
		}
	case "guard":
		db := openDB()
		lines, err := migrations.GuardLines(db)
		for _, line := range lines {
			fmt.Println(line)
		}
		if err != nil {
			log.Fatalf("migration guard failed: %v", err)
		}
	case "plan":
		db := openDB()
		lines, err := migrations.PlanLines(db)
		if err != nil {
			log.Fatalf("migration plan failed: %v", err)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
	case "check":
		db := openDB()
		lines, err := migrations.StatusLines(db)
		if err != nil {
			log.Fatalf("migration check failed: %v", err)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		if err := migrations.Check(db); err != nil {
			log.Fatal(err)
		}
	case "status":
		db := openDB()
		lines, err := migrations.StatusLines(db)
		if err != nil {
			log.Fatalf("migration status failed: %v", err)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
	case "snapshot":
		db := openDB()
		path := migrations.DefaultSchemaSnapshotPath()
		if len(os.Args) > 2 && strings.TrimSpace(os.Args[2]) != "" {
			path = strings.TrimSpace(os.Args[2])
		}
		if err := migrations.WriteSchemaSnapshot(db, path); err != nil {
			log.Fatalf("failed to write schema snapshot: %v", err)
		}
		fmt.Printf("schema_snapshot_path=%s\n", path)
	case "drift-check":
		db := openDB()
		path := migrations.DefaultSchemaSnapshotPath()
		if len(os.Args) > 2 && strings.TrimSpace(os.Args[2]) != "" {
			path = strings.TrimSpace(os.Args[2])
		}
		if err := migrations.DriftCheck(db, path); err != nil {
			log.Fatalf("migration drift check failed: %v", err)
		}
		fmt.Printf("schema_snapshot_path=%s\n", path)
		fmt.Println("drift_status=ok")
	case "new":
		if len(os.Args) < 3 {
			log.Fatal(`usage: go run ./cmd/migrate new <slug>`)
		}
		slug := strings.TrimSpace(os.Args[2])
		path, err := migrations.WriteStub(time.Now().UTC(), slug)
		if err != nil {
			log.Fatalf("failed to create migration stub: %v", err)
		}
		fmt.Printf("stub_path=%s\n", path)
	default:
		log.Fatalf("unknown mode %q (expected: up, plan, check, status, lint, guard, snapshot, drift-check, new)", mode)
	}
}

func openDB() *gorm.DB {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	return db
}
