package ops

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"gorm.io/gorm"
)

const rowsCounterKey = "migration_rows_touched_counter"

var identPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func AttachRowsCounter(db *gorm.DB) (*gorm.DB, *int64) {
	counter := new(int64)
	return db.Set(rowsCounterKey, counter), counter
}

func ReadRowsCounter(counter *int64) int64 {
	if counter == nil {
		return 0
	}
	return atomic.LoadInt64(counter)
}

func AddRowsTouched(db *gorm.DB, rows int64) {
	if rows <= 0 {
		return
	}
	value, ok := db.Get(rowsCounterKey)
	if !ok {
		return
	}
	counter, ok := value.(*int64)
	if !ok {
		return
	}
	atomic.AddInt64(counter, rows)
}

func AddColumnIfNotExists(tx *gorm.DB, table, column, definition string) error {
	if err := validateIdentifier(table); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	if err := validateIdentifier(column); err != nil {
		return fmt.Errorf("invalid column name: %w", err)
	}
	if strings.TrimSpace(definition) == "" {
		return fmt.Errorf("column definition cannot be empty")
	}
	if tx.Migrator().HasColumn(table, column) {
		return nil
	}

	statement := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, definition)
	if err := tx.Exec(statement).Error; err != nil {
		return err
	}
	AddRowsTouched(tx, 1)
	return nil
}

func CreateIndexConcurrently(tx *gorm.DB, indexName, table, columns string) error {
	if err := validateIdentifier(indexName); err != nil {
		return fmt.Errorf("invalid index name: %w", err)
	}
	if err := validateIdentifier(table); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	if strings.TrimSpace(columns) == "" {
		return fmt.Errorf("columns cannot be empty")
	}

	statement := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "%s" ON "%s" (%s)`, indexName, table, columns)
	if tx.Dialector.Name() == "postgres" {
		statement = fmt.Sprintf(`CREATE INDEX CONCURRENTLY IF NOT EXISTS "%s" ON "%s" (%s)`, indexName, table, columns)
	}
	if err := tx.Exec(statement).Error; err != nil {
		return err
	}
	AddRowsTouched(tx, 1)
	return nil
}

func BatchedBackfillByID(
	tx *gorm.DB,
	table, idColumn string,
	batchSize int,
	apply func(tx *gorm.DB, ids []int64) (int64, error),
	logf func(format string, args ...any),
) (int64, error) {
	if err := validateIdentifier(table); err != nil {
		return 0, fmt.Errorf("invalid table name: %w", err)
	}
	if err := validateIdentifier(idColumn); err != nil {
		return 0, fmt.Errorf("invalid id column name: %w", err)
	}
	if batchSize <= 0 {
		return 0, fmt.Errorf("batch size must be > 0")
	}
	if apply == nil {
		return 0, fmt.Errorf("apply function is required")
	}

	query := fmt.Sprintf(
		`SELECT "%s" FROM "%s" WHERE "%s" > ? ORDER BY "%s" ASC LIMIT ?`,
		idColumn,
		table,
		idColumn,
		idColumn,
	)
	lastID := int64(0)
	totalRows := int64(0)
	batch := 0

	for {
		var ids []int64
		if err := tx.Raw(query, lastID, batchSize).Scan(&ids).Error; err != nil {
			return totalRows, err
		}
		if len(ids) == 0 {
			break
		}

		updated, err := apply(tx, ids)
		if err != nil {
			return totalRows, err
		}
		totalRows += updated
		batch++
		lastID = ids[len(ids)-1]

		if logf != nil {
			logf("migration_backfill_batch table=%s batch=%d ids=%d updated_rows=%d", table, batch, len(ids), updated)
		}
	}

	AddRowsTouched(tx, totalRows)
	return totalRows, nil
}

func validateIdentifier(identifier string) error {
	if !identPattern.MatchString(identifier) {
		return fmt.Errorf("identifier %q does not match %s", identifier, identPattern.String())
	}
	return nil
}
