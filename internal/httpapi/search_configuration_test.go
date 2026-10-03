package httpapi_test

import (
	"context"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func searchConfigurationEndpoints(t *testing.T) (*httpapi.CatalogEndpoints, context.Context) {
	t.Helper()
	db := catalogTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}))
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{
		Subject: "admin@example.com", AccountID: 17, Roles: []string{"admin"},
	})
	return endpoints, ctx
}

func TestAdminSearchSynonymCRUD(t *testing.T) {
	endpoints, ctx := searchConfigurationEndpoints(t)
	inactive := false
	createdResponse, err := endpoints.CreateAdminSearchSynonym(ctx, apicontract.CreateAdminSearchSynonymRequestObject{
		Body: &apicontract.SearchSynonymSetInput{
			Name: "Footwear", Direction: apicontract.Uni,
			Terms: []string{"Sneakers", "Running Shoes"}, IsActive: &inactive,
		},
	})
	require.NoError(t, err)
	created := apicontract.SearchSynonymSet(createdResponse.(apicontract.CreateAdminSearchSynonym201JSONResponse))
	require.Equal(t, []string{"sneakers", "running shoes"}, created.Terms)
	require.False(t, created.IsActive)
	require.NotNil(t, created.UpdatedBy)
	require.Equal(t, 17, *created.UpdatedBy)

	active := true
	updatedResponse, err := endpoints.UpdateAdminSearchSynonym(ctx, apicontract.UpdateAdminSearchSynonymRequestObject{
		Id: created.Id, Body: &apicontract.SearchSynonymSetPatch{IsActive: &active},
	})
	require.NoError(t, err)
	updated := apicontract.SearchSynonymSet(updatedResponse.(apicontract.UpdateAdminSearchSynonym200JSONResponse))
	require.True(t, updated.IsActive)
	require.Equal(t, created.Name, updated.Name)
	require.Equal(t, created.Terms, updated.Terms)

	listedResponse, err := endpoints.ListAdminSearchSynonyms(ctx, apicontract.ListAdminSearchSynonymsRequestObject{})
	require.NoError(t, err)
	listed := apicontract.SearchSynonymSetListResponse(listedResponse.(apicontract.ListAdminSearchSynonyms200JSONResponse))
	require.Len(t, listed.Data, 1)

	deletedResponse, err := endpoints.DeleteAdminSearchSynonym(ctx, apicontract.DeleteAdminSearchSynonymRequestObject{Id: created.Id})
	require.NoError(t, err)
	require.IsType(t, apicontract.DeleteAdminSearchSynonym204Response{}, deletedResponse)
	_, err = endpoints.GetAdminSearchSynonym(ctx, apicontract.GetAdminSearchSynonymRequestObject{Id: created.Id})
	require.Error(t, err)
}

func TestAdminSearchTypoProfilesKeepOneActive(t *testing.T) {
	endpoints, ctx := searchConfigurationEndpoints(t)
	createdResponse, err := endpoints.CreateAdminSearchTypoProfile(ctx, apicontract.CreateAdminSearchTypoProfileRequestObject{
		Body: &apicontract.SearchTypoToleranceProfileInput{Name: "default"},
	})
	require.NoError(t, err)
	first := apicontract.SearchTypoToleranceProfile(createdResponse.(apicontract.CreateAdminSearchTypoProfile201JSONResponse))
	require.True(t, first.IsActive)
	require.Equal(t, 4, first.MinimumTokenLength)
	require.Equal(t, 4, first.OneEditMinimumLength)
	require.Equal(t, 8, first.TwoEditMinimumLength)

	active := true
	strict := true
	secondResponse, err := endpoints.CreateAdminSearchTypoProfile(ctx, apicontract.CreateAdminSearchTypoProfileRequestObject{
		Body: &apicontract.SearchTypoToleranceProfileInput{Name: "strict", IsActive: &active, StrictMode: &strict},
	})
	require.NoError(t, err)
	second := apicontract.SearchTypoToleranceProfile(secondResponse.(apicontract.CreateAdminSearchTypoProfile201JSONResponse))
	require.True(t, second.IsActive)
	require.True(t, second.StrictMode)

	firstResponse, err := endpoints.GetAdminSearchTypoProfile(ctx, apicontract.GetAdminSearchTypoProfileRequestObject{Id: first.Id})
	require.NoError(t, err)
	first = apicontract.SearchTypoToleranceProfile(firstResponse.(apicontract.GetAdminSearchTypoProfile200JSONResponse))
	require.False(t, first.IsActive)

	_, err = endpoints.DeleteAdminSearchTypoProfile(ctx, apicontract.DeleteAdminSearchTypoProfileRequestObject{Id: second.Id})
	require.Error(t, err)
	deletedResponse, err := endpoints.DeleteAdminSearchTypoProfile(ctx, apicontract.DeleteAdminSearchTypoProfileRequestObject{Id: first.Id})
	require.NoError(t, err)
	require.IsType(t, apicontract.DeleteAdminSearchTypoProfile204Response{}, deletedResponse)
}
