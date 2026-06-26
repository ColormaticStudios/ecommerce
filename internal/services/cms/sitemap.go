package cms

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Location     string `xml:"loc"`
	LastModified string `xml:"lastmod,omitempty"`
}

func GenerateSitemap(db *gorm.DB, origin string) ([]byte, error) {
	origin = strings.TrimSuffix(strings.TrimSpace(origin), "/")
	if origin == "" {
		return nil, fmt.Errorf("sitemap origin is required")
	}
	set := sitemapURLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: []sitemapURL{}}
	var pages []models.CMSPage
	if err := db.Joins("JOIN cms_entries ON cms_entries.id = cms_pages.entry_id").
		Where("cms_pages.visibility = ? AND cms_entries.published_version_id IS NOT NULL", models.CMSPageVisibilityPublic).
		Order("cms_pages.path ASC").Find(&pages).Error; err != nil {
		return nil, err
	}
	for _, page := range pages {
		var seo models.SEOMetadata
		err := db.Where("entity_type = ? AND entity_id = ?", "cms_page", page.ID).First(&seo).Error
		if err == nil && (seo.NoIndex || strings.HasPrefix(seo.Robots, "noindex")) {
			continue
		}
		location := origin + page.Path
		if err == nil && seo.CanonicalPath != nil && strings.TrimSpace(*seo.CanonicalPath) != "" {
			location = absoluteStorefrontURL(origin, *seo.CanonicalPath)
		}
		set.URLs = append(set.URLs, sitemapURL{Location: location, LastModified: page.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	var products []models.Product
	if err := db.Where("is_published = ?", true).Order("id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	for _, product := range products {
		var seo models.SEOMetadata
		if err := db.Where("entity_type = ? AND entity_id = ?", "product", product.ID).First(&seo).Error; err == nil && seo.NoIndex {
			continue
		}
		set.URLs = append(set.URLs, sitemapURL{Location: fmt.Sprintf("%s/product/%d", origin, product.ID), LastModified: product.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	var categories []models.Category
	if err := db.Where("is_active = ?", true).Order("sort_order ASC, id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	for _, category := range categories {
		set.URLs = append(set.URLs, sitemapURL{Location: origin + "/search?category_slug=" + url.QueryEscape(category.Slug), LastModified: category.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	encoded, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), encoded...), nil
}

func absoluteStorefrontURL(origin, value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.TrimSuffix(origin, "/") + value
}
