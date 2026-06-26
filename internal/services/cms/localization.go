package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/media"
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
	localeCodePattern    = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	marketCodePattern    = regexp.MustCompile(`^[A-Z]{2,3}$`)
)

type LocaleInput struct {
	Code           string
	Name           string
	Enabled        bool
	IsDefault      bool
	FallbackLocale string
}

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

func (s *Service) Locales() ([]models.CMSLocale, error) {
	var locales []models.CMSLocale
	err := s.db.Order("is_default DESC, code ASC").Find(&locales).Error
	return locales, err
}

func (s *Service) UpdateLocales(inputs []LocaleInput, actor string) ([]models.CMSLocale, error) {
	if err := validateLocales(inputs); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		codes := make([]string, 0, len(inputs))
		for _, input := range inputs {
			code := normalizeLocale(input.Code)
			codes = append(codes, code)
			var locale models.CMSLocale
			err := tx.Unscoped().Where("code = ?", code).First(&locale).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				locale = models.CMSLocale{Code: code}
			} else if err != nil {
				return err
			}
			locale.DeletedAt = gorm.DeletedAt{}
			locale.Name = strings.TrimSpace(input.Name)
			locale.Enabled = input.Enabled
			locale.IsDefault = input.IsDefault
			locale.FallbackLocale = normalizeLocale(input.FallbackLocale)
			if locale.ID == 0 {
				if err := tx.Select("*").Create(&locale).Error; err != nil {
					return err
				}
			} else if err := tx.Save(&locale).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.CMSLocale{}).Where("code NOT IN ?", codes).Update("enabled", false).Error; err != nil {
			return err
		}
		return createAuditEvent(tx, 0, nil, nil, "locales.updated", actor, fmt.Sprintf("configured %d locales", len(inputs)))
	})
	if err != nil {
		return nil, err
	}
	return s.Locales()
}

func validateLocales(inputs []LocaleInput) error {
	if len(inputs) == 0 {
		return fmt.Errorf("%w: at least one locale is required", ErrInvalidLocale)
	}
	known := make(map[string]LocaleInput, len(inputs))
	defaults := 0
	for _, input := range inputs {
		code := normalizeLocale(input.Code)
		if !localeCodePattern.MatchString(code) || strings.TrimSpace(input.Name) == "" {
			return fmt.Errorf("%w: locale code and name are required", ErrInvalidLocale)
		}
		if _, exists := known[code]; exists {
			return fmt.Errorf("%w: duplicate locale %s", ErrInvalidLocale, code)
		}
		known[code] = input
		if input.IsDefault {
			defaults++
			if !input.Enabled {
				return fmt.Errorf("%w: default locale must be enabled", ErrInvalidLocale)
			}
		}
	}
	if defaults != 1 {
		return fmt.Errorf("%w: exactly one default locale is required", ErrInvalidLocale)
	}
	for code, input := range known {
		fallback := normalizeLocale(input.FallbackLocale)
		if fallback == "" {
			continue
		}
		if fallback == code {
			return fmt.Errorf("%w: locale cannot fall back to itself", ErrInvalidLocale)
		}
		if _, exists := known[fallback]; !exists {
			return fmt.Errorf("%w: fallback locale %s is not configured", ErrInvalidLocale, fallback)
		}
		seen := map[string]bool{code: true}
		for fallback != "" {
			if seen[fallback] {
				return fmt.Errorf("%w: fallback cycle includes %s", ErrInvalidLocale, fallback)
			}
			seen[fallback] = true
			fallback = normalizeLocale(known[fallback].FallbackLocale)
		}
	}
	return nil
}

func (s *Service) ListVariants(pageID uint) ([]models.CMSPageVariant, error) {
	var variants []models.CMSPageVariant
	err := s.db.Where("page_id = ?", pageID).Order("locale ASC, market ASC, id ASC").Find(&variants).Error
	return variants, err
}

func (s *Service) CreateVariant(pageID uint, input VariantInput) (*models.CMSPageVariant, error) {
	return s.saveVariant(pageID, 0, input)
}

func (s *Service) UpdateVariant(pageID, variantID uint, input VariantInput) (*models.CMSPageVariant, error) {
	return s.saveVariant(pageID, variantID, input)
}

func (s *Service) saveVariant(pageID, variantID uint, input VariantInput) (*models.CMSPageVariant, error) {
	input.Locale = normalizeLocale(input.Locale)
	input.Market = strings.ToUpper(strings.TrimSpace(input.Market))
	input.Path = strings.TrimSpace(input.Path)
	input.Title = strings.TrimSpace(input.Title)
	input.Slug = strings.TrimSpace(input.Slug)
	if input.Locale == "" || !localeCodePattern.MatchString(input.Locale) || input.Title == "" {
		return nil, fmt.Errorf("%w: locale and title are required", ErrInvalidPage)
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
	payload, err := ValidateAndNormalizePayload(input.Payload)
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var saved models.CMSPageVariant
	var cleanupIDs []string
	err = s.db.Transaction(func(tx *gorm.DB) error {
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

func (s *Service) DeleteVariant(pageID, variantID uint, actor string) error {
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
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

func (s *Service) TransitionVariant(pageID, variantID uint, action, actor, comment string) (*models.CMSPageVariant, error) {
	return s.TransitionVariantAsRole(pageID, variantID, action, actor, "publisher", comment)
}

func (s *Service) TransitionVariantAsRole(pageID, variantID uint, action, actor, role, comment string) (*models.CMSPageVariant, error) {
	if (action == "approve" || action == "request_changes") && role != "editor" && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	if (action == "publish" || action == "rollback") && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	var variant models.CMSPageVariant
	err := s.db.Transaction(func(tx *gorm.DB) error {
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

func (s *Service) RoleForSubject(subject string) (string, error) {
	if strings.TrimSpace(subject) == "" {
		return "author", nil
	}
	var assignment models.CMSRoleAssignment
	if err := s.db.Where("subject = ?", subject).First(&assignment).Error; err != nil {
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

func (s *Service) AuditEvents(entryID uint, limit int) ([]models.CMSAuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := s.db.Order("created_at DESC, id DESC").Limit(limit)
	if entryID != 0 {
		query = query.Where("entry_id = ?", entryID)
	}
	var events []models.CMSAuditEvent
	err := query.Find(&events).Error
	return events, err
}

func (s *Service) ResolveLocalized(record *PageRecord, requestedLocale, market string, includeDraft bool) (*ResolvedLocalization, error) {
	locales, err := s.Locales()
	if err != nil {
		return nil, err
	}
	requestedLocale = normalizeLocale(requestedLocale)
	market = strings.ToUpper(strings.TrimSpace(market))
	defaultLocale := "en-US"
	byCode := make(map[string]models.CMSLocale, len(locales))
	for _, locale := range locales {
		byCode[locale.Code] = locale
		if locale.IsDefault {
			defaultLocale = locale.Code
		}
	}
	if requestedLocale == "" {
		requestedLocale = defaultLocale
	}
	requestedForMetadata := requestedLocale
	if configured, exists := byCode[requestedLocale]; !exists || !configured.Enabled {
		requestedLocale = defaultLocale
	}
	chain := localeFallbackChain(requestedLocale, defaultLocale, byCode)
	var variants []models.CMSPageVariant
	if err := s.db.Where("page_id = ?", record.Page.ID).Order("id ASC").Find(&variants).Error; err != nil {
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
	resolved := defaultLocale
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
	return &ResolvedLocalization{RequestedLocale: requestedForMetadata, ResolvedLocale: resolved, Market: market, UsedFallback: resolved != requestedForMetadata || selected == nil, Alternates: alternates}, nil
}

func (s *Service) ResolveForLocale(requestPath, requestedLocale, market string, includeDraft bool) (*PageRecord, *ResolvedLocalization, error) {
	normalized, err := normalizePath(requestPath)
	if err != nil {
		return nil, nil, err
	}
	record, err := s.Resolve(normalized, includeDraft)
	inferredLocale := ""
	if errors.Is(err, ErrNotFound) {
		query := s.db.Where("path = ?", normalized)
		if !includeDraft {
			query = query.Where("status = ?", models.CMSVariantStatusPublished)
		}
		var variant models.CMSPageVariant
		if variantErr := query.Order("market DESC, id ASC").First(&variant).Error; variantErr != nil {
			return nil, nil, ErrNotFound
		}
		record, err = s.Get(variant.PageID)
		inferredLocale = variant.Locale
	}
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(requestedLocale) == "" {
		requestedLocale = inferredLocale
	}
	localization, err := s.ResolveLocalized(record, requestedLocale, market, includeDraft)
	record.Localization = localization
	return record, localization, err
}

func localeFallbackChain(requested, defaultLocale string, locales map[string]models.CMSLocale) []string {
	chain := make([]string, 0, 4)
	seen := map[string]bool{}
	current := requested
	for current != "" && !seen[current] {
		seen[current] = true
		chain = append(chain, current)
		configured, ok := locales[current]
		if ok && configured.FallbackLocale != "" {
			current = configured.FallbackLocale
			continue
		}
		if separator := strings.IndexByte(current, '-'); separator > 0 {
			current = current[:separator]
			continue
		}
		current = ""
	}
	if !seen[defaultLocale] {
		chain = append(chain, defaultLocale)
	}
	return chain
}

func normalizeLocale(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	parts[0] = strings.ToLower(parts[0])
	for index := 1; index < len(parts); index++ {
		if len(parts[index]) == 2 || len(parts[index]) == 3 && index == len(parts)-1 {
			parts[index] = strings.ToUpper(parts[index])
		} else {
			parts[index] = strings.ToLower(parts[index])
		}
	}
	return strings.Join(parts, "-")
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
