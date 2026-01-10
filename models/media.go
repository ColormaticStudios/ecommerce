package models

import "time"

type MediaObject struct {
	ID           string    `json:"id" gorm:"primaryKey;size:128"`
	OriginalPath string    `json:"original_path" gorm:"not null"`
	MimeType     string    `json:"mime_type" gorm:"not null"`
	SizeBytes    int64     `json:"size_bytes" gorm:"not null"`
	Status       string    `json:"status" gorm:"size:16;not null;default:processing"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MediaVariant struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	MediaID   string    `json:"media_id" gorm:"index;not null"`
	Label     string    `json:"label" gorm:"size:64;not null"`
	Path      string    `json:"path" gorm:"not null"`
	MimeType  string    `json:"mime_type" gorm:"not null"`
	SizeBytes int64     `json:"size_bytes" gorm:"not null"`
	Width     int       `json:"width" gorm:"not null"`
	Height    int       `json:"height" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

type MediaReference struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	MediaID   string    `json:"media_id" gorm:"index;not null"`
	OwnerType string    `json:"owner_type" gorm:"size:32;index;not null"`
	OwnerID   uint      `json:"owner_id" gorm:"index;not null"`
	Role      string    `json:"role" gorm:"size:32;index;not null"`
	CreatedAt time.Time `json:"created_at"`
}
