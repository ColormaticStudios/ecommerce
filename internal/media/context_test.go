package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	"ecommerce/internal/jobs"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func contextMediaService(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.MediaObject{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))
	runtime := jobs.NewRuntime(db, jobs.Config{})
	service := NewService(db, t.TempDir(), "/media", log.New(io.Discard, "", 0), runtime)
	require.NoError(t, service.RegisterJobHandlers())
	return service
}

func TestUploadContextCreatePatchAndComplete(t *testing.T) {
	service := contextMediaService(t)
	info, err := service.CreateUpload(context.Background(), 5, map[string]string{"filename": "asset.txt"}, bytes.NewBufferString("ab"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), info.Offset)

	head, err := service.HeadUpload(context.Background(), info.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), head.Offset)

	completed, err := service.PatchUpload(context.Background(), info.ID, 2, bytes.NewBufferString("cde"))
	require.NoError(t, err)
	assert.Equal(t, int64(5), completed.Offset)

	var job models.JobQueue
	require.NoError(t, service.DB.Where("job_type = ? AND idempotency_key = ?", JobTypeProcess, info.ID).First(&job).Error)
	assert.Equal(t, models.JobStatusPending, job.Status)
	var object models.MediaObject
	require.NoError(t, service.DB.First(&object, "id = ?", info.ID).Error)
	assert.Equal(t, StatusProcessing, object.Status)
}

func TestUploadContextRejectsInvalidLengthAndOffset(t *testing.T) {
	service := contextMediaService(t)
	_, err := service.CreateUpload(context.Background(), MaxUploadSizeBytes+1, nil, nil)
	require.ErrorIs(t, err, ErrInvalidUpload)

	info, err := service.CreateUpload(context.Background(), 3, nil, bytes.NewBufferString("a"))
	require.NoError(t, err)
	_, err = service.PatchUpload(context.Background(), info.ID, 0, bytes.NewBufferString("bc"))
	require.ErrorIs(t, err, ErrUploadConflict)

	_, err = service.PatchUpload(context.Background(), info.ID, 1, bytes.NewBufferString("bcd"))
	require.ErrorIs(t, err, ErrInvalidUpload)
	head, err := service.HeadUpload(context.Background(), info.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), head.Offset)
}
