package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
)

const MaxUploadSizeBytes int64 = 500 * 1024 * 1024

// NewTusUploadHandler creates the resumable upload handler used by the media API.
func (s *Service) NewTusUploadHandler() (*handler.Handler, error) {
	composer := handler.NewStoreComposer()
	store := filestore.New(s.TusDir())
	store.UseIn(composer)

	return handler.NewHandler(handler.Config{
		BasePath:              "/api/v1/media/uploads",
		MaxSize:               MaxUploadSizeBytes,
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
	})
}

func (s *Service) HandleTusdComplete(ctx context.Context, info handler.FileInfo) error {
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

	payload := ProcessPayload{
		Version: ProcessPayloadVersion, MediaID: info.ID, Filename: info.MetaData["filename"],
		SizeBytes: info.Size, Metadata: info.MetaData,
	}
	if _, err := s.enqueueProcessing(ctx, payload); err != nil {
		_ = os.Rename(incomingPath, sourcePath)
		return err
	}
	_ = os.Remove(filepath.Join(s.TusDir(), info.ID+".info"))
	return nil
}
