package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce/internal/httpapi"
	"ecommerce/models"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func runSearchConfigurationTestCommand(input string, args ...string) (string, error) {
	command := &cobra.Command{Use: "search", SilenceErrors: true, SilenceUsage: true}
	command.PersistentFlags().String("format", "json", "")
	command.AddCommand(newSearchConfigurationCommands()...)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetIn(strings.NewReader(input))
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func TestSearchConfigurationCLILocalLifecycle(t *testing.T) {
	db := newTestDB(t, &models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchRankingProfile{}, &models.SearchMerchandisingRule{}, &models.SearchMerchandisingAudit{}, &models.Product{})
	endpoint, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	oldOpen, oldRuntime := searchCLIOpenEndpoints, activeCLIRuntime
	searchCLIOpenEndpoints = func(context.Context) (*httpapi.CatalogEndpoints, func(), error) { return endpoint, func() {}, nil }
	activeCLIRuntime = cliRuntime{}
	t.Cleanup(func() { searchCLIOpenEndpoints = oldOpen; activeCLIRuntime = oldRuntime })
	product := models.Product{Name: "Everyday shoes"}
	require.NoError(t, db.Create(&product).Error)
	weights := `"weights":{"token_coverage":1,"exact_phrase":0,"name":0,"brand":0,"attributes":0,"recency":0,"availability":0,"sales":0,"margin":0,"conversion":0}`
	for _, test := range []struct {
		group, input, patch string
		protect             bool
	}{
		{"synonyms", `{"name":"Footwear","direction":"bi","terms":["Sneakers","Shoes"],"is_active":false}`, `{"is_active":true}`, false},
		{"typo-profiles", `{"name":"Default","strict_mode":false}`, `{"strict_mode":true}`, true},
		{"ranking-profiles", `{"name":"Ranking",` + weights + `}`, `{"name":"Revised",` + strings.ReplaceAll(strings.ReplaceAll(weights, `"margin":0`, `"margin":3`), `"conversion":0`, `"conversion":5`) + `}`, true},
		{"merchandising-rules", fmt.Sprintf(`{"name":"Campaign","rule_type":"hide","is_active":false,"action":{"targets":[{"product_id":%d}]}}`, product.ID), `{"is_active":true}`, false},
	} {
		t.Run(test.group, func(t *testing.T) {
			output, err := runSearchConfigurationTestCommand(test.input, test.group, "create", "--file", "-")
			require.NoError(t, err)
			var created map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &created))
			id := fmt.Sprint(created["id"])
			if test.group == "synonyms" || test.group == "merchandising-rules" {
				require.Equal(t, false, created["is_active"])
			}
			_, err = runSearchConfigurationTestCommand("", test.group, "get", id)
			require.NoError(t, err)
			_, err = runSearchConfigurationTestCommand("", test.group, "list")
			require.NoError(t, err)
			output, err = runSearchConfigurationTestCommand(test.patch, test.group, "update", id, "--file", "-")
			require.NoError(t, err)
			if test.group == "ranking-profiles" {
				var updated struct {
					Version int `json:"version"`
					Weights struct {
						Margin     float64 `json:"margin"`
						Conversion float64 `json:"conversion"`
					} `json:"weights"`
				}
				require.NoError(t, json.Unmarshal([]byte(output), &updated))
				require.Equal(t, 2, updated.Version)
				require.Equal(t, float64(3), updated.Weights.Margin)
				require.Equal(t, float64(5), updated.Weights.Conversion)
				_, err = runSearchConfigurationTestCommand(`{"weights":{"margin":null}}`, test.group, "update", id, "--file", "-")
				require.Error(t, err)
				output, err = runSearchConfigurationTestCommand("", test.group, "get", id)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal([]byte(output), &updated))
				require.Equal(t, 2, updated.Version)
			}
			_, err = runSearchConfigurationTestCommand("no\n", test.group, "delete", id)
			require.ErrorContains(t, err, "canceled")
			if test.protect {
				_, err = runSearchConfigurationTestCommand("", test.group, "delete", id, "--yes")
				require.Error(t, err)
				replacement := `{"name":"Replacement","is_active":true}`
				if test.group == "ranking-profiles" {
					replacement = `{"name":"Replacement","is_default":true,` + weights + `}`
				}
				_, err = runSearchConfigurationTestCommand(replacement, test.group, "create", "--file", "-")
				require.NoError(t, err)
			}
			_, err = runSearchConfigurationTestCommand("", test.group, "delete", id, "--yes")
			require.NoError(t, err)
			_, err = runSearchConfigurationTestCommand("", test.group, "get", id)
			require.Error(t, err)
			if test.group == "merchandising-rules" {
				output, err = runSearchConfigurationTestCommand("", test.group, "audit", id)
				require.NoError(t, err)
				var audit struct {
					Data []struct {
						Operation string `json:"operation"`
					}
				}
				require.NoError(t, json.Unmarshal([]byte(output), &audit))
				require.Len(t, audit.Data, 3)
				require.Equal(t, "delete", audit.Data[0].Operation)
				_, err = runSearchConfigurationTestCommand("", "merchandising-audit")
				require.NoError(t, err)
			}
		})
	}
}

func TestSearchConfigurationCLIRejectsInputBeforeTransport(t *testing.T) {
	oldOpen, oldRuntime := searchCLIOpenEndpoints, activeCLIRuntime
	activeCLIRuntime = cliRuntime{}
	searchCLIOpenEndpoints = func(context.Context) (*httpapi.CatalogEndpoints, func(), error) {
		t.Fatal("invalid command reached database")
		return nil, nil, nil
	}
	t.Cleanup(func() { searchCLIOpenEndpoints = oldOpen; activeCLIRuntime = oldRuntime })
	for _, test := range []struct {
		input string
		args  []string
	}{
		{`{"unknown":true}`, []string{"synonyms", "create", "--file", "-"}},
		{`{} {}`, []string{"typo-profiles", "create", "--file", "-"}},
		{`null`, []string{"ranking-profiles", "create", "--file", "-"}},
		{`{}`, []string{"synonyms", "create", "--file", "-", "--format", "xml"}},
		{``, []string{"synonyms", "get", "0"}},
		{``, []string{"synonyms", "delete", "-1", "--yes"}},
		{``, []string{"synonyms", "create"}},
		{`{"name":"Campaign","rule_type":"hide","action":{"targets":[{"product_id":1}]}}`, []string{"merchandising-rules", "create", "--file", "-", "--product-query", "shoes"}},
		{`{"action":{"targets":[]}}`, []string{"merchandising-rules", "update", "1", "--file", "-", "--product-query", "shoes"}},
	} {
		_, err := runSearchConfigurationTestCommand(test.input, test.args...)
		require.Error(t, err, "%v", test.args)
	}
}

func TestSearchConfigurationCLIRemotePathsAndPreviewReplacement(t *testing.T) {
	var method, path string
	var body map[string]json.RawMessage
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		method, path = r.Method, r.URL.Path
		body = nil
		if r.Body != nil && r.ContentLength != 0 {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"data":[]}`))
	}))
	defer server.Close()
	oldRuntime := activeCLIRuntime
	activeCLIRuntime = cliRuntime{Remote: &persistentCLIAuth{APIURL: server.URL, Token: "test-token"}}
	t.Cleanup(func() { activeCLIRuntime = oldRuntime })
	for _, group := range []string{"synonyms", "typo-profiles", "ranking-profiles", "merchandising-rules"} {
		for _, test := range []struct{ verb, method, suffix string }{
			{"list", http.MethodGet, ""}, {"get", http.MethodGet, "/1"}, {"create", http.MethodPost, ""}, {"update", http.MethodPatch, "/1"}, {"delete", http.MethodDelete, "/1"},
		} {
			args := []string{group, test.verb}
			if test.suffix != "" {
				args = append(args, "1")
			}
			if test.verb == "create" || test.verb == "update" {
				args = append(args, "--file", "-")
			}
			if test.verb == "delete" {
				args = append(args, "--yes")
			}
			_, err := runSearchConfigurationTestCommand(`{}`, args...)
			require.NoError(t, err)
			require.Equal(t, test.method, method)
			require.Equal(t, "/api/v1/admin/search/"+group+test.suffix, path)
		}
	}
	for _, test := range []struct {
		input     string
		wantRules bool
	}{
		{`{"filters":{}}`, false}, {`{"filters":{},"rules":[]}`, true},
	} {
		_, err := runSearchConfigurationTestCommand(test.input, "preview", "--file", "-")
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, method)
		require.Equal(t, "/api/v1/admin/search/preview", path)
		_, exists := body["rules"]
		require.Equal(t, test.wantRules, exists)
		if exists {
			require.JSONEq(t, `[]`, string(body["rules"]))
		}
	}
	_, err := runSearchConfigurationTestCommand("", "merchandising-rules", "audit", "1")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/admin/search/merchandising-rules/1/audit", path)
	_, err = runSearchConfigurationTestCommand("", "merchandising-audit")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/admin/search/merchandising-audit", path)
	require.Equal(t, 24, calls)
}

func TestSearchMerchandisingCLILookupAndAmbiguity(t *testing.T) {
	for _, test := range []struct {
		name, response string
		wantID         int
		wantError      string
	}{
		{"single", `{"items":[{"id":27,"name":"Everyday shoes"}],"pagination":{"total":1}}`, 27, ""},
		{"exact name", `{"items":[{"id":27,"name":"Everyday shoes"},{"id":28,"name":"Everyday shoes kit"}],"pagination":{"total":2}}`, 27, ""},
		{"duplicate name", `{"items":[{"id":27,"name":"Everyday shoes"},{"id":28,"name":"Everyday shoes"}],"pagination":{"total":2}}`, 0, "ambiguous"},
		{"absent", `{"items":[],"pagination":{"total":0}}`, 0, "no indexed published product"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					require.Equal(t, "/api/v1/admin/search/products", r.URL.Path)
					require.Equal(t, "Everyday shoes", r.URL.Query().Get("q"))
					_, _ = w.Write([]byte(test.response))
					return
				}
				writes++
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/v1/admin/search/merchandising-rules", r.URL.Path)
				var body struct {
					Action struct {
						Targets []struct {
							ProductID int `json:"product_id"`
							Position  int `json:"position"`
						} `json:"targets"`
					} `json:"action"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body.Action.Targets, 1)
				require.Equal(t, test.wantID, body.Action.Targets[0].ProductID)
				require.Equal(t, 2, body.Action.Targets[0].Position)
				_, _ = w.Write([]byte(`{"id":1}`))
			}))
			defer server.Close()
			oldRuntime := activeCLIRuntime
			activeCLIRuntime = cliRuntime{Remote: &persistentCLIAuth{APIURL: server.URL, Token: "test-token"}}
			defer func() { activeCLIRuntime = oldRuntime }()
			_, err := runSearchConfigurationTestCommand(`{"name":"Campaign","rule_type":"pin","action":{"targets":[{"position":2}]}}`, "merchandising-rules", "create", "--file", "-", "--product-query", "Everyday shoes")
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				require.Zero(t, writes)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, writes)
			}
		})
	}
}
