package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"
)

func (e *CatalogEndpoints) GetAdminSearchIncidents(ctx context.Context, request apicontract.GetAdminSearchIncidentsRequestObject) (apicontract.GetAdminSearchIncidentsResponseObject, error) {
	page, limit, status := 1, 20, ""
	if request.Params.Page != nil {
		page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if request.Params.Status != nil {
		status = string(*request.Params.Status)
	}
	result, err := e.search.ListIncidents(ctx, page, limit, status)
	if errors.Is(err, searchservice.ErrIncidentQueryInvalid) {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Incident page, limit, or status is invalid.", err)
	}
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	rows := make([]apicontract.SearchIndexIncident, 0, len(result.Items))
	for _, row := range result.Items {
		rows = append(rows, apicontract.SearchIndexIncident{Id: int(row.ID), IndexName: row.IndexName, Reason: apicontract.SearchIndexIncidentReason(row.Reason), Status: apicontract.SearchIndexIncidentStatus(row.Status), OpenedAt: row.OpenedAt, DetectedAt: row.DetectedAt, LastObservedAt: row.LastObservedAt, RecoveredAt: row.RecoveredAt, DurationSeconds: row.DurationSeconds, MaxLagSeconds: row.MaxLagSeconds, MaxPendingJobs: row.MaxPendingJobs, DocumentCount: row.DocumentCount})
	}
	return apicontract.GetAdminSearchIncidents200JSONResponse{Data: rows, Pagination: apicontract.Pagination{Page: result.Page, Limit: result.Limit, Total: int(result.Total), TotalPages: result.TotalPages}}, nil
}
