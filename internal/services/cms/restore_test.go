package cms

import (
	"encoding/json"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestRestoreExportReplacesCMSContentAtomically(t *testing.T) {
	db := newServiceTestDB(t)
	service := NewPageService(db)
	original, err := service.CreateDraft(PageDraftInput{
		Path: "/original", Title: "Original",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "rich_text", "body": "Backup content"}}},
	})
	require.NoError(t, err)
	published, err := service.Publish(original.Page.ID, PublishInput{})
	require.NoError(t, err)

	locales, err := service.Locales()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"exported_at":    "2026-06-21T00:00:00Z",
		"locales":        locales,
		"pages": []any{map[string]any{
			"page": published.Page, "entry": published.Entry,
			"current_version": exportVersionForTest(published.CurrentVersion), "published_version": exportVersionForTest(published.PublishedVersion),
			"latest_publication": published.LatestPublication, "has_unpublished_draft": false,
		}},
		"navigation": []any{}, "global_regions": []any{}, "variants": []any{},
	})
	require.NoError(t, err)

	_, err = service.CreateDraft(PageDraftInput{Path: "/discarded", Title: "Discarded", Payload: PagePayload{}})
	require.NoError(t, err)
	require.NoError(t, service.RestoreExport(raw, "publisher-1"))

	restored, err := service.ResolvePublished("/original")
	require.NoError(t, err)
	require.Contains(t, restored.PublishedVersion.PayloadJSON, "Backup content")
	_, err = service.Resolve("/discarded", true)
	require.ErrorIs(t, err, ErrNotFound)

	var audit models.CMSAuditEvent
	require.NoError(t, db.Where("action = ?", "cms.restored").First(&audit).Error)
	var invalidation models.CMSInvalidationEvent
	require.NoError(t, db.Where("reason = ?", "cms.restored").First(&invalidation).Error)
}

func exportVersionForTest(version *models.CMSEntryVersion) map[string]any {
	if version == nil {
		return nil
	}
	var payload PagePayload
	_ = json.Unmarshal([]byte(version.PayloadJSON), &payload)
	return map[string]any{
		"id": version.ID, "entry_id": version.EntryID, "version_number": version.VersionNumber,
		"schema_version": version.SchemaVersion, "payload": payload, "created_by": version.CreatedBy,
		"change_summary": version.ChangeSummary, "created_at": version.CreatedAt,
	}
}

func TestRestoreExportRejectsInvalidBundleWithoutChangingContent(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))
	page, err := service.CreateDraft(PageDraftInput{Path: "/keep", Title: "Keep", Payload: PagePayload{}})
	require.NoError(t, err)

	err = service.RestoreExport([]byte(`{"schema_version":99,"locales":[]}`), "publisher-1")
	require.ErrorIs(t, err, ErrInvalidExport)
	_, err = service.Get(page.Page.ID)
	require.NoError(t, err)
}
