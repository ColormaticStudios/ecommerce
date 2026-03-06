package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var schemaIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func main() {
	if len(os.Args) != 4 {
		log.Fatal(`usage: go run ./scripts/postgres-schema-dsn.go <create|drop> <base-dsn> <schema-name>`)
	}

	mode := strings.TrimSpace(os.Args[1])
	baseDSN := strings.TrimSpace(os.Args[2])
	schemaName := strings.TrimSpace(os.Args[3])

	if baseDSN == "" {
		log.Fatal("base dsn must not be empty")
	}
	if !schemaIdentifierPattern.MatchString(schemaName) {
		log.Fatalf("invalid schema name %q", schemaName)
	}

	db, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	switch mode {
	case "create":
		if err := db.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schemaName)).Error; err != nil {
			log.Fatalf("failed to create schema %q: %v", schemaName, err)
		}
		dsnWithSearchPath, err := withPostgresSearchPath(baseDSN, schemaName)
		if err != nil {
			log.Fatalf("failed to build isolated dsn: %v", err)
		}
		fmt.Println(dsnWithSearchPath)
	case "drop":
		if err := db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)).Error; err != nil {
			log.Fatalf("failed to drop schema %q: %v", schemaName, err)
		}
	default:
		log.Fatalf("unknown mode %q (expected: create or drop)", mode)
	}
}

func withPostgresSearchPath(dsn, searchPath string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("base dsn must be a URL-formatted postgres DSN")
	}
	query := parsed.Query()
	query.Set("search_path", searchPath)
	parsed.RawQuery = query.Encode()

	result := parsed.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("failed to build dsn with search_path")
	}
	return result, nil
}
