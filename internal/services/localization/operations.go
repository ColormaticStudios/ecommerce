package localization

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MetricLookup          = "lookup"
	MetricMissingKey      = "missing_key"
	MetricFallbackHit     = "fallback_hit"
	MetricRolloutFallback = "rollout_fallback"
	MetricPublishLatency  = "publish_latency"
	MetricRollback        = "rollback"
)

var rolloutDomains = []string{"account", "admin", "checkout", "communications", "errors", "storefront"}

type RolloutInput struct {
	Locale     string
	Domain     string
	IsEnabled  bool
	Percentage int
}

type RolloutRecord struct {
	Locale     string
	Domain     string
	IsEnabled  bool
	Percentage int
	UpdatedAt  time.Time
}

type MetricHotspot struct {
	MetricType   string
	Locale       string
	Domain       string
	Key          string
	Count        int64
	AverageValue float64
	MaximumValue int64
	LastSeenAt   time.Time
}

type Metrics struct {
	LookupCount           int64
	MissingKeyCount       int64
	MissingKeyRate        float64
	FallbackHitCount      int64
	FallbackHitRate       float64
	RolloutFallbackCount  int64
	PublishCount          int64
	AveragePublishLatency float64
	MaximumPublishLatency int64
	RollbackCount         int64
	LocaleRates           []LocaleMetricRate
	Hotspots              []MetricHotspot
	GeneratedAt           time.Time
}

type LocaleMetricRate struct {
	Locale           string
	LookupCount      int64
	MissingKeyCount  int64
	FallbackHitCount int64
	MissingKeyRate   float64
	FallbackHitRate  float64
}

type CommunicationInput struct {
	Event           string
	Channel         string
	RecipientLocale string
	RecipientKey    string
	Parameters      map[string]string
}

type Communication struct {
	Event              string
	Channel            string
	Subject            string
	Body               string
	RequestedLocale    string
	ResolvedLocale     string
	SourceLocale       string
	UsedFallback       bool
	MissingTranslation bool
}

type communicationTemplate struct {
	SubjectKey string
	BodyKey    string
}

var communicationTemplates = map[string]map[string]communicationTemplate{
	"order_placed": {
		"email":        {SubjectKey: "order_placed.email.subject", BodyKey: "order_placed.email.body"},
		"text_message": {BodyKey: "order_placed.text_message.body"},
	},
	"payment_failed": {
		"email":        {SubjectKey: "payment_failed.email.subject", BodyKey: "payment_failed.email.body"},
		"text_message": {BodyKey: "payment_failed.text_message.body"},
	},
	"shipment_updated": {
		"email":        {SubjectKey: "shipment_updated.email.subject", BodyKey: "shipment_updated.email.body"},
		"text_message": {BodyKey: "shipment_updated.text_message.body"},
	},
	"return_approved": {
		"email":        {SubjectKey: "return_approved.email.subject", BodyKey: "return_approved.email.body"},
		"text_message": {BodyKey: "return_approved.text_message.body"},
	},
}

func RolloutDomains() []string {
	return append([]string(nil), rolloutDomains...)
}

func validRolloutDomain(domain string) bool {
	return slices.Contains(rolloutDomains, domain)
}

func DomainForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/admin"):
		return "admin"
	case strings.HasPrefix(path, "/api/v1/checkout"):
		return "checkout"
	case strings.HasPrefix(path, "/api/v1/me"), strings.HasPrefix(path, "/api/v1/auth"):
		return "account"
	default:
		return "storefront"
	}
}

func (s *Service) ListRollouts(ctx context.Context) ([]RolloutRecord, error) {
	locales, err := s.ListLocales(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []models.LocalizationRollout
	if err := s.db.WithContext(ctx).Preload("Locale").Order("locale_id ASC, domain ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	byIdentity := make(map[string]models.LocalizationRollout, len(rows))
	for _, row := range rows {
		byIdentity[row.Locale.Code+"\x00"+row.Domain] = row
	}
	result := make([]RolloutRecord, 0, len(locales)*len(rolloutDomains))
	for _, locale := range locales {
		for _, domain := range rolloutDomains {
			row, exists := byIdentity[locale.Code+"\x00"+domain]
			if !exists {
				result = append(result, RolloutRecord{Locale: locale.Code, Domain: domain, IsEnabled: true, Percentage: 100})
				continue
			}
			result = append(result, RolloutRecord{Locale: locale.Code, Domain: domain, IsEnabled: row.IsEnabled, Percentage: row.Percentage, UpdatedAt: row.UpdatedAt})
		}
	}
	return result, nil
}

func (s *Service) ReplaceRollouts(ctx context.Context, inputs []RolloutInput, actorID *uint) ([]RolloutRecord, error) {
	locales, err := s.ListLocales(ctx, false)
	if err != nil {
		return nil, err
	}
	localeByCode := make(map[string]LocaleRecord, len(locales))
	defaultLocale := ""
	for _, locale := range locales {
		localeByCode[locale.Code] = locale
		if locale.IsDefault {
			defaultLocale = locale.Code
		}
	}
	if len(inputs) != len(locales)*len(rolloutDomains) {
		return nil, fmt.Errorf("%w: rollout settings must include every locale and domain", ErrInvalidTranslation)
	}
	normalized := make([]RolloutInput, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		input.Locale, err = NormalizeLocale(input.Locale)
		input.Domain = strings.TrimSpace(input.Domain)
		if err != nil || !validRolloutDomain(input.Domain) || input.Percentage < 0 || input.Percentage > 100 {
			return nil, ErrInvalidTranslation
		}
		locale, exists := localeByCode[input.Locale]
		if !exists {
			return nil, ErrLocaleNotFound
		}
		identity := input.Locale + "\x00" + input.Domain
		if seen[identity] {
			return nil, fmt.Errorf("%w: duplicate rollout setting", ErrInvalidTranslation)
		}
		seen[identity] = true
		if locale.Code == defaultLocale && (!input.IsEnabled || input.Percentage != 100) {
			return nil, fmt.Errorf("%w: the default locale must remain fully rolled out", ErrInvalidTranslation)
		}
		normalized = append(normalized, input)
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, input := range normalized {
			locale := localeByCode[input.Locale]
			now := time.Now().UTC()
			row := map[string]any{
				"locale_id": locale.ID, "domain": input.Domain, "is_enabled": input.IsEnabled,
				"percentage": input.Percentage, "updated_by": actorID, "created_at": now, "updated_at": now, "deleted_at": nil,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "locale_id"}, {Name: "domain"}},
				DoUpdates: clause.AssignmentColumns([]string{"is_enabled", "percentage", "updated_by", "updated_at", "deleted_at"}),
			}).Table("localization_rollouts").Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ListRollouts(ctx)
}

func (s *Service) ApplyRollout(ctx context.Context, resolution Resolution, domain, identity string) (Resolution, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		domain = "storefront"
	}
	if !validRolloutDomain(domain) {
		return Resolution{}, ErrInvalidTranslation
	}
	if len(resolution.FallbackChain) == 0 {
		return Resolution{}, ErrRegistryUnavailable
	}
	var rows []models.LocalizationRollout
	if err := s.db.WithContext(ctx).Preload("Locale").Where("domain = ?", domain).Find(&rows).Error; err != nil {
		return Resolution{}, err
	}
	settings := make(map[string]models.LocalizationRollout, len(rows))
	for _, row := range rows {
		settings[row.Locale.Code] = row
	}
	original := resolution.ResolvedLocale
	selectedIndex := len(resolution.FallbackChain) - 1
	for index, locale := range resolution.FallbackChain {
		setting, exists := settings[locale]
		if !exists || setting.IsEnabled && stableRolloutBucket(identity, domain, locale) < setting.Percentage {
			selectedIndex = index
			break
		}
	}
	resolution.ResolvedLocale = resolution.FallbackChain[selectedIndex]
	resolution.FallbackChain = append([]string(nil), resolution.FallbackChain[selectedIndex:]...)
	if resolution.ResolvedLocale != original {
		resolution.UsedFallback = true
		s.recordMetricBestEffort(ctx, MetricRolloutFallback, original, domain, "", 1, 0)
	}
	return resolution, nil
}

func stableRolloutBucket(identity, domain, locale string) int {
	if strings.TrimSpace(identity) == "" {
		identity = "anonymous"
	}
	hash := sha256.Sum256([]byte(identity + "\x00" + domain + "\x00" + locale))
	return int(hash[0]) * 100 / 256
}

func RolloutIdentity(ctx context.Context) string {
	if principal, ok := requestctx.PrincipalFrom(ctx); ok {
		return "account:" + principal.Subject
	}
	metadata, _ := requestctx.MetadataFrom(ctx)
	for _, name := range []string{"localization_rollout", "checkout_session", "session_token"} {
		if value := strings.TrimSpace(metadata.Cookies[name]); value != "" {
			return "cookie:" + value
		}
	}
	return "client:" + metadata.Headers["X-Forwarded-For"] + "\x00" + metadata.Headers["User-Agent"]
}

func (s *Service) recordMetricBestEffort(ctx context.Context, metricType, locale, domain, key string, count, value int64) {
	if err := s.RecordMetric(ctx, metricType, locale, domain, key, count, value); err != nil {
		log.Printf("[WARN] localization metric write failed type=%s locale=%s domain=%s key=%s: %v", metricType, locale, domain, key, err)
	}
}

func (s *Service) RecordMetric(ctx context.Context, metricType, locale, domain, key string, count, value int64) error {
	if count < 1 {
		return ErrInvalidTranslation
	}
	now := time.Now().UTC()
	row := models.LocalizationMetric{MetricType: metricType, Locale: locale, Domain: domain, Key: key, Count: count, TotalValue: value, MaximumValue: value, LastSeenAt: now, CreatedAt: now, UpdatedAt: now}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "metric_type"}, {Name: "locale"}, {Name: "domain"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"count":         gorm.Expr("localization_metrics.count + ?", count),
			"total_value":   gorm.Expr("localization_metrics.total_value + ?", value),
			"maximum_value": gorm.Expr("CASE WHEN localization_metrics.maximum_value > ? THEN localization_metrics.maximum_value ELSE ? END", value, value),
			"last_seen_at":  now,
			"updated_at":    now,
		}),
	}).Create(&row).Error
}

func (s *Service) Metrics(ctx context.Context) (Metrics, error) {
	var rows []models.LocalizationMetric
	if err := s.db.WithContext(ctx).Order("count DESC, last_seen_at DESC").Find(&rows).Error; err != nil {
		return Metrics{}, err
	}
	result := Metrics{GeneratedAt: time.Now().UTC()}
	localeRates := make(map[string]*LocaleMetricRate)
	for _, row := range rows {
		localeRate := localeRates[row.Locale]
		if row.Locale != "" && localeRate == nil {
			localeRate = &LocaleMetricRate{Locale: row.Locale}
			localeRates[row.Locale] = localeRate
		}
		switch row.MetricType {
		case MetricLookup:
			result.LookupCount += row.Count
			if localeRate != nil {
				localeRate.LookupCount += row.Count
			}
		case MetricMissingKey:
			result.MissingKeyCount += row.Count
			if localeRate != nil {
				localeRate.MissingKeyCount += row.Count
			}
		case MetricFallbackHit:
			result.FallbackHitCount += row.Count
			if localeRate != nil {
				localeRate.FallbackHitCount += row.Count
			}
		case MetricRolloutFallback:
			result.RolloutFallbackCount += row.Count
		case MetricPublishLatency:
			result.PublishCount += row.Count
			result.AveragePublishLatency += float64(row.TotalValue)
			if row.MaximumValue > result.MaximumPublishLatency {
				result.MaximumPublishLatency = row.MaximumValue
			}
		case MetricRollback:
			result.RollbackCount += row.Count
		}
		if row.MetricType != MetricLookup && len(result.Hotspots) < 20 {
			average := float64(0)
			if row.Count > 0 {
				average = float64(row.TotalValue) / float64(row.Count)
			}
			result.Hotspots = append(result.Hotspots, MetricHotspot{MetricType: row.MetricType, Locale: row.Locale, Domain: row.Domain, Key: row.Key, Count: row.Count, AverageValue: average, MaximumValue: row.MaximumValue, LastSeenAt: row.LastSeenAt})
		}
	}
	if result.PublishCount > 0 {
		result.AveragePublishLatency /= float64(result.PublishCount)
	}
	if result.LookupCount > 0 {
		result.MissingKeyRate = float64(result.MissingKeyCount) / float64(result.LookupCount)
		result.FallbackHitRate = float64(result.FallbackHitCount) / float64(result.LookupCount)
	}
	for _, localeRate := range localeRates {
		if localeRate.LookupCount > 0 {
			localeRate.MissingKeyRate = float64(localeRate.MissingKeyCount) / float64(localeRate.LookupCount)
			localeRate.FallbackHitRate = float64(localeRate.FallbackHitCount) / float64(localeRate.LookupCount)
		}
		result.LocaleRates = append(result.LocaleRates, *localeRate)
	}
	sort.Slice(result.LocaleRates, func(left, right int) bool {
		return result.LocaleRates[left].Locale < result.LocaleRates[right].Locale
	})
	return result, nil
}

func (s *Service) RenderCommunication(ctx context.Context, input CommunicationInput) (Communication, error) {
	channels, exists := communicationTemplates[strings.TrimSpace(input.Event)]
	if !exists {
		return Communication{}, fmt.Errorf("%w: unknown communication event", ErrInvalidTranslation)
	}
	template, exists := channels[strings.TrimSpace(input.Channel)]
	if !exists {
		return Communication{}, fmt.Errorf("%w: unsupported communication channel", ErrInvalidTranslation)
	}
	resolution, err := s.ResolveLocale(ctx, ResolutionInput{ExplicitLocale: input.RecipientLocale})
	if err != nil {
		return Communication{}, err
	}
	resolution, err = s.ApplyRollout(ctx, resolution, "communications", input.RecipientKey)
	if err != nil {
		return Communication{}, err
	}
	bundle, err := s.BundleForResolution(ctx, resolution, "communications")
	if err != nil {
		return Communication{}, err
	}
	lookup := func(key string) (LookupResult, error) {
		if key == "" {
			return LookupResult{}, nil
		}
		value, found := bundle.Messages["communications."+key]
		if !found {
			return LookupResult{}, ErrTranslationKeyNotFound
		}
		return value, nil
	}
	subjectValue, err := lookup(template.SubjectKey)
	if err != nil {
		return Communication{}, err
	}
	bodyValue, err := lookup(template.BodyKey)
	if err != nil {
		return Communication{}, err
	}
	subject, err := interpolateCommunication(subjectValue.Value, input.Parameters)
	if err != nil {
		return Communication{}, err
	}
	body, err := interpolateCommunication(bodyValue.Value, input.Parameters)
	if err != nil {
		return Communication{}, err
	}
	usedFallback := resolution.UsedFallback || subjectValue.UsedFallback || bodyValue.UsedFallback
	missing := subjectValue.MissingTranslation || bodyValue.MissingTranslation
	sourceLocale := bodyValue.SourceLocale
	if sourceLocale == "" {
		sourceLocale = subjectValue.SourceLocale
	}
	if usedFallback {
		log.Printf("[INFO] localization communication fallback event=%s channel=%s requested_locale=%s resolved_locale=%s source_locale=%s", input.Event, input.Channel, input.RecipientLocale, resolution.ResolvedLocale, sourceLocale)
	}
	return Communication{Event: input.Event, Channel: input.Channel, Subject: subject, Body: body, RequestedLocale: input.RecipientLocale, ResolvedLocale: resolution.ResolvedLocale, SourceLocale: sourceLocale, UsedFallback: usedFallback, MissingTranslation: missing}, nil
}

func interpolateCommunication(template string, parameters map[string]string) (string, error) {
	if template == "" {
		return "", nil
	}
	missing := ""
	result := placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		value, exists := parameters[name]
		if !exists {
			missing = name
			return match
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("%w: missing communication parameter %s", ErrInvalidTranslation, missing)
	}
	return result, nil
}
