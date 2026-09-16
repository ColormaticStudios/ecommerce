package localization

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"ecommerce/models"

	"golang.org/x/text/language"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidLocale       = errors.New("invalid locale configuration")
	ErrLocaleNotFound      = errors.New("locale not found")
	ErrLocaleDisabled      = errors.New("locale is disabled")
	ErrRegistryUnavailable = errors.New("locale registry is unavailable")
	localeCodePattern      = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	marketCodePattern      = regexp.MustCompile(`^[A-Z]{2,3}$`)
)

type Service struct {
	db          *gorm.DB
	bundleCache bundleCache
}

type bundleCache struct {
	mu        sync.RWMutex
	releaseID uint
	values    map[string]Bundle
}

type LocaleInput struct {
	Code           string
	Name           string
	IsEnabled      bool
	IsDefault      bool
	FallbackLocale string
	DefaultMarkets []string
}

type LocaleRecord struct {
	ID             uint
	Code           string
	Name           string
	IsEnabled      bool
	IsDefault      bool
	FallbackLocale string
	DefaultMarkets []string
}

type ResolutionInput struct {
	ExplicitLocale    string
	AccountPreference string
	MarketDefault     string
}

type Resolution struct {
	RequestedLocale string
	ResolvedLocale  string
	Source          string
	FallbackChain   []string
	UsedFallback    bool
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, bundleCache: bundleCache{values: map[string]Bundle{}}}
}

func NormalizeLocale(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !localeCodePattern.MatchString(value) {
		return "", ErrInvalidLocale
	}
	tag, err := language.Parse(value)
	if err != nil || tag == language.Und {
		return "", ErrInvalidLocale
	}
	return tag.String(), nil
}

func (s *Service) ListLocales(ctx context.Context, enabledOnly bool) ([]LocaleRecord, error) {
	query := s.db.WithContext(ctx).Model(&models.Locale{}).Preload("FallbackLocale")
	if enabledOnly {
		query = query.Where("is_enabled = ?", true)
	}
	var rows []models.Locale
	if err := query.Order("is_default DESC, code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	var marketDefaults []models.LocaleMarketDefault
	if err := s.db.WithContext(ctx).Order("market ASC").Find(&marketDefaults).Error; err != nil {
		return nil, err
	}
	marketsByLocale := make(map[uint][]string)
	for _, marketDefault := range marketDefaults {
		marketsByLocale[marketDefault.LocaleID] = append(marketsByLocale[marketDefault.LocaleID], marketDefault.Market)
	}
	result := make([]LocaleRecord, 0, len(rows))
	for _, row := range rows {
		defaultMarkets := append([]string{}, marketsByLocale[row.ID]...)
		record := LocaleRecord{ID: row.ID, Code: row.Code, Name: row.Name, IsEnabled: row.IsEnabled, IsDefault: row.IsDefault, DefaultMarkets: defaultMarkets}
		if row.FallbackLocale != nil {
			record.FallbackLocale = row.FallbackLocale.Code
		}
		result = append(result, record)
	}
	return result, nil
}

func (s *Service) ReplaceLocales(ctx context.Context, inputs []LocaleInput) ([]LocaleRecord, error) {
	normalized, err := validateLocaleInputs(inputs)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear the old default and fallback references first so the partial
		// default constraint and self-references remain valid while replacing.
		if err := tx.Model(&models.Locale{}).Where("1 = 1").Updates(map[string]any{
			"is_default": false, "fallback_locale_id": nil,
		}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.LocaleMarketDefault{}).Error; err != nil {
			return err
		}

		byCode := make(map[string]models.Locale, len(normalized))
		codes := make([]string, 0, len(normalized))
		for _, input := range normalized {
			codes = append(codes, input.Code)
			var locale models.Locale
			findErr := tx.Unscoped().Where("code = ?", input.Code).First(&locale).Error
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				locale = models.Locale{Code: input.Code}
			} else if findErr != nil {
				return findErr
			}
			locale.DeletedAt = gorm.DeletedAt{}
			locale.Name = input.Name
			locale.IsEnabled = input.IsEnabled
			locale.IsDefault = input.IsDefault
			locale.FallbackLocaleID = nil
			if locale.ID == 0 {
				wantEnabled := locale.IsEnabled
				if err := tx.Select("*").Create(&locale).Error; err != nil {
					return err
				}
				// GORM applies the model's default:true tag to a false bool during
				// Create, even when the field is explicitly selected, and it mutates
				// the in-memory struct back to true too. Persist and restore the
				// requested false value explicitly using the value captured before Create.
				if !wantEnabled {
					if err := tx.Model(&locale).Update("is_enabled", false).Error; err != nil {
						return err
					}
					locale.IsEnabled = false
				}
			} else if err := tx.Select("*").Save(&locale).Error; err != nil {
				return err
			}
			byCode[input.Code] = locale
		}
		if len(codes) > 0 {
			if err := tx.Model(&models.Locale{}).Where("code NOT IN ?", codes).Updates(map[string]any{
				"is_enabled": false, "is_default": false, "fallback_locale_id": nil,
			}).Error; err != nil {
				return err
			}
		}
		for _, input := range normalized {
			if input.FallbackLocale == "" {
				continue
			}
			locale := byCode[input.Code]
			fallback := byCode[input.FallbackLocale]
			if err := tx.Model(&models.Locale{}).Where("id = ?", locale.ID).Update("fallback_locale_id", fallback.ID).Error; err != nil {
				return err
			}
		}
		for _, input := range normalized {
			locale := byCode[input.Code]
			for _, market := range input.DefaultMarkets {
				row := models.LocaleMarketDefault{Market: market, LocaleID: locale.ID}
				if err := tx.Select("*").Create(&row).Error; err != nil {
					return err
				}
			}
		}
		if tx.Migrator().HasTable(&models.LocalizationRollout{}) {
			for _, input := range normalized {
				locale := byCode[input.Code]
				for _, domain := range RolloutDomains() {
					row := models.LocalizationRollout{LocaleID: locale.ID, Domain: domain, IsEnabled: true, Percentage: 100}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Select("*").Create(&row).Error; err != nil {
						return err
					}
				}
			}
		}
		if tx.Migrator().HasTable(&models.TranslationRelease{}) {
			var active models.TranslationRelease
			activeErr := tx.Where("status = ?", models.TranslationReleaseStatusActive).First(&active).Error
			if activeErr != nil && !errors.Is(activeErr, gorm.ErrRecordNotFound) {
				return activeErr
			}
			if activeErr == nil {
				quality, err := validateReleaseQuality(tx, active.ID)
				if err != nil {
					return err
				}
				if !quality.Ready {
					return fmt.Errorf("%w: active release is missing %d critical translations for the proposed locale registry", ErrInvalidLocale, len(quality.Missing))
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidateBundleCache(0)
	return s.ListLocales(ctx, false)
}

func validateLocaleInputs(inputs []LocaleInput) ([]LocaleInput, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: at least one locale is required", ErrInvalidLocale)
	}
	normalized := make([]LocaleInput, len(inputs))
	known := make(map[string]int, len(inputs))
	knownMarkets := make(map[string]string)
	defaultCount := 0
	for index, input := range inputs {
		code, err := NormalizeLocale(input.Code)
		if err != nil || strings.TrimSpace(input.Name) == "" {
			return nil, fmt.Errorf("%w: locale code and name are required", ErrInvalidLocale)
		}
		if _, exists := known[code]; exists {
			return nil, fmt.Errorf("%w: duplicate locale %s", ErrInvalidLocale, code)
		}
		input.Code = code
		input.Name = strings.TrimSpace(input.Name)
		if strings.TrimSpace(input.FallbackLocale) != "" {
			input.FallbackLocale, err = NormalizeLocale(input.FallbackLocale)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid fallback for %s", ErrInvalidLocale, code)
			}
		}
		if input.IsDefault {
			defaultCount++
			if !input.IsEnabled {
				return nil, fmt.Errorf("%w: default locale must be enabled", ErrInvalidLocale)
			}
		}
		markets := make([]string, 0, len(input.DefaultMarkets))
		if len(input.DefaultMarkets) > 0 && !input.IsEnabled {
			return nil, fmt.Errorf("%w: market defaults require enabled locale %s", ErrInvalidLocale, code)
		}
		for _, market := range input.DefaultMarkets {
			market = strings.ToUpper(strings.TrimSpace(market))
			if !marketCodePattern.MatchString(market) {
				return nil, fmt.Errorf("%w: invalid market %q for %s", ErrInvalidLocale, market, code)
			}
			if owner, exists := knownMarkets[market]; exists {
				return nil, fmt.Errorf("%w: market %s is assigned to both %s and %s", ErrInvalidLocale, market, owner, code)
			}
			knownMarkets[market] = code
			markets = append(markets, market)
		}
		sort.Strings(markets)
		input.DefaultMarkets = markets
		known[code] = index
		normalized[index] = input
	}
	if defaultCount != 1 {
		return nil, fmt.Errorf("%w: exactly one default locale is required", ErrInvalidLocale)
	}
	for _, input := range normalized {
		if input.FallbackLocale == "" {
			continue
		}
		fallbackIndex, exists := known[input.FallbackLocale]
		if !exists {
			return nil, fmt.Errorf("%w: fallback locale %s is not configured", ErrInvalidLocale, input.FallbackLocale)
		}
		if input.FallbackLocale == input.Code {
			return nil, fmt.Errorf("%w: locale cannot fall back to itself", ErrInvalidLocale)
		}
		if input.IsEnabled && !normalized[fallbackIndex].IsEnabled {
			return nil, fmt.Errorf("%w: enabled locale %s cannot fall back to disabled locale %s", ErrInvalidLocale, input.Code, input.FallbackLocale)
		}
		seen := map[string]bool{input.Code: true}
		current := input.FallbackLocale
		for current != "" {
			if seen[current] {
				return nil, fmt.Errorf("%w: fallback cycle includes %s", ErrInvalidLocale, current)
			}
			seen[current] = true
			current = normalized[known[current]].FallbackLocale
		}
	}
	return normalized, nil
}

func (s *Service) MarketDefault(ctx context.Context, market string) (string, error) {
	market = strings.ToUpper(strings.TrimSpace(market))
	if market == "" {
		return "", nil
	}
	if !marketCodePattern.MatchString(market) {
		return "", fmt.Errorf("%w: invalid market %q", ErrInvalidLocale, market)
	}
	var row models.LocaleMarketDefault
	if err := s.db.WithContext(ctx).Preload("Locale").Where("market = ?", market).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if !row.Locale.IsEnabled {
		return "", nil
	}
	return row.Locale.Code, nil
}

func (s *Service) RequireEnabledLocale(ctx context.Context, code string) (LocaleRecord, error) {
	normalized, err := NormalizeLocale(code)
	if err != nil {
		return LocaleRecord{}, err
	}
	var row models.Locale
	if err := s.db.WithContext(ctx).Preload("FallbackLocale").Where("code = ?", normalized).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LocaleRecord{}, ErrLocaleNotFound
		}
		return LocaleRecord{}, err
	}
	if !row.IsEnabled {
		return LocaleRecord{}, ErrLocaleDisabled
	}
	record := LocaleRecord{ID: row.ID, Code: row.Code, Name: row.Name, IsEnabled: row.IsEnabled, IsDefault: row.IsDefault}
	if row.FallbackLocale != nil {
		record.FallbackLocale = row.FallbackLocale.Code
	}
	return record, nil
}

func (s *Service) ResolveLocale(ctx context.Context, input ResolutionInput) (Resolution, error) {
	locales, err := s.ListLocales(ctx, true)
	if err != nil {
		return Resolution{}, err
	}
	if len(locales) == 0 {
		return Resolution{}, ErrRegistryUnavailable
	}
	byCode := make(map[string]LocaleRecord, len(locales))
	defaultLocale := ""
	for _, locale := range locales {
		byCode[locale.Code] = locale
		if locale.IsDefault {
			if defaultLocale != "" {
				return Resolution{}, ErrRegistryUnavailable
			}
			defaultLocale = locale.Code
		}
	}
	if defaultLocale == "" {
		return Resolution{}, ErrRegistryUnavailable
	}

	candidates := []struct {
		value  string
		source string
	}{
		{input.ExplicitLocale, "explicit"},
		{input.AccountPreference, "account"},
		{input.MarketDefault, "market"},
		{defaultLocale, "global"},
	}
	requested := ""
	resolved, source := defaultLocale, "global"
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.value) == "" {
			continue
		}
		normalized, normalizeErr := NormalizeLocale(candidate.value)
		if requested == "" && normalizeErr == nil {
			requested = normalized
		}
		if normalizeErr != nil {
			continue
		}
		if _, enabled := byCode[normalized]; enabled {
			resolved, source = normalized, candidate.source
			break
		}
	}
	if requested == "" {
		requested = defaultLocale
	}
	chain := fallbackChain(resolved, defaultLocale, byCode)
	return Resolution{
		RequestedLocale: requested,
		ResolvedLocale:  resolved,
		Source:          source,
		FallbackChain:   chain,
		UsedFallback:    requested != resolved,
	}, nil
}

func fallbackChain(requested, defaultLocale string, locales map[string]LocaleRecord) []string {
	chain := make([]string, 0, 4)
	seen := map[string]bool{}
	current := requested
	for current != "" && !seen[current] {
		seen[current] = true
		chain = append(chain, current)
		configured, exists := locales[current]
		if exists && configured.FallbackLocale != "" {
			current = configured.FallbackLocale
			continue
		}
		if separator := strings.LastIndexByte(current, '-'); separator > 0 {
			parent := current[:separator]
			if _, exists := locales[parent]; exists {
				current = parent
				continue
			}
		}
		current = ""
	}
	if !seen[defaultLocale] {
		chain = append(chain, defaultLocale)
	}
	return chain
}

func SortedNamespaceTaxonomy() []string {
	result := []string{"admin", "checkout", "communications", "errors", "storefront"}
	sort.Strings(result)
	return result
}
