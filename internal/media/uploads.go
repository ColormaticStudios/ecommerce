package media

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrMediaNotFound         = errors.New("media not found")
	ErrMediaProcessingFailed = errors.New("media processing failed")
	ErrMediaStillProcessing  = errors.New("media is still processing")
)

func (s *Service) persistProcessingUpload(id string, sizeBytes int64) error {
	var mediaObj models.MediaObject
	if err := s.DB.Where("id = ?", id).First(&mediaObj).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		mediaObj = models.MediaObject{
			ID:        id,
			SizeBytes: sizeBytes,
			Status:    StatusProcessing,
		}
		if err := s.DB.Create(&mediaObj).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) markJobFailed(jobID string) {
	if jobID == "" {
		return
	}
	s.DB.Model(&models.MediaObject{}).Where("id = ?", jobID).Updates(map[string]any{
		"status": StatusFailed,
	})
}

func (s *Service) WaitUntilReady(mediaID string, timeout time.Duration) (models.MediaObject, error) {
	if strings.TrimSpace(mediaID) == "" {
		return models.MediaObject{}, ErrMediaNotFound
	}

	deadline := time.Now().Add(timeout)
	for {
		var mediaObj models.MediaObject
		if err := s.DB.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
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

func (s *Service) ImportFile(filePath string) (models.MediaObject, error) {
	if strings.TrimSpace(filePath) == "" {
		return models.MediaObject{}, errors.New("file path is required")
	}
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

	job := Job{
		ID:        mediaID,
		Source:    incomingPath,
		Filename:  filepath.Base(filePath),
		SizeBytes: info.Size(),
		Metadata: map[string]string{
			"filename": filepath.Base(filePath),
		},
	}
	if err := s.persistProcessingUpload(job.ID, job.SizeBytes); err != nil {
		_ = os.Remove(incomingPath)
		return models.MediaObject{}, err
	}
	if err := s.processJob(job); err != nil {
		s.markJobFailed(job.ID)
		return models.MediaObject{}, err
	}

	return s.WaitUntilReady(job.ID, 0)
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
