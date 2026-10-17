package commands

import (
	"context"
	"ecommerce/internal/jobs"
	"errors"
	"fmt"
	"net/http"
	"time"

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
			job, err = service.EnqueueOrReuseFullReindex(cmd.Context())
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

		disposition := "reused_completed"
		if job.Status != models.JobStatusSucceeded {
			disposition = "executed"
			waitCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			for {
				executeErr := runtime.Execute(waitCtx, job.ID, "cli-search-"+kind)
				if executeErr != nil && !errors.Is(executeErr, jobs.ErrNoRunnableJob) {
					return fmt.Errorf("execute search job %s: %w", job.ID, executeErr)
				}
				if errors.Is(executeErr, jobs.ErrNoRunnableJob) {
					disposition = "awaited_existing"
				}
				if err = db.WithContext(waitCtx).First(&job, "id = ?", job.ID).Error; err != nil {
					return err
				}
				if job.Status == models.JobStatusSucceeded {
					break
				}
				if job.Status == models.JobStatusDeadLetter {
					return fmt.Errorf("search job %s failed permanently; inspect its dead-letter record", job.ID)
				}
				disposition = "awaited_existing"
				timer := time.NewTimer(100 * time.Millisecond)
				select {
				case <-waitCtx.Done():
					timer.Stop()
					return waitCtx.Err()
				case <-timer.C:
				}
			}
		}
		return writeSearchOutput(cmd, map[string]any{"job_id": job.ID, "status": job.Status, "disposition": disposition})
	}}
}
