package media

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tusdhandler "github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMediaService(t *testing.T) (*Service, *gorm.DB, string) {
	t.Helper()

	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.MediaObject{}, &models.MediaVariant{}, &models.MediaReference{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))

	mediaRoot := t.TempDir()
	runtime := jobs.NewRuntime(db, jobs.Config{})
	service := NewService(db, mediaRoot, "http://localhost:3000/media", nil, runtime)
	require.NoError(t, service.RegisterJobHandlers())
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

func TestTusUploadHandlerRejectsUploadsLargerThanLimit(t *testing.T) {
	service, _, _ := setupMediaService(t)
	uploadHandler, err := service.NewTusUploadHandler()
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "", nil)
	require.NoError(t, err)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", strconv.FormatInt(MaxUploadSizeBytes+1, 10))
	res := httptest.NewRecorder()

	uploadHandler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestHandleTusdCompleteQueuesJobAndPersistsProcessingRecord(t *testing.T) {
	service, db, _ := setupMediaService(t)

	uploadID := "upload-123"
	sourcePath := filepath.Join(service.TusDir(), uploadID)
	require.NoError(t, os.WriteFile(sourcePath, []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(sourcePath+".info", []byte("meta"), 0o644))

	err := service.HandleTusdComplete(context.Background(), tusdhandler.FileInfo{
		ID:   uploadID,
		Size: 5,
		MetaData: map[string]string{
			"filename": "photo.jpg",
		},
	})
	require.NoError(t, err)

	incomingPath := filepath.Join(service.IncomingDir(), uploadID)
	_, statErr := os.Stat(incomingPath)
	require.NoError(t, statErr)

	var objCount int64
	require.NoError(t, db.Table("media_objects").Where("id = ?", uploadID).Count(&objCount).Error)
	assert.EqualValues(t, 1, objCount)

	var job models.JobQueue
	require.NoError(t, db.Where("job_type = ? AND idempotency_key = ?", JobTypeProcess, uploadID).First(&job).Error)
	var payload ProcessPayload
	require.NoError(t, json.Unmarshal([]byte(job.PayloadJSON), &payload))
	assert.Equal(t, uploadID, payload.MediaID)
	assert.Equal(t, "photo.jpg", payload.Filename)
	assert.EqualValues(t, 5, payload.SizeBytes)
}

func TestHandleTusdCompleteRequiresUploadID(t *testing.T) {
	service, _, _ := setupMediaService(t)
	err := service.HandleTusdComplete(context.Background(), tusdhandler.FileInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing upload id")
}

func TestImportFilePersistsAndProcessesLocalUpload(t *testing.T) {
	service, db, mediaRoot := setupMediaService(t)

	sourcePath := filepath.Join(t.TempDir(), "manual-upload.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("hello from cli"), 0o644))

	mediaObj, err := service.ImportFile(context.Background(), sourcePath)
	require.NoError(t, err)

	assert.Equal(t, StatusReady, mediaObj.Status)
	assert.Equal(t, "text/plain; charset=utf-8", mediaObj.MimeType)
	assert.Equal(t, "original.txt", filepath.Base(mediaObj.OriginalPath))

	var stored models.MediaObject
	require.NoError(t, db.First(&stored, "id = ?", mediaObj.ID).Error)
	assert.Equal(t, StatusReady, stored.Status)
	var job models.JobQueue
	require.NoError(t, db.Where("job_type = ? AND idempotency_key = ?", JobTypeProcess, mediaObj.ID).First(&job).Error)
	assert.Equal(t, models.JobStatusSucceeded, job.Status)
	var attempt models.JobAttempt
	require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
	assert.Equal(t, models.JobAttemptOutcomeSucceeded, attempt.Outcome)

	outputPath := filepath.Join(mediaRoot, stored.OriginalPath)
	content, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)
	assert.Equal(t, "hello from cli", string(content))

	originalContent, readErr := os.ReadFile(sourcePath)
	require.NoError(t, readErr)
	assert.Equal(t, "hello from cli", string(originalContent))
}

func TestDurableMediaJobProcessesAfterRuntimeRestart(t *testing.T) {
	service, db, mediaRoot := setupMediaService(t)
	info, err := service.CreateUpload(context.Background(), 5, map[string]string{"filename": "asset.txt"}, strings.NewReader("hello"))
	require.NoError(t, err)

	var queued models.JobQueue
	require.NoError(t, db.Where("job_type = ? AND idempotency_key = ?", JobTypeProcess, info.ID).First(&queued).Error)
	restartedRuntime := jobs.NewRuntime(db, jobs.Config{})
	restartedService := NewService(db, mediaRoot, service.PublicURL, nil, restartedRuntime)
	require.NoError(t, restartedService.RegisterJobHandlers())
	require.NoError(t, restartedRuntime.Execute(context.Background(), queued.ID, "restarted-worker"))

	object, err := restartedService.WaitUntilReady(context.Background(), info.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", object.MimeType)
	assert.FileExists(t, filepath.Join(mediaRoot, object.OriginalPath))
}

func TestDeadLetterMarksMediaFailed(t *testing.T) {
	service, db, _ := setupMediaService(t)
	object := models.MediaObject{ID: "missing-input", SizeBytes: 5, Status: StatusProcessing}
	require.NoError(t, db.Create(&object).Error)
	job, err := service.Jobs.Enqueue(context.Background(), jobs.EnqueueInput{
		JobType: JobTypeProcess, IdempotencyKey: object.ID, MaxAttempts: 1,
		Payload: ProcessPayload{Version: ProcessPayloadVersion, MediaID: object.ID, SizeBytes: object.SizeBytes},
	})
	require.NoError(t, err)
	require.Error(t, service.Jobs.Execute(context.Background(), job.ID, "worker-1"))

	require.NoError(t, db.First(&object, "id = ?", object.ID).Error)
	assert.Equal(t, StatusFailed, object.Status)
	var deadLetter models.JobDeadLetter
	require.NoError(t, db.First(&deadLetter, "job_id = ?", job.ID).Error)
	assert.Equal(t, string(reliability.ClassTerminal), deadLetter.ErrorClass)
}

func TestIncompleteMediaPayloadIsTerminal(t *testing.T) {
	service, db, _ := setupMediaService(t)
	job, err := service.Jobs.Enqueue(context.Background(), jobs.EnqueueInput{
		JobType: JobTypeProcess, MaxAttempts: 5, Payload: map[string]any{"version": ProcessPayloadVersion},
	})
	require.NoError(t, err)
	require.Error(t, service.Jobs.Execute(context.Background(), job.ID, "worker-1"))

	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	assert.Equal(t, 1, job.AttemptCount)
}

func TestMissingMediaDatabaseRowIsTerminal(t *testing.T) {
	service, db, _ := setupMediaService(t)
	job, err := service.Jobs.Enqueue(context.Background(), jobs.EnqueueInput{
		JobType: JobTypeProcess, MaxAttempts: 5,
		Payload: ProcessPayload{Version: ProcessPayloadVersion, MediaID: "missing-row"},
	})
	require.NoError(t, err)
	require.Error(t, service.Jobs.Execute(context.Background(), job.ID, "worker-1"))

	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	assert.Equal(t, 1, job.AttemptCount)
}

func TestCorruptImageIsTerminal(t *testing.T) {
	service, db, _ := setupMediaService(t)
	mediaID := "corrupt-image"
	require.NoError(t, db.Create(&models.MediaObject{ID: mediaID, Status: StatusProcessing}).Error)
	require.NoError(t, os.WriteFile(filepath.Join(service.IncomingDir(), mediaID), []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}, 0o600))
	job, err := service.Jobs.Enqueue(context.Background(), jobs.EnqueueInput{
		JobType: JobTypeProcess, MaxAttempts: 5,
		Payload: ProcessPayload{Version: ProcessPayloadVersion, MediaID: mediaID, Filename: "broken.jpg"},
	})
	require.NoError(t, err)
	require.Error(t, service.Jobs.Execute(context.Background(), job.ID, "worker-1"))

	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	assert.Equal(t, 1, job.AttemptCount)
}

func TestTransientMediaIOErrorRemainsRetryable(t *testing.T) {
	classified := classifyMediaProcessError(&os.PathError{Op: "write", Path: "output", Err: syscall.ENOSPC})
	errorClass, ok := reliability.ErrorClassOf(classified)
	require.True(t, ok)
	assert.Equal(t, reliability.ClassRetryable, errorClass)

	classified = classifyMediaProcessError(driver.ErrBadConn)
	errorClass, ok = reliability.ErrorClassOf(classified)
	require.True(t, ok)
	assert.Equal(t, reliability.ClassRetryable, errorClass)
}
