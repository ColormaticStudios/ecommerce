package operability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestIncidentTemplatesAreValidButExplicitlyNotEvidence(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	for _, name := range []string{"incident.yaml", "drill.yaml"} {
		file, err := os.Open(filepath.Join(repositoryRoot, "incidents", "templates", name))
		require.NoError(t, err)
		_, decodeErr := DecodeIncident(file)
		require.NoError(t, decodeErr, name)
		require.NoError(t, file.Close())
	}
	readme, err := os.ReadFile(filepath.Join(repositoryRoot, "incidents", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "Templates are examples, not proof")
}

func TestFailureDrillComposeIsParseableAndNonProductionScoped(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	contents, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "drills", "compose.example.yaml"))
	require.NoError(t, err)
	var document struct {
		Services map[string]any `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(contents, &document))
	assert.Contains(t, document.Services, "fault-proxy")
	assert.Contains(t, document.Services, "configure-postgres")
	assert.Contains(t, document.Services, "configure-provider")
	assert.Contains(t, document.Services, "control")
	assert.Contains(t, string(contents), "127.0.0.1:15432:15432")
	assert.NotContains(t, strings.ToLower(string(contents)), "production")

	readme, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "drills", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "exclusively for controlled non-production drills")
}

func TestDeploymentExamplesKeepGateCredentialsSeparate(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	contents, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "operability", "compose.example.yaml"))
	require.NoError(t, err)
	var document struct {
		Services map[string]any `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(contents, &document))
	assert.Contains(t, document.Services, "pre-deploy")
	assert.Contains(t, document.Services, "post-deploy")
	assert.NotContains(t, string(contents), "DATABASE_URL")
	assert.NotContains(t, string(contents), "AWS_")

	unit, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "systemd", "ecommerce-deploy-check@.service"))
	require.NoError(t, err)
	assert.Contains(t, string(unit), "NoNewPrivileges=true")
	assert.Contains(t, string(unit), "ProtectSystem=strict")
}
