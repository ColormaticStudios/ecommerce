package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ecommerce/internal/operability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployCheckCommandEmitsSuccessfulReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set(operability.ReleaseHeader, "release-test")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("DEPLOY_HEALTH_URL", server.URL+"/healthz")
	t.Setenv("DEPLOY_READINESS_URL", server.URL+"/readyz")
	t.Setenv("DEPLOY_EXPECTED_RELEASE_ID", "release-test")
	t.Setenv("DEPLOY_GATE_ATTEMPTS", "1")

	output := captureOutput(t, func() error {
		return run(context.Background(), []string{"deploy-check", "post-deploy"})
	})
	var report operability.GateReport
	require.NoError(t, json.Unmarshal(output, &report))
	assert.Equal(t, "succeeded", report.Outcome)
}

func TestIncidentCommandValidatesTemplate(t *testing.T) {
	template := filepath.Join("..", "..", "incidents", "templates", "drill.yaml")
	output := captureOutput(t, func() error {
		return run(context.Background(), []string{"incident", "validate", template})
	})
	var result map[string]any
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, "succeeded", result["outcome"])
}

func captureOutput(t *testing.T, operation func() error) []byte {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = original })

	require.NoError(t, operation())
	require.NoError(t, writer.Close())
	os.Stdout = original
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return output
}
