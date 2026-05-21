package commands

import (
	"testing"
	"time"
)

func TestDiscountCampaignScopedPath(t *testing.T) {
	if got := discountCampaignScopedPath("/api/v1/admin/discounts/history", 0); got != "/api/v1/admin/discounts/history" {
		t.Fatalf("expected unscoped path, got %q", got)
	}
	if got := discountCampaignScopedPath("/api/v1/admin/discounts/history", 42); got != "/api/v1/admin/discounts/history?campaign_id=42" {
		t.Fatalf("expected scoped path, got %q", got)
	}
}

func TestExpandStringListDeduplicatesCommaSeparatedValues(t *testing.T) {
	got := expandStringList([]string{"web, app", "web", " admin "})
	want := []string{"web", "app", "admin"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestParseCLITimeRequiresRFC3339(t *testing.T) {
	got, err := parseCLITime("2026-05-20T12:30:00Z", "starts-at")
	if err != nil {
		t.Fatalf("expected valid RFC3339 time: %v", err)
	}
	if got.Format(time.RFC3339) != "2026-05-20T12:30:00Z" {
		t.Fatalf("unexpected parsed time %s", got.Format(time.RFC3339))
	}

	if _, err := parseCLITime("2026-05-20 12:30", "starts-at"); err == nil {
		t.Fatal("expected invalid time to fail")
	}
}
