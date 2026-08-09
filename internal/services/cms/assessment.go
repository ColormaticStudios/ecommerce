package cms

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AssessmentStatus describes whether CMS content is safe to publish.
type AssessmentStatus string

const (
	AssessmentValid    AssessmentStatus = "valid"
	AssessmentDegraded AssessmentStatus = "degraded"
	AssessmentInvalid  AssessmentStatus = "invalid"
)

// IssueSeverity describes whether an issue blocks publication or can be
// remediated by filtering or normalization.
type IssueSeverity string

const (
	IssueSeverityDegraded IssueSeverity = "degraded"
	IssueSeverityInvalid  IssueSeverity = "invalid"
)

// BlockIssue is a machine-readable content problem. Path is an RFC 6901 JSON
// pointer rooted at the payload.
type BlockIssue struct {
	Code     string        `json:"code"`
	Message  string        `json:"message"`
	Path     string        `json:"path"`
	Severity IssueSeverity `json:"severity"`
}

// BlockAssessment contains the complete assessment for one payload block.
type BlockAssessment struct {
	Index       int              `json:"index"`
	Type        string           `json:"type,omitempty"`
	Status      AssessmentStatus `json:"status"`
	Publishable bool             `json:"publishable"`
	Issues      []BlockIssue     `json:"issues"`
}

// PayloadAssessment aggregates block and payload-level content issues.
type PayloadAssessment struct {
	Status AssessmentStatus  `json:"status"`
	Blocks []BlockAssessment `json:"blocks"`
	Issues []BlockIssue      `json:"issues"`
}

func (a PayloadAssessment) CanPublish() bool {
	if a.Status == AssessmentInvalid {
		return false
	}
	for _, block := range a.Blocks {
		if !block.Publishable {
			return false
		}
	}
	return true
}

// AssessmentError exposes all publication-blocking issues while preserving
// errors.Is(err, ErrInvalidPage) compatibility.
type AssessmentError struct {
	Assessment PayloadAssessment
}

func (e *AssessmentError) Error() string {
	if len(e.Assessment.Issues) == 0 {
		return ErrInvalidPage.Error()
	}
	return fmt.Sprintf("%s: %d content issue(s); first issue at %s: %s", ErrInvalidPage, len(e.Assessment.Issues), e.Assessment.Issues[0].Path, e.Assessment.Issues[0].Message)
}

func (e *AssessmentError) Unwrap() error { return ErrInvalidPage }

// AssessAndNormalizePayload deep-copies a payload, normalizes valid blocks,
// preserves invalid raw blocks, and returns an aggregate assessment.
func AssessAndNormalizePayload(payload PagePayload) (PagePayload, PayloadAssessment, error) {
	normalized, err := clonePayload(payload)
	if err != nil {
		assessment := PayloadAssessment{Status: AssessmentInvalid, Blocks: []BlockAssessment{}, Issues: []BlockIssue{{
			Code: "not_json_serializable", Message: "payload must be JSON serializable", Path: "", Severity: IssueSeverityInvalid,
		}}}
		return nil, assessment, fmt.Errorf("%w: payload must be JSON serializable: %v", ErrInvalidPage, err)
	}
	assessment := assessNormalizedPayload(normalized)
	return normalized, assessment, nil
}

// AssessPayload assesses a payload without modifying the caller's value.
func AssessPayload(payload PagePayload) PayloadAssessment {
	_, assessment, _ := AssessAndNormalizePayload(payload)
	return assessment
}

// FilterPublicPayload returns a normalized copy with invalid and unsupported
// blocks removed. Non-block payload fields are retained.
func FilterPublicPayload(payload PagePayload) (PagePayload, PayloadAssessment) {
	normalized, assessment, err := AssessAndNormalizePayload(payload)
	if err != nil || normalized == nil {
		return PagePayload{}, assessment
	}
	blocks, ok := normalized["blocks"].([]any)
	if !ok {
		delete(normalized, "blocks")
		return normalized, assessment
	}
	filtered := make([]any, 0, len(blocks))
	for index, block := range blocks {
		if index < len(assessment.Blocks) && assessment.Blocks[index].Publishable {
			filtered = append(filtered, block)
		}
	}
	normalized["blocks"] = filtered
	return normalized, assessment
}

// FilterPublicBlocks is a convenience wrapper for renderers that only hold a
// block array.
func FilterPublicBlocks(blocks []any) ([]any, []BlockAssessment) {
	filtered, assessment := FilterPublicPayload(PagePayload{"blocks": blocks})
	publicBlocks, _ := filtered["blocks"].([]any)
	return publicBlocks, assessment.Blocks
}

// FilterPublicPayloadJSON removes malformed blocks from persisted payload JSON.
// It is used as a defensive read boundary for historical content that predates
// publication validation.
func FilterPublicPayloadJSON(raw string) (string, PayloadAssessment, error) {
	var payload PagePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", PayloadAssessment{Status: AssessmentInvalid}, fmt.Errorf("%w: payload is not valid JSON", ErrInvalidPage)
	}
	filtered, assessment := FilterPublicPayload(payload)
	normalized, err := json.Marshal(filtered)
	if err != nil {
		return "", assessment, err
	}
	return string(normalized), assessment, nil
}

func clonePayload(payload PagePayload) (PagePayload, error) {
	if payload == nil {
		return PagePayload{}, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var cloned PagePayload
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func assessNormalizedPayload(payload PagePayload) PayloadAssessment {
	assessment := PayloadAssessment{Status: AssessmentValid, Blocks: []BlockAssessment{}, Issues: []BlockIssue{}}
	rawBlocks, exists := payload["blocks"]
	if !exists {
		return assessment
	}
	blocks, ok := rawBlocks.([]any)
	if !ok {
		issue := BlockIssue{Code: "invalid_type", Message: "blocks must be an array", Path: "/blocks", Severity: IssueSeverityInvalid}
		assessment.Status = AssessmentInvalid
		assessment.Issues = append(assessment.Issues, issue)
		return assessment
	}

	for index, rawBlock := range blocks {
		blockAssessment, normalizedBlock := assessBlock(rawBlock, index)
		assessment.Blocks = append(assessment.Blocks, blockAssessment)
		assessment.Issues = append(assessment.Issues, blockAssessment.Issues...)
		blocks[index] = normalizedBlock
		assessment.Status = combineAssessmentStatus(assessment.Status, blockAssessment.Status)
	}
	payload["blocks"] = blocks
	return assessment
}

func assessBlock(rawBlock any, index int) (BlockAssessment, any) {
	result := BlockAssessment{Index: index, Status: AssessmentValid, Publishable: true, Issues: []BlockIssue{}}
	block, ok := rawBlock.(map[string]any)
	if !ok {
		result.Status = AssessmentInvalid
		result.Publishable = false
		result.Issues = append(result.Issues, BlockIssue{
			Code: "invalid_type", Message: "block must be an object", Path: blockPointer(index), Severity: IssueSeverityInvalid,
		})
		return result, rawBlock
	}
	if blockType, ok := block["type"].(string); ok {
		result.Type = strings.TrimSpace(blockType)
	}
	if result.Type == "" {
		result.Status = AssessmentInvalid
		result.Publishable = false
		result.Issues = append(result.Issues, BlockIssue{
			Code: "required", Message: "block type is required", Path: blockPointer(index) + "/type", Severity: IssueSeverityInvalid,
		})
		return result, rawBlock
	}

	candidate, err := cloneBlock(block)
	if err != nil {
		result.Status = AssessmentInvalid
		result.Publishable = false
		result.Issues = append(result.Issues, BlockIssue{
			Code: "not_json_serializable", Message: "block must be JSON serializable", Path: blockPointer(index), Severity: IssueSeverityInvalid,
		})
		return result, rawBlock
	}
	if !supportedBlockType(result.Type) {
		result.Status = AssessmentDegraded
		result.Publishable = false
		result.Issues = append(result.Issues, BlockIssue{
			Code: "unsupported_type", Message: fmt.Sprintf("unsupported block type %q", result.Type), Path: blockPointer(index) + "/type", Severity: IssueSeverityDegraded,
		})
		return result, rawBlock
	}

	before, _ := json.Marshal(candidate)
	temporary := PagePayload{"blocks": []any{candidate}}
	if err := validateAndNormalizePayload(temporary); err != nil {
		result.Status = AssessmentInvalid
		result.Publishable = false
		result.Issues = append(result.Issues, issueFromValidationError(err, index))
		return result, rawBlock
	}
	normalizedBlock := temporary["blocks"].([]any)[0]
	after, _ := json.Marshal(normalizedBlock)
	if string(before) != string(after) && result.Type == "custom_html" {
		result.Status = AssessmentDegraded
		result.Issues = append(result.Issues, BlockIssue{
			Code: "sanitized", Message: "unsafe custom HTML was sanitized", Path: blockPointer(index) + "/html", Severity: IssueSeverityDegraded,
		})
	}
	return result, normalizedBlock
}

func cloneBlock(block map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(block)
	if err != nil {
		return nil, err
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func supportedBlockType(blockType string) bool {
	switch blockType {
	case "hero", "rich_text", "image", "gallery", "video", "faq", "cta", "promo_banner", "product_rail", "category_tiles", "promotion_highlight", "inventory_message", "testimonial", "social_embed", "custom_html", "footer":
		return true
	default:
		return false
	}
}

func issueFromValidationError(err error, index int) BlockIssue {
	message := err.Error()
	message = strings.TrimPrefix(message, ErrInvalidPage.Error()+": ")
	path := blockPointer(index)
	location := "payload.blocks[0]"
	if start := strings.Index(message, location); start >= 0 {
		end := start + len(location)
		for end < len(message) && message[end] != ' ' {
			end++
		}
		token := strings.TrimRight(message[start:end], ":,.;")
		path = validationLocationToPointer(token, index)
		message = strings.TrimSpace(strings.TrimPrefix(message, token))
	}
	code := "invalid_value"
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "required"):
		code = "required"
	case strings.Contains(lower, "unsafe") || strings.Contains(lower, "not allowed"):
		code = "unsafe_value"
	case strings.Contains(lower, "must be an object"):
		code = "invalid_type"
	case strings.Contains(lower, "array"):
		code = "invalid_array"
	case strings.Contains(lower, "between") || strings.Contains(lower, "positive"):
		code = "out_of_range"
	case strings.Contains(lower, "unsupported"):
		code = "unsupported_value"
	}
	if message == "" {
		message = "block is invalid"
	}
	return BlockIssue{Code: code, Message: message, Path: path, Severity: IssueSeverityInvalid}
}

func validationLocationToPointer(location string, index int) string {
	remainder := strings.TrimPrefix(location, "payload.blocks[0]")
	pointer := blockPointer(index)
	for len(remainder) > 0 {
		switch remainder[0] {
		case '.':
			remainder = remainder[1:]
			end := strings.IndexAny(remainder, ".[")
			if end < 0 {
				end = len(remainder)
			}
			pointer += "/" + escapeJSONPointer(remainder[:end])
			remainder = remainder[end:]
		case '[':
			end := strings.IndexByte(remainder, ']')
			if end < 0 {
				return pointer
			}
			pointer += "/" + escapeJSONPointer(remainder[1:end])
			remainder = remainder[end+1:]
		default:
			return pointer
		}
	}
	return pointer
}

func blockPointer(index int) string { return "/blocks/" + strconv.Itoa(index) }

func escapeJSONPointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func combineAssessmentStatus(current, next AssessmentStatus) AssessmentStatus {
	if current == AssessmentInvalid || next == AssessmentInvalid {
		return AssessmentInvalid
	}
	if current == AssessmentDegraded || next == AssessmentDegraded {
		return AssessmentDegraded
	}
	return AssessmentValid
}

func prepareDraftPayload(payload PagePayload) (PagePayload, error) {
	normalized, _, err := AssessAndNormalizePayload(payload)
	return normalized, err
}

func requirePublishablePayload(payload PagePayload) (PagePayload, PayloadAssessment, error) {
	normalized, assessment, err := AssessAndNormalizePayload(payload)
	if err != nil {
		return nil, assessment, err
	}
	if !assessment.CanPublish() {
		return nil, assessment, &AssessmentError{Assessment: assessment}
	}
	return normalized, assessment, nil
}
