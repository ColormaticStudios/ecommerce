package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
)

const (
	LocaleCookieName              = "locale"
	LocaleHeaderName              = "X-Locale"
	MarketHeaderName              = "X-Market"
	LocalizationRolloutCookieName = "localization_rollout"
	localizationRolloutCookieAge  = 365 * 24 * 60 * 60
)

type AccountLocalePreference func(context.Context, requestctx.Principal) (string, error)

type LocalizationNegotiationOptions struct {
	Service                  *localizationservice.Service
	ResolveAccountPreference AccountLocalePreference
	Renderer                 Renderer
}

func LocalizationNegotiationMiddleware(options LocalizationNegotiationOptions) (gin.HandlerFunc, error) {
	if options.Service == nil {
		return nil, errors.New("localization service is required")
	}
	return func(ctx *gin.Context) {
		requestContext := ctx.Request.Context()
		var err error
		requestContext, err = ensureLocalizationRolloutIdentity(ctx.Writer, ctx.Request, requestContext)
		if err != nil {
			options.Renderer.Render(ctx.Writer, requestContext, http.StatusServiceUnavailable, problemError(
				http.StatusServiceUnavailable, "localization_unavailable", "Localization rollout identity is unavailable.", err,
			))
			ctx.Abort()
			return
		}
		ctx.Request = ctx.Request.WithContext(requestContext)
		accountPreference := ""
		if options.ResolveAccountPreference != nil {
			if principal, ok := requestctx.PrincipalFrom(requestContext); ok {
				preference, err := options.ResolveAccountPreference(requestContext, principal)
				if err != nil {
					options.Renderer.Render(ctx.Writer, requestContext, http.StatusServiceUnavailable, problemError(
						http.StatusServiceUnavailable, "localization_unavailable", "Localization configuration is unavailable.", err,
					))
					ctx.Abort()
					return
				}
				accountPreference = preference
			}
		}
		market := requestMarket(ctx.Request)
		marketDefault, err := options.Service.MarketDefault(requestContext, market)
		if err != nil {
			options.Renderer.Render(ctx.Writer, requestContext, http.StatusBadRequest, problemError(
				http.StatusBadRequest, "invalid_market", "The requested market is invalid.", err,
			))
			ctx.Abort()
			return
		}
		explicitLocale, err := requestLocale(requestContext, ctx.Request, options.Service)
		if err != nil {
			options.Renderer.Render(ctx.Writer, requestContext, http.StatusServiceUnavailable, problemError(
				http.StatusServiceUnavailable, "localization_unavailable", "Localization configuration is unavailable.", err,
			))
			ctx.Abort()
			return
		}
		resolution, err := options.Service.ResolveLocale(requestContext, localizationservice.ResolutionInput{
			ExplicitLocale: explicitLocale, AccountPreference: accountPreference, MarketDefault: marketDefault,
		})
		if err != nil {
			options.Renderer.Render(ctx.Writer, requestContext, http.StatusServiceUnavailable, problemError(
				http.StatusServiceUnavailable, "localization_unavailable", "Localization configuration is unavailable.", err,
			))
			ctx.Abort()
			return
		}
		resolution, err = options.Service.ApplyRollout(
			requestContext,
			resolution,
			localizationRolloutDomain(ctx.Request),
			localizationservice.RolloutIdentity(requestContext),
		)
		if err != nil {
			options.Renderer.Render(ctx.Writer, requestContext, http.StatusServiceUnavailable, problemError(
				http.StatusServiceUnavailable, "localization_unavailable", "Localization rollout configuration is unavailable.", err,
			))
			ctx.Abort()
			return
		}
		requestContext = requestctx.WithLocaleResolution(requestContext, requestctx.LocaleResolution{
			RequestedLocale: resolution.RequestedLocale, ResolvedLocale: resolution.ResolvedLocale,
			Source: resolution.Source, FallbackChain: resolution.FallbackChain, UsedFallback: resolution.UsedFallback, Market: market,
		})
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header("Content-Language", resolution.ResolvedLocale)
		ctx.Next()
	}, nil
}

func ensureLocalizationRolloutIdentity(writer http.ResponseWriter, request *http.Request, ctx context.Context) (context.Context, error) {
	if _, ok := requestctx.PrincipalFrom(ctx); ok {
		return ctx, nil
	}
	metadata, _ := requestctx.MetadataFrom(ctx)
	for _, name := range []string{LocalizationRolloutCookieName, "checkout_session", "session_token"} {
		if strings.TrimSpace(metadata.Cookies[name]) != "" {
			return ctx, nil
		}
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return ctx, err
	}
	value := base64.RawURLEncoding.EncodeToString(random)
	http.SetCookie(writer, &http.Cookie{
		Name: LocalizationRolloutCookieName, Value: value, Path: "/", MaxAge: localizationRolloutCookieAge,
		HttpOnly: true, Secure: request.TLS != nil || strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https"), SameSite: http.SameSiteLaxMode,
	})
	if metadata.Cookies == nil {
		metadata.Cookies = make(map[string]string)
	}
	metadata.Cookies[LocalizationRolloutCookieName] = value
	return requestctx.WithMetadata(ctx, metadata), nil
}

func requestLocale(ctx context.Context, request *http.Request, service *localizationservice.Service) (string, error) {
	if value := localizationBundlePathLocale(request.URL.Path); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(request.URL.Query().Get("locale")); value != "" {
		return value, nil
	}
	if cookie, err := request.Cookie(LocaleCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value), nil
	}
	if value := strings.TrimSpace(request.Header.Get(LocaleHeaderName)); value != "" {
		return value, nil
	}
	candidates := preferredLanguages(request.Header.Get("Accept-Language"))
	for _, candidate := range candidates {
		if _, err := service.RequireEnabledLocale(ctx, candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, localizationservice.ErrInvalidLocale) &&
			!errors.Is(err, localizationservice.ErrLocaleNotFound) &&
			!errors.Is(err, localizationservice.ErrLocaleDisabled) {
			return "", err
		}
	}
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return "", nil
}

func localizationBundlePathLocale(path string) string {
	const prefix = "/api/v1/localization/bundles/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	value := strings.TrimPrefix(path, prefix)
	value = strings.TrimSuffix(value, "/meta")
	if value == "" || strings.Contains(value, "/") {
		return ""
	}
	return value
}

func localizationRolloutDomain(request *http.Request) string {
	if localizationBundlePathLocale(request.URL.Path) != "" {
		if value := strings.TrimSpace(request.URL.Query().Get("domain")); value != "" {
			if isLocalizationRolloutDomain(value) {
				return value
			}
			return "storefront"
		}
		if value := strings.TrimSpace(request.URL.Query().Get("namespace")); value != "" {
			if isLocalizationRolloutDomain(value) {
				return value
			}
			return "storefront"
		}
	}
	return localizationservice.DomainForPath(request.URL.Path)
}

func isLocalizationRolloutDomain(value string) bool {
	switch value {
	case "account", "admin", "checkout", "communications", "errors", "storefront":
		return true
	default:
		return false
	}
}

func requestMarket(request *http.Request) string {
	if value := strings.TrimSpace(request.URL.Query().Get("market")); value != "" {
		return strings.ToUpper(value)
	}
	if cookie, err := request.Cookie("market"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.ToUpper(strings.TrimSpace(cookie.Value))
	}
	return strings.ToUpper(strings.TrimSpace(request.Header.Get(MarketHeaderName)))
}

type weightedLanguage struct {
	value  string
	weight float64
	order  int
}

func preferredLanguages(header string) []string {
	values := make([]weightedLanguage, 0)
	for order, raw := range strings.Split(header, ",") {
		parts := strings.Split(raw, ";")
		value := strings.TrimSpace(parts[0])
		if value == "" || value == "*" {
			continue
		}
		weight := 1.0
		for _, parameter := range parts[1:] {
			name, rawValue, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(name, "q") {
				parsed, err := strconv.ParseFloat(rawValue, 64)
				if err != nil || parsed < 0 || parsed > 1 {
					weight = 0
				} else {
					weight = parsed
				}
			}
		}
		if _, err := language.Parse(value); err == nil && weight > 0 {
			values = append(values, weightedLanguage{value: value, weight: weight, order: order})
		}
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].weight == values[j].weight {
			return values[i].order < values[j].order
		}
		return values[i].weight > values[j].weight
	})
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.value)
	}
	return result
}
