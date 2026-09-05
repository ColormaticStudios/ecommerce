package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrMediaNotFound         = errors.New("media not found")
	ErrMediaProcessingFailed = errors.New("media processing failed")
	ErrMediaStillProcessing  = errors.New("media is still processing")
)

func (s *Service) persistProcessingUploadTx(tx *gorm.DB, id string, sizeBytes int64) error {
	var mediaObj models.MediaObject
	if err := tx.Where("id = ?", id).First(&mediaObj).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		mediaObj = models.MediaObject{
			ID:        id,
			SizeBytes: sizeBytes,
			Status:    StatusProcessing,
		}
		if err := tx.Create(&mediaObj).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) WaitUntilReady(ctx context.Context, mediaID string, timeout time.Duration) (models.MediaObject, error) {
	if strings.TrimSpace(mediaID) == "" {
		return models.MediaObject{}, ErrMediaNotFound
	}

	deadline := time.Now().Add(timeout)
	db := s.DB.WithContext(ctx)
	for {
		var mediaObj models.MediaObject
		if err := db.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if timeout > 0 && time.Now().Before(deadline) {
					time.Sleep(150 * time.Millisecond)
					continue
				}
				return models.MediaObject{}, ErrMediaNotFound
			}
			return models.MediaObject{}, err
		}

		if mediaObj.Status == StatusReady && mediaObj.OriginalPath != "" {
			return mediaObj, nil
		}
		if mediaObj.Status == StatusFailed {
			return mediaObj, ErrMediaProcessingFailed
		}
		if timeout > 0 && time.Now().Before(deadline) {
			time.Sleep(150 * time.Millisecond)
			continue
		}

		return mediaObj, ErrMediaStillProcessing
	}
}

func (s *Service) ImportFile(ctx context.Context, filePath string) (models.MediaObject, error) {
	if strings.TrimSpace(filePath) == "" {
		return models.MediaObject{}, errors.New("file path is required")
	}
	s = s.withContext(ctx)
	if err := s.EnsureDirs(); err != nil {
		return models.MediaObject{}, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return models.MediaObject{}, err
	}
	if !info.Mode().IsRegular() {
		return models.MediaObject{}, fmt.Errorf("file path must point to a regular file: %s", filePath)
	}

	mediaID := uuid.NewString()
	incomingPath := filepath.Join(s.IncomingDir(), mediaID)
	if err := copyFile(filePath, incomingPath); err != nil {
		return models.MediaObject{}, err
	}

	payload := ProcessPayload{
		Version:   ProcessPayloadVersion,
		MediaID:   mediaID,
		Filename:  filepath.Base(filePath),
		SizeBytes: info.Size(),
		Metadata: map[string]string{
			"filename": filepath.Base(filePath),
		},
	}
	job, err := s.enqueueProcessing(ctx, payload)
	if err != nil {
		_ = os.Remove(incomingPath)
		return models.MediaObject{}, err
	}
	if err := s.Jobs.Execute(ctx, job.ID, "media-import"); err != nil {
		return models.MediaObject{}, err
	}
	// Execute is deliberately synchronous here, so a zero-timeout readiness
	// check observes the final state without polling.
	return s.WaitUntilReady(ctx, mediaID, 0)
}

func (s *Service) enqueueProcessing(ctx context.Context, payload ProcessPayload) (models.JobQueue, error) {
	if s.Jobs == nil {
		return models.JobQueue{}, errors.New("media processing requires the shared job runtime")
	}
	var job models.JobQueue
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.persistProcessingUploadTx(tx, payload.MediaID, payload.SizeBytes); err != nil {
			return err
		}
		var err error
		job, err = s.Jobs.EnqueueTx(ctx, tx, jobs.EnqueueInput{
			JobType: JobTypeProcess, Payload: payload, IdempotencyKey: payload.MediaID,
		})
		return err
	})
	return job, err
}

func copyFile(src string, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		if targetFile != nil {
			_ = targetFile.Close()
		}
	}()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}

	if err := targetFile.Close(); err != nil {
		return err
	}
	targetFile = nil
	return nil
}
