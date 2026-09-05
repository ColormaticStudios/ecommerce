package backups

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDockerDeploymentIsParseableAndRunsAsNonRoot(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	composeBytes, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "backup", "compose.example.yaml"))
	require.NoError(t, err)
	var compose struct {
		Services map[string]struct {
			Command     string            `yaml:"command"`
			ReadOnly    bool              `yaml:"read_only"`
			EnvFiles    []string          `yaml:"env_file"`
			Environment map[string]string `yaml:"environment"`
			Secrets     []string          `yaml:"secrets"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(composeBytes, &compose))
	for name, command := range map[string]string{
		"backup-scheduler": "schedule", "backup-metrics": "serve-metrics", "restore-drill": "restore-drill",
	} {
		service, exists := compose.Services[name]
		require.True(t, exists, name)
		assert.Equal(t, command, service.Command, name)
		assert.True(t, service.ReadOnly, name)
	}
	assert.Equal(t, []string{"common.env"}, compose.Services["backup-metrics"].EnvFiles)
	assert.Equal(t, []string{"common.env", "backup.env"}, compose.Services["backup-scheduler"].EnvFiles)
	assert.Equal(t, []string{"common.env", "restore-drill.env"}, compose.Services["restore-drill"].EnvFiles)
	assert.Equal(t, []string{"backup-aws-credentials"}, compose.Services["backup-scheduler"].Secrets)
	assert.Equal(t, []string{"metrics-aws-credentials"}, compose.Services["backup-metrics"].Secrets)
	assert.Equal(t, []string{"restore-aws-credentials"}, compose.Services["restore-drill"].Secrets)
	assert.Equal(t, "/run/secrets/backup-aws-credentials", compose.Services["backup-scheduler"].Environment["AWS_SHARED_CREDENTIALS_FILE"])
	assert.Equal(t, "/run/secrets/metrics-aws-credentials", compose.Services["backup-metrics"].Environment["AWS_SHARED_CREDENTIALS_FILE"])
	assert.Equal(t, "/run/secrets/restore-aws-credentials", compose.Services["restore-drill"].Environment["AWS_SHARED_CREDENTIALS_FILE"])
	commonEnvironment, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "backup", "common.env.example"))
	require.NoError(t, err)
	assert.NotContains(t, string(commonEnvironment), "AWS_ACCESS_KEY_ID")
	for _, name := range []string{"backup-aws-credentials.example", "metrics-aws-credentials.example", "restore-aws-credentials.example"} {
		_, err := os.Stat(filepath.Join(repositoryRoot, "deploy", "backup", "secrets", name))
		require.NoError(t, err, name)
	}

	dockerfile, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "backup", "Dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "postgresql-client")
	assert.Contains(t, string(dockerfile), "USER ecommerce:ecommerce")
}

func TestSystemdDeploymentSeparatesCredentialsAndHardensServices(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	for _, name := range []string{"ecommerce-backup.service", "ecommerce-backup-metrics.service", "ecommerce-restore-drill@.service"} {
		contents, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "systemd", name))
		require.NoError(t, err)
		unit := string(contents)
		assert.Contains(t, unit, "User=ecommerce-backup", name)
		assert.Contains(t, unit, "NoNewPrivileges=true", name)
		assert.Contains(t, unit, "ProtectSystem=strict", name)
	}

	metrics, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "systemd", "ecommerce-backup-metrics.service"))
	require.NoError(t, err)
	assert.NotContains(t, string(metrics), "backup.env")
	assert.NotContains(t, string(metrics), "restore-drill")
	assert.Contains(t, string(metrics), "backup-metrics-credentials.env")

	restore, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "systemd", "ecommerce-restore-drill@.service"))
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(restore), "restore-drill-%i.env"))
	assert.Contains(t, string(restore), "restore-credentials.env")
}
