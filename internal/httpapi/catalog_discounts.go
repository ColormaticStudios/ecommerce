package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ecommerce/internal/apicontract"
	discountservice "ecommerce/internal/services/discounts"
	"ecommerce/models"
)

func (e *CatalogEndpoints) ListAdminDiscountCampaigns(ctx context.Context, request apicontract.ListAdminDiscountCampaignsRequestObject) (apicontract.ListAdminDiscountCampaignsResponseObject, error) {
	status := ""
	if request.Params.Status != nil {
		status = string(*request.Params.Status)
	}
	values, err := e.discounts.ListCampaigns(ctx, status)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.DiscountCampaign, 0, len(values))
	for _, value := range values {
		items = append(items, discountCampaignContract(value))
	}
	return apicontract.ListAdminDiscountCampaigns200JSONResponse{Campaigns: items}, nil
}
func (e *CatalogEndpoints) CreateAdminDiscountCampaign(ctx context.Context, request apicontract.CreateAdminDiscountCampaignRequestObject) (apicontract.CreateAdminDiscountCampaignResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("discount campaign body is required")
	}
	value, err := e.discounts.CreateProductDiscount(ctx, productDiscountInput(*request.Body))
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminDiscountCampaign201JSONResponse(discountCampaignContract(value)), nil
}
func (e *CatalogEndpoints) UpdateAdminDiscountCampaign(ctx context.Context, request apicontract.UpdateAdminDiscountCampaignRequestObject) (apicontract.UpdateAdminDiscountCampaignResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid discount campaign id and body are required")
	}
	value, err := e.discounts.UpdateProductDiscount(ctx, uint(request.Id), productDiscountInput(*request.Body))
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminDiscountCampaign200JSONResponse(discountCampaignContract(value)), nil
}
func (e *CatalogEndpoints) DisableAdminDiscountCampaign(ctx context.Context, request apicontract.DisableAdminDiscountCampaignRequestObject) (apicontract.DisableAdminDiscountCampaignResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("discount campaign id must be positive")
	}
	value, err := e.discounts.DisableProductDiscount(ctx, uint(request.Id), nil)
	if err != nil {
		return nil, err
	}
	return apicontract.DisableAdminDiscountCampaign200JSONResponse(discountCampaignContract(value)), nil
}
func (e *CatalogEndpoints) ArchiveAdminDiscountCampaign(ctx context.Context, request apicontract.ArchiveAdminDiscountCampaignRequestObject) (apicontract.ArchiveAdminDiscountCampaignResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("discount campaign id must be positive")
	}
	value, err := e.discounts.ArchiveCampaign(ctx, uint(request.Id), "", time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return apicontract.ArchiveAdminDiscountCampaign200JSONResponse(discountCampaignContract(value)), nil
}
func (e *CatalogEndpoints) ScheduleAdminDiscountCampaign(ctx context.Context, request apicontract.ScheduleAdminDiscountCampaignRequestObject) (apicontract.ScheduleAdminDiscountCampaignResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid discount campaign id and schedule body are required")
	}
	recurrence, timezone := "", ""
	if request.Body.Recurrence != nil {
		recurrence = string(*request.Body.Recurrence)
	}
	if request.Body.Timezone != nil {
		timezone = *request.Body.Timezone
	}
	value, err := e.discounts.UpsertSchedule(ctx, uint(request.Id), discountservice.ScheduleInput{ScheduleType: string(request.Body.ScheduleType), Recurrence: recurrence, WindowStart: request.Body.WindowStart, WindowEnd: request.Body.WindowEnd, UntilAt: request.Body.UntilAt, Timezone: timezone}, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return apicontract.ScheduleAdminDiscountCampaign200JSONResponse(discountScheduleContract(value)), nil
}
func (e *CatalogEndpoints) RunAdminDiscountLifecycle(ctx context.Context, _ apicontract.RunAdminDiscountLifecycleRequestObject) (apicontract.RunAdminDiscountLifecycleResponseObject, error) {
	value, err := e.discounts.RunLifecycle(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return apicontract.RunAdminDiscountLifecycle200JSONResponse{Activated: value.Activated, Deactivated: value.Deactivated, Archived: value.Archived}, nil
}
func (e *CatalogEndpoints) ListAdminDiscountHistory(ctx context.Context, request apicontract.ListAdminDiscountHistoryRequestObject) (apicontract.ListAdminDiscountHistoryResponseObject, error) {
	id, err := optionalPositiveID(request.Params.CampaignId)
	if err != nil {
		return nil, err
	}
	values, err := e.discounts.ListHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.DiscountStateHistory, 0, len(values))
	for _, value := range values {
		items = append(items, discountHistoryContract(value))
	}
	return apicontract.ListAdminDiscountHistory200JSONResponse{History: items}, nil
}
func (e *CatalogEndpoints) ListAdminDiscountAudit(ctx context.Context, request apicontract.ListAdminDiscountAuditRequestObject) (apicontract.ListAdminDiscountAuditResponseObject, error) {
	id, err := optionalPositiveID(request.Params.CampaignId)
	if err != nil {
		return nil, err
	}
	values, err := e.discounts.ListAudits(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.DiscountCampaignAudit, 0, len(values))
	for _, value := range values {
		items = append(items, discountAuditContract(value))
	}
	return apicontract.ListAdminDiscountAudit200JSONResponse{Audit: items}, nil
}
func (e *CatalogEndpoints) GetAdminDiscountMetrics(_ context.Context, _ apicontract.GetAdminDiscountMetricsRequestObject) (apicontract.GetAdminDiscountMetricsResponseObject, error) {
	value := discountservice.EvaluationMetricsSnapshot()
	var last *time.Time
	if !value.LastEvaluatedAt.IsZero() {
		copied := value.LastEvaluatedAt
		last = &copied
	}
	return apicontract.GetAdminDiscountMetrics200JSONResponse{FailedEvaluations: int64(value.FailedEvaluations), LastCandidateCampaigns: int64(value.LastCandidateCampaigns), LastError: value.LastError, LastEvaluatedAt: last, LastLatencyMs: int64(value.LastLatencyMillis), LastLineCount: int64(value.LastLineCount), LastMatchedCampaigns: int64(value.LastMatchedCampaigns), MatchedEvaluations: int64(value.MatchedEvaluations), TotalEvaluations: int64(value.TotalEvaluations), TotalLatencyMs: int64(value.TotalLatencyMillis)}, nil
}
func (e *CatalogEndpoints) RunAdminDiscountReconciliation(ctx context.Context, _ apicontract.RunAdminDiscountReconciliationRequestObject) (apicontract.RunAdminDiscountReconciliationResponseObject, error) {
	value, err := e.discounts.Reconcile(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	issues := make([]apicontract.DiscountReconciliationIssue, 0, len(value.Issues))
	for _, issue := range value.Issues {
		issues = append(issues, discountReconciliationIssueContract(issue))
	}
	return apicontract.RunAdminDiscountReconciliation200JSONResponse{CheckedAt: time.Unix(int64(value.CheckedAt), 0).UTC(), Issues: issues}, nil
}
func (e *CatalogEndpoints) CreateAdminPromotionCampaign(ctx context.Context, request apicontract.CreateAdminPromotionCampaignRequestObject) (apicontract.CreateAdminPromotionCampaignResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("promotion body is required")
	}
	value, err := e.discounts.CreatePromotion(ctx, promotionInput(*request.Body))
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminPromotionCampaign201JSONResponse(discountCampaignContract(value)), nil
}
func (e *CatalogEndpoints) PreviewAdminPromotion(ctx context.Context, request apicontract.PreviewAdminPromotionRequestObject) (apicontract.PreviewAdminPromotionResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("promotion preview body is required")
	}
	lines := make([]discountservice.CartLine, 0, len(request.Body.Lines))
	for _, line := range request.Body.Lines {
		var brand *uint
		if line.BrandId != nil {
			value := uint(*line.BrandId)
			brand = &value
		}
		categories := []uint{}
		if line.CategoryIds != nil {
			categories = uints(*line.CategoryIds)
		}
		sku := ""
		if line.Sku != nil {
			sku = *line.Sku
		}
		lines = append(lines, discountservice.CartLine{ProductID: uint(line.ProductId), ProductVariantID: uint(line.ProductVariantId), BrandID: brand, CategoryIDs: categories, SKU: sku, Quantity: line.Quantity, UnitPrice: models.MoneyFromFloat(line.UnitPrice)})
	}
	options := discountservice.EvaluationOptions{}
	if request.Body.CouponCode != nil {
		options.CouponCode = *request.Body.CouponCode
	}
	if request.Body.Channel != nil {
		options.Channel = string(*request.Body.Channel)
	}
	if request.Body.CustomerSegment != nil {
		options.CustomerSegment = *request.Body.CustomerSegment
	}
	value, err := e.discounts.Preview(ctx, lines, time.Now().UTC(), options)
	if err != nil {
		return nil, err
	}
	return apicontract.PreviewAdminPromotion200JSONResponse(promotionPreviewContract(value)), nil
}
func (e *CatalogEndpoints) ListAdminPromotionTemplates(ctx context.Context, request apicontract.ListAdminPromotionTemplatesRequestObject) (apicontract.ListAdminPromotionTemplatesResponseObject, error) {
	active := true
	if request.Params.Active != nil {
		active = *request.Params.Active
	}
	values, err := e.discounts.ListTemplates(ctx, active)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.PromotionTemplate, 0, len(values))
	for _, value := range values {
		items = append(items, promotionTemplateContract(value))
	}
	return apicontract.ListAdminPromotionTemplates200JSONResponse{Templates: items}, nil
}
func (e *CatalogEndpoints) CreateAdminPromotionTemplate(ctx context.Context, request apicontract.CreateAdminPromotionTemplateRequestObject) (apicontract.CreateAdminPromotionTemplateResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("promotion template body is required")
	}
	value, err := e.discounts.CreateTemplate(ctx, discountservice.TemplateInput{Name: request.Body.Name, Description: derefString(request.Body.Description), Template: promotionInput(request.Body.Template), IsActive: request.Body.IsActive})
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminPromotionTemplate201JSONResponse(promotionTemplateContract(value)), nil
}
func (e *CatalogEndpoints) InstantiateAdminPromotionTemplate(ctx context.Context, request apicontract.InstantiateAdminPromotionTemplateRequestObject) (apicontract.InstantiateAdminPromotionTemplateResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid promotion template id and body are required")
	}
	channels := []string{}
	if request.Body.Channels != nil {
		for _, value := range *request.Body.Channels {
			channels = append(channels, string(value))
		}
	}
	value, err := e.discounts.InstantiateTemplate(ctx, uint(request.Id), discountservice.InstantiateTemplateInput{Name: request.Body.Name, StartsAt: request.Body.StartsAt, EndsAt: request.Body.EndsAt, CouponCode: request.Body.CouponCode, Channels: channels, CustomerSegment: request.Body.CustomerSegment, GlobalUsageCap: request.Body.GlobalUsageCap, PerCustomerCap: request.Body.PerCustomerUsageCap})
	if err != nil {
		return nil, err
	}
	return apicontract.InstantiateAdminPromotionTemplate201JSONResponse(discountCampaignContract(value)), nil
}

func optionalPositiveID(value *int) (*uint, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 1 {
		return nil, errors.New("id must be positive")
	}
	converted := uint(*value)
	return &converted, nil
}
func productDiscountInput(value apicontract.ProductDiscountInput) discountservice.ProductDiscountInput {
	result := discountservice.ProductDiscountInput{Name: value.Name, ProductIDs: uints(value.ProductIds), DiscountMode: string(value.DiscountMode), DiscountValue: models.MoneyFromFloat(value.DiscountValue), StartsAt: value.StartsAt, EndsAt: value.EndsAt, CouponCode: value.CouponCode, GlobalUsageCap: value.GlobalUsageCap, PerCustomerUsageCap: value.PerCustomerUsageCap}
	if value.Priority != nil {
		result.Priority = *value.Priority
	}
	if value.IsExclusive != nil {
		result.IsExclusive = *value.IsExclusive
	}
	if value.Status != nil {
		result.Status = string(*value.Status)
	}
	if value.Metadata != nil {
		result.Metadata = *value.Metadata
	}
	if value.CustomerSegment != nil {
		result.CustomerSegment = *value.CustomerSegment
	}
	if value.Channels != nil {
		for _, item := range *value.Channels {
			result.Channels = append(result.Channels, string(item))
		}
	}
	return result
}
func promotionInput(value apicontract.PromotionInput) discountservice.CreatePromotionInput {
	result := discountservice.CreatePromotionInput{Name: value.Name, StartsAt: value.StartsAt, EndsAt: value.EndsAt, CouponCode: value.CouponCode, GlobalUsageCap: value.GlobalUsageCap, PerCustomerUsageCap: value.PerCustomerUsageCap}
	if value.Priority != nil {
		result.Priority = *value.Priority
	}
	if value.IsExclusive != nil {
		result.IsExclusive = *value.IsExclusive
	}
	if value.Status != nil {
		result.Status = string(*value.Status)
	}
	if value.Metadata != nil {
		result.Metadata = *value.Metadata
	}
	if value.CustomerSegment != nil {
		result.CustomerSegment = *value.CustomerSegment
	}
	if value.Channels != nil {
		for _, item := range *value.Channels {
			result.Channels = append(result.Channels, string(item))
		}
	}
	if value.Rules != nil {
		for _, item := range *value.Rules {
			stack := ""
			if item.StackPolicy != nil {
				stack = string(*item.StackPolicy)
			}
			result.Rules = append(result.Rules, discountservice.PromotionRuleInput{Condition: promotionCondition(item.Condition), Action: promotionAction(item.Action), StackPolicy: stack, MaxApplicationsPerOrder: item.MaxApplicationsPerOrder})
		}
	}
	if value.Levels != nil {
		for _, item := range *value.Levels {
			stack := ""
			if item.StackPolicy != nil {
				stack = string(*item.StackPolicy)
			}
			priority := 0
			if item.Priority != nil {
				priority = *item.Priority
			}
			targets := []discountservice.PromotionTargetInput{}
			for _, target := range item.Targets {
				targets = append(targets, discountservice.PromotionTargetInput{TargetType: string(target.TargetType), TargetID: uint(target.TargetId)})
			}
			result.Levels = append(result.Levels, discountservice.PromotionLevelInput{Name: item.Name, Priority: priority, Action: promotionAction(item.Action), StackPolicy: stack, MaxApplicationsPerOrder: item.MaxApplicationsPerOrder, Targets: targets})
		}
	}
	return result
}
func promotionCondition(value apicontract.PromotionCondition) discountservice.RuleCondition {
	result := discountservice.RuleCondition{}
	if value.ProductIds != nil {
		result.ProductIDs = uints(*value.ProductIds)
	}
	if value.ProductVariantIds != nil {
		result.ProductVariantIDs = uints(*value.ProductVariantIds)
	}
	if value.CategoryIds != nil {
		result.CategoryIDs = uints(*value.CategoryIds)
	}
	if value.BrandIds != nil {
		result.BrandIDs = uints(*value.BrandIds)
	}
	if value.MinQuantity != nil {
		result.MinQuantity = *value.MinQuantity
	}
	if value.MinSubtotal != nil {
		result.MinSubtotal = models.MoneyFromFloat(*value.MinSubtotal)
	}
	return result
}
func promotionAction(value apicontract.PromotionAction) discountservice.RuleAction {
	result := discountservice.RuleAction{Mode: string(value.Mode)}
	if value.Value != nil {
		result.Value = models.MoneyFromFloat(*value.Value)
	}
	if value.TargetType != nil {
		result.TargetType = string(*value.TargetType)
	}
	if value.TargetIds != nil {
		result.TargetIDs = uints(*value.TargetIds)
	}
	if value.ProductIds != nil {
		result.ProductIDs = uints(*value.ProductIds)
	}
	if value.ProductVariantIds != nil {
		result.ProductVariantIDs = uints(*value.ProductVariantIds)
	}
	if value.CategoryIds != nil {
		result.CategoryIDs = uints(*value.CategoryIds)
	}
	if value.BrandIds != nil {
		result.BrandIDs = uints(*value.BrandIds)
	}
	if value.Sku != nil {
		result.SKU = *value.Sku
	}
	return result
}
func uints(values []int) []uint {
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value > 0 {
			result = append(result, uint(value))
		}
	}
	return result
}

func discountCampaignContract(value models.DiscountCampaign) apicontract.DiscountCampaign {
	targets := make([]apicontract.DiscountTarget, 0, len(value.Targets))
	for _, target := range value.Targets {
		targets = append(targets, apicontract.DiscountTarget{Id: int(target.ID), TargetId: int(target.TargetID), TargetType: apicontract.DiscountTargetTargetType(target.TargetType)})
	}
	channelsRaw := []string{}
	_ = json.Unmarshal([]byte(value.ChannelsJSON), &channelsRaw)
	channels := make([]apicontract.DiscountCampaignChannels, 0, len(channelsRaw))
	for _, item := range channelsRaw {
		channels = append(channels, apicontract.DiscountCampaignChannels(item))
	}
	metadata := map[string]any{}
	_ = json.Unmarshal([]byte(value.MetadataJSON), &metadata)
	return apicontract.DiscountCampaign{Id: int(value.ID), Name: value.Name, Type: apicontract.DiscountCampaignType(value.Type), Status: apicontract.DiscountCampaignStatus(value.Status), StartsAt: value.StartsAt, EndsAt: value.EndsAt, Priority: value.Priority, IsExclusive: value.IsExclusive, DiscountMode: apicontract.DiscountCampaignDiscountMode(value.DiscountMode), DiscountValue: value.DiscountValue.Float64(), Targets: targets, Metadata: &metadata, CouponCode: value.CouponCode, Channels: &channels, CustomerSegment: optionalString(value.CustomerSegment), GlobalUsageCap: value.GlobalUsageCap, PerCustomerUsageCap: value.PerCustomerUsageCap, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func discountScheduleContract(value models.DiscountSchedule) apicontract.DiscountSchedule {
	return apicontract.DiscountSchedule{Id: int(value.ID), CampaignId: int(value.CampaignID), ScheduleType: apicontract.DiscountScheduleScheduleType(value.ScheduleType), Recurrence: optionalString(value.RRule), WindowStart: value.WindowStart, WindowEnd: value.WindowEnd, UntilAt: value.UntilAt, Timezone: value.Timezone, LastRunAt: value.LastRunAt, NextRunAt: value.NextRunAt}
}
func discountHistoryContract(value models.DiscountStateHistory) apicontract.DiscountStateHistory {
	return apicontract.DiscountStateHistory{Id: int(value.ID), CampaignId: int(value.CampaignID), FromStatus: value.FromStatus, ToStatus: value.ToStatus, Source: value.Source, Actor: value.Actor, Reason: value.Reason, ChangedAt: value.ChangedAt}
}
func discountAuditContract(value models.DiscountCampaignAudit) apicontract.DiscountCampaignAudit {
	return apicontract.DiscountCampaignAudit{Id: int(value.ID), CampaignId: int(value.CampaignID), EventType: value.EventType, Source: value.Source, Actor: value.Actor, Summary: value.Summary, BeforeJson: value.BeforeJSON, AfterJson: value.AfterJSON, ChangedAt: value.ChangedAt}
}
func discountReconciliationIssueContract(value discountservice.ReconciliationIssue) apicontract.DiscountReconciliationIssue {
	return apicontract.DiscountReconciliationIssue{CampaignId: int(value.CampaignID), ScheduleId: int(value.ScheduleID), ExpectedStatus: value.ExpectedStatus, ActualStatus: value.ActualStatus, ExpectedStart: &value.ExpectedStart, ActualStart: &value.ActualStart, ExpectedEnd: value.ExpectedEnd, ActualEnd: value.ActualEnd, Message: value.Message}
}
func promotionTemplateContract(value models.PromotionTemplate) apicontract.PromotionTemplate {
	template := map[string]any{}
	_ = json.Unmarshal([]byte(value.TemplateJSON), &template)
	return apicontract.PromotionTemplate{Id: int(value.ID), Name: value.Name, Description: value.Description, Template: template, TemplateJson: value.TemplateJSON, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func promotionPreviewContract(value discountservice.EvaluationResult) apicontract.PromotionEvaluationResponse {
	lines := make([]apicontract.PromotionEvaluationLine, 0, len(value.Lines))
	for _, line := range value.Lines {
		applied := make([]apicontract.AppliedCampaign, 0, len(line.AppliedCampaigns))
		for _, campaign := range line.AppliedCampaigns {
			applied = append(applied, apicontract.AppliedCampaign{Id: int(campaign.ID), LevelId: optionalUint(campaign.LevelID), Name: campaign.Name, DiscountAmount: campaign.DiscountAmount.Float64()})
		}
		lines = append(lines, apicontract.PromotionEvaluationLine{ProductId: int(line.ProductID), ProductVariantId: int(line.ProductVariantID), Quantity: line.Quantity, BasePrice: line.BasePrice.Float64(), DiscountAmount: line.DiscountAmount.Float64(), FinalPrice: line.FinalPrice.Float64(), AppliedCampaigns: applied})
	}
	return apicontract.PromotionEvaluationResponse{Lines: lines, Subtotal: value.Subtotal.Float64(), DiscountTotal: value.DiscountTotal.Float64(), FinalSubtotal: value.FinalSubtotal.Float64()}
}
