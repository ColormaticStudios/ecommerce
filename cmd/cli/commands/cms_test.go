package commands

import (
	"os"
	"path/filepath"
	"testing"

	"ecommerce/internal/services/cms"
	"ecommerce/models"
)

func TestRootCommandIncludesCMSGroup(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"cms"})
	if err != nil {
		t.Fatalf("find cms command: %v", err)
	}
	if cmd == nil || cmd.Name() != "cms" {
		t.Fatalf("expected cms command, got %#v", cmd)
	}
}

func TestLoadCMSPageInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.json")
	payload := `{
		"path": "/about",
		"title": "About",
		"slug": "about",
		"visibility": "public",
		"is_homepage": false,
		"payload": {
			"blocks": [
				{"type": "rich_text", "body": "About us"}
			]
		},
		"change_summary": "initial draft"
	}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write page fixture: %v", err)
	}

	var input cms.PageDraftInput
	if err := loadCMSPageInput(path, &input); err != nil {
		t.Fatalf("load page input: %v", err)
	}
	if input.Path != "/about" || input.Title != "About" || input.ChangeSummary != "initial draft" {
		t.Fatalf("unexpected page input: %#v", input)
	}
	blocks, ok := input.Payload["blocks"].([]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("expected one payload block, got %#v", input.Payload["blocks"])
	}
}

func TestLoadCMSNavigationInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navigation.json")
	payload := `{
		"key": "main",
		"title": "Main navigation",
		"location": "primary",
		"items": [
			{"label": "Home", "item_type": "page", "target_ref": "/", "sort_order": 0, "is_enabled": true},
			{"id": 10, "label": "Shop", "item_type": "dropdown", "sort_order": 1, "is_enabled": true}
		]
	}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write navigation fixture: %v", err)
	}

	input, err := loadCMSNavigationInput(path)
	if err != nil {
		t.Fatalf("load navigation input: %v", err)
	}
	if input.Key != "main" || input.Location != "primary" || len(input.Items) != 2 {
		t.Fatalf("unexpected navigation input: %#v", input)
	}
	if input.Items[1].ID != 10 || input.Items[1].ItemType != "dropdown" {
		t.Fatalf("unexpected dropdown item: %#v", input.Items[1])
	}
}

func TestCMSExportEntryVersionUsesRestoreShape(t *testing.T) {
	version := &models.CMSEntryVersion{
		ID: 7, EntryID: 3, VersionNumber: 2, SchemaVersion: 1, PayloadJSON: `{"blocks":[{"type":"rich_text","body":"Hello"}]}`,
		ChangeSummary: "copy update",
	}

	exported := cmsExportEntryVersion(version)
	if exported == nil {
		t.Fatal("expected export version")
	}
	if exported.Payload["blocks"] == nil {
		t.Fatalf("expected restored payload shape, got %#v", exported.Payload)
	}
	if exported.ChangeSummary == nil || *exported.ChangeSummary != "copy update" {
		t.Fatalf("expected change summary pointer, got %#v", exported.ChangeSummary)
	}
}

func TestLoadCMSDeliveryInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delivery.json")
	payload := `{
		"schedule": {
			"publish_at": "2026-07-01T12:00:00Z",
			"unpublish_at": "2026-07-08T12:00:00Z",
			"timezone": "UTC"
		},
		"targeting_rules": [
			{
				"markets": ["US"],
				"device_classes": ["desktop"],
				"auth_states": ["guest"],
				"priority": 10,
				"is_enabled": true
			}
		],
		"experiment": {
			"name": "Hero test",
			"status": "draft",
			"sticky_key": "visitor",
			"starts_at": "2026-07-01T12:00:00Z",
			"variants": [
				{"name": "Control", "version_id": 1, "allocation": 50},
				{"name": "Variant", "version_id": 2, "allocation": 50}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write delivery fixture: %v", err)
	}

	input, err := loadCMSDeliveryInput(path)
	if err != nil {
		t.Fatalf("load delivery input: %v", err)
	}
	if input.Schedule == nil || input.Schedule.Timezone != "UTC" {
		t.Fatalf("expected schedule, got %#v", input.Schedule)
	}
	if len(input.TargetingRules) != 1 || input.TargetingRules[0].Markets[0] != "US" {
		t.Fatalf("unexpected targeting rules: %#v", input.TargetingRules)
	}
	if input.Experiment == nil || len(input.Experiment.Variants) != 2 {
		t.Fatalf("expected experiment variants, got %#v", input.Experiment)
	}
}

func TestCMSScaffoldPageDraftIsUsable(t *testing.T) {
	payload := scaffoldPageDraft()
	if payload["path"] == "" || payload["payload"] == nil {
		t.Fatalf("expected page scaffold with path and payload: %#v", payload)
	}
}
