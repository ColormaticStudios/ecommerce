package migrations

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
	if db.Dialector.Name() == "postgres" {
		return postgresTableIndexes(db, table)
	}
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
			canonicalSnapshotIndexName(table, index.Name(), index.Columns()),
			strings.Join(index.Columns(), ","),
			unique,
			normalizeWhitespace(index.Option()),
		))
	}
	sort.Strings(lines)
	return lines, nil
}

func canonicalSnapshotIndexName(table, name string, columns []string) string {
	// SQLite allows GORM's default 64-character generated identifiers, whereas
	// PostgreSQL's naming strategy uses 63. Compare generated names at the
	// stricter limit without rewriting explicitly named project indexes.
	if len(columns) == 1 && (schema.NamingStrategy{}).IndexName(table, columns[0]) == name {
		return (schema.NamingStrategy{IdentifierMaxLength: 63}).IndexName(table, columns[0])
	}
	return name
}

// GORM's PostgreSQL index inspector orders attributes by their name, which
// loses the column order that determines a composite index's access paths.
func postgresTableIndexes(db *gorm.DB, table string) ([]string, error) {
	var rows []struct {
		Name, Columns string
		IsUnique      bool
	}
	err := db.Raw(`SELECT idx.relname AS name, i.indisunique AS is_unique,
 string_agg(COALESCE(a.attname, pg_get_indexdef(i.indexrelid, k.ordinality::int, true)), ',' ORDER BY k.ordinality) AS columns
 FROM pg_class tbl JOIN pg_namespace n ON n.oid=tbl.relnamespace
 JOIN pg_index i ON i.indrelid=tbl.oid JOIN pg_class idx ON idx.oid=i.indexrelid
 CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, ordinality)
 LEFT JOIN pg_attribute a ON a.attrelid=tbl.oid AND a.attnum=k.attnum
 WHERE n.nspname=current_schema() AND tbl.relname=? AND left(idx.relname,4)='idx_'
 GROUP BY idx.relname,i.indisunique ORDER BY idx.relname`, table).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes for table %s: %w", table, err)
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s columns=%s unique=%t option=", row.Name, row.Columns, row.IsUnique))
	}
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

func WriteCurrentSchemaSnapshot(db *gorm.DB, path string) error {
	if err := Check(db); err != nil {
		return fmt.Errorf("database must be at latest migration before writing schema snapshot: %w", err)
	}
	return WriteSchemaSnapshot(db, path)
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
