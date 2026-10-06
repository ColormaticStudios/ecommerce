package search

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelevanceMetricDefinitions(t *testing.T) {
	judgments := map[uint]int{1: 3, 2: 2, 3: 1, 4: 0}
	perfect := measureRelevance([]uint{1, 2, 3}, judgments)
	require.InDelta(t, 1, perfect.NDCG10, 1e-12)
	require.InDelta(t, 0.6, perfect.Precision5, 1e-12, "precision always divides by five, even for a short result set")
	swapped := measureRelevance([]uint{2, 1, 99, 3}, judgments)
	ideal := 7.0 + 3/math.Log2(3) + 1/math.Log2(4)
	actual := 3.0 + 7/math.Log2(3) + 1/math.Log2(5)
	require.InDelta(t, actual/ideal, swapped.NDCG10, 1e-12)
	require.InDelta(t, 0.6, swapped.Precision5, 1e-12)
	duplicate := measureRelevance([]uint{1, 1, 1}, judgments)
	require.InDelta(t, 0.2, duplicate.Precision5, 1e-12)
	require.Equal(t, RelevanceMetrics{}, measureRelevance(nil, judgments))
	require.Equal(t, RelevanceMetrics{}, measureRelevance([]uint{4, 99}, map[uint]int{4: 0}))
}

func TestRelevanceGateTolerance(t *testing.T) {
	baseline := RelevanceMetrics{NDCG10: 0.9, Precision5: 0.7}
	for _, test := range []struct {
		name    string
		current RelevanceMetrics
		fails   bool
	}{
		{"equal", baseline, false},
		{"tolerance", RelevanceMetrics{NDCG10: 0.88, Precision5: 0.68}, false},
		{"ndcg regression", RelevanceMetrics{NDCG10: 0.879, Precision5: 0.7}, true},
		{"precision regression", RelevanceMetrics{NDCG10: 0.9, Precision5: 0.679}, true},
		{"improvement", RelevanceMetrics{NDCG10: 1, Precision5: 0.8}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := (RelevanceReport{RelevanceMetrics: test.current, Baseline: baseline}).CheckRegression()
			if test.fails {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRelevanceCorpusRejectsInvalidJudgments(t *testing.T) {
	var corpus relevanceCorpus
	require.NoError(t, decodeRelevanceFixture("testdata/relevance-corpus.json", &corpus))
	require.NoError(t, validateRelevanceCorpus(corpus))
	corpus.Queries[0].Judgments[999] = 3
	require.Error(t, validateRelevanceCorpus(corpus))
	delete(corpus.Queries[0].Judgments, 999)
	corpus.Queries[0].Judgments[1] = 4
	require.Error(t, validateRelevanceCorpus(corpus))
	corpus.Queries[0].Judgments[1] = 3
	corpus.Products[0].AgeDays = -1
	require.Error(t, validateRelevanceCorpus(corpus))
	require.Error(t, validateRelevanceCorpus(relevanceCorpus{Now: time.Now()}))
}

func TestRelevanceQualityGate(t *testing.T) {
	report, err := EvaluateRelevance(context.Background())
	require.NoError(t, err)
	require.NoError(t, report.CheckRegression(), "query report: %+v", report.Queries)
	require.Len(t, report.Queries, 13)
	repeat, err := EvaluateRelevance(context.Background())
	require.NoError(t, err)
	require.Equal(t, report, repeat, "the clock, seeds, and ordering must be deterministic")
}
