package models

type CheckoutProviderSetting struct {
	BaseModel
	ProviderType string `json:"provider_type" gorm:"not null;index:idx_checkout_provider_settings_type_id,unique"`
	ProviderID   string `json:"provider_id" gorm:"not null;index:idx_checkout_provider_settings_type_id,unique"`
	Enabled      bool   `json:"enabled" gorm:"not null;default:true"`
}
