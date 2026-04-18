package commands

import (
	"fmt"
	"net/http"
	"strings"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func NewProductAttributeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "product-attribute",
		Aliases: []string{"attribute"},
		Short:   "Product attribute definition commands",
	}

	cmd.AddCommand(newListProductAttributesCmd())
	cmd.AddCommand(newCreateProductAttributeCmd())
	cmd.AddCommand(newUpdateProductAttributeCmd())
	cmd.AddCommand(newDeleteProductAttributeCmd())

	return cmd
}

func newListProductAttributesCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List product attribute definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.ProductAttributeDefinitionListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   "/api/v1/admin/product-attributes",
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminProductAttributes(db)
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
				fmt.Println("No product attributes found")
				return nil
			}

			fmt.Printf("%-5s %-24s %-24s %-10s %-10s %-10s\n", "ID", "Key", "Slug", "Type", "Filter", "Sort")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, attribute := range resp.Data {
				fmt.Printf(
					"%-5d %-24s %-24s %-10s %-10t %-10t\n",
					attribute.Id,
					attribute.Key,
					attribute.Slug,
					attribute.Type,
					attribute.Filterable,
					attribute.Sortable,
				)
			}
			return nil
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreateProductAttributeCmd() *cobra.Command {
	var input productAttributeInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a product attribute definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := input.toContract(cmd)
			attribute, err := invokeWithDB[apicontract.ProductAttributeDefinition](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/product-attributes",
				Body:   payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.CreateAdminProductAttribute(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(attribute)
				return nil
			}

			fmt.Printf("✓ Product attribute created: %s (ID: %d)\n", attribute.Key, attribute.Id)
			return nil
		},
	}

	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("key")
	cmd.MarkFlagRequired("type")
	return cmd
}

func newUpdateProductAttributeCmd() *cobra.Command {
	var id uint
	var input productAttributeInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a product attribute definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := input.toContract(cmd)
			attribute, err := invokeWithDB[apicontract.ProductAttributeDefinition](localHandlerRequest{
				Method:     http.MethodPatch,
				Path:       fmt.Sprintf("/api/v1/admin/product-attributes/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
				Body:       payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.UpdateAdminProductAttribute(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(attribute)
				return nil
			}

			fmt.Printf("✓ Product attribute updated: %s (ID: %d)\n", attribute.Key, attribute.Id)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Product attribute ID")
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("key")
	cmd.MarkFlagRequired("type")
	return cmd
}

func newDeleteProductAttributeCmd() *cobra.Command {
	var id uint

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a product attribute definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.MessageResponse](localHandlerRequest{
				Method:     http.MethodDelete,
				Path:       fmt.Sprintf("/api/v1/admin/product-attributes/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.DeleteAdminProductAttribute(db)
			})
			if err != nil {
				return err
			}
			fmt.Println(resp.Message)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Product attribute ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

type productAttributeInputFlags struct {
	key        string
	slug       string
	attrType   string
	filterable bool
	sortable   bool
}

func (f *productAttributeInputFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.key, "key", "", "Product attribute key")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Product attribute slug")
	cmd.Flags().StringVar(&f.attrType, "type", "", "Product attribute type: text, number, boolean, enum")
	cmd.Flags().BoolVar(&f.filterable, "filterable", false, "Whether the attribute is filterable")
	cmd.Flags().BoolVar(&f.sortable, "sortable", false, "Whether the attribute is sortable")
}

func (f productAttributeInputFlags) toContract(cmd *cobra.Command) apicontract.ProductAttributeDefinitionInput {
	var slug *string
	if cmd.Flags().Changed("slug") {
		value := strings.TrimSpace(f.slug)
		slug = &value
	}

	return apicontract.ProductAttributeDefinitionInput{
		Filterable: parseBoolPointerSet(cmd, "filterable", f.filterable),
		Key:        strings.TrimSpace(f.key),
		Slug:       slug,
		Sortable:   parseBoolPointerSet(cmd, "sortable", f.sortable),
		Type:       apicontract.ProductAttributeDefinitionInputType(strings.TrimSpace(f.attrType)),
	}
}
