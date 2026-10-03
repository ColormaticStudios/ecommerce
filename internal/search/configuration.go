package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SynonymDirectionUnidirectional = "uni"
	SynonymDirectionBidirectional  = "bi"

	DefaultMinimumTokenLength   = 4
	DefaultOneEditMinimumLength = 4
	DefaultTwoEditMinimumLength = 8
)

var (
	ErrConfigurationInvalid      = errors.New("invalid search configuration")
	ErrConfigurationNotFound     = errors.New("search configuration not found")
	ErrConfigurationConflict     = errors.New("search configuration conflict")
	ErrActiveTypoProfileRequired = errors.New("an active typo tolerance profile is required")
)

// SynonymSet is the decoded application-facing representation of a persisted
// synonym set. For unidirectional sets, Terms[0] is the source and the
// remaining terms are replacements. Bidirectional terms are all equivalent.
type SynonymSet struct {
	ID        uint
	Name      string
	Direction string
	Terms     []string
	IsActive  bool
	UpdatedBy *uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SynonymSetInput is a complete synonym-set replacement. HTTP PATCH handlers
// can merge omitted fields with GetSynonymSet before calling UpdateSynonymSet.
type SynonymSetInput struct {
	Name      string
	Direction string
	Terms     []string
	IsActive  bool
	UpdatedBy *uint
}

// TypoToleranceProfileInput is a complete named typo-policy replacement.
// IsActive requests atomic activation. The current active profile cannot be
// directly deactivated; activate another profile instead.
type TypoToleranceProfileInput struct {
	Name                 string
	MinimumTokenLength   int
	OneEditMinimumLength int
	TwoEditMinimumLength int
	StrictMode           bool
	IsActive             bool
	UpdatedBy            *uint
}

func (s *Service) ListSynonymSets(ctx context.Context) ([]SynonymSet, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []models.SearchSynonymSet
	if err := db.Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	sets := make([]SynonymSet, 0, len(rows))
	for _, row := range rows {
		set, err := synonymSetFromModel(row)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, nil
}

func (s *Service) GetSynonymSet(ctx context.Context, id uint) (SynonymSet, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return SynonymSet{}, err
	}
	var row models.SearchSynonymSet
	if err := db.First(&row, id).Error; err != nil {
		return SynonymSet{}, configurationLookupError(err)
	}
	return synonymSetFromModel(row)
}

func (s *Service) CreateSynonymSet(ctx context.Context, input SynonymSetInput) (SynonymSet, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return SynonymSet{}, err
	}
	name, termsJSON, err := normalizeSynonymSet(input)
	if err != nil {
		return SynonymSet{}, err
	}
	wantActive := input.IsActive
	row := models.SearchSynonymSet{
		Name: name, Direction: input.Direction, TermsJSON: termsJSON,
		IsActive: wantActive, UpdatedBy: input.UpdatedBy,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(&row).Error; err != nil {
			return err
		}
		// SearchSynonymSet has default:true. Correct an explicitly false value
		// in the same transaction because GORM applies that default on Create.
		if !wantActive {
			if err := tx.Model(&row).Update("is_active", false).Error; err != nil {
				return err
			}
			row.IsActive = false
		}
		return nil
	}); err != nil {
		return SynonymSet{}, configurationWriteError(err)
	}
	return synonymSetFromModel(row)
}

func (s *Service) UpdateSynonymSet(ctx context.Context, id uint, input SynonymSetInput) (SynonymSet, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return SynonymSet{}, err
	}
	name, termsJSON, err := normalizeSynonymSet(input)
	if err != nil {
		return SynonymSet{}, err
	}
	var row models.SearchSynonymSet
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&row, id).Error; err != nil {
			return configurationLookupError(err)
		}
		updates := map[string]any{
			"name": name, "direction": input.Direction, "terms_json": termsJSON,
			"is_active": input.IsActive, "updated_by": input.UpdatedBy, "updated_at": s.now().UTC(),
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&row, id).Error
	})
	if err != nil {
		return SynonymSet{}, configurationWriteError(err)
	}
	return synonymSetFromModel(row)
}

func (s *Service) DeleteSynonymSet(ctx context.Context, id uint) error {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return err
	}
	result := db.Unscoped().Delete(&models.SearchSynonymSet{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrConfigurationNotFound
	}
	return nil
}

func (s *Service) ListTypoToleranceProfiles(ctx context.Context) ([]models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, err
	}
	var profiles []models.SearchTypoToleranceProfile
	err = db.Order("name ASC, id ASC").Find(&profiles).Error
	return profiles, err
}

func (s *Service) GetTypoToleranceProfile(ctx context.Context, id uint) (models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	var profile models.SearchTypoToleranceProfile
	if err := db.First(&profile, id).Error; err != nil {
		return models.SearchTypoToleranceProfile{}, configurationLookupError(err)
	}
	return profile, nil
}

func (s *Service) ActiveTypoToleranceProfile(ctx context.Context) (models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	var profile models.SearchTypoToleranceProfile
	if err := db.Where("is_active = ?", true).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SearchTypoToleranceProfile{}, ErrActiveTypoProfileRequired
		}
		return models.SearchTypoToleranceProfile{}, err
	}
	return profile, nil
}

func (s *Service) CreateTypoToleranceProfile(ctx context.Context, input TypoToleranceProfileInput) (models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	input, err = normalizeTypoToleranceProfile(input)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	profile := models.SearchTypoToleranceProfile{
		Name: input.Name, MinimumTokenLength: input.MinimumTokenLength,
		OneEditMinimumLength: input.OneEditMinimumLength, TwoEditMinimumLength: input.TwoEditMinimumLength,
		StrictMode: input.StrictMode, IsActive: false, UpdatedBy: input.UpdatedBy,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&models.SearchTypoToleranceProfile{}).Where("is_active = ?", true).Count(&activeCount).Error; err != nil {
			return err
		}
		if err := tx.Select("*").Create(&profile).Error; err != nil {
			return err
		}
		if input.IsActive || activeCount == 0 {
			return s.activateTypoToleranceProfileTx(tx, profile.ID, input.UpdatedBy)
		}
		return nil
	})
	if err != nil {
		return models.SearchTypoToleranceProfile{}, configurationWriteError(err)
	}
	return s.GetTypoToleranceProfile(ctx, profile.ID)
}

func (s *Service) UpdateTypoToleranceProfile(ctx context.Context, id uint, input TypoToleranceProfileInput) (models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	input, err = normalizeTypoToleranceProfile(input)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var current models.SearchTypoToleranceProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			return configurationLookupError(err)
		}
		if current.IsActive && !input.IsActive {
			return fmt.Errorf("%w: activate another profile before deactivating this one", ErrActiveTypoProfileRequired)
		}
		updates := map[string]any{
			"name": input.Name, "minimum_token_length": input.MinimumTokenLength,
			"one_edit_minimum_length": input.OneEditMinimumLength,
			"two_edit_minimum_length": input.TwoEditMinimumLength,
			"strict_mode":             input.StrictMode, "updated_by": input.UpdatedBy, "updated_at": s.now().UTC(),
		}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		if input.IsActive && !current.IsActive {
			return s.activateTypoToleranceProfileTx(tx, id, input.UpdatedBy)
		}
		return nil
	})
	if err != nil {
		return models.SearchTypoToleranceProfile{}, configurationWriteError(err)
	}
	return s.GetTypoToleranceProfile(ctx, id)
}

func (s *Service) ActivateTypoToleranceProfile(ctx context.Context, id uint, updatedBy *uint) (models.SearchTypoToleranceProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return s.activateTypoToleranceProfileTx(tx, id, updatedBy)
	}); err != nil {
		return models.SearchTypoToleranceProfile{}, configurationWriteError(err)
	}
	return s.GetTypoToleranceProfile(ctx, id)
}

func (s *Service) DeleteTypoToleranceProfile(ctx context.Context, id uint) error {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var profile models.SearchTypoToleranceProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&profile, id).Error; err != nil {
			return configurationLookupError(err)
		}
		if profile.IsActive {
			return fmt.Errorf("%w: activate another profile before deleting this one", ErrActiveTypoProfileRequired)
		}
		result := tx.Unscoped().Delete(&profile)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConfigurationNotFound
		}
		return nil
	})
}

func (s *Service) activateTypoToleranceProfileTx(tx *gorm.DB, id uint, updatedBy *uint) error {
	var profiles []models.SearchTypoToleranceProfile
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC").Find(&profiles).Error; err != nil {
		return err
	}
	found := false
	for _, profile := range profiles {
		if profile.ID == id {
			found = true
			break
		}
	}
	if !found {
		return ErrConfigurationNotFound
	}
	now := s.now().UTC()
	if err := tx.Model(&models.SearchTypoToleranceProfile{}).
		Where("is_active = ? AND id <> ?", true, id).
		Updates(map[string]any{"is_active": false, "updated_by": updatedBy, "updated_at": now}).Error; err != nil {
		return err
	}
	return tx.Model(&models.SearchTypoToleranceProfile{}).Where("id = ?", id).
		Updates(map[string]any{"is_active": true, "updated_by": updatedBy, "updated_at": now}).Error
}

func (s *Service) configurationDB(ctx context.Context) (*gorm.DB, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("search database is required")
	}
	return s.db.WithContext(ctx), nil
}

func normalizeSynonymSet(input SynonymSetInput) (string, string, error) {
	name, err := normalizeConfigurationName(input.Name)
	if err != nil {
		return "", "", err
	}
	if input.Direction != SynonymDirectionUnidirectional && input.Direction != SynonymDirectionBidirectional {
		return "", "", fmt.Errorf("%w: direction must be %q or %q", ErrConfigurationInvalid, SynonymDirectionUnidirectional, SynonymDirectionBidirectional)
	}
	terms, err := normalizeSynonymTerms(input.Terms)
	if err != nil {
		return "", "", err
	}
	raw, err := json.Marshal(terms)
	if err != nil {
		return "", "", err
	}
	return name, string(raw), nil
}

func normalizeSynonymTerms(terms []string) ([]string, error) {
	if len(terms) < 2 || len(terms) > 50 {
		return nil, fmt.Errorf("%w: synonym sets require between 2 and 50 terms", ErrConfigurationInvalid)
	}
	normalized := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		term = NormalizeQuery(term)
		if term == "" {
			return nil, fmt.Errorf("%w: synonym terms cannot be blank", ErrConfigurationInvalid)
		}
		if utf8.RuneCountInString(term) > 120 {
			return nil, fmt.Errorf("%w: synonym terms cannot exceed 120 characters", ErrConfigurationInvalid)
		}
		if _, exists := seen[term]; exists {
			return nil, fmt.Errorf("%w: synonym terms must be unique after normalization", ErrConfigurationInvalid)
		}
		seen[term] = struct{}{}
		normalized = append(normalized, term)
	}
	return normalized, nil
}

func ParseSynonymTerms(raw string) ([]string, error) {
	var terms []string
	if err := json.Unmarshal([]byte(raw), &terms); err != nil {
		return nil, fmt.Errorf("%w: invalid synonym terms: %v", ErrConfigurationInvalid, err)
	}
	return normalizeSynonymTerms(terms)
}

func synonymSetFromModel(row models.SearchSynonymSet) (SynonymSet, error) {
	terms, err := ParseSynonymTerms(row.TermsJSON)
	if err != nil {
		return SynonymSet{}, err
	}
	return SynonymSet{
		ID: row.ID, Name: row.Name, Direction: row.Direction, Terms: terms,
		IsActive: row.IsActive, UpdatedBy: row.UpdatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func normalizeTypoToleranceProfile(input TypoToleranceProfileInput) (TypoToleranceProfileInput, error) {
	name, err := normalizeConfigurationName(input.Name)
	if err != nil {
		return TypoToleranceProfileInput{}, err
	}
	input.Name = name
	if input.MinimumTokenLength < 1 || input.MinimumTokenLength > 128 {
		return TypoToleranceProfileInput{}, fmt.Errorf("%w: minimum token length must be between 1 and 128", ErrConfigurationInvalid)
	}
	if input.OneEditMinimumLength < input.MinimumTokenLength || input.OneEditMinimumLength > 128 {
		return TypoToleranceProfileInput{}, fmt.Errorf("%w: one-edit minimum length must be between the minimum token length and 128", ErrConfigurationInvalid)
	}
	if input.TwoEditMinimumLength <= input.OneEditMinimumLength || input.TwoEditMinimumLength > 128 {
		return TypoToleranceProfileInput{}, fmt.Errorf("%w: two-edit minimum length must be greater than the one-edit minimum length and at most 128", ErrConfigurationInvalid)
	}
	return input, nil
}

func normalizeConfigurationName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: name is required", ErrConfigurationInvalid)
	}
	if utf8.RuneCountInString(value) > 120 {
		return "", fmt.Errorf("%w: name cannot exceed 120 characters", ErrConfigurationInvalid)
	}
	return value, nil
}

func configurationLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrConfigurationNotFound
	}
	return err
}

func configurationWriteError(err error) error {
	if errors.Is(err, ErrConfigurationInvalid) || errors.Is(err, ErrConfigurationNotFound) || errors.Is(err, ErrActiveTypoProfileRequired) {
		return err
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key") || strings.Contains(message, "duplicated key") {
		return fmt.Errorf("%w: name already exists", ErrConfigurationConflict)
	}
	return err
}
