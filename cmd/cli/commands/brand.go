package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func NewBrandCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brand",
		Short: "Brand management commands",
	}

	cmd.AddCommand(newListBrandsCmd())
	cmd.AddCommand(newCreateBrandCmd())
	cmd.AddCommand(newUpdateBrandCmd())
	cmd.AddCommand(newDeleteBrandCmd())

	return cmd
}

func newListBrandsCmd() *cobra.Command {
	var format string
	var query string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List brands",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/brands"
			if trimmed := strings.TrimSpace(query); trimmed != "" {
				path += "?q=" + url.QueryEscape(trimmed)
			}

			resp, err := invokeWithDB[apicontract.BrandListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminBrands(db)
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

			if len(resp.Data) == 0 {
				fmt.Println("No brands found")
				return nil
			}

			fmt.Printf("%-5s %-24s %-24s %-8s\n", "ID", "Name", "Slug", "Active")
			fmt.Println("----------------------------------------------------------------")
			for _, brand := range resp.Data {
				fmt.Printf("%-5d %-24s %-24s %-8t\n", brand.Id, brand.Name, brand.Slug, brand.IsActive)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "q", "", "Search term")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreateBrandCmd() *cobra.Command {
	var input brandInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a brand",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := input.toContract(cmd)
			brand, err := invokeWithDB[apicontract.Brand](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/brands",
				Body:   payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.CreateAdminBrand(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(brand)
				return nil
			}

			fmt.Printf("✓ Brand created: %s (ID: %d)\n", brand.Name, brand.Id)
			return nil
		},
	}

	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("name")
	return cmd
}

func newUpdateBrandCmd() *cobra.Command {
	var id uint
	var input brandInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a brand",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := input.toContract(cmd)
			brand, err := invokeWithDB[apicontract.Brand](localHandlerRequest{
				Method:     http.MethodPatch,
				Path:       fmt.Sprintf("/api/v1/admin/brands/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
				Body:       payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.UpdateAdminBrand(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(brand)
				return nil
			}

			fmt.Printf("✓ Brand updated: %s (ID: %d)\n", brand.Name, brand.Id)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Brand ID")
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newDeleteBrandCmd() *cobra.Command {
	var id uint

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a brand",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.MessageResponse](localHandlerRequest{
				Method:     http.MethodDelete,
				Path:       fmt.Sprintf("/api/v1/admin/brands/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.DeleteAdminBrand(db)
			})
			if err != nil {
				return err
			}
			fmt.Println(resp.Message)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Brand ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

type brandInputFlags struct {
	name        string
	slug        string
	description string
	logoMediaID string
	isActive    bool
}

func (f *brandInputFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Brand name")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Brand slug")
	cmd.Flags().StringVar(&f.description, "description", "", "Brand description")
	cmd.Flags().StringVar(&f.logoMediaID, "logo-media-id", "", "Brand logo media ID")
	cmd.Flags().BoolVar(&f.isActive, "is-active", true, "Whether the brand is active")
}

func (f brandInputFlags) toContract(cmd *cobra.Command) apicontract.BrandInput {
	var slug *string
	if cmd.Flags().Changed("slug") {
		value := strings.TrimSpace(f.slug)
		slug = &value
	}

	var description *string
	if cmd.Flags().Changed("description") {
		value := strings.TrimSpace(f.description)
		description = &value
	}

	var logoMediaID *string
	if cmd.Flags().Changed("logo-media-id") {
		value := strings.TrimSpace(f.logoMediaID)
		logoMediaID = &value
	}

	return apicontract.BrandInput{
		Description: description,
		IsActive:    parseBoolPointerSet(cmd, "is-active", f.isActive),
		LogoMediaId: logoMediaID,
		Name:        strings.TrimSpace(f.name),
		Slug:        slug,
	}
}
