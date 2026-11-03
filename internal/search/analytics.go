package search

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	Page, Limit, TotalQueries int            `json:"-"`
	FacetUsage                map[string]int `json:"facet_usage"`
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
