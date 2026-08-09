package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/services/cms"
	"ecommerce/models"
)

func contractSEOMetadata(metadata models.SEOMetadata) (apicontract.CmsSEOMetadata, error) {
	var jsonLD []map[string]any
	if strings.TrimSpace(metadata.JSONLD) != "" {
		if err := json.Unmarshal([]byte(metadata.JSONLD), &jsonLD); err != nil {
			return apicontract.CmsSEOMetadata{}, err
		}
	}
	return apicontract.CmsSEOMetadata{Title: stringPointerValue(metadata.Title), Description: stringPointerValue(metadata.Description), CanonicalUrl: stringPointerValue(metadata.CanonicalPath), Robots: apicontract.CmsSEOMetadataRobots(metadata.Robots), OgTitle: stringPointerValue(metadata.OGTitle), OgDescription: stringPointerValue(metadata.OGDescription), OgImageMediaId: metadata.OgImageMediaID, TwitterCard: apicontract.CmsSEOMetadataTwitterCard(metadata.TwitterCard), TwitterTitle: stringPointerValue(metadata.TwitterTitle), TwitterDescription: stringPointerValue(metadata.TwitterDescription), TwitterImageMediaId: metadata.TwitterImageMediaID, JsonLd: jsonLD}, nil
}
func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func seoInput(body *apicontract.CmsSEOInput) (cms.SEOInput, error) {
	if body == nil {
		return cms.SEOInput{}, errors.New("CMS SEO body is required")
	}
	return cms.SEOInput{Title: body.Title, Description: body.Description, CanonicalURL: body.CanonicalUrl, Robots: string(body.Robots), OGTitle: body.OgTitle, OGDescription: body.OgDescription, OGImageMediaID: body.OgImageMediaId, TwitterCard: string(body.TwitterCard), TwitterTitle: body.TwitterTitle, TwitterDescription: body.TwitterDescription, TwitterImageMediaID: body.TwitterImageMediaId, JSONLD: body.JsonLd}, nil
}
func contractSEO(record *cms.SEORecord) (apicontract.CmsSEOResponse, error) {
	metadata, err := contractSEOMetadata(record.Metadata)
	return apicontract.CmsSEOResponse{Metadata: metadata, Issues: record.Issues}, err
}
func (e *CmsMediaEndpoints) GetAdminCmsPageSeo(ctx context.Context, request apicontract.GetAdminCmsPageSeoRequestObject) (apicontract.GetAdminCmsPageSeoResponseObject, error) {
	record, err := e.pages.GetSEO(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	value, err := contractSEO(record)
	return apicontract.GetAdminCmsPageSeo200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsPageSeo(ctx context.Context, request apicontract.UpdateAdminCmsPageSeoRequestObject) (apicontract.UpdateAdminCmsPageSeoResponseObject, error) {
	input, err := seoInput(request.Body)
	if err != nil {
		return nil, err
	}
	record, err := e.pages.UpdateSEO(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractSEO(record)
	return apicontract.UpdateAdminCmsPageSeo200JSONResponse(value), err
}

func deliveryInput(body *apicontract.CmsPageDeliveryRequest) (cms.DeliveryInput, error) {
	if body == nil {
		return cms.DeliveryInput{}, errors.New("CMS delivery body is required")
	}
	input := cms.DeliveryInput{}
	if body.Schedule != nil {
		input.Schedule = &cms.ScheduleInput{PublishAt: body.Schedule.PublishAt, UnpublishAt: body.Schedule.UnpublishAt, Timezone: body.Schedule.Timezone}
	}
	for _, rule := range body.TargetingRules {
		input.TargetingRules = append(input.TargetingRules, cms.TargetingRuleInput{TargetingRule: cms.TargetingRule{Markets: rule.Markets, DeviceClasses: stringSlice(rule.DeviceClasses), AuthStates: stringSlice(rule.AuthStates), Referrers: rule.Referrers, UTMSources: rule.UtmSources, SegmentKeys: rule.SegmentKeys}, Priority: rule.Priority, IsEnabled: rule.IsEnabled})
	}
	if body.Experiment != nil {
		experiment := cms.ExperimentInput{Name: body.Experiment.Name, Status: models.CMSExperimentStatus(body.Experiment.Status), StickyKey: string(body.Experiment.StickyKey), StartsAt: body.Experiment.StartsAt, EndsAt: body.Experiment.EndsAt}
		for _, variant := range body.Experiment.Variants {
			experiment.Variants = append(experiment.Variants, cms.ExperimentVariantInput{Name: variant.Name, VersionID: uint(variant.VersionId), Allocation: variant.Allocation})
		}
		input.Experiment = &experiment
	}
	return input, nil
}
func stringSlice[T ~string](values []T) []string {
	result := make([]string, len(values))
	for i := range values {
		result[i] = string(values[i])
	}
	return result
}
func contractDelivery(record *cms.DeliveryRecord) (apicontract.CmsPageDeliveryResponse, error) {
	result := apicontract.CmsPageDeliveryResponse{TargetingRules: []apicontract.CmsTargetingRule{}, RecentPublications: []apicontract.CmsPublication{}}
	if record.Schedule != nil {
		value, err := contractConvert[apicontract.CmsSchedule](record.Schedule)
		if err != nil {
			return result, err
		}
		result.Schedule = &value
	}
	if record.Experiment != nil {
		value, err := contractConvert[apicontract.CmsExperiment](record.Experiment)
		if err != nil {
			return result, err
		}
		result.Experiment = &value
	}
	for _, publication := range record.RecentPublications {
		result.RecentPublications = append(result.RecentPublications, *contractPublication(&publication))
	}
	for _, rule := range record.TargetingRules {
		result.TargetingRules = append(result.TargetingRules, apicontract.CmsTargetingRule{Id: int(rule.Model.ID), Markets: rule.Rule.Markets, DeviceClasses: enumDeviceSlice(rule.Rule.DeviceClasses), AuthStates: enumAuthSlice(rule.Rule.AuthStates), Referrers: rule.Rule.Referrers, UtmSources: rule.Rule.UTMSources, SegmentKeys: rule.Rule.SegmentKeys, Priority: rule.Model.Priority, IsEnabled: rule.Model.IsEnabled})
	}
	return result, nil
}
func enumDeviceSlice(values []string) []apicontract.CmsTargetingRuleDeviceClasses {
	result := make([]apicontract.CmsTargetingRuleDeviceClasses, len(values))
	for i := range values {
		result[i] = apicontract.CmsTargetingRuleDeviceClasses(values[i])
	}
	return result
}
func enumAuthSlice(values []string) []apicontract.CmsTargetingRuleAuthStates {
	result := make([]apicontract.CmsTargetingRuleAuthStates, len(values))
	for i := range values {
		result[i] = apicontract.CmsTargetingRuleAuthStates(values[i])
	}
	return result
}
func (e *CmsMediaEndpoints) GetAdminCmsPageDelivery(ctx context.Context, request apicontract.GetAdminCmsPageDeliveryRequestObject) (apicontract.GetAdminCmsPageDeliveryResponseObject, error) {
	record, err := e.pages.GetDelivery(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	value, err := contractDelivery(record)
	return apicontract.GetAdminCmsPageDelivery200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsPageDelivery(ctx context.Context, request apicontract.UpdateAdminCmsPageDeliveryRequestObject) (apicontract.UpdateAdminCmsPageDeliveryResponseObject, error) {
	input, err := deliveryInput(request.Body)
	if err != nil {
		return nil, err
	}
	record, err := e.pages.UpdateDelivery(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractDelivery(record)
	return apicontract.UpdateAdminCmsPageDelivery200JSONResponse(value), err
}

func contractLocale(locale models.CMSLocale) apicontract.CmsLocale {
	return apicontract.CmsLocale{Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault, FallbackLocale: optionalCMSString(locale.FallbackLocale)}
}
func (e *CmsMediaEndpoints) GetAdminCmsLocales(ctx context.Context, _ apicontract.GetAdminCmsLocalesRequestObject) (apicontract.GetAdminCmsLocalesResponseObject, error) {
	locales, err := e.pages.Locales(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsLocale, 0, len(locales))
	for _, locale := range locales {
		data = append(data, contractLocale(locale))
	}
	return apicontract.GetAdminCmsLocales200JSONResponse{Locales: data}, nil
}
func (e *CmsMediaEndpoints) UpdateAdminCmsLocales(ctx context.Context, request apicontract.UpdateAdminCmsLocalesRequestObject) (apicontract.UpdateAdminCmsLocalesResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS locale body is required")
	}
	inputs := make([]cms.LocaleInput, 0, len(request.Body.Locales))
	for _, locale := range request.Body.Locales {
		inputs = append(inputs, cms.LocaleInput{Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault, FallbackLocale: stringPointerValue(locale.FallbackLocale)})
	}
	actor, _ := cmsActor(ctx)
	locales, err := e.pages.UpdateLocales(ctx, inputs, actor)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsLocale, 0, len(locales))
	for _, locale := range locales {
		data = append(data, contractLocale(locale))
	}
	return apicontract.UpdateAdminCmsLocales200JSONResponse{Locales: data}, nil
}

func contractRedirect(rule models.CMSRedirectRule) apicontract.CmsRedirectRule {
	return apicontract.CmsRedirectRule{Id: int(rule.ID), SourcePattern: rule.SourcePattern, MatchType: apicontract.CmsRedirectRuleMatchType(rule.MatchType), TargetUrl: rule.TargetURL, RedirectType: apicontract.CmsRedirectRuleRedirectType(rule.RedirectType), Priority: rule.Priority, IsEnabled: rule.IsEnabled, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt}
}
func redirectInput(body *apicontract.CmsRedirectInput) (cms.RedirectInput, error) {
	if body == nil {
		return cms.RedirectInput{}, errors.New("CMS redirect body is required")
	}
	return cms.RedirectInput{SourcePattern: body.SourcePattern, MatchType: string(body.MatchType), TargetURL: body.TargetUrl, RedirectType: int(body.RedirectType), Priority: body.Priority, IsEnabled: body.IsEnabled}, nil
}
func (e *CmsMediaEndpoints) ListAdminCmsRedirects(ctx context.Context, _ apicontract.ListAdminCmsRedirectsRequestObject) (apicontract.ListAdminCmsRedirectsResponseObject, error) {
	rules, err := e.redirects.List(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsRedirectRule, 0, len(rules))
	for _, rule := range rules {
		data = append(data, contractRedirect(rule))
	}
	return apicontract.ListAdminCmsRedirects200JSONResponse(data), nil
}
func (e *CmsMediaEndpoints) CreateAdminCmsRedirect(ctx context.Context, request apicontract.CreateAdminCmsRedirectRequestObject) (apicontract.CreateAdminCmsRedirectResponseObject, error) {
	input, err := redirectInput(request.Body)
	if err != nil {
		return nil, err
	}
	rule, err := e.redirects.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminCmsRedirect201JSONResponse(contractRedirect(*rule)), nil
}
func (e *CmsMediaEndpoints) UpdateAdminCmsRedirect(ctx context.Context, request apicontract.UpdateAdminCmsRedirectRequestObject) (apicontract.UpdateAdminCmsRedirectResponseObject, error) {
	input, err := redirectInput(request.Body)
	if err != nil {
		return nil, err
	}
	rule, err := e.redirects.Update(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminCmsRedirect200JSONResponse(contractRedirect(*rule)), nil
}
func (e *CmsMediaEndpoints) DeleteAdminCmsRedirect(ctx context.Context, request apicontract.DeleteAdminCmsRedirectRequestObject) (apicontract.DeleteAdminCmsRedirectResponseObject, error) {
	if err := e.redirects.Delete(ctx, uint(request.Id)); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsRedirect200JSONResponse{Message: "CMS redirect deleted"}, nil
}
func (e *CmsMediaEndpoints) ResolveContentRedirect(ctx context.Context, request apicontract.ResolveContentRedirectRequestObject) (apicontract.ResolveContentRedirectResponseObject, error) {
	rule, target, err := e.redirects.Resolve(ctx, request.Params.Path)
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	return apicontract.ResolveContentRedirect200JSONResponse{TargetUrl: target, RedirectType: apicontract.CmsRedirectResolutionRedirectType(rule.RedirectType)}, nil
}

func (e *CmsMediaEndpoints) PreviewAdminCmsPayload(_ context.Context, request apicontract.PreviewAdminCmsPayloadRequestObject) (apicontract.PreviewAdminCmsPayloadResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("CMS preview body is required")
	}
	payload, err := cmsPayload(request.Body.Payload)
	if err != nil {
		return nil, err
	}
	assessment := cms.AssessPayload(payload)
	blocks := make([]apicontract.CmsPreviewBlock, 0, len(assessment.Blocks))
	for _, block := range assessment.Blocks {
		messages := make([]string, 0, len(block.Issues))
		for _, issue := range block.Issues {
			messages = append(messages, issue.Message)
		}
		status := apicontract.Ok
		if block.Status != cms.AssessmentValid {
			status = apicontract.Degraded
		}
		blocks = append(blocks, apicontract.CmsPreviewBlock{Key: fmt.Sprintf("block-%d", block.Index), Type: block.Type, Status: status, Messages: messages, ItemCount: 0})
	}
	return apicontract.PreviewAdminCmsPayload200JSONResponse{Blocks: blocks}, nil
}
func (e *CmsMediaEndpoints) RecordContentEvent(ctx context.Context, request apicontract.RecordContentEventRequestObject) (apicontract.RecordContentEventResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("content event body is required")
	}
	err := e.pages.RecordContentEvent(ctx, cms.ContentEventInput{ContentVersionID: uint(request.Body.ContentVersionId), ExperimentID: intPointerToUint(request.Body.ExperimentId), ExperimentVariantID: intPointerToUint(request.Body.ExperimentVariantId), CorrelationID: request.Body.CorrelationId, EventType: string(request.Body.EventType)})
	if err != nil {
		return nil, err
	}
	return apicontract.RecordContentEvent202JSONResponse{Message: "Content event accepted"}, nil
}
func intPointerToUint(value *int) *uint {
	if value == nil {
		return nil
	}
	result := uint(*value)
	return &result
}
func (e *CmsMediaEndpoints) GetContentSitemap(ctx context.Context, _ apicontract.GetContentSitemapRequestObject) (apicontract.GetContentSitemapResponseObject, error) {
	if e.sitemapOrigin == "" {
		return nil, errors.New("PUBLIC_URL is required to generate the CMS sitemap")
	}
	raw, err := cms.GenerateSitemap(ctx, e.db, e.sitemapOrigin)
	if err != nil {
		return nil, err
	}
	return apicontract.GetContentSitemap200ApplicationxmlResponse{Body: strings.NewReader(string(raw)), ContentLength: int64(len(raw))}, nil
}
