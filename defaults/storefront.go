package defaults

import _ "embed"

//go:embed storefront.json
var storefrontJSON []byte

func StorefrontJSON() []byte {
	cloned := make([]byte, len(storefrontJSON))
	copy(cloned, storefrontJSON)
	return cloned
}
