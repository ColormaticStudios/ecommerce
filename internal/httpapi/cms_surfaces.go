package httpapi

import (
	"context"
	"errors"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/services/cms"
	"ecommerce/models"
)

func navigationInput(body *apicontract.CmsNavigationDraftRequest, actorID *uint) (cms.NavigationDraftInput, error) {
	if body == nil {
		return cms.NavigationDraftInput{}, errors.New("CMS navigation body is required")
	}
	items := make([]cms.NavigationItemInput, 0, len(body.Items))
	for _, item := range body.Items {
		input := cms.NavigationItemInput{Label: item.Label, ItemType: string(item.ItemType), TargetRef: item.TargetRef, URL: item.Url, SortOrder: item.SortOrder, IsEnabled: item.IsEnabled}
		if item.Id != nil {
			input.ID = uint(*item.Id)
		}
		if item.ParentId != nil {
			id := uint(*item.ParentId)
			input.ParentID = &id
		}
		items = append(items, input)
	}
	input := cms.NavigationDraftInput{Key: body.Key, Title: body.Title, Location: body.Location, Items: items, ActorID: actorID}
	if body.ChangeSummary != nil {
		input.ChangeSummary = *body.ChangeSummary
	}
	return input, nil
}

func contractNavigation(record *cms.NavigationRecord, public bool) (apicontract.CmsNavigationResponse, error) {
	current, err := contractVersion(record.CurrentVersion)
	if err != nil {
		return apicontract.CmsNavigationResponse{}, err
	}
	published, err := contractVersion(record.PublishedVersion)
	if err != nil {
		return apicontract.CmsNavigationResponse{}, err
	}
	items := make([]apicontract.CmsNavigationItem, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, apicontract.CmsNavigationItem{Id: int(item.ID), MenuId: int(item.MenuID), ParentId: uintPointerToInt(item.ParentID), Label: item.Label, ItemType: apicontract.CmsNavigationItemItemType(item.ItemType), TargetRef: item.TargetRef, Url: item.URL, SortOrder: item.SortOrder, IsEnabled: item.IsEnabled})
	}
	result := apicontract.CmsNavigationResponse{Menu: apicontract.CmsNavigationMenu{Id: int(record.Menu.ID), EntryId: int(record.Menu.EntryID), Key: record.Menu.Key, Title: record.Menu.Title, Location: record.Menu.Location, CreatedAt: record.Menu.CreatedAt, UpdatedAt: record.Menu.UpdatedAt}, Entry: contractEntry(record.Entry), Items: items, HasUnpublishedDraft: record.HasUnpublishedDraft}
	if public {
		result.Entry.CurrentVersionId, result.Entry.PublishedVersionId, result.HasUnpublishedDraft, result.PublishedVersion = nil, nil, false, published
	} else {
		result.CurrentVersion, result.PublishedVersion, result.LatestPublication = current, published, contractPublication(record.LatestPublication)
	}
	return result, nil
}

func (e *CmsMediaEndpoints) ListAdminCmsNavigation(ctx context.Context, request apicontract.ListAdminCmsNavigationRequestObject) (apicontract.ListAdminCmsNavigationResponseObject, error) {
	page, limit := cmsPagination(request.Params.Page, request.Params.Limit)
	records, total, err := e.navigation.List(ctx, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsNavigationResponse, 0, len(records))
	for i := range records {
		value, err := contractNavigation(&records[i], false)
		if err != nil {
			return nil, err
		}
		data = append(data, value)
	}
	return apicontract.ListAdminCmsNavigation200JSONResponse{Data: data, Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: totalPages(total, limit)}}, nil
}

func (e *CmsMediaEndpoints) CreateAdminCmsNavigation(ctx context.Context, request apicontract.CreateAdminCmsNavigationRequestObject) (apicontract.CreateAdminCmsNavigationResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := navigationInput(request.Body, actorID)
	if err != nil {
		return nil, err
	}
	record, err := e.navigation.CreateDraft(ctx, input)
	if err != nil {
		return nil, err
	}
	value, err := contractNavigation(record, false)
	return apicontract.CreateAdminCmsNavigation201JSONResponse(value), err
}
func (e *CmsMediaEndpoints) GetAdminCmsNavigation(ctx context.Context, request apicontract.GetAdminCmsNavigationRequestObject) (apicontract.GetAdminCmsNavigationResponseObject, error) {
	record, err := e.navigation.Get(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	value, err := contractNavigation(record, false)
	return apicontract.GetAdminCmsNavigation200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsNavigation(ctx context.Context, request apicontract.UpdateAdminCmsNavigationRequestObject) (apicontract.UpdateAdminCmsNavigationResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := navigationInput(request.Body, actorID)
	if err != nil {
		return nil, err
	}
	record, err := e.navigation.UpdateDraft(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractNavigation(record, false)
	return apicontract.UpdateAdminCmsNavigation200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DeleteAdminCmsNavigation(ctx context.Context, request apicontract.DeleteAdminCmsNavigationRequestObject) (apicontract.DeleteAdminCmsNavigationResponseObject, error) {
	_, actorID := cmsActor(ctx)
	if err := e.navigation.Delete(ctx, uint(request.Id), actorID); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsNavigation200JSONResponse{Message: "CMS navigation deleted"}, nil
}
func (e *CmsMediaEndpoints) PublishAdminCmsNavigation(ctx context.Context, request apicontract.PublishAdminCmsNavigationRequestObject) (apicontract.PublishAdminCmsNavigationResponseObject, error) {
	_, actorID := cmsActor(ctx)
	notes := publishNotes(request.Body)
	record, err := e.navigation.Publish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: notes})
	if err != nil {
		return nil, err
	}
	value, err := contractNavigation(record, false)
	return apicontract.PublishAdminCmsNavigation200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UnpublishAdminCmsNavigation(ctx context.Context, request apicontract.UnpublishAdminCmsNavigationRequestObject) (apicontract.UnpublishAdminCmsNavigationResponseObject, error) {
	_, actorID := cmsActor(ctx)
	notes := publishNotes(request.Body)
	record, err := e.navigation.Unpublish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: notes})
	if err != nil {
		return nil, err
	}
	value, err := contractNavigation(record, false)
	return apicontract.UnpublishAdminCmsNavigation200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DiscardAdminCmsNavigationDraft(ctx context.Context, request apicontract.DiscardAdminCmsNavigationDraftRequestObject) (apicontract.DiscardAdminCmsNavigationDraftResponseObject, error) {
	_, actorID := cmsActor(ctx)
	record, deleted, err := e.navigation.DiscardDraft(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID})
	if err != nil {
		return nil, err
	}
	if deleted {
		return apicontract.DiscardAdminCmsNavigationDraft204Response{}, nil
	}
	value, err := contractNavigation(record, false)
	return apicontract.DiscardAdminCmsNavigationDraft200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) GetContentNavigation(ctx context.Context, request apicontract.GetContentNavigationRequestObject) (apicontract.GetContentNavigationResponseObject, error) {
	record, err := e.navigation.Resolve(ctx, request.Location, draftPreviewActive(ctx))
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	value, err := contractNavigation(record, true)
	return apicontract.GetContentNavigation200JSONResponse(value), err
}

func globalInput(body *apicontract.CmsGlobalRegionDraftRequest, actorID *uint) (cms.GlobalRegionDraftInput, error) {
	if body == nil {
		return cms.GlobalRegionDraftInput{}, errors.New("CMS global region body is required")
	}
	payload, err := cmsPayload(body.Payload)
	if err != nil {
		return cms.GlobalRegionDraftInput{}, err
	}
	input := cms.GlobalRegionDraftInput{Key: body.Key, Title: body.Title, Region: body.Region, Payload: payload, ActorID: actorID}
	if body.ChangeSummary != nil {
		input.ChangeSummary = *body.ChangeSummary
	}
	return input, nil
}
func contractGlobal(record *cms.GlobalRegionRecord, public bool) (apicontract.CmsGlobalRegionResponse, error) {
	current, err := contractVersion(record.CurrentVersion)
	if err != nil {
		return apicontract.CmsGlobalRegionResponse{}, err
	}
	published, err := contractVersion(record.PublishedVersion)
	if err != nil {
		return apicontract.CmsGlobalRegionResponse{}, err
	}
	result := apicontract.CmsGlobalRegionResponse{Region: apicontract.CmsGlobalRegion{Id: int(record.Region.ID), EntryId: int(record.Region.EntryID), Key: record.Region.Key, Title: record.Region.Title, Region: record.Region.Region, CreatedAt: record.Region.CreatedAt, UpdatedAt: record.Region.UpdatedAt}, Entry: contractEntry(record.Entry), HasUnpublishedDraft: record.HasUnpublishedDraft}
	if public {
		result.Entry.CurrentVersionId, result.Entry.PublishedVersionId, result.HasUnpublishedDraft, result.PublishedVersion = nil, nil, false, published
	} else {
		result.CurrentVersion, result.PublishedVersion, result.LatestPublication = current, published, contractPublication(record.LatestPublication)
	}
	return result, nil
}
func (e *CmsMediaEndpoints) ListAdminCmsGlobalRegions(ctx context.Context, request apicontract.ListAdminCmsGlobalRegionsRequestObject) (apicontract.ListAdminCmsGlobalRegionsResponseObject, error) {
	page, limit := cmsPagination(request.Params.Page, request.Params.Limit)
	records, total, err := e.globals.List(ctx, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.CmsGlobalRegionResponse, 0, len(records))
	for i := range records {
		value, err := contractGlobal(&records[i], false)
		if err != nil {
			return nil, err
		}
		data = append(data, value)
	}
	return apicontract.ListAdminCmsGlobalRegions200JSONResponse{Data: data, Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: totalPages(total, limit)}}, nil
}
func (e *CmsMediaEndpoints) CreateAdminCmsGlobalRegion(ctx context.Context, request apicontract.CreateAdminCmsGlobalRegionRequestObject) (apicontract.CreateAdminCmsGlobalRegionResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := globalInput(request.Body, actorID)
	if err != nil {
		return nil, err
	}
	record, err := e.globals.CreateDraft(ctx, input)
	if err != nil {
		return nil, err
	}
	value, err := contractGlobal(record, false)
	return apicontract.CreateAdminCmsGlobalRegion201JSONResponse(value), err
}
func (e *CmsMediaEndpoints) GetAdminCmsGlobalRegion(ctx context.Context, request apicontract.GetAdminCmsGlobalRegionRequestObject) (apicontract.GetAdminCmsGlobalRegionResponseObject, error) {
	record, err := e.globals.Get(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	value, err := contractGlobal(record, false)
	return apicontract.GetAdminCmsGlobalRegion200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UpdateAdminCmsGlobalRegion(ctx context.Context, request apicontract.UpdateAdminCmsGlobalRegionRequestObject) (apicontract.UpdateAdminCmsGlobalRegionResponseObject, error) {
	_, actorID := cmsActor(ctx)
	input, err := globalInput(request.Body, actorID)
	if err != nil {
		return nil, err
	}
	record, err := e.globals.UpdateDraft(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	value, err := contractGlobal(record, false)
	return apicontract.UpdateAdminCmsGlobalRegion200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DeleteAdminCmsGlobalRegion(ctx context.Context, request apicontract.DeleteAdminCmsGlobalRegionRequestObject) (apicontract.DeleteAdminCmsGlobalRegionResponseObject, error) {
	_, actorID := cmsActor(ctx)
	if err := e.globals.Delete(ctx, uint(request.Id), actorID); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminCmsGlobalRegion200JSONResponse{Message: "CMS global region deleted"}, nil
}
func (e *CmsMediaEndpoints) PublishAdminCmsGlobalRegion(ctx context.Context, request apicontract.PublishAdminCmsGlobalRegionRequestObject) (apicontract.PublishAdminCmsGlobalRegionResponseObject, error) {
	_, actorID := cmsActor(ctx)
	record, err := e.globals.Publish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: publishNotes(request.Body)})
	if err != nil {
		return nil, err
	}
	value, err := contractGlobal(record, false)
	return apicontract.PublishAdminCmsGlobalRegion200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) UnpublishAdminCmsGlobalRegion(ctx context.Context, request apicontract.UnpublishAdminCmsGlobalRegionRequestObject) (apicontract.UnpublishAdminCmsGlobalRegionResponseObject, error) {
	_, actorID := cmsActor(ctx)
	record, err := e.globals.Unpublish(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID, Notes: publishNotes(request.Body)})
	if err != nil {
		return nil, err
	}
	value, err := contractGlobal(record, false)
	return apicontract.UnpublishAdminCmsGlobalRegion200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) DiscardAdminCmsGlobalRegionDraft(ctx context.Context, request apicontract.DiscardAdminCmsGlobalRegionDraftRequestObject) (apicontract.DiscardAdminCmsGlobalRegionDraftResponseObject, error) {
	_, actorID := cmsActor(ctx)
	record, deleted, err := e.globals.DiscardDraft(ctx, uint(request.Id), cms.PublishInput{ActorID: actorID})
	if err != nil {
		return nil, err
	}
	if deleted {
		return apicontract.DiscardAdminCmsGlobalRegionDraft204Response{}, nil
	}
	value, err := contractGlobal(record, false)
	return apicontract.DiscardAdminCmsGlobalRegionDraft200JSONResponse(value), err
}
func (e *CmsMediaEndpoints) GetContentGlobalRegion(ctx context.Context, request apicontract.GetContentGlobalRegionRequestObject) (apicontract.GetContentGlobalRegionResponseObject, error) {
	record, err := e.globals.Resolve(ctx, request.Region, draftPreviewActive(ctx))
	if err != nil {
		return nil, cmsEndpointError(err)
	}
	value, err := contractGlobal(record, true)
	return apicontract.GetContentGlobalRegion200JSONResponse(value), err
}

func publishNotes(body *apicontract.CmsPublishRequest) string {
	if body == nil || body.Notes == nil {
		return ""
	}
	return *body.Notes
}

var _ = models.CMSEntry{}
