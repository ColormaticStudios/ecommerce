package cms

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobalRegionPublishAndDraftPreviewResolution(t *testing.T) {
	service := NewGlobalRegionService(newServiceTestDB(t))

	created, err := service.CreateDraft(GlobalRegionDraftInput{
		Key:    "announcement",
		Title:  "Announcement",
		Region: "announcement_bar",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "promo_banner", "title": "Published banner"}},
		},
	})
	require.NoError(t, err)
	_, err = service.Publish(created.Region.ID, PublishInput{})
	require.NoError(t, err)

	_, err = service.UpdateDraft(created.Region.ID, GlobalRegionDraftInput{
		Key:    "announcement",
		Title:  "Announcement",
		Region: "announcement_bar",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "promo_banner", "title": "Draft banner"}},
		},
	})
	require.NoError(t, err)

	publicRegion, err := service.Resolve("announcement_bar", false)
	require.NoError(t, err)
	require.Contains(t, publicRegion.PublishedVersion.PayloadJSON, "Published banner")

	previewRegion, err := service.Resolve("announcement_bar", true)
	require.NoError(t, err)
	require.Contains(t, previewRegion.CurrentVersion.PayloadJSON, "Draft banner")
}

func TestGlobalRegionPublicResolutionSkipsUnpublishedNewerRegion(t *testing.T) {
	service := NewGlobalRegionService(newServiceTestDB(t))

	published, err := service.CreateDraft(GlobalRegionDraftInput{
		Key:    "published-footer",
		Title:  "Published Footer",
		Region: "footer",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":         "footer",
				"brand_name":   "Published Brand",
				"columns":      []any{},
				"social_links": []any{},
				"copyright":    "Published copyright",
				"layout":       "minimal",
			}},
		},
	})
	require.NoError(t, err)
	_, err = service.Publish(published.Region.ID, PublishInput{})
	require.NoError(t, err)

	_, err = service.CreateDraft(GlobalRegionDraftInput{
		Key:    "draft-footer",
		Title:  "Draft Footer",
		Region: "footer",
		Payload: PagePayload{
			"blocks": []any{map[string]any{
				"type":         "footer",
				"brand_name":   "Draft Brand",
				"columns":      []any{},
				"social_links": []any{},
				"copyright":    "Draft copyright",
				"layout":       "minimal",
			}},
		},
	})
	require.NoError(t, err)

	publicRegion, err := service.Resolve("footer", false)
	require.NoError(t, err)
	require.Contains(t, publicRegion.PublishedVersion.PayloadJSON, "Published Brand")

	previewRegion, err := service.Resolve("footer", true)
	require.NoError(t, err)
	require.Contains(t, previewRegion.CurrentVersion.PayloadJSON, "Draft Brand")
}

func TestGlobalRegionServiceDeleteRemovesRegionFromResolution(t *testing.T) {
	service := NewGlobalRegionService(newServiceTestDB(t))

	created, err := service.CreateDraft(GlobalRegionDraftInput{
		Key:    "temporary-banner",
		Title:  "Temporary banner",
		Region: "sitewide_banner",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "promo_banner", "title": "Temporary"}},
		},
	})
	require.NoError(t, err)
	_, err = service.Publish(created.Region.ID, PublishInput{})
	require.NoError(t, err)

	require.NoError(t, service.Delete(created.Region.ID, nil))

	_, err = service.Get(created.Region.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.Resolve("sitewide_banner", false)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestGlobalRegionServiceUnpublishAndDiscardDraft(t *testing.T) {
	service := NewGlobalRegionService(newServiceTestDB(t))

	created, err := service.CreateDraft(GlobalRegionDraftInput{
		Key:    "lifecycle-banner",
		Title:  "Lifecycle banner",
		Region: "sitewide_banner",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "promo_banner", "title": "Published banner"}},
		},
	})
	require.NoError(t, err)
	published, err := service.Publish(created.Region.ID, PublishInput{})
	require.NoError(t, err)

	unpublished, err := service.Unpublish(created.Region.ID, PublishInput{})
	require.NoError(t, err)
	require.Nil(t, unpublished.Entry.PublishedVersionID)
	_, err = service.Resolve("sitewide_banner", false)
	require.ErrorIs(t, err, ErrNotFound)

	_, err = service.Publish(created.Region.ID, PublishInput{})
	require.NoError(t, err)
	_, err = service.UpdateDraft(created.Region.ID, GlobalRegionDraftInput{
		Key:    "lifecycle-banner",
		Title:  "Lifecycle banner",
		Region: "sitewide_banner",
		Payload: PagePayload{
			"blocks": []any{map[string]any{"type": "promo_banner", "title": "Draft banner"}},
		},
	})
	require.NoError(t, err)
	reverted, deleted, err := service.DiscardDraft(created.Region.ID, PublishInput{})
	require.NoError(t, err)
	require.False(t, deleted)
	require.False(t, reverted.HasUnpublishedDraft)
	require.Equal(t, published.PublishedVersion.ID, reverted.CurrentVersion.ID)
}
