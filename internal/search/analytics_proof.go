package search

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ecommerce/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ResultProofLifetime = 5 * time.Minute
const MaxResultTokenLength = 65536
const resultProofDomain = "ecommerce/search-exposure/v1\x00"

type resultProof struct {
	Version    int       `json:"v"`
	SnapshotID string    `json:"id"`
	CreatedAt  time.Time `json:"at"`
	Filters    Filters   `json:"f"`
	Products   []uint    `json:"p"`
	Total      int64     `json:"n"`
	LatencyMs  int64     `json:"ms"`
}

// ConfigureAnalyticsSigningKey is startup-only. All replicas must share the key.
func (s *Service) ConfigureAnalyticsSigningKey(key []byte) error {
	if len(key) == 0 {
		return errors.New("search analytics signing key must not be empty")
	}
	derived := sha256.Sum256(append([]byte(resultProofDomain+"key\x00"), key...))
	s.analyticsKey = append([]byte(nil), derived[:]...)
	return nil
}

// SignResultSnapshot signs only server-computed storefront exposure, never products or previews.
func (s *Service) SignResultSnapshot(result Result, filters Filters, latency time.Duration) (string, error) {
	if result.Degraded || filters.Explain || filters.RuleOverrides != nil || filters.SkipMerchandising {
		return "", nil
	}
	if len(s.analyticsKey) < 32 {
		return "", nil
	} // Collection is unavailable without configured shared signing.
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 {
		filters.Limit = 10
	}
	if filters.Limit > 100 || filters.Page > 1000000 || len(result.Products) > 100 {
		return "", ErrAnalyticsInvalid
	}
	filters.Query = NormalizeQuery(filters.Query)
	ids := make([]uint, 0, len(result.Products))
	for _, p := range result.Products {
		if p.ID == 0 {
			return "", ErrAnalyticsInvalid
		}
		ids = append(ids, p.ID)
	}
	payload, err := json.Marshal(resultProof{1, uuid.NewString(), s.now().UTC(), filters, ids, result.Total, max(0, latency.Milliseconds())})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.analyticsKey)
	mac.Write([]byte(resultProofDomain))
	mac.Write(payload)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(token) > MaxResultTokenLength {
		// Collection is optional. A valid search with many accepted filter
		// values must still succeed when its exposure cannot fit the proof.
		return "", nil
	}
	return token, nil
}

func (s *Service) verifyResultSnapshot(token string) (resultProof, error) {
	return s.decodeResultSnapshot(token, resultProofDomain, ResultProofLifetime)
}

func (s *Service) decodeResultSnapshot(token, domain string, lifetime time.Duration) (resultProof, error) {
	var proof resultProof
	if len(s.analyticsKey) < 32 || len(token) == 0 || len(token) > MaxResultTokenLength {
		return proof, ErrAnalyticsInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return proof, ErrAnalyticsInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return proof, ErrAnalyticsInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return proof, ErrAnalyticsInvalid
	}
	mac := hmac.New(sha256.New, s.analyticsKey)
	mac.Write([]byte(domain))
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) || json.Unmarshal(payload, &proof) != nil {
		return proof, ErrAnalyticsInvalid
	}
	if proof.Version != 1 || proof.CreatedAt.After(s.now()) || !s.now().Before(proof.CreatedAt.Add(lifetime)) || proof.Total < 0 || len(proof.Products) > 100 || proof.Filters.Page < 1 || proof.Filters.Page > 1000000 || proof.Filters.Limit < 1 || proof.Filters.Limit > 100 || proof.Filters.Explain || proof.Filters.RuleOverrides != nil || proof.Filters.SkipMerchandising {
		return proof, ErrAnalyticsInvalid
	}
	if id, err := uuid.Parse(proof.SnapshotID); err != nil || id == uuid.Nil {
		return proof, ErrAnalyticsInvalid
	}
	return proof, nil
}

func facetMask(f Filters) int {
	mask := 0
	for i, selected := range []bool{len(f.BrandSlugs) > 0, len(f.CategorySlugs) > 0, len(f.StockAvailability) > 0, len(f.PriceRanges) > 0 || f.MinPrice != nil || f.MaxPrice != nil, len(f.AttributeValues) > 0} {
		if selected {
			mask |= 1 << i
		}
	}
	return mask
}

// RecordImpression consumes an authoritative proof without calling search or catalog retrieval.
func (s *Service) RecordImpression(ctx context.Context, consent bool, token, eventID, resultToken string) (Impression, error) {
	if eventID == "" {
		eventID = uuid.NewString()
	}
	if id, err := uuid.Parse(eventID); !consent || err != nil || id == uuid.Nil {
		return Impression{}, ErrAnalyticsInvalid
	}
	proof, err := s.verifyResultSnapshot(resultToken)
	if err != nil {
		return Impression{}, err
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
	filters, _ := json.Marshal(proof.Filters)
	products, _ := json.Marshal(proof.Products)
	row := models.SearchQueryEvent{ImpressionID: eventID, SnapshotID: proof.SnapshotID, SessionHash: hash, Query: proof.Filters.Query, NormalizedQuery: NormalizeQuery(proof.Filters.Query), FiltersJSON: string(filters), ProductsJSON: string(products), ResultCount: int(proof.Total), LatencyMs: proof.LatencyMs, CreatedAt: proof.CreatedAt, FirstPosition: (proof.Filters.Page-1)*proof.Filters.Limit + 1, ProductCount: len(proof.Products), FacetMask: facetMask(proof.Filters)}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSession(ctx, tx, hash); err != nil {
			return err
		}
		if err := checkSession(ctx, tx, hash); err != nil {
			return err
		}
		var existing models.SearchQueryEvent
		err := tx.Where("impression_id = ?", eventID).First(&existing).Error
		if err == nil {
			if existing.SessionHash != hash || existing.SnapshotID != proof.SnapshotID {
				return ErrAnalyticsInvalid
			}
			row = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("session_hash = ? AND snapshot_id = ?", hash, proof.SnapshotID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAnalyticsInvalid
			}
			return err
		}
		row = existing
		return nil
	})
	if err != nil {
		return Impression{}, err
	}
	return Impression{ID: row.ImpressionID, SessionToken: token}, nil
}

// Cacheable snapshots carry only signed server-computed result identity. They
// are not impression proofs and cannot be consumed directly by collection.
const CacheableSnapshotLifetime = time.Hour
const cacheableSnapshotDomain = "ecommerce/search-cache-snapshot/v1\x00"

func (s *Service) SignCacheableSnapshot(result Result, filters Filters, latency time.Duration) (string, error) {
	token, err := s.SignResultSnapshot(result, filters, latency)
	if err != nil || token == "" {
		return "", err
	}
	proof, err := s.verifyResultSnapshot(token)
	if err != nil {
		return "", err
	}
	return s.encodeResultSnapshot(proof, cacheableSnapshotDomain)
}

func (s *Service) BrowserResultProof(snapshotToken, eventID string) (string, error) {
	id, err := uuid.Parse(eventID)
	if err != nil || id == uuid.Nil {
		return "", ErrAnalyticsInvalid
	}
	proof, err := s.decodeResultSnapshot(snapshotToken, cacheableSnapshotDomain, CacheableSnapshotLifetime)
	if err != nil {
		return "", err
	}
	proof.CreatedAt = s.now().UTC()
	namespace, err := uuid.Parse(proof.SnapshotID)
	if err != nil {
		return "", ErrAnalyticsInvalid
	}
	proof.SnapshotID = uuid.NewSHA1(namespace, []byte(eventID)).String()
	return s.encodeResultSnapshot(proof, resultProofDomain)
}

func (s *Service) encodeResultSnapshot(proof resultProof, domain string) (string, error) {
	payload, err := json.Marshal(proof)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.analyticsKey)
	mac.Write([]byte(domain))
	mac.Write(payload)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(token) > MaxResultTokenLength {
		return "", ErrAnalyticsInvalid
	}
	return token, nil
}
