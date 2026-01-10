package media

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	defaultImageMaxWidth = 2048
)

func (s *Service) StartProcessor() {
	go func() {
		for job := range s.Queue {
			if err := s.processJob(job); err != nil {
				s.Logger.Printf("[ERROR] Media processing failed for %s: %v", job.ID, err)
				s.DB.Model(&models.MediaObject{}).Where("id = ?", job.ID).Updates(map[string]any{
					"status": StatusFailed,
				})
			}
		}
	}()
}

func (s *Service) processJob(job Job) error {
	if job.ID == "" || job.Source == "" {
		return errors.New("missing job metadata")
	}

	inputPath := job.Source
	mimeType, err := detectMime(inputPath)
	if err != nil {
		return err
	}

	outputRelPath := ""
	outputMime := mimeType
	outputPath := ""

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		outputRelPath = filepath.ToSlash(filepath.Join(job.ID, "original.webp"))
		outputPath = s.LocalPath(outputRelPath)
		if err := convertImageToWebp(inputPath, outputPath); err != nil {
			return err
		}
		outputMime = "image/webp"
	case strings.HasPrefix(mimeType, "video/"):
		outputRelPath = filepath.ToSlash(filepath.Join(job.ID, "original.webm"))
		outputPath = s.LocalPath(outputRelPath)
		if err := convertVideoToWebm(inputPath, outputPath); err != nil {
			return err
		}
		outputMime = "video/webm"
	default:
		ext := filepath.Ext(job.Filename)
		if ext == "" {
			ext = ".bin"
		}
		outputRelPath = filepath.ToSlash(filepath.Join(job.ID, "original"+ext))
		outputPath = s.LocalPath(outputRelPath)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}
		if err := os.Rename(inputPath, outputPath); err != nil {
			return err
		}
	}

	stat, err := os.Stat(outputPath)
	if err != nil {
		return err
	}

	if strings.HasPrefix(outputMime, "image/") {
		thumbRelPath := filepath.ToSlash(filepath.Join(job.ID, "variants", "thumb_512.webp"))
		thumbPath := s.LocalPath(thumbRelPath)
		if err := convertImageToWebpThumbnail(outputPath, thumbPath, 512); err != nil {
			s.Logger.Printf("[WARN] Failed to create thumbnail for %s: %v", job.ID, err)
		} else {
			if err := s.DB.Create(&models.MediaVariant{
				MediaID:   job.ID,
				Label:     "thumb_512",
				Path:      thumbRelPath,
				MimeType:  "image/webp",
				SizeBytes: fileSize(thumbPath),
				Width:     512,
				Height:    512,
			}).Error; err != nil {
				s.Logger.Printf("[WARN] Failed to persist thumbnail for %s: %v", job.ID, err)
			}
		}
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&models.MediaObject{}).Where("id = ?", job.ID).Updates(map[string]any{
			"original_path": outputRelPath,
			"mime_type":     outputMime,
			"size_bytes":    stat.Size(),
			"status":        StatusReady,
		}).Error
	}); err != nil {
		return err
	}

	_ = os.Remove(inputPath)
	return nil
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

func convertImageToWebp(inputPath string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2", defaultImageMaxWidth),
		"-c:v", "libwebp",
		"-quality", "82",
		outputPath,
	}
	return runFFmpeg(args)
}

func convertImageToWebpThumbnail(inputPath string, outputPath string, size int) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", size, size, size, size)
	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", filter,
		"-c:v", "libwebp",
		"-quality", "82",
		outputPath,
	}
	return runFFmpeg(args)
}

func convertVideoToWebm(inputPath string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	args := []string{
		"-y",
		"-i", inputPath,
		"-c:v", "libvpx-vp9",
		"-b:v", "0",
		"-crf", "32",
		"-c:a", "libopus",
		outputPath,
	}
	return runFFmpeg(args)
}

func runFFmpeg(args []string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w (%s)", err, stderr.String())
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
