package handlers

import (
	"testing"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachProductMediaToDraftCreatesDraftReference(t *testing.T) {
	db := newTestDB(t)
	mediaService := media.NewService(db, t.TempDir(), "http://localhost:3000/media", nil)
	require.NoError(t, mediaService.EnsureDirs())

	product := models.Product{
		SKU:         "MEDIA-ATTACH-1",
		Name:        "Attach Me",
		Description: "test product",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       3,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	mediaObj := models.MediaObject{
		ID:           "ready-media",
		OriginalPath: "ready-media/original.txt",
		MimeType:     "text/plain",
		SizeBytes:    5,
		Status:       media.StatusReady,
	}
	require.NoError(t, db.Create(&mediaObj).Error)

	err := AttachProductMediaToDraft(db, mediaService, &product, []string{mediaObj.ID})
	require.NoError(t, err)

	var refs []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
		Order("position asc").
		Find(&refs).Error)
	require.Len(t, refs, 1)
	assert.Equal(t, mediaObj.ID, refs[0].MediaID)
	assert.Equal(t, 1, refs[0].Position)

	var draftCount int64
	require.NoError(t, db.Model(&models.ProductDraft{}).Where("product_id = ?", product.ID).Count(&draftCount).Error)
	assert.EqualValues(t, 1, draftCount)
}

func TestAttachProductMediaToDraftRejectsFailedMedia(t *testing.T) {
	db := newTestDB(t)
	mediaService := media.NewService(db, t.TempDir(), "http://localhost:3000/media", nil)
	require.NoError(t, mediaService.EnsureDirs())

	product := models.Product{
		SKU:         "MEDIA-ATTACH-2",
		Name:        "Attach Me",
		Description: "test product",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       3,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "failed-media",
		OriginalPath: "failed-media/original.txt",
		MimeType:     "text/plain",
		SizeBytes:    5,
		Status:       media.StatusFailed,
	}).Error)

	err := AttachProductMediaToDraft(db, mediaService, &product, []string{"failed-media"})
	require.Error(t, err)

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, 422, statusErr.StatusCode)
	assert.Equal(t, "Media processing failed: failed-media", statusErr.Message)

	var refs int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).Count(&refs).Error)
	assert.EqualValues(t, 0, refs)
}
