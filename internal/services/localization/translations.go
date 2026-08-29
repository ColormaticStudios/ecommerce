package localization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidTranslation       = errors.New("invalid translation")
	ErrTranslationKeyNotFound   = errors.New("translation key not found")
	ErrTranslationValueNotFound = errors.New("translation value not found")
	ErrInvalidTransition        = errors.New("invalid translation state transition")
	ErrReleaseNotFound          = errors.New("translation release not found")
	ErrReleaseUnavailable       = errors.New("active translation release is unavailable")
	ErrReleaseQualityFailed     = errors.New("translation release does not meet the localization quality bar")
	keyPattern                  = regexp.MustCompile(`^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*$`)
)

type KeyInput struct {
	Namespace   string
	Key         string
	SourceText  string
	Description string
	OwnerDomain string
}

type LookupResult struct {
	Key                string
	Value              string
	RequestedLocale    string
	SourceLocale       string
	UsedFallback       bool
	MissingTranslation bool
}

type ReleaseMetadata struct {
	ID           uint
	Name         string
	SnapshotHash string
	Version      string
	PublishedAt  time.Time
}

type Bundle struct {
	Resolution Resolution
	Release    ReleaseMetadata
	Messages   map[string]LookupResult
}

var criticalReleaseNamespaces = []string{"checkout", "errors", "communications"}

type ReleaseQualityMissing struct {
	Locale    string
	Namespace string
	Key       string
}

type ReleaseQuality struct {
	ReleaseID          uint
	Ready              bool
	RequiredLocales    []string
	CriticalNamespaces []string
	Missing            []ReleaseQualityMissing
}

func (s *Service) CreateKey(ctx context.Context, input KeyInput) (models.TranslationKey, error) {
	input.Namespace = strings.TrimSpace(input.Namespace)
	input.Key = strings.TrimSpace(input.Key)
	input.SourceText = strings.TrimSpace(input.SourceText)
	input.Description = strings.TrimSpace(input.Description)
	input.OwnerDomain = strings.TrimSpace(input.OwnerDomain)
	if !validNamespace(input.Namespace) || !keyPattern.MatchString(input.Key) || input.SourceText == "" || input.OwnerDomain == "" {
		return models.TranslationKey{}, ErrInvalidTranslation
	}
	row := models.TranslationKey{
		Namespace: input.Namespace, Key: input.Key, SourceText: input.SourceText,
		Description: input.Description, OwnerDomain: input.OwnerDomain,
	}
	if err := s.db.WithContext(ctx).Select("*").Create(&row).Error; err != nil {
		return models.TranslationKey{}, err
	}
	return row, nil
}

func validNamespace(namespace string) bool {
	for _, allowed := range SortedNamespaceTaxonomy() {
		if namespace == allowed {
			return true
		}
	}
	return false
}

func (s *Service) CreateValue(ctx context.Context, keyID uint, localeCode, value string, actorID *uint) (models.TranslationValue, error) {
	return s.CreateValueWithOptions(ctx, keyID, localeCode, value, actorID, ValueOptions{})
}

func (s *Service) TransitionValue(ctx context.Context, valueID uint, target models.TranslationState, actorID *uint) (models.TranslationValue, error) {
	return s.TransitionValueWithAudit(ctx, valueID, target, actorID, "")
}

type releaseValueRow struct {
	ID               uint
	TranslationKeyID uint
	LocaleID         uint
	Value            string
	Version          uint
	Namespace        string
	Key              string
	LocaleCode       string
}

func (s *Service) CreateRelease(ctx context.Context, name, notes string) (models.TranslationRelease, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.TranslationRelease{}, ErrInvalidTranslation
	}
	var result models.TranslationRelease
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []releaseValueRow
		if err := tx.Table("translation_values AS tv").
			Select("tv.id, tv.translation_key_id, tv.locale_id, tv.value, tv.version, tk.namespace, tk.key, l.code AS locale_code").
			Joins("JOIN translation_keys AS tk ON tk.id = tv.translation_key_id AND tk.deleted_at IS NULL").
			Joins("JOIN locales AS l ON l.id = tv.locale_id AND l.deleted_at IS NULL").
			Where("tv.deleted_at IS NULL AND tv.state = ? AND tk.is_deprecated = ? AND l.is_enabled = ?", models.TranslationStatePublished, false, true).
			Order("tk.namespace ASC, tk.key ASC, l.code ASC, tv.version DESC").Scan(&rows).Error; err != nil {
			return err
		}
		selected := make([]releaseValueRow, 0, len(rows))
		seen := map[string]bool{}
		for _, row := range rows {
			identity := fmt.Sprintf("%d:%d", row.TranslationKeyID, row.LocaleID)
			if seen[identity] {
				continue
			}
			seen[identity] = true
			selected = append(selected, row)
		}
		hash, err := snapshotHash(selected)
		if err != nil {
			return err
		}
		result = models.TranslationRelease{
			Name: name, Notes: strings.TrimSpace(notes), Status: models.TranslationReleaseStatusDraft, SnapshotHash: hash,
		}
		if err := tx.Select("*").Create(&result).Error; err != nil {
			return err
		}
		for _, row := range selected {
			entry := models.TranslationReleaseEntry{ReleaseID: result.ID, TranslationValueID: row.ID}
			if err := tx.Select("*").Create(&entry).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func snapshotHash(rows []releaseValueRow) (string, error) {
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Namespace + "." + rows[i].Key + "\x00" + rows[i].LocaleCode
		right := rows[j].Namespace + "." + rows[j].Key + "\x00" + rows[j].LocaleCode
		return left < right
	})
	payload := make([]struct {
		Key     string `json:"key"`
		Locale  string `json:"locale"`
		Value   string `json:"value"`
		Version uint   `json:"version"`
	}, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, struct {
			Key     string `json:"key"`
			Locale  string `json:"locale"`
			Value   string `json:"value"`
			Version uint   `json:"version"`
		}{Key: row.Namespace + "." + row.Key, Locale: row.LocaleCode, Value: row.Value, Version: row.Version})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) ActivateRelease(ctx context.Context, releaseID uint, actorID *uint) (models.TranslationRelease, error) {
	var result models.TranslationRelease
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, releaseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReleaseNotFound
			}
			return err
		}
		if result.Status == models.TranslationReleaseStatusActive {
			return nil
		}
		if result.Status != models.TranslationReleaseStatusDraft {
			return ErrInvalidTransition
		}
		quality, err := validateReleaseQuality(tx, releaseID)
		if err != nil {
			return err
		}
		if !quality.Ready {
			return fmt.Errorf("%w: %d critical translations are missing", ErrReleaseQualityFailed, len(quality.Missing))
		}
		if err := tx.Model(&models.TranslationRelease{}).
			Where("status = ?", models.TranslationReleaseStatusActive).
			Update("status", models.TranslationReleaseStatusSuperseded).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		result.Status = models.TranslationReleaseStatusActive
		result.PublishedAt = &now
		result.PublishedBy = actorID
		return tx.Select("*").Save(&result).Error
	})
	if err == nil {
		s.invalidateBundleCache(result.ID)
	}
	return result, err
}

func (s *Service) ActiveRelease(ctx context.Context) (models.TranslationRelease, error) {
	var release models.TranslationRelease
	if err := s.db.WithContext(ctx).Where("status = ?", models.TranslationReleaseStatusActive).First(&release).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.TranslationRelease{}, ErrReleaseUnavailable
		}
		return models.TranslationRelease{}, err
	}
	if release.PublishedAt == nil {
		return models.TranslationRelease{}, ErrReleaseUnavailable
	}
	return release, nil
}

func (s *Service) Bundle(ctx context.Context, input ResolutionInput, namespace string) (Bundle, error) {
	resolution, err := s.ResolveLocale(ctx, input)
	if err != nil {
		return Bundle{}, err
	}
	return s.BundleForResolution(ctx, resolution, namespace)
}

func (s *Service) BundleForResolution(ctx context.Context, resolution Resolution, namespace string) (Bundle, error) {
	if len(resolution.FallbackChain) == 0 || strings.TrimSpace(resolution.ResolvedLocale) == "" {
		return Bundle{}, ErrRegistryUnavailable
	}
	if namespace != "" && !validNamespace(namespace) {
		return Bundle{}, ErrInvalidTranslation
	}
	release, err := s.ActiveRelease(ctx)
	if err != nil {
		return Bundle{}, err
	}
	cacheKey := bundleCacheKey(release, resolution, namespace)
	if cached, ok := s.cachedBundle(release.ID, cacheKey); ok {
		s.recordBundleMetrics(ctx, cached)
		return cached, nil
	}
	keyQuery := s.db.WithContext(ctx).Where("is_deprecated = ?", false)
	if namespace != "" {
		keyQuery = keyQuery.Where("namespace = ?", namespace)
	}
	var keys []models.TranslationKey
	if err := keyQuery.Order("namespace ASC, key ASC").Find(&keys).Error; err != nil {
		return Bundle{}, err
	}
	var values []releaseValueRow
	if err := s.db.WithContext(ctx).Table("translation_release_entries AS re").
		Select("tv.id, tv.translation_key_id, tv.locale_id, tv.value, tv.version, tk.namespace, tk.key, l.code AS locale_code").
		Joins("JOIN translation_values AS tv ON tv.id = re.translation_value_id AND tv.deleted_at IS NULL").
		Joins("JOIN translation_keys AS tk ON tk.id = tv.translation_key_id AND tk.deleted_at IS NULL").
		Joins("JOIN locales AS l ON l.id = tv.locale_id AND l.deleted_at IS NULL").
		Where("re.release_id = ? AND re.deleted_at IS NULL", release.ID).Scan(&values).Error; err != nil {
		return Bundle{}, err
	}
	byKeyLocale := make(map[string]releaseValueRow, len(values))
	for _, value := range values {
		byKeyLocale[fmt.Sprintf("%d:%s", value.TranslationKeyID, value.LocaleCode)] = value
	}
	messages := make(map[string]LookupResult, len(keys))
	lookupCounts := make(map[string]int64)
	for _, key := range keys {
		qualified := key.Namespace + "." + key.Key
		domain := rolloutDomainForKey(key)
		lookupCounts[domain]++
		result := LookupResult{Key: qualified, RequestedLocale: resolution.RequestedLocale}
		for _, localeCode := range resolution.FallbackChain {
			if value, ok := byKeyLocale[fmt.Sprintf("%d:%s", key.ID, localeCode)]; ok {
				result.Value = value.Value
				result.SourceLocale = localeCode
				result.UsedFallback = localeCode != resolution.ResolvedLocale
				break
			}
		}
		if result.Value == "" {
			result.Value = key.SourceText
			result.SourceLocale = resolution.FallbackChain[len(resolution.FallbackChain)-1]
			result.UsedFallback = true
			result.MissingTranslation = true
		}
		messages[qualified] = result
	}
	version := bundleVersion(release.SnapshotHash, resolution, namespace)
	bundle := Bundle{
		Resolution: resolution,
		Release:    ReleaseMetadata{ID: release.ID, Name: release.Name, SnapshotHash: release.SnapshotHash, Version: version, PublishedAt: *release.PublishedAt},
		Messages:   messages,
	}
	s.storeCachedBundle(release.ID, cacheKey, bundle)
	for domain, count := range lookupCounts {
		s.recordMetricBestEffort(ctx, MetricLookup, resolution.ResolvedLocale, domain, "", count, 0)
	}
	for qualified, result := range messages {
		domain := rolloutDomainForQualifiedKey(qualified)
		if result.UsedFallback {
			s.recordMetricBestEffort(ctx, MetricFallbackHit, resolution.ResolvedLocale, domain, qualified, 1, 0)
		}
		if result.MissingTranslation {
			s.recordMetricBestEffort(ctx, MetricMissingKey, resolution.ResolvedLocale, domain, qualified, 1, 0)
		}
	}
	return cloneBundle(bundle), nil
}

func bundleVersion(snapshotHash string, resolution Resolution, namespace string) string {
	payload := strings.Join([]string{snapshotHash, resolution.RequestedLocale, resolution.ResolvedLocale, strings.Join(resolution.FallbackChain, ","), namespace}, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func bundleCacheKey(release models.TranslationRelease, resolution Resolution, namespace string) string {
	return fmt.Sprintf("%d:%s", release.ID, bundleVersion(release.SnapshotHash, resolution, namespace))
}

func (s *Service) cachedBundle(releaseID uint, key string) (Bundle, bool) {
	s.bundleCache.mu.RLock()
	defer s.bundleCache.mu.RUnlock()
	if s.bundleCache.releaseID != releaseID {
		return Bundle{}, false
	}
	value, ok := s.bundleCache.values[key]
	return cloneBundle(value), ok
}

func (s *Service) storeCachedBundle(releaseID uint, key string, value Bundle) {
	s.bundleCache.mu.Lock()
	defer s.bundleCache.mu.Unlock()
	if s.bundleCache.releaseID != releaseID {
		s.bundleCache.releaseID = releaseID
		s.bundleCache.values = map[string]Bundle{}
	}
	if len(s.bundleCache.values) >= 64 {
		s.bundleCache.values = map[string]Bundle{}
	}
	s.bundleCache.values[key] = cloneBundle(value)
}

func (s *Service) invalidateBundleCache(activeReleaseID uint) {
	s.bundleCache.mu.Lock()
	defer s.bundleCache.mu.Unlock()
	s.bundleCache.releaseID = activeReleaseID
	s.bundleCache.values = map[string]Bundle{}
}

func cloneBundle(value Bundle) Bundle {
	value.Resolution.FallbackChain = append([]string(nil), value.Resolution.FallbackChain...)
	value.Messages = maps.Clone(value.Messages)
	return value
}

func (s *Service) recordBundleMetrics(ctx context.Context, bundle Bundle) {
	lookupCounts := map[string]int64{}
	for qualified, result := range bundle.Messages {
		domain := rolloutDomainForQualifiedKey(qualified)
		lookupCounts[domain]++
		if result.UsedFallback {
			s.recordMetricBestEffort(ctx, MetricFallbackHit, bundle.Resolution.ResolvedLocale, domain, qualified, 1, 0)
		}
		if result.MissingTranslation {
			s.recordMetricBestEffort(ctx, MetricMissingKey, bundle.Resolution.ResolvedLocale, domain, qualified, 1, 0)
		}
	}
	for domain, count := range lookupCounts {
		s.recordMetricBestEffort(ctx, MetricLookup, bundle.Resolution.ResolvedLocale, domain, "", count, 0)
	}
}

func rolloutDomainForQualifiedKey(qualified string) string {
	namespace, _, _ := strings.Cut(qualified, ".")
	if namespace == "errors" {
		return "errors"
	}
	if validRolloutDomain(namespace) {
		return namespace
	}
	return "storefront"
}

func (s *Service) ReleaseQuality(ctx context.Context, releaseID uint) (ReleaseQuality, error) {
	return validateReleaseQuality(s.db.WithContext(ctx), releaseID)
}

func validateReleaseQuality(db *gorm.DB, releaseID uint) (ReleaseQuality, error) {
	var releaseCount int64
	if err := db.Model(&models.TranslationRelease{}).Where("id = ?", releaseID).Count(&releaseCount).Error; err != nil {
		return ReleaseQuality{}, err
	}
	if releaseCount != 1 {
		return ReleaseQuality{}, ErrReleaseNotFound
	}
	var locales []models.Locale
	if err := db.Where("is_enabled = ?", true).Order("code ASC").Find(&locales).Error; err != nil {
		return ReleaseQuality{}, err
	}
	var keys []models.TranslationKey
	if err := db.Where("is_deprecated = ? AND namespace IN ?", false, criticalReleaseNamespaces).Order("namespace ASC, key ASC").Find(&keys).Error; err != nil {
		return ReleaseQuality{}, err
	}
	type releasedPair struct {
		TranslationKeyID uint
		LocaleID         uint
	}
	var pairs []releasedPair
	if err := db.Table("translation_release_entries AS re").
		Select("tv.translation_key_id, tv.locale_id").
		Joins("JOIN translation_values AS tv ON tv.id = re.translation_value_id AND tv.deleted_at IS NULL").
		Where("re.release_id = ? AND re.deleted_at IS NULL", releaseID).Scan(&pairs).Error; err != nil {
		return ReleaseQuality{}, err
	}
	present := make(map[string]bool, len(pairs))
	for _, pair := range pairs {
		present[fmt.Sprintf("%d:%d", pair.TranslationKeyID, pair.LocaleID)] = true
	}
	quality := ReleaseQuality{ReleaseID: releaseID, CriticalNamespaces: append([]string(nil), criticalReleaseNamespaces...)}
	localeByID := make(map[uint]models.Locale, len(locales))
	var defaultLocaleID uint
	for _, locale := range locales {
		localeByID[locale.ID] = locale
		if locale.IsDefault {
			defaultLocaleID = locale.ID
		}
	}
	for _, locale := range locales {
		quality.RequiredLocales = append(quality.RequiredLocales, locale.Code)
		for _, key := range keys {
			resolved := false
			visited := map[uint]bool{}
			candidate := locale
			for candidate.ID != 0 && !visited[candidate.ID] {
				visited[candidate.ID] = true
				if present[fmt.Sprintf("%d:%d", key.ID, candidate.ID)] {
					resolved = true
					break
				}
				if candidate.FallbackLocaleID == nil {
					break
				}
				candidate = localeByID[*candidate.FallbackLocaleID]
			}
			if !resolved && defaultLocaleID != 0 {
				resolved = present[fmt.Sprintf("%d:%d", key.ID, defaultLocaleID)]
			}
			if !resolved {
				quality.Missing = append(quality.Missing, ReleaseQualityMissing{Locale: locale.Code, Namespace: key.Namespace, Key: key.Key})
			}
		}
	}
	quality.Ready = len(quality.Missing) == 0
	return quality, nil
}

func rolloutDomainForKey(key models.TranslationKey) string {
	owner := strings.TrimSpace(key.OwnerDomain)
	if validRolloutDomain(owner) {
		return owner
	}
	if key.Namespace == "errors" {
		return "errors"
	}
	if validRolloutDomain(key.Namespace) {
		return key.Namespace
	}
	return "storefront"
}

func (s *Service) Lookup(ctx context.Context, namespace, key string, input ResolutionInput) (LookupResult, error) {
	bundle, err := s.Bundle(ctx, input, namespace)
	if err != nil {
		return LookupResult{}, err
	}
	result, exists := bundle.Messages[namespace+"."+key]
	if !exists {
		return LookupResult{}, ErrTranslationKeyNotFound
	}
	return result, nil
}
