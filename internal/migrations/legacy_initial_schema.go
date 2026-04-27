package migrations

import (
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Legacy schema structs freeze the initial migration shape so model evolution
// does not mutate replay behavior for historical versions.
type LegacyBaseModel struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type legacyUser struct {
	LegacyBaseModel
	Subject      string `gorm:"uniqueIndex"`
	Username     string `gorm:"uniqueIndex;not null"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string
	Name         string
	ProfilePhoto string
	Role         string `gorm:"default:customer"`
	Currency     string `gorm:"size:3;default:USD"`
}

func (legacyUser) TableName() string {
	return "users"
}

type legacyProduct struct {
	LegacyBaseModel
	SKU            string `gorm:"unique;not null"`
	Name           string `gorm:"not null"`
	Description    string
	Price          models.Money `gorm:"type:numeric(12,2);not null"`
	Stock          int          `gorm:"default:0"`
	Images         []string     `gorm:"type:text[]"`
	IsPublished    bool         `gorm:"not null;default:true;index"`
	DraftData      string       `gorm:"type:text"`
	DraftUpdatedAt *time.Time
}

type legacyProductRelated struct {
	ProductID uint `gorm:"primaryKey"`
	RelatedID uint `gorm:"primaryKey"`
}

func (legacyProductRelated) TableName() string {
	return "product_related"
}

func (legacyProduct) TableName() string {
	return "products"
}

type legacyOrder struct {
	LegacyBaseModel
	UserID                uint
	Total                 models.Money `gorm:"type:numeric(12,2);not null"`
	Status                string
	PaymentMethodDisplay  string
	ShippingAddressPretty string
}

func (legacyOrder) TableName() string {
	return "orders"
}

type legacyOrderItem struct {
	LegacyBaseModel
	OrderID   uint
	ProductID uint
	Quantity  int
	Price     models.Money `gorm:"type:numeric(12,2);not null"`
}

func (legacyOrderItem) TableName() string {
	return "order_items"
}

type legacyCart struct {
	LegacyBaseModel
	UserID uint `gorm:"uniqueIndex"`
}

func (legacyCart) TableName() string {
	return "carts"
}

type legacyCartItem struct {
	LegacyBaseModel
	CartID    uint
	ProductID uint
	Quantity  int `gorm:"default:1"`
}

func (legacyCartItem) TableName() string {
	return "cart_items"
}

type legacyMediaObject struct {
	ID           string `gorm:"primaryKey;size:128"`
	OriginalPath string `gorm:"not null"`
	MimeType     string `gorm:"not null"`
	SizeBytes    int64  `gorm:"not null"`
	Status       string `gorm:"size:16;not null;default:processing"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (legacyMediaObject) TableName() string {
	return "media_objects"
}

type legacyMediaVariant struct {
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

func (legacyMediaVariant) TableName() string {
	return "media_variants"
}

type legacyMediaReference struct {
	ID        uint   `gorm:"primaryKey"`
	MediaID   string `gorm:"index;not null"`
	OwnerType string `gorm:"size:32;index;not null"`
	OwnerID   uint   `gorm:"index;not null"`
	Role      string `gorm:"size:32;index;not null"`
	Position  int    `gorm:"default:0"`
	CreatedAt time.Time
}

func (legacyMediaReference) TableName() string {
	return "media_references"
}

type legacySavedPaymentMethod struct {
	LegacyBaseModel
	UserID         uint   `gorm:"index;not null"`
	Type           string `gorm:"size:20;default:card"`
	Brand          string `gorm:"size:30"`
	Last4          string `gorm:"size:4"`
	ExpMonth       int
	ExpYear        int
	CardholderName string
	Nickname       string
	IsDefault      bool `gorm:"default:false"`
}

func (legacySavedPaymentMethod) TableName() string {
	return "saved_payment_methods"
}

type legacySavedAddress struct {
	LegacyBaseModel
	UserID     uint `gorm:"index;not null"`
	Label      string
	FullName   string
	Line1      string `gorm:"not null"`
	Line2      string
	City       string `gorm:"not null"`
	State      string
	PostalCode string `gorm:"not null"`
	Country    string `gorm:"size:2;not null"`
	Phone      string
	IsDefault  bool `gorm:"default:false"`
}

func (legacySavedAddress) TableName() string {
	return "saved_addresses"
}

type legacyStorefrontSettings struct {
	ID               uint    `gorm:"primaryKey;autoIncrement:false"`
	ConfigJSON       string  `gorm:"type:jsonb;not null"`
	DraftConfigJSON  *string `gorm:"type:jsonb"`
	DraftUpdatedAt   *time.Time
	PublishedUpdated time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (legacyStorefrontSettings) TableName() string {
	return "storefront_settings"
}

type legacyWebsiteSettings struct {
	ID                           uint   `gorm:"primaryKey;autoIncrement:false"`
	AllowGuestCheckout           bool   `gorm:"not null;default:true"`
	OIDCProvider                 string `gorm:"column:oidc_provider;not null;default:''"`
	OIDCClientID                 string `gorm:"column:oidc_client_id;not null;default:''"`
	OIDCClientSecretEnvelopeJSON string `gorm:"column:oidc_client_secret_envelope_json;type:text;not null;default:''"`
	OIDCClientSecretKeyVersion   string `gorm:"column:oidc_client_secret_key_version;not null;default:''"`
	OIDCRedirectURI              string `gorm:"column:oidc_redirect_uri;not null;default:''"`
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func (legacyWebsiteSettings) TableName() string {
	return "website_settings"
}

type legacyCheckoutProviderSetting struct {
	LegacyBaseModel
	ProviderType string `gorm:"not null;index:idx_checkout_provider_settings_type_id,unique"`
	ProviderID   string `gorm:"not null;index:idx_checkout_provider_settings_type_id,unique"`
	Enabled      bool   `gorm:"not null;default:true"`
}

func (legacyCheckoutProviderSetting) TableName() string {
	return "checkout_provider_settings"
}
