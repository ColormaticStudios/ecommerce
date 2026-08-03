package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound      = errors.New("cms page not found")
	ErrDuplicatePath = errors.New("cms page path already exists")
	ErrInvalidPage   = errors.New("invalid cms page")
	ErrNoDraft       = errors.New("cms page has no draft version")
)

type PagePayload map[string]any

type PageDraftInput struct {
	Path          string
	Slug          string
	Title         string
	TemplateKey   string
	Visibility    string
	IsHomepage    bool
	Payload       PagePayload
	ChangeSummary string
	ActorID       *uint
}

type PublishInput struct {
	ActorID *uint
	Notes   string
}

type RollbackInput struct {
	VersionID uint
	ActorID   *uint
	Notes     string
}

type PageRecord struct {
	Page                models.CMSPage
	Entry               models.CMSEntry
	CurrentVersion      *models.CMSEntryVersion
	PublishedVersion    *models.CMSEntryVersion
	LatestPublication   *models.CMSPublication
	HasUnpublishedDraft bool
	Delivery            *DeliveryDecision
	SEO                 *models.SEOMetadata
	Localization        *ResolvedLocalization
}

type Service struct {
	db    *gorm.DB
	media *media.Service
}

func NewPageService(db *gorm.DB, mediaServices ...*media.Service) *Service {
	var mediaService *media.Service
	if len(mediaServices) > 0 {
		mediaService = mediaServices[0]
	}
	return &Service{db: db, media: mediaService}
}

func (s *Service) CreateDraft(input PageDraftInput) (*PageRecord, error) {
	if err := validateDraftInput(&input); err != nil {
		return nil, err
	}
	var record *PageRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing models.CMSPage
		err := tx.Unscoped().Where("path = ?", input.Path).First(&existing).Error
		if err == nil {
			return ErrDuplicatePath
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		entry := models.CMSEntry{
			EntryType: models.CMSEntryTypePage,
			Key:       input.Path,
			Status:    models.CMSEntryStatusDraft,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		version, err := createVersion(tx, entry.ID, 1, input)
		if err != nil {
			return err
		}
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		page := models.CMSPage{
			EntryID:     entry.ID,
			Path:        input.Path,
			Slug:        input.Slug,
			Title:       input.Title,
			TemplateKey: input.TemplateKey,
			Visibility:  models.CMSPageVisibility(input.Visibility),
			IsHomepage:  input.IsHomepage,
		}
		if err := tx.Create(&page).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, input.Payload, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "page.draft_created", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record = &PageRecord{Page: page, Entry: entry, CurrentVersion: version, HasUnpublishedDraft: true}
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *Service) UpdateDraft(id uint, input PageDraftInput) (*PageRecord, error) {
	if err := validateDraftInput(&input); err != nil {
		return nil, err
	}
	var record *PageRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		var existing models.CMSPage
		err = tx.Unscoped().Where("path = ? AND id <> ?", input.Path, page.ID).First(&existing).Error
		if err == nil {
			return ErrDuplicatePath
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		nextNumber, err := nextVersionNumber(tx, entry.ID)
		if err != nil {
			return err
		}
		version, err := createVersion(tx, entry.ID, nextNumber, input)
		if err != nil {
			return err
		}
		page.Path = input.Path
		page.Slug = input.Slug
		page.Title = input.Title
		page.TemplateKey = input.TemplateKey
		page.Visibility = models.CMSPageVisibility(input.Visibility)
		page.IsHomepage = input.IsHomepage
		if err := tx.Save(&page).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		entry.Key = input.Path
		entry.Status = draftStatusFor(entry)
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, input.Payload, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "page.draft_updated", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record = &PageRecord{Page: page, Entry: entry, CurrentVersion: version, PublishedVersion: entry.PublishedVersion, HasUnpublishedDraft: true}
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *Service) Publish(id uint, input PublishInput) (*PageRecord, error) {
	var record *PageRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil {
			return ErrNoDraft
		}
		publication := models.CMSPublication{
			EntryID:     entry.ID,
			VersionID:   *entry.CurrentVersionID,
			PublishedBy: input.ActorID,
			PublishedAt: time.Now().UTC(),
			Notes:       input.Notes,
		}
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "page.published", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "page.published"); err != nil {
			return err
		}
		entry.Status = models.CMSEntryStatusPublished
		entry.PublishedVersionID = entry.CurrentVersionID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		var currentVersion models.CMSEntryVersion
		if err := tx.Where("id = ? AND entry_id = ?", *entry.CurrentVersionID, entry.ID).First(&currentVersion).Error; err != nil {
			return err
		}
		payload, err := payloadFromVersion(currentVersion)
		if err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, payload, media.RoleCMSContent)
		if err != nil {
			return err
		}
		record, err = assembleRecord(tx, page, entry)
		if err != nil {
			return err
		}
		record.LatestPublication = &publication
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *Service) Rollback(id uint, input RollbackInput) (*PageRecord, error) {
	var record *PageRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		var target models.CMSEntryVersion
		if err := tx.Where("entry_id = ? AND id = ?", entry.ID, input.VersionID).First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		var previous models.CMSPublication
		var previousID *uint
		if err := tx.Where("entry_id = ?", entry.ID).Order("published_at DESC, id DESC").First(&previous).Error; err == nil {
			previousID = &previous.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		publication := models.CMSPublication{
			EntryID:                   entry.ID,
			VersionID:                 target.ID,
			PublishedBy:               input.ActorID,
			PublishedAt:               time.Now().UTC(),
			RollbackFromPublicationID: previousID,
			Notes:                     input.Notes,
		}
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &target.ID, nil, "page.rolled_back", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "page.rolled_back"); err != nil {
			return err
		}
		entry.Status = models.CMSEntryStatusPublished
		entry.CurrentVersionID = &target.ID
		entry.PublishedVersionID = &target.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		payload, err := payloadFromVersion(target)
		if err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, payload, media.RoleCMSContent)
		if err != nil {
			return err
		}
		record, err = assembleRecord(tx, page, entry)
		if err != nil {
			return err
		}
		record.LatestPublication = &publication
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *Service) Unpublish(id uint, input PublishInput) (*PageRecord, error) {
	var record *PageRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.PublishedVersionID == nil {
			return ErrNoDraft
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "page.unpublished", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "page.unpublished"); err != nil {
			return err
		}
		entry.PublishedVersionID = nil
		entry.Status = draftStatusFor(entry)
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		record, err = assembleRecord(tx, page, entry)
		return err
	})
	return record, err
}

func (s *Service) DiscardDraft(id uint, input PublishInput) (*PageRecord, bool, error) {
	var record *PageRecord
	deleted := false
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil || (entry.PublishedVersionID != nil && *entry.CurrentVersionID == *entry.PublishedVersionID) {
			return ErrNoDraft
		}
		if entry.PublishedVersionID == nil {
			removed, err := s.deleteLoadedPage(tx, page, entry, input.ActorID, "page.draft_discarded")
			if err != nil {
				return err
			}
			cleanupIDs = append(cleanupIDs, removed...)
			deleted = true
			return nil
		}
		entry.CurrentVersionID = entry.PublishedVersionID
		entry.Status = models.CMSEntryStatusPublished
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		removed, err := deleteRoleContentMediaReferences(tx, entry.ID, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		cleanupIDs = append(cleanupIDs, removed...)
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "page.draft_discarded", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		record, err = assembleRecord(tx, page, entry)
		return err
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, deleted, err
}

func (s *Service) Delete(id uint, actorID *uint) error {
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		page, entry, err := loadPageEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		removed, err := s.deleteLoadedPage(tx, page, entry, actorID, "page.deleted")
		if err != nil {
			return err
		}
		cleanupIDs = append(cleanupIDs, removed...)
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return err
}

func (s *Service) deleteLoadedPage(tx *gorm.DB, page models.CMSPage, entry models.CMSEntry, actorID *uint, action string) ([]string, error) {
	cleanupIDs := []string{}
	wasPublished := entry.PublishedVersionID != nil
	var variantIDs []uint
	if err := tx.Model(&models.CMSPageVariant{}).Where("page_id = ?", page.ID).Pluck("id", &variantIDs).Error; err != nil {
		return nil, err
	}
	if len(variantIDs) > 0 && tx.Migrator().HasTable(&models.MediaReference{}) {
		var variantRefs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id IN ?", media.OwnerTypeCMSPageVariant, variantIDs).Find(&variantRefs).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("owner_type = ? AND owner_id IN ?", media.OwnerTypeCMSPageVariant, variantIDs).Delete(&models.MediaReference{}).Error; err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, ref := range variantRefs {
			if ref.MediaID != "" && !seen[ref.MediaID] {
				seen[ref.MediaID] = true
				cleanupIDs = append(cleanupIDs, ref.MediaID)
			}
		}
	}
	if err := tx.Where("page_id = ?", page.ID).Delete(&models.CMSPageVariant{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("entity_type = ? AND entity_id = ?", "cms_page", page.ID).Delete(&models.SEOMetadata{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSContentVariant{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSSchedule{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSTargetingRule{}).Error; err != nil {
		return nil, err
	}
	var experimentIDs []uint
	if err := tx.Model(&models.CMSExperiment{}).Where("entry_id = ?", entry.ID).Pluck("id", &experimentIDs).Error; err != nil {
		return nil, err
	}
	if len(experimentIDs) > 0 {
		if err := tx.Where("experiment_id IN ?", experimentIDs).Delete(&models.CMSExperimentVariant{}).Error; err != nil {
			return nil, err
		}
	}
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSExperiment{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Delete(&page).Error; err != nil {
		return nil, err
	}
	if err := tx.Delete(&entry).Error; err != nil {
		return nil, err
	}
	entryCleanupIDs, err := deleteContentMediaReferences(tx, entry.ID)
	if err != nil {
		return nil, err
	}
	cleanupIDs = append(cleanupIDs, entryCleanupIDs...)
	if err := createAuditEvent(tx, entry.ID, nil, nil, action, actorLabel(actorID), page.Path); err != nil {
		return nil, err
	}
	if wasPublished {
		if err := createInvalidationEvent(tx, entry.ID, nil, action); err != nil {
			return nil, err
		}
	}
	return cleanupIDs, nil
}

func (s *Service) Get(id uint) (*PageRecord, error) {
	page, entry, err := loadPageEntry(s.db, id, clause.Locking{})
	if err != nil {
		return nil, err
	}
	return assembleRecord(s.db, page, entry)
}

func (s *Service) List(limit, offset int) ([]PageRecord, int64, error) {
	var total int64
	if err := s.db.Model(&models.CMSPage{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pages []models.CMSPage
	if err := s.db.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&pages).Error; err != nil {
		return nil, 0, err
	}
	records := make([]PageRecord, 0, len(pages))
	for _, page := range pages {
		record, err := s.Get(page.ID)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, *record)
	}
	return records, total, nil
}

func (s *Service) ResolvePublished(requestPath string) (*PageRecord, error) {
	return s.Resolve(requestPath, false)
}

func (s *Service) Resolve(requestPath string, includeDraft bool) (*PageRecord, error) {
	normalized, err := normalizePath(requestPath)
	if err != nil {
		return nil, err
	}
	var page models.CMSPage
	if err := s.db.Where("path = ? AND visibility = ?", normalized, models.CMSPageVisibilityPublic).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	record, err := s.Get(page.ID)
	if err != nil {
		return nil, err
	}
	if !includeDraft && record.Entry.PublishedVersionID == nil {
		return nil, ErrNotFound
	}
	if includeDraft && record.Entry.CurrentVersionID != nil {
		return record, nil
	}
	if record.Entry.PublishedVersionID == nil {
		return nil, ErrNotFound
	}
	return record, nil
}

func validateDraftInput(input *PageDraftInput) error {
	normalized, err := normalizePath(input.Path)
	if err != nil {
		return err
	}
	input.Path = normalized
	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidPage)
	}
	if strings.TrimSpace(input.Slug) == "" {
		input.Slug = strings.Trim(strings.TrimPrefix(normalized, "/"), "/")
		if input.Slug == "" {
			input.Slug = "home"
		}
	}
	if strings.TrimSpace(input.TemplateKey) == "" {
		input.TemplateKey = "default"
	}
	if strings.TrimSpace(input.Visibility) == "" {
		input.Visibility = string(models.CMSPageVisibilityPublic)
	}
	switch models.CMSPageVisibility(input.Visibility) {
	case models.CMSPageVisibilityPublic, models.CMSPageVisibilityHidden:
	default:
		return fmt.Errorf("%w: unsupported visibility", ErrInvalidPage)
	}
	if input.Payload == nil {
		input.Payload = PagePayload{}
	}
	if err := validateAndNormalizePayload(input.Payload); err != nil {
		return err
	}
	return nil
}

func normalizePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%w: path is required", ErrInvalidPage)
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		cleaned = "/"
	}
	if strings.Contains(cleaned, "//") || strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("%w: path must be normalized", ErrInvalidPage)
	}
	return cleaned, nil
}

func createVersion(tx *gorm.DB, entryID uint, versionNumber uint, input PageDraftInput) (*models.CMSEntryVersion, error) {
	raw, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("%w: payload must be JSON serializable", ErrInvalidPage)
	}
	version := models.CMSEntryVersion{
		EntryID:       entryID,
		VersionNumber: versionNumber,
		SchemaVersion: 1,
		PayloadJSON:   string(raw),
		CreatedBy:     input.ActorID,
		ChangeSummary: input.ChangeSummary,
	}
	if err := tx.Create(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

func nextVersionNumber(tx *gorm.DB, entryID uint) (uint, error) {
	nextNumber := uint(1)
	var latest models.CMSEntryVersion
	if err := tx.Where("entry_id = ?", entryID).Order("version_number DESC").First(&latest).Error; err == nil {
		nextNumber = latest.VersionNumber + 1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return nextNumber, nil
}

func loadPageEntry(tx *gorm.DB, id uint, lock clause.Locking) (models.CMSPage, models.CMSEntry, error) {
	var page models.CMSPage
	query := tx
	if lock.Strength != "" {
		query = query.Clauses(lock)
	}
	if err := query.First(&page, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return page, models.CMSEntry{}, ErrNotFound
		}
		return page, models.CMSEntry{}, err
	}
	var entry models.CMSEntry
	if err := tx.First(&entry, page.EntryID).Error; err != nil {
		return page, entry, err
	}
	return page, entry, nil
}

func assembleRecord(tx *gorm.DB, page models.CMSPage, entry models.CMSEntry) (*PageRecord, error) {
	record := &PageRecord{Page: page, Entry: entry}
	if entry.CurrentVersionID != nil {
		var version models.CMSEntryVersion
		if err := tx.First(&version, *entry.CurrentVersionID).Error; err != nil {
			return nil, err
		}
		record.CurrentVersion = &version
	}
	if entry.PublishedVersionID != nil {
		var version models.CMSEntryVersion
		if err := tx.First(&version, *entry.PublishedVersionID).Error; err != nil {
			return nil, err
		}
		record.PublishedVersion = &version
	}
	record.HasUnpublishedDraft = entry.CurrentVersionID != nil && (entry.PublishedVersionID == nil || *entry.CurrentVersionID != *entry.PublishedVersionID)
	var seo models.SEOMetadata
	if err := tx.Where("entity_type = ? AND entity_id = ?", "cms_page", page.ID).First(&seo).Error; err == nil {
		record.SEO = &seo
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return record, nil
}

func draftStatusFor(entry models.CMSEntry) models.CMSEntryStatus {
	if entry.PublishedVersionID != nil {
		return models.CMSEntryStatusPublished
	}
	return models.CMSEntryStatusDraft
}

func isUniqueConstraint(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
