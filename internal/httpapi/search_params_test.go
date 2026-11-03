package httpapi

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"ecommerce/internal/apicontract"

	"github.com/oapi-codegen/runtime"
	"github.com/stretchr/testify/require"
)

func TestParseSearchPriceRange(t *testing.T) {
	for _, input := range []string{"0:25.99", ":50", "100:", "0:0"} {
		value, err := parseSearchPriceRange(input)
		require.NoError(t, err, input)
		require.True(t, value.Min != nil || value.Max != nil)
	}
	for _, input := range []string{"", ":", "-1:2", "1.234:5", "1e2:200", "20:10", "NaN:10", " 1:2", "10000000000:"} {
		_, err := parseSearchPriceRange(input)
		require.Error(t, err, input)
	}
}

func TestGeneratedAttributeFilterBindingAcceptsMultipleValues(t *testing.T) {
	query := url.Values{
		"attribute[color][0]": {"red"},
		"attribute[color][1]": {"blue"},
		"attribute[size][0]":  {"large"},
	}
	var values *map[string][]string
	require.NoError(t, runtime.BindQueryParameter("deepObject", true, false, "attribute", query, &values))
	require.NotNil(t, values)
	require.Equal(t, map[string][]string{"color": {"red", "blue"}, "size": {"large"}}, *values)
}

func TestSearchProductParamsRejectAmbiguousOrOversizedFilters(t *testing.T) {
	longQuery := strings.Repeat("a", 201)
	tooManyBrands := make([]string, 21)
	for index := range tooManyBrands {
		tooManyBrands[index] = "north"
	}
	for _, params := range []apicontract.SearchProductsParams{
		{Q: &longQuery},
		{BrandSlug: &tooManyBrands},
		{CategorySlug: &[]string{"  "}},
		{Attribute: &map[string][]string{"color": {}}},
	} {
		require.Error(t, validateSearchProductParams(params))
	}
	valid := apicontract.SearchProductsParams{Attribute: &map[string][]string{"color": {"red", "blue"}}}
	require.NoError(t, validateSearchProductParams(valid))
	minimum := 10.0
	ranges := []string{"20:30"}
	_, err := (&CatalogEndpoints{}).SearchProducts(context.Background(), apicontract.SearchProductsRequestObject{
		Params: apicontract.SearchProductsParams{MinPrice: &minimum, PriceRange: &ranges},
	})
	require.Error(t, err)
}
