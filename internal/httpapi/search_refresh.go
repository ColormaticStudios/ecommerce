package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	"ecommerce/models"
)

func (e *CatalogEndpoints) CreateAdminSearchSalesRefresh(ctx context.Context, _ apicontract.CreateAdminSearchSalesRefreshRequestObject) (apicontract.CreateAdminSearchSalesRefreshResponseObject, error) {
	job, err := e.search.EnqueueSalesRefresh(ctx)
	if err != nil {
		return nil, searchRefreshEndpointError(err)
	}
	return apicontract.CreateAdminSearchSalesRefresh202JSONResponse(searchRefreshAccepted(job)), nil
}

func (e *CatalogEndpoints) CreateAdminSearchConversionRefresh(ctx context.Context, _ apicontract.CreateAdminSearchConversionRefreshRequestObject) (apicontract.CreateAdminSearchConversionRefreshResponseObject, error) {
	job, err := e.search.EnqueueConversionRefresh(ctx)
	if err != nil {
		return nil, searchRefreshEndpointError(err)
	}
	return apicontract.CreateAdminSearchConversionRefresh202JSONResponse(searchRefreshAccepted(job)), nil
}

func searchRefreshAccepted(job models.JobQueue) apicontract.SearchRefreshAccepted {
	status := apicontract.SearchRefreshAcceptedStatusQueued
	disposition := apicontract.SearchRefreshAcceptedDispositionQueued
	if job.Status == models.JobStatusSucceeded {
		status = apicontract.SearchRefreshAcceptedStatusSucceeded
		disposition = apicontract.SearchRefreshAcceptedDispositionReusedCompleted
	}
	return apicontract.SearchRefreshAccepted{JobId: job.ID, Status: status, Disposition: &disposition}
}

func searchRefreshEndpointError(err error) error {
	if errors.Is(err, jobs.ErrIdempotencyConflict) {
		return problemError(http.StatusConflict, "idempotency_conflict", "The daily search refresh job has a conflicting payload.", err)
	}
	return searchConfigurationEndpointError(err)
}
