package ops

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOpsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestAddColumnIfNotExists(t *testing.T) {
	db := newOpsTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id integer primary key)`).Error)

	tx, _ := AttachRowsCounter(db)
	require.NoError(t, AddColumnIfNotExists(tx, "users", "nickname", "TEXT"))
	require.NoError(t, AddColumnIfNotExists(tx, "users", "nickname", "TEXT"))

	require.True(t, db.Migrator().HasColumn("users", "nickname"))
}

func TestCreateIndexConcurrentlySQLiteFallback(t *testing.T) {
	db := newOpsTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id integer primary key, email TEXT)`).Error)

	tx, _ := AttachRowsCounter(db)
	require.NoError(t, CreateIndexConcurrently(tx, "idx_users_email", "users", "email"))
}

func TestBatchedBackfillByID(t *testing.T) {
	db := newOpsTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE users (id integer primary key, score integer not null default 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, score) VALUES (1,0),(2,0),(3,0),(4,0)`).Error)

	tx, _ := AttachRowsCounter(db)
	rows, err := BatchedBackfillByID(tx, "users", "id", 2, func(tx *gorm.DB, ids []int64) (int64, error) {
		return int64(len(ids)), tx.Exec(`UPDATE users SET score = 1 WHERE id IN ?`, ids).Error
	}, nil)
	require.NoError(t, err)
	require.EqualValues(t, 4, rows)

	var count int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM users WHERE score = 1`).Scan(&count).Error)
	require.EqualValues(t, 4, count)
}
