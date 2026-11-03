package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/apicontract"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const merchandisingPath = "/api/v1/admin/search/merchandising-rules"

func TestMerchandisingRegisteredSecurityAndValidation(t *testing.T) {
	router := searchRankingRouter(t)
	emptyHistory := rankingRequest(t, router, "GET", "/api/v1/admin/search/merchandising-audit", "", "admin")
	require.Equal(t, 200, emptyHistory.Code, emptyHistory.Body.String())
	var empty apicontract.SearchMerchandisingAuditListResponse
	require.NoError(t, json.Unmarshal(emptyHistory.Body.Bytes(), &empty))
	require.Empty(t, empty.Data)

	for _, operation := range []struct{ method, path, body string }{
		{"GET", merchandisingPath, ""}, {"POST", merchandisingPath, `{}`}, {"GET", merchandisingPath + "/1", ""}, {"PATCH", merchandisingPath + "/1", `{}`}, {"DELETE", merchandisingPath + "/1", ""}, {"GET", merchandisingPath + "/1/audit", ""}, {"GET", "/api/v1/admin/search/merchandising-audit", ""}, {"POST", "/api/v1/admin/search/preview", `{"filters":{}}`},
	} {
		for _, role := range []string{"", "customer"} {
			status := 401
			if role != "" {
				status = 403
			}
			assertRankingProblem(t, rankingRequest(t, router, operation.method, operation.path, operation.body, role), status)
		}
	}
	request := httptest.NewRequest("POST", merchandisingPath, nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: signedToken(t, "secret", "admin", "admin")})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertRankingProblem(t, response, 403)
	for _, body := range []string{`{"name":`, `{"name":"missing id","rule_type":"hide","action":{"targets":[{}]}}`} {
		assertRankingProblem(t, rankingRequest(t, router, "POST", merchandisingPath, body, "admin"), 400)
	}
	for _, body := range []string{
		`{"name":"bad","rule_type":"hide","action":{"targets":[{"product_id":-1}]}}`,
		`{"name":"bad","rule_type":"unknown","action":{"targets":[{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"pin","action":{"targets":[{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"pin","action":{"targets":[{"product_id":42,"position":0}]}}`,
		`{"name":"bad","rule_type":"hide","action":{"targets":[{"product_id":42,"position":1}]}}`,
		`{"name":"bad","rule_type":"boost","action":{"targets":[{"product_id":42}],"multiplier":1}}`,
		`{"name":"bad","rule_type":"boost","action":{"targets":[{"product_id":42}],"multiplier":101}}`,
		`{"name":"bad","rule_type":"include","action":{"targets":[{"product_id":42}],"multiplier":2}}`,
		`{"name":"bad","rule_type":"hide","action":{"targets":[{"product_id":42},{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"hide","priority":10001,"action":{"targets":[{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"hide","predicate":{"query":{"mode":"bad","value":"bag"}},"action":{"targets":[{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"hide","predicate":{"channel":"cms"},"action":{"targets":[{"product_id":42}]}}`,
		`{"name":"bad","rule_type":"hide","starts_at":"2030-01-02T00:00:00Z","ends_at":"2030-01-01T00:00:00Z","action":{"targets":[{"product_id":42}]}}`,
	} {
		assertRankingProblem(t, rankingRequest(t, router, "POST", merchandisingPath, body, "admin"), 422)
	}
	for _, body := range []string{`{}`, `{"filters":null}`, `{"filters":{"q":`} {
		assertRankingProblem(t, rankingRequest(t, router, "POST", "/api/v1/admin/search/preview", body, "admin"), 400)
	}
	for _, body := range []string{`{"filters":{"sort":"bad"}}`, `{"filters":{"page":0}}`, `{"filters":{"attribute":{"color":[]}}}`} {
		assertRankingProblem(t, rankingRequest(t, router, "POST", "/api/v1/admin/search/preview", body, "admin"), 400)
	}
	assertRankingProblem(t, rankingRequest(t, router, "GET", merchandisingPath+"/0", "", "admin"), 400)
	assertRankingProblem(t, rankingRequest(t, router, "GET", merchandisingPath+"/999", "", "admin"), 404)
}

func TestMerchandisingRegisteredCRUDAndAuditSurvivesDelete(t *testing.T) {
	router := searchRankingRouter(t)
	created := rankingRequest(t, router, "POST", merchandisingPath, `{"name":"Bag campaign","rule_type":"hide","is_active":false,"starts_at":"2030-01-01T00:00:00Z","predicate":{"query":{"mode":"exact","value":"CANVAS!!"}},"action":{"targets":[{"product_id":42}]}}`, "admin")
	require.Equal(t, 201, created.Code, created.Body.String())
	var rule apicontract.SearchMerchandisingRule
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &rule))
	require.False(t, rule.IsActive)
	require.Equal(t, 1, rule.Version)
	require.Equal(t, "Canvas Bag", rule.Action.Targets[0].ProductName)
	require.Equal(t, "canvas", rule.Predicate.Query.Value)
	require.NotNil(t, rule.UpdatedBy)
	require.Equal(t, 17, *rule.UpdatedBy)
	path := fmt.Sprintf("%s/%d", merchandisingPath, rule.Id)
	assertRankingProblem(t, rankingRequest(t, router, "PATCH", path, `{}`, "admin"), 422)
	assertRankingProblem(t, rankingRequest(t, router, "PATCH", path, `{"starts_at":"2030-01-01T00:00:00Z","clear_starts_at":true}`, "admin"), 422)
	updated := rankingRequest(t, router, "PATCH", path, `{"is_active":true,"clear_starts_at":true,"priority":10}`, "admin")
	require.Equal(t, 200, updated.Code, updated.Body.String())
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &rule))
	require.True(t, rule.IsActive)
	require.Nil(t, rule.StartsAt)
	require.Equal(t, 2, rule.Version)
	require.Equal(t, 10, rule.Priority)
	require.Equal(t, 200, rankingRequest(t, router, "GET", path, "", "admin").Code)
	require.Equal(t, 200, rankingRequest(t, router, "GET", merchandisingPath, "", "admin").Code)
	require.Equal(t, 204, rankingRequest(t, router, "DELETE", path, "", "admin").Code)
	assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", "admin"), 404)
	audit := rankingRequest(t, router, "GET", path+"/audit", "", "admin")
	require.Equal(t, 200, audit.Code, audit.Body.String())
	var history apicontract.SearchMerchandisingAuditListResponse
	require.NoError(t, json.Unmarshal(audit.Body.Bytes(), &history))
	require.Len(t, history.Data, 3)
	byOperation := map[string]apicontract.SearchMerchandisingAudit{}
	for _, entry := range history.Data {
		byOperation[string(entry.Operation)] = entry
		require.NotNil(t, entry.ActorId)
		require.Equal(t, 17, *entry.ActorId)
	}
	require.Nil(t, byOperation["create"].Before)
	require.NotNil(t, byOperation["create"].After)
	require.False(t, byOperation["create"].After.IsActive)
	require.False(t, byOperation["update"].Before.IsActive)
	require.True(t, byOperation["update"].After.IsActive)
	require.Equal(t, "Canvas Bag", byOperation["update"].After.Action.Targets[0].ProductName)
	require.NotNil(t, byOperation["delete"].Before)
	require.Nil(t, byOperation["delete"].After)
	allHistory := rankingRequest(t, router, "GET", "/api/v1/admin/search/merchandising-audit", "", "admin")
	require.Equal(t, 200, allHistory.Code, allHistory.Body.String())
	var retained apicontract.SearchMerchandisingAuditListResponse
	require.NoError(t, json.Unmarshal(allHistory.Body.Bytes(), &retained))
	require.Len(t, retained.Data, 3)
	require.Equal(t, "delete", string(retained.Data[0].Operation))
	require.Equal(t, rule.Id, retained.Data[0].RuleId)
	assertRankingProblem(t, rankingRequest(t, router, "GET", merchandisingPath+"/999/audit", "", "admin"), 404)

}

func readSearchPreview(t *testing.T, router *gin.Engine, body string) apicontract.SearchPreviewResponse {
	t.Helper()
	response := rankingRequest(t, router, "POST", "/api/v1/admin/search/preview", body, "admin")
	require.Equal(t, 200, response.Code, response.Body.String())
	var result apicontract.SearchPreviewResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	return result
}
func TestMerchandisingPreviewDoesNotPersistAndPublicDecisionsStayPrivate(t *testing.T) {
	router := searchRankingRouter(t)
	created := rankingRequest(t, router, "POST", merchandisingPath, `{"name":"Hide bag","rule_type":"hide","predicate":{"query":{"mode":"exact","value":"canvas"}},"action":{"targets":[{"product_id":42}]}}`, "admin")
	require.Equal(t, 201, created.Code, created.Body.String())
	preview := readSearchPreview(t, router, `{"filters":{"q":"canvas"}}`)
	require.Len(t, preview.Baseline.Items, 1)
	require.Empty(t, preview.Baseline.RuleDecisions)
	require.Empty(t, preview.Proposed.Items)
	require.NotEmpty(t, preview.Proposed.RuleDecisions)
	require.Equal(t, "Canvas Bag", preview.Proposed.RuleDecisions[0].ProductName)
	override := readSearchPreview(t, router, `{"filters":{"q":"canvas"},"rules":[{"name":"Try pin","rule_type":"pin","action":{"targets":[{"product_id":42,"position":1}]}}]}`)
	require.Len(t, override.Baseline.Items, 1)
	require.Len(t, override.Proposed.Items, 1)
	require.NotEmpty(t, override.Proposed.RuleDecisions)
	require.Equal(t, "applied", string(override.Proposed.RuleDecisions[0].Outcome))
	disabled := readSearchPreview(t, router, `{"filters":{"q":"canvas"},"rules":[]}`)
	require.Len(t, disabled.Proposed.Items, 1)
	require.Empty(t, disabled.Proposed.RuleDecisions)
	public := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas&sort=price", "", "")
	require.Equal(t, 200, public.Code, public.Body.String())
	require.NotContains(t, public.Body.String(), `"rule_decisions"`)
	require.NotContains(t, public.Body.String(), "Hide bag")
	var result apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(public.Body.Bytes(), &result))
	require.Empty(t, result.Items)
	rules := rankingRequest(t, router, "GET", merchandisingPath, "", "admin")
	var list apicontract.SearchMerchandisingRuleListResponse
	require.NoError(t, json.Unmarshal(rules.Body.Bytes(), &list))
	require.Len(t, list.Data, 1)
	require.Equal(t, "Hide bag", list.Data[0].Name)
}

func TestMerchandisingPreviewExplainsBoostConflictsAndScheduleBoundaries(t *testing.T) {
	router := searchRankingRouter(t)
	boosted := readSearchPreview(t, router, `{"filters":{"q":"canvas"},"rules":[{"name":"Winning boost","rule_type":"boost","priority":10,"action":{"targets":[{"product_id":42}],"multiplier":2}},{"name":"Losing boost","rule_type":"boost","priority":5,"action":{"targets":[{"product_id":42}],"multiplier":3}}]}`)
	require.Len(t, boosted.Proposed.Explanations, 1)
	explanation := boosted.Proposed.Explanations[0]
	require.InDelta(t, explanation.Score*2, explanation.AdjustedScore, 0.000001)
	conflicts := 0
	for _, decision := range boosted.Proposed.RuleDecisions {
		if string(decision.Outcome) == "conflict" {
			conflicts++
			require.Equal(t, "Losing boost", decision.RuleName)
		}
	}
	require.Equal(t, 1, conflicts)
	atStart := readSearchPreview(t, router, `{"filters":{"q":"canvas"},"at":"2030-01-01T00:00:00Z","rules":[{"name":"Scheduled hide","rule_type":"hide","starts_at":"2030-01-01T00:00:00Z","ends_at":"2030-01-02T00:00:00Z","action":{"targets":[{"product_id":42}]}}]}`)
	require.Empty(t, atStart.Proposed.Items)
	atEnd := readSearchPreview(t, router, `{"filters":{"q":"canvas"},"at":"2030-01-02T00:00:00Z","rules":[{"name":"Scheduled hide","rule_type":"hide","starts_at":"2030-01-01T00:00:00Z","ends_at":"2030-01-02T00:00:00Z","action":{"targets":[{"product_id":42}]}}]}`)
	require.Len(t, atEnd.Proposed.Items, 1)
	assertRankingProblem(t, rankingRequest(t, router, "POST", "/api/v1/admin/search/preview", `{"filters":{},"rules":[{"id":7,"name":"First","rule_type":"hide","action":{"targets":[{"product_id":42}]}},{"id":7,"name":"Duplicate","rule_type":"hide","action":{"targets":[{"product_id":42}]}}]}`, "admin"), 422)
}
