package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	searchservice "ecommerce/internal/search"
	"github.com/spf13/cobra"
)

func NewSearchCmd() *cobra.Command {
	command := &cobra.Command{Use: "search", Short: "Administer search configuration, relevance, and index health"}
	command.PersistentFlags().String("format", "text", "Output format: text or json")
	command.AddCommand(newSearchConfigurationCommands()...)
	command.AddCommand(newSearchScaffoldCmd(), newSearchProductsCmd(), newSearchAnalyticsCmd(), newSearchFreshnessCmd(), newSearchOperationsCmd(), newSearchIncidentsCmd(), newSearchEvaluateCmd())
	for _, kind := range []string{"reindex", "refresh-sales", "refresh-conversion"} {
		command.AddCommand(newSearchJobCmd(kind))
	}
	configureSearchCommand(command)
	return command
}

func newSearchProductsCmd() *cobra.Command {
	var file, q, profile, sort, order string
	var page, limit int
	cmd := &cobra.Command{Use: "products", Short: "Search products with admin score explanations and facets", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		params := apicontract.SearchAdminProductsParams{}
		if file != "" {
			if err := readSearchInput(cmd, file, &params); err != nil {
				return err
			}
		}
		if cmd.Flags().Changed("q") {
			params.Q = &q
		}
		if cmd.Flags().Changed("ranking-profile") {
			params.RankingProfile = &profile
		}
		if cmd.Flags().Changed("sort") {
			value := apicontract.SearchAdminProductsParamsSort(sort)
			params.Sort = &value
		}
		if cmd.Flags().Changed("order") {
			value := apicontract.SearchAdminProductsParamsOrder(order)
			params.Order = &value
		}
		if cmd.Flags().Changed("page") || params.Page == nil {
			params.Page = &page
		}
		if cmd.Flags().Changed("limit") || params.Limit == nil {
			params.Limit = &limit
		}
		value, err := searchProductsCLI(cmd, params)
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	cmd.Flags().StringVar(&file, "file", "", "JSON search filters (or - for stdin), including attribute and multi-select facets")
	cmd.Flags().StringVar(&q, "q", "", "Search query")
	cmd.Flags().StringVar(&profile, "ranking-profile", "", "Named ranking profile")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort: relevance, name, price, or created_at")
	cmd.Flags().StringVar(&order, "order", "", "Sort order: asc or desc")
	cmd.Flags().IntVar(&page, "page", 1, "Result page")
	cmd.Flags().IntVar(&limit, "limit", 20, "Page size")
	return cmd
}
func searchProductsCLI(cmd *cobra.Command, params apicontract.SearchAdminProductsParams) (any, error) {
	query, err := searchProductQuery(params)
	if err != nil {
		return nil, err
	}
	return searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/products?"+query, nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
		return e.SearchAdminProducts(ctx, apicontract.SearchAdminProductsRequestObject{Params: params})
	})
}
func searchProductQuery(params apicontract.SearchAdminProductsParams) (string, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	var fields map[string]any
	if err = json.Unmarshal(data, &fields); err != nil {
		return "", err
	}
	query := url.Values{}
	for key, value := range fields {
		switch value := value.(type) {
		case []any:
			for _, item := range value {
				query.Add(key, fmt.Sprint(item))
			}
		case map[string]any:
			for slug, list := range value {
				for i, item := range list.([]any) {
					query.Set(fmt.Sprintf("attribute[%s][%d]", slug, i), fmt.Sprint(item))
				}
			}
		default:
			query.Set(key, fmt.Sprint(value))
		}
	}
	return query.Encode(), nil
}
func resolveSearchProduct(cmd *cobra.Command, query string) (int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, fmt.Errorf("product lookup cannot be empty")
	}
	page, limit := 1, 100
	value, err := searchProductsCLI(cmd, apicontract.SearchAdminProductsParams{Q: &query, Page: &page, Limit: &limit})
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	var result apicontract.AdminProductSearchResponse
	if err = json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	if result.Pagination.Total == 1 && len(result.Items) == 1 {
		return result.Items[0].Id, nil
	}
	// An exact unique name can disambiguate a broad query, but only if all results were inspected.
	if result.Pagination.Total <= limit {
		matches := []int{}
		for _, item := range result.Items {
			if strings.EqualFold(strings.TrimSpace(item.Name), query) {
				matches = append(matches, item.Id)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	if len(result.Items) == 0 {
		return 0, fmt.Errorf("no indexed published product matches %q; use an explicit product ID", query)
	}
	return 0, fmt.Errorf("product lookup %q is ambiguous (%d matches); refine the query or use an explicit product ID", query, result.Pagination.Total)
}
func newSearchAnalyticsCmd() *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "analytics", Short: "Report consented search quality, popular queries, and trends", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if days < 1 || days > 90 {
			return fmt.Errorf("days must be between 1 and 90")
		}
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/analytics?days="+strconv.Itoa(days), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.GetAdminSearchAnalytics(ctx, apicontract.GetAdminSearchAnalyticsRequestObject{Params: apicontract.GetAdminSearchAnalyticsParams{Days: &days}})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	cmd.Flags().IntVar(&days, "days", 7, "Reporting window in days (1–90)")
	return cmd
}
func newSearchFreshnessCmd() *cobra.Command {
	return &cobra.Command{Use: "freshness", Short: "Inspect index freshness and queue health", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/freshness", nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.GetAdminSearchFreshness(ctx, apicontract.GetAdminSearchFreshnessRequestObject{})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
}
func newSearchOperationsCmd() *cobra.Command {
	return &cobra.Command{Use: "operations", Short: "Inspect search limits, queue, and runtime status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/operations", nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.GetAdminSearchOperations(ctx, apicontract.GetAdminSearchOperationsRequestObject{})
		})
		if err != nil {
			return err
		}
		if !isRemoteMode() {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			var local map[string]any
			if err = json.Unmarshal(data, &local); err != nil {
				return err
			}
			for _, key := range []string{"active_searches", "circuit_state", "consecutive_failures", "circuit_open_until"} {
				local[key] = nil
			}
			local["runtime_counters_available"] = false
			local["runtime_note"] = "Live runtime counters require remote-auth mode."
			value = local
		}
		return writeSearchOutput(cmd, value)
	}}
}
func newSearchIncidentsCmd() *cobra.Command {
	var page, limit int
	var status string
	cmd := &cobra.Command{Use: "incidents", Short: "List retained stale-index incidents and durations", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if page < 1 || limit < 1 || limit > 100 || (status != "" && status != "open" && status != "resolved") {
			return fmt.Errorf("use positive page, limit 1–100, and status open or resolved")
		}
		params := apicontract.GetAdminSearchIncidentsParams{Page: &page, Limit: &limit}
		query := url.Values{"page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}}
		if status != "" {
			s := apicontract.GetAdminSearchIncidentsParamsStatus(status)
			params.Status = &s
			query.Set("status", status)
		}
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/incidents?"+query.Encode(), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.GetAdminSearchIncidents(ctx, apicontract.GetAdminSearchIncidentsRequestObject{Params: params})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	cmd.Flags().IntVar(&page, "page", 1, "Result page")
	cmd.Flags().IntVar(&limit, "limit", 20, "Page size")
	cmd.Flags().StringVar(&status, "status", "", "Filter: open or resolved")
	return cmd
}
func newSearchEvaluateCmd() *cobra.Command {
	return &cobra.Command{Use: "evaluate", Short: "Run the offline relevance corpus and regression gate locally", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validateSearchFormat(cmd); err != nil {
			return err
		}
		report, err := searchservice.EvaluateRelevance(cmd.Context())
		if err != nil {
			return err
		}
		if err = writeSearchOutput(cmd, report); err != nil {
			return err
		}
		return report.CheckRegression()
	}}
}

func configureSearchCommand(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	for _, child := range cmd.Commands() {
		configureSearchCommand(child)
	}
}
