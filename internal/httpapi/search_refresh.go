package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
)

func (e *CatalogEndpoints) CreateAdminSearchSalesRefresh(ctx context.Context, _ apicontract.CreateAdminSearchSalesRefreshRequestObject) (apicontract.CreateAdminSearchSalesRefreshResponseObject, error) {
	job, err := e.search.EnqueueSalesRefresh(ctx)
	if err != nil {
		return nil, searchRefreshEndpointError(err)
	}
	return apicontract.CreateAdminSearchSalesRefresh202JSONResponse{JobId: job.ID, Status: apicontract.SearchRefreshAcceptedStatusQueued}, nil
}

func (e *CatalogEndpoints) CreateAdminSearchConversionRefresh(ctx context.Context, _ apicontract.CreateAdminSearchConversionRefreshRequestObject) (apicontract.CreateAdminSearchConversionRefreshResponseObject, error) {
	job, err := e.search.EnqueueConversionRefresh(ctx)
	if err != nil {
		return nil, searchRefreshEndpointError(err)
	}
	return apicontract.CreateAdminSearchConversionRefresh202JSONResponse{JobId: job.ID, Status: apicontract.SearchRefreshAcceptedStatusQueued}, nil
}

func searchRefreshEndpointError(err error) error {
	if errors.Is(err, jobs.ErrIdempotencyConflict) {
		return problemError(http.StatusConflict, "idempotency_conflict", "The daily search refresh job has a conflicting payload.", err)
	}
	return searchConfigurationEndpointError(err)
}
