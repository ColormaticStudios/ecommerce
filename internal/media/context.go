package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidUpload  = errors.New("invalid media upload")
	ErrUploadConflict = errors.New("media upload offset conflict")
)

type UploadInfo struct {
	ID       string            `json:"id"`
	Size     int64             `json:"size"`
	Offset   int64             `json:"offset"`
	Metadata map[string]string `json:"metadata"`
}

func (s *Service) withContext(ctx context.Context) *Service {
	copy := *s
	copy.DB = s.DB.WithContext(ctx)
	return &copy
}

func (s *Service) Lookup(ctx context.Context, mediaID string) (MediaObject, error) {
	return lookupMediaObject(s.DB.WithContext(ctx), mediaID)
}

// MediaObject is an alias used by context-first media APIs without exposing HTTP metadata.
type MediaObject = models.MediaObject

func lookupMediaObject(db *gorm.DB, mediaID string) (models.MediaObject, error) {
	if strings.TrimSpace(mediaID) == "" {
		return models.MediaObject{}, ErrMediaNotFound
	}
	var object models.MediaObject
	if err := db.First(&object, "id = ?", mediaID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.MediaObject{}, ErrMediaNotFound
		}
		return models.MediaObject{}, err
	}
	if object.Status == StatusFailed {
		return object, ErrMediaProcessingFailed
	}
	if object.Status != StatusReady || strings.TrimSpace(object.OriginalPath) == "" {
		return object, ErrMediaStillProcessing
	}
	return object, nil
}

func (s *Service) CreateUpload(ctx context.Context, size int64, metadata map[string]string, body io.Reader) (UploadInfo, error) {
	if size < 0 || size > MaxUploadSizeBytes {
		return UploadInfo{}, fmt.Errorf("%w: upload length must be between 0 and %d", ErrInvalidUpload, MaxUploadSizeBytes)
	}
	if err := s.EnsureDirs(); err != nil {
		return UploadInfo{}, err
	}
	info := UploadInfo{ID: uuid.NewString(), Size: size, Metadata: cloneMetadata(metadata)}
	filePath := filepath.Join(s.TusDir(), info.ID)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return UploadInfo{}, err
	}
	if body != nil {
		info.Offset, err = io.Copy(file, io.LimitReader(body, size+1))
	}
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(filePath)
		return UploadInfo{}, err
	}
	if closeErr != nil {
		_ = os.Remove(filePath)
		return UploadInfo{}, closeErr
	}
	if info.Offset > size {
		_ = os.Remove(filePath)
		return UploadInfo{}, fmt.Errorf("%w: request body exceeds upload length", ErrInvalidUpload)
	}
	if err := writeUploadInfo(s.TusDir(), info); err != nil {
		_ = os.Remove(filePath)
		return UploadInfo{}, err
	}
	if info.Offset == info.Size {
		if err := s.withContext(ctx).completeUploadContext(ctx, info); err != nil {
			return UploadInfo{}, err
		}
	}
	return info, nil
}

func (s *Service) HeadUpload(ctx context.Context, id string) (UploadInfo, error) {
	if err := ctx.Err(); err != nil {
		return UploadInfo{}, err
	}
	return readUploadInfo(s.TusDir(), id)
}

func (s *Service) PatchUpload(ctx context.Context, id string, expectedOffset int64, body io.Reader) (UploadInfo, error) {
	info, err := readUploadInfo(s.TusDir(), id)
	if err != nil {
		return UploadInfo{}, err
	}
	if expectedOffset >= 0 && expectedOffset != info.Offset {
		return UploadInfo{}, fmt.Errorf("%w: expected %d, received %d", ErrUploadConflict, info.Offset, expectedOffset)
	}
	file, err := os.OpenFile(filepath.Join(s.TusDir(), info.ID), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return UploadInfo{}, err
	}
	remaining := info.Size - info.Offset
	written, copyErr := io.Copy(file, io.LimitReader(body, remaining+1))
	closeErr := file.Close()
	if copyErr != nil {
		return UploadInfo{}, copyErr
	}
	if closeErr != nil {
		return UploadInfo{}, closeErr
	}
	if written > remaining {
		if truncateErr := os.Truncate(filepath.Join(s.TusDir(), info.ID), info.Offset); truncateErr != nil {
			return UploadInfo{}, truncateErr
		}
		return UploadInfo{}, fmt.Errorf("%w: chunk exceeds upload length", ErrInvalidUpload)
	}
	info.Offset += written
	if err := writeUploadInfo(s.TusDir(), info); err != nil {
		return UploadInfo{}, err
	}
	if info.Offset == info.Size {
		if err := s.withContext(ctx).completeUploadContext(ctx, info); err != nil {
			return UploadInfo{}, err
		}
	}
	return info, nil
}

func (s *Service) completeUploadContext(ctx context.Context, info UploadInfo) error {
	incomingPath := filepath.Join(s.IncomingDir(), info.ID)
	if err := os.Rename(filepath.Join(s.TusDir(), info.ID), incomingPath); err != nil {
		return err
	}
	_ = os.Remove(uploadInfoPath(s.TusDir(), info.ID))
	if err := s.persistProcessingUpload(info.ID, info.Size); err != nil {
		return err
	}
	job := Job{ID: info.ID, Source: incomingPath, Filename: info.Metadata["filename"], SizeBytes: info.Size, Metadata: info.Metadata}
	select {
	case s.Queue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func readUploadInfo(dir, id string) (UploadInfo, error) {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return UploadInfo{}, ErrInvalidUpload
	}
	raw, err := os.ReadFile(uploadInfoPath(dir, id))
	if errors.Is(err, os.ErrNotExist) {
		return UploadInfo{}, ErrMediaNotFound
	}
	if err != nil {
		return UploadInfo{}, err
	}
	var info UploadInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return UploadInfo{}, err
	}
	return info, nil
}

func writeUploadInfo(dir string, info UploadInfo) error {
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(uploadInfoPath(dir, info.ID), raw, 0o600)
}

func uploadInfoPath(dir, id string) string { return filepath.Join(dir, id+".info") }

func cloneMetadata(metadata map[string]string) map[string]string {
	copy := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copy[key] = value
	}
	return copy
}
