package httpapi_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLocalizationEndpointTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Locale{}, &models.LocaleMarketDefault{}, &models.TranslationKey{}, &models.TranslationValue{},
		&models.TranslationRelease{}, &models.TranslationReleaseEntry{},
		&models.LocalizedEntityValue{}, &models.TranslationComment{}, &models.TranslationAuditEvent{},
		&models.LocalizationGlossaryTerm{}, &models.LocalizationRoleAssignment{},
		&models.LocalizationRollout{}, &models.LocalizationMetric{},
		&models.User{},
	))
	return db
}

func TestNewLocalizationEndpointsRequiresDatabase(t *testing.T) {
	_, err := httpapi.NewLocalizationEndpoints(nil)
	require.Error(t, err)
}

func TestLocalizationEndpointsReturnGeneratedLocaleAndBundleContracts(t *testing.T) {
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: true, FallbackLocale: "en-US"},
	})
	require.NoError(t, err)
	_, err = service.CreateKey(context.Background(), localizationservice.KeyInput{
		Namespace: "storefront", Key: "welcome.title", SourceText: "Welcome", OwnerDomain: "storefront",
	})
	require.NoError(t, err)
	release, err := service.CreateRelease(context.Background(), "test", "")
	require.NoError(t, err)
	_, err = service.ActivateRelease(context.Background(), release.ID, nil)
	require.NoError(t, err)
	endpoints, err := httpapi.NewLocalizationEndpoints(db)
	require.NoError(t, err)

	localesResponse, err := endpoints.ListLocalizationLocales(context.Background(), apicontract.ListLocalizationLocalesRequestObject{})
	require.NoError(t, err)
	locales, ok := localesResponse.(apicontract.ListLocalizationLocales200JSONResponse)
	require.True(t, ok)
	require.Equal(t, "en-US", locales.DefaultLocale)
	require.Len(t, locales.Locales, 2)
	for _, locale := range locales.Locales {
		require.NotNil(t, locale.DefaultForMarkets)
	}

	bundleResponse, err := endpoints.GetLocalizationBundle(context.Background(), apicontract.GetLocalizationBundleRequestObject{
		Locale: "fr", Params: apicontract.GetLocalizationBundleParams{},
	})
	require.NoError(t, err)
	bundle, ok := bundleResponse.(apicontract.GetLocalizationBundle200JSONResponse)
	require.True(t, ok)
	require.Equal(t, "fr", bundle.Body.Resolution.ResolvedLocale)
	require.Equal(t, "Welcome", bundle.Body.Messages["storefront.welcome.title"].Value)
	require.True(t, bundle.Body.Messages["storefront.welcome.title"].MissingTranslation)
	require.NotEmpty(t, bundle.Body.Release.Version)
	require.Equal(t, `"`+bundle.Body.Release.Version+`"`, bundle.Headers.ETag)
	conditionalResponse, err := endpoints.GetLocalizationBundle(context.Background(), apicontract.GetLocalizationBundleRequestObject{
		Locale: "fr", Params: apicontract.GetLocalizationBundleParams{IfNoneMatch: &bundle.Headers.ETag},
	})
	require.NoError(t, err)
	conditional, ok := conditionalResponse.(apicontract.GetLocalizationBundle304Response)
	require.True(t, ok)
	require.Equal(t, bundle.Headers.ETag, conditional.Headers.ETag)
}

func TestLocalizationEndpointsRejectMalformedLocale(t *testing.T) {
	endpoints, err := httpapi.NewLocalizationEndpoints(newLocalizationEndpointTestDB(t))
	require.NoError(t, err)
	_, err = endpoints.GetLocalizationBundle(context.Background(), apicontract.GetLocalizationBundleRequestObject{
		Locale: "not_a_locale", Params: apicontract.GetLocalizationBundleParams{},
	})
	require.Error(t, err)
	var problem *httpapi.ProblemError
	require.ErrorAs(t, err, &problem)
	require.Equal(t, 400, problem.Problem.Status)
}

func TestAdminLocalizationKeysReturnSearchedPagination(t *testing.T) {
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
	})
	require.NoError(t, err)
	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{
		Subject: "admin@example.com", AccountID: 17, Roles: []string{"admin"},
	})
	for _, key := range []string{"catalog.alpha", "catalog.beta", "catalog.gamma"} {
		_, err := service.CreateKey(ctx, localizationservice.KeyInput{
			Namespace: "storefront", Key: key, SourceText: key, OwnerDomain: "catalog",
		})
		require.NoError(t, err)
	}
	endpoints, err := httpapi.NewLocalizationEndpoints(db)
	require.NoError(t, err)
	page, limit, query := 2, 1, "storefront.catalog"

	response, err := endpoints.ListAdminLocalizationKeys(ctx, apicontract.ListAdminLocalizationKeysRequestObject{
		Params: apicontract.ListAdminLocalizationKeysParams{Page: &page, Limit: &limit, Q: &query},
	})
	require.NoError(t, err)
	payload, ok := response.(apicontract.ListAdminLocalizationKeys200JSONResponse)
	require.True(t, ok)
	require.Equal(t, apicontract.Pagination{Page: 2, Limit: 1, Total: 3, TotalPages: 3}, payload.Pagination)
	require.Len(t, payload.Items, 1)
	require.Equal(t, "catalog.beta", payload.Items[0].Key.Key)
}

func TestAdminLocalizationRolloutAndMetricsEndpoints(t *testing.T) {
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: true, FallbackLocale: "en-US"},
	})
	require.NoError(t, err)
	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{
		Subject: "admin@example.com", AccountID: 17, Roles: []string{"admin"},
	})
	endpoints, err := httpapi.NewLocalizationEndpoints(db)
	require.NoError(t, err)

	listResponse, err := endpoints.ListAdminLocalizationRollouts(ctx, apicontract.ListAdminLocalizationRolloutsRequestObject{})
	require.NoError(t, err)
	listed, ok := listResponse.(apicontract.ListAdminLocalizationRollouts200JSONResponse)
	require.True(t, ok)
	require.Len(t, listed.Rollouts, len(localizationservice.RolloutDomains())*2)

	inputs := make([]apicontract.LocalizationRolloutInput, 0, len(listed.Rollouts))
	for _, rollout := range listed.Rollouts {
		input := apicontract.LocalizationRolloutInput{
			Locale: rollout.Locale, Domain: rollout.Domain,
			IsEnabled: rollout.IsEnabled, Percentage: rollout.Percentage,
		}
		if rollout.Locale == "fr" && rollout.Domain == apicontract.LocalizationRolloutDomain("checkout") {
			input.IsEnabled = false
			input.Percentage = 25
		}
		inputs = append(inputs, input)
	}
	replaceResponse, err := endpoints.ReplaceAdminLocalizationRollouts(ctx, apicontract.ReplaceAdminLocalizationRolloutsRequestObject{
		Body: &apicontract.LocalizationRolloutSettingsInput{Rollouts: inputs},
	})
	require.NoError(t, err)
	replaced, ok := replaceResponse.(apicontract.ReplaceAdminLocalizationRollouts200JSONResponse)
	require.True(t, ok)
	var checkoutRollout *apicontract.LocalizationRollout
	for index := range replaced.Rollouts {
		if replaced.Rollouts[index].Locale == "fr" && replaced.Rollouts[index].Domain == apicontract.LocalizationRolloutDomain("checkout") {
			checkoutRollout = &replaced.Rollouts[index]
			break
		}
	}
	require.NotNil(t, checkoutRollout)
	require.False(t, checkoutRollout.IsEnabled)
	require.Equal(t, 25, checkoutRollout.Percentage)

	require.NoError(t, service.RecordMetric(ctx, localizationservice.MetricLookup, "fr", "checkout", "checkout.payment", 4, 0))
	require.NoError(t, service.RecordMetric(ctx, localizationservice.MetricMissingKey, "fr", "checkout", "checkout.payment", 3, 0))
	require.NoError(t, service.RecordMetric(ctx, localizationservice.MetricFallbackHit, "fr", "checkout", "checkout.payment", 2, 0))
	metricsResponse, err := endpoints.GetAdminLocalizationMetrics(ctx, apicontract.GetAdminLocalizationMetricsRequestObject{})
	require.NoError(t, err)
	metrics, ok := metricsResponse.(apicontract.GetAdminLocalizationMetrics200JSONResponse)
	require.True(t, ok)
	require.Equal(t, 4, metrics.LookupCount)
	require.Equal(t, 3, metrics.MissingKeyCount)
	require.InDelta(t, 0.75, metrics.MissingKeyRate, 0.001)
	require.InDelta(t, 0.5, metrics.FallbackHitRate, 0.001)
	require.Len(t, metrics.LocaleRates, 1)
	require.Equal(t, "checkout.payment", metrics.Hotspots[0].Key)
}

func TestLocalizationEndpointsCoverExactStrictFamily(t *testing.T) {
	operations := []string{
		"ListLocalizationLocales", "GetLocalizationBundle", "GetLocalizationBundleMeta",
		"ListAdminLocalizationLocales", "ReplaceAdminLocalizationLocales",
		"ListAdminLocalizationKeys", "CreateAdminLocalizationKey", "ListAdminLocalizationValues", "PutAdminLocalizationValue",
		"ListAdminLocalizationKeyUsages", "ReplaceAdminLocalizationKeyUsages",
		"SubmitAdminLocalizationValueReview", "PublishAdminLocalizationValue", "ListAdminLocalizationComments", "CreateAdminLocalizationComment",
		"ListAdminLocalizationReleases", "CreateAdminLocalizationRelease", "ActivateAdminLocalizationRelease",
		"GetAdminLocalizationReleaseQuality", "RollbackAdminLocalizationRelease",
		"ImportAdminLocalization", "ExportAdminLocalization", "ListAdminLocalizationGlossary", "PutAdminLocalizationGlossaryTerm",
		"DeleteAdminLocalizationGlossaryTerm", "ListAdminLocalizationRoles", "PutAdminLocalizationRole",
		"ListAdminLocalizationAssignees",
		"ListAdminLocalizationRollouts", "ReplaceAdminLocalizationRollouts", "GetAdminLocalizationMetrics",
		"GetAdminEntityLocalization", "PutAdminEntityLocalization",
	}
	family := reflect.TypeOf((*httpapi.LocalizationStrictServer)(nil)).Elem()
	endpoints := reflect.TypeOf((*httpapi.LocalizationEndpoints)(nil))
	require.Equal(t, len(operations), family.NumMethod())
	for _, operation := range operations {
		_, inFamily := family.MethodByName(operation)
		require.Truef(t, inFamily, "LocalizationStrictServer must include %s", operation)
		_, implemented := endpoints.MethodByName(operation)
		require.Truef(t, implemented, "LocalizationEndpoints must implement %s", operation)
	}
}
