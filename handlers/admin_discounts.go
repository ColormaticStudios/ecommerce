package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"ecommerce/internal/apicontract"
	discountservice "ecommerce/internal/services/discounts"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminProductDiscountRequest struct {
	Name                string         `json:"name"`
	ProductIDs          []uint         `json:"product_ids"`
	DiscountMode        string         `json:"discount_mode"`
	DiscountValue       float64        `json:"discount_value"`
	StartsAt            time.Time      `json:"starts_at"`
	EndsAt              *time.Time     `json:"ends_at"`
	Priority            int            `json:"priority"`
	IsExclusive         bool           `json:"is_exclusive"`
	Status              string         `json:"status"`
	Metadata            map[string]any `json:"metadata"`
	CouponCode          *string        `json:"coupon_code"`
	Channels            []string       `json:"channels"`
	CustomerSegment     string         `json:"customer_segment"`
	GlobalUsageCap      *int           `json:"global_usage_cap"`
	PerCustomerUsageCap *int           `json:"per_customer_usage_cap"`
}

type adminPromotionRequest struct {
	Name                string                       `json:"name"`
	StartsAt            time.Time                    `json:"starts_at"`
	EndsAt              *time.Time                   `json:"ends_at"`
	Priority            int                          `json:"priority"`
	IsExclusive         bool                         `json:"is_exclusive"`
	Status              string                       `json:"status"`
	Metadata            map[string]any               `json:"metadata"`
	CouponCode          *string                      `json:"coupon_code"`
	Channels            []string                     `json:"channels"`
	CustomerSegment     string                       `json:"customer_segment"`
	GlobalUsageCap      *int                         `json:"global_usage_cap"`
	PerCustomerUsageCap *int                         `json:"per_customer_usage_cap"`
	Rules               []adminPromotionRuleRequest  `json:"rules"`
	Levels              []adminPromotionLevelRequest `json:"levels"`
}

type adminPromotionRuleRequest struct {
	Condition               discountservice.RuleCondition `json:"condition"`
	Action                  discountservice.RuleAction    `json:"action"`
	StackPolicy             string                        `json:"stack_policy"`
	MaxApplicationsPerOrder *int                          `json:"max_applications_per_order"`
}

type adminPromotionLevelRequest struct {
	Name                    string                                 `json:"name"`
	Priority                int                                    `json:"priority"`
	Action                  discountservice.RuleAction             `json:"action"`
	StackPolicy             string                                 `json:"stack_policy"`
	MaxApplicationsPerOrder *int                                   `json:"max_applications_per_order"`
	Targets                 []discountservice.PromotionTargetInput `json:"targets"`
}

type adminPromotionPreviewRequest struct {
	Lines           []adminPromotionPreviewLine `json:"lines"`
	CouponCode      string                      `json:"coupon_code"`
	Channel         string                      `json:"channel"`
	CustomerSegment string                      `json:"customer_segment"`
}

type adminPromotionPreviewLine struct {
	ProductID        uint    `json:"product_id"`
	ProductVariantID uint    `json:"product_variant_id"`
	BrandID          *uint   `json:"brand_id"`
	CategoryIDs      []uint  `json:"category_ids"`
	SKU              string  `json:"sku"`
	Quantity         int     `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
}

type adminDiscountScheduleRequest struct {
	ScheduleType string     `json:"schedule_type"`
	Recurrence   string     `json:"recurrence"`
	WindowStart  time.Time  `json:"window_start"`
	WindowEnd    time.Time  `json:"window_end"`
	UntilAt      *time.Time `json:"until_at"`
	Timezone     string     `json:"timezone"`
}

type adminPromotionTemplateRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Template    adminPromotionRequest `json:"template"`
	IsActive    *bool                 `json:"is_active"`
}

type adminPromotionTemplateInstantiateRequest struct {
	Name                *string    `json:"name"`
	StartsAt            *time.Time `json:"starts_at"`
	EndsAt              *time.Time `json:"ends_at"`
	CouponCode          *string    `json:"coupon_code"`
	Channels            []string   `json:"channels"`
	CustomerSegment     *string    `json:"customer_segment"`
	GlobalUsageCap      *int       `json:"global_usage_cap"`
	PerCustomerUsageCap *int       `json:"per_customer_usage_cap"`
}

func ListAdminDiscountCampaigns(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		campaigns, err := discountservice.ListDiscountCampaigns(db, c.Query("status"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list discount campaigns"})
			return
		}
		response := make([]apicontract.DiscountCampaign, 0, len(campaigns))
		for _, campaign := range campaigns {
			response = append(response, discountCampaignContract(campaign))
		}
		c.JSON(http.StatusOK, gin.H{"campaigns": response})
	}
}

func ScheduleAdminDiscountCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := discountCampaignIDParam(c)
		if !ok {
			return
		}
		var req adminDiscountScheduleRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		schedule, err := discountservice.UpsertSchedule(db, id, discountservice.ScheduleInput{
			ScheduleType: req.ScheduleType,
			Recurrence:   req.Recurrence,
			WindowStart:  req.WindowStart,
			WindowEnd:    req.WindowEnd,
			UntilAt:      req.UntilAt,
			Timezone:     req.Timezone,
			Actor:        authenticatedActor(c),
		}, time.Now().UTC())
		if err != nil {
			writeDiscountScheduleError(c, err)
			return
		}
		c.JSON(http.StatusOK, discountScheduleContract(schedule))
	}
}

func ArchiveAdminDiscountCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := discountCampaignIDParam(c)
		if !ok {
			return
		}
		campaign, err := discountservice.ArchiveCampaign(db, id, authenticatedActor(c), time.Now().UTC())
		writeDiscountCampaignMutation(c, campaign, err, http.StatusOK)
	}
}

func RunAdminDiscountLifecycle(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := discountservice.RunLifecycle(db, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run discount lifecycle"})
			return
		}
		c.JSON(http.StatusOK, apicontract.DiscountLifecycleRunResponse{
			Activated:   result.Activated,
			Archived:    result.Archived,
			Deactivated: result.Deactivated,
		})
	}
}

func ListAdminDiscountHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		campaignID, ok := optionalCampaignIDQuery(c)
		if !ok {
			return
		}
		history, err := discountservice.ListHistory(db, campaignID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list discount history"})
			return
		}
		entries := make([]apicontract.DiscountStateHistory, 0, len(history))
		for _, entry := range history {
			entries = append(entries, discountHistoryContract(entry))
		}
		c.JSON(http.StatusOK, apicontract.DiscountStateHistoryListResponse{History: entries})
	}
}

func ListAdminDiscountAudit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		campaignID, ok := optionalCampaignIDQuery(c)
		if !ok {
			return
		}
		audits, err := discountservice.ListCampaignAudits(db, campaignID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list discount audit"})
			return
		}
		entries := make([]apicontract.DiscountCampaignAudit, 0, len(audits))
		for _, entry := range audits {
			entries = append(entries, discountAuditContract(entry))
		}
		c.JSON(http.StatusOK, apicontract.DiscountCampaignAuditListResponse{Audit: entries})
	}
}

func GetAdminDiscountMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics := discountservice.EvaluationMetricsSnapshot()
		var lastEvaluatedAt *time.Time
		if !metrics.LastEvaluatedAt.IsZero() {
			lastEvaluatedAt = &metrics.LastEvaluatedAt
		}
		c.JSON(http.StatusOK, apicontract.DiscountEvaluationMetrics{
			FailedEvaluations:      int64(metrics.FailedEvaluations),
			LastCandidateCampaigns: int64(metrics.LastCandidateCampaigns),
			LastError:              metrics.LastError,
			LastEvaluatedAt:        lastEvaluatedAt,
			LastLatencyMs:          int64(metrics.LastLatencyMillis),
			LastLineCount:          int64(metrics.LastLineCount),
			LastMatchedCampaigns:   int64(metrics.LastMatchedCampaigns),
			MatchedEvaluations:     int64(metrics.MatchedEvaluations),
			TotalEvaluations:       int64(metrics.TotalEvaluations),
			TotalLatencyMs:         int64(metrics.TotalLatencyMillis),
		})
	}
}

func RunAdminDiscountReconciliation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().UTC()
		report, err := discountservice.RunReconciliation(db, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run discount reconciliation"})
			return
		}
		issues := make([]apicontract.DiscountReconciliationIssue, 0, len(report.Issues))
		for _, issue := range report.Issues {
			issues = append(issues, discountReconciliationIssueContract(issue))
		}
		c.JSON(http.StatusOK, apicontract.DiscountReconciliationReport{
			CheckedAt: time.Unix(int64(report.CheckedAt), 0).UTC(),
			Issues:    issues,
		})
	}
}

func CreateAdminPromotionCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminPromotionRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		campaign, err := discountservice.CreatePromotion(db, promotionInputFromRequest(req, authenticatedUserID(c)))
		writeDiscountCampaignMutation(c, campaign, err, http.StatusCreated)
	}
}

func PreviewAdminPromotion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminPromotionPreviewRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		lines := make([]discountservice.CartLine, 0, len(req.Lines))
		for _, line := range req.Lines {
			lines = append(lines, discountservice.CartLine{
				ProductID:        line.ProductID,
				ProductVariantID: line.ProductVariantID,
				BrandID:          line.BrandID,
				CategoryIDs:      line.CategoryIDs,
				SKU:              line.SKU,
				Quantity:         line.Quantity,
				UnitPrice:        models.MoneyFromFloat(line.UnitPrice),
			})
		}
		result, err := discountservice.EvaluateCartWithOptions(db, lines, time.Now().UTC(), discountservice.EvaluationOptions{
			CouponCode:      req.CouponCode,
			Channel:         req.Channel,
			CustomerSegment: req.CustomerSegment,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate promotion preview"})
			return
		}
		c.JSON(http.StatusOK, promotionPreviewContract(result))
	}
}

func ListAdminPromotionTemplates(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		templates, err := discountservice.ListTemplates(db, c.Query("active") != "false")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list promotion templates"})
			return
		}
		response := make([]apicontract.PromotionTemplate, 0, len(templates))
		for _, template := range templates {
			response = append(response, promotionTemplateContract(template))
		}
		c.JSON(http.StatusOK, apicontract.PromotionTemplateListResponse{Templates: response})
	}
}

func CreateAdminPromotionTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminPromotionTemplateRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		template, err := discountservice.CreateTemplate(db, discountservice.TemplateInput{
			Name:        req.Name,
			Description: req.Description,
			Template:    promotionInputFromRequest(req.Template, authenticatedUserID(c)),
			IsActive:    req.IsActive,
		})
		writePromotionTemplateMutation(c, template, err)
	}
}

func InstantiateAdminPromotionTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := discountCampaignIDParam(c)
		if !ok {
			return
		}
		var req adminPromotionTemplateInstantiateRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		campaign, err := discountservice.InstantiateTemplate(db, id, discountservice.InstantiateTemplateInput{
			Name:            req.Name,
			StartsAt:        req.StartsAt,
			EndsAt:          req.EndsAt,
			CouponCode:      req.CouponCode,
			Channels:        req.Channels,
			CustomerSegment: req.CustomerSegment,
			GlobalUsageCap:  req.GlobalUsageCap,
			PerCustomerCap:  req.PerCustomerUsageCap,
			ActorID:         authenticatedUserID(c),
		})
		writeDiscountCampaignMutation(c, campaign, err, http.StatusCreated)
	}
}

func CreateAdminDiscountCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminProductDiscountRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		campaign, err := discountservice.CreateProductDiscount(db, discountInputFromRequest(req, authenticatedUserID(c)))
		writeDiscountCampaignMutation(c, campaign, err, http.StatusCreated)
	}
}

func UpdateAdminDiscountCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := discountCampaignIDParam(c)
		if !ok {
			return
		}
		var req adminProductDiscountRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		campaign, err := discountservice.UpdateProductDiscount(db, id, discountInputFromRequest(req, authenticatedUserID(c)))
		writeDiscountCampaignMutation(c, campaign, err, http.StatusOK)
	}
}

func DisableAdminDiscountCampaign(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := discountCampaignIDParam(c)
		if !ok {
			return
		}
		campaign, err := discountservice.DisableProductDiscount(db, id, authenticatedUserID(c))
		writeDiscountCampaignMutation(c, campaign, err, http.StatusOK)
	}
}

func discountInputFromRequest(req adminProductDiscountRequest, actorID *uint) discountservice.ProductDiscountInput {
	return discountservice.ProductDiscountInput{
		Name:                req.Name,
		ProductIDs:          req.ProductIDs,
		DiscountMode:        req.DiscountMode,
		DiscountValue:       models.MoneyFromFloat(req.DiscountValue),
		StartsAt:            req.StartsAt,
		EndsAt:              req.EndsAt,
		Priority:            req.Priority,
		IsExclusive:         req.IsExclusive,
		Status:              req.Status,
		Metadata:            req.Metadata,
		CouponCode:          req.CouponCode,
		Channels:            req.Channels,
		CustomerSegment:     req.CustomerSegment,
		GlobalUsageCap:      req.GlobalUsageCap,
		PerCustomerUsageCap: req.PerCustomerUsageCap,
		ActorID:             actorID,
	}
}

func promotionInputFromRequest(req adminPromotionRequest, actorID *uint) discountservice.CreatePromotionInput {
	rules := make([]discountservice.PromotionRuleInput, 0, len(req.Rules))
	for _, rule := range req.Rules {
		rules = append(rules, discountservice.PromotionRuleInput{
			Condition:               rule.Condition,
			Action:                  rule.Action,
			StackPolicy:             rule.StackPolicy,
			MaxApplicationsPerOrder: rule.MaxApplicationsPerOrder,
		})
	}
	levels := make([]discountservice.PromotionLevelInput, 0, len(req.Levels))
	for _, level := range req.Levels {
		levels = append(levels, discountservice.PromotionLevelInput{
			Name:                    level.Name,
			Priority:                level.Priority,
			Action:                  level.Action,
			StackPolicy:             level.StackPolicy,
			MaxApplicationsPerOrder: level.MaxApplicationsPerOrder,
			Targets:                 level.Targets,
		})
	}
	return discountservice.CreatePromotionInput{
		Name:                req.Name,
		StartsAt:            req.StartsAt,
		EndsAt:              req.EndsAt,
		Priority:            req.Priority,
		IsExclusive:         req.IsExclusive,
		Status:              req.Status,
		Metadata:            req.Metadata,
		CouponCode:          req.CouponCode,
		Channels:            req.Channels,
		CustomerSegment:     req.CustomerSegment,
		GlobalUsageCap:      req.GlobalUsageCap,
		PerCustomerUsageCap: req.PerCustomerUsageCap,
		Rules:               rules,
		Levels:              levels,
		ActorID:             actorID,
	}
}

func writePromotionTemplateMutation(c *gin.Context, template models.PromotionTemplate, err error) {
	if err == nil {
		c.JSON(http.StatusCreated, promotionTemplateContract(template))
		return
	}
	if errors.Is(err, discountservice.ErrInvalidCampaign) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save promotion template"})
}

func writeDiscountCampaignMutation(c *gin.Context, campaign models.DiscountCampaign, err error, successStatus int) {
	if err == nil {
		c.JSON(successStatus, discountCampaignContract(campaign))
		return
	}
	switch {
	case errors.Is(err, discountservice.ErrInvalidCampaign):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Discount campaign not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save discount campaign"})
	}
}

func writeDiscountScheduleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, discountservice.ErrInvalidCampaign):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Discount campaign not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save discount schedule"})
	}
}

func discountCampaignIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid discount campaign ID"})
		return 0, false
	}
	return uint(id), true
}

func optionalCampaignIDQuery(c *gin.Context) (*uint, bool) {
	raw := c.Query("campaign_id")
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || parsed == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign_id"})
		return nil, false
	}
	value := uint(parsed)
	return &value, true
}

func discountCampaignContract(campaign models.DiscountCampaign) apicontract.DiscountCampaign {
	targets := make([]apicontract.DiscountTarget, 0, len(campaign.Targets))
	for _, target := range campaign.Targets {
		targets = append(targets, apicontract.DiscountTarget{
			Id:         int(target.ID),
			TargetId:   int(target.TargetID),
			TargetType: apicontract.DiscountTargetTargetType(target.TargetType),
		})
	}
	return apicontract.DiscountCampaign{
		Channels:            discountCampaignChannels(campaign.ChannelsJSON),
		CouponCode:          campaign.CouponCode,
		CreatedAt:           campaign.CreatedAt,
		DiscountMode:        apicontract.DiscountCampaignDiscountMode(campaign.DiscountMode),
		DiscountValue:       campaign.DiscountValue.Float64(),
		EndsAt:              campaign.EndsAt,
		Id:                  int(campaign.ID),
		IsExclusive:         campaign.IsExclusive,
		CustomerSegment:     optionalString(campaign.CustomerSegment),
		GlobalUsageCap:      campaign.GlobalUsageCap,
		Metadata:            optionalObject(campaign.MetadataJSON),
		Name:                campaign.Name,
		PerCustomerUsageCap: campaign.PerCustomerUsageCap,
		Priority:            campaign.Priority,
		StartsAt:            campaign.StartsAt,
		Status:              apicontract.DiscountCampaignStatus(campaign.Status),
		Targets:             targets,
		Type:                apicontract.DiscountCampaignType(campaign.Type),
		UpdatedAt:           campaign.UpdatedAt,
	}
}

func promotionTemplateContract(template models.PromotionTemplate) apicontract.PromotionTemplate {
	return apicontract.PromotionTemplate{
		CreatedAt:    template.CreatedAt,
		Description:  template.Description,
		Id:           int(template.ID),
		IsActive:     template.IsActive,
		Name:         template.Name,
		Template:     decodeObject(template.TemplateJSON),
		TemplateJson: template.TemplateJSON,
		UpdatedAt:    template.UpdatedAt,
	}
}

func decodeStringList(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	if values == nil {
		return []string{}
	}
	return values
}

func discountCampaignChannels(raw string) *[]apicontract.DiscountCampaignChannels {
	values := decodeStringList(raw)
	channels := make([]apicontract.DiscountCampaignChannels, 0, len(values))
	for _, value := range values {
		channels = append(channels, apicontract.DiscountCampaignChannels(value))
	}
	return &channels
}

func optionalObject(raw string) *map[string]any {
	values := decodeObject(raw)
	return &values
}

func decodeObject(raw string) map[string]any {
	values := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func discountScheduleContract(schedule models.DiscountSchedule) apicontract.DiscountSchedule {
	return apicontract.DiscountSchedule{
		CampaignId:   int(schedule.CampaignID),
		Id:           int(schedule.ID),
		LastRunAt:    schedule.LastRunAt,
		NextRunAt:    schedule.NextRunAt,
		Recurrence:   optionalString(schedule.RRule),
		ScheduleType: apicontract.DiscountScheduleScheduleType(schedule.ScheduleType),
		Timezone:     schedule.Timezone,
		UntilAt:      schedule.UntilAt,
		WindowEnd:    schedule.WindowEnd,
		WindowStart:  schedule.WindowStart,
	}
}

func discountHistoryContract(entry models.DiscountStateHistory) apicontract.DiscountStateHistory {
	return apicontract.DiscountStateHistory{
		Actor:      entry.Actor,
		CampaignId: int(entry.CampaignID),
		ChangedAt:  entry.ChangedAt,
		FromStatus: entry.FromStatus,
		Id:         int(entry.ID),
		Reason:     entry.Reason,
		Source:     entry.Source,
		ToStatus:   entry.ToStatus,
	}
}

func discountAuditContract(entry models.DiscountCampaignAudit) apicontract.DiscountCampaignAudit {
	return apicontract.DiscountCampaignAudit{
		Actor:      entry.Actor,
		AfterJson:  entry.AfterJSON,
		BeforeJson: entry.BeforeJSON,
		CampaignId: int(entry.CampaignID),
		ChangedAt:  entry.ChangedAt,
		EventType:  entry.EventType,
		Id:         int(entry.ID),
		Source:     entry.Source,
		Summary:    entry.Summary,
	}
}

func discountReconciliationIssueContract(issue discountservice.ReconciliationIssue) apicontract.DiscountReconciliationIssue {
	return apicontract.DiscountReconciliationIssue{
		ActualEnd:      issue.ActualEnd,
		ActualStart:    &issue.ActualStart,
		ActualStatus:   issue.ActualStatus,
		CampaignId:     int(issue.CampaignID),
		ExpectedEnd:    issue.ExpectedEnd,
		ExpectedStart:  &issue.ExpectedStart,
		ExpectedStatus: issue.ExpectedStatus,
		Message:        issue.Message,
		ScheduleId:     int(issue.ScheduleID),
	}
}

func promotionPreviewContract(result discountservice.EvaluationResult) apicontract.PromotionEvaluationResponse {
	lines := make([]apicontract.PromotionEvaluationLine, 0, len(result.Lines))
	for _, line := range result.Lines {
		lines = append(lines, apicontract.PromotionEvaluationLine{
			AppliedCampaigns: appliedCampaignContracts(line.AppliedCampaigns),
			BasePrice:        line.BasePrice.Float64(),
			DiscountAmount:   line.DiscountAmount.Float64(),
			FinalPrice:       line.FinalPrice.Float64(),
			ProductId:        int(line.ProductID),
			ProductVariantId: int(line.ProductVariantID),
			Quantity:         line.Quantity,
		})
	}
	return apicontract.PromotionEvaluationResponse{
		DiscountTotal: result.DiscountTotal.Float64(),
		FinalSubtotal: result.FinalSubtotal.Float64(),
		Lines:         lines,
		Subtotal:      result.Subtotal.Float64(),
	}
}

func authenticatedUserID(c *gin.Context) *uint {
	if value, exists := c.Get("user"); exists {
		if user, ok := value.(models.User); ok {
			return &user.ID
		}
		if user, ok := value.(*models.User); ok && user != nil {
			return &user.ID
		}
	}
	return nil
}

func authenticatedActor(c *gin.Context) string {
	if id := authenticatedUserID(c); id != nil {
		return strconv.FormatUint(uint64(*id), 10)
	}
	return ""
}
