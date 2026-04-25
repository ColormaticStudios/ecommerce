package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthorizedOrderReservationExpiresAfterCheckoutSnapshotTTL(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	expiresAt := authorizedOrderReservationExpiresAt(now)

	assert.Equal(t, now.Add(authorizedOrderReservationTTL), expiresAt)
	assert.Greater(t, expiresAt.Sub(now), 15*time.Minute)
}
