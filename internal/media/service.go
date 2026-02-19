package media

import (
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"gorm.io/gorm"
)

const (
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"

	OwnerTypeProduct    = "product"
	OwnerTypeUser       = "user"
	OwnerTypeStorefront = "storefront"

	RoleProductImage   = "product_image"
	RoleProfilePhoto   = "profile_photo"
	RoleStorefrontHero = "storefront_hero"
	DefaultMediaRoot   = "media"
	DefaultPublicPath  = "/media"
)

type Service struct {
	DB        *gorm.DB
	MediaRoot string
	PublicURL string
	Logger    *log.Logger
	Queue     chan Job
}

const (
	DefaultProfilePhotoMaxBytes = 5 * 1024 * 1024
)

type Job struct {
	ID        string
	Source    string
	Filename  string
	SizeBytes int64
	Metadata  map[string]string
}

func NewService(db *gorm.DB, mediaRoot string, publicURL string, logger *log.Logger) *Service {
	if mediaRoot == "" {
		mediaRoot = DefaultMediaRoot
	}
	if publicURL == "" {
		publicURL = DefaultPublicPath
	}
	if logger == nil {
		logger = log.Default()
	}

	return &Service{
		DB:        db,
		MediaRoot: mediaRoot,
		PublicURL: publicURL,
		Logger:    logger,
		Queue:     make(chan Job, 100),
	}
}

func (s *Service) IncomingDir() string {
	return filepath.Join(s.MediaRoot, ".incoming")
}

func (s *Service) TusDir() string {
	return filepath.Join(s.MediaRoot, ".tus")
}

func (s *Service) LocalPath(relPath string) string {
	return filepath.Join(s.MediaRoot, relPath)
}

func (s *Service) PublicURLFor(relPath string) string {
	urlPath := path.Join("/", relPath)
	if s.PublicURL == "" || s.PublicURL == "/" {
		return urlPath
	}

	joined, err := url.JoinPath(s.PublicURL, urlPath)
	if err != nil {
		return s.PublicURL + urlPath
	}
	return joined
}

func (s *Service) EnsureDirs() error {
	if err := os.MkdirAll(s.MediaRoot, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.IncomingDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.TusDir(), 0o755); err != nil {
		return err
	}
	return nil
}
