package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"

	"gorm.io/gorm"
)

type LocalizationEndpoints struct {
	localization *localizationservice.Service
	media        *media.Service
}

func NewLocalizationEndpoints(db *gorm.DB) (*LocalizationEndpoints, error) {
	if db == nil {
		return nil, errors.New("localization database is required")
	}
	return NewLocalizationEndpointsWithMedia(db, media.NewService(db, "", "", nil, nil))
}

func NewLocalizationEndpointsWithMedia(db *gorm.DB, mediaService *media.Service) (*LocalizationEndpoints, error) {
	if db == nil {
		return nil, errors.New("localization database is required")
	}
	if mediaService == nil {
		return nil, errors.New("localization media service is required")
	}
	return &LocalizationEndpoints{localization: localizationservice.NewService(db), media: mediaService}, nil
}

func localizationEndpointError(err error) error {
	switch {
	case errors.Is(err, localizationservice.ErrInvalidLocale),
		errors.Is(err, localizationservice.ErrInvalidTranslation),
		errors.Is(err, localizationservice.ErrLocaleNotFound),
		errors.Is(err, localizationservice.ErrLocaleDisabled):
		return problemError(http.StatusBadRequest, "invalid_request", err.Error(), err)
	case errors.Is(err, localizationservice.ErrLocalizationForbidden):
		return problemError(http.StatusForbidden, "forbidden", "You are not allowed to perform this localization operation.", err)
	case errors.Is(err, localizationservice.ErrTranslationKeyNotFound),
		errors.Is(err, localizationservice.ErrTranslationValueNotFound),
		errors.Is(err, localizationservice.ErrReleaseNotFound),
		errors.Is(err, localizationservice.ErrLocalizedEntityNotFound):
		return problemError(http.StatusNotFound, "not_found", "The requested localization resource was not found.", err)
	case errors.Is(err, localizationservice.ErrInvalidTransition):
		return problemError(http.StatusConflict, "state_conflict", err.Error(), err)
	case errors.Is(err, localizationservice.ErrReleaseQualityFailed):
		return problemError(http.StatusUnprocessableEntity, "localization_quality_failed", err.Error(), err)
	case errors.Is(err, localizationservice.ErrRegistryUnavailable),
		errors.Is(err, localizationservice.ErrReleaseUnavailable):
		return problemError(http.StatusServiceUnavailable, "localization_unavailable", "Localization configuration is unavailable.", err)
	default:
		return err
	}
}

func contractLocalizationLocale(locale localizationservice.LocaleRecord) apicontract.LocalizationLocale {
	return apicontract.LocalizationLocale{
		Code: locale.Code, Name: locale.Name, IsEnabled: locale.IsEnabled, IsDefault: locale.IsDefault,
		FallbackLocale: optionalCMSString(locale.FallbackLocale), DefaultForMarkets: locale.DefaultMarkets,
	}
}

func contractLocalizationLocaleList(locales []localizationservice.LocaleRecord) apicontract.LocalizationLocaleList {
	result := apicontract.LocalizationLocaleList{Locales: make([]apicontract.LocalizationLocale, 0, len(locales))}
	for _, locale := range locales {
		result.Locales = append(result.Locales, contractLocalizationLocale(locale))
		if locale.IsDefault {
			result.DefaultLocale = locale.Code
		}
	}
	return result
}

func contractLocalizationResolution(value localizationservice.Resolution) apicontract.LocalizationResolution {
	return apicontract.LocalizationResolution{
		RequestedLocale: value.RequestedLocale,
		ResolvedLocale:  value.ResolvedLocale,
		Source:          apicontract.LocalizationResolutionSource(value.Source),
		FallbackChain:   value.FallbackChain,
		UsedFallback:    value.UsedFallback,
	}
}

func contractLocalizationRelease(value localizationservice.ReleaseMetadata) apicontract.TranslationReleaseMeta {
	return apicontract.TranslationReleaseMeta{
		Id: int(value.ID), Name: value.Name, SnapshotHash: value.SnapshotHash, Version: value.Version, PublishedAt: value.PublishedAt,
	}
}

const localizationBundleCacheControl = "public, max-age=0, must-revalidate"

func localizationBundleETag(version string) string {
	return `"` + version + `"`
}

func matchesLocalizationBundleETag(candidate *string, etag string) bool {
	if candidate == nil {
		return false
	}
	for _, value := range strings.Split(*candidate, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || value == etag || strings.TrimPrefix(value, "W/") == etag {
			return true
		}
	}
	return false
}

func contractLocalizationBundle(value localizationservice.Bundle) apicontract.LocalizationBundle {
	messages := make(map[string]apicontract.LocalizationBundleMessage, len(value.Messages))
	for key, message := range value.Messages {
		messages[key] = apicontract.LocalizationBundleMessage{
			Value: message.Value, RequestedLocale: message.RequestedLocale, SourceLocale: message.SourceLocale,
			UsedFallback: message.UsedFallback, MissingTranslation: message.MissingTranslation,
		}
	}
	return apicontract.LocalizationBundle{
		Resolution: contractLocalizationResolution(value.Resolution),
		Release:    contractLocalizationRelease(value.Release),
		Messages:   messages,
	}
}

func requestLocalizationResolution(ctx context.Context, locale string) (localizationservice.Resolution, bool) {
	value, ok := requestctx.LocaleResolutionFrom(ctx)
	if !ok || value.RequestedLocale != locale {
		return localizationservice.Resolution{}, false
	}
	return localizationservice.Resolution{
		RequestedLocale: value.RequestedLocale, ResolvedLocale: value.ResolvedLocale,
		Source: value.Source, FallbackChain: value.FallbackChain, UsedFallback: value.UsedFallback,
	}, true
}

func (e *LocalizationEndpoints) ListLocalizationLocales(ctx context.Context, _ apicontract.ListLocalizationLocalesRequestObject) (apicontract.ListLocalizationLocalesResponseObject, error) {
	locales, err := e.localization.ListLocales(ctx, true)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ListLocalizationLocales200JSONResponse(contractLocalizationLocaleList(locales)), nil
}

func (e *LocalizationEndpoints) ListAdminLocalizationLocales(ctx context.Context, _ apicontract.ListAdminLocalizationLocalesRequestObject) (apicontract.ListAdminLocalizationLocalesResponseObject, error) {
	locales, err := e.localization.ListLocales(ctx, false)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ListAdminLocalizationLocales200JSONResponse(contractLocalizationLocaleList(locales)), nil
}

func (e *LocalizationEndpoints) ReplaceAdminLocalizationLocales(ctx context.Context, request apicontract.ReplaceAdminLocalizationLocalesRequestObject) (apicontract.ReplaceAdminLocalizationLocalesResponseObject, error) {
	if request.Body == nil {
		return nil, localizationEndpointError(localizationservice.ErrInvalidLocale)
	}
	inputs := make([]localizationservice.LocaleInput, 0, len(request.Body.Locales))
	for _, locale := range request.Body.Locales {
		fallback := ""
		if locale.FallbackLocale != nil {
			fallback = *locale.FallbackLocale
		}
		inputs = append(inputs, localizationservice.LocaleInput{
			Code: locale.Code, Name: locale.Name, IsEnabled: locale.IsEnabled,
			IsDefault: locale.IsDefault, FallbackLocale: fallback, DefaultMarkets: locale.DefaultForMarkets,
		})
	}
	locales, err := e.localization.ReplaceLocales(ctx, inputs)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	return apicontract.ReplaceAdminLocalizationLocales200JSONResponse(contractLocalizationLocaleList(locales)), nil
}

func (e *LocalizationEndpoints) GetLocalizationBundle(ctx context.Context, request apicontract.GetLocalizationBundleRequestObject) (apicontract.GetLocalizationBundleResponseObject, error) {
	locale, err := localizationservice.NormalizeLocale(request.Locale)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	namespace := ""
	if request.Params.Namespace != nil {
		namespace = string(*request.Params.Namespace)
	}
	domain := "storefront"
	if request.Params.Domain != nil {
		domain = string(*request.Params.Domain)
	} else if namespace == "checkout" || namespace == "admin" || namespace == "communications" || namespace == "errors" {
		domain = namespace
	}
	resolution, resolvedByMiddleware := requestLocalizationResolution(ctx, locale)
	if !resolvedByMiddleware {
		resolution, err = e.localization.ResolveLocale(ctx, localizationservice.ResolutionInput{ExplicitLocale: locale})
		if err != nil {
			return nil, localizationEndpointError(err)
		}
		resolution, err = e.localization.ApplyRollout(ctx, resolution, domain, localizationservice.RolloutIdentity(ctx))
		if err != nil {
			return nil, localizationEndpointError(err)
		}
	}
	bundle, err := e.localization.BundleForResolution(ctx, resolution, namespace)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	etag := localizationBundleETag(bundle.Release.Version)
	if matchesLocalizationBundleETag(request.Params.IfNoneMatch, etag) {
		return apicontract.GetLocalizationBundle304Response{Headers: apicontract.GetLocalizationBundle304ResponseHeaders{ETag: etag, CacheControl: localizationBundleCacheControl}}, nil
	}
	return apicontract.GetLocalizationBundle200JSONResponse{
		Body:    contractLocalizationBundle(bundle),
		Headers: apicontract.GetLocalizationBundle200ResponseHeaders{ETag: etag, CacheControl: localizationBundleCacheControl},
	}, nil
}

func (e *LocalizationEndpoints) GetLocalizationBundleMeta(ctx context.Context, request apicontract.GetLocalizationBundleMetaRequestObject) (apicontract.GetLocalizationBundleMetaResponseObject, error) {
	locale, err := localizationservice.NormalizeLocale(request.Locale)
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	domain := "storefront"
	if request.Params.Domain != nil {
		domain = string(*request.Params.Domain)
	}
	resolution, resolvedByMiddleware := requestLocalizationResolution(ctx, locale)
	if !resolvedByMiddleware {
		resolution, err = e.localization.ResolveLocale(ctx, localizationservice.ResolutionInput{ExplicitLocale: locale})
		if err != nil {
			return nil, localizationEndpointError(err)
		}
		resolution, err = e.localization.ApplyRollout(ctx, resolution, domain, localizationservice.RolloutIdentity(ctx))
		if err != nil {
			return nil, localizationEndpointError(err)
		}
	}
	bundle, err := e.localization.BundleForResolution(ctx, resolution, "")
	if err != nil {
		return nil, localizationEndpointError(err)
	}
	etag := localizationBundleETag(bundle.Release.Version)
	if matchesLocalizationBundleETag(request.Params.IfNoneMatch, etag) {
		return apicontract.GetLocalizationBundleMeta304Response{Headers: apicontract.GetLocalizationBundleMeta304ResponseHeaders{ETag: etag, CacheControl: localizationBundleCacheControl}}, nil
	}
	return apicontract.GetLocalizationBundleMeta200JSONResponse{
		Body: apicontract.LocalizationBundleMeta{
			Resolution: contractLocalizationResolution(bundle.Resolution),
			Release:    contractLocalizationRelease(bundle.Release),
		},
		Headers: apicontract.GetLocalizationBundleMeta200ResponseHeaders{ETag: etag, CacheControl: localizationBundleCacheControl},
	}, nil
}
