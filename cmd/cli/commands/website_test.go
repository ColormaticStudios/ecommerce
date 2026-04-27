package commands

import (
	"testing"

	"ecommerce/handlers"
)

func TestRootCommandIncludesWebsiteSettingsGroup(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"website"})
	if err != nil {
		t.Fatalf("find website command: %v", err)
	}
	if cmd == nil || cmd.Name() != "website" {
		t.Fatalf("expected website command, got %#v", cmd)
	}
}

func TestWebsiteOIDCConfiguredRequiresCoreFields(t *testing.T) {
	settings := handlers.WebsiteSettingsPayload{
		OIDCProvider:    "https://issuer.example",
		OIDCClientID:    "client-id",
		OIDCRedirectURI: "https://shop.example/api/v1/auth/oidc/callback",
	}
	if !websiteOIDCConfigured(settings) {
		t.Fatalf("expected OIDC to be configured")
	}

	settings.OIDCClientID = ""
	if websiteOIDCConfigured(settings) {
		t.Fatalf("expected OIDC to be unconfigured without client id")
	}
}
