package cms

import (
	"context"
	"io"
	"log"
	"testing"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceMethodsApplyContextToDatabaseOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := NewPageService(newServiceTestDB(t)).List(ctx, 10, 0)
	require.ErrorIs(t, err, context.Canceled)
}

func TestSaveEntryVariantReconcilesReadyMediaReferences(t *testing.T) {
	db := newServiceTestDB(t)
	mediaService := media.NewService(db, t.TempDir(), "/media", log.New(io.Discard, "", 0))
	service := NewPageService(db, mediaService)
	page, err := service.CreateDraft(context.Background(), PageDraftInput{Path: "/variant-owner", Title: "Variant owner", Payload: PagePayload{}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.MediaObject{ID: "ready-image", Status: media.StatusReady, OriginalPath: "ready-image/original.webp", MimeType: "image/webp"}).Error)

	variant, err := service.SaveEntryVariant(context.Background(), page.Entry.ID, 0, EntryVariantInput{
		Locale:  "en-US",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "image", "media_id": "ready-image"}}},
	})
	require.NoError(t, err)

	var reference models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeCMSContentVariant, variant.ID, media.RoleCMSDraftContent).First(&reference).Error)
	assert.Equal(t, "ready-image", reference.MediaID)
}

func TestSaveEntryVariantRejectsProcessingMedia(t *testing.T) {
	db := newServiceTestDB(t)
	service := NewPageService(db)
	page, err := service.CreateDraft(context.Background(), PageDraftInput{Path: "/processing-variant", Title: "Processing variant", Payload: PagePayload{}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.MediaObject{ID: "processing-image", Status: media.StatusProcessing, MimeType: "image/webp"}).Error)

	_, err = service.SaveEntryVariant(context.Background(), page.Entry.ID, 0, EntryVariantInput{
		Locale:  "en-US",
		Payload: PagePayload{"blocks": []any{map[string]any{"type": "image", "media_id": "processing-image"}}},
	})
	require.ErrorIs(t, err, ErrInvalidPage)

	var count int64
	require.NoError(t, db.Model(&models.CMSContentVariant{}).Count(&count).Error)
	assert.Zero(t, count)
}
