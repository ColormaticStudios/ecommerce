// search-load runs a disposable local projection workload, never a configured
// application database. It is a diagnostic rather than an SLO assertion.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"ecommerce/internal/search"
	"ecommerce/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	count := flag.Int("products", 10000, "disposable published products")
	workers := flag.Int("concurrency", 50, "concurrent searches")
	requests := flag.Int("requests", 100, "measured searches per cold/warm wave")
	representative := flag.Bool("representative", false, "100 categories,20 brands,4 attributes with25values,2variantsperproduct")
	flag.Parse()
	if *count < 1 || *count > 100000 || *workers < 1 || *workers > 512 || *requests < *workers {
		return fmt.Errorf("invalid workload")
	}
	file, err := os.CreateTemp("", "ecommerce-search-load-*.sqlite")
	if err != nil {
		return err
	}
	path := file.Name()
	file.Close()
	defer os.Remove(path)
	defer os.Remove(path + "-wal")
	defer os.Remove(path + "-shm")
	db, err := gorm.Open(sqlite.Open(path+"?_journal_mode=WAL&_busy_timeout=10000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(*workers)
	if err = db.AutoMigrate(&models.Product{}, &models.ProductVariant{}, &models.Brand{}, &models.Category{}, &models.ProductCategory{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SearchDocument{}, &models.SearchIndexState{}, &models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchRankingProfile{}, &models.SearchSalesSignal{}, &models.SearchConversionSignal{}, &models.SearchMerchandisingRule{}, &models.SearchMerchandisingAudit{}); err != nil {
		return err
	}
	svc := search.NewService(db, nil, nil)
	cfg := search.DefaultHardeningConfig()
	cfg.MaxConcurrent = *workers
	cfg.SearchTimeoutMS = 60000
	if err = svc.ConfigureHardening(cfg); err != nil {
		return err
	}
	if _, err = svc.CreateRankingProfile(context.Background(), search.RankingProfileInput{Name: "default", Weights: search.DefaultRankingWeights(), IsDefault: true}, nil); err != nil {
		return err
	}
	if _, err = svc.CreateTypoToleranceProfile(context.Background(), search.TypoToleranceProfileInput{Name: "default", MinimumTokenLength: 3, OneEditMinimumLength: 4, TwoEditMinimumLength: 7, IsActive: true}); err != nil {
		return err
	}
	for start := 0; start < *count; start += 100 {
		products := make([]models.Product, 0, 100)
		for i := start; i < min(start+100, *count); i++ {
			products = append(products, models.Product{Name: fmt.Sprintf("Load item %05d", i), SKU: fmt.Sprintf("LOAD-%05d", i), Description: "Synthetic catalog for disposable search capacity measurement", Price: models.MoneyFromFloat(float64(1 + i%500)), Stock: 10, IsPublished: true})
		}
		if err = db.Create(&products).Error; err != nil {
			return err
		}
	}
	if *representative {
		if err = seedRepresentative(db, *count); err != nil {
			return err
		}
	}
	indexing := time.Now()
	if err = svc.Reindex(context.Background()); err != nil {
		return err
	}
	indexDuration := time.Since(indexing)
	type sample struct {
		DurationMS float64
		Error      string
		Degraded   bool
		Kind       string
		Wave       string
	}
	samples := make([]sample, *requests*2)
	queue := make(chan int)
	var wg, waveWG sync.WaitGroup
	started := time.Now()
	for worker := 0; worker < *workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				query := fmt.Sprintf("load item %05d", (i%*requests)%*count)
				kind := "specific"
				if i%5 == 0 {
					query = "load item"
					kind = "broad"
				}
				at := time.Now()
				result, err := svc.Search(context.Background(), search.Filters{Query: query, Limit: 20})
				samples[i] = sample{DurationMS: float64(time.Since(at).Microseconds()) / 1000, Degraded: result.Degraded, Kind: kind, Wave: map[bool]string{true: "cold", false: "warm"}[i < *requests]}
				if err != nil {
					samples[i].Error = err.Error()
				}
				waveWG.Done()
			}
		}()
	}
	for wave := 0; wave < 2; wave++ {
		waveWG.Add(*requests)
		for i := wave * (*requests); i < (wave+1)*(*requests); i++ {
			queue <- i
		}
		waveWG.Wait()
	}
	close(queue)
	wg.Wait()
	elapsed := time.Since(started)
	durations := make([]float64, 0, *requests)
	errors, degraded := 0, 0
	for _, s := range samples {
		durations = append(durations, s.DurationMS)
		if s.Error != "" {
			errors++
		}
		if s.Degraded {
			degraded++
		}
	}
	sort.Float64s(durations)
	percentile := func(p float64) float64 { return durations[min(int(float64(len(durations))*p), len(durations)-1)] }
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	waveStats := map[string]any{}
	for _, wave := range []string{"cold", "warm"} {
		times := []float64{}
		for _, sample := range samples {
			if sample.Wave == wave {
				times = append(times, sample.DurationMS)
			}
		}
		sort.Float64s(times)
		waveStats[wave] = map[string]float64{"p50_ms": times[min(len(times)/2, len(times)-1)], "p95_ms": times[min(int(float64(len(times))*.95), len(times)-1)], "p99_ms": times[min(int(float64(len(times))*.99), len(times)-1)]}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"fixture": map[bool]string{true: "100 categories;20brands;4filterableattributes/25values;1published+1unpublishedvariantperproduct;costs", false: "minimal products only"}[*representative], "wave_latencies": waveStats, "backend": "sqlite local WAL; service-level not HTTP", "products": *count, "concurrency": *workers, "requests": *requests * 2, "requests_per_wave": *requests, "waves": []string{"cold", "warm"}, "logical_cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "go_version": runtime.Version(), "index_duration_ms": float64(indexDuration.Milliseconds()), "elapsed_ms": float64(elapsed.Milliseconds()), "p50_ms": percentile(.5), "p95_ms": percentile(.95), "p99_ms": percentile(.99), "throughput_requests_second": float64(*requests*2) / elapsed.Seconds(), "errors": errors, "degraded": degraded, "heap_sys_mb": float64(memory.HeapSys) / (1024 * 1024), "samples": samples})
}

func seedRepresentative(db *gorm.DB, count int) error {
	return db.Transaction(func(db *gorm.DB) error {
		brands := make([]models.Brand, 20)
		for i := range brands {
			brands[i] = models.Brand{Name: fmt.Sprintf("Loadbrand%02d", i), Slug: fmt.Sprintf("loadbrand-%02d", i), IsActive: true}
		}
		if err := db.Create(&brands).Error; err != nil {
			return err
		}
		for i, brand := range brands {
			if err := db.Model(&models.Product{}).Where("id % 20 = ?", i).Update("brand_id", brand.ID).Error; err != nil {
				return err
			}
		}
		categories := make([]models.Category, 100)
		for i := range categories {
			categories[i] = models.Category{Name: fmt.Sprintf("Loadcategory%03d", i), Slug: fmt.Sprintf("loadcategory-%03d", i), Path: fmt.Sprintf("loadcategory-%03d", i), IsActive: true}
		}
		if err := db.Create(&categories).Error; err != nil {
			return err
		}
		attributes := make([]models.ProductAttribute, 4)
		for i := range attributes {
			attributes[i] = models.ProductAttribute{Key: fmt.Sprintf("Loadattribute%d", i), Slug: fmt.Sprintf("loadattribute-%d", i), Type: "text", Filterable: true}
		}
		if err := db.Create(&attributes).Error; err != nil {
			return err
		}
		for start := 0; start < count; start += 100 {
			joins := []models.ProductCategory{}
			values := []models.ProductAttributeValue{}
			variants := []models.ProductVariant{}
			for i := start; i < min(start+100, count); i++ {
				id := uint(i + 1)
				joins = append(joins, models.ProductCategory{ProductID: id, CategoryID: categories[i%100].ID})
				for a, attribute := range attributes {
					value := fmt.Sprintf("choice-%02d", (i+a)%25)
					values = append(values, models.ProductAttributeValue{ProductID: id, ProductAttributeID: attribute.ID, TextValue: &value, Position: a + 1})
				}
				price := models.MoneyFromFloat(float64(1 + i%500))
				cost := models.MoneyFromFloat(price.Float64() * .6)
				variants = append(variants, models.ProductVariant{ProductID: id, SKU: fmt.Sprintf("LOADVARIANT-%05d", i), Title: "Published synthetic variant", Price: price, UnitCost: &cost, Stock: i % 10, IsPublished: true}, models.ProductVariant{ProductID: id, SKU: fmt.Sprintf("LOADDRAFTVARIANT-%05d", i), Title: "Unpublished synthetic variant", Price: price, UnitCost: nil, Stock: 0, IsPublished: true})
			}
			if err := db.Create(&joins).Error; err != nil {
				return err
			}
			if err := db.CreateInBatches(&values, 100).Error; err != nil {
				return err
			}
			if err := db.CreateInBatches(&variants, 100).Error; err != nil {
				return err
			}
			if err := db.Model(&models.ProductVariant{}).Where("sku LIKE ?", "LOADDRAFTVARIANT-%").Update("is_published", false).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
