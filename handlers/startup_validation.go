package handlers

import (
	"fmt"
)

// ValidateStartupDefaults validates static defaults that handlers depend on.
func ValidateStartupDefaults() error {
	loadStorefrontLimits()
	if storefrontLimitsErr != nil {
		return fmt.Errorf("storefront limits defaults invalid: %w", storefrontLimitsErr)
	}

	defaultStorefrontOnce.Do(func() {
		defaultStorefront, defaultStorefrontErr = decodeDefaultStorefrontSettingsStrict()
	})
	if defaultStorefrontErr != nil {
		return fmt.Errorf("storefront defaults invalid: %w", defaultStorefrontErr)
	}

	return nil
}
