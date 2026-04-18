package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"ecommerce/internal/apicontract"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type cliAuthResponse struct {
	Token *string          `json:"token,omitempty"`
	User  apicontract.User `json:"user"`
}

const targetModePromptHelp = "Path mode points the CLI at a local ecommerce API checkout on disk.\nAuth mode points it at a remote ecommerce API URL and stores login details for remote requests."

type setupPrompter interface {
	Info(text string)
	Text(label string, defaultValue string) (string, error)
	Password(label string) (string, error)
	Choice(label string, choices []string) (string, error)
}

type linePrompter struct {
	reader *bufio.Reader
	stdout io.Writer
}

type promptuiPrompter struct {
	stdin  io.ReadCloser
	stdout io.WriteCloser
}

func newPromptTemplates() *promptui.PromptTemplates {
	return &promptui.PromptTemplates{
		Valid:   `{{ ">" | bold }} {{ . | bold }}: `,
		Success: `{{ ">" | faint }} {{ . | faint }}: `,
	}
}

func newSelectTemplates() *promptui.SelectTemplates {
	return &promptui.SelectTemplates{
		Selected: `> {{ . | faint }}`,
	}
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Configure the CLI target and authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(newSetupPrompter(os.Stdin, os.Stdout))
		},
	}
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and update the persistent CLI config",
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigUsePathCmd())
	cmd.AddCommand(newConfigUseAuthCmd())
	cmd.AddCommand(newConfigLoginCmd())
	cmd.AddCommand(newConfigLogoutCmd())
	cmd.AddCommand(newConfigClearCmd())

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the saved CLI config and auth metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, found, err := loadPersistentCLIConfig()
			if err != nil {
				return err
			}

			auth, authFound, err := loadPersistentCLIAuth()
			if err != nil {
				return err
			}

			payload := map[string]any{
				"data_dir": mustCLIDataDir(),
				"config":   nil,
				"auth":     nil,
			}

			if found {
				payload["config"] = cfg
			}
			if authFound {
				payload["auth"] = map[string]any{
					"version":     auth.Version,
					"api_url":     auth.APIURL,
					"auth_method": auth.AuthMethod,
					"user_email":  auth.UserEmail,
					"user_name":   auth.UserName,
					"user_role":   auth.UserRole,
					"token":       redactToken(auth.Token),
				}
			}

			printJSON(payload)
			return nil
		},
	}
}

func newConfigUsePathCmd() *cobra.Command {
	var configuredPath string

	cmd := &cobra.Command{
		Use:   "use-path",
		Short: "Point the CLI at a local ecommerce API server path",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := filepath.Abs(strings.TrimSpace(configuredPath))
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}
			if _, err := os.Stat(targetPath); err != nil {
				return fmt.Errorf("stat path %s: %w", targetPath, err)
			}
			if !looksLikeLocalServerDir(targetPath) {
				return fmt.Errorf("%s does not look like an ecommerce API server directory", targetPath)
			}

			if err := writePersistentCLIConfig(persistentCLIConfig{
				Version: 1,
				Mode:    cliTargetModePath,
				Path:    targetPath,
			}); err != nil {
				return err
			}

			fmt.Printf("Configured CLI path target: %s\n", targetPath)
			return nil
		},
		Args: cobra.NoArgs,
	}

	cmd.Flags().StringVar(&configuredPath, "path", "", "Path to the ecommerce API server")
	cmd.MarkFlagRequired("path")
	return cmd
}

func newConfigUseAuthCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "use-auth",
		Short: "Point the CLI at a remote ecommerce API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalized, err := normalizeAPIBaseURL(apiURL)
			if err != nil {
				return err
			}

			if err := writePersistentCLIConfig(persistentCLIConfig{
				Version: 1,
				Mode:    cliTargetModeAuth,
				APIURL:  normalized,
			}); err != nil {
				return err
			}

			fmt.Printf("Configured remote API target: %s\n", normalized)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "Remote ecommerce API base URL")
	cmd.MarkFlagRequired("api-url")
	return cmd
}

func newConfigLoginCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the configured remote API and refresh the auth file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthSetup(newSetupPrompter(os.Stdin, os.Stdout), apiURL)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "Remote ecommerce API base URL (overrides saved config)")
	return cmd
}

func newConfigLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the saved CLI auth token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deletePersistentCLIAuth(); err != nil {
				return err
			}
			fmt.Println("Deleted CLI auth file")
			return nil
		},
	}
}

func newConfigClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Delete the saved CLI config and auth files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deletePersistentCLIConfig(); err != nil {
				return err
			}
			if err := deletePersistentCLIAuth(); err != nil {
				return err
			}
			fmt.Println("Deleted CLI config and auth files")
			return nil
		},
	}
}

func newSetupPrompter(stdin *os.File, stdout *os.File) setupPrompter {
	if term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		return &promptuiPrompter{stdin: stdin, stdout: stdout}
	}

	return &linePrompter{
		reader: bufio.NewReader(stdin),
		stdout: stdout,
	}
}

func runSetup(prompter setupPrompter) error {
	prompter.Info(targetModePromptHelp)

	mode, err := prompter.Choice("Target mode", []string{"path", "auth"})
	if err != nil {
		return err
	}

	switch mode {
	case "path":
		return runPathSetup(prompter)
	case "auth":
		return runAuthSetup(prompter, "")
	default:
		return fmt.Errorf("unsupported setup mode %q", mode)
	}
}

func runPathSetup(prompter setupPrompter) error {
	path, err := prompter.Text("Path to ecommerce API server", "")
	if err != nil {
		return err
	}

	absolutePath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if !looksLikeLocalServerDir(absolutePath) {
		return fmt.Errorf("%s does not look like an ecommerce API server directory", absolutePath)
	}

	if err := writePersistentCLIConfig(persistentCLIConfig{
		Version: 1,
		Mode:    cliTargetModePath,
		Path:    absolutePath,
	}); err != nil {
		return err
	}

	fmt.Printf("Saved CLI config in path mode: %s\n", absolutePath)
	return nil
}

func runAuthSetup(prompter setupPrompter, explicitAPIURL string) error {
	apiURL, err := resolveSetupAPIURL(prompter, explicitAPIURL)
	if err != nil {
		return err
	}

	authConfig, err := fetchRemoteAuthConfig(apiURL)
	if err != nil {
		return err
	}

	authMethod, err := chooseAuthMethod(prompter, authConfig)
	if err != nil {
		return err
	}

	var auth persistentCLIAuth
	switch authMethod {
	case "local":
		auth, err = runLocalAccountLogin(prompter, apiURL)
	case "oidc":
		auth, err = runOIDCLogin(prompter, apiURL)
	default:
		err = fmt.Errorf("unsupported auth method %q", authMethod)
	}
	if err != nil {
		return err
	}

	if err := writePersistentCLIConfig(persistentCLIConfig{
		Version: 1,
		Mode:    cliTargetModeAuth,
		APIURL:  apiURL,
	}); err != nil {
		return err
	}
	if err := writePersistentCLIAuth(auth); err != nil {
		return err
	}

	fmt.Printf("Saved CLI remote target: %s\n", apiURL)
	fmt.Printf("Saved CLI auth for %s (%s)\n", auth.UserEmail, auth.UserRole)
	return nil
}

func resolveSetupAPIURL(prompter setupPrompter, explicitAPIURL string) (string, error) {
	if strings.TrimSpace(explicitAPIURL) != "" {
		return normalizeAPIBaseURL(explicitAPIURL)
	}

	if cfg, found, err := loadPersistentCLIConfig(); err == nil && found && strings.TrimSpace(cfg.APIURL) != "" {
		value, err := prompter.Text("API URL", cfg.APIURL)
		if err != nil {
			return "", err
		}
		return normalizeAPIBaseURL(value)
	}

	value, err := prompter.Text("API URL", "http://localhost:3000")
	if err != nil {
		return "", err
	}
	return normalizeAPIBaseURL(value)
}

func chooseAuthMethod(prompter setupPrompter, authConfig apicontract.AuthConfigResponse) (string, error) {
	switch {
	case authConfig.LocalSignInEnabled && authConfig.OidcEnabled:
		return prompter.Choice("Authentication method", []string{"local", "oidc"})
	case authConfig.LocalSignInEnabled:
		fmt.Println("Server auth mode: local sign-in")
		return "local", nil
	case authConfig.OidcEnabled:
		fmt.Println("Server auth mode: OIDC")
		return "oidc", nil
	default:
		return "", errors.New("the server does not have any supported sign-in methods enabled")
	}
}

func runLocalAccountLogin(prompter setupPrompter, apiURL string) (persistentCLIAuth, error) {
	email, err := prompter.Text("Email", "")
	if err != nil {
		return persistentCLIAuth{}, err
	}
	password, err := prompter.Password("Password")
	if err != nil {
		return persistentCLIAuth{}, err
	}

	resp, err := doRemoteJSON[cliAuthResponse](http.MethodPost, apiURL+"/api/v1/auth/login", map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	}, "")
	if err != nil {
		return persistentCLIAuth{}, err
	}
	if resp.Token == nil || strings.TrimSpace(*resp.Token) == "" {
		return persistentCLIAuth{}, errors.New("login succeeded but the server did not return a bearer token")
	}

	return persistentCLIAuth{
		Version:    1,
		APIURL:     apiURL,
		Token:      strings.TrimSpace(*resp.Token),
		AuthMethod: "local",
		UserEmail:  resp.User.Email,
		UserName:   resp.User.Username,
		UserRole:   string(resp.User.Role),
	}, nil
}

func runOIDCLogin(prompter setupPrompter, apiURL string) (persistentCLIAuth, error) {
	loginURL := apiURL + "/api/v1/auth/oidc/login?response_format=json"
	fmt.Printf("Open this URL in a browser to sign in with OIDC:\n%s\n", loginURL)
	_ = tryOpenBrowser(loginURL)

	rawToken, err := prompter.Text("Paste the token or the full JSON response", "")
	if err != nil {
		return persistentCLIAuth{}, err
	}
	token, err := parsePastedToken(rawToken)
	if err != nil {
		return persistentCLIAuth{}, err
	}

	user, err := fetchRemoteProfile(apiURL, token)
	if err != nil {
		return persistentCLIAuth{}, err
	}

	return persistentCLIAuth{
		Version:    1,
		APIURL:     apiURL,
		Token:      token,
		AuthMethod: "oidc",
		UserEmail:  user.Email,
		UserName:   user.Username,
		UserRole:   string(user.Role),
	}, nil
}

func fetchRemoteAuthConfig(apiURL string) (apicontract.AuthConfigResponse, error) {
	return doRemoteJSON[apicontract.AuthConfigResponse](http.MethodGet, apiURL+"/api/v1/auth/config", nil, "")
}

func fetchRemoteProfile(apiURL string, token string) (apicontract.User, error) {
	return doRemoteJSON[apicontract.User](http.MethodGet, apiURL+"/api/v1/me/", nil, token)
}

func doRemoteJSON[T any](method string, targetURL string, body any, token string) (T, error) {
	var zero T

	var requestBody io.Reader = http.NoBody
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return zero, err
		}
		requestBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, targetURL, requestBody)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return zero, decodeHandlerError(resp.StatusCode, payload)
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return zero, nil
	}

	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return zero, err
	}
	return value, nil
}

func parsePastedToken(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("token is required")
	}

	if strings.HasPrefix(trimmed, "{") {
		var resp cliAuthResponse
		if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
			return "", fmt.Errorf("decode pasted JSON: %w", err)
		}
		if resp.Token == nil || strings.TrimSpace(*resp.Token) == "" {
			return "", errors.New("the pasted JSON did not include a token")
		}
		return strings.TrimSpace(*resp.Token), nil
	}

	return trimmed, nil
}

func (p *linePrompter) Info(text string) {
	fmt.Fprintln(p.stdout, text)
}

func (p *linePrompter) Text(label string, defaultValue string) (string, error) {
	if strings.TrimSpace(defaultValue) != "" {
		fmt.Fprintf(p.stdout, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.stdout, "%s: ", label)
	}

	value, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func (p *linePrompter) Password(label string) (string, error) {
	return p.Text(label, "")
}

func (p *linePrompter) Choice(label string, choices []string) (string, error) {
	fmt.Fprintf(p.stdout, "%s (%s): ", label, strings.Join(choices, "/"))
	value, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, choice := range choices {
		if value == choice {
			return choice, nil
		}
	}
	return "", fmt.Errorf("expected one of %s", strings.Join(choices, ", "))
}

func (p *promptuiPrompter) Info(text string) {
	fmt.Fprintln(p.stdout, text)
}

func (p *promptuiPrompter) Text(label string, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:     label,
		Default:   defaultValue,
		AllowEdit: true,
		Templates: newPromptTemplates(),
		Stdin:     p.stdin,
		Stdout:    p.stdout,
	}

	return prompt.Run()
}

func (p *promptuiPrompter) Password(label string) (string, error) {
	prompt := promptui.Prompt{
		Label:       label,
		Mask:        '*',
		HideEntered: true,
		Templates:   newPromptTemplates(),
		Stdin:       p.stdin,
		Stdout:      p.stdout,
	}

	return prompt.Run()
}

func (p *promptuiPrompter) Choice(label string, choices []string) (string, error) {
	selectPrompt := promptui.Select{
		Label:     label,
		Items:     choices,
		Size:      len(choices),
		Templates: newSelectTemplates(),
		Stdin:     p.stdin,
		Stdout:    p.stdout,
	}

	_, value, err := selectPrompt.Run()
	return value, err
}

func tryOpenBrowser(targetURL string) error {
	var command []string
	switch runtime.GOOS {
	case "darwin":
		command = []string{"open", targetURL}
	case "windows":
		command = []string{"rundll32", "url.dll,FileProtocolHandler", targetURL}
	default:
		command = []string{"xdg-open", targetURL}
	}

	if len(command) == 0 {
		return nil
	}
	return exec.Command(command[0], command[1:]...).Start()
}

func redactToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "********"
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}

func mustCLIDataDir() string {
	dir, err := cliDataDir()
	if err != nil {
		return ""
	}
	return dir
}
