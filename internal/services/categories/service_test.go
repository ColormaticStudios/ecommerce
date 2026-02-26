package categories

import "testing"

func TestNormalizeSlug(t *testing.T) {
	if got := NormalizeSlug("  Men's Shoes  "); got != "mens-shoes" {
		t.Fatalf("expected mens-shoes, got %q", got)
	}
	if got := NormalizeSlug("Summer__Sale!!!"); got != "summer-sale" {
		t.Fatalf("expected summer-sale, got %q", got)
	}
}

func TestIsValidName(t *testing.T) {
	if IsValidName("A") {
		t.Fatal("single-character name should be invalid")
	}
	if !IsValidName("Accessories") {
		t.Fatal("expected Accessories to be valid")
	}
}
