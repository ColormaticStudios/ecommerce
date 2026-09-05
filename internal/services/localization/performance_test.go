//go:build performance

package localization

import (
	"context"
	"testing"
	"time"

	"ecommerce/models"
)

func newLocalizationPerformanceService(b testing.TB) *Service {
	b.Helper()
	service, _ := newLocalizationTestService(b)
	configureTestLocales(b, service)
	key, err := service.CreateKey(context.Background(), KeyInput{Namespace: "storefront", Key: "performance.message", SourceText: "Hello {name}", OwnerDomain: "storefront"})
	if err != nil {
		b.Fatal(err)
	}
	value, err := service.CreateValue(context.Background(), key.ID, "en-US", "Hello {name}", nil)
	if err != nil {
		b.Fatal(err)
	}
	value, err = service.TransitionValue(context.Background(), value.ID, models.TranslationStateReview, nil)
	if err != nil {
		b.Fatal(err)
	}
	if _, err = service.TransitionValue(context.Background(), value.ID, models.TranslationStatePublished, nil); err != nil {
		b.Fatal(err)
	}
	release, err := service.CreateRelease(context.Background(), "performance", "")
	if err != nil {
		b.Fatal(err)
	}
	if _, err = service.ActivateRelease(context.Background(), release.ID, nil); err != nil {
		b.Fatal(err)
	}
	return service
}

func TestWarmBundleLookupPerformanceBudget(t *testing.T) {
	service := newLocalizationPerformanceService(t)
	input := ResolutionInput{ExplicitLocale: "en-US"}
	if _, err := service.Bundle(context.Background(), input, "storefront"); err != nil {
		t.Fatal(err)
	}
	const iterations = 100
	started := time.Now()
	for range iterations {
		if _, err := service.Bundle(context.Background(), input, "storefront"); err != nil {
			t.Fatal(err)
		}
	}
	average := time.Since(started) / iterations
	if average >= time.Millisecond {
		t.Fatalf("warm bundle lookup average %s exceeds 1ms budget", average)
	}
}

func BenchmarkWarmBundleLookup(b *testing.B) {
	service := newLocalizationPerformanceService(b)
	input := ResolutionInput{ExplicitLocale: "en-US"}
	_, _ = service.Bundle(context.Background(), input, "storefront")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = service.Bundle(context.Background(), input, "storefront")
	}
}
