package models

import "time"

type MediaObject struct {
	ID           string `gorm:"primaryKey;size:128"`
	OriginalPath string `json:"original_path" gorm:"not null"`
	MimeType     string `json:"mime_type" gorm:"not null"`
	SizeBytes    int64  `json:"size_bytes" gorm:"not null"`
	Status       string `json:"status" gorm:"size:16;not null;default:processing"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MediaVariant struct {
	ID        uint   `gorm:"primaryKey"`
	MediaID   string `gorm:"index;not null"`
	Label     string `gorm:"size:64;not null"`
	Path      string `gorm:"not null"`
	MimeType  string `gorm:"not null"`
	SizeBytes int64  `gorm:"not null"`
	Width     int    `gorm:"not null"`
	Height    int    `gorm:"not null"`
	CreatedAt time.Time
}

type MediaReference struct {
	ID        uint   `gorm:"primaryKey"`
	MediaID   string `gorm:"index;not null"`
	OwnerType string `gorm:"size:32;index;not null"`
	OwnerID   uint   `gorm:"index;not null"`
	Role      string `gorm:"size:32;index;not null"`
	CreatedAt time.Time
}
