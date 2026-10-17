package jobs

import (
	"context"
	"testing"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConfiguredRuntimeHandleSurvivesContextAndTransactions(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, func(config *Config) { config.MaxAttempts = 17 })
	_, err := RuntimeFromDB(db)
	require.Error(t, err)
	bound := WithRuntime(db, runtime)
	for _, handle := range []*gorm.DB{bound, bound.WithContext(context.Background()), bound.Model(&models.JobQueue{}), bound.Session(&gorm.Session{})} {
		actual, err := RuntimeFromDB(handle)
		require.NoError(t, err)
		require.Same(t, runtime, actual)
	}
	require.NoError(t, bound.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		actual, err := RuntimeFromDB(tx)
		require.NoError(t, err)
		require.Same(t, runtime, actual)
		job, err := actual.EnqueueTx(context.Background(), tx, EnqueueInput{JobType: "test.configured", Payload: map[string]int{"version": 1}})
		require.NoError(t, err)
		require.Equal(t, 17, job.MaxAttempts)
		return tx.Transaction(func(nested *gorm.DB) error {
			actual, err := RuntimeFromDB(nested)
			require.NoError(t, err)
			require.Same(t, runtime, actual)
			return nil
		})
	}))
	// Repeated use does not retain previous query clauses or pollute the
	// original, unconfigured database handle.
	var count int64
	require.NoError(t, bound.Model(&models.JobQueue{}).Where("job_type = ?", "missing").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, bound.Model(&models.JobQueue{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = RuntimeFromDB(db)
	require.Error(t, err)
}
