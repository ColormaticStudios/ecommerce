package cms

import (
	"errors"

	"ecommerce/models"

	"gorm.io/gorm"
)

// BootstrapResult lists the starter content created during a bootstrap run.
// Existing content is never changed.
type BootstrapResult struct {
	CreatedPages      []string
	UpgradedHomepage  bool
	CreatedNavigation bool
	CreatedFooter     bool
}

// BootstrapStarterSite creates a usable, published CMS site when the corresponding
// page paths and global/navigation keys do not already exist. It is safe to run again:
// existing entries are left untouched so merchant-authored content is preserved.
func BootstrapStarterSite(db *gorm.DB) (BootstrapResult, error) {
	result := BootstrapResult{CreatedPages: []string{}}
	pageService := NewPageService(db)

	for _, page := range starterPages() {
		var existing models.CMSPage
		err := db.Where("path = ?", page.Path).First(&existing).Error
		switch {
		case err == nil:
			if page.Path == "/" && isGeneratedLegacyHomepage(pageService, existing.ID) {
				record, err := pageService.UpdateDraft(existing.ID, page)
				if err != nil {
					return result, err
				}
				if _, err := pageService.Publish(record.Page.ID, PublishInput{Notes: "Replace generated legacy homepage with starter site"}); err != nil {
					return result, err
				}
				result.UpgradedHomepage = true
			}
			continue
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return result, err
		}

		record, err := pageService.CreateDraft(page)
		if err != nil {
			return result, err
		}
		if _, err := pageService.Publish(record.Page.ID, PublishInput{Notes: "Starter site bootstrap"}); err != nil {
			return result, err
		}
		result.CreatedPages = append(result.CreatedPages, page.Path)
	}

	var existingNavigation models.CMSNavigationMenu
	err := db.Where("key = ?", "main").First(&existingNavigation).Error
	switch {
	case err == nil:
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return result, err
	default:
		navigationService := NewNavigationService(db)
		record, err := navigationService.CreateDraft(starterNavigation())
		if err != nil {
			return result, err
		}
		if _, err := navigationService.Publish(record.Menu.ID, PublishInput{Notes: "Starter site bootstrap"}); err != nil {
			return result, err
		}
		result.CreatedNavigation = true
	}

	var existingFooter models.CMSGlobalRegion
	err = db.Where("key = ?", "site-footer").First(&existingFooter).Error
	switch {
	case err == nil:
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return result, err
	default:
		globalService := NewGlobalRegionService(db)
		record, err := globalService.CreateDraft(starterFooter())
		if err != nil {
			return result, err
		}
		if _, err := globalService.Publish(record.Region.ID, PublishInput{Notes: "Starter site bootstrap"}); err != nil {
			return result, err
		}
		result.CreatedFooter = true
	}

	return result, nil
}

// isGeneratedLegacyHomepage identifies only the fallback page created by the
// storefront-to-CMS migration when no legacy homepage configuration existed. It
// deliberately does not match migrated custom content or merchant-edited pages.
func isGeneratedLegacyHomepage(service *Service, pageID uint) bool {
	record, err := service.Get(pageID)
	if err != nil || record.HasUnpublishedDraft || record.Page.Title != "Home" || record.PublishedVersion == nil {
		return false
	}
	if record.PublishedVersion.ChangeSummary != "Migrated from legacy storefront" {
		return false
	}
	payload, err := payloadFromVersion(*record.PublishedVersion)
	if err != nil {
		return false
	}
	blocks, ok := payload["blocks"].([]any)
	if !ok || len(blocks) != 1 {
		return false
	}
	hero, ok := blocks[0].(map[string]any)
	return ok && hero["type"] == "hero" && hero["title"] == "Welcome" && hero["subtitle"] == ""
}

func starterPages() []PageDraftInput {
	return []PageDraftInput{
		{
			Path: "/", Slug: "home", Title: "Find your next favorite", Visibility: string(models.CMSPageVisibilityPublic), IsHomepage: true,
			Payload: PagePayload{"blocks": []any{
				map[string]any{
					"type": "hero", "title": "Good things, thoughtfully chosen.",
					"subtitle":    "Discover new arrivals and everyday favorites selected for your store.",
					"primary_cta": map[string]any{"label": "Shop new arrivals", "url": "/search"},
				},
				map[string]any{
					"type": "promo_banner", "title": "Make this storefront your own",
					"body": "Update this campaign message, add imagery, and feature the products your customers should see first.",
					"link": map[string]any{"label": "Browse the collection", "url": "/search"},
				},
				map[string]any{"type": "product_rail", "title": "Fresh finds", "subtitle": "The newest additions to the collection.", "source": "newest", "limit": 8},
				map[string]any{
					"type": "cta", "label": "Meet the people behind the store", "url": "/about",
					"body": "Share your point of view, what you make, and why your customers will love shopping with you.",
				},
			}, "change_summary": "Create starter homepage"},
		},
		{
			Path: "/about", Slug: "about", Title: "About us", Visibility: string(models.CMSPageVisibilityPublic),
			Payload: PagePayload{"blocks": []any{
				map[string]any{"type": "hero", "title": "About us", "subtitle": "Share the story behind your store."},
				map[string]any{"type": "rich_text", "body": "Use this page to introduce your business, your values, and the people behind your products."},
			}, "change_summary": "Create starter about page"},
		},
		{
			Path: "/contact", Slug: "contact", Title: "Contact", Visibility: string(models.CMSPageVisibilityPublic),
			Payload: PagePayload{"blocks": []any{
				map[string]any{"type": "hero", "title": "Contact us", "subtitle": "We are here to help."},
				map[string]any{"type": "rich_text", "body": "Add your preferred contact details, business hours, and customer support information here."},
			}, "change_summary": "Create starter contact page"},
		},
		{
			Path: "/faq", Slug: "faq", Title: "Frequently asked questions", Visibility: string(models.CMSPageVisibilityPublic),
			Payload: PagePayload{"blocks": []any{
				map[string]any{"type": "hero", "title": "Frequently asked questions", "subtitle": "Helpful information for your customers."},
				map[string]any{"type": "faq", "items": []any{
					map[string]any{"question": "When will my order ship?", "answer": "Add your shipping and fulfillment policy here."},
					map[string]any{"question": "How can I contact support?", "answer": "Add your support contact details here."},
				}},
			}, "change_summary": "Create starter FAQ page"},
		},
	}
}

func starterNavigation() NavigationDraftInput {
	return NavigationDraftInput{
		Key: "main", Title: "Main navigation", Location: "primary", ChangeSummary: "Create starter navigation",
		Items: []NavigationItemInput{
			{Label: "Home", ItemType: string(models.CMSNavigationItemTypePage), TargetRef: "/", SortOrder: 0, IsEnabled: true},
			{Label: "Shop", ItemType: string(models.CMSNavigationItemTypeInternal), URL: "/search", SortOrder: 1, IsEnabled: true},
			{Label: "About", ItemType: string(models.CMSNavigationItemTypePage), TargetRef: "/about", SortOrder: 2, IsEnabled: true},
			{Label: "Contact", ItemType: string(models.CMSNavigationItemTypePage), TargetRef: "/contact", SortOrder: 3, IsEnabled: true},
			{Label: "FAQ", ItemType: string(models.CMSNavigationItemTypePage), TargetRef: "/faq", SortOrder: 4, IsEnabled: true},
		},
	}
}

func starterFooter() GlobalRegionDraftInput {
	return GlobalRegionDraftInput{
		Key: "site-footer", Title: "Site footer", Region: "footer", ChangeSummary: "Create starter footer",
		Payload: PagePayload{"blocks": []any{map[string]any{
			"type": "footer", "brand_name": "Your store", "tagline": "", "layout": "columns",
			"columns": []any{
				map[string]any{"title": "Shop", "links": []any{map[string]any{"label": "Browse products", "url": "/search"}}},
				map[string]any{"title": "Information", "links": []any{map[string]any{"label": "About us", "url": "/about"}, map[string]any{"label": "Contact", "url": "/contact"}, map[string]any{"label": "FAQ", "url": "/faq"}}},
			},
			"social_links": []any{}, "copyright": "© Your store",
		}}},
	}
}
