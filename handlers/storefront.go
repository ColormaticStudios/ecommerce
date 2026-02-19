package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"ecommerce/defaults"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StorefrontLink struct {
	Label string `json:"label"`
	Url   string `json:"url"`
}

type StorefrontHero struct {
	Eyebrow                string         `json:"eyebrow"`
	Title                  string         `json:"title"`
	Subtitle               string         `json:"subtitle"`
	BackgroundImageUrl     string         `json:"background_image_url"`
	BackgroundImageMediaID string         `json:"background_image_media_id,omitempty"`
	PrimaryCta             StorefrontLink `json:"primary_cta"`
	SecondaryCta           StorefrontLink `json:"secondary_cta"`
}

type StorefrontProductSection struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	Source          string `json:"source"`
	Query           string `json:"query"`
	ProductIds      []int  `json:"product_ids"`
	Sort            string `json:"sort"`
	Order           string `json:"order"`
	Limit           int    `json:"limit"`
	ShowStock       bool   `json:"show_stock"`
	ShowDescription bool   `json:"show_description"`
	ImageAspect     string `json:"image_aspect"`
}

type StorefrontPromoCard struct {
	Kicker      string         `json:"kicker"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ImageUrl    string         `json:"image_url"`
	Link        StorefrontLink `json:"link"`
}

type StorefrontHomepageSection struct {
	ID             string                    `json:"id"`
	Type           string                    `json:"type"`
	Enabled        bool                      `json:"enabled"`
	Hero           *StorefrontHero           `json:"hero,omitempty"`
	ProductSection *StorefrontProductSection `json:"product_section,omitempty"`
	PromoCards     []StorefrontPromoCard     `json:"promo_cards,omitempty"`
	PromoCardLimit int                       `json:"promo_card_limit,omitempty"`
	Badges         []string                  `json:"badges,omitempty"`
}

type StorefrontFooterColumn struct {
	Title string           `json:"title"`
	Links []StorefrontLink `json:"links"`
}

type StorefrontFooter struct {
	BrandName    string                   `json:"brand_name"`
	Tagline      string                   `json:"tagline"`
	Copyright    string                   `json:"copyright"`
	Columns      []StorefrontFooterColumn `json:"columns"`
	SocialLinks  []StorefrontLink         `json:"social_links"`
	BottomNotice string                   `json:"bottom_notice"`
}

type StorefrontSettingsPayload struct {
	SiteTitle        string                      `json:"site_title"`
	HomepageSections []StorefrontHomepageSection `json:"homepage_sections"`
	Footer           StorefrontFooter            `json:"footer"`
}

type StorefrontSettingsResponse struct {
	Settings  StorefrontSettingsPayload `json:"settings"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

type UpsertStorefrontSettingsRequest struct {
	Settings StorefrontSettingsPayload `json:"settings" binding:"required"`
}

type storefrontLimits struct {
	MaxHomepageSections     int `json:"max_homepage_sections"`
	MaxManualProductIDs     int `json:"max_manual_product_ids"`
	MaxSectionPromoCards    int `json:"max_section_promo_cards"`
	MaxSectionBadges        int `json:"max_section_badges"`
	MaxFooterColumns        int `json:"max_footer_columns"`
	MaxFooterLinksPerColumn int `json:"max_footer_links_per_column"`
	MaxSocialLinks          int `json:"max_social_links"`
	DefaultProductLimit     int `json:"default_product_section_limit"`
	MaxProductSectionLimit  int `json:"max_product_section_limit"`
}

var (
	storefrontLimitsOnce sync.Once
	storefrontLimitsErr  error
	storefrontLimitsData storefrontLimits

	defaultStorefrontOnce   sync.Once
	defaultStorefront       StorefrontSettingsPayload
	defaultStorefrontErr    error
	defaultSectionPromoOnce sync.Once
	defaultSectionPromo     []StorefrontPromoCard
	defaultSectionBadges    []string
)

func loadStorefrontLimits() {
	storefrontLimitsOnce.Do(func() {
		const fallbackMin = 1
		storefrontLimitsData = storefrontLimits{
			MaxHomepageSections:     16,
			MaxManualProductIDs:     24,
			MaxSectionPromoCards:    12,
			MaxSectionBadges:        20,
			MaxFooterColumns:        4,
			MaxFooterLinksPerColumn: 8,
			MaxSocialLinks:          8,
			DefaultProductLimit:     8,
			MaxProductSectionLimit:  24,
		}

		raw := defaults.StorefrontLimitsJSON()
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var parsed storefrontLimits
		if err := decoder.Decode(&parsed); err != nil {
			storefrontLimitsErr = fmt.Errorf("invalid defaults/storefront-limits.json: %w", err)
			return
		}
		if parsed.MaxHomepageSections < fallbackMin ||
			parsed.MaxManualProductIDs < fallbackMin ||
			parsed.MaxSectionPromoCards < fallbackMin ||
			parsed.MaxSectionBadges < fallbackMin ||
			parsed.MaxFooterColumns < fallbackMin ||
			parsed.MaxFooterLinksPerColumn < fallbackMin ||
			parsed.MaxSocialLinks < fallbackMin ||
			parsed.DefaultProductLimit < fallbackMin ||
			parsed.MaxProductSectionLimit < fallbackMin ||
			parsed.DefaultProductLimit > parsed.MaxProductSectionLimit {
			storefrontLimitsErr = errors.New("defaults/storefront-limits.json contains invalid values")
			return
		}
		storefrontLimitsData = parsed
	})
	if storefrontLimitsErr != nil {
		panic(storefrontLimitsErr)
	}
}

func limits() storefrontLimits {
	loadStorefrontLimits()
	return storefrontLimitsData
}

func defaultProductSection(title, source string) StorefrontProductSection {
	limitValues := limits()
	return StorefrontProductSection{
		Title:           title,
		Subtitle:        "",
		Source:          source,
		Query:           "",
		ProductIds:      []int{},
		Sort:            "created_at",
		Order:           "desc",
		Limit:           limitValues.DefaultProductLimit,
		ShowStock:       true,
		ShowDescription: true,
		ImageAspect:     "square",
	}
}

func assertRequiredKeys(raw map[string]json.RawMessage, required []string, context string) error {
	missing := make([]string, 0)
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			missing = append(missing, fmt.Sprintf("%s.%s", context, key))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required keys: %s", strings.Join(missing, ", "))
	}
	return nil
}

func unmarshalObject(raw json.RawMessage, context string) (map[string]json.RawMessage, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", context, err)
	}
	return value, nil
}

func unmarshalObjectArray(raw json.RawMessage, context string) ([]map[string]json.RawMessage, error) {
	var value []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", context, err)
	}
	return value, nil
}

func decodeDefaultStorefrontSettingsStrict() (StorefrontSettingsPayload, error) {
	rawJSON := defaults.StorefrontJSON()

	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &topLevel); err != nil {
		return StorefrontSettingsPayload{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := assertRequiredKeys(topLevel, []string{"site_title", "homepage_sections", "footer"}, "settings"); err != nil {
		return StorefrontSettingsPayload{}, err
	}

	sectionsRaw, err := unmarshalObjectArray(topLevel["homepage_sections"], "settings.homepage_sections")
	if err != nil {
		return StorefrontSettingsPayload{}, err
	}
	for i, section := range sectionsRaw {
		sectionPath := fmt.Sprintf("settings.homepage_sections[%d]", i)
		if err := assertRequiredKeys(section, []string{"id", "type", "enabled"}, sectionPath); err != nil {
			return StorefrontSettingsPayload{}, err
		}
		var sectionType string
		if err := json.Unmarshal(section["type"], &sectionType); err != nil {
			return StorefrontSettingsPayload{}, fmt.Errorf("invalid %s.type JSON: %w", sectionPath, err)
		}
		switch sectionType {
		case string(apicontract.Hero):
			heroRaw, ok := section["hero"]
			if !ok {
				return StorefrontSettingsPayload{}, fmt.Errorf("missing required keys: %s.hero", sectionPath)
			}
			heroObj, err := unmarshalObject(heroRaw, sectionPath+".hero")
			if err != nil {
				return StorefrontSettingsPayload{}, err
			}
			if err := assertRequiredKeys(heroObj, []string{"eyebrow", "title", "subtitle", "background_image_url", "background_image_media_id", "primary_cta", "secondary_cta"}, sectionPath+".hero"); err != nil {
				return StorefrontSettingsPayload{}, err
			}
			primaryCTA, err := unmarshalObject(heroObj["primary_cta"], sectionPath+".hero.primary_cta")
			if err != nil {
				return StorefrontSettingsPayload{}, err
			}
			if err := assertRequiredKeys(primaryCTA, []string{"label", "url"}, sectionPath+".hero.primary_cta"); err != nil {
				return StorefrontSettingsPayload{}, err
			}
			secondaryCTA, err := unmarshalObject(heroObj["secondary_cta"], sectionPath+".hero.secondary_cta")
			if err != nil {
				return StorefrontSettingsPayload{}, err
			}
			if err := assertRequiredKeys(secondaryCTA, []string{"label", "url"}, sectionPath+".hero.secondary_cta"); err != nil {
				return StorefrontSettingsPayload{}, err
			}
		case string(apicontract.Products):
			if _, ok := section["product_section"]; !ok {
				return StorefrontSettingsPayload{}, fmt.Errorf("missing required keys: %s.product_section", sectionPath)
			}
		case string(apicontract.PromoCards):
			if _, ok := section["promo_cards"]; !ok {
				return StorefrontSettingsPayload{}, fmt.Errorf("missing required keys: %s.promo_cards", sectionPath)
			}
			if _, ok := section["promo_card_limit"]; !ok {
				return StorefrontSettingsPayload{}, fmt.Errorf("missing required keys: %s.promo_card_limit", sectionPath)
			}
		case string(apicontract.Badges):
			if _, ok := section["badges"]; !ok {
				return StorefrontSettingsPayload{}, fmt.Errorf("missing required keys: %s.badges", sectionPath)
			}
		}
	}

	footerRaw, err := unmarshalObject(topLevel["footer"], "settings.footer")
	if err != nil {
		return StorefrontSettingsPayload{}, err
	}
	if err := assertRequiredKeys(footerRaw, []string{"brand_name", "tagline", "copyright", "columns", "social_links", "bottom_notice"}, "settings.footer"); err != nil {
		return StorefrontSettingsPayload{}, err
	}
	footerColumns, err := unmarshalObjectArray(footerRaw["columns"], "settings.footer.columns")
	if err != nil {
		return StorefrontSettingsPayload{}, err
	}
	for i, column := range footerColumns {
		columnPath := fmt.Sprintf("settings.footer.columns[%d]", i)
		if err := assertRequiredKeys(column, []string{"title", "links"}, columnPath); err != nil {
			return StorefrontSettingsPayload{}, err
		}
	}

	var parsed StorefrontSettingsPayload
	decoder := json.NewDecoder(bytes.NewReader(rawJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return StorefrontSettingsPayload{}, fmt.Errorf("shape mismatch: %w", err)
	}
	return parsed, nil
}

func cloneStorefrontSettings(input StorefrontSettingsPayload) StorefrontSettingsPayload {
	payload, err := json.Marshal(input)
	if err != nil {
		panic(fmt.Sprintf("failed to clone storefront defaults: %v", err))
	}
	var cloned StorefrontSettingsPayload
	if err := json.Unmarshal(payload, &cloned); err != nil {
		panic(fmt.Sprintf("failed to decode cloned storefront defaults: %v", err))
	}
	return cloned
}

func defaultStorefrontSettings() StorefrontSettingsPayload {
	defaultStorefrontOnce.Do(func() {
		defaultStorefront, defaultStorefrontErr = decodeDefaultStorefrontSettingsStrict()
	})
	if defaultStorefrontErr != nil {
		panic(fmt.Sprintf("invalid defaults/storefront.json: %v", defaultStorefrontErr))
	}
	return cloneStorefrontSettings(defaultStorefront)
}

func defaultSectionFallbacks() ([]StorefrontPromoCard, []string) {
	defaultSectionPromoOnce.Do(func() {
		defaults := defaultStorefrontSettings()
		for _, section := range defaults.HomepageSections {
			if section.Type == string(apicontract.PromoCards) && len(section.PromoCards) > 0 && len(defaultSectionPromo) == 0 {
				defaultSectionPromo = section.PromoCards
			}
			if section.Type == string(apicontract.Badges) && len(section.Badges) > 0 && len(defaultSectionBadges) == 0 {
				defaultSectionBadges = section.Badges
			}
		}
		if len(defaultSectionPromo) == 0 {
			defaultSectionPromo = []StorefrontPromoCard{{Link: StorefrontLink{}}}
		}
		if len(defaultSectionBadges) == 0 {
			defaultSectionBadges = []string{"Secure checkout"}
		}
	})
	return defaultSectionPromo, defaultSectionBadges
}

func normalizeStorefrontLink(link StorefrontLink) StorefrontLink {
	return StorefrontLink{Label: strings.TrimSpace(link.Label), Url: strings.TrimSpace(link.Url)}
}

func normalizeHomepageSectionType(value string) string {
	switch value {
	case string(apicontract.Hero), string(apicontract.Products), string(apicontract.PromoCards), string(apicontract.Badges):
		return value
	default:
		return string(apicontract.Products)
	}
}

func normalizeProductSource(value string) string {
	switch value {
	case string(apicontract.Manual), string(apicontract.Newest), string(apicontract.Search):
		return value
	default:
		return string(apicontract.Newest)
	}
}

func normalizeSort(value string) string {
	switch value {
	case string(apicontract.StorefrontProductSectionSortPrice),
		string(apicontract.StorefrontProductSectionSortName),
		string(apicontract.StorefrontProductSectionSortCreatedAt):
		return value
	default:
		return string(apicontract.StorefrontProductSectionSortCreatedAt)
	}
}

func normalizeOrder(value string) string {
	switch value {
	case string(apicontract.StorefrontProductSectionOrderAsc), string(apicontract.StorefrontProductSectionOrderDesc):
		return value
	default:
		return string(apicontract.StorefrontProductSectionOrderDesc)
	}
}

func normalizeImageAspect(value string) string {
	switch value {
	case string(apicontract.Square), string(apicontract.Wide):
		return value
	default:
		return string(apicontract.Square)
	}
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizeProductSection(input *StorefrontProductSection) *StorefrontProductSection {
	limitValues := limits()
	if input == nil {
		def := defaultProductSection("Products", string(apicontract.Newest))
		return &def
	}

	normalized := StorefrontProductSection{
		Title:           strings.TrimSpace(input.Title),
		Subtitle:        strings.TrimSpace(input.Subtitle),
		Source:          normalizeProductSource(strings.TrimSpace(input.Source)),
		Query:           strings.TrimSpace(input.Query),
		Sort:            normalizeSort(strings.TrimSpace(input.Sort)),
		Order:           normalizeOrder(strings.TrimSpace(input.Order)),
		Limit:           clamp(input.Limit, 1, limitValues.MaxProductSectionLimit),
		ShowStock:       input.ShowStock,
		ShowDescription: input.ShowDescription,
		ImageAspect:     normalizeImageAspect(strings.TrimSpace(input.ImageAspect)),
		ProductIds:      make([]int, 0, limitValues.MaxManualProductIDs),
	}
	if normalized.Title == "" {
		normalized.Title = "Products"
	}

	seen := make(map[int]struct{})
	for _, id := range input.ProductIds {
		if id <= 0 {
			continue
		}
		if len(normalized.ProductIds) >= limitValues.MaxManualProductIDs {
			break
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized.ProductIds = append(normalized.ProductIds, id)
	}
	return &normalized
}

func normalizeHeroSection(input *StorefrontHero, fallback StorefrontHero) *StorefrontHero {
	if input == nil {
		hero := fallback
		return &hero
	}
	normalized := StorefrontHero{
		Eyebrow:                strings.TrimSpace(input.Eyebrow),
		Title:                  strings.TrimSpace(input.Title),
		Subtitle:               strings.TrimSpace(input.Subtitle),
		BackgroundImageUrl:     strings.TrimSpace(input.BackgroundImageUrl),
		BackgroundImageMediaID: strings.TrimSpace(input.BackgroundImageMediaID),
		PrimaryCta:             normalizeStorefrontLink(input.PrimaryCta),
		SecondaryCta:           normalizeStorefrontLink(input.SecondaryCta),
	}
	if normalized.Eyebrow == "" {
		normalized.Eyebrow = fallback.Eyebrow
	}
	if normalized.Title == "" {
		normalized.Title = fallback.Title
	}
	if normalized.Subtitle == "" {
		normalized.Subtitle = fallback.Subtitle
	}
	return &normalized
}

func normalizePromoCards(cards []StorefrontPromoCard) []StorefrontPromoCard {
	limitValues := limits()
	normalized := make([]StorefrontPromoCard, 0, limitValues.MaxSectionPromoCards)
	for _, card := range cards {
		if len(normalized) >= limitValues.MaxSectionPromoCards {
			break
		}
		normalized = append(normalized, StorefrontPromoCard{
			Kicker:      strings.TrimSpace(card.Kicker),
			Title:       strings.TrimSpace(card.Title),
			Description: strings.TrimSpace(card.Description),
			ImageUrl:    strings.TrimSpace(card.ImageUrl),
			Link:        normalizeStorefrontLink(card.Link),
		})
	}
	if len(normalized) == 0 {
		promoDefaults, _ := defaultSectionFallbacks()
		return append([]StorefrontPromoCard(nil), promoDefaults...)
	}
	return normalized
}

func normalizeBadges(badges []string) []string {
	limitValues := limits()
	normalized := make([]string, 0, limitValues.MaxSectionBadges)
	for _, badge := range badges {
		if len(normalized) >= limitValues.MaxSectionBadges {
			break
		}
		value := strings.TrimSpace(badge)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		_, badgeDefaults := defaultSectionFallbacks()
		return append([]string(nil), badgeDefaults...)
	}
	return normalized
}

func normalizeStorefrontSettings(settings StorefrontSettingsPayload) StorefrontSettingsPayload {
	limitValues := limits()
	defaults := defaultStorefrontSettings()
	normalized := defaults
	defaultHero := StorefrontHero{
		PrimaryCta:   StorefrontLink{},
		SecondaryCta: StorefrontLink{},
	}
	for _, section := range defaults.HomepageSections {
		if section.Type == string(apicontract.Hero) && section.Hero != nil {
			defaultHero = *section.Hero
			break
		}
	}

	normalized.SiteTitle = strings.TrimSpace(settings.SiteTitle)
	if normalized.SiteTitle == "" {
		normalized.SiteTitle = defaults.SiteTitle
	}

	normalized.HomepageSections = make([]StorefrontHomepageSection, 0, limitValues.MaxHomepageSections)
	for i, section := range settings.HomepageSections {
		if len(normalized.HomepageSections) >= limitValues.MaxHomepageSections {
			break
		}
		sectionType := normalizeHomepageSectionType(strings.TrimSpace(section.Type))
		sectionID := strings.TrimSpace(section.ID)
		if sectionID == "" {
			sectionID = fmt.Sprintf("%s-%d", sectionType, i+1)
		}

		next := StorefrontHomepageSection{
			ID:      sectionID,
			Type:    sectionType,
			Enabled: section.Enabled,
		}
		switch sectionType {
		case string(apicontract.Hero):
			next.Hero = normalizeHeroSection(section.Hero, defaultHero)
		case string(apicontract.Products):
			next.ProductSection = normalizeProductSection(section.ProductSection)
		case string(apicontract.PromoCards):
			next.PromoCards = normalizePromoCards(section.PromoCards)
			limit := section.PromoCardLimit
			if limit <= 0 {
				limit = len(next.PromoCards)
			}
			next.PromoCardLimit = clamp(limit, 1, limitValues.MaxSectionPromoCards)
		case string(apicontract.Badges):
			next.Badges = normalizeBadges(section.Badges)
		}
		if sectionType != string(apicontract.Hero) {
			next.Hero = nil
		}
		if sectionType != string(apicontract.Products) {
			next.ProductSection = nil
		}
		if sectionType != string(apicontract.PromoCards) {
			next.PromoCards = nil
			next.PromoCardLimit = 0
		}
		if sectionType != string(apicontract.Badges) {
			next.Badges = nil
		}
		normalized.HomepageSections = append(normalized.HomepageSections, next)
	}
	if len(normalized.HomepageSections) == 0 {
		normalized.HomepageSections = defaults.HomepageSections
	}

	normalized.Footer.BrandName = strings.TrimSpace(settings.Footer.BrandName)
	normalized.Footer.Tagline = strings.TrimSpace(settings.Footer.Tagline)
	normalized.Footer.Copyright = strings.TrimSpace(settings.Footer.Copyright)
	normalized.Footer.BottomNotice = strings.TrimSpace(settings.Footer.BottomNotice)

	normalized.Footer.Columns = make([]StorefrontFooterColumn, 0, limitValues.MaxFooterColumns)
	for _, column := range settings.Footer.Columns {
		if len(normalized.Footer.Columns) >= limitValues.MaxFooterColumns {
			break
		}
		nextColumn := StorefrontFooterColumn{
			Title: strings.TrimSpace(column.Title),
			Links: make([]StorefrontLink, 0, limitValues.MaxFooterLinksPerColumn),
		}
		for _, link := range column.Links {
			if len(nextColumn.Links) >= limitValues.MaxFooterLinksPerColumn {
				break
			}
			normalizedLink := normalizeStorefrontLink(link)
			if normalizedLink.Label == "" && normalizedLink.Url == "" {
				continue
			}
			nextColumn.Links = append(nextColumn.Links, normalizedLink)
		}
		normalized.Footer.Columns = append(normalized.Footer.Columns, nextColumn)
	}
	if len(normalized.Footer.Columns) == 0 {
		normalized.Footer.Columns = defaults.Footer.Columns
	}

	normalized.Footer.SocialLinks = make([]StorefrontLink, 0, limitValues.MaxSocialLinks)
	for _, link := range settings.Footer.SocialLinks {
		if len(normalized.Footer.SocialLinks) >= limitValues.MaxSocialLinks {
			break
		}
		normalizedLink := normalizeStorefrontLink(link)
		if normalizedLink.Label == "" && normalizedLink.Url == "" {
			continue
		}
		normalized.Footer.SocialLinks = append(normalized.Footer.SocialLinks, normalizedLink)
	}

	return normalized
}

func waitForReadyImage(db *gorm.DB, mediaID string) (models.MediaObject, error) {
	var mediaObj models.MediaObject
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := db.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) && time.Now().Before(deadline) {
				time.Sleep(150 * time.Millisecond)
				continue
			}
			return models.MediaObject{}, err
		}
		if mediaObj.Status == media.StatusReady && mediaObj.OriginalPath != "" {
			break
		}
		if mediaObj.Status == media.StatusFailed {
			return models.MediaObject{}, errors.New("media processing failed")
		}
		if time.Now().After(deadline) {
			return models.MediaObject{}, errors.New("media is still processing")
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !strings.HasPrefix(mediaObj.MimeType, "image/") {
		return models.MediaObject{}, errors.New("media must be an image")
	}
	return mediaObj, nil
}

func collectHeroMediaIDs(settings StorefrontSettingsPayload) []string {
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, section := range settings.HomepageSections {
		if section.Type != string(apicontract.Hero) || section.Hero == nil {
			continue
		}
		mediaID := strings.TrimSpace(section.Hero.BackgroundImageMediaID)
		if mediaID == "" {
			continue
		}
		if _, exists := seen[mediaID]; exists {
			continue
		}
		seen[mediaID] = struct{}{}
		ids = append(ids, mediaID)
	}
	return ids
}

func syncStorefrontHeroMedia(db *gorm.DB, mediaService *media.Service, mediaIDs []string) error {
	var existing []models.MediaReference
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHero).
		Order("position asc").Order("id asc").Find(&existing).Error; err != nil {
		return err
	}

	for _, mediaID := range mediaIDs {
		if _, err := waitForReadyImage(db, mediaID); err != nil {
			return err
		}
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHero).
			Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}

		for position, mediaID := range mediaIDs {
			if err := tx.Create(&models.MediaReference{
				MediaID:   mediaID,
				OwnerType: media.OwnerTypeStorefront,
				OwnerID:   models.StorefrontSettingsSingletonID,
				Role:      media.RoleStorefrontHero,
				Position:  position,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if mediaService != nil {
		active := make(map[string]struct{}, len(mediaIDs))
		for _, id := range mediaIDs {
			active[id] = struct{}{}
		}
		for _, ref := range existing {
			if _, keep := active[ref.MediaID]; keep {
				continue
			}
			_ = mediaService.DeleteIfOrphan(ref.MediaID)
		}
	}
	return nil
}

func applyHeroBackgroundURLs(settings *StorefrontSettingsPayload, mediaService *media.Service) {
	if settings == nil || mediaService == nil {
		return
	}

	ids := collectHeroMediaIDs(*settings)
	if len(ids) == 0 {
		for i := range settings.HomepageSections {
			if settings.HomepageSections[i].Type == string(apicontract.Hero) && settings.HomepageSections[i].Hero != nil {
				settings.HomepageSections[i].Hero.BackgroundImageUrl = ""
			}
		}
		return
	}

	var mediaObjs []models.MediaObject
	if err := mediaService.DB.Where("id IN ?", ids).Find(&mediaObjs).Error; err != nil {
		return
	}
	mediaByID := make(map[string]models.MediaObject, len(mediaObjs))
	for _, mediaObj := range mediaObjs {
		mediaByID[mediaObj.ID] = mediaObj
	}

	for i := range settings.HomepageSections {
		section := &settings.HomepageSections[i]
		if section.Type != string(apicontract.Hero) || section.Hero == nil {
			continue
		}
		mediaObj, ok := mediaByID[section.Hero.BackgroundImageMediaID]
		if !ok || mediaObj.Status != media.StatusReady || mediaObj.OriginalPath == "" {
			section.Hero.BackgroundImageUrl = ""
			continue
		}
		section.Hero.BackgroundImageUrl = mediaService.PublicURLFor(mediaObj.OriginalPath)
	}
}

func loadOrCreateStorefrontSettings(db *gorm.DB, mediaService *media.Service) (models.StorefrontSettings, StorefrontSettingsPayload, error) {
	var record models.StorefrontSettings
	err := db.First(&record, models.StorefrontSettingsSingletonID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.StorefrontSettings{}, StorefrontSettingsPayload{}, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		settings := defaultStorefrontSettings()
		payload, marshalErr := json.Marshal(settings)
		if marshalErr != nil {
			return models.StorefrontSettings{}, StorefrontSettingsPayload{}, marshalErr
		}
		record = models.StorefrontSettings{ID: models.StorefrontSettingsSingletonID, ConfigJSON: string(payload)}
		if createErr := db.Create(&record).Error; createErr != nil {
			return models.StorefrontSettings{}, StorefrontSettingsPayload{}, createErr
		}
		return record, settings, nil
	}

	settings := defaultStorefrontSettings()
	if strings.TrimSpace(record.ConfigJSON) != "" {
		var legacy struct {
			Hero *StorefrontHero `json:"hero"`
		}
		_ = json.Unmarshal([]byte(record.ConfigJSON), &legacy)

		if unmarshalErr := json.Unmarshal([]byte(record.ConfigJSON), &settings); unmarshalErr != nil {
			settings = defaultStorefrontSettings()
		}
		if legacy.Hero != nil {
			for i := range settings.HomepageSections {
				if settings.HomepageSections[i].Type == string(apicontract.Hero) && settings.HomepageSections[i].Hero == nil {
					hero := *legacy.Hero
					settings.HomepageSections[i].Hero = &hero
				}
			}
		}
	}
	settings = normalizeStorefrontSettings(settings)

	applyHeroBackgroundURLs(&settings, mediaService)

	return record, settings, nil
}

func filterEnabledHomepageSections(settings StorefrontSettingsPayload) StorefrontSettingsPayload {
	filtered := settings
	filtered.HomepageSections = make([]StorefrontHomepageSection, 0, len(settings.HomepageSections))
	for _, section := range settings.HomepageSections {
		if !section.Enabled {
			continue
		}
		filtered.HomepageSections = append(filtered.HomepageSections, section)
	}
	return filtered
}

func GetStorefrontSettings(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		record, settings, err := loadOrCreateStorefrontSettings(db, mediaService)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
			return
		}
		c.JSON(http.StatusOK, StorefrontSettingsResponse{
			Settings:  filterEnabledHomepageSections(settings),
			UpdatedAt: record.UpdatedAt,
		})
	}
}

func GetAdminStorefrontSettings(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		record, settings, err := loadOrCreateStorefrontSettings(db, mediaService)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
			return
		}
		c.JSON(http.StatusOK, StorefrontSettingsResponse{Settings: settings, UpdatedAt: record.UpdatedAt})
	}
}

func UpsertStorefrontSettings(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpsertStorefrontSettingsRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		normalized := normalizeStorefrontSettings(req.Settings)
		if err := syncStorefrontHeroMedia(db, mediaService, collectHeroMediaIDs(normalized)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		record, _, err := loadOrCreateStorefrontSettings(db, mediaService)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
			return
		}

		payload, marshalErr := json.Marshal(normalized)
		if marshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode storefront settings"})
			return
		}
		record.ConfigJSON = string(payload)
		if err := db.Save(&record).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save storefront settings"})
			return
		}

		_, loaded, err := loadOrCreateStorefrontSettings(db, mediaService)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
			return
		}
		c.JSON(http.StatusOK, StorefrontSettingsResponse{Settings: loaded, UpdatedAt: record.UpdatedAt})
	}
}
