package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/services/cms"
	"ecommerce/models"
)

func contractPageVariant(variant models.CMSPageVariant) (apicontract.CmsPageVariant, error) {
	payload, err := contractPayload(variant.DraftPayloadJSON)
	if err != nil {
		return apicontract.CmsPageVariant{}, err
	}
	return apicontract.CmsPageVariant{Id: int(variant.ID), PageId: int(variant.PageID), EntryId: int(variant.EntryID), Locale: variant.Locale, Market: variant.Market, Path: variant.Path, Slug: variant.Slug, Title: variant.Title, Payload: payload, Status: apicontract.CmsPageVariantStatus(variant.Status), Revision: int(variant.Revision), SubmittedBy: optionalCMSString(variant.SubmittedBy), ApprovedBy: optionalCMSString(variant.ApprovedBy), PublishedAt: variant.PublishedAt, CreatedAt: variant.CreatedAt, UpdatedAt: variant.UpdatedAt}, nil
}
func pageVariantInput(body *apicontract.CmsPageVariantInput, actor string) (cms.VariantInput, error) {
	if body == nil {
		return cms.VariantInput{}, errors.New("CMS page variant body is required")
	}
	payload, err := cmsPayload(body.Payload)
	if err != nil {
		return cms.VariantInput{}, err
	}
	input := cms.VariantInput{Locale: body.Locale, Path: body.Path, Title: body.Title, Payload: payload, Actor: actor}
	if body.Market != nil {
		input.Market = *body.Market
	}
	if body.Slug != nil {
		input.Slug = *body.Slug
	}
	if body.ChangeSummary != nil {
		input.ChangeSummary = *body.ChangeSummary
	}
	return input, nil
}
func (e *CmsMediaEndpoints) ListAdminCmsPageVariants(ctx context.Context, request apicontract.ListAdminCmsPageVariantsRequestObject) (apicontract.ListAdminCmsPageVariantsResponseObject, error) {
	variants, err := e.pages.ListVariants(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsPageVariant, 0, len(variants))
	for _, variant := range variants {
		value, err := contractPageVariant(variant)
		if err != nil {
			return nil, err
		}
		data = append(data, value)
	}
	return apicontract.ListAdminCmsPageVariants200JSONResponse(data), nil
}
func (e *CmsMediaEndpoints) CreateAdminCmsPageVariant(ctx context.Context, request apicontract.CreateAdminCmsPageVariantRequestObject) (apicontract.CreateAdminCmsPageVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	input, err := pageVariantInput(request.Body, actor)
	if err != nil {
		return nil, err
	}
	variant, err := e.pages.CreateVariant(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractPageVariant(*variant)
	return apicontract.CreateAdminCmsPageVariant201JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsPageVariant(ctx context.Context, request apicontract.UpdateAdminCmsPageVariantRequestObject) (apicontract.UpdateAdminCmsPageVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	input, err := pageVariantInput(request.Body, actor)
	if err != nil {
		return nil, err
	}
	variant, err := e.pages.UpdateVariant(ctx, uint(request.Id), uint(request.VariantId), input)
	if err != nil {
		return nil, err
	}
	value, err := contractPageVariant(*variant)
	return apicontract.UpdateAdminCmsPageVariant200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DeleteAdminCmsPageVariant(ctx context.Context, request apicontract.DeleteAdminCmsPageVariantRequestObject) (apicontract.DeleteAdminCmsPageVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	if err := e.pages.DeleteVariant(ctx, uint(request.Id), uint(request.VariantId), actor); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsPageVariant200JSONResponse{Message: "CMS page variant deleted"}, nil
}
func (e *CmsMediaEndpoints) TransitionAdminCmsPageVariant(ctx context.Context, request apicontract.TransitionAdminCmsPageVariantRequestObject) (apicontract.TransitionAdminCmsPageVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	role, err := e.pages.RoleForSubject(ctx, actor)
	if err != nil {
		return nil, err
	}
	comment := workflowComment(request.Body)
	variant, err := e.pages.TransitionVariantAsRole(ctx, uint(request.Id), uint(request.VariantId), string(request.Action), actor, role, comment)
	if err != nil {
		return nil, err
	}
	value, err := contractPageVariant(*variant)
	return apicontract.TransitionAdminCmsPageVariant200JSONResponse(value), err
}

func contractEntryVariant(variant models.CMSContentVariant) (apicontract.CmsEntryVariant, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(variant.DraftPayloadJSON), &payload); err != nil {
		return apicontract.CmsEntryVariant{}, err
	}
	return apicontract.CmsEntryVariant{Id: int(variant.ID), EntryId: int(variant.EntryID), Locale: variant.Locale, Market: variant.Market, Payload: payload, Status: apicontract.CmsEntryVariantStatus(variant.Status), Revision: int(variant.Revision), SubmittedBy: optionalCMSString(variant.SubmittedBy), ApprovedBy: optionalCMSString(variant.ApprovedBy), PublishedAt: variant.PublishedAt, CreatedAt: variant.CreatedAt, UpdatedAt: variant.UpdatedAt}, nil
}
func entryVariantInput(body *apicontract.CmsEntryVariantInput, actor string) (cms.EntryVariantInput, error) {
	if body == nil {
		return cms.EntryVariantInput{}, errors.New("CMS entry variant body is required")
	}
	payload, err := contractConvert[cms.PagePayload](body.Payload)
	if err != nil {
		return cms.EntryVariantInput{}, err
	}
	input := cms.EntryVariantInput{Locale: body.Locale, Payload: payload, Actor: actor}
	if body.Market != nil {
		input.Market = *body.Market
	}
	if body.ChangeSummary != nil {
		input.ChangeSummary = *body.ChangeSummary
	}
	return input, nil
}
func (e *CmsMediaEndpoints) ListAdminCmsEntryVariants(ctx context.Context, request apicontract.ListAdminCmsEntryVariantsRequestObject) (apicontract.ListAdminCmsEntryVariantsResponseObject, error) {
	variants, err := e.pages.ListEntryVariants(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsEntryVariant, 0, len(variants))
	for _, variant := range variants {
		value, err := contractEntryVariant(variant)
		if err != nil {
			return nil, err
		}
		data = append(data, value)
	}
	return apicontract.ListAdminCmsEntryVariants200JSONResponse(data), nil
}
func (e *CmsMediaEndpoints) CreateAdminCmsEntryVariant(ctx context.Context, request apicontract.CreateAdminCmsEntryVariantRequestObject) (apicontract.CreateAdminCmsEntryVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	input, err := entryVariantInput(request.Body, actor)
	if err != nil {
		return nil, err
	}
	variant, err := e.pages.SaveEntryVariant(ctx, uint(request.Id), 0, input)
	if err != nil {
		return nil, err
	}
	value, err := contractEntryVariant(*variant)
	return apicontract.CreateAdminCmsEntryVariant201JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsEntryVariant(ctx context.Context, request apicontract.UpdateAdminCmsEntryVariantRequestObject) (apicontract.UpdateAdminCmsEntryVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	input, err := entryVariantInput(request.Body, actor)
	if err != nil {
		return nil, err
	}
	variant, err := e.pages.SaveEntryVariant(ctx, uint(request.Id), uint(request.VariantId), input)
	if err != nil {
		return nil, err
	}
	value, err := contractEntryVariant(*variant)
	return apicontract.UpdateAdminCmsEntryVariant200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DeleteAdminCmsEntryVariant(ctx context.Context, request apicontract.DeleteAdminCmsEntryVariantRequestObject) (apicontract.DeleteAdminCmsEntryVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	if err := e.pages.DeleteEntryVariant(ctx, uint(request.Id), uint(request.VariantId), actor); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsEntryVariant200JSONResponse{Message: "CMS entry variant deleted"}, nil
}
func (e *CmsMediaEndpoints) TransitionAdminCmsEntryVariant(ctx context.Context, request apicontract.TransitionAdminCmsEntryVariantRequestObject) (apicontract.TransitionAdminCmsEntryVariantResponseObject, error) {
	actor, _ := cmsActor(ctx)
	role, err := e.pages.RoleForSubject(ctx, actor)
	if err != nil {
		return nil, err
	}
	variant, err := e.pages.TransitionEntryVariant(ctx, uint(request.Id), uint(request.VariantId), string(request.Action), actor, role, workflowComment(request.Body))
	if err != nil {
		return nil, err
	}
	value, err := contractEntryVariant(*variant)
	return apicontract.TransitionAdminCmsEntryVariant200JSONResponse(value), err
}

func contractComment(comment models.CMSChangeComment) apicontract.CmsChangeComment {
	return apicontract.CmsChangeComment{Id: int(comment.ID), EntryId: int(comment.EntryID), VariantId: uintPointerToInt(comment.VariantID), Actor: comment.Actor, Body: comment.Body, ResolvedBy: optionalCMSString(comment.ResolvedBy), ResolvedAt: comment.ResolvedAt, CreatedAt: comment.CreatedAt}
}
func workflowComment(body *apicontract.CmsWorkflowActionInput) string {
	if body == nil || body.Comment == nil {
		return ""
	}
	return *body.Comment
}
func (e *CmsMediaEndpoints) CreateAdminCmsEntryComment(ctx context.Context, request apicontract.CreateAdminCmsEntryCommentRequestObject) (apicontract.CreateAdminCmsEntryCommentResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS comment body is required")
	}
	actor, _ := cmsActor(ctx)
	comment, err := e.pages.CreateComment(ctx, uint(request.Id), actor, request.Body.Body)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminCmsEntryComment201JSONResponse(contractComment(*comment)), nil
}
func (e *CmsMediaEndpoints) ResolveAdminCmsComment(ctx context.Context, request apicontract.ResolveAdminCmsCommentRequestObject) (apicontract.ResolveAdminCmsCommentResponseObject, error) {
	actor, _ := cmsActor(ctx)
	comment, err := e.pages.ResolveComment(ctx, uint(request.Id), actor)
	if err != nil {
		return nil, err
	}
	return apicontract.ResolveAdminCmsComment200JSONResponse(contractComment(*comment)), nil
}
func contractWorkflow(workflow *models.CMSEntryWorkflow, comments []models.CMSChangeComment) apicontract.CmsEntryWorkflow {
	values := make([]apicontract.CmsChangeComment, 0, len(comments))
	for _, comment := range comments {
		values = append(values, contractComment(comment))
	}
	return apicontract.CmsEntryWorkflow{EntryId: int(workflow.EntryID), VersionId: int(workflow.VersionID), Status: apicontract.CmsEntryWorkflowStatus(workflow.Status), SubmittedBy: optionalCMSString(workflow.SubmittedBy), ApprovedBy: optionalCMSString(workflow.ApprovedBy), Comments: values}
}
func (e *CmsMediaEndpoints) GetAdminCmsEntryWorkflow(ctx context.Context, request apicontract.GetAdminCmsEntryWorkflowRequestObject) (apicontract.GetAdminCmsEntryWorkflowResponseObject, error) {
	workflow, comments, err := e.pages.EntryWorkflow(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminCmsEntryWorkflow200JSONResponse(contractWorkflow(workflow, comments)), nil
}
func (e *CmsMediaEndpoints) TransitionAdminCmsEntryWorkflow(ctx context.Context, request apicontract.TransitionAdminCmsEntryWorkflowRequestObject) (apicontract.TransitionAdminCmsEntryWorkflowResponseObject, error) {
	actor, _ := cmsActor(ctx)
	role, err := e.pages.RoleForSubject(ctx, actor)
	if err != nil {
		return nil, err
	}
	workflow, comments, err := e.pages.TransitionEntryWorkflow(ctx, uint(request.Id), string(request.Action), actor, role, workflowComment(request.Body))
	if err != nil {
		return nil, err
	}
	return apicontract.TransitionAdminCmsEntryWorkflow200JSONResponse(contractWorkflow(workflow, comments)), nil
}

func (e *CmsMediaEndpoints) ListAdminCmsAuditEvents(ctx context.Context, request apicontract.ListAdminCmsAuditEventsRequestObject) (apicontract.ListAdminCmsAuditEventsResponseObject, error) {
	entryID, limit := uint(0), 100
	if request.Params.EntryId != nil {
		entryID = uint(*request.Params.EntryId)
	}
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	events, err := e.pages.AuditEvents(ctx, entryID, limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsAuditEvent, 0, len(events))
	for _, event := range events {
		data = append(data, apicontract.CmsAuditEvent{Id: int(event.ID), EntryId: int(event.EntryID), VersionId: uintPointerToInt(event.VersionID), VariantId: uintPointerToInt(event.VariantID), Action: event.Action, Actor: event.Actor, Detail: event.Detail, CreatedAt: event.CreatedAt})
	}
	return apicontract.ListAdminCmsAuditEvents200JSONResponse(data), nil
}

func contractGovernance(value *cms.Governance) apicontract.CmsGovernance {
	roles := make([]apicontract.CmsRoleAssignment, 0, len(value.Roles))
	for _, role := range value.Roles {
		roles = append(roles, apicontract.CmsRoleAssignment{Subject: role.Subject, Role: apicontract.CmsRoleAssignmentRole(role.Role)})
	}
	return apicontract.CmsGovernance{ApprovalRequired: value.ApprovalRequired, InvalidationWebhookUrl: value.InvalidationWebhookURL, Roles: roles}
}
func (e *CmsMediaEndpoints) GetAdminCmsGovernance(ctx context.Context, _ apicontract.GetAdminCmsGovernanceRequestObject) (apicontract.GetAdminCmsGovernanceResponseObject, error) {
	value, err := e.pages.Governance(ctx)
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminCmsGovernance200JSONResponse(contractGovernance(value)), nil
}
func (e *CmsMediaEndpoints) UpdateAdminCmsGovernance(ctx context.Context, request apicontract.UpdateAdminCmsGovernanceRequestObject) (apicontract.UpdateAdminCmsGovernanceResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS governance body is required")
	}
	roles := make([]cms.RoleAssignmentInput, 0, len(request.Body.Roles))
	for _, role := range request.Body.Roles {
		roles = append(roles, cms.RoleAssignmentInput{Subject: role.Subject, Role: string(role.Role)})
	}
	value, err := e.pages.UpdateGovernance(ctx, cms.GovernanceInput{ApprovalRequired: request.Body.ApprovalRequired, InvalidationWebhookURL: request.Body.InvalidationWebhookUrl, Roles: roles})
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminCmsGovernance200JSONResponse(contractGovernance(value)), nil
}
func contractInvalidation(event models.CMSInvalidationEvent) apicontract.CmsInvalidationEvent {
	return apicontract.CmsInvalidationEvent{Id: int(event.ID), EntryId: int(event.EntryID), VariantId: uintPointerToInt(event.VariantID), Reason: event.Reason, Status: apicontract.CmsInvalidationEventStatus(event.Status), Attempts: event.Attempts, LastError: event.LastError, CreatedAt: event.CreatedAt, SentAt: event.SentAt}
}
func (e *CmsMediaEndpoints) GetAdminCmsOperations(ctx context.Context, _ apicontract.GetAdminCmsOperationsRequestObject) (apicontract.GetAdminCmsOperationsResponseObject, error) {
	value, err := e.pages.Operations(ctx)
	if err != nil {
		return nil, err
	}
	invalidations := make([]apicontract.CmsInvalidationEvent, 0, len(value.Invalidations))
	for _, event := range value.Invalidations {
		invalidations = append(invalidations, contractInvalidation(event))
	}
	return apicontract.GetAdminCmsOperations200JSONResponse{PendingSchedules: int(value.PendingSchedules), ActiveExperiments: int(value.ActiveExperiments), Invalidations: invalidations}, nil
}
func (e *CmsMediaEndpoints) RetryAdminCmsInvalidation(ctx context.Context, request apicontract.RetryAdminCmsInvalidationRequestObject) (apicontract.RetryAdminCmsInvalidationResponseObject, error) {
	if _, err := e.pages.RetryInvalidation(ctx, uint(request.Id)); err != nil {
		return nil, err
	}
	return apicontract.RetryAdminCmsInvalidation200JSONResponse{Message: "CMS invalidation queued"}, nil
}

func (e *CmsMediaEndpoints) ExportAdminCmsContent(ctx context.Context, _ apicontract.ExportAdminCmsContentRequestObject) (apicontract.ExportAdminCmsContentResponseObject, error) {
	value, err := e.exportCMS(ctx)
	if err != nil {
		return nil, err
	}
	return apicontract.ExportAdminCmsContent200JSONResponse(value), nil
}
func (e *CmsMediaEndpoints) exportCMS(ctx context.Context) (apicontract.CmsContentExport, error) {
	pages, _, err := e.pages.List(ctx, 10000, 0)
	if err != nil {
		return apicontract.CmsContentExport{}, err
	}
	navigation, _, err := e.navigation.List(ctx, 10000, 0)
	if err != nil {
		return apicontract.CmsContentExport{}, err
	}
	globals, _, err := e.globals.List(ctx, 10000, 0)
	if err != nil {
		return apicontract.CmsContentExport{}, err
	}
	locales, err := e.pages.Locales(ctx)
	if err != nil {
		return apicontract.CmsContentExport{}, err
	}
	var variants []models.CMSPageVariant
	if err := e.db.WithContext(ctx).Order("id ASC").Find(&variants).Error; err != nil {
		return apicontract.CmsContentExport{}, err
	}
	result := apicontract.CmsContentExport{SchemaVersion: 1, ExportedAt: time.Now().UTC(), Pages: []apicontract.CmsPageResponse{}, Navigation: []apicontract.CmsNavigationResponse{}, GlobalRegions: []apicontract.CmsGlobalRegionResponse{}, Locales: []apicontract.CmsLocale{}, Variants: []apicontract.CmsPageVariant{}}
	for i := range pages {
		value, err := contractPageRecord(&pages[i], false)
		if err != nil {
			return result, err
		}
		result.Pages = append(result.Pages, value)
	}
	for i := range navigation {
		value, err := contractNavigation(&navigation[i], false)
		if err != nil {
			return result, err
		}
		result.Navigation = append(result.Navigation, value)
	}
	for i := range globals {
		value, err := contractGlobal(&globals[i], false)
		if err != nil {
			return result, err
		}
		result.GlobalRegions = append(result.GlobalRegions, value)
	}
	for _, locale := range locales {
		result.Locales = append(result.Locales, contractLocale(locale))
	}
	for _, variant := range variants {
		value, err := contractPageVariant(variant)
		if err != nil {
			return result, err
		}
		result.Variants = append(result.Variants, value)
	}
	return result, nil
}
func (e *CmsMediaEndpoints) RestoreAdminCmsContent(ctx context.Context, request apicontract.RestoreAdminCmsContentRequestObject) (apicontract.RestoreAdminCmsContentResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS restore body is required")
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	actor, _ := cmsActor(ctx)
	if err := e.pages.RestoreExport(ctx, raw, actor); err != nil {
		return nil, err
	}
	return apicontract.RestoreAdminCmsContent200JSONResponse{Message: "CMS content restored"}, nil
}
func (e *CmsMediaEndpoints) PreviewAdminCmsRestore(ctx context.Context, request apicontract.PreviewAdminCmsRestoreRequestObject) (apicontract.PreviewAdminCmsRestoreResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS restore body is required")
	}
	raw, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	valid, schema, pages, navigation, globals, variants, warnings, validationErrors := e.pages.PreviewRestore(ctx, raw)
	return apicontract.PreviewAdminCmsRestore200JSONResponse{Valid: valid, SchemaVersion: schema, Pages: pages, Navigation: navigation, GlobalRegions: globals, Variants: variants, Warnings: warnings, Errors: validationErrors}, nil
}
