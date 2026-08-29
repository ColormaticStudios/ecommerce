package cms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidLocale     = errors.New("invalid CMS locale configuration")
	ErrDuplicateVariant  = errors.New("CMS page variant already exists")
	ErrInvalidTransition = errors.New("invalid CMS workflow transition")
	ErrApprovalRequired  = errors.New("CMS variant must be approved before publishing")
	ErrPermissionDenied  = errors.New("insufficient CMS permission")
	marketCodePattern    = regexp.MustCompile(`^[A-Z]{2,3}$`)
)

type VariantInput struct {
	Locale        string
	Market        string
	Path          string
	Slug          string
	Title         string
	Payload       PagePayload
	ChangeSummary string
	Actor         string
}

type ResolvedLocalization struct {
	RequestedLocale string
	ResolvedLocale  string
	Market          string
	UsedFallback    bool
	Alternates      []models.CMSPageVariant
}

func (s *Service) ListVariants(ctx context.Context, pageID uint) ([]models.CMSPageVariant, error) {
	db := s.db.WithContext(ctx)
	var variants []models.CMSPageVariant
	err := db.Where("page_id = ?", pageID).Order("locale ASC, market ASC, id ASC").Find(&variants).Error
	return variants, err
}

func (s *Service) CreateVariant(ctx context.Context, pageID uint, input VariantInput) (*models.CMSPageVariant, error) {
	return s.saveVariant(s.db.WithContext(ctx), pageID, 0, input)
}

func (s *Service) UpdateVariant(ctx context.Context, pageID, variantID uint, input VariantInput) (*models.CMSPageVariant, error) {
	return s.saveVariant(s.db.WithContext(ctx), pageID, variantID, input)
}

func (s *Service) saveVariant(db *gorm.DB, pageID, variantID uint, input VariantInput) (*models.CMSPageVariant, error) {
	input.Locale = normalizeLocale(input.Locale)
	input.Market = strings.ToUpper(strings.TrimSpace(input.Market))
	input.Path = strings.TrimSpace(input.Path)
	input.Title = strings.TrimSpace(input.Title)
	input.Slug = strings.TrimSpace(input.Slug)
	if input.Locale == "" || input.Title == "" {
		return nil, fmt.Errorf("%w: locale and title are required", ErrInvalidPage)
	}
	if _, err := s.localization.RequireEnabledLocale(db.Statement.Context, input.Locale); err != nil {
		return nil, fmt.Errorf("%w: locale must exist and be enabled: %v", ErrInvalidLocale, err)
	}
	if input.Market != "" && !marketCodePattern.MatchString(input.Market) {
		return nil, fmt.Errorf("%w: market must be a 2 or 3 letter region code", ErrInvalidPage)
	}
	pathValue, err := normalizePath(input.Path)
	if err != nil {
		return nil, err
	}
	input.Path = pathValue
	if input.Slug == "" {
		input.Slug = strings.Trim(pathValue, "/")
	}
	payload, err := prepareDraftPayload(input.Payload)
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var saved models.CMSPageVariant
	var cleanupIDs []string
	err = db.Transaction(func(tx *gorm.DB) error {
		var page models.CMSPage
		if err := tx.First(&page, pageID).Error; err != nil {
			return ErrNotFound
		}
		if variantID == 0 {
			saved = models.CMSPageVariant{PageID: page.ID, EntryID: page.EntryID, Revision: 1}
		} else if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND page_id = ?", variantID, pageID).First(&saved).Error; err != nil {
			return ErrNotFound
		} else {
			saved.Revision++
		}
		saved.Locale = input.Locale
		saved.Market = input.Market
		saved.Path = input.Path
		saved.Slug = input.Slug
		saved.Title = input.Title
		saved.DraftPayloadJSON = string(payloadJSON)
		saved.Status = models.CMSVariantStatusDraft
		saved.SubmittedBy = ""
		saved.ApprovedBy = ""
		if saved.ID == 0 {
			if err := tx.Select("*").Create(&saved).Error; err != nil {
				if isUniqueConstraint(err) {
					return ErrDuplicateVariant
				}
				return err
			}
		} else if err := tx.Save(&saved).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicateVariant
			}
			return err
		}
		cleanupIDs, err = syncVariantMediaReferences(tx, saved.ID, payload)
		if err != nil {
			return err
		}
		return createAuditEvent(tx, page.EntryID, nil, &saved.ID, "variant.draft_saved", input.Actor, input.ChangeSummary)
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return &saved, err
}

func (s *Service) DeleteVariant(ctx context.Context, pageID, variantID uint, actor string) error {
	db := s.db.WithContext(ctx)
	var cleanupIDs []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var variant models.CMSPageVariant
		if err := tx.Where("id = ? AND page_id = ?", variantID, pageID).First(&variant).Error; err != nil {
			return ErrNotFound
		}
		if err := tx.Delete(&variant).Error; err != nil {
			return err
		}
		var references []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeCMSPageVariant, variant.ID).Find(&references).Error; err != nil {
			return err
		}
		for _, reference := range references {
			cleanupIDs = append(cleanupIDs, reference.MediaID)
		}
		if err := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeCMSPageVariant, variant.ID).Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}
		return createAuditEvent(tx, variant.EntryID, nil, &variant.ID, "variant.deleted", actor, variant.Locale+" "+variant.Market)
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return err
}

func (s *Service) TransitionVariant(ctx context.Context, pageID, variantID uint, action, actor, comment string) (*models.CMSPageVariant, error) {
	return s.TransitionVariantAsRole(ctx, pageID, variantID, action, actor, "publisher", comment)
}

func (s *Service) TransitionVariantAsRole(ctx context.Context, pageID, variantID uint, action, actor, role, comment string) (*models.CMSPageVariant, error) {
	db := s.db.WithContext(ctx)
	if (action == "approve" || action == "request_changes") && role != "editor" && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	if (action == "publish" || action == "rollback") && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	var variant models.CMSPageVariant
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND page_id = ?", variantID, pageID).First(&variant).Error; err != nil {
			return ErrNotFound
		}
		now := time.Now().UTC()
		switch action {
		case "submit":
			if variant.Status != models.CMSVariantStatusDraft && variant.Status != models.CMSVariantStatusChangesRequested {
				return ErrInvalidTransition
			}
			variant.Status = models.CMSVariantStatusInReview
			variant.SubmittedBy = actor
		case "approve":
			if variant.Status != models.CMSVariantStatusInReview {
				return ErrInvalidTransition
			}
			variant.Status = models.CMSVariantStatusApproved
			variant.ApprovedBy = actor
		case "request_changes":
			if variant.Status != models.CMSVariantStatusInReview {
				return ErrInvalidTransition
			}
			variant.Status = models.CMSVariantStatusChangesRequested
		case "publish":
			if variant.Status != models.CMSVariantStatusApproved {
				return ErrApprovalRequired
			}
			if _, err := publicationPayloadJSON(tx, variant.DraftPayloadJSON); err != nil {
				return err
			}
			variant.Status = models.CMSVariantStatusPublished
			variant.PublishedPayloadJSON = variant.DraftPayloadJSON
			variant.PublishedAt = &now
			if err := createInvalidationEvent(tx, variant.EntryID, &variant.ID, "variant.published"); err != nil {
				return err
			}
		case "rollback":
			if variant.PublishedPayloadJSON == "" || variant.PublishedPayloadJSON == "{}" {
				return ErrInvalidTransition
			}
			if _, err := publicationPayloadJSON(tx, variant.PublishedPayloadJSON); err != nil {
				return err
			}
			variant.DraftPayloadJSON = variant.PublishedPayloadJSON
			variant.Status = models.CMSVariantStatusPublished
		default:
			return ErrInvalidTransition
		}
		if err := tx.Save(&variant).Error; err != nil {
			return err
		}
		if strings.TrimSpace(comment) != "" {
			variantID := variant.ID
			changeComment := models.CMSChangeComment{EntryID: variant.EntryID, VariantID: &variantID, Actor: actor, Body: strings.TrimSpace(comment), CreatedAt: now}
			if err := tx.Create(&changeComment).Error; err != nil {
				return err
			}
		}
		return createAuditEvent(tx, variant.EntryID, nil, &variant.ID, "variant."+action, actor, strings.TrimSpace(comment))
	})
	return &variant, err
}

func (s *Service) RoleForSubject(ctx context.Context, subject string) (string, error) {
	db := s.db.WithContext(ctx)
	if strings.TrimSpace(subject) == "" {
		return "author", nil
	}
	var assignment models.CMSRoleAssignment
	if err := db.Where("subject = ?", subject).First(&assignment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "publisher", nil
		}
		return "", err
	}
	if assignment.Role != "author" && assignment.Role != "editor" && assignment.Role != "publisher" {
		return "", ErrPermissionDenied
	}
	return assignment.Role, nil
}

func (s *Service) AuditEvents(ctx context.Context, entryID uint, limit int) ([]models.CMSAuditEvent, error) {
	db := s.db.WithContext(ctx)
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := db.Order("created_at DESC, id DESC").Limit(limit)
	if entryID != 0 {
		query = query.Where("entry_id = ?", entryID)
	}
	var events []models.CMSAuditEvent
	err := query.Find(&events).Error
	return events, err
}

func (s *Service) ResolveLocalized(ctx context.Context, record *PageRecord, requestedLocale, market string, includeDraft bool) (*ResolvedLocalization, error) {
	db := s.db.WithContext(ctx)
	market = strings.ToUpper(strings.TrimSpace(market))
	var resolution localizationservice.Resolution
	if negotiated, ok := requestctx.LocaleResolutionFrom(ctx); ok && strings.TrimSpace(requestedLocale) == "" {
		resolution = localizationservice.Resolution{
			RequestedLocale: negotiated.RequestedLocale, ResolvedLocale: negotiated.ResolvedLocale,
			Source: negotiated.Source, FallbackChain: negotiated.FallbackChain, UsedFallback: negotiated.UsedFallback,
		}
		if market == "" {
			market = negotiated.Market
		}
	} else {
		var err error
		resolution, err = s.localization.ResolveLocale(ctx, localizationservice.ResolutionInput{ExplicitLocale: requestedLocale})
		if err != nil {
			return nil, err
		}
	}
	requestedForMetadata := resolution.RequestedLocale
	chain := resolution.FallbackChain
	var variants []models.CMSPageVariant
	if err := db.Where("page_id = ?", record.Page.ID).Order("id ASC").Find(&variants).Error; err != nil {
		return nil, err
	}
	statusAllowed := func(variant models.CMSPageVariant) bool {
		return includeDraft || variant.Status == models.CMSVariantStatusPublished
	}
	var selected *models.CMSPageVariant
	for _, locale := range chain {
		for _, candidateMarket := range []string{market, ""} {
			for index := range variants {
				variant := &variants[index]
				if statusAllowed(*variant) && variant.Locale == locale && variant.Market == candidateMarket {
					selected = variant
					break
				}
			}
			if selected != nil {
				break
			}
		}
		if selected != nil {
			break
		}
	}
	resolved := resolution.ResolvedLocale
	if selected != nil {
		resolved = selected.Locale
		record.Page.Path = selected.Path
		record.Page.Slug = selected.Slug
		record.Page.Title = selected.Title
		payloadJSON := selected.PublishedPayloadJSON
		if includeDraft {
			payloadJSON = selected.DraftPayloadJSON
		}
		version := record.PublishedVersion
		if includeDraft {
			version = record.CurrentVersion
		}
		if version != nil {
			copyVersion := *version
			copyVersion.PayloadJSON = payloadJSON
			if !includeDraft {
				filtered, _, err := FilterPublicPayloadJSON(copyVersion.PayloadJSON)
				if err != nil {
					return nil, err
				}
				copyVersion.PayloadJSON = filtered
			}
			if includeDraft {
				record.CurrentVersion = &copyVersion
			} else {
				record.PublishedVersion = &copyVersion
			}
		}
	}
	alternates := make([]models.CMSPageVariant, 0)
	for _, variant := range variants {
		if variant.Status == models.CMSVariantStatusPublished {
			alternates = append(alternates, variant)
		}
	}
	sort.Slice(alternates, func(i, j int) bool {
		if alternates[i].Locale == alternates[j].Locale {
			return alternates[i].Market < alternates[j].Market
		}
		return alternates[i].Locale < alternates[j].Locale
	})
	return &ResolvedLocalization{RequestedLocale: requestedForMetadata, ResolvedLocale: resolved, Market: market, UsedFallback: resolution.UsedFallback || resolved != resolution.ResolvedLocale || selected == nil, Alternates: alternates}, nil
}

func (s *Service) ResolveForLocale(ctx context.Context, requestPath, requestedLocale, market string, includeDraft bool) (*PageRecord, *ResolvedLocalization, error) {
	db := s.db.WithContext(ctx)
	normalized, err := normalizePath(requestPath)
	if err != nil {
		return nil, nil, err
	}
	record, err := s.Resolve(ctx, normalized, includeDraft)
	inferredLocale := ""
	if errors.Is(err, ErrNotFound) {
		query := db.Where("path = ?", normalized)
		if !includeDraft {
			query = query.Where("status = ?", models.CMSVariantStatusPublished)
		}
		var variant models.CMSPageVariant
		if variantErr := query.Order("market DESC, id ASC").First(&variant).Error; variantErr != nil {
			return nil, nil, ErrNotFound
		}
		record, err = s.Get(ctx, variant.PageID)
		inferredLocale = variant.Locale
	}
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(requestedLocale) == "" {
		requestedLocale = inferredLocale
	}
	localization, err := s.ResolveLocalized(ctx, record, requestedLocale, market, includeDraft)
	record.Localization = localization
	return record, localization, err
}

func normalizeLocale(value string) string {
	normalized, err := localizationservice.NormalizeLocale(value)
	if err != nil {
		return ""
	}
	return normalized
}

func createAuditEvent(tx *gorm.DB, entryID uint, versionID, variantID *uint, action, actor, detail string) error {
	event := models.CMSAuditEvent{EntryID: entryID, VersionID: versionID, VariantID: variantID, Action: action, Actor: actor, Detail: detail, CreatedAt: time.Now().UTC()}
	return tx.Create(&event).Error
}

func createInvalidationEvent(tx *gorm.DB, entryID uint, variantID *uint, reason string) error {
	event := models.CMSInvalidationEvent{EntryID: entryID, VariantID: variantID, Reason: reason, Status: "pending", CreatedAt: time.Now().UTC()}
	return tx.Create(&event).Error
}

func actorLabel(actorID *uint) string {
	if actorID == nil {
		return "system"
	}
	return fmt.Sprintf("user:%d", *actorID)
}
