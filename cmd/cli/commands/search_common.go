package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ecommerce/config"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/migrations"
	searchservice "ecommerce/internal/search"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Local commands are trusted database administration; they do not invent an API actor.
func openCLIDatabase(ctx context.Context) (*gorm.DB, config.Config, func(), error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, cfg, nil, err
	}
	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("connect to database: %w", err)
	}
	close := func() { closeDB(db) }
	if err = migrations.EnsureReady(db.WithContext(ctx), cfg.AutoApplyMigrations); err != nil {
		close()
		return nil, cfg, nil, err
	}
	return db, cfg, close, nil
}
func openCLICatalogEndpoints(ctx context.Context) (*httpapi.CatalogEndpoints, func(), error) {
	db, cfg, close, err := openCLIDatabase(ctx)
	if err != nil {
		return nil, nil, err
	}
	e, err := httpapi.NewCatalogEndpoints(db, nil, newJobRuntime(db, cfg))
	if err != nil {
		close()
		return nil, nil, err
	}
	err = e.ConfigureSearchHardening(searchservice.HardeningConfig{MaxConcurrent: cfg.SearchMaxConcurrent, SearchTimeoutMS: cfg.SearchTimeoutMS, CircuitFailureThreshold: cfg.SearchCircuitFailureThreshold, CircuitOpenMS: cfg.SearchCircuitOpenMS, ReindexQueueLimit: cfg.SearchReindexQueueLimit})
	if err != nil {
		close()
		return nil, nil, err
	}
	return e, close, nil
}

var searchCLIOpenEndpoints = openCLICatalogEndpoints

func invokeCLIJSON[T any](ctx context.Context, method, path string, body any) (T, error) {
	var zero T
	auth, err := currentRemoteAuth()
	if err != nil {
		return zero, err
	}
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return zero, err
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(auth.APIURL, "/")+path, input)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return zero, err
	}
	if len(data) > 16*1024*1024 {
		return zero, fmt.Errorf("API response exceeds 16 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var problem struct {
			Detail  string `json:"detail"`
			Title   string `json:"title"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &problem)
		for _, detail := range []string{problem.Detail, problem.Message, problem.Error, problem.Title} {
			if detail != "" {
				return zero, fmt.Errorf("API returned %d: %s", resp.StatusCode, detail)
			}
		}
		return zero, fmt.Errorf("API returned %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(data)) == 0 {
		return zero, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode API response: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return zero, fmt.Errorf("API response must contain exactly one JSON value")
	}
	return zero, nil
}

var searchCLIOpenDatabase = openCLIDatabase

func searchCLIRequest(cmd *cobra.Command, method, path string, body any, local func(context.Context, *httpapi.CatalogEndpoints) (any, error)) (any, error) {
	if err := validateSearchFormat(cmd); err != nil {
		return nil, err
	}
	if isRemoteMode() {
		return invokeCLIJSON[any](cmd.Context(), method, path, body)
	}
	e, close, err := searchCLIOpenEndpoints(cmd.Context())
	if err != nil {
		return nil, err
	}
	defer close()
	return local(cmd.Context(), e)
}

func validateSearchFormat(cmd *cobra.Command) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("format must be text or json")
	}
	return nil
}
func writeSearchOutput(cmd *cobra.Command, value any) error {
	if err := validateSearchFormat(cmd); err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString("format")
	if format == "json" {
		return writeJSON(cmd.OutOrStdout(), value)
	}
	// Use JSON field names and omit absent fields in readable structured text.
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var normalized any
	if err = json.Unmarshal(data, &normalized); err != nil {
		return err
	}
	data, err = yaml.Marshal(normalized)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(data)
	return err
}
func readSearchInput(cmd *cobra.Command, path string, target any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--file is required (use - for stdin)")
	}
	var reader io.Reader = cmd.InOrStdin()
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, 1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 1024*1024 {
		return fmt.Errorf("input exceeds 1 MiB")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return fmt.Errorf("input must be a JSON object")
	}
	var raw any
	if err = json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}
	if containsSearchNull(raw) {
		return fmt.Errorf("search input fields cannot be null; omit unchanged fields and use clear_starts_at/clear_ends_at to clear schedules")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("input must contain exactly one JSON object")
	}
	return nil
}
func searchIDArg(args []string) (int, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("one positive ID is required")
	}
	id, err := strconv.Atoi(args[0])
	if err != nil || id < 1 {
		return 0, fmt.Errorf("ID must be a positive integer")
	}
	return id, nil
}
func confirmSearchDelete(cmd *cobra.Command, id int) error {
	if err := validateSearchFormat(cmd); err != nil {
		return err
	}
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Delete %d? Type yes to confirm: ", id)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	if strings.TrimSpace(line) != "yes" {
		return fmt.Errorf("deletion canceled; use --yes to confirm")
	}
	return nil
}

func containsSearchNull(value any) bool {
	if value == nil {
		return true
	}
	switch value := value.(type) {
	case map[string]any:
		for _, item := range value {
			if containsSearchNull(item) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if containsSearchNull(item) {
				return true
			}
		}
	}
	return false
}
