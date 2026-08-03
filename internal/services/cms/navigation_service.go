package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxNavigationDepth = 3

type NavigationItemInput struct {
	ID        uint
	ParentID  *uint
	Label     string
	ItemType  string
	TargetRef string
	URL       string
	SortOrder int
	IsEnabled bool
}

type NavigationDraftInput struct {
	Key           string
	Title         string
	Location      string
	Items         []NavigationItemInput
	ChangeSummary string
	ActorID       *uint
}

type NavigationRecord struct {
	Menu                models.CMSNavigationMenu
	Entry               models.CMSEntry
	Items               []models.CMSNavigationItem
	CurrentVersion      *models.CMSEntryVersion
	PublishedVersion    *models.CMSEntryVersion
	LatestPublication   *models.CMSPublication
	HasUnpublishedDraft bool
}

type NavigationService struct {
	db *gorm.DB
}

func NewNavigationService(db *gorm.DB) *NavigationService {
	return &NavigationService{db: db}
}

func (s *NavigationService) CreateDraft(input NavigationDraftInput) (*NavigationRecord, error) {
	if err := s.validateInput(&input); err != nil {
		return nil, err
	}
	var record *NavigationRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing models.CMSNavigationMenu
		err := tx.Unscoped().Where("key = ?", input.Key).First(&existing).Error
		if err == nil {
			return ErrDuplicatePath
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entry := models.CMSEntry{EntryType: models.CMSEntryTypeNavigation, Key: input.Key, Status: models.CMSEntryStatusDraft}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		menu := models.CMSNavigationMenu{EntryID: entry.ID, Key: input.Key, Title: input.Title, Location: input.Location}
		if err := tx.Create(&menu).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		items, err := replaceNavigationItems(tx, menu.ID, input.Items)
		if err != nil {
			return err
		}
		version, err := createNavigationVersion(tx, entry.ID, 1, input, items)
		if err != nil {
			return err
		}
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "navigation.draft_created", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record = &NavigationRecord{Menu: menu, Entry: entry, Items: items, CurrentVersion: version, HasUnpublishedDraft: true}
		return nil
	})
	return record, err
}

func (s *NavigationService) UpdateDraft(id uint, input NavigationDraftInput) (*NavigationRecord, error) {
	if err := s.validateInput(&input); err != nil {
		return nil, err
	}
	var record *NavigationRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		menu, entry, err := loadNavigationMenuEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		var existing models.CMSNavigationMenu
		err = tx.Unscoped().Where("key = ? AND id <> ?", input.Key, menu.ID).First(&existing).Error
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
		menu.Key = input.Key
		menu.Title = input.Title
		menu.Location = input.Location
		if err := tx.Save(&menu).Error; err != nil {
			if isUniqueConstraint(err) {
				return ErrDuplicatePath
			}
			return err
		}
		items, err := replaceNavigationItems(tx, menu.ID, input.Items)
		if err != nil {
			return err
		}
		version, err := createNavigationVersion(tx, entry.ID, nextNumber, input, items)
		if err != nil {
			return err
		}
		entry.Key = input.Key
		entry.Status = draftStatusFor(entry)
		entry.CurrentVersionID = &version.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, &version.ID, nil, "navigation.draft_updated", actorLabel(input.ActorID), input.ChangeSummary); err != nil {
			return err
		}
		record, err = assembleNavigationRecord(tx, menu, entry, items)
		if err != nil {
			return err
		}
		record.CurrentVersion = version
		return nil
	})
	return record, err
}

func (s *NavigationService) Publish(id uint, input PublishInput) (*NavigationRecord, error) {
	var record *NavigationRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		menu, entry, err := loadNavigationMenuEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil {
			return ErrNoDraft
		}
		var items []models.CMSNavigationItem
		if err := tx.Where("menu_id = ?", menu.ID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
			return err
		}
		if err := s.validatePublishedTargets(tx, items); err != nil {
			return err
		}
		publication := models.CMSPublication{EntryID: entry.ID, VersionID: *entry.CurrentVersionID, PublishedBy: input.ActorID, PublishedAt: time.Now().UTC(), Notes: input.Notes}
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "navigation.published", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "navigation.published"); err != nil {
			return err
		}
		entry.Status = models.CMSEntryStatusPublished
		entry.PublishedVersionID = entry.CurrentVersionID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		record, err = assembleNavigationRecord(tx, menu, entry, items)
		if err != nil {
			return err
		}
		record.LatestPublication = &publication
		return nil
	})
	return record, err
}

func (s *NavigationService) Unpublish(id uint, input PublishInput) (*NavigationRecord, error) {
	var record *NavigationRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		menu, entry, err := loadNavigationMenuEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.PublishedVersionID == nil {
			return ErrNoDraft
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "navigation.unpublished", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		if err := createInvalidationEvent(tx, entry.ID, nil, "navigation.unpublished"); err != nil {
			return err
		}
		entry.PublishedVersionID = nil
		entry.Status = draftStatusFor(entry)
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		record, err = assembleNavigationRecord(tx, menu, entry, nil)
		return err
	})
	return record, err
}

func (s *NavigationService) DiscardDraft(id uint, input PublishInput) (*NavigationRecord, bool, error) {
	var record *NavigationRecord
	deleted := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		menu, entry, err := loadNavigationMenuEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil || (entry.PublishedVersionID != nil && *entry.CurrentVersionID == *entry.PublishedVersionID) {
			return ErrNoDraft
		}
		if entry.PublishedVersionID == nil {
			if err := s.deleteLoadedMenu(tx, menu, entry, input.ActorID, "navigation.draft_discarded"); err != nil {
				return err
			}
			deleted = true
			return nil
		}
		entry.CurrentVersionID = entry.PublishedVersionID
		entry.Status = models.CMSEntryStatusPublished
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		if err := createAuditEvent(tx, entry.ID, entry.CurrentVersionID, nil, "navigation.draft_discarded", actorLabel(input.ActorID), input.Notes); err != nil {
			return err
		}
		record, err = assembleNavigationRecord(tx, menu, entry, nil)
		return err
	})
	return record, deleted, err
}

func (s *NavigationService) Delete(id uint, actorID *uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		menu, entry, err := loadNavigationMenuEntry(tx, id, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		return s.deleteLoadedMenu(tx, menu, entry, actorID, "navigation.deleted")
	})
}

func (s *NavigationService) deleteLoadedMenu(tx *gorm.DB, menu models.CMSNavigationMenu, entry models.CMSEntry, actorID *uint, action string) error {
	wasPublished := entry.PublishedVersionID != nil
	if err := tx.Where("menu_id = ?", menu.ID).Delete(&models.CMSNavigationItem{}).Error; err != nil {
		return err
	}
	if err := tx.Delete(&menu).Error; err != nil {
		return err
	}
	if err := tx.Delete(&entry).Error; err != nil {
		return err
	}
	if err := createAuditEvent(tx, entry.ID, nil, nil, action, actorLabel(actorID), menu.Key); err != nil {
		return err
	}
	if wasPublished {
		if err := createInvalidationEvent(tx, entry.ID, nil, action); err != nil {
			return err
		}
	}
	return nil
}

func (s *NavigationService) Get(id uint) (*NavigationRecord, error) {
	menu, entry, err := loadNavigationMenuEntry(s.db, id, clause.Locking{})
	if err != nil {
		return nil, err
	}
	var items []models.CMSNavigationItem
	if err := s.db.Where("menu_id = ?", menu.ID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return assembleNavigationRecord(s.db, menu, entry, items)
}

func (s *NavigationService) List(limit, offset int) ([]NavigationRecord, int64, error) {
	var total int64
	if err := s.db.Model(&models.CMSNavigationMenu{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var menus []models.CMSNavigationMenu
	if err := s.db.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&menus).Error; err != nil {
		return nil, 0, err
	}
	records := make([]NavigationRecord, 0, len(menus))
	for _, menu := range menus {
		record, err := s.Get(menu.ID)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, *record)
	}
	return records, total, nil
}

func (s *NavigationService) Resolve(location string, includeDraft bool) (*NavigationRecord, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, fmt.Errorf("%w: location is required", ErrInvalidPage)
	}
	var menu models.CMSNavigationMenu
	if err := s.db.Where("location = ?", location).Order("updated_at DESC, id DESC").First(&menu).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	record, err := s.Get(menu.ID)
	if err != nil {
		return nil, err
	}
	if includeDraft && record.CurrentVersion != nil {
		if items, err := navigationItemsFromVersion(record.CurrentVersion.PayloadJSON); err == nil {
			record.Items = items
		}
		return record, nil
	}
	if record.PublishedVersion == nil {
		return nil, ErrNotFound
	}
	if items, err := navigationItemsFromVersion(record.PublishedVersion.PayloadJSON); err == nil {
		record.Items = items
	}
	return record, nil
}

func (s *NavigationService) validateInput(input *NavigationDraftInput) error {
	input.Key = slugLike(input.Key)
	input.Location = slugLike(input.Location)
	input.Title = strings.TrimSpace(input.Title)
	if input.Key == "" || input.Location == "" || input.Title == "" {
		return fmt.Errorf("%w: key, title, and location are required", ErrInvalidPage)
	}
	if len(input.Items) == 0 {
		return nil
	}
	seen := map[uint]NavigationItemInput{}
	for _, item := range input.Items {
		if item.ID != 0 {
			seen[item.ID] = item
		}
		if strings.TrimSpace(item.Label) == "" {
			return fmt.Errorf("%w: navigation item label is required", ErrInvalidPage)
		}
		switch models.CMSNavigationItemType(item.ItemType) {
		case models.CMSNavigationItemTypeInternal, models.CMSNavigationItemTypeExternal, models.CMSNavigationItemTypeCategory, models.CMSNavigationItemTypeProduct, models.CMSNavigationItemTypePage, models.CMSNavigationItemTypeDropdown:
		default:
			return fmt.Errorf("%w: unsupported navigation item type", ErrInvalidPage)
		}
		if item.URL != "" && !isSafeURL(item.URL) {
			return fmt.Errorf("%w: navigation item URL is unsafe", ErrInvalidPage)
		}
	}
	for _, item := range input.Items {
		if navigationDepth(item, seen) > maxNavigationDepth {
			return fmt.Errorf("%w: navigation menu exceeds max depth", ErrInvalidPage)
		}
	}
	return nil
}

func (s *NavigationService) validatePublishedTargets(tx *gorm.DB, items []models.CMSNavigationItem) error {
	coreRoutes := map[string]struct{}{"/": {}, "/search": {}, "/cart": {}, "/checkout": {}, "/login": {}, "/signup": {}, "/orders": {}, "/profile": {}}
	for _, item := range items {
		if !item.IsEnabled {
			continue
		}
		if item.ItemType != models.CMSNavigationItemTypeInternal && item.ItemType != models.CMSNavigationItemTypePage {
			continue
		}
		target := strings.TrimSpace(item.TargetRef)
		if target == "" {
			target = strings.TrimSpace(item.URL)
		}
		if target == "" || !strings.HasPrefix(target, "/") {
			return fmt.Errorf("%w: navigation item %q must target an internal path", ErrInvalidPage, item.Label)
		}
		if _, ok := coreRoutes[target]; ok {
			continue
		}
		var page models.CMSPage
		if err := tx.Where("path = ? AND visibility = ?", target, models.CMSPageVisibilityPublic).First(&page).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: navigation item %q targets an unpublished or missing page", ErrInvalidPage, item.Label)
			}
			return err
		}
		var entry models.CMSEntry
		if err := tx.First(&entry, page.EntryID).Error; err != nil {
			return err
		}
		if entry.PublishedVersionID == nil {
			return fmt.Errorf("%w: navigation item %q targets an unpublished page", ErrInvalidPage, item.Label)
		}
	}
	return nil
}

func replaceNavigationItems(tx *gorm.DB, menuID uint, input []NavigationItemInput) ([]models.CMSNavigationItem, error) {
	if err := tx.Where("menu_id = ?", menuID).Delete(&models.CMSNavigationItem{}).Error; err != nil {
		return nil, err
	}
	items := make([]models.CMSNavigationItem, 0, len(input))
	for _, item := range input {
		enabled := item.IsEnabled
		items = append(items, models.CMSNavigationItem{
			MenuID:    menuID,
			ParentID:  item.ParentID,
			Label:     strings.TrimSpace(item.Label),
			ItemType:  models.CMSNavigationItemType(item.ItemType),
			TargetRef: strings.TrimSpace(item.TargetRef),
			URL:       strings.TrimSpace(item.URL),
			SortOrder: item.SortOrder,
			IsEnabled: enabled,
		})
	}
	if len(items) > 0 {
		if err := tx.Create(&items).Error; err != nil {
			return nil, err
		}
	}
	return items, nil
}

func createNavigationVersion(tx *gorm.DB, entryID uint, versionNumber uint, input NavigationDraftInput, items []models.CMSNavigationItem) (*models.CMSEntryVersion, error) {
	payload := map[string]any{"key": input.Key, "title": input.Title, "location": input.Location, "items": navigationItemsPayload(items)}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	version := models.CMSEntryVersion{EntryID: entryID, VersionNumber: versionNumber, SchemaVersion: 1, PayloadJSON: string(raw), CreatedBy: input.ActorID, ChangeSummary: input.ChangeSummary}
	if err := tx.Create(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

func loadNavigationMenuEntry(tx *gorm.DB, id uint, lock clause.Locking) (models.CMSNavigationMenu, models.CMSEntry, error) {
	var menu models.CMSNavigationMenu
	query := tx
	if lock.Strength != "" {
		query = query.Clauses(lock)
	}
	if err := query.First(&menu, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return menu, models.CMSEntry{}, ErrNotFound
		}
		return menu, models.CMSEntry{}, err
	}
	var entry models.CMSEntry
	if err := tx.First(&entry, menu.EntryID).Error; err != nil {
		return menu, entry, err
	}
	return menu, entry, nil
}

func assembleNavigationRecord(tx *gorm.DB, menu models.CMSNavigationMenu, entry models.CMSEntry, items []models.CMSNavigationItem) (*NavigationRecord, error) {
	record := &NavigationRecord{Menu: menu, Entry: entry, Items: items}
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

func navigationItemsPayload(items []models.CMSNavigationItem) []map[string]any {
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, map[string]any{
			"id":         item.ID,
			"parent_id":  item.ParentID,
			"label":      item.Label,
			"item_type":  string(item.ItemType),
			"target_ref": item.TargetRef,
			"url":        item.URL,
			"sort_order": item.SortOrder,
			"is_enabled": item.IsEnabled,
		})
	}
	return payload
}

func navigationItemsFromVersion(raw string) ([]models.CMSNavigationItem, error) {
	var payload struct {
		Items []models.CMSNavigationItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func navigationDepth(item NavigationItemInput, items map[uint]NavigationItemInput) int {
	depth := 1
	parentID := item.ParentID
	seen := map[uint]struct{}{}
	for parentID != nil {
		if _, ok := seen[*parentID]; ok {
			return maxNavigationDepth + 1
		}
		seen[*parentID] = struct{}{}
		parent, ok := items[*parentID]
		if !ok {
			return depth
		}
		depth++
		parentID = parent.ParentID
	}
	return depth
}

func slugLike(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
