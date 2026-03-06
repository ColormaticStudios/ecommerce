package migrations

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gorm.io/gorm"
)

func SchemaSnapshot(db *gorm.DB) (string, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return "", fmt.Errorf("failed to list tables for snapshot: %w", err)
	}
	sort.Strings(tables)

	var builder strings.Builder
	for _, table := range tables {
		if strings.HasPrefix(table, "sqlite_") {
			continue
		}
		builder.WriteString(fmt.Sprintf("TABLE %s\n", table))
		columnTypes, err := db.Migrator().ColumnTypes(table)
		if err != nil {
			return "", fmt.Errorf("failed to list columns for table %s: %w", table, err)
		}
		columnNames := make([]string, 0, len(columnTypes))
		for _, column := range columnTypes {
			name := column.Name()
			columnNames = append(columnNames, name)
		}
		sort.Strings(columnNames)
		for _, columnName := range columnNames {
			builder.WriteString(fmt.Sprintf("  COLUMN %s\n", columnName))
		}

		indexes, err := tableIndexes(db, table)
		if err != nil {
			return "", err
		}
		for _, index := range indexes {
			builder.WriteString(fmt.Sprintf("  INDEX %s\n", index))
		}
	}

	return builder.String(), nil
}

func tableIndexes(db *gorm.DB, table string) ([]string, error) {
	indexes, err := db.Migrator().GetIndexes(table)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes for table %s: %w", table, err)
	}

	lines := make([]string, 0, len(indexes))
	for _, index := range indexes {
		// Keep snapshot dialect-neutral by tracking project-defined indexes only.
		// Engine-generated indexes (pkey/autoindex names) vary across dialects.
		if !strings.HasPrefix(index.Name(), "idx_") {
			continue
		}
		unique := "unknown"
		if value, ok := index.Unique(); ok {
			if value {
				unique = "true"
			} else {
				unique = "false"
			}
		}
		lines = append(lines, fmt.Sprintf(
			"%s columns=%s unique=%s option=%s",
			index.Name(),
			strings.Join(index.Columns(), ","),
			unique,
			normalizeWhitespace(index.Option()),
		))
	}
	sort.Strings(lines)
	return lines, nil
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func WriteSchemaSnapshot(db *gorm.DB, path string) error {
	snapshot, err := SchemaSnapshot(db)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(snapshot), 0o644)
}

func DriftCheck(db *gorm.DB, path string) error {
	expected, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read schema snapshot at %s: %w", path, err)
	}

	current, err := SchemaSnapshot(db)
	if err != nil {
		return err
	}

	if string(expected) != current {
		return fmt.Errorf("schema drift detected against %s", path)
	}
	return nil
}
