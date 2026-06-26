package cms

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
)

var ErrRedirectLoop = errors.New("cms redirect loop")

type RedirectInput struct {
	SourcePattern string
	MatchType     string
	TargetURL     string
	RedirectType  int
	Priority      int
	IsEnabled     bool
}

type RedirectService struct{ db *gorm.DB }

func NewRedirectService(db *gorm.DB) *RedirectService { return &RedirectService{db: db} }

func (s *RedirectService) List() ([]models.CMSRedirectRule, error) {
	var rules []models.CMSRedirectRule
	err := s.db.Order("priority DESC, source_pattern ASC, id ASC").Find(&rules).Error
	return rules, err
}

func (s *RedirectService) Create(input RedirectInput) (*models.CMSRedirectRule, error) {
	rule, err := normalizeRedirect(input)
	if err != nil {
		return nil, err
	}
	if err := s.validateRules(0, rule); err != nil {
		return nil, err
	}
	if err := s.db.Select("*").Create(&rule).Error; err != nil {
		if isUniqueConstraint(err) {
			return nil, fmt.Errorf("%w: redirect source already exists", ErrInvalidPage)
		}
		return nil, err
	}
	return &rule, nil
}

func (s *RedirectService) Update(id uint, input RedirectInput) (*models.CMSRedirectRule, error) {
	var existing models.CMSRedirectRule
	if err := s.db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rule, err := normalizeRedirect(input)
	if err != nil {
		return nil, err
	}
	rule.ID, rule.CreatedAt = existing.ID, existing.CreatedAt
	if err := s.validateRules(id, rule); err != nil {
		return nil, err
	}
	if err := s.db.Select("*").Save(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *RedirectService) Delete(id uint) error {
	result := s.db.Delete(&models.CMSRedirectRule{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *RedirectService) Resolve(requestPath string) (*models.CMSRedirectRule, string, error) {
	normalized, err := normalizeRedirectPath(requestPath)
	if err != nil {
		return nil, "", ErrNotFound
	}
	rules, err := s.enabledRules()
	if err != nil {
		return nil, "", err
	}
	for index := range rules {
		if target, ok := redirectTarget(rules[index], normalized); ok {
			return &rules[index], target, nil
		}
	}
	return nil, "", ErrNotFound
}

func (s *RedirectService) validateRules(excludeID uint, candidate models.CMSRedirectRule) error {
	rules, err := s.enabledRules()
	if err != nil {
		return err
	}
	filtered := make([]models.CMSRedirectRule, 0, len(rules)+1)
	for _, rule := range rules {
		if rule.ID != excludeID {
			filtered = append(filtered, rule)
		}
	}
	if candidate.IsEnabled {
		filtered = append(filtered, candidate)
	}
	sortRedirectRules(filtered)
	for _, start := range filtered {
		current := start.SourcePattern
		seen := map[string]bool{}
		for step := 0; step <= len(filtered); step++ {
			if seen[current] {
				return fmt.Errorf("%w: %s", ErrRedirectLoop, current)
			}
			seen[current] = true
			next := ""
			for _, rule := range filtered {
				if target, ok := redirectTarget(rule, current); ok && strings.HasPrefix(target, "/") {
					next = target
					break
				}
			}
			if next == "" {
				break
			}
			current = next
		}
	}
	if candidate.IsEnabled && strings.HasPrefix(candidate.TargetURL, "/") {
		finalTarget := candidate.TargetURL
		for step := 0; step < len(filtered); step++ {
			next := ""
			for _, rule := range filtered {
				if rule.ID == candidate.ID && rule.ID != 0 {
					continue
				}
				if target, ok := redirectTarget(rule, redirectPathOnly(finalTarget)); ok {
					next = target
					break
				}
			}
			if next == "" {
				break
			}
			finalTarget = next
		}
		if !knownCoreRoute(redirectPathOnly(finalTarget)) {
			var count int64
			if err := s.db.Table("cms_pages").Joins("JOIN cms_entries ON cms_entries.id = cms_pages.entry_id").
				Where("cms_pages.path = ? AND cms_pages.visibility = ? AND cms_entries.published_version_id IS NOT NULL", redirectPathOnly(finalTarget), models.CMSPageVisibilityPublic).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return fmt.Errorf("%w: redirect target is not a published page", ErrInvalidPage)
			}
		}
	}
	return nil
}

func (s *RedirectService) enabledRules() ([]models.CMSRedirectRule, error) {
	var rules []models.CMSRedirectRule
	if err := s.db.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	sortRedirectRules(rules)
	return rules, nil
}

func sortRedirectRules(rules []models.CMSRedirectRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		if len(rules[i].SourcePattern) != len(rules[j].SourcePattern) {
			return len(rules[i].SourcePattern) > len(rules[j].SourcePattern)
		}
		return rules[i].ID < rules[j].ID
	})
}

func normalizeRedirect(input RedirectInput) (models.CMSRedirectRule, error) {
	source, err := normalizeRedirectPath(input.SourcePattern)
	if err != nil {
		return models.CMSRedirectRule{}, err
	}
	if input.MatchType != "exact" && input.MatchType != "prefix" {
		return models.CMSRedirectRule{}, fmt.Errorf("%w: invalid redirect match type", ErrInvalidPage)
	}
	target := strings.TrimSpace(input.TargetURL)
	if strings.HasPrefix(target, "/") {
		target, err = normalizeRedirectTarget(target)
		if err != nil {
			return models.CMSRedirectRule{}, err
		}
	} else {
		parsed, parseErr := url.ParseRequestURI(target)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return models.CMSRedirectRule{}, fmt.Errorf("%w: redirect target must be a storefront path or HTTP URL", ErrInvalidPage)
		}
	}
	if input.RedirectType != 301 && input.RedirectType != 302 {
		return models.CMSRedirectRule{}, fmt.Errorf("%w: invalid redirect type", ErrInvalidPage)
	}
	return models.CMSRedirectRule{SourcePattern: source, MatchType: input.MatchType, TargetURL: target, RedirectType: input.RedirectType, Priority: input.Priority, IsEnabled: input.IsEnabled}, nil
}

func normalizeRedirectTarget(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "", fmt.Errorf("%w: redirect target must be a storefront path", ErrInvalidPage)
	}
	parsed.Path = path.Clean(parsed.Path)
	return parsed.String(), nil
}

func redirectPathOnly(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	return parsed.Path
}

func knownCoreRoute(value string) bool {
	if value == "/" || value == "/search" || value == "/cart" || value == "/checkout" || value == "/login" || value == "/signup" || value == "/profile" || value == "/orders" || value == "/product" {
		return true
	}
	for _, prefix := range []string{"/product/", "/orders/", "/orders/claim"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func normalizeRedirectPath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "", fmt.Errorf("%w: redirect source must be a storefront path", ErrInvalidPage)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: redirect paths cannot contain query strings or fragments", ErrInvalidPage)
	}
	return path.Clean(parsed.Path), nil
}

func redirectTarget(rule models.CMSRedirectRule, requestPath string) (string, bool) {
	switch rule.MatchType {
	case "exact":
		return rule.TargetURL, requestPath == rule.SourcePattern
	case "prefix":
		if requestPath == rule.SourcePattern || strings.HasPrefix(requestPath, strings.TrimSuffix(rule.SourcePattern, "/")+"/") {
			suffix := strings.TrimPrefix(requestPath, rule.SourcePattern)
			if strings.HasPrefix(rule.TargetURL, "/") {
				return strings.TrimSuffix(rule.TargetURL, "/") + "/" + strings.TrimPrefix(suffix, "/"), true
			}
			return rule.TargetURL, true
		}
	}
	return "", false
}
