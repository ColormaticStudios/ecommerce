package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var stubDir = "internal/migrations/stubs"

var slugPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

func GenerateStub(now time.Time, slug string) (filename string, content string, err error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if !slugPattern.MatchString(slug) {
		return "", "", fmt.Errorf("slug must match %s", slugPattern.String())
	}

	version := now.UTC().Format("20060102") + "01_" + slug
	fileName := fmt.Sprintf("%s.go", version)
	body := fmt.Sprintf(`package stubs

// %s
// Copy this entry into internal/migrations/migrations.go orderedMigrations.
//
// {
//   Version: "%s",
//   Name:    "TODO: describe migration intent",
//   TransactionMode: migrations.TransactionModeRequired, // or migrations.TransactionModeNone
//   Tags: []string{"expand"}, // use "contract" for destructive steps
//   // ContractBlockers: []string{"allow_contract_migrations"}, // required for contract migrations
//   // PostChecks: []migrations.PostCheck{
//   //   {
//   //     Name: "TODO_check_name",
//   //     Check: func(tx *gorm.DB) error {
//   //       return nil
//   //     },
//   //   },
//   // },
//   Up: func(tx *gorm.DB) error {
//     // Use helpers in internal/migrations/ops where possible.
//     return nil
//   },
// },
`, version, version)

	return fileName, body, nil
}

func WriteStub(now time.Time, slug string) (string, error) {
	fileName, content, err := GenerateStub(now, slug)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(stubDir, fileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
