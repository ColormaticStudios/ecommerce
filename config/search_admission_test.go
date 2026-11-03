package config

import "testing"

func TestTrustedProxyRangesAreExplicitAndValidated(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.TrustedProxyCIDRs) != 0 {
		t.Fatal("proxies must be untrusted by default")
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32, ::1/128")
	c, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.TrustedProxyCIDRs) != 2 || c.TrustedProxyCIDRs[1] != "::1/128" {
		t.Fatal("proxy ranges were not decoded and trimmed")
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-proxy")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("invalid proxy range accepted")
	}
}

func TestSearchAdmissionConfigurationEnvironmentAndValidation(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	t.Setenv("SEARCH_CLIENT_MAX_CONCURRENT", "7")
	t.Setenv("SEARCH_CLIENT_REQUESTS_PER_MINUTE", "81")
	t.Setenv("SEARCH_IMPRESSION_MAX_CONCURRENT", "3")
	t.Setenv("SEARCH_IMPRESSION_REQUESTS_PER_MINUTE", "40")
	t.Setenv("SEARCH_CLIENT_IMPRESSIONS_PER_MINUTE", "9")
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.SearchClientMaxConcurrent != 7 || c.SearchClientRequestsPerMinute != 81 || c.SearchImpressionMaxConcurrent != 3 || c.SearchImpressionRequestsPerMinute != 40 || c.SearchClientImpressionsPerMinute != 9 {
		t.Fatalf("search admission overrides did not bind")
	}
	for _, key := range []string{"SEARCH_CLIENT_MAX_CONCURRENT", "SEARCH_CLIENT_REQUESTS_PER_MINUTE", "SEARCH_IMPRESSION_MAX_CONCURRENT", "SEARCH_IMPRESSION_REQUESTS_PER_MINUTE", "SEARCH_CLIENT_IMPRESSIONS_PER_MINUTE"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "0")
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("accepted zero %s", key)
			}
		})
	}
}
