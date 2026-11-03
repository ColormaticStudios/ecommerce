package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/jobs"
	"ecommerce/internal/migrations"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type searchCapacityMeasurement struct {
	Catalog           int         `json:"catalog"`
	Case              string      `json:"case"`
	Mode              string      `json:"mode"`
	Concurrency       int         `json:"concurrency"`
	Requests          int         `json:"requests"`
	P50MS             float64     `json:"p50_ms"`
	P95MS             float64     `json:"p95_ms"`
	P99MS             float64     `json:"p99_ms"`
	BytesPerRequest   uint64      `json:"bytes_per_request"`
	QueriesPerRequest float64     `json:"queries_per_request"`
	MaxItems          int         `json:"max_items"`
	Statuses          map[int]int `json:"statuses"`
	Degraded          int         `json:"degraded"`
}

// Explicit opt-in keeps capacity work out of normal integration tests. Each
// dataset owns a new schema and uses the registered HTTP boundary over TCP.
// Cold means fresh application services, NOT cold PostgreSQL/OS buffers.
func TestPostgresSearchHTTPCapacity(t *testing.T) {
	if os.Getenv("SEARCH_TEST_CAPACITY") != "1" {
		t.Skip("set SEARCH_TEST_CAPACITY=1 and SEARCH_TEST_POSTGRES_DSN for isolated HTTP capacity measurements")
	}
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	require.NotEmpty(t, dsn)
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	bootstrapSQL, err := bootstrap.DB()
	require.NoError(t, err)
	defer bootstrapSQL.Close()
	var postgresVersion string
	require.NoError(t, bootstrap.Raw("SHOW server_version").Scan(&postgresVersion).Error)
	t.Logf("capacity environment: go=%s cpus=%d gomaxprocs=%d postgres=%s", runtime.Version(), runtime.NumCPU(), runtime.GOMAXPROCS(0), postgresVersion)
	report := []searchCapacityMeasurement{}
	sizes, requests := []int{10000, 25000}, 30
	profile := os.Getenv("SEARCH_CAPACITY_PROFILE") == "1"
	if profile {
		sizes, requests = []int{25000}, 1
	}
	for _, size := range sizes {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			schema := os.Getenv(fmt.Sprintf("SEARCH_CAPACITY_SCHEMA_%d", size))
			reuse := schema != ""
			if profile {
				require.True(t, reuse, "profiling requires an existing isolated capacity schema")
			}
			if reuse {
				require.Regexp(t, regexp.MustCompile(fmt.Sprintf(`^search_capacity_%d_[0-9]+$`, size)), schema, "never profile/reuse an application schema")
			} else {
				schema = fmt.Sprintf("search_capacity_%d_%d", size, time.Now().UnixNano())
				require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
				if os.Getenv("SEARCH_CAPACITY_RETAIN_SCHEMA") == "1" {
					t.Logf("retained diagnostic schema (caller must remove): %s", schema)
				} else {
					defer func() { require.NoError(t, bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema)).Error) }()
				}
			}
			parsed, err := url.Parse(dsn)
			require.NoError(t, err)
			params := parsed.Query()
			params.Set("search_path", schema)
			parsed.RawQuery = params.Encode()
			db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(12)
			sqlDB.SetMaxIdleConns(12)
			defer sqlDB.Close()
			if !reuse {
				require.NoError(t, migrations.Run(db))
				seedSearchCapacityCatalog(t, db, size)
				require.NoError(t, db.Exec("ANALYZE").Error)
			}
			if profile {
				db.Logger = logger.Default.LogMode(logger.Info)
			}
			var queries atomic.Int64
			count := func(*gorm.DB) { queries.Add(1) }
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("capacity_query_count", count))
			require.NoError(t, db.Callback().Row().Before("gorm:row").Register("capacity_row_count", count))
			newServer := func() *httptest.Server {
				endpoints, err := httpapi.NewCatalogEndpoints(db, nil, jobs.NewRuntime(db, jobs.Config{}))
				require.NoError(t, err)
				require.NoError(t, endpoints.ConfigureAnalyticsSigningKey([]byte("capacity-test-signing-key")))
				admission := httpapi.DefaultSearchAdmissionConfig()
				admission.ClientRequestsPerMinute = 60000
				require.NoError(t, endpoints.ConfigureSearchAdmission(admission))
				policies, err := httpapi.ContractPolicySet()
				require.NoError(t, err)
				gin.SetMode(gin.TestMode)
				router := gin.New()
				require.NoError(t, httpapi.RegisterStrict(router, &httpapi.Server{CatalogEndpoints: endpoints}, httpapi.RegisterStrictOptions{Strict: httpapi.StrictOptions{Policies: policies}}))
				return httptest.NewServer(router)
			}
			cases := []struct{ name, path string }{{"broad", "/api/v1/search/products?q=trail&limit=20"}, {"typo", "/api/v1/search/products?q=trali&limit=20"}, {"mixed_facets", "/api/v1/search/products?q=shoe&brand_slug=brand1&brand_slug=brand5&category_slug=category1&category_slug=category5&attribute%5Bcolor%5D%5B0%5D=blue&limit=20"}, {"deep_page", fmt.Sprintf("/api/v1/search/products?q=trail&page=%d&limit=20", size/40)}, {"suggest", "/api/v1/search/suggestions?q=trali&limit=10"}}
			client := &http.Client{Timeout: 10 * time.Second}
			defer client.CloseIdleConnections()
			for _, c := range cases {
				for _, mode := range []string{"cold", "warm"} {
					server := newServer()
					if mode == "warm" {
						r, err := client.Get(server.URL + c.path)
						require.NoError(t, err)
						_, err = io.Copy(io.Discard, r.Body)
						r.Body.Close()
						require.NoError(t, err)
					}
					measurement := searchCapacityMeasurement{Catalog: size, Case: c.name, Mode: mode, Concurrency: 1, Requests: requests, Statuses: map[int]int{}}
					durations := make([]time.Duration, 0, requests)
					var allocated uint64
					queryStart := queries.Load()
					for i := 0; i < requests; i++ {
						if mode == "cold" {
							server.Close()
							server = newServer()
						}
						var before, after runtime.MemStats
						runtime.ReadMemStats(&before)
						started := time.Now()
						r, err := client.Get(server.URL + c.path)
						require.NoError(t, err)
						body, err := io.ReadAll(r.Body)
						r.Body.Close()
						require.NoError(t, err)
						durations = append(durations, time.Since(started))
						runtime.ReadMemStats(&after)
						allocated += after.TotalAlloc - before.TotalAlloc
						measurement.Statuses[r.StatusCode]++
						if c.name != "suggest" && r.StatusCode == 200 {
							var result apicontract.ProductSearchResponse
							require.NoError(t, json.Unmarshal(body, &result))
							for _, product := range result.Items {
								require.NotEqual(t, 7, product.Id, "active hide rule")
								require.Len(t, product.Variants, 2, "only published variants")
								for _, variant := range product.Variants {
									require.True(t, variant.IsPublished)
									require.Nil(t, variant.UnitCost, "storefront cost privacy")
								}
							}
							if c.name == "broad" {
								counts := map[string]int{}
								for _, facet := range result.Facets {
									counts[facet.Name] = len(facet.Values)
								}
								require.Equal(t, 100, counts["category"])
								require.Equal(t, 20, counts["brand"])
								for _, slug := range []string{"color", "material", "style", "size"} {
									require.Equal(t, 25, counts["attribute:"+slug])
								}
							}
							measurement.MaxItems = max(measurement.MaxItems, len(result.Items))
							if result.Metadata.Degraded {
								measurement.Degraded++
							}
							if c.name == "broad" || c.name == "typo" || c.name == "deep_page" {
								require.Equal(t, size-1, result.Pagination.Total, "exact totals, including active hide rule")
							}
						} else {
							require.Equal(t, 200, r.StatusCode, string(body))
						}
					}
					server.Close()
					measurement.BytesPerRequest = allocated / uint64(requests)
					measurement.QueriesPerRequest = float64(queries.Load()-queryStart) / float64(requests)
					finishSearchCapacityMeasurement(&measurement, durations)
					report = append(report, measurement)
					t.Logf("%+v", measurement)
					require.Equal(t, requests, measurement.Statuses[200])
					require.Zero(t, measurement.Degraded)
					require.LessOrEqual(t, measurement.MaxItems, 20)
				}
			}
			if profile {
				return
			}
			// Four same-client requests exercise the default per-client concurrency bound
			// without disabling admission or hiding capacity failures in latency samples.
			server := newServer()
			defer server.Close()
			measurement := searchCapacityMeasurement{Catalog: size, Case: "mixed_query_load", Mode: "warm", Concurrency: 4, Requests: 60, Statuses: map[int]int{}}
			durations := []time.Duration{}
			caseDurations := map[string][]time.Duration{}
			var mu sync.Mutex
			var wg sync.WaitGroup
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			queryStart := queries.Load()
			for worker := 0; worker < 4; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					for i := 0; i < 15; i++ {
						c := cases[(worker+i)%len(cases)]
						started := time.Now()
						r, err := client.Get(server.URL + c.path)
						if err != nil {
							t.Errorf("capacity HTTP request: %v", err)
							continue
						}
						body, err := io.ReadAll(r.Body)
						r.Body.Close()
						if err != nil {
							t.Errorf("capacity HTTP body: %v", err)
							continue
						}
						duration := time.Since(started)
						var result apicontract.ProductSearchResponse
						if c.name != "suggest" && r.StatusCode == 200 {
							if err := json.Unmarshal(body, &result); err != nil {
								t.Errorf("capacity JSON: %v", err)
							}
						}
						mu.Lock()
						durations = append(durations, duration)
						caseDurations[c.name] = append(caseDurations[c.name], duration)
						measurement.Statuses[r.StatusCode]++
						measurement.MaxItems = max(measurement.MaxItems, len(result.Items))
						if result.Metadata.Degraded {
							measurement.Degraded++
						}
						mu.Unlock()
					}
				}(worker)
			}
			wg.Wait()
			runtime.ReadMemStats(&after)
			measurement.BytesPerRequest = (after.TotalAlloc - before.TotalAlloc) / 60
			measurement.QueriesPerRequest = float64(queries.Load()-queryStart) / 60
			finishSearchCapacityMeasurement(&measurement, durations)
			for _, c := range cases {
				diagnostic := searchCapacityMeasurement{Catalog: size, Case: c.name, Mode: "concurrent_diagnostic", Concurrency: 4, Requests: len(caseDurations[c.name])}
				finishSearchCapacityMeasurement(&diagnostic, caseDurations[c.name])
				t.Logf("concurrent workload distribution: %+v", diagnostic)
			}
			report = append(report, measurement)
			t.Logf("%+v", measurement)
			require.Equal(t, 60, measurement.Statuses[200])
			require.Zero(t, measurement.Degraded)
			require.LessOrEqual(t, measurement.MaxItems, 20)
		})
	}
	output, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	if path := os.Getenv("SEARCH_CAPACITY_REPORT"); path != "" {
		require.NoError(t, os.WriteFile(path, append(output, '\n'), 0600))
	}
	t.Log(string(output))
	if profile {
		t.Log("single-request diagnostic only; not a capacity/SLO gate")
		return
	}
	// This gate applies to real serialized responses and includes all statuses;
	// no averages, removed outliers, degraded successes, or candidate caps.
	for _, measurement := range report {
		require.LessOrEqual(t, measurement.P95MS, float64(500), "capacity gate: %+v", measurement)
	}
}

func finishSearchCapacityMeasurement(m *searchCapacityMeasurement, durations []time.Duration) {
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	if len(durations) == 0 {
		return
	}
	at := func(percent float64) float64 {
		return float64(durations[max(0, int(math.Ceil(percent*float64(len(durations))))-1)]) / float64(time.Millisecond)
	}
	m.P50MS = at(.5)
	m.P95MS = at(.95)
	m.P99MS = at(.99)
}

func seedSearchCapacityCatalog(t *testing.T, db *gorm.DB, size int) {
	t.Helper()
	var locale models.Locale
	require.NoError(t, db.Where("is_default = ?", true).First(&locale).Error)
	now := time.Now().UTC()
	brands := []models.Brand{}
	categories := []models.Category{}
	localized := []models.LocalizedEntityValue{}
	for i := 0; i < 20; i++ {
		brands = append(brands, models.Brand{Name: fmt.Sprintf("Brand %d", i), Slug: fmt.Sprintf("brand%d", i), IsActive: true})
	}
	require.NoError(t, db.CreateInBatches(&brands, 100).Error)
	for i := 0; i < 100; i++ {
		categories = append(categories, models.Category{Name: fmt.Sprintf("Category %d", i), Slug: fmt.Sprintf("category%d", i), IsActive: true})
	}
	require.NoError(t, db.CreateInBatches(&categories, 100).Error)
	for _, b := range brands {
		localized = append(localized, models.LocalizedEntityValue{EntityType: "brand", EntityID: b.ID, LocaleID: locale.ID, Field: "name", Value: b.Name})
	}
	for _, c := range categories {
		localized = append(localized, models.LocalizedEntityValue{EntityType: "category", EntityID: c.ID, LocaleID: locale.ID, Field: "name", Value: c.Name})
	}
	definitions := []models.ProductAttribute{
		{Slug: "color", Key: "Color", Filterable: true},
		{Slug: "material", Key: "Material", Filterable: true},
		{Slug: "style", Key: "Style", Filterable: true},
		{Slug: "size", Key: "Size", Filterable: true},
	}
	require.NoError(t, db.Create(&definitions).Error)
	for _, definition := range definitions {
		localized = append(localized, models.LocalizedEntityValue{EntityType: "product_attribute", EntityID: definition.ID, LocaleID: locale.ID, Field: "key", Value: definition.Key})
	}
	products := make([]models.Product, 0, size)
	variants := make([]models.ProductVariant, 0, size*3)
	attributes := make([]models.ProductAttributeValue, 0, size*len(definitions))
	links := make([]struct{ ProductID, CategoryID uint }, 0, size)
	for i := 1; i <= size; i++ {
		color := fmt.Sprintf("color-%02d", i%25)
		if i%25 < 4 {
			color = []string{"red", "blue", "green", "black"}[i%25]
		}
		brandID := brands[i%20].ID
		price := models.MoneyFromFloat(float64(10 + i%200))
		cost := models.MoneyFromFloat(price.Float64() * .6)
		p := models.Product{BaseModel: models.BaseModel{ID: uint(i), CreatedAt: now.Add(-time.Duration(i%60) * 24 * time.Hour), UpdatedAt: now}, SKU: fmt.Sprintf("capacity-%d", i), Name: fmt.Sprintf("%s Trail Shoe %d", color, i), Description: "waterproof hiking footwear durable lightweight", Price: price, Stock: i % 7, IsPublished: true, BrandID: &brandID}
		products = append(products, p)
		links = append(links, struct{ ProductID, CategoryID uint }{uint(i), categories[i%len(categories)].ID})
		for index, definition := range definitions {
			value := color
			if index > 0 {
				value = fmt.Sprintf("%s-%02d", definition.Slug, (i*[]int{1, 3, 7, 11}[index]+index*7)%25)
			}
			attributes = append(attributes, models.ProductAttributeValue{ProductID: uint(i), ProductAttributeID: definition.ID, TextValue: &value})
		}
		localized = append(localized, models.LocalizedEntityValue{EntityType: "product", EntityID: uint(i), LocaleID: locale.ID, Field: "name", Value: p.Name}, models.LocalizedEntityValue{EntityType: "product", EntityID: uint(i), LocaleID: locale.ID, Field: "description", Value: p.Description})
		for v := 0; v < 3; v++ {
			variants = append(variants, models.ProductVariant{ProductID: uint(i), SKU: fmt.Sprintf("capacity-%d-%d", i, v), Title: fmt.Sprintf("Size %d", v), Price: price + models.Money(v*100), UnitCost: &cost, Stock: i % 7, IsPublished: v < 2})
		}
	}
	require.NoError(t, db.Omit("Brand", "Categories", "Variants", "AttributeValues", "Related").CreateInBatches(&products, 500).Error)
	require.NoError(t, db.Table("product_categories").CreateInBatches(&links, 1000).Error)
	require.NoError(t, db.CreateInBatches(&attributes, 500).Error)
	require.NoError(t, db.CreateInBatches(&variants, 500).Error)
	require.NoError(t, db.Model(&models.ProductVariant{}).Where("sku LIKE ?", "%-2").Update("is_published", false).Error)
	for _, v := range variants {
		localized = append(localized, models.LocalizedEntityValue{EntityType: "product_variant", EntityID: v.ID, LocaleID: locale.ID, Field: "title", Value: v.Title})
	}
	require.NoError(t, db.CreateInBatches(&localized, 500).Error)
	sales := make([]models.SearchSalesSignal, 0, size/3)
	conversions := make([]models.SearchConversionSignal, 0, size/3)
	for i := 3; i <= size; i += 3 {
		sales = append(sales, models.SearchSalesSignal{ProductID: uint(i), Units30Days: int64(i % 80), AsOf: now, UpdatedAt: now})
		conversions = append(conversions, models.SearchConversionSignal{ProductID: uint(i), Impressions30Days: int64(40 + i%120), Conversions30Days: int64(i % 10), AsOf: now, UpdatedAt: now})
	}
	require.NoError(t, db.CreateInBatches(&sales, 500).Error)
	require.NoError(t, db.CreateInBatches(&conversions, 500).Error)
	service := searchservice.NewService(db, nil, nil)
	require.NoError(t, service.Reindex(context.Background()))
	_, err := service.CreateMerchandisingRule(context.Background(), searchservice.MerchandisingRuleInput{Name: "Capacity hide", RuleType: "hide", IsActive: true, Predicate: searchservice.MerchandisingPredicate{}, Action: searchservice.MerchandisingAction{Targets: []searchservice.MerchandisingTarget{{ProductID: 7}}}}, nil)
	require.NoError(t, err)
}
