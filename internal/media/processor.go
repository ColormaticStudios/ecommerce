package media

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/h2non/bimg"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	defaultImageMaxWidth  = 2048
	JobTypeProcess        = "media.process"
	ProcessPayloadVersion = 1
)

var errInvalidMedia = errors.New("invalid or unsupported media")

type ProcessPayload struct {
	Version   int               `json:"version"`
	MediaID   string            `json:"media_id"`
	Filename  string            `json:"filename,omitempty"`
	SizeBytes int64             `json:"size_bytes"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (s *Service) RegisterJobHandlers() error {
	if s.Jobs == nil {
		return errors.New("media processing requires the shared job runtime")
	}
	return s.Jobs.Register(JobTypeProcess, jobs.Registration{
		Handle: s.handleProcessJob,
		OnDeadLetter: func(ctx context.Context, tx *gorm.DB, payload json.RawMessage, _ error) error {
			var input ProcessPayload
			if err := json.Unmarshal(payload, &input); err != nil || strings.TrimSpace(input.MediaID) == "" {
				return nil
			}
			return tx.WithContext(ctx).Model(&models.MediaObject{}).Where("id = ?", input.MediaID).Update("status", StatusFailed).Error
		},
	})
}

func (s *Service) handleProcessJob(ctx context.Context, raw json.RawMessage) error {
	var payload ProcessPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return reliability.Classify(fmt.Errorf("decode media process payload: %w", err), reliability.ClassTerminal)
	}
	if payload.Version != ProcessPayloadVersion || strings.TrimSpace(payload.MediaID) == "" {
		return reliability.Classify(errors.New("unsupported or incomplete media process payload"), reliability.ClassTerminal)
	}
	if err := s.processJob(ctx, payload); err != nil {
		return classifyMediaProcessError(err)
	}
	return nil
}

func classifyMediaProcessError(err error) error {
	if _, classified := reliability.ErrorClassOf(err); classified {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, os.ErrNotExist) || errors.Is(err, errInvalidMedia) {
		return reliability.Classify(err, reliability.ClassTerminal)
	}
	if isRetryableDatabaseError(err) {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	for _, transient := range []error{syscall.EINTR, syscall.EIO, syscall.ENOSPC, syscall.EMFILE, syscall.ENFILE, syscall.ETIMEDOUT, syscall.ECONNRESET} {
		if errors.Is(err, transient) {
			return reliability.Classify(err, reliability.ClassRetryable)
		}
	}
	type temporaryError interface{ Temporary() bool }
	var temporary temporaryError
	if errors.As(err, &temporary) && temporary.Temporary() {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	// Unknown processing failures are terminal by default. A retry must be
	// backed by a known transient database, I/O, or infrastructure condition.
	return reliability.Classify(err, reliability.ClassTerminal)
}

func isRetryableDatabaseError(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return false
	}
	if len(postgresError.Code) >= 2 {
		switch postgresError.Code[:2] {
		case "08", "40", "53": // connection, transaction rollback, insufficient resources
			return true
		}
	}
	switch postgresError.Code {
	case "55P03", "57P01", "57P02", "57P03": // lock unavailable or server shutdown/startup
		return true
	default:
		return false
	}
}

func (s *Service) processJob(ctx context.Context, job ProcessPayload) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var mediaObject models.MediaObject
	if err := s.DB.WithContext(ctx).First(&mediaObject, "id = ?", job.MediaID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("media database row %s: %w", job.MediaID, err)
		}
		return fmt.Errorf("load media database row: %w", err)
	}
	if mediaObject.Status == StatusReady && mediaObject.OriginalPath != "" {
		return nil
	}

	inputPath := filepath.Join(s.IncomingDir(), job.MediaID)
	if _, err := os.Stat(inputPath); errors.Is(err, os.ErrNotExist) {
		if recovered, recoverErr := s.recoverWrittenOutput(ctx, job.MediaID); recovered || recoverErr != nil {
			return recoverErr
		}
		return fmt.Errorf("media input %s: %w", job.MediaID, os.ErrNotExist)
	} else if err != nil {
		return err
	}
	mimeType, err := detectMime(inputPath)
	if err != nil {
		return err
	}

	outputRelPath := ""
	outputMime := mimeType
	outputPath := ""
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		outputRelPath = filepath.ToSlash(filepath.Join(job.MediaID, "original.webp"))
		outputPath = s.LocalPath(outputRelPath)
		if err := convertImageToWebp(inputPath, outputPath); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "webp") {
				ext := filepath.Ext(job.Filename)
				if ext == "" {
					ext = extensionForMime(mimeType)
				}
				if ext == "" {
					ext = ".img"
				}
				outputRelPath = filepath.ToSlash(filepath.Join(job.MediaID, "original"+ext))
				outputPath = s.LocalPath(outputRelPath)
				if err := moveFile(inputPath, outputPath); err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			outputMime = "image/webp"
		}
	case strings.HasPrefix(mimeType, "video/"):
		outputRelPath = filepath.ToSlash(filepath.Join(job.MediaID, "original.webm"))
		outputPath = s.LocalPath(outputRelPath)
		if err := convertVideoToWebm(ctx, inputPath, outputPath); err != nil {
			return err
		}
		outputMime = "video/webm"
	default:
		ext := filepath.Ext(job.Filename)
		if ext == "" {
			ext = ".bin"
		}
		outputRelPath = filepath.ToSlash(filepath.Join(job.MediaID, "original"+ext))
		outputPath = s.LocalPath(outputRelPath)
		if err := moveFile(inputPath, outputPath); err != nil {
			return err
		}
	}
	if err := s.finalizeOutput(ctx, job.MediaID, outputRelPath, outputMime); err != nil {
		return fmt.Errorf("finalize media database row: %w", err)
	}
	_ = os.Remove(inputPath)
	return nil
}

func (s *Service) recoverWrittenOutput(ctx context.Context, mediaID string) (bool, error) {
	matches, err := filepath.Glob(filepath.Join(s.LocalPath(mediaID), "original.*"))
	if err != nil || len(matches) == 0 {
		return false, err
	}
	outputPath := matches[0]
	mimeType, err := detectMime(outputPath)
	if err != nil {
		return true, err
	}
	relativePath, err := filepath.Rel(s.MediaRoot, outputPath)
	if err != nil {
		return true, err
	}
	return true, s.finalizeOutput(ctx, mediaID, filepath.ToSlash(relativePath), mimeType)
}

func (s *Service) finalizeOutput(ctx context.Context, mediaID, outputRelPath, outputMime string) error {
	outputPath := s.LocalPath(outputRelPath)
	stat, err := os.Stat(outputPath)
	if err != nil {
		return err
	}
	if strings.HasPrefix(outputMime, "image/") {
		thumbRelPath := filepath.ToSlash(filepath.Join(mediaID, "variants", "thumb_512.webp"))
		thumbPath := s.LocalPath(thumbRelPath)
		if err := convertImageToWebpThumbnail(outputPath, thumbPath, 512); err != nil {
			s.Logger.Printf("[WARN] Failed to create thumbnail for %s: %v", mediaID, err)
		} else {
			variant := models.MediaVariant{MediaID: mediaID, Label: "thumb_512"}
			values := models.MediaVariant{Path: thumbRelPath, MimeType: "image/webp", SizeBytes: fileSize(thumbPath), Width: 512, Height: 512}
			if err := s.DB.WithContext(ctx).Where("media_id = ? AND label = ?", mediaID, "thumb_512").Assign(values).FirstOrCreate(&variant).Error; err != nil {
				s.Logger.Printf("[WARN] Failed to persist thumbnail for %s: %v", mediaID, err)
			}
		}
	}
	return s.DB.WithContext(ctx).Model(&models.MediaObject{}).Where("id = ?", mediaID).Updates(map[string]any{
		"original_path": outputRelPath, "mime_type": outputMime, "size_bytes": stat.Size(), "status": StatusReady,
	}).Error
}

func detectMime(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var buf [512]byte
	n, err := file.Read(buf[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

func extensionForMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	default:
		return ""
	}
}

func moveFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

func convertImageToWebp(inputPath, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	buffer, err := bimg.Read(inputPath)
	if err != nil {
		return err
	}
	image := bimg.NewImage(buffer)
	size, err := image.Size()
	if err != nil {
		return fmt.Errorf("%w: read image dimensions: %v", errInvalidMedia, err)
	}
	options := bimg.Options{Type: bimg.WEBP, Quality: 82}
	if size.Width > defaultImageMaxWidth {
		options.Width = defaultImageMaxWidth
	}
	processed, err := image.Process(options)
	if err != nil {
		return fmt.Errorf("%w: decode or convert image: %v", errInvalidMedia, err)
	}
	return bimg.Write(outputPath, processed)
}

func convertImageToWebpThumbnail(inputPath, outputPath string, size int) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	buffer, err := bimg.Read(inputPath)
	if err != nil {
		return err
	}
	processed, err := bimg.NewImage(buffer).Process(bimg.Options{Type: bimg.WEBP, Quality: 82, Width: size, Height: size, Crop: true})
	if err != nil {
		return err
	}
	return bimg.Write(outputPath, processed)
}

func convertVideoToWebm(ctx context.Context, inputPath, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return runFFmpeg(ctx, []string{"-y", "-i", inputPath, "-c:v", "libvpx-vp9", "-b:v", "0", "-crf", "32", "-c:a", "libopus", outputPath})
}

func runFFmpeg(ctx context.Context, args []string) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("%w: ffmpeg is unavailable: %v", errInvalidMedia, err)
		}
		lowerStderr := strings.ToLower(stderr.String())
		for _, marker := range []string{"no space left on device", "input/output error", "resource temporarily unavailable"} {
			if strings.Contains(lowerStderr, marker) {
				return reliability.Classify(fmt.Errorf("ffmpeg infrastructure failure: %w (%s)", err, stderr.String()), reliability.ClassRetryable)
			}
		}
		return fmt.Errorf("%w: ffmpeg validation or conversion failed: %v (%s)", errInvalidMedia, err, stderr.String())
	}
	return nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
