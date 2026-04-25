package commands

import "testing"

func TestInventoryListPathNormalizesStatusesAndLimit(t *testing.T) {
	path := inventoryListPath("/api/v1/admin/inventory/alerts", []string{"open, acked", " resolved "}, 25, 100)

	expected := "/api/v1/admin/inventory/alerts?limit=25&status=OPEN&status=ACKED&status=RESOLVED"
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestInventoryListPathOmitsDefaultLimitAndEmptyStatuses(t *testing.T) {
	path := inventoryListPath("/api/v1/admin/inventory/reservations", []string{"", "  , "}, 100, 100)

	expected := "/api/v1/admin/inventory/reservations"
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestThresholdVariantLabel(t *testing.T) {
	if got := thresholdVariantLabel(nil); got != "default" {
		t.Fatalf("expected default threshold label, got %q", got)
	}

	id := 42
	if got := thresholdVariantLabel(&id); got != "42" {
		t.Fatalf("expected variant threshold label, got %q", got)
	}
}

func TestInventoryAlertActionPastTense(t *testing.T) {
	if got := inventoryAlertActionPastTense("ack"); got != "acked" {
		t.Fatalf("expected acked, got %q", got)
	}
	if got := inventoryAlertActionPastTense("resolve"); got != "resolved" {
		t.Fatalf("expected resolved, got %q", got)
	}
}
