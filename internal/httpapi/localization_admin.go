package httpapi

import (
	"context"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func localizationActor(ctx context.Context) (*uint, string) {
	principal, ok := requestctx.PrincipalFrom(ctx)
	if !ok {
		return nil, ""
	}
	if principal.AccountID == 0 {
		return nil, principal.Subject
	}
	return &principal.AccountID, principal.Subject
}

func contractTranslationKey(value models.TranslationKey) apicontract.TranslationKey {
	return apicontract.TranslationKey{Id: int(value.ID), Namespace: value.Namespace, Key: value.Key, SourceText: value.SourceText, Description: value.Description, OwnerDomain: value.OwnerDomain, IsDeprecated: value.IsDeprecated, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func contractValidationIssues(values []localizationservice.ValidationIssue) []apicontract.ValidationIssue {
	result := make([]apicontract.ValidationIssue, 0, len(values))
	for _, value := range values {
		result = append(result, apicontract.ValidationIssue{Path: value.Path, Code: value.Code, Detail: value.Detail})
	}
	return result
}

func contractTranslationValue(value models.TranslationValue, issues []localizationservice.ValidationIssue) apicontract.TranslationValue {
	locale := value.Locale.Code
	return apicontract.TranslationValue{Id: int(value.ID), TranslationKeyId: int(value.TranslationKeyID), Locale: locale, Value: value.Value, State: apicontract.TranslationValueState(value.State), Version: int(value.Version), UpdatedBy: optionalUint(value.UpdatedBy), ReviewedBy: optionalUint(value.ReviewedBy), AssigneeId: optionalUint(value.AssigneeID), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, ValidationIssues: contractValidationIssues(issues)}
}

func contractTranslationRelease(value models.TranslationRelease) apicontract.TranslationRelease {
	return apicontract.TranslationRelease{Id: int(value.ID), Name: value.Name, Status: apicontract.TranslationReleaseStatus(value.Status), Notes: value.Notes, SnapshotHash: value.SnapshotHash, PublishedAt: value.PublishedAt, PublishedBy: optionalUint(value.PublishedBy), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func contractTranslationComment(value models.TranslationComment) apicontract.TranslationComment {
	return apicontract.TranslationComment{Id: int(value.ID), TranslationValueId: int(value.TranslationValueID), AuthorId: optionalUint(value.AuthorID), AuthorName: value.AuthorName, Comment: value.Comment, ResolvedAt: value.ResolvedAt, CreatedAt: value.CreatedAt}
}

func contractTranslationKeyUsages(values []localizationservice.UsageRecord) apicontract.TranslationKeyUsageListResponse {
	result := make([]apicontract.TranslationKeyUsage, 0, len(values))
	for _, value := range values {
		result = append(result, apicontract.TranslationKeyUsage{
			Id: int(value.ID), TranslationKeyId: int(value.TranslationKeyID), Route: value.Route,
			Component: value.Component, Description: value.Description, Position: value.Position,
			ScreenshotMediaId: optionalCMSString(value.ScreenshotMediaID), ScreenshotUrl: optionalCMSString(value.ScreenshotURL), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return apicontract.TranslationKeyUsageListResponse{Usages: result}
}

func (e *LocalizationEndpoints) ListAdminLocalizationKeyUsages(ctx context.Context, request apicontract.ListAdminLocalizationKeyUsagesRequestObject) (apicontract.ListAdminLocalizationKeyUsagesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListUsages(ctx, uint(request.Id), e.media)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ListAdminLocalizationKeyUsages200JSONResponse(contractTranslationKeyUsages(values)), nil
}

func (e *LocalizationEndpoints) ReplaceAdminLocalizationKeyUsages(ctx context.Context, request apicontract.ReplaceAdminLocalizationKeyUsagesRequestObject) (apicontract.ReplaceAdminLocalizationKeyUsagesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	inputs := make([]localizationservice.UsageInput, 0, len(request.Body.Usages))
	for _, value := range request.Body.Usages {
		inputs = append(inputs, localizationservice.UsageInput{Route: value.Route, Component: value.Component, Description: value.Description, Position: value.Position, ScreenshotMediaID: stringPointerValue(value.ScreenshotMediaId)})
	}
	values, err := e.localization.ReplaceUsages(ctx, uint(request.Id), inputs, e.media)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ReplaceAdminLocalizationKeyUsages200JSONResponse(contractTranslationKeyUsages(values)), nil
}

func contractEntityLocalization(value localizationservice.EntityLocalizationRecord) apicontract.EntityLocalizationResponse {
	localizations := make([]apicontract.LocalizedFieldSet, 0, len(value.Localizations))
	for _, item := range value.Localizations {
		localizations = append(localizations, apicontract.LocalizedFieldSet{Locale: item.Locale, Fields: item.Fields})
	}
	return apicontract.EntityLocalizationResponse{EntityType: apicontract.LocalizedEntityType(value.EntityType), EntityId: int(value.EntityID), Localizations: localizations, Resolved: contractEntityResolution(value.Resolved)}
}

func contractEntityResolution(value localizationservice.EntityResolution) apicontract.EntityLocalizationResolution {
	return apicontract.EntityLocalizationResolution{RequestedLocale: value.RequestedLocale, ResolvedLocale: value.ResolvedLocale, FallbackChain: value.FallbackChain, Fields: value.Fields, SourceLocales: value.SourceLocales, UsedFallback: value.UsedFallback}
}

func contractLocalizationRollouts(values []localizationservice.RolloutRecord) apicontract.LocalizationRolloutListResponse {
	result := make([]apicontract.LocalizationRollout, 0, len(values))
	for _, value := range values {
		result = append(result, apicontract.LocalizationRollout{
			Locale: value.Locale, Domain: apicontract.LocalizationRolloutDomain(value.Domain),
			IsEnabled: value.IsEnabled, Percentage: value.Percentage, UpdatedAt: value.UpdatedAt,
		})
	}
	return apicontract.LocalizationRolloutListResponse{Rollouts: result}
}

func contractLocalizationMetrics(value localizationservice.Metrics) apicontract.LocalizationMetricsResponse {
	hotspots := make([]apicontract.LocalizationMetricHotspot, 0, len(value.Hotspots))
	for _, hotspot := range value.Hotspots {
		hotspots = append(hotspots, apicontract.LocalizationMetricHotspot{
			MetricType: apicontract.LocalizationMetricType(hotspot.MetricType), Locale: hotspot.Locale,
			Domain: hotspot.Domain, Key: hotspot.Key, Count: int(hotspot.Count),
			AverageValue: float32(hotspot.AverageValue), MaximumValue: int(hotspot.MaximumValue), LastSeenAt: hotspot.LastSeenAt,
		})
	}
	localeRates := make([]apicontract.LocalizationLocaleMetricRate, 0, len(value.LocaleRates))
	for _, rate := range value.LocaleRates {
		localeRates = append(localeRates, apicontract.LocalizationLocaleMetricRate{
			Locale: rate.Locale, LookupCount: int(rate.LookupCount),
			MissingKeyCount: int(rate.MissingKeyCount), MissingKeyRate: float32(rate.MissingKeyRate),
			FallbackHitCount: int(rate.FallbackHitCount), FallbackHitRate: float32(rate.FallbackHitRate),
		})
	}
	return apicontract.LocalizationMetricsResponse{
		LookupCount: int(value.LookupCount), MissingKeyCount: int(value.MissingKeyCount), MissingKeyRate: float32(value.MissingKeyRate),
		FallbackHitCount: int(value.FallbackHitCount), FallbackHitRate: float32(value.FallbackHitRate),
		RolloutFallbackCount: int(value.RolloutFallbackCount), PublishCount: int(value.PublishCount),
		AveragePublishLatencyMs: float32(value.AveragePublishLatency), MaximumPublishLatencyMs: int(value.MaximumPublishLatency),
		RollbackCount: int(value.RollbackCount), LocaleRates: localeRates, Hotspots: hotspots, GeneratedAt: value.GeneratedAt,
	}
}

func (e *LocalizationEndpoints) adminLocale(ctx context.Context, requested *string) (string, error) {
	if requested != nil && strings.TrimSpace(*requested) != "" {
		return localizationservice.NormalizeLocale(*requested)
	}
	locales, err := e.localization.ListLocales(ctx, true)
	if err != nil {
		return "", err
	}
	for _, locale := range locales {
		if locale.IsDefault {
			return locale.Code, nil
		}
	}
	return "", localizationservice.ErrRegistryUnavailable
}

func (e *LocalizationEndpoints) ListAdminLocalizationKeys(ctx context.Context, request apicontract.ListAdminLocalizationKeysRequestObject) (apicontract.ListAdminLocalizationKeysResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	locale, err := e.adminLocale(ctx, request.Params.Locale)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	filter := localizationservice.QueueFilter{Locale: locale, Missing: request.Params.Missing, Stale: request.Params.Stale, Page: 1, Limit: 25}
	if request.Params.Namespace != nil {
		filter.Namespace = *request.Params.Namespace
	}
	if request.Params.State != nil {
		filter.State = string(*request.Params.State)
	}
	if request.Params.Q != nil {
		filter.Query = *request.Params.Q
	}
	if request.Params.AssigneeId != nil && *request.Params.AssigneeId > 0 {
		value := uint(*request.Params.AssigneeId)
		filter.AssigneeID = &value
	}
	if request.Params.Page != nil {
		filter.Page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	}
	page, err := e.localization.ListQueue(ctx, filter)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	result := make([]apicontract.TranslationQueueItem, 0, len(page.Items))
	for _, item := range page.Items {
		contract := apicontract.TranslationQueueItem{Key: contractTranslationKey(item.Key), Locale: item.Locale, Missing: item.Missing, Stale: item.Stale, PreviewUrl: item.PreviewURL, ValidationIssues: contractValidationIssues(item.ValidationIssues)}
		if item.LatestValue != nil {
			value := contractTranslationValue(*item.LatestValue, item.ValidationIssues)
			contract.LatestValue = &value
		}
		if item.PublishedValue != nil {
			value := contractTranslationValue(*item.PublishedValue, nil)
			contract.PublishedValue = &value
		}
		result = append(result, contract)
	}
	return apicontract.ListAdminLocalizationKeys200JSONResponse{
		Items: result,
		Pagination: apicontract.Pagination{
			Page:       page.Page,
			Limit:      page.Limit,
			Total:      page.Total,
			TotalPages: page.TotalPages,
		},
	}, nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationRollouts(ctx context.Context, _ apicontract.ListAdminLocalizationRolloutsRequestObject) (apicontract.ListAdminLocalizationRolloutsResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListRollouts(ctx)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ListAdminLocalizationRollouts200JSONResponse(contractLocalizationRollouts(values)), nil
}

func (e *LocalizationEndpoints) ReplaceAdminLocalizationRollouts(ctx context.Context, request apicontract.ReplaceAdminLocalizationRolloutsRequestObject) (apicontract.ReplaceAdminLocalizationRolloutsResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	inputs := make([]localizationservice.RolloutInput, 0, len(request.Body.Rollouts))
	for _, value := range request.Body.Rollouts {
		inputs = append(inputs, localizationservice.RolloutInput{Locale: value.Locale, Domain: string(value.Domain), IsEnabled: value.IsEnabled, Percentage: value.Percentage})
	}
	actorID, _ := localizationActor(ctx)
	values, err := e.localization.ReplaceRollouts(ctx, inputs, actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ReplaceAdminLocalizationRollouts200JSONResponse(contractLocalizationRollouts(values)), nil
}

func (e *LocalizationEndpoints) GetAdminLocalizationMetrics(ctx context.Context, _ apicontract.GetAdminLocalizationMetricsRequestObject) (apicontract.GetAdminLocalizationMetricsResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	value, err := e.localization.Metrics(ctx)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.GetAdminLocalizationMetrics200JSONResponse(contractLocalizationMetrics(value)), nil
}

func (e *LocalizationEndpoints) CreateAdminLocalizationKey(ctx context.Context, request apicontract.CreateAdminLocalizationKeyRequestObject) (apicontract.CreateAdminLocalizationKeyResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	value, err := e.localization.CreateKey(ctx, localizationservice.KeyInput{Namespace: string(request.Body.Namespace), Key: request.Body.Key, SourceText: request.Body.SourceText, Description: stringValue(request.Body.Description), OwnerDomain: request.Body.OwnerDomain})
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.CreateAdminLocalizationKey201JSONResponse(contractTranslationKey(value)), nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationValues(ctx context.Context, request apicontract.ListAdminLocalizationValuesRequestObject) (apicontract.ListAdminLocalizationValuesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListValues(ctx, uint(request.Id))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	result := make([]apicontract.TranslationValue, 0, len(values))
	for _, value := range values {
		issues, validationErr := e.localization.ValidateValue(ctx, value.ID)
		if validationErr != nil {
			return nil, localizationEndpointError(validationErr)
		}
		result = append(result, contractTranslationValue(value, issues))
	}
	return apicontract.ListAdminLocalizationValues200JSONResponse{Values: result}, nil
}

func (e *LocalizationEndpoints) PutAdminLocalizationValue(ctx context.Context, request apicontract.PutAdminLocalizationValueRequestObject) (apicontract.PutAdminLocalizationValueResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	actorID, _ := localizationActor(ctx)
	options := localizationservice.ValueOptions{AssigneeID: uintFromInt(request.Body.AssigneeId), ChangeSummary: stringValue(request.Body.ChangeSummary)}
	if request.Body.ExpectedVersion != nil && *request.Body.ExpectedVersion >= 0 {
		value := uint(*request.Body.ExpectedVersion)
		options.ExpectedVersion = &value
	}
	result, err := e.localization.CreateValueWithOptions(ctx, uint(request.Id), request.Locale, request.Body.Value, actorID, options)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	result.Locale.Code = request.Locale
	issues, err := e.localization.ValidateValue(ctx, result.ID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.PutAdminLocalizationValue200JSONResponse(contractTranslationValue(result, issues)), nil
}

func uintFromInt(value *int) *uint {
	if value == nil || *value < 1 {
		return nil
	}
	converted := uint(*value)
	return &converted
}

func transitionSummary(body *apicontract.TranslationTransitionInput) string {
	if body == nil {
		return ""
	}
	return stringValue(body.ChangeSummary)
}

func (e *LocalizationEndpoints) SubmitAdminLocalizationValueReview(ctx context.Context, request apicontract.SubmitAdminLocalizationValueReviewRequestObject) (apicontract.SubmitAdminLocalizationValueReviewResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.TransitionValueWithAudit(ctx, uint(request.Id), models.TranslationStateReview, actorID, transitionSummary(request.Body))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.SubmitAdminLocalizationValueReview200JSONResponse(contractTranslationValue(value, nil)), nil
}

func (e *LocalizationEndpoints) PublishAdminLocalizationValue(ctx context.Context, request apicontract.PublishAdminLocalizationValueRequestObject) (apicontract.PublishAdminLocalizationValueResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.TransitionValueWithAudit(ctx, uint(request.Id), models.TranslationStatePublished, actorID, transitionSummary(request.Body))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.PublishAdminLocalizationValue200JSONResponse(contractTranslationValue(value, nil)), nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationComments(ctx context.Context, request apicontract.ListAdminLocalizationCommentsRequestObject) (apicontract.ListAdminLocalizationCommentsResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListComments(ctx, uint(request.Id))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	comments := make([]apicontract.TranslationComment, 0, len(values))
	for _, value := range values {
		comments = append(comments, contractTranslationComment(value))
	}
	return apicontract.ListAdminLocalizationComments200JSONResponse{Comments: comments}, nil
}

func (e *LocalizationEndpoints) CreateAdminLocalizationComment(ctx context.Context, request apicontract.CreateAdminLocalizationCommentRequestObject) (apicontract.CreateAdminLocalizationCommentResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	actorID, subject := localizationActor(ctx)
	value, err := e.localization.AddComment(ctx, uint(request.Id), request.Body.Comment, subject, actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.CreateAdminLocalizationComment201JSONResponse(contractTranslationComment(value)), nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationReleases(ctx context.Context, _ apicontract.ListAdminLocalizationReleasesRequestObject) (apicontract.ListAdminLocalizationReleasesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListReleases(ctx)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	releases := make([]apicontract.TranslationRelease, 0, len(values))
	for _, value := range values {
		releases = append(releases, contractTranslationRelease(value))
	}
	return apicontract.ListAdminLocalizationReleases200JSONResponse{Releases: releases}, nil
}

func (e *LocalizationEndpoints) CreateAdminLocalizationRelease(ctx context.Context, request apicontract.CreateAdminLocalizationReleaseRequestObject) (apicontract.CreateAdminLocalizationReleaseResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.CreateReleaseWithAudit(ctx, request.Body.Name, stringValue(request.Body.Notes), actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.CreateAdminLocalizationRelease201JSONResponse(contractTranslationRelease(value)), nil
}

func (e *LocalizationEndpoints) ActivateAdminLocalizationRelease(ctx context.Context, request apicontract.ActivateAdminLocalizationReleaseRequestObject) (apicontract.ActivateAdminLocalizationReleaseResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.ActivateReleaseWithAudit(ctx, uint(request.Id), actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ActivateAdminLocalizationRelease200JSONResponse(contractTranslationRelease(value)), nil
}

func contractTranslationReleaseQuality(value localizationservice.ReleaseQuality) apicontract.TranslationReleaseQuality {
	missing := make([]apicontract.TranslationReleaseQualityMissing, 0, len(value.Missing))
	for _, item := range value.Missing {
		missing = append(missing, apicontract.TranslationReleaseQualityMissing{
			Locale: item.Locale, Namespace: apicontract.TranslationReleaseQualityMissingNamespace(item.Namespace), Key: item.Key,
		})
	}
	namespaces := make([]apicontract.TranslationReleaseQualityCriticalNamespaces, 0, len(value.CriticalNamespaces))
	for _, namespace := range value.CriticalNamespaces {
		namespaces = append(namespaces, apicontract.TranslationReleaseQualityCriticalNamespaces(namespace))
	}
	return apicontract.TranslationReleaseQuality{
		ReleaseId: int(value.ReleaseID), Ready: value.Ready, RequiredLocales: value.RequiredLocales,
		CriticalNamespaces: namespaces, MissingCount: len(value.Missing), Missing: missing,
	}
}

func (e *LocalizationEndpoints) GetAdminLocalizationReleaseQuality(ctx context.Context, request apicontract.GetAdminLocalizationReleaseQualityRequestObject) (apicontract.GetAdminLocalizationReleaseQualityResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	quality, err := e.localization.ReleaseQuality(ctx, uint(request.Id))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.GetAdminLocalizationReleaseQuality200JSONResponse(contractTranslationReleaseQuality(quality)), nil
}

func (e *LocalizationEndpoints) RollbackAdminLocalizationRelease(ctx context.Context, request apicontract.RollbackAdminLocalizationReleaseRequestObject) (apicontract.RollbackAdminLocalizationReleaseResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	actorID, _ := localizationActor(ctx)
	release, err := e.localization.RollbackReleaseWithAudit(ctx, uint(request.Id), actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.RollbackAdminLocalizationRelease201JSONResponse(contractTranslationRelease(release)), nil
}

func (e *LocalizationEndpoints) ExportAdminLocalization(ctx context.Context, request apicontract.ExportAdminLocalizationRequestObject) (apicontract.ExportAdminLocalizationResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	value, err := e.localization.Export(ctx, request.Body.Locale, stringValue(request.Body.Namespace), string(request.Body.Format))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ExportAdminLocalization200JSONResponse{Locale: value.Locale, Namespace: optionalCMSString(value.Namespace), Format: apicontract.TranslationDocumentFormat(value.Format), Filename: value.Filename, Content: value.Content}, nil
}

func (e *LocalizationEndpoints) ImportAdminLocalization(ctx context.Context, request apicontract.ImportAdminLocalizationRequestObject) (apicontract.ImportAdminLocalizationResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.Import(ctx, request.Body.Locale, stringValue(request.Body.Namespace), string(request.Body.Format), request.Body.Content, request.Body.DryRun, actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	entries := make([]apicontract.TranslationImportEntry, 0, len(value.Entries))
	for _, entry := range value.Entries {
		entries = append(entries, apicontract.TranslationImportEntry{Key: entry.Key, Status: apicontract.TranslationImportEntryStatus(entry.Status), Detail: entry.Detail})
	}
	return apicontract.ImportAdminLocalization200JSONResponse{DryRun: value.DryRun, Created: value.Created, Unchanged: value.Unchanged, Conflicts: value.Conflicts, Invalid: value.Invalid, Entries: entries}, nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationGlossary(ctx context.Context, request apicontract.ListAdminLocalizationGlossaryRequestObject) (apicontract.ListAdminLocalizationGlossaryResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	locale := ""
	if request.Params.Locale != nil {
		locale = *request.Params.Locale
	}
	values, err := e.localization.ListGlossary(ctx, locale)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	terms := make([]apicontract.LocalizationGlossaryTerm, 0, len(values))
	for _, value := range values {
		terms = append(terms, apicontract.LocalizationGlossaryTerm{Id: int(value.ID), Locale: value.Locale.Code, SourceTerm: value.SourceTerm, TranslatedTerm: value.TranslatedTerm, Description: value.Description, IsLocked: value.IsLocked, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt})
	}
	return apicontract.ListAdminLocalizationGlossary200JSONResponse{Terms: terms}, nil
}

func (e *LocalizationEndpoints) PutAdminLocalizationGlossaryTerm(ctx context.Context, request apicontract.PutAdminLocalizationGlossaryTermRequestObject) (apicontract.PutAdminLocalizationGlossaryTermResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	value, err := e.localization.PutGlossary(ctx, request.Body.Locale, request.Body.SourceTerm, request.Body.TranslatedTerm, stringValue(request.Body.Description), request.Body.IsLocked)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.PutAdminLocalizationGlossaryTerm200JSONResponse{Id: int(value.ID), Locale: value.Locale.Code, SourceTerm: value.SourceTerm, TranslatedTerm: value.TranslatedTerm, Description: value.Description, IsLocked: value.IsLocked, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}, nil
}

func (e *LocalizationEndpoints) DeleteAdminLocalizationGlossaryTerm(ctx context.Context, request apicontract.DeleteAdminLocalizationGlossaryTermRequestObject) (apicontract.DeleteAdminLocalizationGlossaryTermResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	if err := e.localization.DeleteGlossary(ctx, uint(request.Id)); err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.DeleteAdminLocalizationGlossaryTerm204Response{}, nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationRoles(ctx context.Context, _ apicontract.ListAdminLocalizationRolesRequestObject) (apicontract.ListAdminLocalizationRolesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	values, err := e.localization.ListRoles(ctx)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	assignments := make([]apicontract.LocalizationRoleAssignment, 0, len(values))
	for _, value := range values {
		assignments = append(assignments, apicontract.LocalizationRoleAssignment{Id: int(value.ID), Subject: value.Subject, Role: apicontract.LocalizationRoleAssignmentRole(value.Role), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt})
	}
	return apicontract.ListAdminLocalizationRoles200JSONResponse{Assignments: assignments}, nil
}

func (e *LocalizationEndpoints) PutAdminLocalizationRole(ctx context.Context, request apicontract.PutAdminLocalizationRoleRequestObject) (apicontract.PutAdminLocalizationRoleResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRolePublisher); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	value, err := e.localization.PutRole(ctx, request.Body.Subject, models.LocalizationRole(request.Body.Role))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.PutAdminLocalizationRole200JSONResponse{Id: int(value.ID), Subject: value.Subject, Role: apicontract.LocalizationRoleAssignmentRole(value.Role), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}, nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationAssignees(ctx context.Context, request apicontract.ListAdminLocalizationAssigneesRequestObject) (apicontract.ListAdminLocalizationAssigneesResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	search := ""
	if request.Params.Q != nil {
		search = *request.Params.Q
	}
	values, err := e.localization.ListAssignees(ctx, search, limit)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	assignees := make([]apicontract.LocalizationAssignee, 0, len(values))
	for _, value := range values {
		assignees = append(assignees, apicontract.LocalizationAssignee{Id: int(value.ID), Name: value.Name, Email: openapi_types.Email(value.Email), LocalizationRole: apicontract.LocalizationAssigneeLocalizationRole(value.LocalizationRole)})
	}
	return apicontract.ListAdminLocalizationAssignees200JSONResponse{Assignees: assignees}, nil
}

func contextResolutionInput(ctx context.Context) localizationservice.ResolutionInput {
	value, ok := requestctx.LocaleResolutionFrom(ctx)
	if !ok {
		return localizationservice.ResolutionInput{}
	}
	return localizationservice.ResolutionInput{ExplicitLocale: value.ResolvedLocale}
}

func (e *LocalizationEndpoints) GetAdminEntityLocalization(ctx context.Context, request apicontract.GetAdminEntityLocalizationRequestObject) (apicontract.GetAdminEntityLocalizationResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleTranslator); err != nil {
		return nil, localizationEndpointError(err)
	}
	entityType, err := localizationservice.ParseEntityType(string(request.EntityType))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	value, err := e.localization.EntityLocalizations(ctx, entityType, uint(request.EntityId), contextResolutionInput(ctx))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.GetAdminEntityLocalization200JSONResponse(contractEntityLocalization(value)), nil
}

func (e *LocalizationEndpoints) PutAdminEntityLocalization(ctx context.Context, request apicontract.PutAdminEntityLocalizationRequestObject) (apicontract.PutAdminEntityLocalizationResponseObject, error) {
	if err := e.localization.RequireRole(ctx, models.LocalizationRoleEditor); err != nil {
		return nil, localizationEndpointError(err)
	}
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidTranslation)
	}
	entityType, err := localizationservice.ParseEntityType(string(request.EntityType))
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	actorID, _ := localizationActor(ctx)
	value, err := e.localization.PutEntityLocalization(ctx, entityType, uint(request.EntityId), request.Body.Locale, request.Body.Fields, actorID)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.PutAdminEntityLocalization200JSONResponse(contractEntityLocalization(value)), nil
}
