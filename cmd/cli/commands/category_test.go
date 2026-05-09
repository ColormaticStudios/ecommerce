package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCategoryInputFlagsToContractSetsChangedOptionalFields(t *testing.T) {
	cmd := newCreateCategoryCmd()
	flags := categoryInputFlags{
		name:        " Apparel ",
		slug:        " outerwear ",
		description: " Jackets ",
		parentID:    42,
		sortOrder:   7,
		isActive:    false,
	}
	mustSetFlag(t, cmd, "slug", flags.slug)
	mustSetFlag(t, cmd, "description", flags.description)
	mustSetFlag(t, cmd, "parent-id", "42")
	mustSetFlag(t, cmd, "sort-order", "7")
	mustSetFlag(t, cmd, "is-active", "false")

	payload := flags.toContract(cmd)
	if payload.Name != "Apparel" {
		t.Fatalf("expected trimmed name, got %q", payload.Name)
	}
	assertStringPtr(t, payload.Slug, "outerwear")
	assertStringPtr(t, payload.Description, "Jackets")
	assertIntPtr(t, payload.ParentId, 42)
	assertIntPtr(t, payload.SortOrder, 7)
	if payload.IsActive == nil || *payload.IsActive {
		t.Fatalf("expected is_active false, got %+v", payload.IsActive)
	}
}

func TestCategoryInputFlagsToContractOmitsUnchangedOptionalFields(t *testing.T) {
	cmd := newCreateCategoryCmd()
	payload := categoryInputFlags{name: "Apparel"}.toContract(cmd)
	if payload.Slug != nil || payload.Description != nil || payload.ParentId != nil || payload.SortOrder != nil || payload.IsActive != nil {
		t.Fatalf("expected optional fields to be omitted, got %+v", payload)
	}
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name string, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

func assertStringPtr(t *testing.T, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("expected %q, got %+v", want, got)
	}
}

func assertIntPtr(t *testing.T, got *int, want int) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("expected %d, got %+v", want, got)
	}
}
