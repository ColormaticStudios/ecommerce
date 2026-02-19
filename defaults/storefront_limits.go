package defaults

import _ "embed"

//go:embed storefront-limits.json
var storefrontLimitsJSON []byte

func StorefrontLimitsJSON() []byte {
	cloned := make([]byte, len(storefrontLimitsJSON))
	copy(cloned, storefrontLimitsJSON)
	return cloned
}
