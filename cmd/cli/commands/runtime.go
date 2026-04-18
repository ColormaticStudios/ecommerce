package commands

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const cliDataDirEnv = "ECOMMERCE_CLI_DATA_DIR"

type cliTargetMode string

const (
	cliTargetModePath cliTargetMode = "path"
	cliTargetModeAuth cliTargetMode = "auth"
)

type persistentCLIConfig struct {
	Version int           `toml:"version"`
	Mode    cliTargetMode `toml:"mode"`
	Path    string        `toml:"path,omitempty"`
	APIURL  string        `toml:"api_url,omitempty"`
}

type persistentCLIAuth struct {
	Version    int    `toml:"version"`
	APIURL     string `toml:"api_url"`
	Token      string `toml:"token"`
	AuthMethod string `toml:"auth_method,omitempty"`
	UserEmail  string `toml:"user_email,omitempty"`
	UserName   string `toml:"user_name,omitempty"`
	UserRole   string `toml:"user_role,omitempty"`
}

type cliRuntime struct {
	LocalPath string
	Remote    *persistentCLIAuth
}

var activeCLIRuntime cliRuntime

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ecommerce-cli",
		Short: "Ecommerce API CLI tool for administrative tasks",
		Long:  "A command-line tool for managing users, products, and other administrative tasks for the ecommerce API.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareRuntimeForCommand(cmd)
		},
	}

	rootCmd.AddCommand(NewUserCmd())
	rootCmd.AddCommand(NewProductCmd())
	rootCmd.AddCommand(NewBrandCmd())
	rootCmd.AddCommand(NewProductAttributeCmd())
	rootCmd.AddCommand(NewOrderCmd())
	rootCmd.AddCommand(NewStorefrontCmd())
	rootCmd.AddCommand(NewMigrateCmd())
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newConfigCmd())

	return rootCmd
}

func resetRuntimeState() {
	activeCLIRuntime = cliRuntime{}
}

func prepareRuntimeForCommand(cmd *cobra.Command) error {
	resetRuntimeState()

	if shouldSkipTargetResolution(cmd) {
		return nil
	}

	cfg, found, err := loadPersistentCLIConfig()
	if err != nil {
		return err
	}

	if found {
		switch cfg.Mode {
		case cliTargetModePath:
			targetPath := strings.TrimSpace(cfg.Path)
			if targetPath == "" {
				return fmt.Errorf("CLI config is in path mode but no path is configured; run `ecommerce-cli setup`")
			}
			absolutePath, err := filepath.Abs(targetPath)
			if err != nil {
				return fmt.Errorf("resolve configured path: %w", err)
			}
			if err := os.Chdir(absolutePath); err != nil {
				return fmt.Errorf("switch to configured path %s: %w", absolutePath, err)
			}
			activeCLIRuntime.LocalPath = absolutePath
			return nil
		case cliTargetModeAuth:
			auth, authFound, err := loadPersistentCLIAuth()
			if err != nil {
				return err
			}
			if !authFound {
				return fmt.Errorf("CLI config points at a remote API, but no auth file was found; run `ecommerce-cli setup` or `ecommerce-cli config login`")
			}
			if strings.TrimSpace(cfg.APIURL) != "" && strings.TrimSpace(auth.APIURL) != "" && !sameAPIBaseURL(cfg.APIURL, auth.APIURL) {
				return fmt.Errorf("CLI config points at %s, but the auth file is for %s; run `ecommerce-cli config login`", cfg.APIURL, auth.APIURL)
			}
			if strings.TrimSpace(auth.Token) == "" {
				return fmt.Errorf("CLI auth file is missing a token; run `ecommerce-cli config login`")
			}
			activeCLIRuntime.Remote = &auth
			return nil
		default:
			return fmt.Errorf("unsupported CLI config mode %q", cfg.Mode)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current working directory: %w", err)
	}
	if looksLikeLocalServerDir(cwd) {
		activeCLIRuntime.LocalPath = cwd
		return nil
	}

	return fmt.Errorf("no CLI config was found and %s does not look like an ecommerce API server directory; run `ecommerce-cli setup`", cwd)
}

func shouldSkipTargetResolution(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd == cmd.Root() {
		return true
	}

	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "setup", "config", "help", "completion":
			return true
		}
	}

	return false
}

func isRemoteMode() bool {
	return activeCLIRuntime.Remote != nil
}

func currentRemoteAuth() (persistentCLIAuth, error) {
	if activeCLIRuntime.Remote == nil {
		return persistentCLIAuth{}, errors.New("CLI is not configured for remote auth mode")
	}
	return *activeCLIRuntime.Remote, nil
}

func requireLocalMode(feature string) error {
	if !isRemoteMode() {
		return nil
	}
	return fmt.Errorf("%s is only available when the CLI is pointed at a local ecommerce API path; run `ecommerce-cli config use-path` or `ecommerce-cli config clear`", feature)
}

func normalizeAPIBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("API URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse API URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("API URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("API URL must include a host")
	}

	normalized := strings.TrimRight(parsed.String(), "/")
	return normalized, nil
}

func sameAPIBaseURL(left string, right string) bool {
	leftURL, leftErr := normalizeAPIBaseURL(left)
	rightURL, rightErr := normalizeAPIBaseURL(right)
	if leftErr != nil || rightErr != nil {
		return strings.TrimSpace(left) == strings.TrimSpace(right)
	}
	return leftURL == rightURL
}

func cliDataDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(cliDataDirEnv)); override != "" {
		return override, nil
	}

	switch runtime.GOOS {
	case "linux":
		if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "ecommerce-cli"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "ecommerce-cli"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "ecommerce-cli"), nil
	case "windows":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		return filepath.Join(configDir, "ecommerce-cli"), nil
	default:
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		return filepath.Join(configDir, "ecommerce-cli"), nil
	}
}

func cliConfigPath() (string, error) {
	base, err := cliDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.toml"), nil
}

func cliAuthPath() (string, error) {
	base, err := cliDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "auth.toml"), nil
}

func ensureCLIDataDir() (string, error) {
	base, err := cliDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create CLI data directory %s: %w", base, err)
	}
	return base, nil
}

func loadPersistentCLIConfig() (persistentCLIConfig, bool, error) {
	path, err := cliConfigPath()
	if err != nil {
		return persistentCLIConfig{}, false, err
	}

	var cfg persistentCLIConfig
	found, err := readTOMLFile(path, &cfg)
	if err != nil {
		return persistentCLIConfig{}, false, err
	}
	return cfg, found, nil
}

func loadPersistentCLIAuth() (persistentCLIAuth, bool, error) {
	path, err := cliAuthPath()
	if err != nil {
		return persistentCLIAuth{}, false, err
	}

	var auth persistentCLIAuth
	found, err := readTOMLFile(path, &auth)
	if err != nil {
		return persistentCLIAuth{}, false, err
	}
	return auth, found, nil
}

func readTOMLFile(path string, target any) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := toml.Unmarshal(payload, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", path, err)
	}
	return true, nil
}

func writePersistentCLIConfig(cfg persistentCLIConfig) error {
	if _, err := ensureCLIDataDir(); err != nil {
		return err
	}
	path, err := cliConfigPath()
	if err != nil {
		return err
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	return writeTOMLToPath(path, cfg)
}

func writePersistentCLIAuth(auth persistentCLIAuth) error {
	if _, err := ensureCLIDataDir(); err != nil {
		return err
	}
	path, err := cliAuthPath()
	if err != nil {
		return err
	}
	if auth.Version == 0 {
		auth.Version = 1
	}
	return writeTOMLToPath(path, auth)
}

func writeTOMLToPath(path string, value any) error {
	payload, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func deletePersistentCLIConfig() error {
	path, err := cliConfigPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func deletePersistentCLIAuth() error {
	path, err := cliAuthPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func looksLikeLocalServerDir(dir string) bool {
	markers := []string{
		".env",
		"config.toml",
		"go.mod",
		"main.go",
		filepath.Join("api", "openapi.yaml"),
		filepath.Join("cmd", "cli", "main.go"),
		filepath.Join("config", "config.example.toml"),
	}

	found := 0
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			found++
		}
	}

	return found >= 2
}

func httpStatusError(resp *http.Response) error {
	if resp == nil {
		return errors.New("request failed")
	}
	return fmt.Errorf("request failed: %s", resp.Status)
}
