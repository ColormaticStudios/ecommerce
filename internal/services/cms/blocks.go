package cms

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	scriptTagPattern      = regexp.MustCompile(`(?is)<\s*script[^>]*>.*?<\s*/\s*script\s*>`)
	eventAttributePattern = regexp.MustCompile(`(?is)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	jsURLPattern          = regexp.MustCompile(`(?is)javascript\s*:`)
)

func validateAndNormalizePayload(payload PagePayload) error {
	rawBlocks, ok := payload["blocks"]
	if !ok {
		return nil
	}
	blocks, ok := rawBlocks.([]any)
	if !ok {
		return fmt.Errorf("%w: payload.blocks must be an array", ErrInvalidPage)
	}
	normalized := make([]any, 0, len(blocks))
	for index, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: payload.blocks[%d] must be an object", ErrInvalidPage, index)
		}
		blockType, ok := stringField(block, "type")
		if !ok {
			return fmt.Errorf("%w: payload.blocks[%d].type is required", ErrInvalidPage, index)
		}
		switch blockType {
		case "hero":
			if err := validateHeroBlock(block, index); err != nil {
				return err
			}
		case "rich_text":
			if _, ok := stringField(block, "body"); !ok {
				return fmt.Errorf("%w: payload.blocks[%d].body is required", ErrInvalidPage, index)
			}
		case "image":
			if _, ok := stringField(block, "media_id"); !ok {
				return fmt.Errorf("%w: payload.blocks[%d].media_id is required", ErrInvalidPage, index)
			}
		case "gallery":
			images, ok := block["images"].([]any)
			if !ok || len(images) == 0 {
				return fmt.Errorf("%w: payload.blocks[%d].images must be a non-empty array", ErrInvalidPage, index)
			}
			for imageIndex, rawImage := range images {
				image, ok := rawImage.(map[string]any)
				if !ok {
					return fmt.Errorf("%w: payload.blocks[%d].images[%d] must be an object", ErrInvalidPage, index, imageIndex)
				}
				if _, ok := stringField(image, "media_id"); !ok {
					return fmt.Errorf("%w: payload.blocks[%d].images[%d].media_id is required", ErrInvalidPage, index, imageIndex)
				}
			}
		case "video":
			url, ok := stringField(block, "url")
			if !ok {
				return fmt.Errorf("%w: payload.blocks[%d].url is required", ErrInvalidPage, index)
			}
			if !isSafeURL(url) {
				return fmt.Errorf("%w: payload.blocks[%d].url is unsafe", ErrInvalidPage, index)
			}
		case "faq":
			items, ok := block["items"].([]any)
			if !ok || len(items) == 0 {
				return fmt.Errorf("%w: payload.blocks[%d].items must be a non-empty array", ErrInvalidPage, index)
			}
			for itemIndex, rawItem := range items {
				item, ok := rawItem.(map[string]any)
				if !ok {
					return fmt.Errorf("%w: payload.blocks[%d].items[%d] must be an object", ErrInvalidPage, index, itemIndex)
				}
				if _, ok := stringField(item, "question"); !ok {
					return fmt.Errorf("%w: payload.blocks[%d].items[%d].question is required", ErrInvalidPage, index, itemIndex)
				}
				if _, ok := stringField(item, "answer"); !ok {
					return fmt.Errorf("%w: payload.blocks[%d].items[%d].answer is required", ErrInvalidPage, index, itemIndex)
				}
			}
		case "cta":
			if _, ok := stringField(block, "label"); !ok {
				return fmt.Errorf("%w: payload.blocks[%d].label is required", ErrInvalidPage, index)
			}
			url, ok := stringField(block, "url")
			if !ok {
				return fmt.Errorf("%w: payload.blocks[%d].url is required", ErrInvalidPage, index)
			}
			if !isSafeURL(url) {
				return fmt.Errorf("%w: payload.blocks[%d].url is unsafe", ErrInvalidPage, index)
			}
		case "promo_banner":
			if _, ok := stringField(block, "title"); !ok {
				return fmt.Errorf("%w: payload.blocks[%d].title is required", ErrInvalidPage, index)
			}
			if link, ok := block["link"]; ok {
				if err := validateLinkObject(link, fmt.Sprintf("payload.blocks[%d].link", index)); err != nil {
					return err
				}
			}
		case "product_rail":
			if err := validateProductRailBlock(block, index); err != nil {
				return err
			}
		case "category_tiles":
			if err := validateCategoryTilesBlock(block, index); err != nil {
				return err
			}
		case "promotion_highlight":
			if err := validatePromotionHighlightBlock(block, index); err != nil {
				return err
			}
		case "inventory_message":
			if err := validateInventoryMessageBlock(block, index); err != nil {
				return err
			}
		case "testimonial":
			if err := validateTestimonialBlock(block, index); err != nil {
				return err
			}
		case "social_embed":
			if err := validateSocialEmbedBlock(block, index); err != nil {
				return err
			}
		case "custom_html":
			body, ok := stringField(block, "html")
			if !ok {
				return fmt.Errorf("%w: payload.blocks[%d].html is required", ErrInvalidPage, index)
			}
			block["html"] = sanitizeCustomHTML(body)
		case "footer":
			if err := validateFooterBlock(block, index); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported block type %q", ErrInvalidPage, blockType)
		}
		normalized = append(normalized, block)
	}
	payload["blocks"] = normalized
	return nil
}

func validateFooterBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "brand_name"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].brand_name is required", ErrInvalidPage, index)
	}
	columns, ok := block["columns"].([]any)
	if !ok || len(columns) > 6 {
		return fmt.Errorf("%w: payload.blocks[%d].columns is invalid", ErrInvalidPage, index)
	}
	for columnIndex, rawColumn := range columns {
		column, ok := rawColumn.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: payload.blocks[%d].columns[%d] must be an object", ErrInvalidPage, index, columnIndex)
		}
		if _, ok := stringField(column, "title"); !ok {
			return fmt.Errorf("%w: payload.blocks[%d].columns[%d].title is required", ErrInvalidPage, index, columnIndex)
		}
		if err := validateFooterLinks(column["links"], fmt.Sprintf("payload.blocks[%d].columns[%d].links", index, columnIndex), 10); err != nil {
			return err
		}
	}
	if err := validateFooterLinks(block["social_links"], fmt.Sprintf("payload.blocks[%d].social_links", index), 8); err != nil {
		return err
	}
	layout, ok := stringField(block, "layout")
	if !ok || (layout != "columns" && layout != "centered" && layout != "minimal") {
		return fmt.Errorf("%w: payload.blocks[%d].layout is unsupported", ErrInvalidPage, index)
	}
	delete(block, "theme")
	return nil
}

func validateFooterLinks(raw any, location string, max int) error {
	links, ok := raw.([]any)
	if !ok || len(links) > max {
		return fmt.Errorf("%w: %s is invalid", ErrInvalidPage, location)
	}
	for linkIndex, rawLink := range links {
		if err := validateLinkObject(rawLink, fmt.Sprintf("%s[%d]", location, linkIndex)); err != nil {
			return err
		}
	}
	return nil
}

func ValidateAndNormalizePayload(payload PagePayload) (PagePayload, error) {
	normalized := make(PagePayload, len(payload))
	for key, value := range payload {
		normalized[key] = value
	}
	if err := validateAndNormalizePayload(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func validateHeroBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "title"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].title is required", ErrInvalidPage, index)
	}
	if cta, ok := block["primary_cta"]; ok {
		if err := validateLinkObject(cta, fmt.Sprintf("payload.blocks[%d].primary_cta", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateProductRailBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "title"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].title is required", ErrInvalidPage, index)
	}
	source, ok := stringField(block, "source")
	if !ok {
		return fmt.Errorf("%w: payload.blocks[%d].source is required", ErrInvalidPage, index)
	}
	switch source {
	case "manual", "newest", "search", "category":
	default:
		return fmt.Errorf("%w: payload.blocks[%d].source is unsupported", ErrInvalidPage, index)
	}
	if source == "category" {
		if _, ok := stringField(block, "category_slug"); !ok {
			return fmt.Errorf("%w: payload.blocks[%d].category_slug is required", ErrInvalidPage, index)
		}
	}
	limit := numericField(block, "limit", 8)
	if limit < 1 || limit > 24 {
		return fmt.Errorf("%w: payload.blocks[%d].limit must be between 1 and 24", ErrInvalidPage, index)
	}
	block["limit"] = limit
	sort, _ := stringField(block, "sort")
	if sort == "" {
		block["sort"] = "created_at"
	} else if sort != "created_at" && sort != "price" && sort != "name" {
		return fmt.Errorf("%w: payload.blocks[%d].sort is unsupported", ErrInvalidPage, index)
	}
	order, _ := stringField(block, "order")
	if order == "" {
		block["order"] = "desc"
	} else if order != "asc" && order != "desc" {
		return fmt.Errorf("%w: payload.blocks[%d].order is unsupported", ErrInvalidPage, index)
	}
	return nil
}

func validateCategoryTilesBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "title"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].title is required", ErrInvalidPage, index)
	}
	slugs, ok := stringSliceField(block, "category_slugs")
	if !ok {
		return fmt.Errorf("%w: payload.blocks[%d].category_slugs must be a non-empty array", ErrInvalidPage, index)
	}
	block["category_slugs"] = slugs
	if rawImages, ok := block["category_media_ids"]; ok {
		images, ok := rawImages.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: payload.blocks[%d].category_media_ids must be an object", ErrInvalidPage, index)
		}
		for slug, rawID := range images {
			mediaID, ok := rawID.(string)
			if !ok || strings.TrimSpace(mediaID) == "" {
				return fmt.Errorf("%w: payload.blocks[%d].category_media_ids[%s] is invalid", ErrInvalidPage, index, slug)
			}
		}
	}
	if aspect, ok := stringField(block, "image_aspect"); ok && aspect != "square" && aspect != "wide" {
		return fmt.Errorf("%w: payload.blocks[%d].image_aspect is unsupported", ErrInvalidPage, index)
	}
	return nil
}

func validatePromotionHighlightBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "title"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].title is required", ErrInvalidPage, index)
	}
	if campaignID := numericField(block, "campaign_id", 0); campaignID < 0 {
		return fmt.Errorf("%w: payload.blocks[%d].campaign_id must be positive", ErrInvalidPage, index)
	} else if campaignID > 0 {
		block["campaign_id"] = campaignID
	}
	if link, ok := block["link"]; ok {
		if err := validateLinkObject(link, fmt.Sprintf("payload.blocks[%d].link", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateInventoryMessageBlock(block map[string]any, index int) error {
	productID := numericField(block, "product_id", 0)
	if productID <= 0 {
		return fmt.Errorf("%w: payload.blocks[%d].product_id is required", ErrInvalidPage, index)
	}
	block["product_id"] = productID
	threshold := numericField(block, "low_stock_threshold", 5)
	if threshold < 1 || threshold > 1000 {
		return fmt.Errorf("%w: payload.blocks[%d].low_stock_threshold must be between 1 and 1000", ErrInvalidPage, index)
	}
	block["low_stock_threshold"] = threshold
	return nil
}

func validateTestimonialBlock(block map[string]any, index int) error {
	if _, ok := stringField(block, "quote"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].quote is required", ErrInvalidPage, index)
	}
	if _, ok := stringField(block, "attribution"); !ok {
		return fmt.Errorf("%w: payload.blocks[%d].attribution is required", ErrInvalidPage, index)
	}
	if rating := numericField(block, "rating", 0); rating != 0 {
		if rating < 1 || rating > 5 {
			return fmt.Errorf("%w: payload.blocks[%d].rating must be between 1 and 5", ErrInvalidPage, index)
		}
		block["rating"] = rating
	}
	return nil
}

func validateSocialEmbedBlock(block map[string]any, index int) error {
	provider, ok := stringField(block, "provider")
	if !ok {
		return fmt.Errorf("%w: payload.blocks[%d].provider is required", ErrInvalidPage, index)
	}
	embedURL, ok := stringField(block, "url")
	if !ok {
		return fmt.Errorf("%w: payload.blocks[%d].url is required", ErrInvalidPage, index)
	}
	if !isAllowedSocialURL(provider, embedURL) {
		return fmt.Errorf("%w: payload.blocks[%d].url is not allowed for provider", ErrInvalidPage, index)
	}
	return nil
}

func validateLinkObject(value any, location string) error {
	link, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: %s must be an object", ErrInvalidPage, location)
	}
	if _, ok := stringField(link, "label"); !ok {
		return fmt.Errorf("%w: %s.label is required", ErrInvalidPage, location)
	}
	if _, ok := stringField(link, "url"); !ok {
		return fmt.Errorf("%w: %s.url is required", ErrInvalidPage, location)
	}
	if url, _ := stringField(link, "url"); !isSafeURL(url) {
		return fmt.Errorf("%w: %s.url is unsafe", ErrInvalidPage, location)
	}
	return nil
}

func stringSliceField(record map[string]any, key string) ([]string, bool) {
	values, ok := record[key].([]any)
	if !ok || len(values) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func stringField(record map[string]any, key string) (string, bool) {
	value, ok := record[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}

func numericField(record map[string]any, key string, fallback int) int {
	switch value := record[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	default:
		return fallback
	}
}

func sanitizeCustomHTML(value string) string {
	withoutScripts := scriptTagPattern.ReplaceAllString(value, "")
	withoutEvents := eventAttributePattern.ReplaceAllString(withoutScripts, "")
	withoutJSURLs := jsURLPattern.ReplaceAllString(withoutEvents, "")
	if strings.Contains(strings.ToLower(withoutJSURLs), "<iframe") {
		return html.EscapeString(withoutJSURLs)
	}
	return withoutJSURLs
}

func isSafeURL(value string) bool {
	normalized := strings.TrimSpace(strings.ToLower(value))
	return normalized == "" ||
		strings.HasPrefix(normalized, "/") ||
		strings.HasPrefix(normalized, "#") ||
		strings.HasPrefix(normalized, "https://") ||
		strings.HasPrefix(normalized, "http://") ||
		strings.HasPrefix(normalized, "mailto:")
}

func isAllowedSocialURL(provider string, value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	switch provider {
	case "instagram":
		return host == "instagram.com"
	case "tiktok":
		return host == "tiktok.com"
	case "youtube":
		return host == "youtube.com" || host == "youtu.be"
	default:
		return false
	}
}
