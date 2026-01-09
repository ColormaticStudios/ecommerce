package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	// Identity fields from OIDC provider (optional for email/password users)
	Subject      string `gorm:"uniqueIndex"` // The unique ID from the IdP (e.g., "sub" claim)
	Username     string `gorm:"uniqueIndex;not null"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `json:"-"` // Hashed password (only for email/password auth)

	// Profile information
	Name         string `json:"name"`
	ProfilePhoto string `json:"profile_photo_url"`

	// Preferences & Roles
	Role     string `json:"role" gorm:"default:customer"` // "admin" or "customer"
	Currency string `json:"currency" gorm:"size:3;default:USD"`
}
