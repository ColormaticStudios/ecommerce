package search

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"ecommerce/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAnalyticsInvalid = errors.New("invalid search analytics event")

const AnalyticsRetention = 90 * 24 * time.Hour
const AttributionWindow = 7 * 24 * time.Hour

type Impression struct {
	ID, SessionToken string
	Result           Result
}
type ClickInput struct {
	SessionToken, ImpressionID, EventID string
	ProductID                           uint
	Position                            int
}
type QueryMetrics struct {
	ZeroResultRate   float64 `json:"zero_result_rate"`
	CTRAt5           float64 `json:"ctr_at_5"`
	CTRAt10          float64 `json:"ctr_at_10"`
	ConversionAt5    float64 `json:"conversion_at_5"`
	ConversionAt10   float64 `json:"conversion_at_10"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	Query            string  `json:"query"`
	Searches         int64   `json:"searches"`
	UniqueSessions   int64   `json:"unique_sessions"`
	ZeroResults      int64   `json:"zero_results"`
	Clicks           int64   `json:"clicks"`
	AddToCarts       int64   `json:"add_to_carts"`
	PaidOrders       int64   `json:"paid_orders"`
	CTR              float64 `json:"ctr"`
	ConversionRate   float64 `json:"conversion_rate"`
	AverageLatencyMs float64 `json:"average_latency_ms"`
}
type Analytics struct {
	FacetUsage map[string]int `json:"facet_usage"`
	QueryMetrics
	Days     int            `json:"days"`
	Queries  []QueryMetrics `json:"queries"`
	Popular  []string       `json:"popular"`
	Trending []string       `json:"trending"`
}

func sessionHash(token string) (string, error) {
	raw, err := hex.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return "", ErrAnalyticsInvalid
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (s *Service) CreateImpression(ctx context.Context, consent bool, token string, filters Filters) (Impression, error) {
	return s.CreateImpressionIdempotent(ctx, consent, token, uuid.NewString(), filters)
}
func (s *Service) CreateImpressionIdempotent(ctx context.Context, consent bool, token, eventID string, filters Filters) (Impression, error) {
	if parsed, err := uuid.Parse(eventID); err != nil || parsed == uuid.Nil {
		return Impression{}, ErrAnalyticsInvalid
	}
	if !consent || filters.Explain || filters.RuleOverrides != nil || filters.SkipMerchandising {
		return Impression{}, ErrAnalyticsInvalid
	}
	if token == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return Impression{}, err
		}
		token = hex.EncodeToString(raw)
	}
	hash, err := sessionHash(token)
	if err != nil {
		return Impression{}, err
	}
	if err := checkSession(ctx, s.db, hash); err != nil {
		return Impression{}, err
	}
	savedFilters, _ := json.Marshal(filters)
	var existing models.SearchQueryEvent
	err = s.db.WithContext(ctx).Where("impression_id = ?", eventID).First(&existing).Error
	if err == nil {
		if existing.SessionHash != hash || existing.FiltersJSON != string(savedFilters) {
			return Impression{}, ErrAnalyticsInvalid
		}
		var result Result
		if err := json.Unmarshal([]byte(existing.ResultJSON), &result); err != nil {
			return Impression{}, err
		}
		return Impression{ID: eventID, SessionToken: token, Result: result}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Impression{}, err
	}
	started := time.Now()
	result, err := s.Search(ctx, filters)
	if err != nil {
		return Impression{}, err
	}
	ids := make([]uint, 0, len(result.Products))
	for _, p := range result.Products {
		ids = append(ids, p.ID)
	}
	if result.Degraded {
		return Impression{}, ErrAnalyticsInvalid
	}
	resultJSON, _ := json.Marshal(result)
	products, _ := json.Marshal(ids)
	savedFilters, _ = json.Marshal(filters)
	row := models.SearchQueryEvent{ImpressionID: eventID, ResultJSON: string(resultJSON), SessionHash: hash, Query: filters.Query, NormalizedQuery: NormalizeQuery(filters.Query), FiltersJSON: string(savedFilters), ProductsJSON: string(products), ResultCount: int(result.Total), LatencyMs: time.Since(started).Milliseconds(), CreatedAt: s.now()}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSession(ctx, tx, hash); err != nil {
			return err
		}
		if err := checkSession(ctx, tx, hash); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	}); err != nil {
		return Impression{}, err
	}
	var stored models.SearchQueryEvent
	if err := s.db.WithContext(ctx).Where("impression_id = ?", row.ImpressionID).First(&stored).Error; err != nil {
		return Impression{}, err
	}
	if stored.SessionHash != hash || stored.FiltersJSON != row.FiltersJSON {
		return Impression{}, ErrAnalyticsInvalid
	}
	if err := json.Unmarshal([]byte(stored.ResultJSON), &result); err != nil {
		return Impression{}, err
	}
	return Impression{ID: row.ImpressionID, SessionToken: token, Result: result}, nil
}
func (s *Service) RecordClick(ctx context.Context, input ClickInput) (string, error) {
	hash, err := sessionHash(input.SessionToken)
	if err != nil {
		return "", err
	}
	if parsed, err := uuid.Parse(input.EventID); err != nil || parsed == uuid.Nil || input.ProductID == 0 || input.Position < 1 {
		return "", ErrAnalyticsInvalid
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSession(ctx, tx, hash); err != nil {
			return err
		}
		if err := checkSession(ctx, tx, hash); err != nil {
			return err
		}
		var row models.SearchQueryEvent
		if err := tx.First(&row, "impression_id = ? AND session_hash = ? AND created_at >= ?", input.ImpressionID, hash, s.now().Add(-AttributionWindow)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAnalyticsInvalid
			}
			return err
		}
		var ids []uint
		if err := json.Unmarshal([]byte(row.ProductsJSON), &ids); err != nil {
			return err
		}
		var f Filters
		if err := json.Unmarshal([]byte(row.FiltersJSON), &f); err != nil {
			return err
		}
		page, limit := f.Page, f.Limit
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 10
		}
		local := input.Position - (page-1)*limit - 1
		if local < 0 || local >= len(ids) || ids[local] != input.ProductID {
			return ErrAnalyticsInvalid
		}
		event := models.SearchClickEvent{EventID: input.EventID, ImpressionID: row.ImpressionID, QueryEventID: row.ID, SessionHash: hash, ProductID: input.ProductID, Position: input.Position, ClickedAt: s.now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
			return err
		}
		var existing models.SearchClickEvent
		if err := tx.First(&existing, "event_id = ?", event.EventID).Error; err != nil {
			return err
		}
		if existing.SessionHash != hash || existing.ImpressionID != row.ImpressionID || existing.ProductID != event.ProductID || existing.Position != event.Position {
			return ErrAnalyticsInvalid
		}
		return nil
	})
	return input.EventID, err
}

// RecordCartAttributionTx participates in the commerce owner's transaction.
func RecordCartAttributionTx(ctx context.Context, tx *gorm.DB, token, clickID string, productID, cartItemID uint) error {
	if token == "" && clickID == "" {
		return nil
	}
	hash, err := sessionHash(token)
	if err != nil {
		return err
	}
	if err := lockSession(ctx, tx, hash); err != nil {
		return err
	}
	if err := checkSession(ctx, tx, hash); err != nil {
		return err
	}
	now := time.Now().UTC()
	var click models.SearchClickEvent
	if err := tx.WithContext(ctx).Where("session_hash = ? AND product_id = ? AND clicked_at >= ?", hash, productID, now.Add(-AttributionWindow)).Order("clicked_at DESC, id DESC").First(&click).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAnalyticsInvalid
		}
		return err
	}
	if click.EventID != clickID || cartItemID == 0 {
		return ErrAnalyticsInvalid
	}
	var item models.CartItem
	if err := tx.WithContext(ctx).Preload("ProductVariant").First(&item, cartItemID).Error; err != nil {
		return err
	}
	if item.ProductVariant.ProductID != productID {
		return ErrAnalyticsInvalid
	}
	row := models.SearchCartAttribution{CartItemID: cartItemID, ClickID: click.EventID, ImpressionID: click.ImpressionID, ProductID: productID, CreatedAt: now}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "cart_item_id"}}, DoUpdates: clause.AssignmentColumns([]string{"click_id", "impression_id", "product_id", "created_at"})}).Create(&row).Error
}
func CaptureOrderAttributionTx(ctx context.Context, tx *gorm.DB, orderID, checkoutSessionID uint) error {
	var order models.Order
	if err := tx.WithContext(ctx).Preload("Items.ProductVariant").First(&order, orderID).Error; err != nil {
		return err
	}
	if order.CheckoutSessionID != checkoutSessionID {
		return ErrAnalyticsInvalid
	}
	products := map[uint]bool{}
	for _, item := range order.Items {
		products[item.ProductVariant.ProductID] = true
	}
	if err := tx.WithContext(ctx).Where("order_id = ? AND paid_at IS NULL", orderID).Delete(&models.SearchOrderAttribution{}).Error; err != nil {
		return err
	}
	var rows []models.SearchCartAttribution
	if err := tx.WithContext(ctx).Table("search_cart_attributions AS a").Select("a.*").Joins("JOIN cart_items ci ON ci.id = a.cart_item_id AND ci.deleted_at IS NULL").Joins("JOIN carts c ON c.id = ci.cart_id AND c.deleted_at IS NULL").Where("c.checkout_session_id = ? AND a.created_at >= ?", checkoutSessionID, time.Now().UTC().Add(-AttributionWindow)).Order("a.created_at DESC, a.cart_item_id DESC").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if !products[row.ProductID] {
			continue
		}
		var click models.SearchClickEvent
		if err := tx.WithContext(ctx).Where("event_id = ? AND clicked_at >= ?", row.ClickID, time.Now().UTC().Add(-AttributionWindow)).First(&click).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if err := lockSession(ctx, tx, click.SessionHash); err != nil {
			return err
		}
		if err := checkSession(ctx, tx, click.SessionHash); err != nil {
			if errors.Is(err, ErrAnalyticsInvalid) {
				continue
			}
			return err
		}
		if err := tx.WithContext(ctx).Where("session_hash = ? AND product_id = ? AND clicked_at >= ?", click.SessionHash, row.ProductID, time.Now().UTC().Add(-AttributionWindow)).Order("clicked_at DESC, id DESC").First(&click).Error; err != nil {
			return err
		}
		a := models.SearchOrderAttribution{OrderID: orderID, ProductID: row.ProductID, ImpressionID: click.ImpressionID, ClickID: click.EventID, CreatedAt: time.Now().UTC()}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
			return err
		}
	}
	return nil
}
func RecordPaidOrderAttributionTx(ctx context.Context, tx *gorm.DB, orderID uint) error {
	var order models.Order
	if err := tx.WithContext(ctx).First(&order, orderID).Error; err != nil {
		return err
	}
	if order.Status != models.StatusPaid {
		return nil
	}
	var rows []models.SearchOrderAttribution
	if err := tx.WithContext(ctx).Where("order_id = ? AND paid_at IS NULL", orderID).Find(&rows).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		var click models.SearchClickEvent
		if err := tx.WithContext(ctx).Where("event_id = ? AND clicked_at >= ?", row.ClickID, now.Add(-AttributionWindow)).First(&click).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if err := lockSession(ctx, tx, click.SessionHash); err != nil {
			return err
		}
		if err := checkSession(ctx, tx, click.SessionHash); err != nil {
			if errors.Is(err, ErrAnalyticsInvalid) {
				continue
			}
			return err
		}
		if err := tx.WithContext(ctx).Model(&models.SearchOrderAttribution{}).Where("id = ? AND paid_at IS NULL", row.ID).Update("paid_at", now).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) PruneAnalytics(ctx context.Context) error {
	cutoff := s.now().Add(-AnalyticsRetention)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&models.SearchRevokedSession{}, &models.SearchCartAttribution{}, &models.SearchOrderAttribution{}, &models.SearchQueryEvent{}} {
			if err := tx.Where("created_at < ?", cutoff).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Where("clicked_at < ?", cutoff).Delete(&models.SearchClickEvent{}).Error
	})
}

func (s *Service) Analytics(ctx context.Context, days int) (Analytics, error) {
	if days < 1 || days > 90 {
		return Analytics{}, ErrAnalyticsInvalid
	}
	since := s.now().Add(-time.Duration(days) * 24 * time.Hour)
	result := Analytics{Days: days, Queries: []QueryMetrics{}, Popular: []string{}, Trending: []string{}}
	type aggregate struct {
		Query                                          string
		Searches, UniqueSessions, ZeroResults, Latency int64
	}
	var rows []aggregate
	if err := s.db.WithContext(ctx).Table("search_query_events").Select("normalized_query AS query, COUNT(*) AS searches, COUNT(DISTINCT session_hash) AS unique_sessions, SUM(CASE WHEN result_count = 0 THEN 1 ELSE 0 END) AS zero_results, SUM(latency_ms) AS latency").Where("created_at >= ? AND session_hash <> '' AND impression_id <> ''", since).Group("normalized_query").Order("searches DESC, normalized_query ASC").Scan(&rows).Error; err != nil {
		return result, err
	}
	var unique int64
	if err := s.db.WithContext(ctx).Table("search_query_events").Where("created_at >= ? AND session_hash <> '' AND impression_id <> ''", since).Distinct("session_hash").Count(&unique).Error; err != nil {
		return result, err
	}
	result.UniqueSessions = unique
	type fact struct {
		Query string
		Count int64
	}
	facts := map[string]map[string]int64{}
	for _, kind := range []string{"clicks", "cart", "paid"} {
		var values []fact
		var q *gorm.DB
		switch kind {
		case "clicks":
			q = s.db.WithContext(ctx).Table("search_click_events AS e").Joins("JOIN search_query_events q ON q.impression_id = e.impression_id").Where("e.clicked_at >= ? AND q.created_at >= ?", since, since)
		case "cart":
			q = s.db.WithContext(ctx).Table("search_cart_attributions AS e").Joins("JOIN search_query_events q ON q.impression_id = e.impression_id").Where("e.created_at >= ? AND q.created_at >= ?", since, since)
		default:
			q = s.db.WithContext(ctx).Table("search_order_attributions AS e").Joins("JOIN search_query_events q ON q.impression_id = e.impression_id").Where("e.paid_at >= ? AND q.created_at >= ?", since, since)
		}
		count := "COUNT(*)"
		if kind == "paid" {
			count = "COUNT(DISTINCT e.order_id)"
		}
		if err := q.Select("q.normalized_query AS query, " + count + " AS count").Group("q.normalized_query").Scan(&values).Error; err != nil {
			return result, err
		}
		facts[kind] = map[string]int64{}
		for _, v := range values {
			facts[kind][v.Query] = v.Count
		}
	}
	var totalLatency int64
	for _, row := range rows {
		m := QueryMetrics{Query: row.Query, Searches: row.Searches, UniqueSessions: row.UniqueSessions, ZeroResults: row.ZeroResults, Clicks: facts["clicks"][row.Query], AddToCarts: facts["cart"][row.Query], PaidOrders: facts["paid"][row.Query]}
		m.CTR = float64(m.Clicks) / float64(m.Searches)
		m.ConversionRate = float64(m.PaidOrders) / float64(m.Searches)
		m.AverageLatencyMs = float64(row.Latency) / float64(row.Searches)
		if err := s.enrichMetrics(ctx, since, &m, true); err != nil {
			return result, err
		}
		result.Queries = append(result.Queries, m)
		result.Searches += m.Searches
		result.ZeroResults += m.ZeroResults
		result.Clicks += m.Clicks
		result.AddToCarts += m.AddToCarts
		result.PaidOrders += m.PaidOrders
		totalLatency += row.Latency
	}
	if result.Searches > 0 {
		result.CTR = float64(result.Clicks) / float64(result.Searches)
		result.ConversionRate = float64(result.PaidOrders) / float64(result.Searches)
		result.AverageLatencyMs = float64(totalLatency) / float64(result.Searches)
	}
	if err := s.enrichMetrics(ctx, since, &result.QueryMetrics, false); err != nil {
		return result, err
	}
	result.FacetUsage = map[string]int{}
	var filterRows []string
	if err := s.db.WithContext(ctx).Model(&models.SearchQueryEvent{}).Where("created_at >= ? AND session_hash <> '' AND impression_id <> ''", since).Pluck("filters_json", &filterRows).Error; err != nil {
		return result, err
	}
	for _, raw := range filterRows {
		var f Filters
		if json.Unmarshal([]byte(raw), &f) != nil {
			continue
		}
		for name, count := range map[string]int{"brand": len(f.BrandSlugs), "category": len(f.CategorySlugs), "stock": len(f.StockAvailability), "price": len(f.PriceRanges), "attribute": len(f.AttributeValues)} {
			if count > 0 {
				result.FacetUsage[name]++
			}
		}
	}
	popular, trending, err := s.PopularQueries(ctx, "", 20)
	result.Popular = popular
	result.Trending = trending
	return result, err
}
func (s *Service) PopularQueries(ctx context.Context, prefix string, limit int) ([]string, []string, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	now := s.now()
	var rows []struct {
		Query           string
		Count, Sessions int64
	}
	q := s.db.WithContext(ctx).Table("search_query_events").Select("normalized_query AS query, COUNT(*) AS count, COUNT(DISTINCT session_hash) AS sessions").Where("created_at >= ? AND normalized_query <> '' AND session_hash <> '' AND impression_id <> ''", now.Add(-7*24*time.Hour)).Group("normalized_query").Having("COUNT(DISTINCT session_hash) >= ?", 5).Order("count DESC, normalized_query ASC")
	if normalized := NormalizeQuery(prefix); normalized != "" {
		q = q.Where("normalized_query LIKE ? ESCAPE '\\'", escapeLike(normalized)+"%")
	}
	if err := q.Limit(200).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	popular := []string{}
	type trend struct {
		query  string
		growth int64
	}
	trends := []trend{}
	for _, row := range rows {
		if len(popular) < limit {
			popular = append(popular, row.Query)
		}
		var previous int64
		if err := s.db.WithContext(ctx).Model(&models.SearchQueryEvent{}).Where("normalized_query = ? AND created_at >= ? AND created_at < ? AND session_hash <> '' AND impression_id <> ''", row.Query, now.Add(-14*24*time.Hour), now.Add(-7*24*time.Hour)).Count(&previous).Error; err != nil {
			return nil, nil, err
		}
		if row.Count > previous {
			trends = append(trends, trend{row.Query, row.Count - previous})
		}
	}
	sort.Slice(trends, func(i, j int) bool {
		if trends[i].growth != trends[j].growth {
			return trends[i].growth > trends[j].growth
		}
		return trends[i].query < trends[j].query
	})
	trending := []string{}
	for _, v := range trends {
		if len(trending) >= limit {
			break
		}
		trending = append(trending, v.query)
	}
	return popular, trending, nil
}

func checkSession(ctx context.Context, db *gorm.DB, hash string) error {
	var count int64
	if err := db.WithContext(ctx).Model(&models.SearchRevokedSession{}).Where("session_hash = ?", hash).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrAnalyticsInvalid
	}
	return nil
}
func (s *Service) RevokeConsent(ctx context.Context, token string) error {
	hash, err := sessionHash(token)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSession(ctx, tx, hash); err != nil {
			return err
		}
		sub := tx.Model(&models.SearchQueryEvent{}).Select("impression_id").Where("session_hash = ?", hash)
		for _, model := range []any{&models.SearchCartAttribution{}, &models.SearchOrderAttribution{}} {
			if err := tx.Where("impression_id IN (?)", sub).Delete(model).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("session_hash = ?", hash).Delete(&models.SearchClickEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_hash = ?", hash).Delete(&models.SearchQueryEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SearchRevokedSession{SessionHash: hash, CreatedAt: s.now()}).Error; err != nil {
			return err
		}
		return s.RecomputeConversionSignalsTx(ctx, tx)
	})
}
func (s *Service) enrichMetrics(ctx context.Context, since time.Time, m *QueryMetrics, filterQuery bool) error {
	if m.Searches == 0 {
		return nil
	}
	m.ZeroResultRate = float64(m.ZeroResults) / float64(m.Searches)
	q := s.db.WithContext(ctx).Model(&models.SearchQueryEvent{}).Where("created_at >= ? AND session_hash <> '' AND impression_id <> ''", since)
	if filterQuery {
		q = q.Where("normalized_query = ?", m.Query)
	}
	var latency []int64
	if err := q.Session(&gorm.Session{}).Order("latency_ms ASC").Pluck("latency_ms", &latency).Error; err != nil {
		return err
	}
	if len(latency) > 0 {
		percentile := func(p float64) float64 {
			i := int(math.Ceil(float64(len(latency))*p)) - 1
			if i >= len(latency) {
				i = len(latency) - 1
			}
			return float64(latency[i])
		}
		m.P95LatencyMs = percentile(.95)
		m.P99LatencyMs = percentile(.99)
	}
	var snapshots []models.SearchQueryEvent
	if err := q.Find(&snapshots).Error; err != nil {
		return err
	}
	for _, k := range []int{0, 5, 10} {
		eligible := 0
		for _, snap := range snapshots {
			var filters Filters
			var products []uint
			if json.Unmarshal([]byte(snap.FiltersJSON), &filters) != nil || json.Unmarshal([]byte(snap.ProductsJSON), &products) != nil || len(products) == 0 {
				continue
			}
			page, limit := filters.Page, filters.Limit
			if page < 1 {
				page = 1
			}
			if limit < 1 {
				limit = 10
			}
			if k == 0 || (page-1)*limit+1 <= k {
				eligible++
			}
		}
		rank := k
		if rank == 0 {
			rank = 1000000000
		}
		var clicks, paid int64
		c := s.db.WithContext(ctx).Table("search_click_events e").Joins("JOIN search_query_events q ON q.impression_id = e.impression_id").Where("q.created_at >= ? AND e.clicked_at >= ? AND e.position <= ?", since, since, rank)
		if filterQuery {
			c = c.Where("q.normalized_query = ?", m.Query)
		}
		if err := c.Distinct("q.impression_id").Count(&clicks).Error; err != nil {
			return err
		}
		p := s.db.WithContext(ctx).Table("search_order_attributions a").Joins("JOIN search_click_events e ON e.event_id = a.click_id").Joins("JOIN search_query_events q ON q.impression_id = a.impression_id").Where("q.created_at >= ? AND a.paid_at >= ? AND e.position <= ?", since, since, rank)
		if filterQuery {
			p = p.Where("q.normalized_query = ?", m.Query)
		}
		if err := p.Distinct("q.impression_id").Count(&paid).Error; err != nil {
			return err
		}
		if k == 0 {
			m.CTR = float64(clicks) / float64(m.Searches)
			m.ConversionRate = float64(paid) / float64(m.Searches)
			continue
		}
		if eligible == 0 {
			continue
		}
		if k == 5 {
			m.CTRAt5 = float64(clicks) / float64(eligible)
			m.ConversionAt5 = float64(paid) / float64(eligible)
		} else {
			m.CTRAt10 = float64(clicks) / float64(eligible)
			m.ConversionAt10 = float64(paid) / float64(eligible)
		}
	}
	return nil
}

// Serialize consent revocation and event writes across PostgreSQL replicas. The
// lock identity is derived from an anonymous owner hash, never a raw token.
func lockSession(ctx context.Context, tx *gorm.DB, hash string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	raw, err := hex.DecodeString(hash)
	if err != nil || len(raw) < 8 {
		return ErrAnalyticsInvalid
	}
	key := int64(binary.BigEndian.Uint64(raw[:8]))
	return tx.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(?)", key).Error
}
