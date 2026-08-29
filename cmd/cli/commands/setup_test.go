package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ecommerce/internal/apicontract"
)

type fakeSetupPrompter struct {
	infoMessages    []string
	textLabels      []string
	passwordLabels  []string
	choiceLabels    []string
	textResponses   []string
	passwordResults []string
	choiceResults   []string
}

func (p *fakeSetupPrompter) Info(text string) {
	p.infoMessages = append(p.infoMessages, text)
}

func (p *fakeSetupPrompter) Text(label string, defaultValue string) (string, error) {
	p.textLabels = append(p.textLabels, label)
	if len(p.textResponses) == 0 {
		return defaultValue, nil
	}
	value := p.textResponses[0]
	p.textResponses = p.textResponses[1:]
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func (p *fakeSetupPrompter) Password(label string) (string, error) {
	p.passwordLabels = append(p.passwordLabels, label)
	if len(p.passwordResults) == 0 {
		return "", nil
	}
	value := p.passwordResults[0]
	p.passwordResults = p.passwordResults[1:]
	return value, nil
}

func (p *fakeSetupPrompter) Choice(label string, choices []string) (string, error) {
	p.choiceLabels = append(p.choiceLabels, label)
	value := p.choiceResults[0]
	p.choiceResults = p.choiceResults[1:]
	return value, nil
}

func TestRunSetupShowsTargetModeHelpAndSavesPathConfig(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "cli-data")
	t.Setenv(cliDataDirEnv, dataDir)

	serverDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(serverDir, ".env"), []byte("PORT=3000\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	prompter := &fakeSetupPrompter{
		choiceResults: []string{"path"},
		textResponses: []string{serverDir},
	}

	if err := runSetup(prompter); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	if len(prompter.infoMessages) != 1 || prompter.infoMessages[0] != targetModePromptHelp {
		t.Fatalf("expected target mode help %q, got %#v", targetModePromptHelp, prompter.infoMessages)
	}

	cfg, found, err := loadPersistentCLIConfig()
	if err != nil {
		t.Fatalf("loadPersistentCLIConfig: %v", err)
	}
	if !found {
		t.Fatal("expected persisted CLI config")
	}
	if cfg.Mode != cliTargetModePath {
		t.Fatalf("expected path mode, got %q", cfg.Mode)
	}

	absolutePath, err := filepath.Abs(serverDir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if cfg.Path != absolutePath {
		t.Fatalf("expected saved path %q, got %q", absolutePath, cfg.Path)
	}
}

func TestRunLocalAccountLoginUsesPasswordPrompt(t *testing.T) {
	const expectedEmail = "admin@example.com"
	const expectedPassword = "supersecret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["email"] != expectedEmail {
			t.Fatalf("expected email %q, got %q", expectedEmail, payload["email"])
		}
		if payload["password"] != expectedPassword {
			t.Fatalf("expected password %q, got %q", expectedPassword, payload["password"])
		}

		token := "token-123"
		resp := cliAuthResponse{
			Token: &token,
			User: apicontract.User{
				Email:    expectedEmail,
				Username: "admin",
				Role:     apicontract.UserRoleAdmin,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	prompter := &fakeSetupPrompter{
		textResponses:   []string{expectedEmail},
		passwordResults: []string{expectedPassword},
	}

	auth, err := runLocalAccountLogin(prompter, server.URL)
	if err != nil {
		t.Fatalf("runLocalAccountLogin: %v", err)
	}

	if len(prompter.passwordLabels) != 1 || prompter.passwordLabels[0] != "Password" {
		t.Fatalf("expected password prompt, got %#v", prompter.passwordLabels)
	}
	if got := len(prompter.textLabels); got != 1 || prompter.textLabels[0] != "Email" {
		t.Fatalf("expected only email text prompt, got %#v", prompter.textLabels)
	}
	if auth.Token != "token-123" {
		t.Fatalf("expected token to be saved, got %q", auth.Token)
	}
	if auth.AuthMethod != "local" {
		t.Fatalf("expected local auth method, got %q", auth.AuthMethod)
	}
}

func TestPromptTemplatesUsePlainArrowMarker(t *testing.T) {
	templates := newPromptTemplates()

	if templates.Valid != `{{ ">" | bold }} {{ . | bold }}: ` {
		t.Fatalf("unexpected valid template %q", templates.Valid)
	}
	if templates.Success != `{{ ">" | faint }} {{ . | faint }}: ` {
		t.Fatalf("unexpected success template %q", templates.Success)
	}
}

func TestSelectTemplatesUsePlainArrowMarker(t *testing.T) {
	templates := newSelectTemplates()

	if templates.Selected != `> {{ . | faint }}` {
		t.Fatalf("unexpected selected template %q", templates.Selected)
	}
}
