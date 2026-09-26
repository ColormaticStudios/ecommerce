package commands

import (
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"

	"github.com/spf13/cobra"
)

func NewSearchCmd() *cobra.Command {
	command := &cobra.Command{Use: "search", Short: "Search index operations"}
	command.AddCommand(newSearchReindexCmd())
	return command
}

func newSearchReindexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Queue and run a full product search reindex",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if isRemoteMode() {
				accepted, err := invokeRemoteJSON[apicontract.SearchReindexAccepted](http.MethodPost, "/api/v1/admin/search/reindex", nil)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Queued search reindex job %s\n", accepted.JobId)
				return nil
			}

			db := getDB()
			defer closeDB(db)
			runtime := newJobRuntime(db, getConfig())
			service := searchservice.NewService(db, nil, runtime)
			if err := service.RegisterJobHandlers(); err != nil {
				return err
			}
			job, err := service.EnqueueFullReindex(cmd.Context())
			if err != nil {
				return err
			}
			if err := runtime.Execute(cmd.Context(), job.ID, "cli-search-reindex"); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilt product search index with job %s\n", job.ID)
			return nil
		},
	}
}
