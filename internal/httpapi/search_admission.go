package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"ecommerce/internal/requestctx"
)

type SearchAdmissionConfig struct {
	ClientMaxConcurrent         int
	ClientRequestsPerMinute     int
	ImpressionMaxConcurrent     int
	ImpressionRequestsPerMinute int
	ClientImpressionsPerMinute  int
}

func DefaultSearchAdmissionConfig() SearchAdmissionConfig {
	return SearchAdmissionConfig{4, 120, 8, 600, 20}
}

// ConfigureSearchAdmission is startup-only. Trusted ingress policy owns ClientIP.
func (e *CatalogEndpoints) ConfigureSearchAdmission(c SearchAdmissionConfig) error {
	if c.ClientMaxConcurrent < 1 || c.ClientMaxConcurrent > 512 || c.ClientRequestsPerMinute < 1 || c.ClientRequestsPerMinute > 60000 || c.ImpressionMaxConcurrent < 1 || c.ImpressionMaxConcurrent > 64 || c.ImpressionRequestsPerMinute < 1 || c.ImpressionRequestsPerMinute > 60000 || c.ClientImpressionsPerMinute < 1 || c.ClientImpressionsPerMinute > 6000 {
		return errors.New("invalid search admission configuration")
	}
	e.searchClients = newSearchClientLimiter(c)
	return nil
}
func (e *CatalogEndpoints) ConfigureAnalyticsSigningKey(key []byte) error {
	return e.search.ConfigureAnalyticsSigningKey(key)
}

type clientBudget struct {
	start, last              time.Time
	requests, writes, active int
}
type searchClientLimiter struct {
	mu                    sync.Mutex
	config                SearchAdmissionConfig
	clients               map[string]*clientBudget
	writeStart, lastSweep time.Time
	writes, activeWrites  int
	now                   func() time.Time
}

func newSearchClientLimiter(c SearchAdmissionConfig) *searchClientLimiter {
	return &searchClientLimiter{config: c, clients: map[string]*clientBudget{}, now: time.Now}
}

type searchRateLimitError struct {
	error
	seconds int
}

func (e *searchRateLimitError) Unwrap() error      { return e.error }
func (e *searchRateLimitError) RetryAfter() string { return strconv.Itoa(e.seconds) }
func rateLimitError(seconds int) error {
	return &searchRateLimitError{problemError(http.StatusTooManyRequests, "search_client_rate_limited", "Search request budget exhausted; retry later.", nil), max(1, seconds)}
}

func (l *searchClientLimiter) admit(key string, write bool) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= time.Minute {
		for key, b := range l.clients {
			if b.active == 0 && now.Sub(b.last) >= 2*time.Minute {
				delete(l.clients, key)
			}
		}
		l.lastSweep = now
	}
	b := l.clients[key]
	if b == nil {
		if len(l.clients) >= 4096 {
			oldestKey := ""
			var oldest *clientBudget
			for candidateKey, candidate := range l.clients {
				if candidate.active == 0 && (oldest == nil || candidate.last.Before(oldest.last) || candidate.last.Equal(oldest.last) && candidateKey < oldestKey) {
					oldestKey, oldest = candidateKey, candidate
				}
			}
			if oldest == nil {
				return nil, rateLimitError(1)
			}
			delete(l.clients, oldestKey)
		}
		b = &clientBudget{start: now}
		l.clients[key] = b
	}
	if now.Sub(b.start) >= time.Minute {
		b.start = now
		b.requests = 0
		b.writes = 0
	}
	b.last = now
	if now.Sub(l.writeStart) >= time.Minute {
		l.writeStart = now
		l.writes = 0
	}
	retry := max(1, int((b.start.Add(time.Minute).Sub(now)+time.Second-1)/time.Second))
	if b.active >= l.config.ClientMaxConcurrent {
		return nil, rateLimitError(1)
	}
	if write {
		if l.writes >= l.config.ImpressionRequestsPerMinute {
			return nil, rateLimitError(max(1, int((l.writeStart.Add(time.Minute).Sub(now)+time.Second-1)/time.Second)))
		}
		if b.writes >= l.config.ClientImpressionsPerMinute {
			return nil, rateLimitError(retry)
		}
		if l.activeWrites >= l.config.ImpressionMaxConcurrent {
			return nil, rateLimitError(1)
		}
		b.writes++
		l.writes++
		l.activeWrites++
	} else {
		if b.requests >= l.config.ClientRequestsPerMinute {
			return nil, rateLimitError(retry)
		}
		b.requests++
	}
	b.active++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			b.active--
			if write {
				l.activeWrites--
			}
		})
	}, nil
}

func (e *CatalogEndpoints) admitSearchClient(ctx context.Context, write bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := "ip:unknown"
	if m, ok := requestctx.MetadataFrom(ctx); ok && m.ClientIP != "" {
		key = searchClientIPKey(m.ClientIP)
	}
	if p, ok := requestctx.PrincipalFrom(ctx); ok {
		if p.AccountID > 0 {
			key = "account:" + strconv.FormatUint(uint64(p.AccountID), 10)
		} else {
			key = "subject:" + p.Subject
		}
	}
	if e.searchClients == nil {
		return nil, errors.New("search admission is not configured")
	}
	return e.searchClients.admit(key, write)
}

func searchClientIPKey(value string) string {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "ip:unknown"
	}
	address = address.Unmap()
	if address.Is6() {
		return "ip:" + netip.PrefixFrom(address, 64).Masked().String()
	}
	return "ip:" + address.String()
}
