package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCacheableSnapshotExchangeKeepsAuthoritativeResultsWithoutSearch(t *testing.T) {
	service, _ := analyticsFixture(t)
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	filters := Filters{Query: "boots", Page: 1, Limit: 10}
	result, err := service.Search(context.Background(), filters)
	require.NoError(t, err)
	snapshot, err := service.SignCacheableSnapshot(result, filters, time.Millisecond)
	require.NoError(t, err)
	_, err = service.verifyResultSnapshot(snapshot)
	require.ErrorIs(t, err, ErrAnalyticsInvalid, "cacheable snapshots cannot be collected directly")
	now = now.Add(31 * time.Minute)
	reads := 0
	require.NoError(t, service.db.Callback().Query().Before("gorm:query").Register("snapshot_exchange_reads", func(*gorm.DB) { reads++ }))
	eventID := uuid.NewString()
	proof, err := service.BrowserResultProof(snapshot, eventID)
	require.NoError(t, err)
	require.Zero(t, reads, "exchange must not query or rerun search")
	decoded, err := service.verifyResultSnapshot(proof)
	require.NoError(t, err)
	require.NotEqual(t, eventID, decoded.SnapshotID)
	require.NotEmpty(t, decoded.SnapshotID)
	retry, err := service.BrowserResultProof(snapshot, eventID)
	require.NoError(t, err)
	retryDecoded, err := service.verifyResultSnapshot(retry)
	require.NoError(t, err)
	require.Equal(t, decoded.SnapshotID, retryDecoded.SnapshotID)
	otherSnapshot, err := service.SignCacheableSnapshot(result, filters, time.Millisecond)
	require.NoError(t, err)
	changed, err := service.BrowserResultProof(otherSnapshot, eventID)
	require.NoError(t, err)
	changedDecoded, err := service.verifyResultSnapshot(changed)
	require.NoError(t, err)
	require.NotEqual(t, decoded.SnapshotID, changedDecoded.SnapshotID)
	require.Equal(t, result.Products[0].ID, decoded.Products[0])
	require.Equal(t, now, decoded.CreatedAt)
	_, err = service.BrowserResultProof(proof, eventID)
	require.ErrorIs(t, err, ErrAnalyticsInvalid, "proof cannot extend its own expiry")
	for _, invalid := range []string{snapshot + "x", strings.Repeat("x", MaxResultTokenLength+1), ""} {
		_, err = service.BrowserResultProof(invalid, eventID)
		require.ErrorIs(t, err, ErrAnalyticsInvalid)
	}
	_, err = service.BrowserResultProof(snapshot, uuid.Nil.String())
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	now = now.Add(ResultProofLifetime)
	_, err = service.verifyResultSnapshot(proof)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	now = now.Add(24 * time.Minute)
	_, err = service.BrowserResultProof(snapshot, eventID)
	require.ErrorIs(t, err, ErrAnalyticsInvalid, "one-hour snapshots cannot be renewed")
}
