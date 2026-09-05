// Package operability owns provider-neutral deployment gates and incident evidence.
package operability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const ReleaseHeader = "X-Ecommerce-Release-ID"

type Probe struct {
	Name           string `json:"name" yaml:"name"`
	URL            string `json:"url" yaml:"url"`
	ExpectedStatus int    `json:"expected_status" yaml:"expected_status"`
	VerifyRelease  bool   `json:"verify_release,omitempty" yaml:"verify_release,omitempty"`
}

type GateConfig struct {
	Phase             string
	ExpectedReleaseID string
	Attempts          int
	Interval          time.Duration
	RequestTimeout    time.Duration
	Probes            []Probe
}

type ProbeResult struct {
	Name          string  `json:"name"`
	Outcome       string  `json:"outcome"`
	FailureKind   string  `json:"failure_kind,omitempty"`
	StatusCode    int     `json:"status_code,omitempty"`
	LatencyMillis float64 `json:"latency_ms"`
}

type GateReport struct {
	SchemaVersion int           `json:"schema_version"`
	Phase         string        `json:"phase"`
	ReleaseID     string        `json:"expected_release_id,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Outcome       string        `json:"outcome"`
	Attempts      int           `json:"attempts"`
	Results       []ProbeResult `json:"results"`
}

type GateFailure struct{ Report GateReport }

func (e GateFailure) Error() string { return "deployment gate failed" }

func LoadGateConfig(phase string) (GateConfig, error) {
	phase = strings.TrimSpace(phase)
	if phase != "pre-deploy" && phase != "post-deploy" {
		return GateConfig{}, errors.New("deploy-check phase must be pre-deploy or post-deploy")
	}
	config := GateConfig{
		Phase: phase, ExpectedReleaseID: strings.TrimSpace(os.Getenv("DEPLOY_EXPECTED_RELEASE_ID")),
		Interval: 5 * time.Second, RequestTimeout: 5 * time.Second,
	}
	if phase == "pre-deploy" {
		config.Attempts = 1
	} else {
		config.Attempts = 12
		if config.ExpectedReleaseID == "" {
			return GateConfig{}, errors.New("DEPLOY_EXPECTED_RELEASE_ID is required for post-deploy checks")
		}
	}
	if value := strings.TrimSpace(os.Getenv("DEPLOY_GATE_ATTEMPTS")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return GateConfig{}, errors.New("DEPLOY_GATE_ATTEMPTS must be an integer")
		}
		config.Attempts = parsed
	}
	for key, target := range map[string]*time.Duration{
		"DEPLOY_GATE_INTERVAL": &config.Interval, "DEPLOY_GATE_REQUEST_TIMEOUT": &config.RequestTimeout,
	} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return GateConfig{}, fmt.Errorf("%s must be a duration: %w", key, err)
			}
			*target = parsed
		}
	}
	healthURL := strings.TrimSpace(os.Getenv("DEPLOY_HEALTH_URL"))
	readinessURL := strings.TrimSpace(os.Getenv("DEPLOY_READINESS_URL"))
	config.Probes = []Probe{
		{Name: "health", URL: healthURL, ExpectedStatus: http.StatusOK, VerifyRelease: config.ExpectedReleaseID != ""},
		{Name: "readiness", URL: readinessURL, ExpectedStatus: http.StatusOK, VerifyRelease: config.ExpectedReleaseID != ""},
	}
	if raw := strings.TrimSpace(os.Getenv("DEPLOY_CANARIES_JSON")); raw != "" {
		var canaries []Probe
		if err := json.Unmarshal([]byte(raw), &canaries); err != nil {
			return GateConfig{}, fmt.Errorf("decode DEPLOY_CANARIES_JSON: %w", err)
		}
		for index := range canaries {
			canaries[index].VerifyRelease = false
		}
		config.Probes = append(config.Probes, canaries...)
	}
	return config, config.Validate()
}

func (c GateConfig) Validate() error {
	if c.Phase != "pre-deploy" && c.Phase != "post-deploy" {
		return errors.New("gate phase must be pre-deploy or post-deploy")
	}
	if c.Attempts < 1 || c.Attempts > 120 {
		return errors.New("gate attempts must be between 1 and 120")
	}
	if c.Interval <= 0 || c.Interval > time.Minute || c.RequestTimeout <= 0 || c.RequestTimeout > time.Minute {
		return errors.New("gate interval and request timeout must be positive and no greater than one minute")
	}
	if len(c.Probes) < 2 || len(c.Probes) > 22 {
		return errors.New("deployment gate requires health/readiness and at most twenty canaries")
	}
	names := make(map[string]struct{}, len(c.Probes))
	for _, probe := range c.Probes {
		if !validBoundedName(probe.Name) {
			return fmt.Errorf("invalid probe name %q", probe.Name)
		}
		if _, duplicate := names[probe.Name]; duplicate {
			return fmt.Errorf("duplicate probe name %q", probe.Name)
		}
		names[probe.Name] = struct{}{}
		parsed, err := url.Parse(probe.URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("probe %q must use an absolute HTTP(S) URL without user information or a fragment", probe.Name)
		}
		if probe.ExpectedStatus < 100 || probe.ExpectedStatus > 599 {
			return fmt.Errorf("probe %q expected status must be between 100 and 599", probe.Name)
		}
	}
	if len(c.ExpectedReleaseID) > 128 || strings.ContainsAny(c.ExpectedReleaseID, "\r\n") {
		return errors.New("expected release ID must be no longer than 128 characters and contain no line breaks")
	}
	return nil
}

type GateRunner struct {
	Client *http.Client
	Now    func() time.Time
	Sleep  func(context.Context, time.Duration) error
}

func (r GateRunner) Run(ctx context.Context, config GateConfig) (GateReport, error) {
	if err := config.Validate(); err != nil {
		return GateReport{}, err
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	if r.Sleep == nil {
		r.Sleep = sleepContext
	}
	if r.Client == nil {
		r.Client = &http.Client{
			Timeout:       config.RequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	report := GateReport{SchemaVersion: 1, Phase: config.Phase, ReleaseID: config.ExpectedReleaseID, StartedAt: r.Now().UTC()}
	for attempt := 1; attempt <= config.Attempts; attempt++ {
		report.Attempts = attempt
		report.Results = r.runAttempt(ctx, config)
		if probesSucceeded(report.Results) {
			report.Outcome = "succeeded"
			report.CompletedAt = r.Now().UTC()
			return report, nil
		}
		if attempt < config.Attempts {
			if err := r.Sleep(ctx, config.Interval); err != nil {
				report.Outcome = "failed"
				report.CompletedAt = r.Now().UTC()
				return report, errors.Join(GateFailure{Report: report}, err)
			}
		}
	}
	report.Outcome = "failed"
	report.CompletedAt = r.Now().UTC()
	return report, GateFailure{Report: report}
}

func (r GateRunner) runAttempt(ctx context.Context, config GateConfig) []ProbeResult {
	results := make([]ProbeResult, 0, len(config.Probes))
	for _, probe := range config.Probes {
		started := r.Now()
		result := ProbeResult{Name: probe.Name, Outcome: "failed"}
		requestContext, cancel := context.WithTimeout(ctx, config.RequestTimeout)
		request, err := http.NewRequestWithContext(requestContext, http.MethodGet, probe.URL, nil)
		if err == nil {
			request.Header.Set("User-Agent", "ecommerce-ops/deploy-check")
			var response *http.Response
			response, err = r.Client.Do(request)
			if err == nil {
				result.StatusCode = response.StatusCode
				_, _ = io.CopyN(io.Discard, response.Body, 4096)
				_ = response.Body.Close()
				switch {
				case response.StatusCode != probe.ExpectedStatus:
					result.FailureKind = "unexpected_status"
				case probe.VerifyRelease && response.Header.Get(ReleaseHeader) != config.ExpectedReleaseID:
					result.FailureKind = "release_mismatch"
				default:
					result.Outcome = "succeeded"
				}
			}
		}
		cancel()
		if err != nil {
			result.FailureKind = "request_failed"
		}
		result.LatencyMillis = float64(r.Now().Sub(started).Microseconds()) / 1000
		results = append(results, result)
	}
	return results
}

func probesSucceeded(results []ProbeResult) bool {
	for _, result := range results {
		if result.Outcome != "succeeded" {
			return false
		}
	}
	return true
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validBoundedName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}
