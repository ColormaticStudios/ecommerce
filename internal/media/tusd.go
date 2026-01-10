package media

import (
	"errors"
	"os"
	"path/filepath"

	"ecommerce/models"

	"github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/gorm"
)

func (s *Service) HandleTusdComplete(info handler.FileInfo) error {
	if info.ID == "" {
		return errors.New("missing upload id")
	}

	incomingDir := s.IncomingDir()
	if err := os.MkdirAll(incomingDir, 0o755); err != nil {
		return err
	}

	sourcePath := filepath.Join(s.TusDir(), info.ID)
	incomingPath := filepath.Join(incomingDir, info.ID)
	if err := os.Rename(sourcePath, incomingPath); err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(s.TusDir(), info.ID+".info"))

	var mediaObj models.MediaObject
	if err := s.DB.Where("id = ?", info.ID).First(&mediaObj).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		mediaObj = models.MediaObject{
			ID:        info.ID,
			SizeBytes: info.Size,
			Status:    StatusProcessing,
		}
		if err := s.DB.Create(&mediaObj).Error; err != nil {
			return err
		}
	}

	s.Queue <- Job{
		ID:        info.ID,
		Source:    incomingPath,
		Filename:  info.MetaData["filename"],
		SizeBytes: info.Size,
		Metadata:  info.MetaData,
	}

	return nil
}
