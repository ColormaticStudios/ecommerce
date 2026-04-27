package commands

import (
	"fmt"
	"net/http"

	"ecommerce/handlers"
	"ecommerce/internal/services/providerops"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
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
			var settings handlers.WebsiteSettingsPayload
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
			if cmd.Flags().Changed("oidc-provider") {
				settings.OIDCProvider = oidcProvider
			}
			if cmd.Flags().Changed("oidc-client-id") {
				settings.OIDCClientID = oidcClientID
			}
			if cmd.Flags().Changed("oidc-client-secret") {
				settings.OIDCClientSecret = oidcClientSecret
			}
			if cmd.Flags().Changed("clear-oidc-client-secret") {
				settings.ClearOIDCClientSecret = clearOIDCClientSecret
			}
			if settings.ClearOIDCClientSecret && settings.OIDCClientSecret != "" {
				return fmt.Errorf("--oidc-client-secret and --clear-oidc-client-secret cannot be used together")
			}
			if cmd.Flags().Changed("oidc-redirect-uri") {
				settings.OIDCRedirectURI = oidcRedirectURI
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
	cmd.Flags().StringVar(&oidcProvider, "oidc-provider", "", "OIDC issuer/provider URL")
	cmd.Flags().StringVar(&oidcClientID, "oidc-client-id", "", "OIDC client ID")
	cmd.Flags().StringVar(&oidcClientSecret, "oidc-client-secret", "", "OIDC client secret")
	cmd.Flags().BoolVar(&clearOIDCClientSecret, "clear-oidc-client-secret", false, "Remove the stored OIDC client secret")
	cmd.Flags().StringVar(&oidcRedirectURI, "oidc-redirect-uri", "", "OIDC redirect URI")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func getWebsiteSettings() (handlers.WebsiteSettingsResponse, error) {
	return invokeWithDB[handlers.WebsiteSettingsResponse](localHandlerRequest{
		Method: http.MethodGet,
		Path:   "/api/v1/admin/website",
	}, func(db *gorm.DB) gin.HandlerFunc {
		return handlers.GetAdminWebsiteSettings(db)
	})
}

func updateWebsiteSettings(settings handlers.WebsiteSettingsPayload) (handlers.WebsiteSettingsResponse, error) {
	return invokeWithDB[handlers.WebsiteSettingsResponse](localHandlerRequest{
		Method: http.MethodPut,
		Path:   "/api/v1/admin/website",
		Body:   handlers.UpsertWebsiteSettingsRequest{Settings: settings},
	}, func(db *gorm.DB) gin.HandlerFunc {
		return handlers.UpsertWebsiteSettingsWithCredentials(db, newWebsiteCredentialService())
	})
}

func printWebsiteSettings(resp handlers.WebsiteSettingsResponse) {
	fmt.Printf("Updated: %s\n", resp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("Allow Guest Checkout: %t\n", resp.Settings.AllowGuestCheckout)
	fmt.Printf("OIDC Enabled: %t\n", websiteOIDCConfigured(resp.Settings))
	fmt.Printf("OIDC Provider: %s\n", resp.Settings.OIDCProvider)
	fmt.Printf("OIDC Client ID: %s\n", resp.Settings.OIDCClientID)
	fmt.Printf("OIDC Client Secret Configured: %t\n", resp.Settings.OIDCClientSecretConfigured)
	fmt.Printf("OIDC Redirect URI: %s\n", resp.Settings.OIDCRedirectURI)
}

func websiteOIDCConfigured(settings handlers.WebsiteSettingsPayload) bool {
	return settings.OIDCProvider != "" && settings.OIDCClientID != "" && settings.OIDCRedirectURI != ""
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
