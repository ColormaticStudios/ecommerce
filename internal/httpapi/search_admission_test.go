package httpapi

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSearchClientAdmissionFairnessAndBoundedState(t *testing.T) {
	c := DefaultSearchAdmissionConfig()
	c.ClientMaxConcurrent = 1
	c.ClientRequestsPerMinute = 2
	c.ClientImpressionsPerMinute = 1
	l := newSearchClientLimiter(c)
	now := time.Now()
	l.now = func() time.Time { return now }
	release, err := l.admit("abuser", false)
	require.NoError(t, err)
	_, err = l.admit("abuser", false)
	require.Error(t, err)
	other, err := l.admit("other", false)
	require.NoError(t, err)
	other()
	release()
	release()
	release, err = l.admit("abuser", false)
	require.NoError(t, err)
	release()
	_, err = l.admit("abuser", false)
	require.Error(t, err)
	var retry interface{ RetryAfter() string }
	require.ErrorAs(t, err, &retry)
	require.Equal(t, "60", retry.RetryAfter())
	write, err := l.admit("abuser", true)
	require.NoError(t, err)
	write()
	_, err = l.admit("abuser", true)
	require.Error(t, err)
	for i := len(l.clients); i < 4096; i++ {
		l.clients[fmt.Sprintf("client-%d", i)] = &clientBudget{start: now, last: now}
	}
	release, err = l.admit("new-client", false)
	require.NoError(t, err)
	release()
	require.Len(t, l.clients, 4096)
	now = now.Add(2 * time.Minute)
	release, err = l.admit("new-client", false)
	require.NoError(t, err)
	release()
	require.Len(t, l.clients, 1)
}

func TestSearchImpressionAdmissionSeparateGlobalBudget(t *testing.T) {
	c := DefaultSearchAdmissionConfig()
	c.ImpressionMaxConcurrent = 1
	c.ImpressionRequestsPerMinute = 1
	l := newSearchClientLimiter(c)
	write, err := l.admit("one", true)
	require.NoError(t, err)
	_, err = l.admit("two", true)
	require.Error(t, err)
	read, err := l.admit("two", false)
	require.NoError(t, err)
	read()
	write()
	_, err = l.admit("two", true)
	require.Error(t, err)
}

func TestSearchClientIPEquivalence(t *testing.T) {
	require.Equal(t, searchClientIPKey("2001:db8:1:2::1"), searchClientIPKey("2001:db8:1:2:ffff::9"))
	require.NotEqual(t, searchClientIPKey("2001:db8:1:2::1"), searchClientIPKey("2001:db8:1:3::1"))
	require.Equal(t, searchClientIPKey("192.0.2.1"), searchClientIPKey("::ffff:192.0.2.1"))
	require.Equal(t, "ip:unknown", searchClientIPKey("invalid"))
}
func TestSearchClientAdmissionEvictsOldestIdleNeverActive(t *testing.T) {
	l := newSearchClientLimiter(DefaultSearchAdmissionConfig())
	now := time.Now()
	l.now = func() time.Time { return now }
	l.lastSweep = now
	active, err := l.admit("active", false)
	require.NoError(t, err)
	l.clients["oldest-idle"] = &clientBudget{start: now, last: now.Add(-time.Second)}
	for i := len(l.clients); i < 4096; i++ {
		l.clients[fmt.Sprintf("client-%d", i)] = &clientBudget{start: now, last: now}
	}
	release, err := l.admit("new", false)
	require.NoError(t, err)
	require.NotContains(t, l.clients, "oldest-idle")
	require.Contains(t, l.clients, "active")
	require.Len(t, l.clients, 4096)
	release()
	active()
	require.Zero(t, l.clients["active"].active)
}
