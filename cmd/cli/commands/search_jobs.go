package commands

import (
	"context"
	"fmt"
	"net/http"

	"ecommerce/internal/httpapi"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"
	"github.com/spf13/cobra"
)

func newSearchJobCmd(kind string) *cobra.Command {
	return &cobra.Command{Use: kind, Short: "Run " + kind + " locally or queue it on the remote API", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validateSearchFormat(cmd); err != nil {
			return err
		}
		if isRemoteMode() {
			value, err := searchCLIRequest(cmd, http.MethodPost, "/api/v1/admin/search/"+kind, nil, func(context.Context, *httpapi.CatalogEndpoints) (any, error) {
				return nil, fmt.Errorf("remote command required")
			})
			if err != nil {
				return err
			}
			return writeSearchOutput(cmd, value)
		}
		db, cfg, close, err := searchCLIOpenDatabase(cmd.Context())
		if err != nil {
			return err
		}
		defer close()
		runtime := newJobRuntime(db, cfg)
		service := searchservice.NewService(db, nil, runtime)
		if err = service.ConfigureHardening(searchservice.HardeningConfig{MaxConcurrent: cfg.SearchMaxConcurrent, SearchTimeoutMS: cfg.SearchTimeoutMS, CircuitFailureThreshold: cfg.SearchCircuitFailureThreshold, CircuitOpenMS: cfg.SearchCircuitOpenMS, ReindexQueueLimit: cfg.SearchReindexQueueLimit}); err != nil {
			return err
		}
		if err = service.RegisterJobHandlers(); err != nil {
			return err
		}
		var job models.JobQueue
		switch kind {
		case "reindex":
			job, err = service.EnqueueFullReindex(cmd.Context())
		case "refresh-sales":
			job, err = service.EnqueueSalesRefresh(cmd.Context())
		case "refresh-conversion":
			job, err = service.EnqueueConversionRefresh(cmd.Context())
		default:
			return fmt.Errorf("unknown search job %q", kind)
		}
		if err != nil {
			return err
		}
		if job.Status != models.JobStatusSucceeded {
			if err = runtime.Execute(cmd.Context(), job.ID, "cli-search-"+kind); err != nil {
				return fmt.Errorf("execute search job %s: %w", job.ID, err)
			}
		}
		return writeSearchOutput(cmd, map[string]any{"job_id": job.ID, "status": "succeeded"})
	}}
}
