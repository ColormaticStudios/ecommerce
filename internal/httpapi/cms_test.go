package httpapi_test

import (
	"context"
	"reflect"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewCmsMediaEndpointsRequiresDatabase(t *testing.T) {
	_, err := httpapi.NewCmsMediaEndpoints(nil, nil)
	require.Error(t, err)
}

func TestCmsMediaEndpointsPreviewUsesGeneratedRequestAndResponse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	endpoints, err := httpapi.NewCmsMediaEndpoints(db, nil)
	require.NoError(t, err)

	response, err := endpoints.PreviewAdminCmsPayload(context.Background(), apicontract.PreviewAdminCmsPayloadRequestObject{Body: &apicontract.CmsPreviewRequest{Payload: apicontract.CmsPagePayload{}}})
	require.NoError(t, err)
	preview, ok := response.(apicontract.PreviewAdminCmsPayload200JSONResponse)
	require.True(t, ok)
	assert.Empty(t, preview.Blocks)

	_, err = endpoints.PreviewAdminCmsPayload(context.Background(), apicontract.PreviewAdminCmsPayloadRequestObject{})
	require.Error(t, err)
}

func TestCmsMediaEndpointsCoversExactStrictFamily(t *testing.T) {
	operations := []string{
		"ListAdminCmsAuditEvents", "ResolveAdminCmsComment", "CreateAdminCmsEntryComment",
		"ListAdminCmsEntryVariants", "CreateAdminCmsEntryVariant", "DeleteAdminCmsEntryVariant", "UpdateAdminCmsEntryVariant", "TransitionAdminCmsEntryVariant",
		"GetAdminCmsEntryWorkflow", "TransitionAdminCmsEntryWorkflow", "ExportAdminCmsContent", "RestoreAdminCmsContent",
		"ListAdminCmsGlobalRegions", "CreateAdminCmsGlobalRegion", "DeleteAdminCmsGlobalRegion", "GetAdminCmsGlobalRegion", "UpdateAdminCmsGlobalRegion", "DiscardAdminCmsGlobalRegionDraft", "PublishAdminCmsGlobalRegion", "UnpublishAdminCmsGlobalRegion",
		"GetAdminCmsGovernance", "UpdateAdminCmsGovernance", "GetAdminCmsLocales", "UpdateAdminCmsLocales",
		"ListAdminCmsNavigation", "CreateAdminCmsNavigation", "DeleteAdminCmsNavigation", "GetAdminCmsNavigation", "UpdateAdminCmsNavigation", "DiscardAdminCmsNavigationDraft", "PublishAdminCmsNavigation", "UnpublishAdminCmsNavigation",
		"GetAdminCmsOperations", "RetryAdminCmsInvalidation",
		"ListAdminCmsPages", "CreateAdminCmsPage", "DeleteAdminCmsPage", "GetAdminCmsPage", "UpdateAdminCmsPage", "GetAdminCmsPageDelivery", "UpdateAdminCmsPageDelivery", "DiscardAdminCmsPageDraft", "PublishAdminCmsPage", "RollbackAdminCmsPage", "GetAdminCmsPageSeo", "UpdateAdminCmsPageSeo", "UnpublishAdminCmsPage",
		"ListAdminCmsPageVariants", "CreateAdminCmsPageVariant", "DeleteAdminCmsPageVariant", "UpdateAdminCmsPageVariant", "TransitionAdminCmsPageVariant",
		"PreviewAdminCmsPayload", "ListAdminCmsRedirects", "CreateAdminCmsRedirect", "DeleteAdminCmsRedirect", "UpdateAdminCmsRedirect", "PreviewAdminCmsRestore",
		"ResolveContentHomepage", "RecordContentEvent", "GetContentGlobalRegion", "GetContentNavigation", "ResolveContentRedirect", "GetContentSitemap", "ResolveContentPage",
		"CreateMediaUpload", "HeadMediaUpload", "PatchMediaUpload", "SetProfilePhoto", "DeleteProfilePhoto",
	}

	family := reflect.TypeOf((*httpapi.CmsMediaStrictServer)(nil)).Elem()
	endpoints := reflect.TypeOf((*httpapi.CmsMediaEndpoints)(nil))
	require.Equal(t, len(operations), family.NumMethod(), "operation inventory must be exhaustive and contain no unrelated strict operations")
	for _, operation := range operations {
		_, inFamily := family.MethodByName(operation)
		assert.Truef(t, inFamily, "CmsMediaStrictServer must include %s", operation)
		_, implemented := endpoints.MethodByName(operation)
		assert.Truef(t, implemented, "CmsMediaEndpoints must implement %s", operation)
	}
}
