package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/services/cms"
)

func (e *CmsMediaEndpoints) ListAdminCmsPages(ctx context.Context, request apicontract.ListAdminCmsPagesRequestObject) (apicontract.ListAdminCmsPagesResponseObject, error) {
	page, limit := cmsPagination(request.Params.Page, request.Params.Limit)
	records, total, err := e.pages.List(ctx, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsPageResponse, 0, len(records))
	for i := range records {
		value, err := contractPageRecord(&records[i], false)
		if err != nil {
			return nil, err
		}
		data = append(data, value)
	}
	return apicontract.ListAdminCmsPages200JSONResponse{Data: data, Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: totalPages(total, limit)}}, nil
}

func (e *CmsMediaEndpoints) CreateAdminCmsPage(ctx context.Context, request apicontract.CreateAdminCmsPageRequestObject) (apicontract.CreateAdminCmsPageResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := cmsPageInput(request.Body, actorID)
	if err != nil {
		return nil, problemError(400, "invalid_request", "The CMS page request is invalid.", err)
	}
	record, err := e.pages.CreateDraft(ctx, input)
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	value, err := contractPageRecord(record, false)
	return apicontract.CreateAdminCmsPage201JSONResponse(value), err
}

func (e *CmsMediaEndpoints) GetAdminCmsPage(ctx context.Context, request apicontract.GetAdminCmsPageRequestObject) (apicontract.GetAdminCmsPageResponseObject, error) {
	record, err := e.pages.Get(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	value, err := contractPageRecord(record, false)
	return apicontract.GetAdminCmsPage200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) UpdateAdminCmsPage(ctx context.Context, request apicontract.UpdateAdminCmsPageRequestObject) (apicontract.UpdateAdminCmsPageResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := cmsPageInput(request.Body, actorID)
	if err != nil {
		return nil, err
	}
	record, err := e.pages.UpdateDraft(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractPageRecord(record, false)
	return apicontract.UpdateAdminCmsPage200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) DeleteAdminCmsPage(ctx context.Context, request apicontract.DeleteAdminCmsPageRequestObject) (apicontract.DeleteAdminCmsPageResponseObject, error) {
	_, actorID := cmsActor(ctx)
	if err := e.pages.Delete(ctx, uint(request.Id), actorID); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsPage200JSONResponse{Message: "CMS page deleted"}, nil
}

func (e *CmsMediaEndpoints) PublishAdminCmsPage(ctx context.Context, request apicontract.PublishAdminCmsPageRequestObject) (apicontract.PublishAdminCmsPageResponseObject, error) {
	_, actorID := cmsActor(ctx)
	notes := ""
	if request.Body != nil && request.Body.Notes != nil {
		notes = *request.Body.Notes
	}
	record, err := e.pages.Publish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: notes})
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	value, err := contractPageRecord(record, false)
	return apicontract.PublishAdminCmsPage200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) UnpublishAdminCmsPage(ctx context.Context, request apicontract.UnpublishAdminCmsPageRequestObject) (apicontract.UnpublishAdminCmsPageResponseObject, error) {
	_, actorID := cmsActor(ctx)
	notes := ""
	if request.Body != nil && request.Body.Notes != nil {
		notes = *request.Body.Notes
	}
	record, err := e.pages.Unpublish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: notes})
	if err != nil {
		return nil, err
	}
	value, err := contractPageRecord(record, false)
	return apicontract.UnpublishAdminCmsPage200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) RollbackAdminCmsPage(ctx context.Context, request apicontract.RollbackAdminCmsPageRequestObject) (apicontract.RollbackAdminCmsPageResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(400, "invalid_request", "A CMS rollback body is required.", errors.New("CMS rollback body is required"))
	}
	_, actorID := cmsActor(ctx)
	notes := ""
	if request.Body.Notes != nil {
		notes = *request.Body.Notes
	}
	record, err := e.pages.Rollback(ctx, uint(request.Id), cms.RollbackInput{VersionID: uint(request.Body.VersionId), ActorID: actorID, Notes: notes})
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	value, err := contractPageRecord(record, false)
	return apicontract.RollbackAdminCmsPage200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) DiscardAdminCmsPageDraft(ctx context.Context, request apicontract.DiscardAdminCmsPageDraftRequestObject) (apicontract.DiscardAdminCmsPageDraftResponseObject, error) {
	_, actorID := cmsActor(ctx)
	record, deleted, err := e.pages.DiscardDraft(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID})
	if err != nil {
		return nil, err
	}
	if deleted {
		return apicontract.DiscardAdminCmsPageDraft204Response{}, nil
	}
	value, err := contractPageRecord(record, false)
	return apicontract.DiscardAdminCmsPageDraft200JSONResponse(value), err
}

func (e *CmsMediaEndpoints) ResolveContentHomepage(ctx context.Context, request apicontract.ResolveContentHomepageRequestObject) (apicontract.ResolveContentHomepageResponseObject, error) {
	value, preview, err := e.resolveContentPage(ctx, "/", stringParam(request.Params.Locale), stringParam(request.Params.Market), deviceString(request.Params.Device), stringParam(request.Params.Segment), stringParam(request.Params.UtmSource), stringParam(request.Params.AssignmentKey))
	if err != nil {
		return nil, err
	}
	if preview {
		return cmsHomepagePreviewResponse{body: value}, nil
	}
	return apicontract.ResolveContentHomepage200JSONResponse(value), nil
}

func (e *CmsMediaEndpoints) ResolveContentPage(ctx context.Context, request apicontract.ResolveContentPageRequestObject) (apicontract.ResolveContentPageResponseObject, error) {
	value, preview, err := e.resolveContentPage(ctx, request.Path, stringParam(request.Params.Locale), stringParam(request.Params.Market), deviceString(request.Params.Device), stringParam(request.Params.Segment), stringParam(request.Params.UtmSource), stringParam(request.Params.AssignmentKey))
	if err != nil {
		return nil, err
	}
	if preview {
		return cmsPagePreviewResponse{body: value}, nil
	}
	return apicontract.ResolveContentPage200JSONResponse(value), nil
}

func (e *CmsMediaEndpoints) resolveContentPage(ctx context.Context, path, locale, market, device, segment, utmSource, assignmentKey string) (apicontract.CmsPageResponse, bool, error) {
	preview := draftPreviewActive(ctx)
	service := e.pages
	record, localization, err := service.ResolveForLocale(ctx, path, locale, market, preview)
	if err != nil {
		return apicontract.CmsPageResponse{}, false, cmsEndpointError(err)
	}
	record.Localization = localization
	if !preview {
		decision, visible, err := service.ResolveDelivery(ctx, record, cms.RequestContext{Market: market, DeviceClass: device, SegmentKey: segment, UTMSource: utmSource, AssignmentKey: assignmentKey}, time.Now().UTC())
		if err != nil {
			return apicontract.CmsPageResponse{}, false, cmsEndpointError(err)
		}
		if !visible {
			return apicontract.CmsPageResponse{}, false, cmsEndpointError(cms.ErrNotFound)
		}
		record.Delivery = decision
	}
	value, err := contractPageRecord(record, true)
	if err != nil {
		return apicontract.CmsPageResponse{}, false, err
	}
	if preview {
		value.CurrentVersion, err = contractVersion(record.CurrentVersion)
		if err != nil {
			return apicontract.CmsPageResponse{}, false, err
		}
	}
	return value, preview, nil
}

type cmsHomepagePreviewResponse struct{ body apicontract.CmsPageResponse }

func (response cmsHomepagePreviewResponse) VisitResolveContentHomepageResponse(writer http.ResponseWriter) error {
	setCMSPreviewHeaders(writer)
	return writeJSONResponse(writer, http.StatusOK, response.body)
}

type cmsPagePreviewResponse struct{ body apicontract.CmsPageResponse }

func (response cmsPagePreviewResponse) VisitResolveContentPageResponse(writer http.ResponseWriter) error {
	setCMSPreviewHeaders(writer)
	return writeJSONResponse(writer, http.StatusOK, response.body)
}

func setCMSPreviewHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("X-Robots-Tag", "noindex")
	writer.Header().Set("Vary", "Cookie, Authorization")
}

func cmsPagination(pageParam, limitParam *int) (int, int) {
	page, limit := 1, 20
	if pageParam != nil && *pageParam > 0 {
		page = *pageParam
	}
	if limitParam != nil && *limitParam > 0 {
		limit = min(*limitParam, 100)
	}
	return page, limit
}

func totalPages(total int64, limit int) int {
	if total == 0 {
		return 0
	}
	return int((total + int64(limit) - 1) / int64(limit))
}

func stringParam(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
func deviceString[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
