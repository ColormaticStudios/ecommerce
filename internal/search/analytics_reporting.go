package search

import (
	"context"
	"fmt"
	"time"
)

// Analytics retains the CLI/service default while the per-query list is bounded.
func (s *Service) Analytics(ctx context.Context, days int) (Analytics, error) {
	return s.AnalyticsPage(ctx, days, 1, 20)
}

// analyticsCTE uses portable window functions for exact nearest-rank percentiles.
// Every join aggregates before joining impressions, avoiding event fanout.
const analyticsCTE = `WITH population AS (
 SELECT * FROM search_query_events q WHERE created_at >= ? AND session_hash <> '' AND impression_id <> ''
 AND NOT EXISTS (SELECT 1 FROM search_revoked_sessions r WHERE r.session_hash=q.session_hash)
), clicks AS (
 SELECT e.impression_id, COUNT(*) clicks, MAX(CASE WHEN e.position<=5 THEN 1 ELSE 0 END) clicked5,
 MAX(CASE WHEN e.position<=10 THEN 1 ELSE 0 END) clicked10
 FROM search_click_events e JOIN population q ON q.impression_id=e.impression_id AND q.id=e.query_event_id AND q.session_hash=e.session_hash
 WHERE e.clicked_at>=? GROUP BY e.impression_id
), carts AS (
 SELECT e.impression_id,COUNT(*) carts FROM search_cart_attributions e JOIN population q ON q.impression_id=e.impression_id
 WHERE e.created_at>=? GROUP BY e.impression_id
 ), paid_events AS (
  SELECT a.impression_id,a.order_id,q.normalized_query,e.position
 FROM search_order_attributions a JOIN population q ON q.impression_id=a.impression_id
 JOIN search_click_events e ON e.event_id=a.click_id AND e.impression_id=q.impression_id AND e.query_event_id=q.id AND e.session_hash=q.session_hash AND e.product_id=a.product_id
  WHERE a.paid_at>=?
 ), paid AS (
  SELECT impression_id,COUNT(DISTINCT order_id) paid,MAX(CASE WHEN position<=5 THEN 1 ELSE 0 END) paid5,
  MAX(CASE WHEN position<=10 THEN 1 ELSE 0 END) paid10 FROM paid_events GROUP BY impression_id
), enriched AS (
 SELECT q.*,COALESCE(c.clicks,0) clicks,COALESCE(c.clicked5,0) clicked5,COALESCE(c.clicked10,0) clicked10,
 COALESCE(a.carts,0) carts,COALESCE(p.paid,0) paid,COALESCE(p.paid5,0) paid5,COALESCE(p.paid10,0) paid10
 FROM population q LEFT JOIN clicks c ON c.impression_id=q.impression_id LEFT JOIN carts a ON a.impression_id=q.impression_id LEFT JOIN paid p ON p.impression_id=q.impression_id
), ranked AS (
 SELECT *,ROW_NUMBER() OVER (PARTITION BY normalized_query ORDER BY latency_ms,id) rn,
 COUNT(*) OVER (PARTITION BY normalized_query) cnt,
 ROW_NUMBER() OVER (ORDER BY latency_ms,id) global_rn,COUNT(*) OVER () global_cnt FROM enriched
) `

type analyticsAggregate struct {
	Query                                                                         string
	Searches, UniqueSessions, ZeroResults, Clicks, AddToCarts, PaidOrders         int64
	Clicked, Converted, Clicked5, Clicked10, Paid5, Paid10, Eligible5, Eligible10 int64
	AverageLatencyMs, P95LatencyMs, P99LatencyMs                                  float64
}

func (a analyticsAggregate) metrics() QueryMetrics {
	m := QueryMetrics{Query: a.Query, Searches: a.Searches, UniqueSessions: a.UniqueSessions, ZeroResults: a.ZeroResults, Clicks: a.Clicks, AddToCarts: a.AddToCarts, PaidOrders: a.PaidOrders, AverageLatencyMs: a.AverageLatencyMs, P95LatencyMs: a.P95LatencyMs, P99LatencyMs: a.P99LatencyMs}
	if a.Searches > 0 {
		m.ZeroResultRate = float64(a.ZeroResults) / float64(a.Searches)
		m.CTR = float64(a.Clicked) / float64(a.Searches)
		m.ConversionRate = float64(a.Converted) / float64(a.Searches)
	}
	if a.Eligible5 > 0 {
		m.CTRAt5 = float64(a.Clicked5) / float64(a.Eligible5)
		m.ConversionAt5 = float64(a.Paid5) / float64(a.Eligible5)
	}
	if a.Eligible10 > 0 {
		m.CTRAt10 = float64(a.Clicked10) / float64(a.Eligible10)
		m.ConversionAt10 = float64(a.Paid10) / float64(a.Eligible10)
	}
	return m
}

func aggregateSQL(global bool) string {
	rn, cnt := "rn", "cnt"
	query := "normalized_query"
	group := " GROUP BY normalized_query"
	paidCount := "(SELECT COUNT(DISTINCT order_id) FROM paid_events pe WHERE pe.normalized_query=ranked.normalized_query)"
	if global {
		rn, cnt = "global_rn", "global_cnt"
		query = "''"
		group = ""
		paidCount = "(SELECT COUNT(DISTINCT order_id) FROM paid_events)"
	}
	return fmt.Sprintf(`SELECT %s AS query,COUNT(*) searches,COUNT(DISTINCT session_hash) unique_sessions,
 COALESCE(SUM(CASE WHEN result_count=0 THEN 1 ELSE 0 END),0) zero_results,
  COALESCE(SUM(clicks),0) clicks,COALESCE(SUM(carts),0) add_to_carts,%s paid_orders,
 COALESCE(SUM(CASE WHEN clicks>0 THEN 1 ELSE 0 END),0) clicked,COALESCE(SUM(CASE WHEN paid>0 THEN 1 ELSE 0 END),0) converted,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=5 THEN 1 ELSE 0 END),0) eligible5,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=10 THEN 1 ELSE 0 END),0) eligible10,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=5 THEN clicked5 ELSE 0 END),0) clicked5,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=10 THEN clicked10 ELSE 0 END),0) clicked10,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=5 THEN paid5 ELSE 0 END),0) paid5,
 COALESCE(SUM(CASE WHEN product_count>0 AND first_position<=10 THEN paid10 ELSE 0 END),0) paid10,
 COALESCE(AVG(latency_ms),0) average_latency_ms,
 COALESCE(MAX(CASE WHEN %s=(%s*95+99)/100 THEN latency_ms END),0) p95_latency_ms,
 COALESCE(MAX(CASE WHEN %s=(%s*99+99)/100 THEN latency_ms END),0) p99_latency_ms
  FROM ranked%s`, query, paidCount, rn, cnt, rn, cnt, group)
}

func (s *Service) AnalyticsPage(ctx context.Context, days, page, limit int) (Analytics, error) {
	result := Analytics{Days: days, Page: page, Limit: limit, Queries: []QueryMetrics{}, Popular: []string{}, Trending: []string{}, FacetUsage: map[string]int{}}
	if days < 1 || days > 90 || page < 1 || page > 1000000 || limit < 1 || limit > 100 {
		return result, ErrAnalyticsInvalid
	}
	since := s.now().Add(-time.Duration(days) * 24 * time.Hour)
	args := []any{since, since, since, since}
	var total analyticsAggregate
	if err := s.db.WithContext(ctx).Raw(analyticsCTE+aggregateSQL(true), args...).Scan(&total).Error; err != nil {
		return result, err
	}
	result.QueryMetrics = total.metrics()
	var rows []analyticsAggregate
	pageArgs := append(append([]any{}, args...), limit, (page-1)*limit)
	if err := s.db.WithContext(ctx).Raw(analyticsCTE+aggregateSQL(false)+" ORDER BY searches DESC,query ASC LIMIT ? OFFSET ?", pageArgs...).Scan(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Queries = append(result.Queries, row.metrics())
	}
	if err := s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM (SELECT normalized_query FROM search_query_events q WHERE created_at>=? AND session_hash<>'' AND impression_id<>'' AND NOT EXISTS (SELECT 1 FROM search_revoked_sessions r WHERE r.session_hash=q.session_hash) GROUP BY normalized_query) groups`, since).Scan(&result.TotalQueries).Error; err != nil {
		return result, err
	}
	var facets struct{ Brand, Category, Stock, Price, Attribute int }
	if err := s.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(CASE WHEN (facet_mask & 1)<>0 THEN 1 ELSE 0 END),0) brand,COALESCE(SUM(CASE WHEN (facet_mask & 2)<>0 THEN 1 ELSE 0 END),0) category,COALESCE(SUM(CASE WHEN (facet_mask & 4)<>0 THEN 1 ELSE 0 END),0) stock,COALESCE(SUM(CASE WHEN (facet_mask & 8)<>0 THEN 1 ELSE 0 END),0) price,COALESCE(SUM(CASE WHEN (facet_mask & 16)<>0 THEN 1 ELSE 0 END),0) attribute FROM search_query_events q WHERE created_at>=? AND session_hash<>'' AND impression_id<>'' AND NOT EXISTS (SELECT 1 FROM search_revoked_sessions r WHERE r.session_hash=q.session_hash)`, since).Scan(&facets).Error; err != nil {
		return result, err
	}
	for name, count := range map[string]int{"brand": facets.Brand, "category": facets.Category, "stock": facets.Stock, "price": facets.Price, "attribute": facets.Attribute} {
		if count > 0 {
			result.FacetUsage[name] = count
		}
	}
	var err error
	result.Popular, result.Trending, err = s.PopularQueries(ctx, "", 20)
	return result, err
}

// PopularQueries is administrative diagnostics only. Public suggestions never call it.
func (s *Service) PopularQueries(ctx context.Context, prefix string, limit int) ([]string, []string, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	now := s.now()
	boundary := now.Add(-7 * 24 * time.Hour)
	base := `WITH counts AS (SELECT normalized_query query,
 SUM(CASE WHEN created_at>=? THEN 1 ELSE 0 END) recent,
 SUM(CASE WHEN created_at<? THEN 1 ELSE 0 END) previous
 FROM search_query_events q WHERE created_at>=? AND normalized_query<>'' AND session_hash<>'' AND impression_id<>''
 AND NOT EXISTS (SELECT 1 FROM search_revoked_sessions r WHERE r.session_hash=q.session_hash)
 AND normalized_query LIKE ? ESCAPE '\' GROUP BY normalized_query
 HAVING COUNT(DISTINCT CASE WHEN created_at>=? THEN session_hash END)>=5) `
	args := []any{boundary, boundary, now.Add(-14 * 24 * time.Hour), escapeLike(NormalizeQuery(prefix)) + "%", boundary, limit}
	popular, trending := []string{}, []string{}
	if err := s.db.WithContext(ctx).Raw(base+`SELECT query FROM counts ORDER BY recent DESC,query ASC LIMIT ?`, args...).Scan(&popular).Error; err != nil {
		return nil, nil, err
	}
	if err := s.db.WithContext(ctx).Raw(base+`SELECT query FROM counts WHERE recent>previous ORDER BY (recent-previous) DESC,query ASC LIMIT ?`, args...).Scan(&trending).Error; err != nil {
		return nil, nil, err
	}
	return popular, trending, nil
}
