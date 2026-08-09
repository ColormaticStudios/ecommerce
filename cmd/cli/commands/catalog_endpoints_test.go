package commands

import (
	"context"
	"strings"
	"testing"

	"ecommerce/internal/httpapi"
)

func TestWithCatalogEndpointsRejectsRemoteModeBeforeRunningCallback(t *testing.T) {
	previous := activeCLIRuntime
	activeCLIRuntime = cliRuntime{Remote: &persistentCLIAuth{APIURL: "https://example.test", Token: "token"}}
	t.Cleanup(func() { activeCLIRuntime = previous })

	called := false
	_, err := withCatalogEndpoints(context.Background(), func(context.Context, *httpapi.CatalogEndpoints) (struct{}, error) {
		called = true
		return struct{}{}, nil
	})
	if err == nil {
		t.Fatal("expected remote mode to be rejected")
	}
	if !strings.Contains(err.Error(), "only available") {
		t.Fatalf("expected an actionable local-mode error, got %q", err)
	}
	if called {
		t.Fatal("catalog callback ran in remote mode")
	}
}
