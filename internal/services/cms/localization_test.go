package cms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestLocaleConfigurationRejectsFallbackCycles(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))

	_, err := service.UpdateLocales([]LocaleInput{
		{Code: "en-US", Name: "English", Enabled: true, IsDefault: true, FallbackLocale: "fr-FR"},
		{Code: "fr-FR", Name: "French", Enabled: true, FallbackLocale: "en-US"},
	}, "admin-1")
	require.ErrorIs(t, err, ErrInvalidLocale)
}

func TestInvalidationWebhookDeliveryMarksOutboxEventSent(t *testing.T) {
	db := newServiceTestDB(t)
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Get("X-CMS-Event")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	event := models.CMSInvalidationEvent{EntryID: 42, Reason: "page.published", Status: "pending"}
	require.NoError(t, db.Create(&event).Error)
	require.NoError(t, deliverInvalidationEvents(context.Background(), db, server.URL))
	require.Equal(t, "page.published", <-received)
	require.NoError(t, db.First(&event, event.ID).Error)
	require.Equal(t, "sent", event.Status)
	require.NotNil(t, event.SentAt)
}

func TestInvalidationWebhookDeliveryUsesStoredGovernanceURL(t *testing.T) {
	db := newServiceTestDB(t)
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Get("X-CMS-Event")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	require.NoError(t, db.Save(&models.CMSSettings{ID: 1, ApprovalRequired: true, InvalidationWebhookURL: server.URL}).Error)
	event := models.CMSInvalidationEvent{EntryID: 42, Reason: "page.published", Status: "pending"}
	require.NoError(t, db.Create(&event).Error)

	require.NoError(t, deliverInvalidationEvents(context.Background(), db, ""))
	require.Equal(t, "page.published", <-received)
	require.NoError(t, db.First(&event, event.ID).Error)
	require.Equal(t, "sent", event.Status)
}

func TestInvalidationWebhookResolverPrefersStoredGovernanceURL(t *testing.T) {
	db := newServiceTestDB(t)
	require.NoError(t, db.Save(&models.CMSSettings{ID: 1, ApprovalRequired: true, InvalidationWebhookURL: "https://stored.example/hook"}).Error)

	webhookURL, err := resolveInvalidationWebhookURL(db, "https://env.example/hook")

	require.NoError(t, err)
	require.Equal(t, "https://stored.example/hook", webhookURL)
}

func TestPageVariantWorkflowAndLocaleMarketFallback(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))
	_, err := service.UpdateLocales([]LocaleInput{
		{Code: "en-US", Name: "English", Enabled: true, IsDefault: true},
		{Code: "fr", Name: "French", Enabled: true, FallbackLocale: "en-US"},
		{Code: "fr-CA", Name: "French (Canada)", Enabled: true, FallbackLocale: "fr"},
	}, "admin-1")
	require.NoError(t, err)

	page, err := service.CreateDraft(PageDraftInput{
		Path: "/shipping", Title: "Shipping",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Shipping"}}},
	})
	require.NoError(t, err)
	_, err = service.Publish(page.Page.ID, PublishInput{})
	require.NoError(t, err)

	variant, err := service.CreateVariant(page.Page.ID, VariantInput{
		Locale: "fr", Path: "/livraison", Title: "Livraison", Actor: "author-1",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Livraison France"}}},
	})
	require.NoError(t, err)
	_, err = service.TransitionVariant(page.Page.ID, variant.ID, "publish", "publisher-1", "")
	require.ErrorIs(t, err, ErrApprovalRequired)

	variant, err = service.TransitionVariant(page.Page.ID, variant.ID, "submit", "author-1", "Ready for review")
	require.NoError(t, err)
	require.Equal(t, models.CMSVariantStatusInReview, variant.Status)
	_, err = service.TransitionVariantAsRole(page.Page.ID, variant.ID, "approve", "author-1", "author", "")
	require.ErrorIs(t, err, ErrPermissionDenied)
	variant, err = service.TransitionVariant(page.Page.ID, variant.ID, "approve", "editor-1", "Approved")
	require.NoError(t, err)
	variant, err = service.TransitionVariant(page.Page.ID, variant.ID, "publish", "publisher-1", "Published")
	require.NoError(t, err)
	require.Equal(t, models.CMSVariantStatusPublished, variant.Status)

	resolved, localization, err := service.ResolveForLocale("/livraison", "fr-CA", "CA", false)
	require.NoError(t, err)
	require.Equal(t, "Livraison", resolved.Page.Title)
	require.Equal(t, "fr", localization.ResolvedLocale)
	require.True(t, localization.UsedFallback)
	require.Contains(t, resolved.PublishedVersion.PayloadJSON, "Livraison France")

	events, err := service.AuditEvents(page.Entry.ID, 20)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(events), 4)
}
