package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GlobalRegionDraftInput struct {
	Key           string
	Title         string
	Region        string
	Payload       PagePayload
	ChangeSummary string
	ActorID       *uint
}

type GlobalRegionRecord struct {
	Region              models.CMSGlobalRegion
	Entry               models.CMSEntry
	CurrentVersion      *models.CMSEntryVersion
	PublishedVersion    *models.CMSEntryVersion
	LatestPublication   *models.CMSPublication
	HasUnpublishedDraft bool
}

type GlobalRegionService struct {
	db    *gorm.DB
	media *media.Service
}

func NewGlobalRegionService(db *gorm.DB, mediaServices ...*media.Service) *GlobalRegionService {
	var mediaService *media.Service
	if len(mediaServices) > 0 {
		mediaService = mediaServices[0]
	}
	return &GlobalRegionService{db: db, media: mediaService}
}

func (s *GlobalRegionService) CreateDraft(input GlobalRegionDraftInput) (*GlobalRegionRecord, error) {
	if err := validateGlobalInput(&input); err != nil {
		return nil, err
	}
	var record *GlobalRegionRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing models.CMSGlobalRegion
		err := tx.Unscoped().Where("key = ?", input.Key).First(&existing).Error
		if err == nil {
			return ErrDuplicatePath
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entry := models.CMSEntry{EntryType: models.CMSEntryTypeGlobal, Key: input.Key, Status: models.CMSEntryStatusDraft}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		region := models.CMSGlobalRegion{EntryID: entry.ID, Key: input.Key, Title: input.Title, Region: input.Region}
		if err := tx.Create(&region).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		version, err := createGlobalVersion(tx, entry.ID, 1, input)
		if err != nil {
			return err
		}
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, input.Payload, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "global.draft_created", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record = &GlobalRegionRecord{Region: region, Entry: entry, CurrentVersion: version, HasUnpublishedDraft: true}
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *GlobalRegionService) UpdateDraft(id uint, input GlobalRegionDraftInput) (*GlobalRegionRecord, error) {
	if err := validateGlobalInput(&input); err != nil {
		return nil, err
	}
	var record *GlobalRegionRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		region, entry, err := loadGlobalRegionEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		var existing models.CMSGlobalRegion
		err = tx.Unscoped().Where("key = ? AND id <> ?", input.Key, region.ID).First(&existing).Error
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
		region.Key = input.Key
		region.Title = input.Title
		region.Region = input.Region
		if err := tx.Save(&region).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		version, err := createGlobalVersion(tx, entry.ID, nextNumber, input)
		if err != nil {
			return err
		}
		entry.Key = input.Key
		entry.Status = draftStatusFor(entry)
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, input.Payload, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "global.draft_updated", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record, err = assembleGlobalRegionRecord(tx, region, entry)
		if err != nil {
			return err
		}
		record.CurrentVersion = version
		return nil
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, err
}

func (s *GlobalRegionService) Publish(id uint, input PublishInput) (*GlobalRegionRecord, error) {
	var record *GlobalRegionRecord
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		region, entry, err := loadGlobalRegionEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil {
			return ErrNoDraft
		}
		publication := models.CMSPublication{EntryID: entry.ID, VersionID: *entry.CurrentVersionID, PublishedBy: input.ActorID, PublishedAt: time.Now().UTC(), Notes: input.Notes}
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "global.published", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "global.published"); err != nil {
			return err
		}
		entry.Status = models.CMSEntryStatusPublished
		entry.PublishedVersionID = entry.CurrentVersionID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		var version models.CMSEntryVersion
		if err := tx.Where("id = ? AND entry_id = ?", *entry.CurrentVersionID, entry.ID).First(&version).Error; err != nil {
			return err
		}
		payload, err := payloadFromVersion(version)
		if err != nil {
			return err
		}
		cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, payload, media.RoleCMSContent)
		if err != nil {
			return err
		}
		record, err = assembleGlobalRegionRecord(tx, region, entry)
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

func (s *GlobalRegionService) cleanupOrphanMedia(ids []string) {
	if s.media == nil {
		return
	}
	for _, id := range ids {
		_ = s.media.DeleteIfOrphan(id)
	}
}

func (s *GlobalRegionService) Unpublish(id uint, input PublishInput) (*GlobalRegionRecord, error) {
	var record *GlobalRegionRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		region, entry, err := loadGlobalRegionEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.PublishedVersionID == nil {
			return ErrNoDraft
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "global.unpublished", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "global.unpublished"); err != nil {
			return err
		}
		entry.PublishedVersionID = nil
		entry.Status = draftStatusFor(entry)
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		record, err = assembleGlobalRegionRecord(tx, region, entry)
		return err
	})
	return record, err
}

func (s *GlobalRegionService) DiscardDraft(id uint, input PublishInput) (*GlobalRegionRecord, bool, error) {
	var record *GlobalRegionRecord
	deleted := false
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		region, entry, err := loadGlobalRegionEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil || (entry.PublishedVersionID != nil && *entry.CurrentVersionID == *entry.PublishedVersionID) {
			return ErrNoDraft
		}
		if entry.PublishedVersionID == nil {
			removed, err := s.deleteLoadedRegion(tx, region, entry, input.ActorID, "global.draft_discarded")
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
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "global.draft_discarded", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		record, err = assembleGlobalRegionRecord(tx, region, entry)
		return err
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return record, deleted, err
}

func (s *GlobalRegionService) Delete(id uint, actorID *uint) error {
	var cleanupIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		region, entry, err := loadGlobalRegionEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		removed, err := s.deleteLoadedRegion(tx, region, entry, actorID, "global.deleted")
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

func (s *GlobalRegionService) deleteLoadedRegion(tx *gorm.DB, region models.CMSGlobalRegion, entry models.CMSEntry, actorID *uint, action string) ([]string, error) {
	wasPublished := entry.PublishedVersionID != nil
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSContentVariant{}).Error; err != nil {
		return nil, err
	}
	if err := tx.Delete(&region).Error; err != nil {
		return nil, err
	}
	if err := tx.Delete(&entry).Error; err != nil {
		return nil, err
	}
	cleanupIDs, err := deleteContentMediaReferences(tx, entry.ID)
	if err != nil {
		return nil, err
	}
	if err := createAuditEvent(tx, entry.ID, nil, nil, action, actorLabel(actorID), region.Region); err != nil {
		return nil, err
	}
	if wasPublished {
		if err := createInvalidationEvent(tx, entry.ID, nil, action); err != nil {
			return nil, err
		}
	}
	return cleanupIDs, nil
}

func (s *GlobalRegionService) Get(id uint) (*GlobalRegionRecord, error) {
	region, entry, err := loadGlobalRegionEntry(s.db, id, clause.Locking{})
	if err != nil {
		return nil, err
	}
	return assembleGlobalRegionRecord(s.db, region, entry)
}

func (s *GlobalRegionService) List(limit, offset int) ([]GlobalRegionRecord, int64, error) {
	var total int64
	if err := s.db.Model(&models.CMSGlobalRegion{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var regions []models.CMSGlobalRegion
	if err := s.db.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&regions).Error; err != nil {
		return nil, 0, err
	}
	records := make([]GlobalRegionRecord, 0, len(regions))
	for _, region := range regions {
		record, err := s.Get(region.ID)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, *record)
	}
	return records, total, nil
}

func (s *GlobalRegionService) Resolve(regionKey string, includeDraft bool) (*GlobalRegionRecord, error) {
	regionKey = strings.TrimSpace(regionKey)
	if regionKey == "" {
		return nil, fmt.Errorf("%w: region is required", ErrInvalidPage)
	}
	var region models.CMSGlobalRegion
	query := s.db.Where("cms_global_regions.region = ?", regionKey)
	if !includeDraft {
		query = query.
			Joins("JOIN cms_entries ON cms_entries.id = cms_global_regions.entry_id").
			Where("cms_entries.published_version_id IS NOT NULL")
	}
	if err := query.Order("cms_global_regions.updated_at DESC, cms_global_regions.id DESC").First(&region).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	record, err := s.Get(region.ID)
	if err != nil {
		return nil, err
	}
	if includeDraft && record.CurrentVersion != nil {
		return record, nil
	}
	if record.PublishedVersion == nil {
		return nil, ErrNotFound
	}
	return record, nil
}

func validateGlobalInput(input *GlobalRegionDraftInput) error {
	input.Key = slugLike(input.Key)
	input.Region = slugLike(input.Region)
	input.Title = strings.TrimSpace(input.Title)
	if input.Key == "" || input.Region == "" || input.Title == "" {
		return fmt.Errorf("%w: key, title, and region are required", ErrInvalidPage)
	}
	if input.Payload == nil {
		input.Payload = PagePayload{}
	}
	return validateAndNormalizePayload(input.Payload)
}

func createGlobalVersion(tx *gorm.DB, entryID uint, versionNumber uint, input GlobalRegionDraftInput) (*models.CMSEntryVersion, error) {
	raw, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, err
	}
	version := models.CMSEntryVersion{EntryID: entryID, VersionNumber: versionNumber, SchemaVersion: 1, PayloadJSON: string(raw), CreatedBy: input.ActorID, ChangeSummary: input.ChangeSummary}
	if err := tx.Create(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

func loadGlobalRegionEntry(tx *gorm.DB, id uint, lock clause.Locking) (models.CMSGlobalRegion, models.CMSEntry, error) {
	var region models.CMSGlobalRegion
	query := tx
	if lock.Strength != "" {
		query = query.Clauses(lock)
	}
	if err := query.First(&region, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return region, models.CMSEntry{}, ErrNotFound
		}
		return region, models.CMSEntry{}, err
	}
	var entry models.CMSEntry
	if err := tx.First(&entry, region.EntryID).Error; err != nil {
		return region, entry, err
	}
	return region, entry, nil
}

func assembleGlobalRegionRecord(tx *gorm.DB, region models.CMSGlobalRegion, entry models.CMSEntry) (*GlobalRegionRecord, error) {
	record := &GlobalRegionRecord{Region: region, Entry: entry}
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
	return record, nil
}
