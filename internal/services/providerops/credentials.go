package providerops

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

var (
	ErrCredentialServiceUnconfigured      = errors.New("provider credential service is not configured")
	ErrProviderCredentialNotFound         = errors.New("provider credential not found")
	ErrProviderCredentialWrongEnvironment = errors.New("provider credential is not configured for this environment")
	ErrUnsupportedProviderCurrency        = errors.New("provider does not support the requested currency")
	ErrInvalidProviderType                = errors.New("invalid provider type")
	ErrInvalidProviderEnvironment         = errors.New("invalid provider environment")
)

type CredentialMetadata struct {
	SupportedCurrencies []string `json:"supported_currencies,omitempty"`
	SettlementCurrency  string   `json:"settlement_currency,omitempty"`
	FXMode              string   `json:"fx_mode,omitempty"`
}

type StoredCredential struct {
	Record   models.ProviderCredential
	Metadata CredentialMetadata
}

type ResolvedCredential struct {
	StoredCredential
	SecretData map[string]string
}

type StoreCredentialInput struct {
	ProviderType string
	ProviderID   string
	Environment  string
	Label        string
	SecretData   map[string]string
	Metadata     CredentialMetadata
}

type CredentialService struct {
	keyring          map[string][]byte
	activeKeyVersion string
}

type credentialEnvelope struct {
	KeyVersion string `json:"key_version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func NewCredentialService(keyring map[string][]byte, activeKeyVersion string) (*CredentialService, error) {
	normalized := make(map[string][]byte, len(keyring))
	for version, secret := range keyring {
		version = strings.TrimSpace(version)
		if version == "" {
			return nil, fmt.Errorf("provider credential key version is required")
		}
		if len(secret) != 16 && len(secret) != 24 && len(secret) != 32 {
			return nil, fmt.Errorf("provider credential key %q must be 16, 24, or 32 bytes", version)
		}
		copied := make([]byte, len(secret))
		copy(copied, secret)
		normalized[version] = copied
	}

	activeKeyVersion = strings.TrimSpace(activeKeyVersion)
	if len(normalized) == 0 && activeKeyVersion == "" {
		return &CredentialService{}, nil
	}
	if activeKeyVersion == "" {
		return nil, fmt.Errorf("provider credential active key version is required")
	}
	if _, ok := normalized[activeKeyVersion]; !ok {
		return nil, fmt.Errorf("provider credential active key version %q is missing from keyring", activeKeyVersion)
	}
	return &CredentialService{
		keyring:          normalized,
		activeKeyVersion: activeKeyVersion,
	}, nil
}

func ParseKeyringConfig(raw string) (map[string][]byte, error) {
	keyring := map[string][]byte{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return keyring, nil
	}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, ":", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid provider credential key definition %q", part)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pieces[1]))
		if err != nil {
			return nil, fmt.Errorf("decode provider credential key %q: %w", strings.TrimSpace(pieces[0]), err)
		}
		keyring[strings.TrimSpace(pieces[0])] = decoded
	}
	return keyring, nil
}

func (s *CredentialService) Enabled() bool {
	return s != nil && s.activeKeyVersion != "" && len(s.keyring) > 0
}

func (s *CredentialService) Store(ctx context.Context, db *gorm.DB, input StoreCredentialInput) (StoredCredential, error) {
	_ = ctx
	if !s.Enabled() {
		return StoredCredential{}, ErrCredentialServiceUnconfigured
	}

	providerType, err := normalizeProviderType(input.ProviderType)
	if err != nil {
		return StoredCredential{}, err
	}
	environment, err := normalizeProviderEnvironment(input.Environment)
	if err != nil {
		return StoredCredential{}, err
	}
	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		return StoredCredential{}, fmt.Errorf("provider id is required")
	}
	if len(input.SecretData) == 0 {
		return StoredCredential{}, fmt.Errorf("secret_data is required")
	}

	metadata := normalizeCredentialMetadata(input.Metadata)
	envelope, err := s.encrypt(providerType, providerID, environment, input.SecretData)
	if err != nil {
		return StoredCredential{}, err
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return StoredCredential{}, err
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return StoredCredential{}, err
	}

	record := models.ProviderCredential{}
	now := time.Now().UTC()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		findErr := tx.Where("provider_type = ? AND provider_id = ? AND environment = ?", providerType, providerID, environment).First(&record).Error
		switch {
		case findErr == nil:
			record.Label = strings.TrimSpace(input.Label)
			record.SecretEnvelopeJSON = string(envelopeJSON)
			record.KeyVersion = s.activeKeyVersion
			record.MetadataJSON = string(metadataJSON)
			record.LastRotatedAt = now
			return tx.Save(&record).Error
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			record = models.ProviderCredential{
				ProviderType:       providerType,
				ProviderID:         providerID,
				Environment:        environment,
				Label:              strings.TrimSpace(input.Label),
				SecretEnvelopeJSON: string(envelopeJSON),
				KeyVersion:         s.activeKeyVersion,
				MetadataJSON:       string(metadataJSON),
				LastRotatedAt:      now,
			}
			return tx.Create(&record).Error
		default:
			return findErr
		}
	})
	if err != nil {
		return StoredCredential{}, err
	}

	return StoredCredential{Record: record, Metadata: metadata}, nil
}

func (s *CredentialService) List(ctx context.Context, db *gorm.DB, providerType string) ([]StoredCredential, error) {
	query := db.WithContext(ctx).Model(&models.ProviderCredential{})
	if strings.TrimSpace(providerType) != "" {
		normalized, err := normalizeProviderType(providerType)
		if err != nil {
			return nil, err
		}
		query = query.Where("provider_type = ?", normalized)
	}

	var records []models.ProviderCredential
	if err := query.Order("provider_type ASC, provider_id ASC, environment ASC").Find(&records).Error; err != nil {
		return nil, err
	}

	result := make([]StoredCredential, 0, len(records))
	for _, record := range records {
		metadata, err := decodeCredentialMetadata(record.MetadataJSON)
		if err != nil {
			return nil, err
		}
		result = append(result, StoredCredential{Record: record, Metadata: metadata})
	}
	return result, nil
}

func (s *CredentialService) Rotate(ctx context.Context, db *gorm.DB, credentialID uint) (StoredCredential, error) {
	if !s.Enabled() {
		return StoredCredential{}, ErrCredentialServiceUnconfigured
	}

	var record models.ProviderCredential
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&record, credentialID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderCredentialNotFound
			}
			return err
		}

		if record.KeyVersion == s.activeKeyVersion {
			return nil
		}

		secretData, err := s.decryptRecord(record)
		if err != nil {
			return err
		}
		envelope, err := s.encrypt(record.ProviderType, record.ProviderID, record.Environment, secretData)
		if err != nil {
			return err
		}
		envelopeJSON, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		record.SecretEnvelopeJSON = string(envelopeJSON)
		record.KeyVersion = s.activeKeyVersion
		record.LastRotatedAt = time.Now().UTC()
		return tx.Save(&record).Error
	})
	if err != nil {
		return StoredCredential{}, err
	}

	metadata, err := decodeCredentialMetadata(record.MetadataJSON)
	if err != nil {
		return StoredCredential{}, err
	}
	return StoredCredential{Record: record, Metadata: metadata}, nil
}

func (s *CredentialService) Resolve(ctx context.Context, db *gorm.DB, providerType, providerID, environment string) (*ResolvedCredential, error) {
	_ = ctx
	if db == nil || !db.Migrator().HasTable(&models.ProviderCredential{}) {
		return nil, nil
	}

	providerType, err := normalizeProviderType(providerType)
	if err != nil {
		return nil, err
	}
	environment, err = normalizeProviderEnvironment(environment)
	if err != nil {
		return nil, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, fmt.Errorf("provider id is required")
	}

	var record models.ProviderCredential
	err = db.WithContext(ctx).
		Where("provider_type = ? AND provider_id = ? AND environment = ?", providerType, providerID, environment).
		First(&record).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		hasAny, lookupErr := s.hasAnyCredential(ctx, db, providerType, providerID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if hasAny {
			return nil, ErrProviderCredentialWrongEnvironment
		}
		return nil, nil
	default:
		return nil, err
	}

	metadata, err := decodeCredentialMetadata(record.MetadataJSON)
	if err != nil {
		return nil, err
	}
	secretData, err := s.decryptRecord(record)
	if err != nil {
		return nil, err
	}
	return &ResolvedCredential{
		StoredCredential: StoredCredential{
			Record:   record,
			Metadata: metadata,
		},
		SecretData: secretData,
	}, nil
}

func (s *CredentialService) ValidateCurrency(currency string, credential *ResolvedCredential) error {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" || credential == nil {
		return nil
	}
	supported := credential.Metadata.SupportedCurrencies
	if len(supported) == 0 {
		return nil
	}
	if slices.Contains(supported, currency) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedProviderCurrency, currency)
}

func (s *CredentialService) EncryptSecretData(scope string, secretData map[string]string) (string, string, error) {
	if !s.Enabled() {
		return "", "", ErrCredentialServiceUnconfigured
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "", "", fmt.Errorf("credential encryption scope is required")
	}
	if len(secretData) == 0 {
		return "", "", fmt.Errorf("secret_data is required")
	}
	envelope, err := s.encrypt("app", scope, "global", secretData)
	if err != nil {
		return "", "", err
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return "", "", err
	}
	return string(envelopeJSON), s.activeKeyVersion, nil
}

func (s *CredentialService) DecryptSecretData(scope string, envelopeJSON string) (map[string]string, error) {
	if !s.Enabled() {
		return nil, ErrCredentialServiceUnconfigured
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil, fmt.Errorf("credential encryption scope is required")
	}
	return s.decryptEnvelope("app", scope, "global", envelopeJSON)
}

func (s *CredentialService) hasAnyCredential(ctx context.Context, db *gorm.DB, providerType, providerID string) (bool, error) {
	if db == nil || !db.Migrator().HasTable(&models.ProviderCredential{}) {
		return false, nil
	}
	var count int64
	if err := db.WithContext(ctx).Model(&models.ProviderCredential{}).
		Where("provider_type = ? AND provider_id = ?", providerType, providerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *CredentialService) encrypt(providerType, providerID, environment string, secretData map[string]string) (credentialEnvelope, error) {
	block, err := aes.NewCipher(s.keyring[s.activeKeyVersion])
	if err != nil {
		return credentialEnvelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return credentialEnvelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return credentialEnvelope{}, err
	}

	payload, err := json.Marshal(secretData)
	if err != nil {
		return credentialEnvelope{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, payload, credentialAAD(providerType, providerID, environment))
	return credentialEnvelope{
		KeyVersion: s.activeKeyVersion,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (s *CredentialService) decryptRecord(record models.ProviderCredential) (map[string]string, error) {
	if !s.Enabled() {
		return nil, ErrCredentialServiceUnconfigured
	}
	return s.decryptEnvelope(record.ProviderType, record.ProviderID, record.Environment, record.SecretEnvelopeJSON)
}

func (s *CredentialService) decryptEnvelope(providerType, providerID, environment string, envelopeJSON string) (map[string]string, error) {
	var envelope credentialEnvelope
	if err := json.Unmarshal([]byte(envelopeJSON), &envelope); err != nil {
		return nil, err
	}
	key := s.keyring[strings.TrimSpace(envelope.KeyVersion)]
	if len(key) == 0 {
		return nil, fmt.Errorf("provider credential key version %q is unavailable", envelope.KeyVersion)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, credentialAAD(providerType, providerID, environment))
	if err != nil {
		return nil, err
	}

	var secretData map[string]string
	if err := json.Unmarshal(plaintext, &secretData); err != nil {
		return nil, err
	}
	if secretData == nil {
		secretData = map[string]string{}
	}
	return secretData, nil
}

func credentialAAD(providerType, providerID, environment string) []byte {
	return []byte(strings.Join([]string{providerType, providerID, environment}, "|"))
}

func normalizeCredentialMetadata(metadata CredentialMetadata) CredentialMetadata {
	result := CredentialMetadata{
		SupportedCurrencies: make([]string, 0, len(metadata.SupportedCurrencies)),
		SettlementCurrency:  strings.ToUpper(strings.TrimSpace(metadata.SettlementCurrency)),
		FXMode:              strings.TrimSpace(metadata.FXMode),
	}
	if result.FXMode == "" {
		result.FXMode = models.ProviderFXModeSameCurrencyOnly
	}
	for _, currency := range metadata.SupportedCurrencies {
		currency = strings.ToUpper(strings.TrimSpace(currency))
		if currency == "" || slices.Contains(result.SupportedCurrencies, currency) {
			continue
		}
		result.SupportedCurrencies = append(result.SupportedCurrencies, currency)
	}
	return result
}

func decodeCredentialMetadata(raw string) (CredentialMetadata, error) {
	if strings.TrimSpace(raw) == "" {
		return normalizeCredentialMetadata(CredentialMetadata{}), nil
	}
	var metadata CredentialMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return CredentialMetadata{}, err
	}
	return normalizeCredentialMetadata(metadata), nil
}

func normalizeProviderType(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case models.ProviderTypePayment:
		return models.ProviderTypePayment, nil
	case models.ProviderTypeShipping:
		return models.ProviderTypeShipping, nil
	case models.ProviderTypeTax:
		return models.ProviderTypeTax, nil
	default:
		return "", ErrInvalidProviderType
	}
}

func normalizeProviderEnvironment(value string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case models.ProviderEnvironmentSandbox:
		return models.ProviderEnvironmentSandbox, nil
	case models.ProviderEnvironmentProduction:
		return models.ProviderEnvironmentProduction, nil
	default:
		return "", ErrInvalidProviderEnvironment
	}
}
