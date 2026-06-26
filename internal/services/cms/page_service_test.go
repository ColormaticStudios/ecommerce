package cms

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"ecommerce/internal/media"
	"ecommerce/internal/migrations"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrations.RunWithoutContract(db))
	return db
}

func TestPageServiceCreatePublishResolveAndRollback(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:    "/shipping",
		Title:   "Shipping",
		Payload: PagePayload{"headline": "Draft shipping"},
	})
	require.NoError(t, err)
	require.True(t, created.HasUnpublishedDraft)

	_, err = service.ResolvePublished("/shipping")
	require.ErrorIs(t, err, ErrNotFound)

	published, err := service.Publish(created.Page.ID, PublishInput{Notes: "go live"})
	require.NoError(t, err)
	require.False(t, published.HasUnpublishedDraft)
	require.NotNil(t, published.PublishedVersion)
	require.Equal(t, "Draft shipping", published.PublishedVersionPayload()["headline"])

	updated, err := service.UpdateDraft(created.Page.ID, PageDraftInput{
		Path:    "/shipping",
		Title:   "Shipping",
		Payload: PagePayload{"headline": "Updated shipping"},
	})
	require.NoError(t, err)
	require.True(t, updated.HasUnpublishedDraft)

	resolved, err := service.ResolvePublished("/shipping")
	require.NoError(t, err)
	require.Equal(t, "Draft shipping", resolved.PublishedVersionPayload()["headline"])

	rolledBack, err := service.Rollback(created.Page.ID, RollbackInput{VersionID: published.PublishedVersion.ID})
	require.NoError(t, err)
	require.False(t, rolledBack.HasUnpublishedDraft)
	require.Equal(t, published.PublishedVersion.ID, rolledBack.PublishedVersion.ID)
}

func TestPageServiceDeleteRemovesPageFromResolution(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:    "/delete-me",
		Title:   "Delete me",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Temporary"}}},
	})
	require.NoError(t, err)
	_, err = service.Publish(created.Page.ID, PublishInput{})
	require.NoError(t, err)

	require.NoError(t, service.Delete(created.Page.ID, nil))

	_, err = service.Get(created.Page.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.ResolvePublished("/delete-me")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPageServiceUnpublishAndDiscardDraft(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:    "/lifecycle",
		Title:   "Lifecycle",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Published"}}},
	})
	require.NoError(t, err)
	published, err := service.Publish(created.Page.ID, PublishInput{})
	require.NoError(t, err)

	unpublished, err := service.Unpublish(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	require.Nil(t, unpublished.Entry.PublishedVersionID)
	_, err = service.ResolvePublished("/lifecycle")
	require.ErrorIs(t, err, ErrNotFound)

	_, err = service.Publish(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	_, err = service.UpdateDraft(created.Page.ID, PageDraftInput{
		Path:    "/lifecycle",
		Title:   "Lifecycle",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Draft"}}},
	})
	require.NoError(t, err)
	reverted, deleted, err := service.DiscardDraft(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	require.False(t, deleted)
	require.False(t, reverted.HasUnpublishedDraft)
	require.Equal(t, published.PublishedVersion.ID, reverted.CurrentVersion.ID)
}

func TestPageServiceDiscardDraftOnlyDeletesPage(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:    "/draft-only",
		Title:   "Draft only",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Draft"}}},
	})
	require.NoError(t, err)
	_, deleted, err := service.DiscardDraft(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	require.True(t, deleted)
	_, err = service.Get(created.Page.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPageServiceRejectsDuplicatePath(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	_, err := service.CreateDraft(PageDraftInput{
		Path:    "/about",
		Title:   "About",
		Payload: PagePayload{"body": "one"},
	})
	require.NoError(t, err)

	_, err = service.CreateDraft(PageDraftInput{
		Path:    "about",
		Title:   "About duplicate",
		Payload: PagePayload{"body": "two"},
	})
	require.ErrorIs(t, err, ErrDuplicatePath)
}

func TestPageServiceValidatesBlocksAndSanitizesCustomHTML(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:  "/privacy",
		Title: "Privacy",
		Payload: PagePayload{
			"blocks": []any{
				map[string]any{"type": "rich_text", "body": "Plain policy copy."},
				map[string]any{
					"type": "custom_html",
					"html": `<p onclick="alert(1)">Allowed</p><script>alert(1)</script>`,
				},
			},
		},
	})
	require.NoError(t, err)
	payload := jsonObjectForTest(created.CurrentVersion.PayloadJSON)
	blocks := payload["blocks"].([]any)
	customHTML := blocks[1].(map[string]any)["html"].(string)
	require.NotContains(t, customHTML, "script")
	require.NotContains(t, customHTML, "onclick")

	_, err = service.CreateDraft(PageDraftInput{
		Path:  "/broken",
		Title: "Broken",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "hero"}},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPage)
}

func TestPageServiceValidatesProductRailBlocks(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:  "/campaign",
		Title: "Campaign",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":          "product_rail",
				"title":         "New arrivals",
				"source":        "newest",
				"limit":         8,
				"sort":          "created_at",
				"order":         "desc",
				"image_aspect":  "square",
				"product_ids":   []any{},
				"category_slug": "",
			}},
		},
	})
	require.NoError(t, err)
	payload := jsonObjectForTest(created.CurrentVersion.PayloadJSON)
	block := payload["blocks"].([]any)[0].(map[string]any)
	require.Equal(t, "product_rail", block["type"])
	require.Equal(t, float64(8), block["limit"])

	_, err = service.CreateDraft(PageDraftInput{
		Path:  "/broken-campaign",
		Title: "Broken campaign",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":   "product_rail",
				"title":  "Bad rail",
				"source": "category",
				"limit":  8,
			}},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPage)
}

func TestPageServiceValidatesCommerceCampaignBlocks(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	created, err := service.CreateDraft(PageDraftInput{
		Path:  "/summer-sale",
		Title: "Summer sale",
		Payload: PagePayload{
			"blocks": []any{
				map[string]any{
					"type":           "category_tiles",
					"title":          "Shop categories",
					"category_slugs": []any{"new-arrivals", "sale"},
					"image_aspect":   "wide",
				},
				map[string]any{
					"type":           "promotion_highlight",
					"title":          "Take 20% off",
					"promotion_code": "SUMMER20",
					"link":           map[string]any{"label": "Shop sale", "url": "/search"},
				},
				map[string]any{
					"type":                "inventory_message",
					"product_id":          float64(42),
					"low_stock_threshold": float64(4),
				},
				map[string]any{
					"type":        "testimonial",
					"quote":       "Great launch.",
					"attribution": "A customer",
					"rating":      float64(5),
				},
				map[string]any{
					"type":     "social_embed",
					"provider": "youtube",
					"url":      "https://www.youtube.com/watch?v=abc123",
				},
			},
		},
	})
	require.NoError(t, err)
	payload := jsonObjectForTest(created.CurrentVersion.PayloadJSON)
	blocks := payload["blocks"].([]any)
	require.Equal(t, []any{"new-arrivals", "sale"}, blocks[0].(map[string]any)["category_slugs"])
	require.Equal(t, float64(42), blocks[2].(map[string]any)["product_id"])

	_, err = service.CreateDraft(PageDraftInput{
		Path:  "/bad-social",
		Title: "Bad social",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":     "social_embed",
				"provider": "instagram",
				"url":      "https://example.com/not-instagram",
			}},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPage)

	_, err = service.CreateDraft(PageDraftInput{
		Path:  "/bad-category-tiles",
		Title: "Bad category tiles",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":           "category_tiles",
				"title":          "Empty",
				"category_slugs": []any{""},
			}},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPage)
}

func TestPageServiceTracksDraftAndLiveMediaReferences(t *testing.T) {
	db := newServiceTestDB(t)
	mediaService := media.NewService(db, t.TempDir(), "/media", nil)
	for _, id := range []string{"cms-old", "cms-new"} {
		require.NoError(t, db.Create(&models.MediaObject{
			ID: id, OriginalPath: id + "/original.webp", MimeType: "image/webp", Status: media.StatusReady,
		}).Error)
	}
	service := NewPageService(db, mediaService)
	created, err := service.CreateDraft(PageDraftInput{
		Path: "/media-page", Title: "Media page",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "image", "media_id": "cms-old"}}},
	})
	require.NoError(t, err)
	requireCMSMediaReference(t, db, created.Entry.ID, media.RoleCMSDraftContent, "cms-old")

	_, err = service.Publish(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	requireCMSMediaReference(t, db, created.Entry.ID, media.RoleCMSContent, "cms-old")

	_, err = service.UpdateDraft(created.Page.ID, PageDraftInput{
		Path: "/media-page", Title: "Media page",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "image", "media_id": "cms-new"}}},
	})
	require.NoError(t, err)
	requireCMSMediaReference(t, db, created.Entry.ID, media.RoleCMSDraftContent, "cms-new")
	var oldCount int64
	require.NoError(t, db.Model(&models.MediaObject{}).Where("id = ?", "cms-old").Count(&oldCount).Error)
	require.EqualValues(t, 1, oldCount, "published media must not be cleaned while still live")

	_, err = service.Publish(created.Page.ID, PublishInput{})
	require.NoError(t, err)
	requireCMSMediaReference(t, db, created.Entry.ID, media.RoleCMSContent, "cms-new")
	require.NoError(t, db.Model(&models.MediaObject{}).Where("id = ?", "cms-old").Count(&oldCount).Error)
	require.Zero(t, oldCount, "replaced unreferenced media should be cleaned")
}

func requireCMSMediaReference(t *testing.T, db *gorm.DB, entryID uint, role, mediaID string) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.MediaReference{}).
		Where("owner_type = ? AND owner_id = ? AND role = ? AND media_id = ?", media.OwnerTypeCMSEntry, entryID, role, mediaID).
		Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func (r *PageRecord) PublishedVersionPayload() map[string]any {
	if r.PublishedVersion == nil {
		return nil
	}
	return jsonObjectForTest(r.PublishedVersion.PayloadJSON)
}

func jsonObjectForTest(raw string) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
