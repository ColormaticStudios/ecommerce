package operability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentGatePassesHealthReadinessCanaryAndRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set(ReleaseHeader, "release-42")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	config := GateConfig{
		Phase: "post-deploy", ExpectedReleaseID: "release-42", Attempts: 1,
		Interval: time.Millisecond, RequestTimeout: time.Second,
		Probes: []Probe{
			{Name: "health", URL: server.URL + "/healthz", ExpectedStatus: 200, VerifyRelease: true},
			{Name: "readiness", URL: server.URL + "/readyz", ExpectedStatus: 200, VerifyRelease: true},
			{Name: "catalog", URL: server.URL + "/api/v1/products", ExpectedStatus: 200},
		},
	}
	report, err := (GateRunner{}).Run(context.Background(), config)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", report.Outcome)
	assert.Equal(t, 1, report.Attempts)
	assert.Len(t, report.Results, 3)
}

func TestDeploymentGateRetriesAndFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/healthz" {
			writer.Header().Set(ReleaseHeader, "old-release")
			writer.WriteHeader(http.StatusOK)
			return
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	config := GateConfig{
		Phase: "post-deploy", ExpectedReleaseID: "new-release", Attempts: 2,
		Interval: time.Millisecond, RequestTimeout: time.Second,
		Probes: []Probe{
			{Name: "health", URL: server.URL + "/healthz", ExpectedStatus: 200, VerifyRelease: true},
			{Name: "readiness", URL: server.URL + "/readyz", ExpectedStatus: 200, VerifyRelease: true},
		},
	}
	report, err := (GateRunner{}).Run(context.Background(), config)
	require.Error(t, err)
	assert.Equal(t, "failed", report.Outcome)
	assert.Equal(t, 2, report.Attempts)
	assert.Equal(t, "release_mismatch", report.Results[0].FailureKind)
	assert.Equal(t, "unexpected_status", report.Results[1].FailureKind)
	encoded, encodeErr := json.Marshal(report)
	require.NoError(t, encodeErr)
	assert.NotContains(t, string(encoded), server.URL)
}

func TestGateConfigRejectsUnsafeAndUnboundedInput(t *testing.T) {
	config := GateConfig{
		Phase: "pre-deploy", Attempts: 1, Interval: time.Second, RequestTimeout: time.Second,
		Probes: []Probe{
			{Name: "health", URL: "https://user:secret@example.com/healthz", ExpectedStatus: 200},
			{Name: "readiness", URL: "https://example.com/readyz", ExpectedStatus: 200},
		},
	}
	require.ErrorContains(t, config.Validate(), "without user information")
	config.Probes[0].URL = "https://example.com/healthz"
	config.Probes = append(config.Probes, Probe{Name: "health", URL: "https://example.com/canary", ExpectedStatus: 200})
	require.ErrorContains(t, config.Validate(), "duplicate probe")
}

func TestLoadGateConfigBuildsReadOnlyCanaries(t *testing.T) {
	t.Setenv("DEPLOY_HEALTH_URL", "https://telemetry.example/healthz")
	t.Setenv("DEPLOY_READINESS_URL", "https://telemetry.example/readyz")
	t.Setenv("DEPLOY_EXPECTED_RELEASE_ID", "release-42")
	t.Setenv("DEPLOY_GATE_ATTEMPTS", "3")
	t.Setenv("DEPLOY_GATE_INTERVAL", "250ms")
	t.Setenv("DEPLOY_GATE_REQUEST_TIMEOUT", "2s")
	t.Setenv("DEPLOY_CANARIES_JSON", `[{"name":"catalog","url":"https://shop.example/api/v1/products","expected_status":200,"verify_release":true}]`)

	config, err := LoadGateConfig("post-deploy")
	require.NoError(t, err)
	assert.Equal(t, 3, config.Attempts)
	assert.Equal(t, 250*time.Millisecond, config.Interval)
	require.Len(t, config.Probes, 3)
	assert.True(t, config.Probes[0].VerifyRelease)
	assert.False(t, config.Probes[2].VerifyRelease, "public canaries must not be trusted to prove candidate identity")
}
