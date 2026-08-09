package httpapi_test

import (
	"testing"

	"ecommerce/internal/httpapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policy(operationID string) httpapi.OperationPolicy {
	return httpapi.OperationPolicy{
		OperationID:  operationID,
		Access:       httpapi.AccessAuthenticated,
		CSRF:         httpapi.CSRFRequired,
		MaxBodyBytes: 1 << 20,
	}
}

func TestPolicySetValidatesCompletenessAndStaleEntries(t *testing.T) {
	set, err := httpapi.NewPolicySet(policy("createThing"), policy("staleThing"))
	require.NoError(t, err)

	err = set.ValidateComplete([]string{"createThing", "listThings"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing=[listThings]")
	assert.Contains(t, err.Error(), "unknown=[staleThing]")
}

func TestPolicySetRejectsImplicitSecurityDecisions(t *testing.T) {
	_, err := httpapi.NewPolicySet(httpapi.OperationPolicy{OperationID: "listThings"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unspecified access")

	public := policy("publicThing")
	public.Access = httpapi.AccessPublic
	public.Roles = []string{"admin"}
	_, err = httpapi.NewPolicySet(public)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot require roles")
}

func TestContractPolicySetIsCompleteAndConservative(t *testing.T) {
	set, err := httpapi.ContractPolicySet()
	require.NoError(t, err)
	require.NoError(t, set.ValidateContract())

	admin, ok := set.Lookup("CreateAdminBrand")
	require.True(t, ok)
	assert.Equal(t, httpapi.AccessAuthenticated, admin.Access)
	assert.Equal(t, []string{"admin"}, admin.Roles)
	assert.Equal(t, httpapi.CSRFRequired, admin.CSRF)

	public, ok := set.Lookup("ReceiveWebhookEvent")
	require.True(t, ok)
	assert.Equal(t, httpapi.AccessPublic, public.Access)
	assert.Equal(t, httpapi.CSRFExempt, public.CSRF)
	assert.EqualValues(t, 2<<20, public.MaxBodyBytes)

	upload, ok := set.Lookup("PatchMediaUpload")
	require.True(t, ok)
	assert.EqualValues(t, 500<<20, upload.MaxBodyBytes)
}

func TestContractOperationIDsAreGeneratedAndUnique(t *testing.T) {
	operationIDs, err := httpapi.ContractOperationIDs()
	require.NoError(t, err)
	assert.NotEmpty(t, operationIDs)

	seen := make(map[string]struct{}, len(operationIDs))
	for _, operationID := range operationIDs {
		_, duplicate := seen[operationID]
		assert.False(t, duplicate, operationID)
		seen[operationID] = struct{}{}
	}
}
