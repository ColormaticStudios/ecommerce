package checkoutplugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultExternalPluginTimeout = 3 * time.Second
	maxExternalPluginTimeout     = 15 * time.Second
)

type externalProvider struct {
	definition Definition
	command    string
	args       []string
	timeout    time.Duration
	workDir    string
}

type externalPluginManifest struct {
	ID          string            `json:"id"`
	Type        ProviderType      `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Fields      []FieldDefinition `json:"fields"`
	States      []State           `json:"states"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	TimeoutMS   int               `json:"timeout_ms"`
	baseDir     string
}

type externalPluginRequest struct {
	Action string              `json:"action"`
	Input  externalPluginInput `json:"input"`
}

type externalPluginInput struct {
	ProviderID   string            `json:"provider_id"`
	ProviderType ProviderType      `json:"provider_type"`
	Currency     string            `json:"currency"`
	Subtotal     float64           `json:"subtotal"`
	TaxableBase  float64           `json:"taxable_base"`
	Data         map[string]string `json:"data"`
}

type externalQuoteResponse struct {
	Valid  bool    `json:"valid"`
	Amount float64 `json:"amount"`
	States []State `json:"states"`
	Error  string  `json:"error"`
}

type externalResolveResponse struct {
	Valid           bool    `json:"valid"`
	PaymentDisplay  string  `json:"payment_display"`
	ShippingAddress string  `json:"shipping_address"`
	States          []State `json:"states"`
	Error           string  `json:"error"`
}

func (m *Manager) LoadExternalPluginsFromDir(dir string) (int, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(trimmedDir)
	if err != nil {
		return 0, fmt.Errorf("read checkout plugin manifest dir: %w", err)
	}

	registered := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(trimmedDir, entry.Name())
		manifest, err := readExternalPluginManifest(path)
		if err != nil {
			return registered, err
		}
		if err := m.registerExternalManifest(manifest); err != nil {
			return registered, err
		}
		registered++
	}

	return registered, nil
}

func readExternalPluginManifest(path string) (externalPluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return externalPluginManifest{}, fmt.Errorf("read checkout plugin manifest %s: %w", path, err)
	}

	var manifest externalPluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return externalPluginManifest{}, fmt.Errorf("parse checkout plugin manifest %s: %w", path, err)
	}

	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Status = strings.TrimSpace(manifest.Status)
	manifest.Command = strings.TrimSpace(manifest.Command)
	if manifest.Status == "" {
		manifest.Status = "available"
	}
	if manifest.ID == "" || manifest.Name == "" || manifest.Command == "" {
		return externalPluginManifest{}, fmt.Errorf("invalid checkout plugin manifest %s: id, name, and command are required", path)
	}
	if manifest.Type != ProviderTypePayment && manifest.Type != ProviderTypeShipping && manifest.Type != ProviderTypeTax {
		return externalPluginManifest{}, fmt.Errorf("invalid checkout plugin manifest %s: unsupported type %q", path, manifest.Type)
	}

	manifest.baseDir = filepath.Dir(path)

	if !filepath.IsAbs(manifest.Command) && strings.Contains(manifest.Command, string(os.PathSeparator)) {
		manifest.Command = filepath.Clean(filepath.Join(manifest.baseDir, manifest.Command))
	}
	for i, arg := range manifest.Args {
		trimmedArg := strings.TrimSpace(arg)
		if trimmedArg == "" {
			continue
		}
		if filepath.IsAbs(trimmedArg) {
			manifest.Args[i] = filepath.Clean(trimmedArg)
			continue
		}
		if strings.HasPrefix(trimmedArg, "-") {
			continue
		}
		if strings.Contains(trimmedArg, string(os.PathSeparator)) {
			manifest.Args[i] = filepath.Clean(filepath.Join(manifest.baseDir, trimmedArg))
		}
	}

	return manifest, nil
}

func (m *Manager) registerExternalManifest(manifest externalPluginManifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	definition := Definition{
		ID:          manifest.ID,
		Type:        manifest.Type,
		Name:        manifest.Name,
		Description: manifest.Description,
		Status:      manifest.Status,
		Fields:      manifest.Fields,
		States:      manifest.States,
	}

	timeout := defaultExternalPluginTimeout
	if manifest.TimeoutMS > 0 {
		timeout = time.Duration(manifest.TimeoutMS) * time.Millisecond
	}
	if timeout > maxExternalPluginTimeout {
		timeout = maxExternalPluginTimeout
	}

	provider := &externalProvider{
		definition: definition,
		command:    manifest.Command,
		args:       manifest.Args,
		timeout:    timeout,
		workDir:    manifest.baseDir,
	}

	internalRegistry, externalRegistry := m.providerRegistries(manifest.Type)
	if _, exists := internalRegistry[manifest.ID]; exists {
		return fmt.Errorf("checkout plugin id %q conflicts with built-in provider", manifest.ID)
	}
	if _, exists := externalRegistry[manifest.ID]; exists {
		return fmt.Errorf("checkout plugin id %q is duplicated", manifest.ID)
	}

	externalRegistry[manifest.ID] = provider
	if manifest.Type == ProviderTypeTax {
		if m.hasActiveTaxLocked() {
			m.enabled[providerSettingKey(manifest.Type, manifest.ID)] = false
		} else {
			m.enabled[providerSettingKey(manifest.Type, manifest.ID)] = true
		}
		m.normalizeTaxSelectionLocked()
	} else {
		m.enabled[providerSettingKey(manifest.Type, manifest.ID)] = true
	}
	return nil
}

func (p *externalProvider) quote(subtotal, taxableBase float64, currency string, data map[string]string) (externalQuoteResponse, error) {
	response := externalQuoteResponse{Valid: true}
	err := p.runExternal("quote", subtotal, taxableBase, currency, data, &response)
	if err != nil {
		return externalQuoteResponse{}, err
	}
	if response.Error != "" {
		return externalQuoteResponse{}, errors.New(response.Error)
	}
	return response, nil
}

func (p *externalProvider) resolve(subtotal, taxableBase float64, currency string, data map[string]string) (externalResolveResponse, error) {
	response := externalResolveResponse{Valid: true}
	err := p.runExternal("resolve", subtotal, taxableBase, currency, data, &response)
	if err != nil {
		return externalResolveResponse{}, err
	}
	if response.Error != "" {
		return externalResolveResponse{}, errors.New(response.Error)
	}
	if !response.Valid {
		return externalResolveResponse{}, fmt.Errorf("external plugin %s returned invalid resolve response", p.definition.ID)
	}
	return response, nil
}

func (p *externalProvider) runExternal(action string, subtotal, taxableBase float64, currency string, data map[string]string, into any) error {
	if data == nil {
		data = map[string]string{}
	}

	request := externalPluginRequest{
		Action: action,
		Input: externalPluginInput{
			ProviderID:   p.definition.ID,
			ProviderType: p.definition.Type,
			Currency:     currency,
			Subtotal:     subtotal,
			TaxableBase:  taxableBase,
			Data:         data,
		},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.command, p.args...)
	if p.workDir != "" {
		cmd.Dir = p.workDir
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("external plugin %s timed out", p.definition.ID)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("external plugin %s failed: %s", p.definition.ID, message)
	}

	if err := json.Unmarshal(stdout.Bytes(), into); err != nil {
		return fmt.Errorf("external plugin %s returned invalid json: %w", p.definition.ID, err)
	}

	return nil
}

func AvailableExternalPluginManifests(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}
