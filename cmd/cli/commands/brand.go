package commands

import (
	"context"
	"fmt"

	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"

	"github.com/spf13/cobra"
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
			resp, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.BrandListResponse, error) {
				trimmed := strings.TrimSpace(query)
				response, err := endpoints.ListAdminBrands(ctx, apicontract.ListAdminBrandsRequestObject{Params: apicontract.ListAdminBrandsParams{Q: &trimmed}})
				if err != nil {
					return apicontract.BrandListResponse{}, err
				}
				return apicontract.BrandListResponse(response.(apicontract.ListAdminBrands200JSONResponse)), nil
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
			brand, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.Brand, error) {
				response, err := endpoints.CreateAdminBrand(ctx, apicontract.CreateAdminBrandRequestObject{Body: &payload})
				if err != nil {
					return apicontract.Brand{}, err
				}
				return apicontract.Brand(response.(apicontract.CreateAdminBrand201JSONResponse)), nil
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
			brand, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.Brand, error) {
				response, err := endpoints.UpdateAdminBrand(ctx, apicontract.UpdateAdminBrandRequestObject{Id: int(id), Body: &payload})
				if err != nil {
					return apicontract.Brand{}, err
				}
				return apicontract.Brand(response.(apicontract.UpdateAdminBrand200JSONResponse)), nil
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
			resp, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.MessageResponse, error) {
				response, err := endpoints.DeleteAdminBrand(ctx, apicontract.DeleteAdminBrandRequestObject{Id: int(id)})
				if err != nil {
					return apicontract.MessageResponse{}, err
				}
				return apicontract.MessageResponse(response.(apicontract.DeleteAdminBrand200JSONResponse)), nil
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
		Logo: func() *apicontract.BrandLogoInput {
			if logoMediaID == nil {
				return nil
			}
			return &apicontract.BrandLogoInput{MediaId: *logoMediaID}
		}(),
		Name: strings.TrimSpace(f.name),
		Slug: slug,
	}
}
