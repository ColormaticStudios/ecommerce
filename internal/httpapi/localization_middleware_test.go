package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLocalizationNegotiationMiddlewareUsesDocumentedPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: true, FallbackLocale: "en-US", DefaultMarkets: []string{"CA"}},
		{Code: "fr-CA", Name: "French (Canada)", IsEnabled: true, FallbackLocale: "fr"},
	})
	require.NoError(t, err)

	negotiation, err := httpapi.LocalizationNegotiationMiddleware(httpapi.LocalizationNegotiationOptions{
		Service: service,
		ResolveAccountPreference: func(_ context.Context, principal requestctx.Principal) (string, error) {
			if principal.Subject == "account" {
				return "fr-CA", nil
			}
			return "", nil
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		path          string
		headers       map[string]string
		cookies       []*http.Cookie
		principal     bool
		wantLocale    string
		wantSource    string
		wantMarket    string
		wantRequested string
		wantFallback  bool
	}{
		{name: "global default", path: "/", wantLocale: "en-US", wantSource: "global", wantRequested: "en-US"},
		{name: "market default", path: "/?market=ca", wantLocale: "fr", wantSource: "market", wantMarket: "CA", wantRequested: "fr"},
		{name: "account preference", path: "/?market=CA", principal: true, wantLocale: "fr-CA", wantSource: "account", wantMarket: "CA", wantRequested: "fr-CA"},
		{name: "weighted language header", path: "/", headers: map[string]string{"Accept-Language": "en-US;q=0.3, fr-CA;q=0.9"}, principal: true, wantLocale: "fr-CA", wantSource: "explicit", wantRequested: "fr-CA"},
		{name: "language header skips unavailable preference", path: "/", headers: map[string]string{"Accept-Language": "de-DE;q=1, fr;q=0.8"}, wantLocale: "fr", wantSource: "explicit", wantRequested: "fr"},
		{name: "locale header", path: "/", headers: map[string]string{httpapi.LocaleHeaderName: "fr"}, wantLocale: "fr", wantSource: "explicit", wantRequested: "fr"},
		{name: "cookie overrides header", path: "/", headers: map[string]string{httpapi.LocaleHeaderName: "en-US"}, cookies: []*http.Cookie{{Name: httpapi.LocaleCookieName, Value: "fr-CA"}}, wantLocale: "fr-CA", wantSource: "explicit", wantRequested: "fr-CA"},
		{name: "query overrides cookie", path: "/?locale=fr", cookies: []*http.Cookie{{Name: httpapi.LocaleCookieName, Value: "en-US"}}, wantLocale: "fr", wantSource: "explicit", wantRequested: "fr"},
		{name: "unavailable explicit falls back to account", path: "/?locale=de-DE", principal: true, wantLocale: "fr-CA", wantSource: "account", wantRequested: "de-DE", wantFallback: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			if test.principal {
				router.Use(func(ctx *gin.Context) {
					ctx.Request = ctx.Request.WithContext(requestctx.WithPrincipal(ctx.Request.Context(), requestctx.Principal{Subject: "account"}))
					ctx.Next()
				})
			}
			router.Use(negotiation)
			router.GET("/", func(ctx *gin.Context) {
				resolution, ok := requestctx.LocaleResolutionFrom(ctx.Request.Context())
				require.True(t, ok)
				ctx.JSON(http.StatusOK, resolution)
			})

			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			for _, cookie := range test.cookies {
				request.AddCookie(cookie)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, test.wantLocale, recorder.Header().Get("Content-Language"))
			var resolution requestctx.LocaleResolution
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resolution))
			require.Equal(t, test.wantLocale, resolution.ResolvedLocale)
			require.Equal(t, test.wantSource, resolution.Source)
			require.Equal(t, test.wantMarket, resolution.Market)
			require.Equal(t, test.wantRequested, resolution.RequestedLocale)
			require.Equal(t, test.wantFallback, resolution.UsedFallback)
		})
	}
}

func TestLocalizationNegotiationMiddlewareRejectsInvalidMarket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
	})
	require.NoError(t, err)
	negotiation, err := httpapi.LocalizationNegotiationMiddleware(httpapi.LocalizationNegotiationOptions{Service: service})
	require.NoError(t, err)

	router := gin.New()
	router.Use(negotiation)
	router.GET("/", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/?market=invalid", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	require.Equal(t, "invalid_market", problem.Code)
	require.Equal(t, "errors.invalid_market", problem.MessageKey)
}

func TestLocalizationNegotiationMiddlewareIssuesStableAnonymousRolloutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
	})
	require.NoError(t, err)
	negotiation, err := httpapi.LocalizationNegotiationMiddleware(httpapi.LocalizationNegotiationOptions{Service: service})
	require.NoError(t, err)

	router := gin.New()
	router.Use(negotiation)
	router.GET("/", func(ctx *gin.Context) {
		metadata, ok := requestctx.MetadataFrom(ctx.Request.Context())
		require.True(t, ok)
		require.NotEmpty(t, metadata.Cookies[httpapi.LocalizationRolloutCookieName])
		ctx.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, httpapi.LocalizationRolloutCookieName, cookies[0].Name)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	require.Equal(t, 365*24*60*60, cookies[0].MaxAge)
}

func TestLocalizationNegotiationMiddlewareUsesBundleLocaleAndDomainRollout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: true, FallbackLocale: "en-US"},
	})
	require.NoError(t, err)
	rollouts, err := service.ListRollouts(context.Background())
	require.NoError(t, err)
	inputs := make([]localizationservice.RolloutInput, 0, len(rollouts))
	for _, rollout := range rollouts {
		input := localizationservice.RolloutInput{
			Locale: rollout.Locale, Domain: rollout.Domain,
			IsEnabled: rollout.IsEnabled, Percentage: rollout.Percentage,
		}
		if rollout.Locale == "fr" && rollout.Domain == "communications" {
			input.IsEnabled = false
			input.Percentage = 0
		}
		inputs = append(inputs, input)
	}
	_, err = service.ReplaceRollouts(context.Background(), inputs, nil)
	require.NoError(t, err)
	negotiation, err := httpapi.LocalizationNegotiationMiddleware(httpapi.LocalizationNegotiationOptions{Service: service})
	require.NoError(t, err)

	router := gin.New()
	router.Use(negotiation)
	router.GET("/api/v1/localization/bundles/:locale", func(ctx *gin.Context) {
		resolution, ok := requestctx.LocaleResolutionFrom(ctx.Request.Context())
		require.True(t, ok)
		ctx.JSON(http.StatusOK, resolution)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/localization/bundles/fr?domain=communications", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "en-US", recorder.Header().Get("Content-Language"))
	var resolution requestctx.LocaleResolution
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resolution))
	require.Equal(t, "fr", resolution.RequestedLocale)
	require.Equal(t, "en-US", resolution.ResolvedLocale)
	require.True(t, resolution.UsedFallback)
}
