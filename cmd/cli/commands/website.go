package commands

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	accountservice "ecommerce/internal/services/account"
	"ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/spf13/cobra"
)

func NewWebsiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "website",
		Short: "Website-level settings commands",
	}

	cmd.AddCommand(newPrintWebsiteCmd())
	cmd.AddCommand(newExportWebsiteCmd())
	cmd.AddCommand(newImportWebsiteCmd())
	cmd.AddCommand(newSetWebsiteCmd())

	return cmd
}

func newPrintWebsiteCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print website settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := getWebsiteSettings()
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(resp)
				return nil
			}

			printWebsiteSettings(resp)
			return nil
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newExportWebsiteCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the current website settings JSON payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := getWebsiteSettings()
			if err != nil {
				return err
			}

			if outputPath != "" {
				if err := writeJSONFile(outputPath, resp.Settings); err != nil {
					return err
				}
				fmt.Printf("website_json_path=%s\n", outputPath)
				return nil
			}

			printJSON(resp.Settings)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "out", "", "Write website settings JSON to a file")
	return cmd
}

func newImportWebsiteCmd() *cobra.Command {
	var filePath string
	var format string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import website settings from JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			var settings apicontract.WebsiteSettings
			if err := loadJSONFile(filePath, &settings); err != nil {
				return err
			}

			resp, err := updateWebsiteSettings(settings)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(resp)
				return nil
			}

			fmt.Println("✓ Website settings updated")
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to website settings JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("file")
	return cmd
}

func newSetWebsiteCmd() *cobra.Command {
	var allowGuestCheckout bool
	var couponCodesEnabled bool
	var oidcProvider string
	var oidcClientID string
	var oidcClientSecret string
	var clearOIDCClientSecret bool
	var oidcRedirectURI string
	var format string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update selected website settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			current, err := getWebsiteSettings()
			if err != nil {
				return err
			}
			settings := current.Settings

			if cmd.Flags().Changed("allow-guest-checkout") {
				settings.AllowGuestCheckout = allowGuestCheckout
			}
			if cmd.Flags().Changed("coupon-codes-enabled") {
				settings.CouponCodesEnabled = couponCodesEnabled
			}
			if cmd.Flags().Changed("oidc-provider") {
				settings.OidcProvider = oidcProvider
			}
			if cmd.Flags().Changed("oidc-client-id") {
				settings.OidcClientId = oidcClientID
			}
			if cmd.Flags().Changed("oidc-client-secret") {
				settings.OidcClientSecret = oidcClientSecret
			}
			if cmd.Flags().Changed("clear-oidc-client-secret") {
				settings.ClearOidcClientSecret = clearOIDCClientSecret
			}
			if settings.ClearOidcClientSecret && settings.OidcClientSecret != "" {
				return fmt.Errorf("--oidc-client-secret and --clear-oidc-client-secret cannot be used together")
			}
			if cmd.Flags().Changed("oidc-redirect-uri") {
				settings.OidcRedirectUri = oidcRedirectURI
			}

			resp, err := updateWebsiteSettings(settings)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(resp)
				return nil
			}

			fmt.Println("✓ Website settings updated")
			return nil
		},
	}

	cmd.Flags().BoolVar(&allowGuestCheckout, "allow-guest-checkout", true, "Allow guest cart and checkout access")
	cmd.Flags().BoolVar(&couponCodesEnabled, "coupon-codes-enabled", true, "Allow coupon-code-gated discounts during checkout")
	cmd.Flags().StringVar(&oidcProvider, "oidc-provider", "", "OIDC issuer/provider URL")
	cmd.Flags().StringVar(&oidcClientID, "oidc-client-id", "", "OIDC client ID")
	cmd.Flags().StringVar(&oidcClientSecret, "oidc-client-secret", "", "OIDC client secret")
	cmd.Flags().BoolVar(&clearOIDCClientSecret, "clear-oidc-client-secret", false, "Remove the stored OIDC client secret")
	cmd.Flags().StringVar(&oidcRedirectURI, "oidc-redirect-uri", "", "OIDC redirect URI")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func getWebsiteSettings() (apicontract.WebsiteSettingsResponse, error) {
	if isRemoteMode() {
		return invokeRemoteJSON[apicontract.WebsiteSettingsResponse](http.MethodGet, "/api/v1/admin/website", nil)
	}
	db := getDB()
	defer closeDB(db)
	settings, err := accountservice.NewService(db, newWebsiteCredentialService()).GetWebsiteSettings(context.Background())
	if err != nil {
		return apicontract.WebsiteSettingsResponse{}, err
	}
	return websiteSettingsContract(settings), nil
}

func updateWebsiteSettings(settings apicontract.WebsiteSettings) (apicontract.WebsiteSettingsResponse, error) {
	if isRemoteMode() {
		return invokeRemoteJSON[apicontract.WebsiteSettingsResponse](http.MethodPut, "/api/v1/admin/website", apicontract.WebsiteSettingsRequest{Settings: settings})
	}
	db := getDB()
	defer closeDB(db)
	updated, err := accountservice.NewService(db, newWebsiteCredentialService()).UpdateWebsiteSettings(context.Background(), accountservice.WebsiteSettingsInput{
		SiteTitle: settings.SiteTitle, AllowGuestCheckout: settings.AllowGuestCheckout, CouponCodesEnabled: settings.CouponCodesEnabled,
		OIDCProvider: settings.OidcProvider, OIDCClientID: settings.OidcClientId, OIDCClientSecret: settings.OidcClientSecret,
		ClearOIDCClientSecret: settings.ClearOidcClientSecret, OIDCRedirectURI: settings.OidcRedirectUri,
	})
	if err != nil {
		return apicontract.WebsiteSettingsResponse{}, err
	}
	return websiteSettingsContract(updated), nil
}

func websiteSettingsContract(settings models.WebsiteSettings) apicontract.WebsiteSettingsResponse {
	return apicontract.WebsiteSettingsResponse{
		Settings: apicontract.WebsiteSettings{
			SiteTitle: settings.SiteTitle, AllowGuestCheckout: settings.AllowGuestCheckout, CouponCodesEnabled: settings.CouponCodesEnabled,
			OidcProvider: settings.OIDCProvider, OidcClientId: settings.OIDCClientID, OidcClientSecret: "",
			OidcClientSecretConfigured: strings.TrimSpace(settings.OIDCClientSecretEnvelopeJSON) != "", OidcRedirectUri: settings.OIDCRedirectURI,
		},
		UpdatedAt: settings.UpdatedAt,
	}
}

func printWebsiteSettings(resp apicontract.WebsiteSettingsResponse) {
	fmt.Printf("Updated: %s\n", resp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("Allow Guest Checkout: %t\n", resp.Settings.AllowGuestCheckout)
	fmt.Printf("Coupon Codes Enabled: %t\n", resp.Settings.CouponCodesEnabled)
	fmt.Printf("OIDC Enabled: %t\n", websiteOIDCConfigured(resp.Settings))
	fmt.Printf("OIDC Provider: %s\n", resp.Settings.OidcProvider)
	fmt.Printf("OIDC Client ID: %s\n", resp.Settings.OidcClientId)
	fmt.Printf("OIDC Client Secret Configured: %t\n", resp.Settings.OidcClientSecretConfigured)
	fmt.Printf("OIDC Redirect URI: %s\n", resp.Settings.OidcRedirectUri)
}

func websiteOIDCConfigured(settings apicontract.WebsiteSettings) bool {
	return settings.OidcProvider != "" && settings.OidcClientId != "" && settings.OidcRedirectUri != ""
}

func newWebsiteCredentialService() *providerops.CredentialService {
	cfg := getConfig()
	keyring, err := providerops.ParseKeyringConfig(cfg.ProviderCredentialsKeys)
	if err != nil {
		return &providerops.CredentialService{}
	}
	service, err := providerops.NewCredentialService(keyring, cfg.ProviderCredentialsKeyVersion)
	if err != nil {
		return &providerops.CredentialService{}
	}
	return service
}
