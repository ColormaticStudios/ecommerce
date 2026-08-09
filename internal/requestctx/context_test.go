package requestctx_test

import (
	"context"

	"testing"
	"time"

	"ecommerce/internal/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrincipalRoundTripIsImmutable(t *testing.T) {
	roles := []string{"admin"}
	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{
		Subject: "subject-1",
		Roles:   roles,
	})
	roles[0] = "customer"

	principal, ok := requestctx.PrincipalFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"admin"}, principal.Roles)
	assert.True(t, principal.HasRole("admin"))

	principal.Roles[0] = "editor"
	again, ok := requestctx.PrincipalFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"admin"}, again.Roles)
}

func TestRequirePrincipalRejectsAnonymousContext(t *testing.T) {
	_, err := requestctx.RequirePrincipal(context.Background())
	assert.ErrorIs(t, err, requestctx.ErrPrincipalRequired)

	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{})
	_, err = requestctx.RequirePrincipal(ctx)
	assert.ErrorIs(t, err, requestctx.ErrPrincipalRequired)
}

func TestMetadataAndOperationRoundTrip(t *testing.T) {
	started := time.Now()
	cookies := map[string]string{"session": "one"}
	ctx := requestctx.WithMetadata(context.Background(), requestctx.Metadata{
		RequestID: "request-1",
		Method:    "GET",
		Path:      "/things",
		StartedAt: started,
		Cookies:   cookies,
	})
	cookies["session"] = "changed"
	ctx = requestctx.WithOperation(ctx, "listThings")

	metadata, ok := requestctx.MetadataFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "request-1", metadata.RequestID)
	assert.Equal(t, "listThings", metadata.OperationID)
	assert.Equal(t, "one", metadata.Cookies["session"])
	metadata.Cookies["session"] = "mutated"
	again, _ := requestctx.MetadataFrom(ctx)
	assert.Equal(t, "one", again.Cookies["session"])
}
