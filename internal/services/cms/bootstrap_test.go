package cms

import (
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestBootstrapStarterSiteCreatesPublishedEditableContent(t *testing.T) {
	db := newServiceTestDB(t)

	result, err := BootstrapStarterSite(db)
	require.NoError(t, err)
	require.Equal(t, []string{"/", "/about", "/contact", "/faq"}, result.CreatedPages)
	require.False(t, result.UpgradedHomepage)
	require.True(t, result.CreatedNavigation)
	require.True(t, result.CreatedFooter)

	pages := NewPageService(db)
	for _, path := range result.CreatedPages {
		page, err := pages.ResolvePublished(path)
		require.NoError(t, err)
		require.NotNil(t, page.PublishedVersion)
	}
	home, err := pages.ResolvePublished("/")
	require.NoError(t, err)
	require.Equal(t, "Find your next favorite", home.Page.Title)
	homeBlocks, ok := home.PublishedVersionPayload()["blocks"].([]any)
	require.True(t, ok)
	require.Len(t, homeBlocks, 4)
	require.Equal(t, "hero", homeBlocks[0].(map[string]any)["type"])
	require.Equal(t, "promo_banner", homeBlocks[1].(map[string]any)["type"])
	require.Equal(t, "product_rail", homeBlocks[2].(map[string]any)["type"])
	require.Equal(t, "cta", homeBlocks[3].(map[string]any)["type"])

	var navigation models.CMSNavigationMenu
	require.NoError(t, db.Where("key = ?", "main").First(&navigation).Error)
	navRecord, err := NewNavigationService(db).Get(navigation.ID)
	require.NoError(t, err)
	require.NotNil(t, navRecord.PublishedVersion)
	require.Len(t, navRecord.Items, 5)

	var footer models.CMSGlobalRegion
	require.NoError(t, db.Where("key = ?", "site-footer").First(&footer).Error)
	footerRecord, err := NewGlobalRegionService(db).Get(footer.ID)
	require.NoError(t, err)
	require.NotNil(t, footerRecord.PublishedVersion)
}

func TestBootstrapStarterSiteUpgradesGeneratedLegacyHomepage(t *testing.T) {
	db := newServiceTestDB(t)
	pages := NewPageService(db)
	legacyHome, err := pages.CreateDraft(PageDraftInput{
		Path: "/", Slug: "home", Title: "Home", IsHomepage: true,
		Payload:       PagePayload{"blocks": []any{map[string]any{"type": "hero", "title": "Welcome", "subtitle": ""}}},
		ChangeSummary: "Migrated from legacy storefront",
	})
	require.NoError(t, err)
	_, err = pages.Publish(legacyHome.Page.ID, PublishInput{})
	require.NoError(t, err)

	result, err := BootstrapStarterSite(db)
	require.NoError(t, err)
	require.True(t, result.UpgradedHomepage)
	require.Equal(t, []string{"/about", "/contact", "/faq"}, result.CreatedPages)

	home, err := pages.ResolvePublished("/")
	require.NoError(t, err)
	require.Equal(t, "Find your next favorite", home.Page.Title)
	homeBlocks, ok := home.PublishedVersionPayload()["blocks"].([]any)
	require.True(t, ok)
	require.Equal(t, "Good things, thoughtfully chosen.", homeBlocks[0].(map[string]any)["title"])
}

func TestBootstrapStarterSiteDoesNotOverwriteExistingContent(t *testing.T) {
	db := newServiceTestDB(t)
	pages := NewPageService(db)
	home, err := pages.CreateDraft(PageDraftInput{
		Path: "/", Title: "My custom home", IsHomepage: true,
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Merchant content"}}},
	})
	require.NoError(t, err)
	require.NoError(t, func() error {
		_, err := pages.Publish(home.Page.ID, PublishInput{})
		return err
	}())

	result, err := BootstrapStarterSite(db)
	require.NoError(t, err)
	require.Equal(t, []string{"/about", "/contact", "/faq"}, result.CreatedPages)

	reloaded, err := pages.ResolvePublished("/")
	require.NoError(t, err)
	require.Equal(t, "My custom home", reloaded.Page.Title)

	secondResult, err := BootstrapStarterSite(db)
	require.NoError(t, err)
	require.Empty(t, secondResult.CreatedPages)
	require.False(t, secondResult.CreatedNavigation)
	require.False(t, secondResult.CreatedFooter)
}
