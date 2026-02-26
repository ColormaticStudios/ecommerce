package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	toml := `PORT = "1111"
DEV_MODE = true
PUBLIC_URL = "toml.example.com"
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	dotenv := `PORT="2222"
PUBLIC_URL="envfile.example.com"
DISABLE_LOCAL_SIGN_IN="true"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(dotenv), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("PORT", "3333")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "3333" {
		t.Fatalf("expected PORT from environment variable, got %q", cfg.Port)
	}
	if cfg.PublicURL != "envfile.example.com" {
		t.Fatalf("expected PUBLIC_URL from .env, got %q", cfg.PublicURL)
	}
	if !cfg.DevMode {
		t.Fatalf("expected DEV_MODE from config.toml fallback to be true")
	}
	if !cfg.DisableLocalSignIn {
		t.Fatalf("expected DISABLE_LOCAL_SIGN_IN from .env to parse as true")
	}
}

func TestLoadConfigAllowsMissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("PORT", "8080")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig with env-only setup: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected PORT from env-only setup, got %q", cfg.Port)
	}
}
