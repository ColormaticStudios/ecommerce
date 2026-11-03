package httpapi

import (
	"context"
	"ecommerce/internal/apicontract"
	"encoding/json"
	"net/http"
)

func (e *CatalogEndpoints) ListAdminSearchConfigurationHistory(ctx context.Context, request apicontract.ListAdminSearchConfigurationHistoryRequestObject) (apicontract.ListAdminSearchConfigurationHistoryResponseObject, error) {
	p := request.Params
	page, limit := 1, 20
	if p.Page != nil {
		page = *p.Page
	}
	if p.Limit != nil {
		limit = *p.Limit
	}
	kind := ""
	if p.Kind != nil {
		kind = string(*p.Kind)
	}
	id := uint(0)
	if p.EntityId != nil {
		if *p.EntityId < 1 {
			return nil, problemError(http.StatusBadRequest, "invalid_request", "Configuration ID must be positive.", nil)
		}
		id = uint(*p.EntityId)
	}
	if page < 1 || page > 1000000 || limit < 1 || limit > 100 || (id > 0 && kind == "") || (kind != "" && kind != "synonym" && kind != "typo_profile" && kind != "ranking_profile") {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Use a positive page, limit from 1 to 100, and a kind when selecting an entity.", nil)
	}
	rows, total, err := e.search.ListConfigurationHistory(ctx, kind, id, page, limit)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	items := make([]apicontract.SearchConfigurationAudit, 0, len(rows))
	for _, row := range rows {
		item := apicontract.SearchConfigurationAudit{Id: int(row.ID), Kind: apicontract.SearchConfigurationAuditKind(row.Kind), EntityId: int(row.EntityID), Operation: apicontract.SearchConfigurationAuditOperation(row.Operation), ActorId: searchConfigurationUpdatedBy(row.ActorID), CreatedAt: row.CreatedAt}
		for _, field := range []struct {
			raw    *string
			target **map[string]interface{}
		}{{row.BeforeJSON, &item.Before}, {row.AfterJSON, &item.After}} {
			if field.raw == nil {
				continue
			}
			snapshot := map[string]interface{}{}
			if err := json.Unmarshal([]byte(*field.raw), &snapshot); err != nil {
				return nil, err
			}
			*field.target = &snapshot
		}
		items = append(items, item)
	}
	return apicontract.ListAdminSearchConfigurationHistory200JSONResponse{Items: items, Pagination: pagination(page, limit, total)}, nil
}
