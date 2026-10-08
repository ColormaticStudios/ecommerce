package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDefaultRankingProfileRequired = errors.New("a default ranking profile is required")

type RankingWeights struct {
	TokenCoverage float64 `json:"token_coverage"`
	ExactPhrase   float64 `json:"exact_phrase"`
	Name          float64 `json:"name"`
	Brand         float64 `json:"brand"`
	Attributes    float64 `json:"attributes"`
	Recency       float64 `json:"recency"`
	Availability  float64 `json:"availability"`
	Sales         float64 `json:"sales"`
}

type RankingProfile struct {
	ID        uint
	Name      string
	Weights   RankingWeights
	IsDefault bool
	Version   int
	UpdatedBy *uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RankingProfileInput struct {
	Name      string
	Weights   RankingWeights
	IsDefault bool
}
type RankingProfilePatch struct {
	Name      *string
	Weights   *RankingWeights
	IsDefault *bool
}
type RankingComponent struct {
	Name                        string
	Value, Weight, Contribution float64
}
type RankingExplanation struct {
	ProductID     uint
	Score         float64
	AdjustedScore float64
	Components    []RankingComponent
}

func DefaultRankingWeights() RankingWeights {
	return RankingWeights{TokenCoverage: 8, ExactPhrase: 6, Name: 5, Brand: 2, Attributes: 2, Recency: 0.5, Availability: 1, Sales: 1}
}

func validateRankingWeights(w RankingWeights) error {
	values := []float64{w.TokenCoverage, w.ExactPhrase, w.Name, w.Brand, w.Attributes, w.Recency, w.Availability, w.Sales}
	total := 0.0
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return fmt.Errorf("%w: weights must be finite values between 0 and 100", ErrConfigurationInvalid)
		}
		total += value
	}
	if total == 0 {
		return fmt.Errorf("%w: at least one ranking weight must be positive", ErrConfigurationInvalid)
	}
	return nil
}

func rankingProfileFromModel(row models.SearchRankingProfile) (RankingProfile, error) {
	var weights RankingWeights
	if err := json.Unmarshal([]byte(row.WeightsJSON), &weights); err != nil {
		return RankingProfile{}, fmt.Errorf("invalid persisted ranking weights: %w", err)
	}
	if err := validateRankingWeights(weights); err != nil {
		return RankingProfile{}, err
	}
	return RankingProfile{ID: row.ID, Name: row.Name, Weights: weights, IsDefault: row.IsDefault, Version: row.Version, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (s *Service) ListRankingProfiles(ctx context.Context) ([]RankingProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []models.SearchRankingProfile
	if err := db.Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]RankingProfile, 0, len(rows))
	for _, row := range rows {
		profile, err := rankingProfileFromModel(row)
		if err != nil {
			return nil, err
		}
		result = append(result, profile)
	}
	return result, nil
}
func (s *Service) GetRankingProfile(ctx context.Context, id uint) (RankingProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return RankingProfile{}, err
	}
	var row models.SearchRankingProfile
	if err := db.First(&row, id).Error; err != nil {
		return RankingProfile{}, configurationLookupError(err)
	}
	return rankingProfileFromModel(row)
}

// Lock every profile in a consistent order before changing default ownership.
// The partial unique index provides the final invariant under concurrent creates.
func lockRankingProfiles(tx *gorm.DB) ([]models.SearchRankingProfile, error) {
	var rows []models.SearchRankingProfile
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC").Find(&rows).Error
	return rows, err
}
func (s *Service) clearRankingDefault(tx *gorm.DB, actor *uint) error {
	return tx.Model(&models.SearchRankingProfile{}).Where("is_default = ?", true).Updates(map[string]any{"is_default": false, "version": gorm.Expr("version + 1"), "updated_by": actor, "updated_at": s.now().UTC()}).Error
}

func (s *Service) CreateRankingProfile(ctx context.Context, input RankingProfileInput, actor *uint) (RankingProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return RankingProfile{}, err
	}
	name, err := normalizeConfigurationName(input.Name)
	if err != nil {
		return RankingProfile{}, err
	}
	if err := validateRankingWeights(input.Weights); err != nil {
		return RankingProfile{}, err
	}
	raw, _ := json.Marshal(input.Weights)
	row := models.SearchRankingProfile{Name: name, WeightsJSON: string(raw), Version: 1, UpdatedBy: actor}
	err = db.Transaction(func(tx *gorm.DB) error {
		rows, err := lockRankingProfiles(tx)
		if err != nil {
			return err
		}
		if input.IsDefault || len(rows) == 0 {
			if err := s.clearRankingDefault(tx, actor); err != nil {
				return err
			}
			row.IsDefault = true
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return RankingProfile{}, configurationWriteError(err)
	}
	return rankingProfileFromModel(row)
}
func (s *Service) UpdateRankingProfile(ctx context.Context, id uint, patch RankingProfilePatch, actor *uint) (RankingProfile, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return RankingProfile{}, err
	}
	if patch.Name == nil && patch.Weights == nil && patch.IsDefault == nil {
		return RankingProfile{}, fmt.Errorf("%w: patch cannot be empty", ErrConfigurationInvalid)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		rows, err := lockRankingProfiles(tx)
		if err != nil {
			return err
		}
		var current *models.SearchRankingProfile
		for i := range rows {
			if rows[i].ID == id {
				current = &rows[i]
				break
			}
		}
		if current == nil {
			return ErrConfigurationNotFound
		}
		updates := map[string]any{"version": gorm.Expr("version + 1"), "updated_by": actor, "updated_at": s.now().UTC()}
		if patch.Name != nil {
			name, err := normalizeConfigurationName(*patch.Name)
			if err != nil {
				return err
			}
			updates["name"] = name
		}
		if patch.Weights != nil {
			if err := validateRankingWeights(*patch.Weights); err != nil {
				return err
			}
			raw, _ := json.Marshal(patch.Weights)
			updates["weights_json"] = string(raw)
		}
		if patch.IsDefault != nil {
			if current.IsDefault && !*patch.IsDefault {
				return ErrDefaultRankingProfileRequired
			}
			if *patch.IsDefault && !current.IsDefault {
				if err := s.clearRankingDefault(tx, actor); err != nil {
					return err
				}
			}
			updates["is_default"] = *patch.IsDefault
		}
		return tx.Model(current).Updates(updates).Error
	})
	if err != nil {
		return RankingProfile{}, configurationWriteError(err)
	}
	return s.GetRankingProfile(ctx, id)
}
func (s *Service) DeleteRankingProfile(ctx context.Context, id uint) error {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		rows, err := lockRankingProfiles(tx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ID == id {
				if row.IsDefault {
					return ErrDefaultRankingProfileRequired
				}
				return tx.Unscoped().Delete(&row).Error
			}
		}
		return ErrConfigurationNotFound
	})
}

func (b *databaseBackend) loadRankingProfile(ctx context.Context, name string) (RankingProfile, error) {
	var rows []models.SearchRankingProfile
	q := b.db.WithContext(ctx)
	if name != "" {
		q = q.Where("name = ?", name)
	} else {
		q = q.Where("is_default = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return RankingProfile{}, err
	}
	if len(rows) != 1 {
		if name != "" && len(rows) == 0 {
			return RankingProfile{}, ErrConfigurationNotFound
		}
		return RankingProfile{}, errors.New("search ranking configuration requires exactly one default profile")
	}
	profile, err := rankingProfileFromModel(rows[0])
	if err != nil {
		return RankingProfile{}, fmt.Errorf("search ranking configuration invalid: %v", err)
	}
	return profile, nil
}

// Boundary matching prevents short tokens in an unrelated field from receiving
// a field boost. Retrieval retains its existing substring recall behavior.
func containsRankingTerm(field, term string) bool {
	return term != "" && strings.Contains(" "+NormalizeQuery(field)+" ", " "+term+" ")
}
func rankingCoverage(field string, plan queryPlan) float64 {
	if len(plan.groups) == 0 {
		return 0
	}
	total := 0.0
	for _, group := range plan.groups {
		if containsRankingTerm(field, group.original) {
			total++
			continue
		}
		for _, alternative := range group.alternatives {
			if containsRankingTerm(field, alternative) {
				total += 0.5
				break
			}
		}
	}
	return total / float64(len(plan.groups))
}
func explainRanking(document indexedProduct, plan queryPlan, query string, profile RankingProfile, units int64, now time.Time) RankingExplanation {
	attributes := make([]string, 0, len(document.product.AttributeValues))
	for _, value := range document.product.AttributeValues {
		attributes = append(attributes, attributeValue(value))
	}
	brand := ""
	if document.product.Brand != nil {
		brand = document.product.Brand.Name
	}
	phrase := 0.0
	if len(strings.Fields(query)) > 1 {
		if containsRankingTerm(document.product.Name, query) {
			phrase = 1
		} else if containsRankingTerm(document.product.Description, query) {
			phrase = 0.5
		}
	}
	age := math.Max(0, now.Sub(document.document.SourceCreatedAt).Hours()/24)
	available := 0.0
	if document.document.Available {
		available = 1
	}
	sales := 0.0
	if units > 0 {
		sales = float64(units) / (float64(units) + 20)
	}
	result := RankingExplanation{ProductID: document.document.EntityID, Components: []RankingComponent{
		{Name: "token_coverage", Value: rankingCoverage(document.document.SearchableText, plan), Weight: profile.Weights.TokenCoverage},
		{Name: "exact_phrase", Value: phrase, Weight: profile.Weights.ExactPhrase},
		{Name: "name", Value: rankingCoverage(document.product.Name, plan), Weight: profile.Weights.Name},
		{Name: "brand", Value: rankingCoverage(brand, plan), Weight: profile.Weights.Brand},
		{Name: "attributes", Value: rankingCoverage(strings.Join(attributes, " "), plan), Weight: profile.Weights.Attributes},
		{Name: "recency", Value: math.Exp2(-age / 30), Weight: profile.Weights.Recency},
		{Name: "availability", Value: available, Weight: profile.Weights.Availability},
		{Name: "sales", Value: sales, Weight: profile.Weights.Sales},
	}}
	for i := range result.Components {
		component := &result.Components[i]
		component.Contribution = component.Value * component.Weight
		result.Score += component.Contribution
	}
	result.AdjustedScore = result.Score
	return result
}
func (b *databaseBackend) rankProducts(ctx context.Context, documents []indexedProduct, plan queryPlan, query string, profile RankingProfile, order string) error {
	signals := map[uint]int64{}
	// Batch IDs to remain below SQLite/Postgres parameter limits.
	for start := 0; start < len(documents); start += 500 {
		end := min(start+500, len(documents))
		ids := make([]uint, 0, end-start)
		for _, doc := range documents[start:end] {
			ids = append(ids, doc.document.EntityID)
		}
		var rows []models.SearchSalesSignal
		if err := b.db.WithContext(ctx).Where("product_id IN ?", ids).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			signals[row.ProductID] = row.Units30Days
		}
	}
	now := b.now().UTC().Truncate(24 * time.Hour)
	for i := range documents {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		documents[i].ranking = explainRanking(documents[i], plan, query, profile, signals[documents[i].document.EntityID], now)
	}
	ascending := strings.EqualFold(order, "asc")
	sort.Slice(documents, func(i, j int) bool {
		a, c := documents[i].ranking.Score, documents[j].ranking.Score
		if a == c {
			return documents[i].document.EntityID < documents[j].document.EntityID
		}
		if ascending {
			return a < c
		}
		return a > c
	})
	return nil
}
