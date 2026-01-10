package media

import (
	"os"
	"path/filepath"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMediaService(t *testing.T) (*Service, *gorm.DB, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.MediaObject{}, &models.MediaVariant{}, &models.MediaReference{}))

	mediaRoot := t.TempDir()
	service := NewService(db, mediaRoot, "http://localhost:3000/media", nil)
	require.NoError(t, service.EnsureDirs())

	return service, db, mediaRoot
}

func TestPublicURLFor(t *testing.T) {
	service, _, _ := setupMediaService(t)

	url := service.PublicURLFor("abc/original.webp")
	require.Equal(t, "http://localhost:3000/media/abc/original.webp", url)
}

func TestProductMediaURLs(t *testing.T) {
	service, db, _ := setupMediaService(t)

	ready := models.MediaObject{ID: "ready", OriginalPath: "ready/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: StatusReady}
	processing := models.MediaObject{ID: "processing", OriginalPath: "processing/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: StatusProcessing}
	require.NoError(t, db.Create(&ready).Error)
	require.NoError(t, db.Create(&processing).Error)

	refs := []models.MediaReference{
		{MediaID: "ready", OwnerType: OwnerTypeProduct, OwnerID: 1, Role: RoleProductImage},
		{MediaID: "processing", OwnerType: OwnerTypeProduct, OwnerID: 1, Role: RoleProductImage},
	}
	require.NoError(t, db.Create(&refs).Error)

	urls, err := service.ProductMediaURLs(1)
	require.NoError(t, err)
	require.Equal(t, []string{"http://localhost:3000/media/ready/original.webp"}, urls)
}

func TestUserProfilePhotoURLUsesThumbnail(t *testing.T) {
	service, db, _ := setupMediaService(t)

	mediaObj := models.MediaObject{ID: "userphoto", OriginalPath: "userphoto/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: StatusReady}
	variant := models.MediaVariant{MediaID: "userphoto", Label: "thumb_512", Path: "userphoto/variants/thumb_512.webp", MimeType: "image/webp", SizeBytes: 5, Width: 512, Height: 512}
	ref := models.MediaReference{MediaID: "userphoto", OwnerType: OwnerTypeUser, OwnerID: 7, Role: RoleProfilePhoto}

	require.NoError(t, db.Create(&mediaObj).Error)
	require.NoError(t, db.Create(&variant).Error)
	require.NoError(t, db.Create(&ref).Error)

	url, err := service.UserProfilePhotoURL(7)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3000/media/userphoto/variants/thumb_512.webp", url)
}

func TestDeleteIfOrphanRemovesFilesAndRecords(t *testing.T) {
	service, db, mediaRoot := setupMediaService(t)

	mediaObj := models.MediaObject{ID: "orphan", OriginalPath: "orphan/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: StatusReady}
	variant := models.MediaVariant{MediaID: "orphan", Label: "thumb_512", Path: "orphan/variants/thumb_512.webp", MimeType: "image/webp", SizeBytes: 5, Width: 512, Height: 512}
	require.NoError(t, db.Create(&mediaObj).Error)
	require.NoError(t, db.Create(&variant).Error)

	originalPath := filepath.Join(mediaRoot, mediaObj.OriginalPath)
	thumbPath := filepath.Join(mediaRoot, variant.Path)
	require.NoError(t, os.MkdirAll(filepath.Dir(originalPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(thumbPath), 0o755))
	require.NoError(t, os.WriteFile(originalPath, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(thumbPath, []byte("x"), 0o644))

	require.NoError(t, service.DeleteIfOrphan("orphan"))

	var count int64
	require.NoError(t, db.Model(&models.MediaObject{}).Where("id = ?", "orphan").Count(&count).Error)
	require.Equal(t, int64(0), count)

	_, err := os.Stat(originalPath)
	require.Error(t, err)
	_, err = os.Stat(thumbPath)
	require.Error(t, err)
}

func TestDeleteIfOrphanKeepsReferencedMedia(t *testing.T) {
	service, db, _ := setupMediaService(t)

	mediaObj := models.MediaObject{ID: "linked", OriginalPath: "linked/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: StatusReady}
	ref := models.MediaReference{MediaID: "linked", OwnerType: OwnerTypeProduct, OwnerID: 2, Role: RoleProductImage}
	require.NoError(t, db.Create(&mediaObj).Error)
	require.NoError(t, db.Create(&ref).Error)

	require.NoError(t, service.DeleteIfOrphan("linked"))

	var count int64
	require.NoError(t, db.Model(&models.MediaObject{}).Where("id = ?", "linked").Count(&count).Error)
	require.Equal(t, int64(1), count)
}
