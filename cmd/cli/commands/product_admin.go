package commands

import (
	"fmt"
	"net/http"
	"os"
	"reflect"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"

	"github.com/spf13/cobra"
)

func newPrintProductCmd() *cobra.Command {
	var id, sku string
	var live bool
	var format string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print a product JSON document for CLI-driven editing",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			product, err := findProductByIDOrSKU(db, id, sku)
			if err != nil {
				return err
			}

			var input apicontract.ProductUpsertInput
			if live {
				input, err = loadLiveProductUpsertInput(db, nil, product.ID)
			} else {
				mediaService := newMediaService()
				defer closeMediaService(mediaService)
				input, err = loadCurrentProductUpsertInput(mediaService.DB, mediaService, product.ID)
			}
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}

			if outputPath != "" {
				if err := writeJSONFile(outputPath, input); err != nil {
					return err
				}
				fmt.Printf("product_json_path=%s\n", outputPath)
				return nil
			}

			if selectedFormat == outputFormatJSON {
				printJSON(input)
				return nil
			}

			fmt.Printf("ID: %d\n", product.ID)
			fmt.Printf("SKU: %s\n", input.Sku)
			fmt.Printf("Name: %s\n", input.Name)
			fmt.Printf("Variants: %d\n", len(input.Variants))
			fmt.Printf("Options: %d\n", len(input.Options))
			fmt.Printf("Attributes: %d\n", len(input.Attributes))
			if live {
				fmt.Println("Mode: live")
			} else {
				fmt.Println("Mode: current")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().BoolVar(&live, "live", false, "Print the published/live product instead of the current admin view")
	cmd.Flags().StringVar(&outputPath, "out", "", "Write the JSON document to a file")
	addOutputFormatFlag(cmd, &format, string(outputFormatJSON))
	cmd.MarkFlagsOneRequired("id", "sku")
	return cmd
}

func newApplyProductDraftCmd() *cobra.Command {
	var id, sku string
	var filePath string
	var format string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Replace a product draft from a JSON document",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			product, err := findProductByIDOrSKU(db, id, sku)
			if err != nil {
				return err
			}

			var input apicontract.ProductUpsertInput
			if err := loadJSONFile(filePath, &input); err != nil {
				return err
			}

			updated, err := invokeLocalJSON[apicontract.Product](handlers.UpdateProduct(db), localHandlerRequest{
				Method:     http.MethodPatch,
				Path:       fmt.Sprintf("/api/v1/admin/products/%d", product.ID),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", product.ID)},
				Body:       input,
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(updated)
				return nil
			}

			fmt.Printf("✓ Product draft updated:\n")
			fmt.Printf("  ID: %d\n", updated.Id)
			fmt.Printf("  SKU: %s\n", updated.Sku)
			fmt.Printf("  Name: %s\n", updated.Name)
			fmt.Printf("  Variants: %d\n", len(updated.Variants))
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a product JSON document")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagsOneRequired("id", "sku")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newDiffProductDraftCmd() *cobra.Command {
	var id, sku string
	var format string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show the diff between live and current product draft state",
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaService := newMediaService()
			defer closeMediaService(mediaService)

			product, err := findProductByIDOrSKU(mediaService.DB, id, sku)
			if err != nil {
				return err
			}

			liveInput, err := loadLiveProductUpsertInput(mediaService.DB, mediaService, product.ID)
			if err != nil {
				return err
			}
			currentInput, err := loadCurrentProductUpsertInput(mediaService.DB, mediaService, product.ID)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}

			hasChanges := !reflect.DeepEqual(liveInput, currentInput)
			if selectedFormat == outputFormatJSON {
				printJSON(map[string]any{
					"has_changes": hasChanges,
					"live":        liveInput,
					"current":     currentInput,
				})
				return nil
			}

			if !hasChanges {
				fmt.Println("No draft changes")
				return nil
			}

			diff, err := buildUnifiedJSONDiff(liveInput, currentInput, "live", "current")
			if err != nil {
				return err
			}
			fmt.Println(diff)
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagsOneRequired("id", "sku")
	return cmd
}

func newDiscardProductDraftCmd() *cobra.Command {
	var id, sku string
	var format string

	cmd := &cobra.Command{
		Use:   "discard",
		Short: "Discard a product draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaService := newMediaService()
			defer closeMediaService(mediaService)

			product, err := findProductByIDOrSKU(mediaService.DB, id, sku)
			if err != nil {
				return err
			}

			updated, err := invokeLocalJSON[apicontract.Product](handlers.DiscardProductDraft(mediaService.DB, mediaService), localHandlerRequest{
				Method:     http.MethodDelete,
				Path:       fmt.Sprintf("/api/v1/admin/products/%d/draft", product.ID),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", product.ID)},
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(updated)
				return nil
			}

			fmt.Printf("✓ Product draft discarded for %s (ID: %d)\n", updated.Name, updated.Id)
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagsOneRequired("id", "sku")
	return cmd
}

func newPublishProductCmd() *cobra.Command {
	var id, sku string
	var format string

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish the current product draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaService := newMediaService()
			defer closeMediaService(mediaService)

			product, err := findProductByIDOrSKU(mediaService.DB, id, sku)
			if err != nil {
				return err
			}

			updated, err := invokeLocalJSON[apicontract.Product](handlers.PublishProduct(mediaService.DB, mediaService), localHandlerRequest{
				Method:     http.MethodPost,
				Path:       fmt.Sprintf("/api/v1/admin/products/%d/publish", product.ID),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", product.ID)},
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(updated)
				return nil
			}

			fmt.Printf("✓ Product published:\n")
			fmt.Printf("  ID: %d\n", updated.Id)
			fmt.Printf("  SKU: %s\n", updated.Sku)
			fmt.Printf("  Name: %s\n", updated.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagsOneRequired("id", "sku")
	return cmd
}

func newUnpublishProductCmd() *cobra.Command {
	var id, sku string
	var format string

	cmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Unpublish a product while preserving its draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaService := newMediaService()
			defer closeMediaService(mediaService)

			product, err := findProductByIDOrSKU(mediaService.DB, id, sku)
			if err != nil {
				return err
			}

			updated, err := invokeLocalJSON[apicontract.Product](handlers.UnpublishProduct(mediaService.DB, mediaService), localHandlerRequest{
				Method:     http.MethodPost,
				Path:       fmt.Sprintf("/api/v1/admin/products/%d/unpublish", product.ID),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", product.ID)},
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(updated)
				return nil
			}

			fmt.Printf("✓ Product unpublished for %s (ID: %d)\n", updated.Name, updated.Id)
			return nil
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagsOneRequired("id", "sku")
	return cmd
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeJSON(file, value)
}
