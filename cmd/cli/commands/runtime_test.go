package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func TestPrepareRuntimeForCommandUsesCurrentWorkingDirectoryWhenItLooksLocal(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(cliDataDirEnv, filepath.Join(tempDir, "cli-data"))

	writeTestFile(t, filepath.Join(tempDir, "go.mod"), "module ecommerce\n")
	writeTestFile(t, filepath.Join(tempDir, "main.go"), "package main\n")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore wd: %v", chdirErr)
		}
	})

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	cmd := newTestLeafCommand()
	if err := prepareRuntimeForCommand(cmd); err != nil {
		t.Fatalf("prepare runtime: %v", err)
	}

	if activeCLIRuntime.LocalPath != tempDir {
		t.Fatalf("expected local path %s, got %s", tempDir, activeCLIRuntime.LocalPath)
	}
	if activeCLIRuntime.Remote != nil {
		t.Fatalf("expected remote auth to be nil")
	}
}

func TestPrepareRuntimeForCommandUsesSavedRemoteAuth(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(cliDataDirEnv, tempDir)

	if err := writePersistentCLIConfig(persistentCLIConfig{
		Version: 1,
		Mode:    cliTargetModeAuth,
		APIURL:  "https://api.example.test",
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := writePersistentCLIAuth(persistentCLIAuth{
		Version: 1,
		APIURL:  "https://api.example.test",
		Token:   "token-123",
	}); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	cmd := newTestLeafCommand()
	if err := prepareRuntimeForCommand(cmd); err != nil {
		t.Fatalf("prepare runtime: %v", err)
	}

	if activeCLIRuntime.Remote == nil {
		t.Fatalf("expected remote auth to be loaded")
	}
	if activeCLIRuntime.Remote.Token != "token-123" {
		t.Fatalf("expected token-123, got %s", activeCLIRuntime.Remote.Token)
	}
}

func TestPrepareRuntimeForCommandFailsWithoutConfigOrLocalMarkers(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(cliDataDirEnv, filepath.Join(tempDir, "cli-data"))

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore wd: %v", chdirErr)
		}
	})

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	cmd := newTestLeafCommand()
	if err := prepareRuntimeForCommand(cmd); err == nil {
		t.Fatalf("expected prepare runtime to fail")
	}
}

func TestParsePastedTokenFromJSON(t *testing.T) {
	token, err := parsePastedToken(`{"token":"abc123"}`)
	if err != nil {
		t.Fatalf("parse pasted token: %v", err)
	}
	if token != "abc123" {
		t.Fatalf("expected abc123, got %s", token)
	}
}

func TestWritePersistentCLIConfigUsesTOML(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(cliDataDirEnv, tempDir)

	if err := writePersistentCLIConfig(persistentCLIConfig{
		Version: 1,
		Mode:    cliTargetModePath,
		Path:    "/srv/ecommerce",
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	configPath, err := cliConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if filepath.Ext(configPath) != ".toml" {
		t.Fatalf("expected .toml config path, got %s", configPath)
	}

	payload, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var decoded persistentCLIConfig
	if err := toml.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode config TOML: %v", err)
	}
	if decoded.Mode != cliTargetModePath || decoded.Path != "/srv/ecommerce" {
		t.Fatalf("unexpected decoded config: %+v", decoded)
	}
}

func newTestLeafCommand() *cobra.Command {
	root := &cobra.Command{Use: "ecommerce-cli"}
	child := &cobra.Command{Use: "brand"}
	root.AddCommand(child)
	return child
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
