package cms

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFooterBlock(t *testing.T) {
	payload := PagePayload{"blocks": []any{map[string]any{
		"type":       "footer",
		"brand_name": "Example Store",
		"columns": []any{map[string]any{
			"title": "Help",
			"links": []any{map[string]any{"label": "Shipping", "url": "/shipping"}},
		}},
		"social_links": []any{map[string]any{"label": "Instagram", "url": "https://instagram.com/example"}},
		"copyright":    "Copyright Example Store",
		"layout":       "columns",
	}}}

	_, err := ValidateAndNormalizePayload(payload)
	require.NoError(t, err)
}

func TestValidateFooterBlockRejectsUnsafeLink(t *testing.T) {
	payload := PagePayload{"blocks": []any{map[string]any{
		"type":       "footer",
		"brand_name": "Example Store",
		"columns": []any{map[string]any{
			"title": "Help",
			"links": []any{map[string]any{"label": "Bad link", "url": "javascript:alert(1)"}},
		}},
		"social_links": []any{},
		"copyright":    "Copyright Example Store",
		"layout":       "columns",
	}}}

	_, err := ValidateAndNormalizePayload(payload)
	require.ErrorIs(t, err, ErrInvalidPage)
	require.ErrorContains(t, err, "url is unsafe")
}
