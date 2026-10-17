package commands

import (
	"context"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"strconv"
)

func newSearchConfigurationHistoryCmd() *cobra.Command {
	var kind string
	var id, page, limit int
	cmd := &cobra.Command{Use: "configuration-history", Short: "Inspect retained synonym, typo, and ranking configuration changes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if page < 1 || page > 1000000 || limit < 1 || limit > 100 || id < 0 || (id > 0 && kind == "") {
			return fmt.Errorf("use a page from 1 to 1000000, limit from 1 to 100, and a kind when selecting an entity")
		}
		if kind != "" && kind != "synonym" && kind != "typo_profile" && kind != "ranking_profile" {
			return fmt.Errorf("kind must be synonym, typo_profile, or ranking_profile")
		}
		q := url.Values{"page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}}
		params := apicontract.ListAdminSearchConfigurationHistoryParams{Page: &page, Limit: &limit}
		if kind != "" {
			value := apicontract.ListAdminSearchConfigurationHistoryParamsKind(kind)
			params.Kind = &value
			q.Set("kind", kind)
		}
		if id > 0 {
			params.EntityId = &id
			q.Set("entity_id", strconv.Itoa(id))
		}
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/configuration-history?"+q.Encode(), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.ListAdminSearchConfigurationHistory(ctx, apicontract.ListAdminSearchConfigurationHistoryRequestObject{Params: params})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	cmd.Flags().StringVar(&kind, "kind", "", "Configuration kind: synonym, typo_profile, ranking_profile")
	cmd.Flags().IntVar(&id, "entity-id", 0, "Configuration ID (requires --kind)")
	cmd.Flags().IntVar(&page, "page", 1, "History page")
	cmd.Flags().IntVar(&limit, "limit", 20, "Entries per page (1–100)")
	return cmd
}
