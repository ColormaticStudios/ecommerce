package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"ecommerce/internal/apicontract"
)

func (e *CmsMediaEndpoints) CreateMediaUpload(ctx context.Context, request apicontract.CreateMediaUploadRequestObject) (apicontract.CreateMediaUploadResponseObject, error) {
	if e.media == nil {
		return nil, errors.New("media service is required")
	}
	if request.Params.UploadLength == nil {
		return nil, errors.New("Upload-Length is required")
	}
	if strings.TrimSpace(string(request.Params.TusResumable)) != "1.0.0" {
		return nil, errors.New("unsupported Tus-Resumable version")
	}
	metadata, err := parseUploadMetadata(request.Params.UploadMetadata)
	if err != nil {
		return nil, err
	}
	info, err := e.media.CreateUpload(ctx, int64(*request.Params.UploadLength), metadata, request.Body)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateMediaUpload201Response{Headers: apicontract.CreateMediaUpload201ResponseHeaders{
		Location: "/api/v1/media/uploads/" + info.ID, TusResumable: "1.0.0", UploadOffset: info.Offset,
	}}, nil
}

func (e *CmsMediaEndpoints) HeadMediaUpload(ctx context.Context, request apicontract.HeadMediaUploadRequestObject) (apicontract.HeadMediaUploadResponseObject, error) {
	if e.media == nil {
		return nil, errors.New("media service is required")
	}
	if string(request.Params.TusResumable) != "1.0.0" {
		return nil, errors.New("unsupported Tus-Resumable version")
	}
	info, err := e.media.HeadUpload(ctx, request.Path)
	if err != nil {
		return nil, err
	}
	return apicontract.HeadMediaUpload200Response{Headers: apicontract.HeadMediaUpload200ResponseHeaders{
		TusResumable: "1.0.0", UploadLength: info.Size, UploadOffset: info.Offset,
	}}, nil
}

func (e *CmsMediaEndpoints) PatchMediaUpload(ctx context.Context, request apicontract.PatchMediaUploadRequestObject) (apicontract.PatchMediaUploadResponseObject, error) {
	if e.media == nil {
		return nil, errors.New("media service is required")
	}
	if string(request.Params.TusResumable) != "1.0.0" {
		return nil, errors.New("unsupported Tus-Resumable version")
	}
	if request.Body == nil {
		return nil, errors.New("upload chunk body is required")
	}
	info, err := e.media.PatchUpload(ctx, request.Path, request.Params.UploadOffset, request.Body)
	if err != nil {
		return nil, err
	}
	return apicontract.PatchMediaUpload204Response{Headers: apicontract.PatchMediaUpload204ResponseHeaders{
		TusResumable: "1.0.0", UploadOffset: info.Offset,
	}}, nil
}

func parseUploadMetadata(raw *string) (map[string]string, error) {
	metadata := map[string]string{}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return metadata, nil
	}
	for _, item := range strings.Split(*raw, ",") {
		parts := strings.Fields(strings.TrimSpace(item))
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid Upload-Metadata item %q", item)
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode Upload-Metadata %q: %w", parts[0], err)
		}
		metadata[parts[0]] = string(decoded)
	}
	return metadata, nil
}
