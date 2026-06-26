package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SEOInput struct {
	Title               string
	Description         string
	CanonicalURL        string
	Robots              string
	OGTitle             string
	OGDescription       string
	OGImageMediaID      *string
	TwitterCard         string
	TwitterTitle        string
	TwitterDescription  string
	TwitterImageMediaID *string
	JSONLD              []map[string]any
}

type SEORecord struct {
	Metadata models.SEOMetadata
	Issues   []string
}

var allowedJSONLDTypes = map[string]bool{
	"Organization": true, "WebSite": true, "WebPage": true, "BreadcrumbList": true,
	"FAQPage": true, "Product": true,
}

func (s *Service) GetSEO(pageID uint) (*SEORecord, error) {
	page, _, err := loadPageEntry(s.db, pageID, clause.Locking{})
	if err != nil {
		return nil, err
	}
	metadata, err := loadOrDefaultSEO(s.db, page)
	if err != nil {
		return nil, err
	}
	return &SEORecord{Metadata: metadata, Issues: seoIssues(metadata)}, nil
}

func (s *Service) UpdateSEO(pageID uint, input SEOInput) (*SEORecord, error) {
	page, entry, err := loadPageEntry(s.db, pageID, clause.Locking{})
	if err != nil {
		return nil, err
	}
	metadata, err := normalizeSEOInput(page, input)
	if err != nil {
		return nil, err
	}
	var cleanupIDs []string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing models.SEOMetadata
		findErr := tx.Where("entity_type = ? AND entity_id = ?", "cms_page", page.ID).First(&existing).Error
		if findErr == nil {
			metadata.ID = existing.ID
			metadata.CreatedAt = existing.CreatedAt
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if err := tx.Select("*").Save(&metadata).Error; err != nil {
			if isUniqueConstraint(err) {
				return fmt.Errorf("%w: canonical URL is already used", ErrInvalidPage)
			}
			return err
		}
		page.SEOMetadataID = &metadata.ID
		if err := tx.Model(&models.CMSPage{}).Where("id = ?", page.ID).Update("seo_metadata_id", metadata.ID).Error; err != nil {
			return err
		}
		cleanupIDs, err = syncSEOMediaReferences(tx, entry.ID, metadata)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.cleanupOrphanMedia(cleanupIDs)
	return &SEORecord{Metadata: metadata, Issues: seoIssues(metadata)}, nil
}

func loadOrDefaultSEO(db *gorm.DB, page models.CMSPage) (models.SEOMetadata, error) {
	var metadata models.SEOMetadata
	err := db.Where("entity_type = ? AND entity_id = ?", "cms_page", page.ID).First(&metadata).Error
	if err == nil {
		return metadata, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return metadata, err
	}
	title, canonical := page.Title, page.Path
	return models.SEOMetadata{
		EntityType: "cms_page", EntityID: page.ID, Title: &title, CanonicalPath: &canonical,
		Robots: "index_follow", TwitterCard: "summary_large_image", JSONLD: "[]",
	}, nil
}

func normalizeSEOInput(page models.CMSPage, input SEOInput) (models.SEOMetadata, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.CanonicalURL = strings.TrimSpace(input.CanonicalURL)
	if input.CanonicalURL == "" {
		input.CanonicalURL = page.Path
	}
	if !validCanonicalURL(input.CanonicalURL) {
		return models.SEOMetadata{}, fmt.Errorf("%w: canonical URL must be an absolute URL or storefront path", ErrInvalidPage)
	}
	switch input.Robots {
	case "index_follow", "noindex_follow", "index_nofollow", "noindex_nofollow":
	default:
		return models.SEOMetadata{}, fmt.Errorf("%w: invalid robots directive", ErrInvalidPage)
	}
	if input.TwitterCard != "summary" && input.TwitterCard != "summary_large_image" {
		return models.SEOMetadata{}, fmt.Errorf("%w: invalid Twitter card", ErrInvalidPage)
	}
	for index, item := range input.JSONLD {
		typeName, _ := item["@type"].(string)
		if !allowedJSONLDTypes[typeName] {
			return models.SEOMetadata{}, fmt.Errorf("%w: JSON-LD item %d has unsupported type", ErrInvalidPage, index+1)
		}
		item["@context"] = "https://schema.org"
	}
	rawJSONLD, err := json.Marshal(input.JSONLD)
	if err != nil {
		return models.SEOMetadata{}, fmt.Errorf("%w: invalid JSON-LD", ErrInvalidPage)
	}
	title, description, canonical := input.Title, input.Description, input.CanonicalURL
	ogTitle, ogDescription := strings.TrimSpace(input.OGTitle), strings.TrimSpace(input.OGDescription)
	twitterTitle, twitterDescription := strings.TrimSpace(input.TwitterTitle), strings.TrimSpace(input.TwitterDescription)
	return models.SEOMetadata{
		EntityType: "cms_page", EntityID: page.ID, Title: &title, Description: &description,
		CanonicalPath: &canonical, OgImageMediaID: trimStringPointer(input.OGImageMediaID),
		NoIndex: strings.HasPrefix(input.Robots, "noindex"), Robots: input.Robots,
		OGTitle: &ogTitle, OGDescription: &ogDescription, TwitterCard: input.TwitterCard,
		TwitterTitle: &twitterTitle, TwitterDescription: &twitterDescription,
		TwitterImageMediaID: trimStringPointer(input.TwitterImageMediaID), JSONLD: string(rawJSONLD),
	}, nil
}

func validCanonicalURL(value string) bool {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func seoIssues(metadata models.SEOMetadata) []string {
	issues := []string{}
	if metadata.Title == nil || strings.TrimSpace(*metadata.Title) == "" {
		issues = append(issues, "SEO title is missing.")
	} else if len([]rune(*metadata.Title)) > 60 {
		issues = append(issues, "SEO title is longer than 60 characters.")
	}
	if metadata.Description == nil || strings.TrimSpace(*metadata.Description) == "" {
		issues = append(issues, "Meta description is missing.")
	} else if len([]rune(*metadata.Description)) > 160 {
		issues = append(issues, "Meta description is longer than 160 characters.")
	}
	if metadata.CanonicalPath == nil || strings.TrimSpace(*metadata.CanonicalPath) == "" {
		issues = append(issues, "Canonical URL is missing.")
	}
	return issues
}

func syncSEOMediaReferences(tx *gorm.DB, entryID uint, metadata models.SEOMetadata) ([]string, error) {
	ids := []string{}
	for _, value := range []*string{metadata.OgImageMediaID, metadata.TwitterImageMediaID} {
		if value != nil && strings.TrimSpace(*value) != "" {
			ids = append(ids, strings.TrimSpace(*value))
		}
	}
	payload := PagePayload{"blocks": []any{map[string]any{"media_id": firstString(ids)}, map[string]any{"media_id": secondString(ids)}}}
	return syncContentMediaReferences(tx, entryID, payload, media.RoleCMSSEO)
}

func firstString(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func secondString(values []string) string {
	if len(values) > 1 {
		return values[1]
	}
	return ""
}

func trimStringPointer(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
