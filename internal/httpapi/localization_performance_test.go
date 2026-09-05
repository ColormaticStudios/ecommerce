//go:build performance

package httpapi_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	localizationservice "ecommerce/internal/services/localization"

	"github.com/stretchr/testify/require"
)

func TestLocalBundleEndpointPerformanceBudget(t *testing.T) {
	db := newLocalizationEndpointTestDB(t)
	service := localizationservice.NewService(db)
	_, err := service.ReplaceLocales(context.Background(), []localizationservice.LocaleInput{{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}})
	require.NoError(t, err)
	release, err := service.CreateRelease(context.Background(), "performance", "")
	require.NoError(t, err)
	_, err = service.ActivateRelease(context.Background(), release.ID, nil)
	require.NoError(t, err)
	endpoints, err := httpapi.NewLocalizationEndpoints(db)
	require.NoError(t, err)
	durations := make([]time.Duration, 0, 50)
	for range 50 {
		started := time.Now()
		_, err = endpoints.GetLocalizationBundle(context.Background(), apicontract.GetLocalizationBundleRequestObject{Locale: "en-US", Params: apicontract.GetLocalizationBundleParams{}})
		require.NoError(t, err)
		durations = append(durations, time.Since(started))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[int(float64(len(durations))*0.95)]
	require.Less(t, p95, 150*time.Millisecond)
}
