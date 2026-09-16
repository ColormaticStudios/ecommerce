package cms

import (
	"context"
	"strings"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestRedirectServiceResolvesPriorityAndRejectsLoops(t *testing.T) {
	service := NewRedirectService(newServiceTestDB(t))
	_, err := service.Create(context.Background(), RedirectInput{SourcePattern: "/old", MatchType: "prefix", TargetURL: "/product", RedirectType: 301, Priority: 1, IsEnabled: true})
	require.NoError(t, err)
	_, err = service.Create(context.Background(), RedirectInput{SourcePattern: "/old/special", MatchType: "exact", TargetURL: "/search", RedirectType: 302, Priority: 10, IsEnabled: true})
	require.NoError(t, err)

	rule, target, err := service.Resolve(context.Background(), "/old/special")
	require.NoError(t, err)
	require.Equal(t, 302, rule.RedirectType)
	require.Equal(t, "/search", target)
	_, target, err = service.Resolve(context.Background(), "/old/item")
	require.NoError(t, err)
	require.Equal(t, "/product/item", target)

	_, err = service.Create(context.Background(), RedirectInput{SourcePattern: "/search", MatchType: "exact", TargetURL: "/old/special", RedirectType: 301, IsEnabled: true})
	require.ErrorIs(t, err, ErrRedirectLoop)
}

func TestRedirectServiceDeleteRemovesRuleFromResolution(t *testing.T) {
	service := NewRedirectService(newServiceTestDB(t))
	rule, err := service.Create(context.Background(), RedirectInput{SourcePattern: "/old", MatchType: "exact", TargetURL: "/search", RedirectType: 301, IsEnabled: true})
	require.NoError(t, err)

	require.NoError(t, service.Delete(context.Background(), rule.ID))

	_, _, err = service.Resolve(context.Background(), "/old")
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, service.Delete(context.Background(), rule.ID), ErrNotFound)
}

func TestRedirectServicePersistsExplicitlyDisabledState(t *testing.T) {
	service := NewRedirectService(newServiceTestDB(t))
	rule, err := service.Create(context.Background(), RedirectInput{SourcePattern: "/disabled", MatchType: "exact", TargetURL: "/search", RedirectType: 301, IsEnabled: false})
	require.NoError(t, err)
	require.False(t, rule.IsEnabled, "explicitly disabled redirect must persist as disabled")

	_, _, err = service.Resolve(context.Background(), "/disabled")
	require.ErrorIs(t, err, ErrNotFound, "disabled redirects must not resolve")
}

func TestPageSEOValidatesStructuredData(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))
	page, err := service.CreateDraft(context.Background(), PageDraftInput{Path: "/about", Title: "About", Payload: PagePayload{"blocks": []any{}}})
	require.NoError(t, err)

	_, err = service.UpdateSEO(context.Background(), page.Page.ID, SEOInput{
		Title: "About", Description: "About this store", CanonicalURL: "/about", Robots: "index_follow",
		TwitterCard: "summary", JSONLD: []map[string]any{{"@type": "UnknownType"}},
	})
	require.ErrorIs(t, err, ErrInvalidPage)

	record, err := service.UpdateSEO(context.Background(), page.Page.ID, SEOInput{
		Title: "About", Description: "About this store", CanonicalURL: "/about", Robots: "index_follow",
		OGTitle: "About", TwitterCard: "summary_large_image", TwitterTitle: "About",
		JSONLD: []map[string]any{{"@type": "WebPage", "name": "About"}},
	})
	require.NoError(t, err)
	require.Empty(t, record.Issues)
	require.Contains(t, record.Metadata.JSONLD, "https://schema.org")
}

func TestSitemapIncludesOnlyPublishedIndexableContent(t *testing.T) {
	db := newServiceTestDB(t)
	service := NewPageService(db)
	visible, err := service.CreateDraft(context.Background(), PageDraftInput{Path: "/visible", Title: "Visible", Payload: PagePayload{"blocks": []any{}}})
	require.NoError(t, err)
	_, err = service.Publish(context.Background(), visible.Page.ID, PublishInput{})
	require.NoError(t, err)
	hidden, err := service.CreateDraft(context.Background(), PageDraftInput{Path: "/hidden", Title: "Hidden", Payload: PagePayload{"blocks": []any{}}})
	require.NoError(t, err)
	_, err = service.Publish(context.Background(), hidden.Page.ID, PublishInput{})
	require.NoError(t, err)
	_, err = service.UpdateSEO(context.Background(), hidden.Page.ID, SEOInput{Title: "Hidden", Description: "Hidden", CanonicalURL: "/hidden", Robots: "noindex_follow", TwitterCard: "summary", JSONLD: []map[string]any{}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.Product{SKU: "sitemap-product", Name: "Product", IsPublished: true}).Error)
	require.NoError(t, db.Create(&models.Category{Name: "Category", Slug: "category", Path: "/category", IsActive: true}).Error)

	result, err := GenerateSitemap(context.Background(), db, "https://shop.example")
	require.NoError(t, err)
	xml := string(result)
	require.Contains(t, xml, "https://shop.example/visible")
	require.NotContains(t, xml, "https://shop.example/hidden")
	require.Contains(t, xml, "https://shop.example/product/")
	require.True(t, strings.Contains(xml, "category_slug=category") || strings.Contains(xml, "category_slug%3Dcategory"))
}

func TestRedirectCreateRollsBackWhenDisablingFails(t *testing.T) {
	db := newServiceTestDB(t)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_redirect_disable BEFORE UPDATE OF is_enabled ON cms_redirect_rules BEGIN SELECT RAISE(ABORT, 'disable failed'); END`).Error)
	_, err := NewRedirectService(db).Create(context.Background(), RedirectInput{SourcePattern: "/disabled-failure", MatchType: "exact", TargetURL: "/search", RedirectType: 301, IsEnabled: false})
	require.ErrorContains(t, err, "disable failed")
	var count int64
	require.NoError(t, db.Model(&models.CMSRedirectRule{}).Where("source_pattern = ?", "/disabled-failure").Count(&count).Error)
	require.Zero(t, count, "failed creation must not leave an enabled redirect")
}
