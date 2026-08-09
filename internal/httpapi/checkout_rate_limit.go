package httpapi

import (
	"strings"
	"sync"
	"time"
)

type CheckoutRateLimitConfig struct {
	Limit  int
	Window time.Duration
}

var CheckoutSubmissionRateLimit = &CheckoutRateLimitConfig{Limit: 6, Window: time.Minute}

var strictCheckoutLimiter = struct {
	sync.Mutex
	history map[string][]time.Time
}{history: make(map[string][]time.Time)}

func allowCheckoutSubmission(scope, token, client string, now time.Time) bool {
	config := CheckoutSubmissionRateLimit
	if config == nil || config.Limit <= 0 || config.Window <= 0 {
		return true
	}
	key := strings.Join([]string{scope, client, token}, "|")
	strictCheckoutLimiter.Lock()
	defer strictCheckoutLimiter.Unlock()
	cutoff := now.Add(-config.Window)
	entries := strictCheckoutLimiter.history[key][:0]
	for _, entry := range strictCheckoutLimiter.history[key] {
		if entry.After(cutoff) {
			entries = append(entries, entry)
		}
	}
	if len(entries) >= config.Limit {
		strictCheckoutLimiter.history[key] = entries
		return false
	}
	strictCheckoutLimiter.history[key] = append(entries, now)
	return true
}
