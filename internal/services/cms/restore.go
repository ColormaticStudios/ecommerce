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
)

var ErrInvalidExport = errors.New("invalid CMS export")

type restoreVersion struct {
	ID            uint        `json:"id"`
	EntryID       uint        `json:"entry_id"`
	VersionNumber uint        `json:"version_number"`
	SchemaVersion uint        `json:"schema_version"`
	Payload       PagePayload `json:"payload"`
	CreatedBy     *uint       `json:"created_by"`
	ChangeSummary *string     `json:"change_summary"`
	CreatedAt     time.Time   `json:"created_at"`
}

type restoreBundle struct {
	SchemaVersion int `json:"schema_version"`
	Locales       []struct {
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		Enabled        bool    `json:"enabled"`
		IsDefault      bool    `json:"is_default"`
		FallbackLocale *string `json:"fallback_locale"`
	} `json:"locales"`
	Pages []struct {
		Page              models.CMSPage         `json:"page"`
		Entry             models.CMSEntry        `json:"entry"`
		CurrentVersion    *restoreVersion        `json:"current_version"`
		PublishedVersion  *restoreVersion        `json:"published_version"`
		LatestPublication *models.CMSPublication `json:"latest_publication"`
	} `json:"pages"`
	Navigation []struct {
		Menu              models.CMSNavigationMenu   `json:"menu"`
		Entry             models.CMSEntry            `json:"entry"`
		Items             []models.CMSNavigationItem `json:"items"`
		CurrentVersion    *restoreVersion            `json:"current_version"`
		PublishedVersion  *restoreVersion            `json:"published_version"`
		LatestPublication *models.CMSPublication     `json:"latest_publication"`
	} `json:"navigation"`
	GlobalRegions []struct {
		Region            models.CMSGlobalRegion `json:"region"`
		Entry             models.CMSEntry        `json:"entry"`
		CurrentVersion    *restoreVersion        `json:"current_version"`
		PublishedVersion  *restoreVersion        `json:"published_version"`
		LatestPublication *models.CMSPublication `json:"latest_publication"`
	} `json:"global_regions"`
	Variants []struct {
		ID          uint        `json:"id"`
		PageID      uint        `json:"page_id"`
		EntryID     uint        `json:"entry_id"`
		Locale      string      `json:"locale"`
		Market      string      `json:"market"`
		Path        string      `json:"path"`
		Slug        string      `json:"slug"`
		Title       string      `json:"title"`
		Payload     PagePayload `json:"payload"`
		Status      string      `json:"status"`
		Revision    uint        `json:"revision"`
		SubmittedBy *string     `json:"submitted_by"`
		ApprovedBy  *string     `json:"approved_by"`
		PublishedAt *time.Time  `json:"published_at"`
		CreatedAt   time.Time   `json:"created_at"`
		UpdatedAt   time.Time   `json:"updated_at"`
	} `json:"variants"`
}

func (s *Service) RestoreExport(raw []byte, actor string) error {
	var bundle restoreBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("%w: malformed JSON", ErrInvalidExport)
	}
	if bundle.SchemaVersion != 1 {
		return fmt.Errorf("%w: unsupported schema version %d", ErrInvalidExport, bundle.SchemaVersion)
	}
	localeInputs := make([]LocaleInput, 0, len(bundle.Locales))
	for _, locale := range bundle.Locales {
		localeInputs = append(localeInputs, LocaleInput{
			Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault,
			FallbackLocale: stringValue(locale.FallbackLocale),
		})
	}
	if err := validateLocales(localeInputs); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidExport, err)
	}
	if err := validateRestoreBundle(bundle); err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, deletion := range []struct {
			model any
			query string
			args  []any
		}{
			{&models.MediaReference{}, "owner_type IN ?", []any{[]string{media.OwnerTypeCMSEntry, media.OwnerTypeCMSPageVariant}}},
			{&models.CMSPageVariant{}, "1 = 1", nil},
			{&models.CMSNavigationItem{}, "1 = 1", nil},
			{&models.CMSPage{}, "1 = 1", nil},
			{&models.CMSNavigationMenu{}, "1 = 1", nil},
			{&models.CMSGlobalRegion{}, "1 = 1", nil},
			{&models.CMSPublication{}, "1 = 1", nil},
			{&models.CMSEntryVersion{}, "1 = 1", nil},
			{&models.CMSEntry{}, "1 = 1", nil},
			{&models.CMSLocale{}, "1 = 1", nil},
		} {
			if err := tx.Unscoped().Where(deletion.query, deletion.args...).Delete(deletion.model).Error; err != nil {
				return err
			}
		}

		for _, locale := range localeInputs {
			row := models.CMSLocale{
				Code: normalizeLocale(locale.Code), Name: strings.TrimSpace(locale.Name), Enabled: locale.Enabled,
				IsDefault: locale.IsDefault, FallbackLocale: normalizeLocale(locale.FallbackLocale),
			}
			if err := tx.Select("*").Create(&row).Error; err != nil {
				return err
			}
		}

		for _, page := range bundle.Pages {
			if err := restoreEntry(tx, page.Entry, page.CurrentVersion, page.PublishedVersion, page.LatestPublication); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&page.Page).Error; err != nil {
				return err
			}
			if err := restoreEntryMedia(tx, page.Entry.ID, page.CurrentVersion, page.PublishedVersion); err != nil {
				return err
			}
		}
		for _, navigation := range bundle.Navigation {
			if err := restoreEntry(tx, navigation.Entry, navigation.CurrentVersion, navigation.PublishedVersion, navigation.LatestPublication); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&navigation.Menu).Error; err != nil {
				return err
			}
			for index := range navigation.Items {
				if err := tx.Select("*").Create(&navigation.Items[index]).Error; err != nil {
					return err
				}
			}
		}
		for _, region := range bundle.GlobalRegions {
			if err := restoreEntry(tx, region.Entry, region.CurrentVersion, region.PublishedVersion, region.LatestPublication); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&region.Region).Error; err != nil {
				return err
			}
			if err := restoreEntryMedia(tx, region.Entry.ID, region.CurrentVersion, region.PublishedVersion); err != nil {
				return err
			}
		}
		for _, variant := range bundle.Variants {
			payload, err := ValidateAndNormalizePayload(variant.Payload)
			if err != nil {
				return fmt.Errorf("%w: variant %d: %v", ErrInvalidExport, variant.ID, err)
			}
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			row := models.CMSPageVariant{
				PageID: variant.PageID, EntryID: variant.EntryID, Locale: normalizeLocale(variant.Locale), Market: strings.ToUpper(variant.Market),
				Path: variant.Path, Slug: variant.Slug, Title: variant.Title, DraftPayloadJSON: string(payloadJSON),
				Status: models.CMSVariantStatus(variant.Status), Revision: variant.Revision, SubmittedBy: stringValue(variant.SubmittedBy),
				ApprovedBy: stringValue(variant.ApprovedBy), PublishedAt: variant.PublishedAt,
			}
			row.ID, row.CreatedAt, row.UpdatedAt = variant.ID, variant.CreatedAt, variant.UpdatedAt
			if row.Status == models.CMSVariantStatusPublished {
				row.PublishedPayloadJSON = row.DraftPayloadJSON
			}
			if err := tx.Select("*").Create(&row).Error; err != nil {
				return err
			}
			if _, err := syncVariantMediaReferences(tx, row.ID, payload); err != nil {
				return err
			}
		}
		if err := resetCMSSequences(tx); err != nil {
			return err
		}
		if err := createAuditEvent(tx, 0, nil, nil, "cms.restored", actor, fmt.Sprintf("restored %d pages, %d navigation menus, %d global regions, and %d variants", len(bundle.Pages), len(bundle.Navigation), len(bundle.GlobalRegions), len(bundle.Variants))); err != nil {
			return err
		}
		return createInvalidationEvent(tx, 0, nil, "cms.restored")
	})
}

func validateRestoreBundle(bundle restoreBundle) error {
	entryIDs := map[uint]bool{}
	pageIDs := map[uint]bool{}
	for _, page := range bundle.Pages {
		if page.Page.ID == 0 || page.Entry.ID == 0 || page.Page.EntryID != page.Entry.ID || entryIDs[page.Entry.ID] || pageIDs[page.Page.ID] {
			return fmt.Errorf("%w: duplicate or inconsistent page identifiers", ErrInvalidExport)
		}
		entryIDs[page.Entry.ID], pageIDs[page.Page.ID] = true, true
	}
	for _, navigation := range bundle.Navigation {
		if navigation.Menu.ID == 0 || navigation.Entry.ID == 0 || navigation.Menu.EntryID != navigation.Entry.ID || entryIDs[navigation.Entry.ID] {
			return fmt.Errorf("%w: duplicate or inconsistent navigation identifiers", ErrInvalidExport)
		}
		entryIDs[navigation.Entry.ID] = true
	}
	for _, region := range bundle.GlobalRegions {
		if region.Region.ID == 0 || region.Entry.ID == 0 || region.Region.EntryID != region.Entry.ID || entryIDs[region.Entry.ID] {
			return fmt.Errorf("%w: duplicate or inconsistent global region identifiers", ErrInvalidExport)
		}
		entryIDs[region.Entry.ID] = true
	}
	for _, variant := range bundle.Variants {
		if variant.ID == 0 || !pageIDs[variant.PageID] || !entryIDs[variant.EntryID] {
			return fmt.Errorf("%w: variant references an unknown page or entry", ErrInvalidExport)
		}
	}
	return nil
}

func restoreEntry(tx *gorm.DB, entry models.CMSEntry, current, published *restoreVersion, publication *models.CMSPublication) error {
	entry.CurrentVersion, entry.PublishedVersion, entry.Versions = nil, nil, nil
	if err := tx.Select("*").Create(&entry).Error; err != nil {
		return err
	}
	versions := map[uint]restoreVersion{}
	if current != nil {
		versions[current.ID] = *current
	}
	if published != nil {
		versions[published.ID] = *published
	}
	for _, version := range versions {
		payload, err := ValidateAndNormalizePayload(version.Payload)
		if err != nil {
			return fmt.Errorf("%w: entry %d version %d: %v", ErrInvalidExport, entry.ID, version.ID, err)
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		row := models.CMSEntryVersion{
			ID: version.ID, EntryID: version.EntryID, VersionNumber: version.VersionNumber,
			SchemaVersion: version.SchemaVersion, PayloadJSON: string(payloadJSON), CreatedBy: version.CreatedBy,
			ChangeSummary: stringValue(version.ChangeSummary), CreatedAt: version.CreatedAt,
		}
		if err := tx.Select("*").Create(&row).Error; err != nil {
			return err
		}
	}
	if publication != nil {
		publication.Entry, publication.Version = models.CMSEntry{}, models.CMSEntryVersion{}
		if err := tx.Select("*").Create(publication).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreEntryMedia(tx *gorm.DB, entryID uint, current, published *restoreVersion) error {
	if current != nil {
		payload, err := ValidateAndNormalizePayload(current.Payload)
		if err != nil {
			return fmt.Errorf("%w: invalid current payload", ErrInvalidExport)
		}
		if _, err := syncContentMediaReferences(tx, entryID, payload, media.RoleCMSDraftContent); err != nil {
			return err
		}
	}
	if published != nil {
		payload, err := ValidateAndNormalizePayload(published.Payload)
		if err != nil {
			return fmt.Errorf("%w: invalid published payload", ErrInvalidExport)
		}
		if _, err := syncContentMediaReferences(tx, entryID, payload, media.RoleCMSContent); err != nil {
			return err
		}
	}
	return nil
}

func resetCMSSequences(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	for _, table := range []string{"cms_entries", "cms_entry_versions", "cms_publications", "cms_pages", "cms_navigation_menus", "cms_navigation_items", "cms_global_regions", "cms_page_variants"} {
		statement := fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE((SELECT MAX(id) FROM %s), 1), true)", table, table)
		if err := tx.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
