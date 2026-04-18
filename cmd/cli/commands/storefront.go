package commands

import (
	"fmt"
	"net/http"

	"ecommerce/handlers"
	"ecommerce/internal/media"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func NewStorefrontCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storefront",
		Short: "Storefront configuration commands",
	}

	cmd.AddCommand(newPrintStorefrontCmd())
	cmd.AddCommand(newExportStorefrontCmd())
	cmd.AddCommand(newImportStorefrontCmd())
	cmd.AddCommand(newPublishStorefrontCmd())
	cmd.AddCommand(newDiscardStorefrontCmd())

	return cmd
}

func newPrintStorefrontCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print storefront settings including draft metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   "/api/v1/admin/storefront",
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.GetAdminStorefrontSettings(mediaService.DB, mediaService)
			})
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

			fmt.Printf("Updated: %s\n", resp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
			fmt.Printf("Published Updated: %s\n", resp.PublishedUpdated.Format("2006-01-02T15:04:05Z07:00"))
			fmt.Printf("Has Draft Changes: %t\n", resp.HasDraftChanges)
			fmt.Printf("Sections: %d\n", len(resp.Settings.HomepageSections))
			return nil
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatJSON))
	return cmd
}

func newExportStorefrontCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the current storefront settings JSON payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   "/api/v1/admin/storefront",
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.GetAdminStorefrontSettings(mediaService.DB, mediaService)
			})
			if err != nil {
				return err
			}

			if outputPath != "" {
				if err := writeJSONFile(outputPath, resp.Settings); err != nil {
					return err
				}
				fmt.Printf("storefront_json_path=%s\n", outputPath)
				return nil
			}

			printJSON(resp.Settings)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "out", "", "Write storefront JSON to a file")
	return cmd
}

func newImportStorefrontCmd() *cobra.Command {
	var filePath string
	var publish bool
	var format string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import storefront settings from JSON into the draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			var settings handlers.StorefrontSettingsPayload
			if err := loadJSONFile(filePath, &settings); err != nil {
				return err
			}

			resp, err := invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
				Method: http.MethodPut,
				Path:   "/api/v1/admin/storefront",
				Body:   handlers.UpsertStorefrontSettingsRequest{Settings: settings},
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.UpsertStorefrontSettings(mediaService.DB, mediaService)
			})
			if err != nil {
				return err
			}

			if publish {
				resp, err = invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
					Method: http.MethodPost,
					Path:   "/api/v1/admin/storefront/publish",
				}, func(mediaService *media.Service) gin.HandlerFunc {
					return handlers.PublishStorefrontSettings(mediaService.DB, mediaService)
				})
				if err != nil {
					return err
				}
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(resp)
				return nil
			}

			if publish {
				fmt.Println("✓ Storefront imported and published")
			} else {
				fmt.Println("✓ Storefront draft updated")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to storefront JSON")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish after importing")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("file")
	return cmd
}

func newPublishStorefrontCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish the storefront draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/storefront/publish",
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.PublishStorefrontSettings(mediaService.DB, mediaService)
			})
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

			fmt.Println("✓ Storefront draft published")
			return nil
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newDiscardStorefrontCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "discard",
		Short: "Discard the storefront draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithMediaService[handlers.StorefrontSettingsResponse](localHandlerRequest{
				Method: http.MethodDelete,
				Path:   "/api/v1/admin/storefront/draft",
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.DiscardStorefrontDraft(mediaService.DB, mediaService)
			})
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

			fmt.Println("✓ Storefront draft discarded")
			return nil
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}
