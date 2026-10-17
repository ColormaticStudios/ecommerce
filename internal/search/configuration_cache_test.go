package search

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConfigurationCacheTTLAndIndependentCopies(t *testing.T) {
	ctx := context.Background()
	cache := newConfigurationCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	value, calls := "original", 0
	load := func() ([]MerchandisingRule, error) {
		calls++
		position := 1
		return []MerchandisingRule{{MerchandisingRuleInput: MerchandisingRuleInput{Predicate: MerchandisingPredicate{Query: &MerchandisingQuery{Value: value}, CategorySlugs: []string{"shoes"}}, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: 1, Position: &position}}}}}}, nil
	}
	first, err := cachedConfiguration(ctx, cache, merchandisingConfiguration, load)
	require.NoError(t, err)
	first[0].Predicate.Query.Value = "mutated"
	first[0].Predicate.CategorySlugs[0] = "mutated"
	*first[0].Action.Targets[0].Position = 99
	value = "new"
	second, err := cachedConfiguration(ctx, cache, merchandisingConfiguration, load)
	require.NoError(t, err)
	require.Equal(t, "original", second[0].Predicate.Query.Value)
	require.Equal(t, "shoes", second[0].Predicate.CategorySlugs[0])
	require.Equal(t, 1, *second[0].Action.Targets[0].Position)
	require.Equal(t, 1, calls)
	now = now.Add(configurationCacheTTL)
	third, err := cachedConfiguration(ctx, cache, merchandisingConfiguration, load)
	require.NoError(t, err)
	require.Equal(t, "new", third[0].Predicate.Query.Value)
	require.Equal(t, 2, calls)
}

func TestConfigurationCacheProductionSearchSkipsWarmConfigurationQueries(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db).(*databaseBackend)
	counts := map[string]int{}
	configurationTables := map[string]bool{"search_synonym_sets": true, "search_typo_tolerance_profiles": true, "search_ranking_profiles": true, "search_merchandising_rules": true}
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:configuration_queries", func(tx *gorm.DB) {
		if configurationTables[tx.Statement.Table] {
			counts[tx.Statement.Table]++
		}
	}))
	defer db.Callback().Query().Remove("test:configuration_queries")
	_, err := backend.Search(ctx, Filters{Query: "shoe"})
	require.NoError(t, err)
	for table := range configurationTables {
		require.Equal(t, 1, counts[table], table)
	}
	_, err = backend.Search(ctx, Filters{Query: "boot"})
	require.NoError(t, err)
	for table := range configurationTables {
		require.Equal(t, 1, counts[table], "warm configuration query: %s", table)
	}
	// Named profile misses do not allocate shopper-controlled entries or read DB.
	for _, name := range []string{"missing-one", "missing-two", "missing-three"} {
		_, err := backend.loadRankingProfile(ctx, name)
		require.ErrorIs(t, err, ErrConfigurationNotFound)
	}
	require.Equal(t, 1, counts["search_ranking_profiles"])
}

func TestConfigurationCacheRollbackPreservesCommittedSnapshot(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db).(*databaseBackend)
	service := NewService(db, backend, nil)
	input := SynonymSetInput{Name: "rollback", Direction: "bi", Terms: []string{"shoe", "boot"}, IsActive: true}
	set, err := service.CreateSynonymSet(ctx, input)
	require.NoError(t, err)
	_, err = backend.loadSynonymRules(ctx)
	require.NoError(t, err)
	generation := backend.configuration.entries[synonymConfiguration].generation
	failed := errors.New("audit write failed")
	err = service.configurationTransaction(db.WithContext(ctx), synonymConfiguration, func(tx *gorm.DB) error {
		if err := tx.Model(&models.SearchSynonymSet{}).Where("id = ?", set.ID).Update("terms_json", `["shoe","sandal"]`).Error; err != nil {
			return err
		}
		return failed
	})
	require.ErrorIs(t, err, failed)
	require.Equal(t, generation, backend.configuration.entries[synonymConfiguration].generation)
	actual, err := service.GetSynonymSet(ctx, set.ID)
	require.NoError(t, err)
	require.Equal(t, input.Terms, actual.Terms)
	rules, err := backend.loadSynonymRules(ctx)
	require.NoError(t, err)
	require.Equal(t, input.Terms, rules[0].alternatives)
}

func TestConfigurationCachePreviewOverridesAndLiveTargets(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db).(*databaseBackend)
	service := NewService(db, backend, nil)
	product := models.Product{SKU: "cache-preview", Name: "Cache Shoe", IsPublished: true, Variants: []models.ProductVariant{{SKU: "cache-preview-variant", IsPublished: true, Stock: 1}}}
	require.NoError(t, db.Create(&product).Error)
	document, include, err := projectProduct(product, time.Now())
	require.NoError(t, err)
	require.True(t, include)
	require.NoError(t, backend.Upsert(ctx, document))
	_, err = service.CreateMerchandisingRule(ctx, MerchandisingRuleInput{Name: "hide cache shoe", RuleType: "hide", IsActive: true, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: product.ID}}}}, nil)
	require.NoError(t, err)
	filters := Filters{Query: "cache"}
	result, err := backend.Search(ctx, filters)
	require.NoError(t, err)
	require.Zero(t, result.Total)
	noRules := []MerchandisingRule{}
	preview, err := service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: filters, Rules: &noRules})
	require.NoError(t, err)
	require.Equal(t, int64(1), preview.Proposed.Total)
	result, err = backend.Search(ctx, filters)
	require.NoError(t, err)
	require.Zero(t, result.Total, "preview must not mutate saved cache")
	require.NoError(t, db.Model(&product).Update("name", "Renamed live target").Error)
	rules, err := service.ListMerchandisingRules(ctx)
	require.NoError(t, err)
	require.Equal(t, "Renamed live target", rules[0].Action.Targets[0].ProductName)
	// Removing an indexed target is visible immediately, independent of cache TTL.
	require.NoError(t, backend.Delete(ctx, ProductEntityType, product.ID, document.Version+1))
	result, err = backend.Search(ctx, Filters{Query: "cache", Explain: true})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	require.NotEmpty(t, result.RuleDecisions)
	require.Equal(t, "skipped", result.RuleDecisions[0].Outcome)
	require.Contains(t, result.RuleDecisions[0].Reason, "indexed")
	require.Equal(t, "Renamed live target", result.RuleDecisions[0].ProductName)
}

func TestConfigurationCacheFailuresAndBoundsAreNotRetained(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name string
		rows []string
		err  error
	}{
		{"failure", nil, errors.New("database unavailable")},
		{"row bound", make([]string, configurationCacheMaxRows+1), nil},
		{"byte bound", []string{strings.Repeat("a", configurationCacheMaxBytes)}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			cache, calls := newConfigurationCache(), 0
			load := func() ([]string, error) { calls++; return test.rows, test.err }
			for i := 0; i < 2; i++ {
				rows, err := cachedConfiguration(ctx, cache, synonymConfiguration, load)
				require.Equal(t, test.err, err)
				require.Equal(t, test.rows, rows)
			}
			require.Equal(t, 2, calls)
		})
	}
}

func TestConfigurationCacheCoalescesLoadsAndCancelsWaiters(t *testing.T) {
	cache := newConfigurationCache()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	load := func() ([]string, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return []string{"saved"}, nil
	}
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			rows, err := cachedConfiguration(context.Background(), cache, synonymConfiguration, load)
			if err != nil || len(rows) != 1 || rows[0] != "saved" {
				t.Errorf("unexpected result: %v, %v", rows, err)
			}
		}()
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := cachedConfiguration(ctx, cache, synonymConfiguration, load)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	close(release)
	group.Wait()
	require.Equal(t, int32(1), calls.Load())
}

func TestConfigurationCacheInvalidationDuringLoadCannotRestoreOldGeneration(t *testing.T) {
	cache := newConfigurationCache()
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = cachedConfiguration(context.Background(), cache, synonymConfiguration, func() ([]string, error) {
			close(started)
			<-release
			return []string{"old"}, nil
		})
	}()
	<-started
	cache.invalidate(synonymConfiguration)
	close(release)
	<-finished
	rows, err := cachedConfiguration(context.Background(), cache, synonymConfiguration, func() ([]string, error) { return []string{"new"}, nil })
	require.NoError(t, err)
	require.Equal(t, []string{"new"}, rows)
}

func TestConfigurationCacheAdminEditsInvalidateAndOtherBackendExpires(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db).(*databaseBackend)
	other := NewDatabaseBackend(db).(*databaseBackend)
	now := time.Now()
	other.configuration.now = func() time.Time { return now }
	service := NewService(db, backend, nil)
	input := SynonymSetInput{Name: "cache", Direction: "bi", Terms: []string{"shoe", "boot"}, IsActive: true}
	set, err := service.CreateSynonymSet(ctx, input)
	require.NoError(t, err)
	_, err = backend.loadSynonymRules(ctx)
	require.NoError(t, err)
	_, err = other.loadSynonymRules(ctx)
	require.NoError(t, err)
	input.Terms = []string{"shoe", "sandal"}
	_, err = service.UpdateSynonymSet(ctx, set.ID, input)
	require.NoError(t, err)
	rules, err := backend.loadSynonymRules(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"shoe", "sandal"}, rules[0].alternatives)
	rules, err = other.loadSynonymRules(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"shoe", "boot"}, rules[0].alternatives)
	// Admin reads observe committed state even on a backend with a warm cache.
	admin := NewService(db, other, nil)
	actual, err := admin.GetSynonymSet(ctx, set.ID)
	require.NoError(t, err)
	require.Equal(t, input.Terms, actual.Terms)
	now = now.Add(configurationCacheTTL)
	rules, err = other.loadSynonymRules(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"shoe", "sandal"}, rules[0].alternatives)
	require.NoError(t, service.DeleteSynonymSet(ctx, set.ID, nil))
	rules, err = backend.loadSynonymRules(ctx)
	require.NoError(t, err)
	require.Empty(t, rules)

	profile, err := backend.loadTypoToleranceProfile(ctx)
	require.NoError(t, err)
	_, err = service.UpdateTypoToleranceProfile(ctx, profile.ID, TypoToleranceProfileInput{Name: profile.Name, MinimumTokenLength: 5, OneEditMinimumLength: 5, TwoEditMinimumLength: 9, StrictMode: true, IsActive: true})
	require.NoError(t, err)
	profile, err = backend.loadTypoToleranceProfile(ctx)
	require.NoError(t, err)
	require.True(t, profile.StrictMode)
	require.Equal(t, 5, profile.MinimumTokenLength)

	rank, err := backend.loadRankingProfile(ctx, "")
	require.NoError(t, err)
	weights := RankingWeights{Name: 42}
	_, err = service.UpdateRankingProfile(ctx, rank.ID, RankingProfilePatch{Weights: &weights}, nil)
	require.NoError(t, err)
	rank, err = backend.loadRankingProfile(ctx, "")
	require.NoError(t, err)
	require.Equal(t, weights, rank.Weights)

	require.NoError(t, db.Create(&models.Product{BaseModel: models.BaseModel{ID: 999}, SKU: "cache-target", Name: "Live target"}).Error)
	_, err = backend.loadMerchandisingRules(ctx)
	require.NoError(t, err)
	rule, err := service.CreateMerchandisingRule(ctx, MerchandisingRuleInput{Name: "cache rule", RuleType: "hide", IsActive: true, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: 999}}}}, nil)
	require.NoError(t, err)
	saved, err := backend.loadMerchandisingRules(ctx)
	require.NoError(t, err)
	require.Len(t, saved, 1)
	require.Empty(t, saved[0].Action.Targets[0].ProductName)
	active := false
	_, err = service.UpdateMerchandisingRule(ctx, rule.ID, MerchandisingRulePatch{IsActive: &active}, nil)
	require.NoError(t, err)
	saved, err = backend.loadMerchandisingRules(ctx)
	require.NoError(t, err)
	require.False(t, saved[0].IsActive)
	require.NoError(t, service.DeleteMerchandisingRule(ctx, rule.ID, nil))
	saved, err = backend.loadMerchandisingRules(ctx)
	require.NoError(t, err)
	require.Empty(t, saved)
}
