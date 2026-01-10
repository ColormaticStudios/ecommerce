package models

type User struct {
	BaseModel
	// Identity fields from OIDC provider (optional for email/password users)
	Subject      string `json:"subject" gorm:"uniqueIndex"` // The unique ID from the IdP (e.g., "sub" claim)
	Username     string `json:"username" gorm:"uniqueIndex;not null"`
	Email        string `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string `json:"-"` // Hashed password (only for email/password auth)

	// Profile information
	Name         string `json:"name"`
	ProfilePhoto string `json:"profile_photo_url"`

	// Preferences & Roles
	Role     string `json:"role" gorm:"default:customer"` // "admin" or "customer"
	Currency string `json:"currency" gorm:"size:3;default:USD"`
}
