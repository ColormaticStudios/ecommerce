package search

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAnalyticsFailureRollsBackOnlyOptionalWrites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE commerce (id integer PRIMARY KEY)").Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("INSERT INTO commerce (id) VALUES (1)").Error)
		BestEffortAnalyticsTx(context.Background(), tx, func() error {
			require.NoError(t, tx.Exec("INSERT INTO commerce (id) VALUES (2)").Error)
			return errors.New("optional analytics write failed")
		})
		return tx.Exec("INSERT INTO commerce (id) VALUES (3)").Error
	}))
	var ids []int
	require.NoError(t, db.Table("commerce").Order("id").Pluck("id", &ids).Error)
	require.Equal(t, []int{1, 3}, ids)
}

func TestAnalyticsWritesRemainInsideCommerceTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE commerce (id integer PRIMARY KEY)").Error)
	outerFailure := errors.New("commerce failed")
	err = db.Transaction(func(tx *gorm.DB) error {
		BestEffortAnalyticsTx(context.Background(), tx, func() error { return tx.Exec("INSERT INTO commerce (id) VALUES (1)").Error })
		return outerFailure
	})
	require.ErrorIs(t, err, outerFailure)
	var count int64
	require.NoError(t, db.Table("commerce").Count(&count).Error)
	require.Zero(t, count)
}
