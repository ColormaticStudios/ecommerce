package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	"ecommerce/internal/services/cms"
	"ecommerce/models"

	"gorm.io/gorm"
)

// CmsMediaEndpoints is an embeddable strict endpoint family for all generated
// CMS administration, public content delivery, and media-upload operations.
// Embedding it in the composed server promotes the complete disjoint family.
type CmsMediaEndpoints struct {
	db            *gorm.DB
	media         *media.Service
	pages         *cms.Service
	navigation    *cms.NavigationService
	globals       *cms.GlobalRegionService
	redirects     *cms.RedirectService
	sitemapOrigin string
}

func NewCmsMediaEndpoints(db *gorm.DB, mediaService *media.Service) (*CmsMediaEndpoints, error) {
	if db == nil {
		return nil, errors.New("CMS database is required")
	}
	return &CmsMediaEndpoints{
		db:            db,
		media:         mediaService,
		pages:         cms.NewPageService(db, mediaService),
		navigation:    cms.NewNavigationService(db),
		globals:       cms.NewGlobalRegionService(db, mediaService),
		redirects:     cms.NewRedirectService(db),
		sitemapOrigin: normalizeCMSOrigin(os.Getenv("PUBLIC_URL")),
	}, nil
}

func draftPreviewActive(ctx context.Context) bool {
	metadata, ok := requestctx.MetadataFrom(ctx)
	return ok && metadata.DraftPreview
}

func cmsEndpointError(err error) error {
	switch {
	case errors.Is(err, cms.ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return problemError(http.StatusNotFound, "not_found", "The requested CMS content was not found.", err)
	case errors.Is(err, cms.ErrDuplicatePath), errors.Is(err, cms.ErrDuplicateVariant):
		return problemError(http.StatusConflict, "state_conflict", err.Error(), err)
	case errors.Is(err, cms.ErrNoDraft), errors.Is(err, cms.ErrInvalidTransition), errors.Is(err, cms.ErrApprovalRequired):
		return problemError(http.StatusConflict, "state_conflict", err.Error(), err)
	case errors.Is(err, cms.ErrPermissionDenied):
		return problemError(http.StatusForbidden, "forbidden", "The CMS operation is not permitted.", err)
	case errors.Is(err, cms.ErrInvalidPage), errors.Is(err, cms.ErrInvalidDelivery), errors.Is(err, cms.ErrInvalidLocale):
		return problemError(http.StatusBadRequest, "invalid_request", err.Error(), err)
	default:
		return err
	}
}

func normalizeCMSOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return ""
	}
	if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
		origin = "https://" + origin
	}
	return origin
}

func contractConvert[T any](value any) (T, error) {
	var result T
	raw, err := json.Marshal(value)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
}

func cmsPayload(value apicontract.CmsPagePayload) (cms.PagePayload, error) {
	return contractConvert[cms.PagePayload](value)
}

func contractPayload(raw string) (apicontract.CmsPagePayload, error) {
	var result apicontract.CmsPagePayload
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return result, err
	}
	return result, nil
}

func cmsActor(ctx context.Context) (subject string, actorID *uint) {
	principal, ok := requestctx.PrincipalFrom(ctx)
	if !ok {
		return "system", nil
	}
	subject = strings.TrimSpace(principal.Subject)
	if subject == "" {
		subject = "system"
	}
	if principal.AccountID != 0 {
		id := principal.AccountID
		actorID = &id
	}
	return subject, actorID
}

func cmsPageInput(body *apicontract.CmsPageDraftRequest, actorID *uint) (cms.PageDraftInput, error) {
	if body == nil {
		return cms.PageDraftInput{}, errors.New("CMS page body is required")
	}
	payload, err := cmsPayload(body.Payload)
	if err != nil {
		return cms.PageDraftInput{}, fmt.Errorf("decode CMS page payload: %w", err)
	}
	input := cms.PageDraftInput{Path: body.Path, Title: body.Title, Payload: payload, ActorID: actorID}
	if body.Slug != nil {
		input.Slug = *body.Slug
	}
	if body.TemplateKey != nil {
		input.TemplateKey = *body.TemplateKey
	}
	if body.Visibility != nil {
		input.Visibility = string(*body.Visibility)
	}
	if body.IsHomepage != nil {
		input.IsHomepage = *body.IsHomepage
	}
	if body.ChangeSummary != nil {
		input.ChangeSummary = *body.ChangeSummary
	}
	return input, nil
}

func contractEntry(entry models.CMSEntry) apicontract.CmsEntry {
	return apicontract.CmsEntry{Id: int(entry.ID), EntryType: apicontract.CmsEntryEntryType(entry.EntryType), Key: entry.Key, Status: apicontract.CmsEntryStatus(entry.Status), CurrentVersionId: uintPointerToInt(entry.CurrentVersionID), PublishedVersionId: uintPointerToInt(entry.PublishedVersionID), CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt}
}

func contractVersion(version *models.CMSEntryVersion) (*apicontract.CmsEntryVersion, error) {
	if version == nil {
		return nil, nil
	}
	payload, err := contractPayload(version.PayloadJSON)
	if err != nil {
		return nil, err
	}
	changeSummary := version.ChangeSummary
	result := &apicontract.CmsEntryVersion{Id: int(version.ID), EntryId: int(version.EntryID), VersionNumber: int(version.VersionNumber), SchemaVersion: int(version.SchemaVersion), Payload: payload, CreatedBy: uintPointerToInt(version.CreatedBy), ChangeSummary: &changeSummary, CreatedAt: version.CreatedAt}
	return result, nil
}

func contractPublication(publication *models.CMSPublication) *apicontract.CmsPublication {
	if publication == nil {
		return nil
	}
	return &apicontract.CmsPublication{Id: int(publication.ID), EntryId: int(publication.EntryID), VersionId: int(publication.VersionID), PublishedBy: uintPointerToInt(publication.PublishedBy), PublishedAt: publication.PublishedAt, RollbackFromPublicationId: uintPointerToInt(publication.RollbackFromPublicationID), Notes: optionalCMSString(publication.Notes)}
}

func contractPageRecord(record *cms.PageRecord, public bool) (apicontract.CmsPageResponse, error) {
	current, err := contractVersion(record.CurrentVersion)
	if err != nil {
		return apicontract.CmsPageResponse{}, err
	}
	published, err := contractVersion(record.PublishedVersion)
	if err != nil {
		return apicontract.CmsPageResponse{}, err
	}
	result := apicontract.CmsPageResponse{
		Page:  apicontract.CmsPage{Id: int(record.Page.ID), EntryId: int(record.Page.EntryID), Path: record.Page.Path, Slug: record.Page.Slug, Title: record.Page.Title, TemplateKey: record.Page.TemplateKey, Visibility: apicontract.CmsPageVisibility(record.Page.Visibility), IsHomepage: record.Page.IsHomepage, SeoMetadataId: uintPointerToInt(record.Page.SEOMetadataID), CreatedAt: record.Page.CreatedAt, UpdatedAt: record.Page.UpdatedAt},
		Entry: contractEntry(record.Entry), HasUnpublishedDraft: record.HasUnpublishedDraft,
	}
	if !public {
		result.CurrentVersion, result.PublishedVersion, result.LatestPublication = current, published, contractPublication(record.LatestPublication)
	} else {
		result.Entry.CurrentVersionId = nil
		result.Entry.PublishedVersionId = nil
		result.HasUnpublishedDraft = false
		result.PublishedVersion = published
	}
	if record.Delivery != nil {
		result.Delivery = &apicontract.CmsDeliveryDecision{ContentVersionId: int(record.Delivery.ContentVersionID), ExperimentId: uintPointerToInt(record.Delivery.ExperimentID), ExperimentVariantId: uintPointerToInt(record.Delivery.ExperimentVariantID), CorrelationId: record.Delivery.CorrelationID}
	}
	if record.Localization != nil {
		localization, err := contractConvert[apicontract.CmsResolvedLocalization](record.Localization)
		if err != nil {
			return result, err
		}
		result.Localization = &localization
	}
	if record.SEO != nil {
		seo, err := contractSEOMetadata(*record.SEO)
		if err != nil {
			return result, err
		}
		result.Seo = &seo
	}
	return result, nil
}

func uintPointerToInt(value *uint) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func optionalCMSString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	result := value
	return &result
}
