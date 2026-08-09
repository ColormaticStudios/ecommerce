package cms

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNavigationPublishBlocksMissingInternalPageTarget(t *testing.T) {
	service := NewNavigationService(newServiceTestDB(t))

	created, err := service.CreateDraft(context.Background(), NavigationDraftInput{
		Key:      "main",
		Title:    "Main",
		Location: "header",
		Items: []NavigationItemInput{
			{Label: "Missing", ItemType: "page", TargetRef: "/missing", URL: "/missing", IsEnabled: true},
		},
	})
	require.NoError(t, err)

	_, err = service.Publish(context.Background(), created.Menu.ID, PublishInput{})
	require.ErrorIs(t, err, ErrInvalidPage)
}

func TestNavigationPublishesAndResolvesSnapshot(t *testing.T) {
	db := newServiceTestDB(t)
	pageService := NewPageService(db)
	page, err := pageService.CreateDraft(context.Background(), PageDraftInput{
		Path:    "/shipping",
		Title:   "Shipping",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Shipping"}}},
	})
	require.NoError(t, err)
	_, err = pageService.Publish(context.Background(), page.Page.ID, PublishInput{})
	require.NoError(t, err)

	service := NewNavigationService(db)
	created, err := service.CreateDraft(context.Background(), NavigationDraftInput{
		Key:      "main",
		Title:    "Main",
		Location: "header",
		Items: []NavigationItemInput{
			{Label: "Shipping", ItemType: "page", TargetRef: "/shipping", URL: "/shipping", IsEnabled: true},
		},
	})
	require.NoError(t, err)
	published, err := service.Publish(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)
	require.NotNil(t, published.PublishedVersion)

	_, err = service.UpdateDraft(context.Background(), created.Menu.ID, NavigationDraftInput{
		Key:      "main",
		Title:    "Main",
		Location: "header",
		Items: []NavigationItemInput{
			{Label: "Shipping draft", ItemType: "page", TargetRef: "/shipping", URL: "/shipping", IsEnabled: true},
		},
	})
	require.NoError(t, err)

	resolved, err := service.Resolve(context.Background(), "header", false)
	require.NoError(t, err)
	require.Equal(t, "Shipping", resolved.Items[0].Label)

	preview, err := service.Resolve(context.Background(), "header", true)
	require.NoError(t, err)
	require.Equal(t, "Shipping draft", preview.Items[0].Label)
}

func TestNavigationServiceDeleteRemovesMenuFromResolution(t *testing.T) {
	service := NewNavigationService(newServiceTestDB(t))

	created, err := service.CreateDraft(context.Background(), NavigationDraftInput{
		Key:      "footer",
		Title:    "Footer",
		Location: "footer",
		Items: []NavigationItemInput{
			{Label: "Search", ItemType: "internal", TargetRef: "/search", URL: "/search", IsEnabled: true},
		},
	})
	require.NoError(t, err)
	_, err = service.Publish(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)

	require.NoError(t, service.Delete(context.Background(), created.Menu.ID, nil))

	_, err = service.Get(context.Background(), created.Menu.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.Resolve(context.Background(), "footer", false)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestNavigationServiceUnpublishAndDiscardDraft(t *testing.T) {
	service := NewNavigationService(newServiceTestDB(t))

	created, err := service.CreateDraft(context.Background(), NavigationDraftInput{
		Key:      "main",
		Title:    "Main",
		Location: "header",
		Items: []NavigationItemInput{
			{Label: "Search", ItemType: "internal", TargetRef: "/search", URL: "/search", IsEnabled: true},
		},
	})
	require.NoError(t, err)
	published, err := service.Publish(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)

	unpublished, err := service.Unpublish(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)
	require.Nil(t, unpublished.Entry.PublishedVersionID)
	_, err = service.Resolve(context.Background(), "header", false)
	require.ErrorIs(t, err, ErrNotFound)

	_, err = service.Publish(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)
	_, err = service.UpdateDraft(context.Background(), created.Menu.ID, NavigationDraftInput{
		Key:      "main",
		Title:    "Main",
		Location: "header",
		Items: []NavigationItemInput{
			{Label: "Draft search", ItemType: "internal", TargetRef: "/search", URL: "/search", IsEnabled: true},
		},
	})
	require.NoError(t, err)
	reverted, deleted, err := service.DiscardDraft(context.Background(), created.Menu.ID, PublishInput{})
	require.NoError(t, err)
	require.False(t, deleted)
	require.False(t, reverted.HasUnpublishedDraft)
	require.Equal(t, published.PublishedVersion.ID, reverted.CurrentVersion.ID)
}
