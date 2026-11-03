package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/jobs"
	catalogadmin "ecommerce/internal/services/catalogadmin"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func variantCostFixture() apicontract.Product {
	cost := 12.34
	zero := 0.0
	published := false
	defaultSKU := "OTHER"
	return apicontract.Product{Id: 7, Sku: "PRODUCT", Name: "Draft product", Description: "Keep description", Subtitle: stringPtr("Subtitle"), Images: []string{"https://example.com/image.png"}, Brand: &apicontract.Brand{Id: 3}, DefaultVariantSku: &defaultSKU, Variants: []apicontract.ProductVariant{{Sku: "TARGET", Title: "Target", Price: 30, Stock: 2, UnitCost: &cost, IsPublished: published, Position: 1, Selections: []apicontract.ProductVariantSelection{{OptionName: "Size", OptionValue: "M", Position: 1}}}, {Sku: "OTHER", Title: "Other", Price: 40, Stock: 4, UnitCost: &zero, IsPublished: true, Position: 2}}, Options: []apicontract.ProductOption{{Name: "Size", DisplayType: "select", Position: 1, Values: []apicontract.ProductOptionValue{{Value: "M", Position: 1}}}}, Attributes: []apicontract.ProductAttributeValue{{ProductAttributeId: 4, TextValue: stringPtr("Cotton"), Position: 1}}, Seo: apicontract.ProductSEO{Title: stringPtr("SEO title"), CanonicalPath: stringPtr("/keep")}, Categories: []apicontract.Category{{Id: 5}}, RelatedProducts: []apicontract.RelatedProduct{{Id: 8}}}
}

func TestVariantCostMutationsPreserveOtherFields(t *testing.T) {
	for _, args := range [][]string{{"--unit-cost", "0"}, {"--unit-cost", "15.25"}, {"--clear"}} {
		t.Run(args[0]+args[len(args)-1], func(t *testing.T) {
			original := variantCostFixture()
			calls := 0
			access := variantCostProductAccess{get: func(context.Context, int) (apicontract.Product, error) { return original, nil }, update: func(_ context.Context, id int, input apicontract.ProductUpsertInput) (apicontract.Product, error) {
				calls++
				require.Equal(t, 7, id)
				expected := productContractToUpsertInput(original)
				if args[0] == "--clear" {
					require.Nil(t, input.Variants[0].UnitCost)
				} else {
					require.NotNil(t, input.Variants[0].UnitCost)
					if args[1] == "0" {
						require.Zero(t, *input.Variants[0].UnitCost)
					} else {
						require.Equal(t, 15.25, *input.Variants[0].UnitCost)
					}
				}
				input.Variants[0].UnitCost = expected.Variants[0].UnitCost
				require.True(t, reflect.DeepEqual(expected, input), "partial edit lost catalog fields")
				return original, nil
			}}
			cmd := newVariantCostCmdWithAccess(access)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs(append([]string{"--id", "7", "--variant-sku", "TARGET"}, args...))
			require.NoError(t, cmd.Execute())
			require.Equal(t, 1, calls)
			require.Contains(t, output.String(), "Saved to draft")
		})
	}
}

func TestVariantCostRejectsInvalidArgumentsBeforeAccess(t *testing.T) {
	for _, args := range [][]string{{"--id", "0", "--variant-sku", "TARGET"}, {"--id", "7", "--variant-sku", " "}, {"--unit-cost", "-1"}, {"--unit-cost", "NaN"}, {"--unit-cost", "Inf"}, {"--unit-cost", "10000000000"}, {"--unit-cost", "0", "--clear"}, {"--unit-cost", "1", "--format", "xml"}, {"--clear=false"}, {"unexpected"}} {
		t.Run(stringJoinArgs(args), func(t *testing.T) {
			cmd := newVariantCostCmdWithAccess(variantCostProductAccess{get: func(context.Context, int) (apicontract.Product, error) {
				t.Fatal("invalid input accessed catalog")
				return apicontract.Product{}, nil
			}})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append([]string{"--id", "7", "--variant-sku", "TARGET"}, args...))
			require.Error(t, cmd.Execute())
		})
	}
}

func stringJoinArgs(args []string) string {
	result := ""
	for _, arg := range args {
		result += arg
	}
	return result
}

func TestVariantCostReadMissingAndError(t *testing.T) {
	for _, scenario := range []string{"read", "missing", "ambiguous", "get error", "update error"} {
		t.Run(scenario, func(t *testing.T) {
			product := variantCostFixture()
			updated := false
			if scenario == "ambiguous" {
				product.Variants = append(product.Variants, product.Variants[0])
			}
			cmd := newVariantCostCmdWithAccess(variantCostProductAccess{get: func(context.Context, int) (apicontract.Product, error) {
				if scenario == "get error" {
					return product, errors.New("get failed")
				}
				return product, nil
			}, update: func(context.Context, int, apicontract.ProductUpsertInput) (apicontract.Product, error) {
				updated = true
				return product, errors.New("update failed")
			}})
			args := []string{"--id", "7", "--variant-sku", "TARGET", "--format", "json"}
			if scenario == "missing" {
				args[3] = "UNKNOWN"
			}
			if scenario == "update error" {
				args = append(args, "--clear")
			}
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(args)
			err := cmd.Execute()
			if scenario == "read" {
				require.NoError(t, err)
				var result apicontract.Product
				require.NoError(t, json.Unmarshal(output.Bytes(), &result))
				require.Equal(t, 12.34, *result.Variants[0].UnitCost)
			} else {
				require.Error(t, err)
				require.Empty(t, output.String())
			}
			require.Equal(t, scenario == "update error", updated)
		})
	}
}

func TestVariantCostRemoteUsesAdminProductAndPreservesPayload(t *testing.T) {
	old := activeCLIRuntime
	t.Cleanup(func() { activeCLIRuntime = old })
	product := variantCostFixture()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "/api/v1/admin/products/7", r.URL.Path)
		calls++
		if r.Method == http.MethodPatch {
			var input apicontract.ProductUpsertInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			require.Zero(t, *input.Variants[0].UnitCost)
			expected := productContractToUpsertInput(product)
			input.Variants[0].UnitCost = expected.Variants[0].UnitCost
			require.Equal(t, expected, input)
			zero := 0.0
			product.Variants[0].UnitCost = &zero
		} else {
			require.Equal(t, http.MethodGet, r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(product))
	}))
	defer server.Close()
	activeCLIRuntime = cliRuntime{Remote: &persistentCLIAuth{APIURL: server.URL, Token: "test-token"}}
	cmd := newVariantCostCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--id", "7", "--variant-sku", "TARGET", "--unit-cost", "0", "--format", "json"})
	require.NoError(t, cmd.Execute())
	require.Equal(t, 2, calls)
	require.Contains(t, output.String(), `"unit_cost": 0`)
}

func TestVariantCostLocalDraftPublishLifecycle(t *testing.T) {
	db := newTestDB(t, &models.Locale{}, &models.LocalizedEntityValue{}, &models.Product{}, &models.ProductVariant{}, &models.ProductDraft{}, &models.ProductVariantDraft{}, &models.ProductRelatedDraft{}, &models.ProductCategory{}, &models.ProductCategoryDraft{}, &models.ProductAttributeValueDraft{}, &models.ProductOptionDraft{}, &models.ProductOptionValueDraft{}, &models.ProductVariantOptionValueDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SEOMetadata{}, &models.MediaReference{}, &models.Brand{}, &models.Category{}, &models.JobQueue{})
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	service := catalogadmin.NewService(db, nil, runtime)
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil, runtime)
	require.NoError(t, err)
	attribute := models.ProductAttribute{Key: "material", Slug: "material", Type: "text"}
	require.NoError(t, db.Create(&attribute).Error)
	brand := models.Brand{Name: "Acme", Slug: "acme", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	input := productContractToUpsertInput(variantCostFixture())
	input.BrandId = new(int)
	*input.BrandId = int(brand.ID)
	input.Attributes[0].ProductAttributeId = int(attribute.ID)
	input.CategoryIds = nil
	input.RelatedProductIds = nil
	product, err := service.CreateProduct(context.Background(), input)
	require.NoError(t, err)
	access := variantCostProductAccess{get: func(ctx context.Context, id int) (apicontract.Product, error) {
		result, err := endpoints.GetAdminProduct(ctx, apicontract.GetAdminProductRequestObject{Id: id})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(result.(apicontract.GetAdminProduct200JSONResponse)), nil
	}, update: func(ctx context.Context, id int, input apicontract.ProductUpsertInput) (apicontract.Product, error) {
		result, err := endpoints.UpdateProduct(ctx, apicontract.UpdateProductRequestObject{Id: id, Body: &input})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(result.(apicontract.UpdateProduct200JSONResponse)), nil
	}}
	before, err := access.get(context.Background(), int(product.ID))
	require.NoError(t, err)
	for _, args := range [][]string{{"--unit-cost", "0"}, {"--clear"}, {"--unit-cost", "18.50"}} {
		cmd := newVariantCostCmdWithAccess(access)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs(append([]string{"--id", fmt.Sprint(product.ID), "--variant-sku", "TARGET"}, args...))
		require.NoError(t, cmd.Execute())
		after, err := access.get(context.Background(), int(product.ID))
		require.NoError(t, err)
		original := productContractToUpsertInput(before)
		current := productContractToUpsertInput(after)
		current.Variants[0].UnitCost = original.Variants[0].UnitCost
		require.Equal(t, original, current)
	}
	_, err = service.PublishProduct(context.Background(), product.ID)
	require.NoError(t, err)
	live, err := service.ProductInput(context.Background(), product.ID)
	require.NoError(t, err)
	require.Len(t, live.Options, 1)
	require.Len(t, live.Attributes, 1)
	require.Len(t, live.Variants[0].Selections, 1)
	require.Equal(t, "SEO title", *live.Seo.Title)
	require.Equal(t, "OTHER", *live.DefaultVariantSku)
	cmd := newVariantCostCmdWithAccess(access)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--id", fmt.Sprint(product.ID), "--variant-sku", "TARGET", "--clear"})
	require.NoError(t, cmd.Execute())
	published, err := catalogadmin.LoadLiveProductUpsertInput(db, nil, product.ID)
	require.NoError(t, err)
	require.Equal(t, 18.50, *published.Variants[0].UnitCost, "editing a cost must leave published state intact")
	_, err = service.PublishProduct(context.Background(), product.ID)
	require.NoError(t, err)
	live, err = service.ProductInput(context.Background(), product.ID)
	require.NoError(t, err)
	require.Nil(t, live.Variants[0].UnitCost)
	require.Len(t, live.Variants[0].Selections, 1)
	require.Len(t, live.Options, 1)
	require.Len(t, live.Attributes, 1)
	_, err = service.UnpublishProduct(context.Background(), product.ID)
	require.NoError(t, err)
	draft, err := service.ProductInput(context.Background(), product.ID)
	require.NoError(t, err)
	require.Equal(t, live.Options, draft.Options)
	require.Equal(t, live.Attributes, draft.Attributes)
	require.Equal(t, live.Seo, draft.Seo)
}
