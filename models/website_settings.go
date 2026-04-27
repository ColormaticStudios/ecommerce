package models

import "time"

const WebsiteSettingsSingletonID uint = 1

type WebsiteSettings struct {
	ID                           uint      `json:"id" gorm:"primaryKey;autoIncrement:false"`
	AllowGuestCheckout           bool      `json:"allow_guest_checkout" gorm:"not null;default:true"`
	OIDCProvider                 string    `json:"oidc_provider" gorm:"column:oidc_provider;not null;default:''"`
	OIDCClientID                 string    `json:"oidc_client_id" gorm:"column:oidc_client_id;not null;default:''"`
	OIDCClientSecretEnvelopeJSON string    `json:"oidc_client_secret_envelope_json" gorm:"column:oidc_client_secret_envelope_json;type:text;not null;default:''"`
	OIDCClientSecretKeyVersion   string    `json:"oidc_client_secret_key_version" gorm:"column:oidc_client_secret_key_version;not null;default:''"`
	OIDCRedirectURI              string    `json:"oidc_redirect_uri" gorm:"column:oidc_redirect_uri;not null;default:''"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
}
