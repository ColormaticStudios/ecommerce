package categories

import (
	"regexp"
	"strings"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)
var repeatedHyphens = regexp.MustCompile(`-+`)

// NormalizeSlug converts arbitrary labels to URL-safe category slugs.
func NormalizeSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = nonSlugChars.ReplaceAllString(slug, "")
	slug = repeatedHyphens.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

func IsValidName(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed) >= 2 && len(trimmed) <= 120
}
