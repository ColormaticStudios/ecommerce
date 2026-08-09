package cms

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssessAndNormalizePayloadPreservesInvalidBlocks(t *testing.T) {
	payload := PagePayload{"blocks": []any{
		map[string]any{"type": "rich_text", "body": "Valid"},
		map[string]any{"type": "hero"},
		map[string]any{"type": "future_block", "value": "preserve me"},
	}}

	normalized, assessment, err := AssessAndNormalizePayload(payload)
	require.NoError(t, err)
	require.Len(t, normalized["blocks"], 3)
	require.Len(t, assessment.Blocks, 3)
	require.Equal(t, AssessmentValid, assessment.Blocks[0].Status)
	require.Equal(t, AssessmentInvalid, assessment.Blocks[1].Status)
	require.Equal(t, AssessmentDegraded, assessment.Blocks[2].Status)
	require.False(t, assessment.CanPublish())
	require.Equal(t, "/blocks/1/title", assessment.Blocks[1].Issues[0].Path)
}

func TestFilterPublicPayloadKeepsOnlyPublishableBlocks(t *testing.T) {
	filtered, assessment := FilterPublicPayload(PagePayload{"blocks": []any{
		map[string]any{"type": "rich_text", "body": "Valid"},
		map[string]any{"type": "hero"},
		map[string]any{"type": "future_block", "value": "hidden"},
	}})

	require.False(t, assessment.CanPublish())
	blocks, ok := filtered["blocks"].([]any)
	require.True(t, ok)
	require.Len(t, blocks, 1)
	require.Equal(t, "rich_text", blocks[0].(map[string]any)["type"])
}

func TestFilterPublicPayloadJSONSanitizesAndFilters(t *testing.T) {
	raw, err := json.Marshal(PagePayload{"blocks": []any{
		map[string]any{"type": "custom_html", "html": `<p onclick="bad()">Safe</p><script>bad()</script>`},
		map[string]any{"type": "image"},
	}})
	require.NoError(t, err)

	filtered, assessment, err := FilterPublicPayloadJSON(string(raw))
	require.NoError(t, err)
	require.False(t, assessment.CanPublish())
	require.NotContains(t, filtered, "script")
	require.NotContains(t, filtered, "onclick")

	var payload PagePayload
	require.NoError(t, json.Unmarshal([]byte(filtered), &payload))
	require.Len(t, payload["blocks"], 1)
}

func TestAssessmentErrorUnwrapsInvalidPage(t *testing.T) {
	_, assessment, err := requirePublishablePayload(PagePayload{"blocks": []any{
		map[string]any{"type": "hero"},
	}})
	require.ErrorIs(t, err, ErrInvalidPage)
	require.False(t, assessment.CanPublish())
}
