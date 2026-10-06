package search

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"ecommerce/internal/migrations"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed testdata/relevance-corpus.json testdata/relevance-baseline.json
var relevanceFixtures embed.FS

// RelevanceMetrics uses graded NDCG@10 and binary precision@5, macro averaged
// across the judged queries. Unjudged results have relevance zero.
type RelevanceMetrics struct {
	NDCG10     float64 `json:"ndcg_at_10"`
	Precision5 float64 `json:"precision_at_5"`
}

type RelevanceQueryReport struct {
	Name string `json:"name"`
	IDs  []uint `json:"product_ids"`
	RelevanceMetrics
}

type RelevanceReport struct {
	RelevanceMetrics
	Baseline RelevanceMetrics       `json:"baseline"`
	Queries  []RelevanceQueryReport `json:"queries"`
}

type relevanceCorpus struct {
	Now      time.Time          `json:"now"`
	Products []relevanceProduct `json:"products"`
	Synonyms []SynonymSetInput  `json:"synonyms"`
	Queries  []relevanceQuery   `json:"queries"`
}

type relevanceProduct struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Brand       string  `json:"brand"`
	Category    string  `json:"category"`
	Color       string  `json:"color"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	AgeDays     int     `json:"age_days"`
	Sales       int64   `json:"sales"`
}

type relevanceQuery struct {
	Name      string       `json:"name"`
	Query     string       `json:"query"`
	Profile   string       `json:"profile"`
	Brand     string       `json:"brand"`
	Category  string       `json:"category"`
	Color     string       `json:"color"`
	Available *bool        `json:"available"`
	MaxPrice  *float64     `json:"max_price"`
	Judgments map[uint]int `json:"judgments"`
}

// EvaluateRelevance runs the production projector and retrieval pipeline in a
// disposable database with the production migration seeds and a frozen clock.
// It does not read or mutate the configured application database.
func EvaluateRelevance(ctx context.Context) (RelevanceReport, error) {
	var corpus relevanceCorpus
	if err := decodeRelevanceFixture("testdata/relevance-corpus.json", &corpus); err != nil {
		return RelevanceReport{}, err
	}
	if err := validateRelevanceCorpus(corpus); err != nil {
		return RelevanceReport{}, err
	}
	var baseline RelevanceMetrics
	if err := decodeRelevanceFixture("testdata/relevance-baseline.json", &baseline); err != nil {
		return RelevanceReport{}, err
	}
	db, err := gorm.Open(sqlite.Open("file:search-eval-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return RelevanceReport{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return RelevanceReport{}, err
	}
	defer sqlDB.Close()
	if err := migrations.Run(db.WithContext(ctx)); err != nil {
		return RelevanceReport{}, fmt.Errorf("evaluation migrations: %w", err)
	}
	service := NewService(db, nil, nil)
	service.now = func() time.Time { return corpus.Now }
	service.backend.(*databaseBackend).now = service.now
	documents := make([]models.SearchDocument, 0, len(corpus.Products))
	for _, fixture := range corpus.Products {
		created := corpus.Now.AddDate(0, 0, -fixture.AgeDays)
		product := models.Product{BaseModel: models.BaseModel{ID: fixture.ID, CreatedAt: created, UpdatedAt: created}, SKU: fmt.Sprintf("EVAL-%d", fixture.ID), Name: fixture.Name, Description: fixture.Description, Price: models.MoneyFromFloat(fixture.Price), Stock: fixture.Stock, IsPublished: true}
		if fixture.Brand != "" {
			product.Brand = &models.Brand{Name: fixture.Brand, Slug: fixture.Brand, IsActive: true}
		}
		if fixture.Category != "" {
			product.Categories = []models.Category{{Name: fixture.Category, Slug: fixture.Category, IsActive: true}}
		}
		if fixture.Color != "" {
			value := fixture.Color
			product.AttributeValues = []models.ProductAttributeValue{{TextValue: &value, ProductAttribute: &models.ProductAttribute{Key: "Color", Slug: "color", Filterable: true}}}
		}
		document, include, err := projectProduct(product, corpus.Now)
		if err != nil {
			return RelevanceReport{}, err
		}
		if !include {
			return RelevanceReport{}, fmt.Errorf("fixture %d cannot be indexed", fixture.ID)
		}
		documents = append(documents, document)
		if err := db.WithContext(ctx).Create(&models.SearchSalesSignal{ProductID: fixture.ID, Units30Days: fixture.Sales, AsOf: corpus.Now, UpdatedAt: corpus.Now}).Error; err != nil {
			return RelevanceReport{}, err
		}
	}
	if err := service.backend.ReplaceAll(ctx, documents, corpus.Now); err != nil {
		return RelevanceReport{}, err
	}
	for _, synonym := range corpus.Synonyms {
		if _, err := service.CreateSynonymSet(ctx, synonym); err != nil {
			return RelevanceReport{}, err
		}
	}
	report := RelevanceReport{Baseline: baseline, Queries: make([]RelevanceQueryReport, 0, len(corpus.Queries))}
	for _, query := range corpus.Queries {
		filters := Filters{Query: query.Query, RankingProfile: query.Profile, SortField: "relevance", Page: 1, Limit: 10, BrandSlug: query.Brand, HasVariantStock: query.Available, MaxPrice: query.MaxPrice}
		if query.Category != "" {
			filters.CategorySlugs = []string{query.Category}
		}
		if query.Color != "" {
			filters.AttributeValues = map[string][]string{"color": {query.Color}}
		}
		result, err := service.Search(ctx, filters)
		if err != nil {
			return RelevanceReport{}, fmt.Errorf("query %s: %w", query.Name, err)
		}
		ids := make([]uint, 0, len(result.Products))
		for _, product := range result.Products {
			ids = append(ids, product.ID)
		}
		metrics := measureRelevance(ids, query.Judgments)
		report.Queries = append(report.Queries, RelevanceQueryReport{Name: query.Name, IDs: ids, RelevanceMetrics: metrics})
		report.NDCG10 += metrics.NDCG10
		report.Precision5 += metrics.Precision5
	}
	report.NDCG10 /= float64(len(report.Queries))
	report.Precision5 /= float64(len(report.Queries))
	return report, nil
}

// CheckRegression fails when either macro metric loses more than 0.02 absolute
// against the committed baseline. Equality at the tolerance is accepted.
func (report RelevanceReport) CheckRegression() error {
	const tolerance = 0.02
	if report.NDCG10+tolerance+1e-12 < report.Baseline.NDCG10 || report.Precision5+tolerance+1e-12 < report.Baseline.Precision5 {
		return fmt.Errorf("search relevance regression: NDCG@10 %.6f (baseline %.6f), precision@5 %.6f (baseline %.6f), allowed loss %.2f", report.NDCG10, report.Baseline.NDCG10, report.Precision5, report.Baseline.Precision5, tolerance)
	}
	return nil
}

func measureRelevance(ids []uint, judgments map[uint]int) RelevanceMetrics {
	grades := make([]int, 0, len(judgments))
	for _, grade := range judgments {
		grades = append(grades, grade)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	ideal := 0.0
	for index, grade := range grades {
		if index >= 10 {
			break
		}
		ideal += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(index+2))
	}
	actual := 0.0
	relevant := 0
	seen := map[uint]bool{}
	for index, id := range ids {
		if index >= 10 {
			break
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		grade := judgments[id]
		actual += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(index+2))
		if index < 5 && grade > 0 {
			relevant++
		}
	}
	metrics := RelevanceMetrics{Precision5: float64(relevant) / 5}
	if ideal > 0 {
		metrics.NDCG10 = actual / ideal
	}
	return metrics
}

func decodeRelevanceFixture(path string, target any) error {
	data, err := relevanceFixtures.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func validateRelevanceCorpus(corpus relevanceCorpus) error {
	if corpus.Now.IsZero() || len(corpus.Products) == 0 || len(corpus.Queries) == 0 {
		return fmt.Errorf("relevance corpus requires a clock, products, and queries")
	}
	ids := map[uint]bool{}
	for _, product := range corpus.Products {
		if product.ID == 0 || ids[product.ID] || product.Name == "" || product.AgeDays < 0 || product.Sales < 0 {
			return fmt.Errorf("invalid relevance product %d", product.ID)
		}
		ids[product.ID] = true
	}
	names := map[string]bool{}
	for _, query := range corpus.Queries {
		if query.Name == "" || names[query.Name] || len(query.Judgments) == 0 {
			return fmt.Errorf("invalid relevance query %q", query.Name)
		}
		names[query.Name] = true
		positive := false
		for id, grade := range query.Judgments {
			if !ids[id] || grade < 0 || grade > 3 {
				return fmt.Errorf("invalid judgment for %s product %d", query.Name, id)
			}
			positive = positive || grade > 0
		}
		if !positive {
			return fmt.Errorf("query %s needs a positive judgment", query.Name)
		}
	}
	return nil
}
