package commands

import (
	"context"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
)

func catalogDiscountCampaigns(ctx context.Context, status string) (apicontract.DiscountCampaignListResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaignListResponse, error) {
		var filter *apicontract.ListAdminDiscountCampaignsParamsStatus
		if status != "" {
			value := apicontract.ListAdminDiscountCampaignsParamsStatus(status)
			filter = &value
		}
		response, err := e.ListAdminDiscountCampaigns(ctx, apicontract.ListAdminDiscountCampaignsRequestObject{Params: apicontract.ListAdminDiscountCampaignsParams{Status: filter}})
		if err != nil {
			return apicontract.DiscountCampaignListResponse{}, err
		}
		return apicontract.DiscountCampaignListResponse(response.(apicontract.ListAdminDiscountCampaigns200JSONResponse)), nil
	})
}
func catalogCreateDiscount(ctx context.Context, body apicontract.ProductDiscountInput) (apicontract.DiscountCampaign, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaign, error) {
		response, err := e.CreateAdminDiscountCampaign(ctx, apicontract.CreateAdminDiscountCampaignRequestObject{Body: &body})
		if err != nil {
			return apicontract.DiscountCampaign{}, err
		}
		return apicontract.DiscountCampaign(response.(apicontract.CreateAdminDiscountCampaign201JSONResponse)), nil
	})
}
func catalogUpdateDiscount(ctx context.Context, id uint, body apicontract.ProductDiscountInput) (apicontract.DiscountCampaign, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaign, error) {
		response, err := e.UpdateAdminDiscountCampaign(ctx, apicontract.UpdateAdminDiscountCampaignRequestObject{Id: int(id), Body: &body})
		if err != nil {
			return apicontract.DiscountCampaign{}, err
		}
		return apicontract.DiscountCampaign(response.(apicontract.UpdateAdminDiscountCampaign200JSONResponse)), nil
	})
}
func catalogDiscountAction(ctx context.Context, id uint, action string) (apicontract.DiscountCampaign, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaign, error) {
		if action == "disable" {
			response, err := e.DisableAdminDiscountCampaign(ctx, apicontract.DisableAdminDiscountCampaignRequestObject{Id: int(id)})
			if err != nil {
				return apicontract.DiscountCampaign{}, err
			}
			return apicontract.DiscountCampaign(response.(apicontract.DisableAdminDiscountCampaign200JSONResponse)), nil
		}
		response, err := e.ArchiveAdminDiscountCampaign(ctx, apicontract.ArchiveAdminDiscountCampaignRequestObject{Id: int(id)})
		if err != nil {
			return apicontract.DiscountCampaign{}, err
		}
		return apicontract.DiscountCampaign(response.(apicontract.ArchiveAdminDiscountCampaign200JSONResponse)), nil
	})
}
func catalogDiscountSchedule(ctx context.Context, id uint, body apicontract.DiscountScheduleInput) (apicontract.DiscountSchedule, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountSchedule, error) {
		response, err := e.ScheduleAdminDiscountCampaign(ctx, apicontract.ScheduleAdminDiscountCampaignRequestObject{Id: int(id), Body: &body})
		if err != nil {
			return apicontract.DiscountSchedule{}, err
		}
		return apicontract.DiscountSchedule(response.(apicontract.ScheduleAdminDiscountCampaign200JSONResponse)), nil
	})
}
func catalogDiscountLifecycle(ctx context.Context) (apicontract.DiscountLifecycleRunResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountLifecycleRunResponse, error) {
		response, err := e.RunAdminDiscountLifecycle(ctx, apicontract.RunAdminDiscountLifecycleRequestObject{})
		if err != nil {
			return apicontract.DiscountLifecycleRunResponse{}, err
		}
		return apicontract.DiscountLifecycleRunResponse(response.(apicontract.RunAdminDiscountLifecycle200JSONResponse)), nil
	})
}
func catalogDiscountHistory(ctx context.Context, id uint) (apicontract.DiscountStateHistoryListResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountStateHistoryListResponse, error) {
		var filter *int
		if id > 0 {
			value := int(id)
			filter = &value
		}
		response, err := e.ListAdminDiscountHistory(ctx, apicontract.ListAdminDiscountHistoryRequestObject{Params: apicontract.ListAdminDiscountHistoryParams{CampaignId: filter}})
		if err != nil {
			return apicontract.DiscountStateHistoryListResponse{}, err
		}
		return apicontract.DiscountStateHistoryListResponse(response.(apicontract.ListAdminDiscountHistory200JSONResponse)), nil
	})
}
func catalogDiscountAudit(ctx context.Context, id uint) (apicontract.DiscountCampaignAuditListResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaignAuditListResponse, error) {
		var filter *int
		if id > 0 {
			value := int(id)
			filter = &value
		}
		response, err := e.ListAdminDiscountAudit(ctx, apicontract.ListAdminDiscountAuditRequestObject{Params: apicontract.ListAdminDiscountAuditParams{CampaignId: filter}})
		if err != nil {
			return apicontract.DiscountCampaignAuditListResponse{}, err
		}
		return apicontract.DiscountCampaignAuditListResponse(response.(apicontract.ListAdminDiscountAudit200JSONResponse)), nil
	})
}
func catalogDiscountMetrics(ctx context.Context) (apicontract.DiscountEvaluationMetrics, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountEvaluationMetrics, error) {
		response, err := e.GetAdminDiscountMetrics(ctx, apicontract.GetAdminDiscountMetricsRequestObject{})
		if err != nil {
			return apicontract.DiscountEvaluationMetrics{}, err
		}
		return apicontract.DiscountEvaluationMetrics(response.(apicontract.GetAdminDiscountMetrics200JSONResponse)), nil
	})
}
func catalogDiscountReconciliation(ctx context.Context) (apicontract.DiscountReconciliationReport, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountReconciliationReport, error) {
		response, err := e.RunAdminDiscountReconciliation(ctx, apicontract.RunAdminDiscountReconciliationRequestObject{})
		if err != nil {
			return apicontract.DiscountReconciliationReport{}, err
		}
		return apicontract.DiscountReconciliationReport(response.(apicontract.RunAdminDiscountReconciliation200JSONResponse)), nil
	})
}
func catalogCreatePromotion(ctx context.Context, body apicontract.PromotionInput) (apicontract.DiscountCampaign, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaign, error) {
		response, err := e.CreateAdminPromotionCampaign(ctx, apicontract.CreateAdminPromotionCampaignRequestObject{Body: &body})
		if err != nil {
			return apicontract.DiscountCampaign{}, err
		}
		return apicontract.DiscountCampaign(response.(apicontract.CreateAdminPromotionCampaign201JSONResponse)), nil
	})
}
func catalogPreviewPromotion(ctx context.Context, body apicontract.PromotionEvaluationRequest) (apicontract.PromotionEvaluationResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.PromotionEvaluationResponse, error) {
		response, err := e.PreviewAdminPromotion(ctx, apicontract.PreviewAdminPromotionRequestObject{Body: &body})
		if err != nil {
			return apicontract.PromotionEvaluationResponse{}, err
		}
		return apicontract.PromotionEvaluationResponse(response.(apicontract.PreviewAdminPromotion200JSONResponse)), nil
	})
}
func catalogPromotionTemplates(ctx context.Context, active bool) (apicontract.PromotionTemplateListResponse, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.PromotionTemplateListResponse, error) {
		response, err := e.ListAdminPromotionTemplates(ctx, apicontract.ListAdminPromotionTemplatesRequestObject{Params: apicontract.ListAdminPromotionTemplatesParams{Active: &active}})
		if err != nil {
			return apicontract.PromotionTemplateListResponse{}, err
		}
		return apicontract.PromotionTemplateListResponse(response.(apicontract.ListAdminPromotionTemplates200JSONResponse)), nil
	})
}
func catalogCreatePromotionTemplate(ctx context.Context, body apicontract.PromotionTemplateInput) (apicontract.PromotionTemplate, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.PromotionTemplate, error) {
		response, err := e.CreateAdminPromotionTemplate(ctx, apicontract.CreateAdminPromotionTemplateRequestObject{Body: &body})
		if err != nil {
			return apicontract.PromotionTemplate{}, err
		}
		return apicontract.PromotionTemplate(response.(apicontract.CreateAdminPromotionTemplate201JSONResponse)), nil
	})
}
func catalogInstantiatePromotionTemplate(ctx context.Context, id uint, body apicontract.PromotionTemplateInstantiateInput) (apicontract.DiscountCampaign, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.DiscountCampaign, error) {
		response, err := e.InstantiateAdminPromotionTemplate(ctx, apicontract.InstantiateAdminPromotionTemplateRequestObject{Id: int(id), Body: &body})
		if err != nil {
			return apicontract.DiscountCampaign{}, err
		}
		return apicontract.DiscountCampaign(response.(apicontract.InstantiateAdminPromotionTemplate201JSONResponse)), nil
	})
}
