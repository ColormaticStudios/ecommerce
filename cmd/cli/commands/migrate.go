package commands

import (
	"fmt"
	"log"
	"time"

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
	migrateCmd.AddCommand(newMigratePlanCmd())
	migrateCmd.AddCommand(newMigrateCheckCmd())
	migrateCmd.AddCommand(newMigrateStatusCmd())
	migrateCmd.AddCommand(newMigrateLintCmd())
	migrateCmd.AddCommand(newMigrateGuardCmd())
	migrateCmd.AddCommand(newMigrateSnapshotCmd())
	migrateCmd.AddCommand(newMigrateDriftCheckCmd())
	migrateCmd.AddCommand(newMigrateNewCmd())

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

func newMigratePlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Show ordered pending migration steps",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			lines, err := migrations.PlanLines(db)
			if err != nil {
				log.Fatalf("Migration plan failed: %v", err)
			}
			for _, line := range lines {
				fmt.Println(line)
			}
		},
	}
}

func newMigrateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Fail if pending migrations exist",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			lines, err := migrations.StatusLines(db)
			if err != nil {
				log.Fatalf("Migration check failed: %v", err)
			}
			for _, line := range lines {
				fmt.Println(line)
			}
			if err := migrations.Check(db); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show latest known/applied migration and pending count",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			lines, err := migrations.StatusLines(db)
			if err != nil {
				log.Fatalf("Migration status failed: %v", err)
			}
			for _, line := range lines {
				fmt.Println(line)
			}
		},
	}
}

func newMigrateLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Run migration definition lint checks",
		Run: func(cmd *cobra.Command, args []string) {
			lines, err := migrations.LintLines()
			for _, line := range lines {
				fmt.Println(line)
			}
			if err != nil {
				log.Fatalf("Migration lint failed: %v", err)
			}
		},
	}
}

func newMigrateGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard",
		Short: "Validate readiness checks for pending contract migrations",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			lines, err := migrations.GuardLines(db)
			for _, line := range lines {
				fmt.Println(line)
			}
			if err != nil {
				log.Fatalf("Migration guard failed: %v", err)
			}
		},
	}
}

func newMigrateSnapshotCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Write schema snapshot artifact from current database",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			if path == "" {
				path = migrations.DefaultSchemaSnapshotPath()
			}
			if err := migrations.WriteSchemaSnapshot(db, path); err != nil {
				log.Fatalf("Migration snapshot failed: %v", err)
			}
			fmt.Printf("schema_snapshot_path=%s\n", path)
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "snapshot output path (default internal/migrations/schema_snapshot.sql)")
	return cmd
}

func newMigrateDriftCheckCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "drift-check",
		Short: "Fail if current schema differs from committed snapshot",
		Run: func(cmd *cobra.Command, args []string) {
			db := openMigrateDB()
			if path == "" {
				path = migrations.DefaultSchemaSnapshotPath()
			}
			if err := migrations.DriftCheck(db, path); err != nil {
				log.Fatalf("Migration drift check failed: %v", err)
			}
			fmt.Printf("schema_snapshot_path=%s\n", path)
			fmt.Println("drift_status=ok")
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "snapshot path to compare (default internal/migrations/schema_snapshot.sql)")
	return cmd
}

func newMigrateNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <slug>",
		Short: "Generate a migration stub file under internal/migrations/stubs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path, err := migrations.WriteStub(time.Now().UTC(), args[0])
			if err != nil {
				log.Fatalf("Migration stub generation failed: %v", err)
			}
			fmt.Printf("stub_path=%s\n", path)
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
