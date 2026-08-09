package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
)

type Access uint8

const (
	AccessUnspecified Access = iota
	AccessPublic
	AccessAuthenticated
)

type CSRFPolicy uint8

const (
	CSRFUnspecified CSRFPolicy = iota
	CSRFExempt
	CSRFRequired
)

// OperationPolicy is explicit security and request-boundary metadata for one
// generated operation. OperationSecurityMiddleware enforces CSRF and body limits
// before generated binding; completeness validation makes omission detectable.
type OperationPolicy struct {
	OperationID  string
	Access       Access
	Roles        []string
	CSRF         CSRFPolicy
	MaxBodyBytes int64
}

func (policy OperationPolicy) validate() error {
	if strings.TrimSpace(policy.OperationID) == "" {
		return errors.New("operation ID is required")
	}
	if policy.Access != AccessPublic && policy.Access != AccessAuthenticated {
		return fmt.Errorf("operation %q has unspecified access", policy.OperationID)
	}
	if policy.CSRF != CSRFExempt && policy.CSRF != CSRFRequired {
		return fmt.Errorf("operation %q has unspecified CSRF policy", policy.OperationID)
	}
	if policy.MaxBodyBytes <= 0 {
		return fmt.Errorf("operation %q must declare a positive body limit", policy.OperationID)
	}
	if policy.Access == AccessPublic && len(policy.Roles) != 0 {
		return fmt.Errorf("public operation %q cannot require roles", policy.OperationID)
	}
	for _, role := range policy.Roles {
		if strings.TrimSpace(role) == "" {
			return fmt.Errorf("operation %q contains an empty role", policy.OperationID)
		}
	}
	return nil
}

type PolicySet struct {
	byOperation map[string]OperationPolicy
}

func NewPolicySet(policies ...OperationPolicy) (PolicySet, error) {
	set := PolicySet{byOperation: make(map[string]OperationPolicy, len(policies))}
	for _, policy := range policies {
		if err := policy.validate(); err != nil {
			return PolicySet{}, err
		}
		if _, exists := set.byOperation[policy.OperationID]; exists {
			return PolicySet{}, fmt.Errorf("duplicate policy for operation %q", policy.OperationID)
		}
		policy.Roles = slices.Clone(policy.Roles)
		set.byOperation[policy.OperationID] = policy
	}
	return set, nil
}

func (set PolicySet) Lookup(operationID string) (OperationPolicy, bool) {
	policy, ok := set.byOperation[operationID]
	policy.Roles = slices.Clone(policy.Roles)
	return policy, ok
}

// ValidateComplete rejects both missing generated operations and stale policy
// entries, making contract changes fail startup/tests until policy is reviewed.
func (set PolicySet) ValidateComplete(operationIDs []string) error {
	expected := make(map[string]struct{}, len(operationIDs))
	for _, operationID := range operationIDs {
		if operationID == "" {
			return errors.New("contract contains an empty operation ID")
		}
		if _, duplicate := expected[operationID]; duplicate {
			return fmt.Errorf("contract contains duplicate operation ID %q", operationID)
		}
		expected[operationID] = struct{}{}
	}

	var missing, unknown []string
	for operationID := range expected {
		if _, ok := set.byOperation[operationID]; !ok {
			missing = append(missing, operationID)
		}
	}
	for operationID := range set.byOperation {
		if _, ok := expected[operationID]; !ok {
			unknown = append(unknown, operationID)
		}
	}
	sort.Strings(missing)
	sort.Strings(unknown)
	if len(missing) != 0 || len(unknown) != 0 {
		return fmt.Errorf("operation policy mismatch: missing=%v unknown=%v", missing, unknown)
	}
	return nil
}

func ContractOperationIDs() ([]string, error) {
	spec, err := apicontract.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load generated OpenAPI contract: %w", err)
	}
	if spec.Paths == nil {
		return nil, errors.New("generated OpenAPI contract has no paths")
	}
	var operationIDs []string
	for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			if operation.OperationID == "" {
				return nil, errors.New("generated OpenAPI operation is missing operationId")
			}
			operationIDs = append(operationIDs, operation.OperationID)
		}
	}
	sort.Strings(operationIDs)
	return operationIDs, nil
}

func (set PolicySet) ValidateContract() error {
	operationIDs, err := ContractOperationIDs()
	if err != nil {
		return err
	}
	return set.ValidateComplete(operationIDs)
}

const operationPolicyExtension = "x-operation-policy"

type contractPolicyMetadata struct {
	DefaultMaxBodyBytes int64            `json:"default-max-body-bytes"`
	AdminPathPrefix     string           `json:"admin-path-prefix"`
	AdminRoles          []string         `json:"admin-roles"`
	CSRFMode            string           `json:"csrf-mode"`
	BodyLimits          map[string]int64 `json:"body-limits"`
}

func parseContractPolicyMetadata(value any) (contractPolicyMetadata, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return contractPolicyMetadata{}, fmt.Errorf("encode %s: %w", operationPolicyExtension, err)
	}
	var metadata contractPolicyMetadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return contractPolicyMetadata{}, fmt.Errorf("decode %s: %w", operationPolicyExtension, err)
	}
	if metadata.DefaultMaxBodyBytes <= 0 {
		return contractPolicyMetadata{}, fmt.Errorf("%s must declare a positive default-max-body-bytes", operationPolicyExtension)
	}
	if metadata.AdminPathPrefix == "" || len(metadata.AdminRoles) == 0 {
		return contractPolicyMetadata{}, fmt.Errorf("%s must declare the admin path prefix and roles", operationPolicyExtension)
	}
	if metadata.CSRFMode != "cookie-authenticated-unsafe" {
		return contractPolicyMetadata{}, fmt.Errorf("%s has unsupported csrf-mode %q", operationPolicyExtension, metadata.CSRFMode)
	}
	for route, limit := range metadata.BodyLimits {
		if strings.TrimSpace(route) == "" || limit <= 0 {
			return contractPolicyMetadata{}, fmt.Errorf("%s contains invalid body limit %q=%d", operationPolicyExtension, route, limit)
		}
	}
	return metadata, nil
}

// ContractPolicySet derives every operation policy from the generated OpenAPI
// document. Standard security declarations control access, while the required
// x-operation-policy extension declares role, CSRF, and body-limit rules.
func ContractPolicySet() (PolicySet, error) {
	spec, err := apicontract.GetSwagger()
	if err != nil {
		return PolicySet{}, fmt.Errorf("load generated OpenAPI contract: %w", err)
	}
	metadataValue, ok := spec.Extensions[operationPolicyExtension]
	if !ok {
		return PolicySet{}, fmt.Errorf("generated OpenAPI contract is missing %s", operationPolicyExtension)
	}
	metadata, err := parseContractPolicyMetadata(metadataValue)
	if err != nil {
		return PolicySet{}, err
	}
	unusedBodyLimits := make(map[string]struct{}, len(metadata.BodyLimits))
	for route := range metadata.BodyLimits {
		unusedBodyLimits[route] = struct{}{}
	}
	policies := make([]OperationPolicy, 0, len(spec.Paths.Map()))
	for path, item := range spec.Paths.Map() {
		for method, operation := range item.Operations() {
			security := spec.Security
			if operation.Security != nil {
				security = *operation.Security
			}
			policy := OperationPolicy{
				OperationID:  operation.OperationID,
				Access:       AccessAuthenticated,
				CSRF:         CSRFExempt,
				MaxBodyBytes: metadata.DefaultMaxBodyBytes,
			}
			if len(security) == 0 || strings.HasPrefix(path, "/api/v1/checkout/") {
				policy.Access = AccessPublic
			}
			if strings.HasPrefix(path, metadata.AdminPathPrefix) {
				policy.Roles = slices.Clone(metadata.AdminRoles)
			}
			if policy.Access == AccessAuthenticated && isUnsafeMethod(method) {
				policy.CSRF = CSRFRequired
			}
			route := strings.ToUpper(method) + " " + path
			if limit, overridden := metadata.BodyLimits[route]; overridden {
				policy.MaxBodyBytes = limit
				delete(unusedBodyLimits, route)
			}
			policies = append(policies, policy)
		}
	}
	if len(unusedBodyLimits) != 0 {
		unknown := make([]string, 0, len(unusedBodyLimits))
		for route := range unusedBodyLimits {
			unknown = append(unknown, route)
		}
		sort.Strings(unknown)
		return PolicySet{}, fmt.Errorf("%s contains unknown body-limit routes: %v", operationPolicyExtension, unknown)
	}
	set, err := NewPolicySet(policies...)
	if err != nil {
		return PolicySet{}, err
	}
	if err := set.ValidateContract(); err != nil {
		return PolicySet{}, err
	}
	return set, nil
}

func isUnsafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func authorizePolicy(ctx context.Context, policy OperationPolicy) error {
	if policy.Access == AccessPublic {
		return nil
	}
	principal, ok := requestctx.PrincipalFrom(ctx)
	if !ok {
		return ErrorProblem(Problem{
			Type:   TypeAuthenticationRequired,
			Title:  http.StatusText(http.StatusUnauthorized),
			Status: http.StatusUnauthorized,
			Code:   "authentication_required",
			Detail: "Authentication is required.",
		}, requestctx.ErrPrincipalRequired)
	}
	if len(policy.Roles) == 0 {
		return nil
	}
	for _, role := range policy.Roles {
		if principal.HasRole(role) {
			return nil
		}
	}
	return ErrorProblem(Problem{
		Type:   TypeForbidden,
		Title:  http.StatusText(http.StatusForbidden),
		Status: http.StatusForbidden,
		Code:   "forbidden",
		Detail: "The authenticated principal is not allowed to perform this operation.",
	}, nil)
}

func interfaceIsNil(value any) bool {
	if value == nil {
		return true
	}
	kind := reflect.ValueOf(value).Kind()
	return (kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice) && reflect.ValueOf(value).IsNil()
}
