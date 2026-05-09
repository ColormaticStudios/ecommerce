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

func NewCategoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "category",
		Aliases: []string{"categories"},
		Short:   "Category management commands",
	}

	cmd.AddCommand(newListCategoriesCmd())
	cmd.AddCommand(newCreateCategoryCmd())
	cmd.AddCommand(newUpdateCategoryCmd())
	cmd.AddCommand(newDeleteCategoryCmd())

	return cmd
}

func newListCategoriesCmd() *cobra.Command {
	var format string
	var query string
	var includeInactive bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if trimmed := strings.TrimSpace(query); trimmed != "" {
				params.Set("q", trimmed)
			}
			if includeInactive {
				params.Set("include_inactive", "true")
			}

			path := "/api/v1/admin/categories"
			if encoded := params.Encode(); encoded != "" {
				path += "?" + encoded
			}
			resp, err := invokeWithDB[apicontract.CategoryListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminCategories(db)
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
				fmt.Println("No categories found")
				return nil
			}

			fmt.Printf("%-5s %-28s %-28s %-8s %-5s %-28s\n", "ID", "Name", "Slug", "Active", "Depth", "Path")
			fmt.Println("--------------------------------------------------------------------------------------------------------")
			for _, category := range resp.Data {
				fmt.Printf("%-5d %-28s %-28s %-8t %-5d %-28s\n",
					category.Id,
					category.Name,
					category.Slug,
					category.IsActive,
					category.Depth,
					category.Path,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "q", "", "Search term")
	cmd.Flags().BoolVar(&includeInactive, "include-inactive", false, "Include inactive categories")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreateCategoryCmd() *cobra.Command {
	var input categoryInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a category",
		RunE: func(cmd *cobra.Command, args []string) error {
			category, err := invokeWithDB[apicontract.Category](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/categories",
				Body:   input.toContract(cmd),
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.CreateAdminCategory(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(category)
				return nil
			}
			fmt.Printf("✓ Category created: %s (ID: %d, slug: %s)\n", category.Name, category.Id, category.Slug)
			return nil
		},
	}

	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("name")
	return cmd
}

func newUpdateCategoryCmd() *cobra.Command {
	var id uint
	var input categoryInputFlags
	var format string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a category",
		RunE: func(cmd *cobra.Command, args []string) error {
			category, err := invokeWithDB[apicontract.Category](localHandlerRequest{
				Method:     http.MethodPatch,
				Path:       fmt.Sprintf("/api/v1/admin/categories/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
				Body:       input.toContract(cmd),
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.UpdateAdminCategory(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(category)
				return nil
			}
			fmt.Printf("✓ Category updated: %s (ID: %d, slug: %s)\n", category.Name, category.Id, category.Slug)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Category ID")
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newDeleteCategoryCmd() *cobra.Command {
	var id uint

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a category",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.MessageResponse](localHandlerRequest{
				Method:     http.MethodDelete,
				Path:       fmt.Sprintf("/api/v1/admin/categories/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.DeleteAdminCategory(db)
			})
			if err != nil {
				return err
			}
			fmt.Println(resp.Message)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Category ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

type categoryInputFlags struct {
	name        string
	slug        string
	description string
	parentID    uint
	sortOrder   int
	isActive    bool
}

func (f *categoryInputFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Category name")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Category slug")
	cmd.Flags().StringVar(&f.description, "description", "", "Category description")
	cmd.Flags().UintVar(&f.parentID, "parent-id", 0, "Parent category ID")
	cmd.Flags().IntVar(&f.sortOrder, "sort-order", 0, "Category sort order")
	cmd.Flags().BoolVar(&f.isActive, "is-active", true, "Whether the category is active")
}

func (f categoryInputFlags) toContract(cmd *cobra.Command) apicontract.CategoryInput {
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
	var parentID *int
	if cmd.Flags().Changed("parent-id") {
		value := int(f.parentID)
		parentID = &value
	}

	return apicontract.CategoryInput{
		Description: description,
		IsActive:    parseBoolPointerSet(cmd, "is-active", f.isActive),
		Name:        strings.TrimSpace(f.name),
		ParentId:    parentID,
		Slug:        slug,
		SortOrder:   intPointerSet(cmd, "sort-order", f.sortOrder),
	}
}

func intPointerSet(cmd *cobra.Command, name string, value int) *int {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	result := value
	return &result
}
