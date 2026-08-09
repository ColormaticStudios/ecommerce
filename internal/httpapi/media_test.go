package httpapi

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/apicontract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUploadMetadataAtEndpointBoundary(t *testing.T) {
	filename := base64.StdEncoding.EncodeToString([]byte("hero image.webp"))
	kind := base64.StdEncoding.EncodeToString([]byte("cms"))
	raw := "filename " + filename + ",kind " + kind

	metadata, err := parseUploadMetadata(&raw)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"filename": "hero image.webp", "kind": "cms"}, metadata)
}

func TestGeneratedTUSResponsesWriteRequiredResumableHeaders(t *testing.T) {
	createRecorder := httptest.NewRecorder()
	create := apicontract.CreateMediaUpload201Response{Headers: apicontract.CreateMediaUpload201ResponseHeaders{
		Location: "/api/v1/media/uploads/upload-1", TusResumable: "1.0.0", UploadOffset: 17,
	}}
	require.NoError(t, create.VisitCreateMediaUploadResponse(createRecorder))
	assert.Equal(t, http.StatusCreated, createRecorder.Code)
	assert.Equal(t, "/api/v1/media/uploads/upload-1", createRecorder.Header().Get("Location"))
	assert.Equal(t, "1.0.0", createRecorder.Header().Get("Tus-Resumable"))
	assert.Equal(t, "17", createRecorder.Header().Get("Upload-Offset"))

	headRecorder := httptest.NewRecorder()
	head := apicontract.HeadMediaUpload200Response{Headers: apicontract.HeadMediaUpload200ResponseHeaders{
		TusResumable: "1.0.0", UploadLength: 100, UploadOffset: 17,
	}}
	require.NoError(t, head.VisitHeadMediaUploadResponse(headRecorder))
	assert.Equal(t, http.StatusOK, headRecorder.Code)
	assert.Equal(t, "1.0.0", headRecorder.Header().Get("Tus-Resumable"))
	assert.Equal(t, "100", headRecorder.Header().Get("Upload-Length"))
	assert.Equal(t, "17", headRecorder.Header().Get("Upload-Offset"))

	patchRecorder := httptest.NewRecorder()
	patch := apicontract.PatchMediaUpload204Response{Headers: apicontract.PatchMediaUpload204ResponseHeaders{
		TusResumable: "1.0.0", UploadOffset: 42,
	}}
	require.NoError(t, patch.VisitPatchMediaUploadResponse(patchRecorder))
	assert.Equal(t, http.StatusNoContent, patchRecorder.Code)
	assert.Equal(t, "1.0.0", patchRecorder.Header().Get("Tus-Resumable"))
	assert.Equal(t, "42", patchRecorder.Header().Get("Upload-Offset"))
}

func TestParseUploadMetadataRejectsMalformedValues(t *testing.T) {
	for _, raw := range []string{"filename", "filename !!!", " filename "} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseUploadMetadata(&raw)
			require.Error(t, err)
		})
	}
}
