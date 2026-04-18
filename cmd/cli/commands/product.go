package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func NewProductCmd() *cobra.Command {
	productCmd := &cobra.Command{
		Use:   "product",
		Short: "Product management commands",
		Long:  "Commands for managing products: create, list, delete, etc.",
	}

	productCmd.AddCommand(newCreateProductCmd())
	productCmd.AddCommand(newEditProductCmd())
	productCmd.AddCommand(newListProductsCmd())
	productCmd.AddCommand(newDeleteProductCmd())
	productCmd.AddCommand(newSetRelatedProductsCmd())
	productCmd.AddCommand(newUploadProductMediaCmd())
	productCmd.AddCommand(newPrintProductCmd())
	productCmd.AddCommand(newApplyProductDraftCmd())
	productCmd.AddCommand(newDiffProductDraftCmd())
	productCmd.AddCommand(newDiscardProductDraftCmd())
	productCmd.AddCommand(newPublishProductCmd())
	productCmd.AddCommand(newUnpublishProductCmd())

	return productCmd
}

func newCreateProductCmd() *cobra.Command {
	var sku, name, description string
	var price float64
	var stock int
	var filePath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new product",
		Long:  "Create a new product in the catalog",
		Run: func(cmd *cobra.Command, args []string) {
			if isRemoteMode() {
				var input apicontract.ProductUpsertInput
				if strings.TrimSpace(filePath) != "" {
					if err := loadJSONFile(filePath, &input); err != nil {
						log.Fatalf("Error loading product JSON: %v", err)
					}
				} else {
					if sku == "" || name == "" {
						log.Fatal("SKU and name are required")
					}
					if price <= 0 {
						log.Fatal("Price must be greater than 0")
					}
					input = buildSimpleProductUpsertInput(sku, name, description, price, stock)
				}

				product, err := invokeRemoteJSON[apicontract.Product](http.MethodPost, "/api/v1/admin/products", input)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("✓ Product created successfully:\n")
				fmt.Printf("  ID: %d\n", product.Id)
				fmt.Printf("  SKU: %s\n", product.Sku)
				fmt.Printf("  Name: %s\n", product.Name)
				fmt.Printf("  Variants: %d\n", len(product.Variants))
				return
			}

			db := getDB()
			defer closeDB(db)

			if strings.TrimSpace(filePath) != "" {
				var input apicontract.ProductUpsertInput
				if err := loadJSONFile(filePath, &input); err != nil {
					log.Fatalf("Error loading product JSON: %v", err)
				}

				product, err := invokeLocalJSON[apicontract.Product](handlers.CreateProduct(db), localHandlerRequest{
					Method: http.MethodPost,
					Path:   "/api/v1/admin/products",
					Body:   input,
				})
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("✓ Product created successfully:\n")
				fmt.Printf("  ID: %d\n", product.Id)
				fmt.Printf("  SKU: %s\n", product.Sku)
				fmt.Printf("  Name: %s\n", product.Name)
				fmt.Printf("  Variants: %d\n", len(product.Variants))
				return
			}

			if sku == "" || name == "" {
				log.Fatal("SKU and name are required")
			}

			if price <= 0 {
				log.Fatal("Price must be greater than 0")
			}

			// Check if SKU already exists
			var existingProduct models.Product
			if err := db.Where("sku = ?", sku).First(&existingProduct).Error; err == nil {
				log.Fatalf("Product with SKU '%s' already exists", sku)
			}

			product := models.Product{
				SKU:         sku,
				Name:        name,
				Description: description,
				Price:       models.MoneyFromFloat(price),
				Stock:       stock,
			}

			if err := db.Create(&product).Error; err != nil {
				log.Fatalf("Error creating product: %v", err)
			}
			variant := models.ProductVariant{
				ProductID:   product.ID,
				SKU:         product.SKU,
				Title:       product.Name,
				Price:       product.Price,
				Stock:       product.Stock,
				Position:    1,
				IsPublished: product.IsPublished,
			}
			if err := db.Create(&variant).Error; err != nil {
				log.Fatalf("Error creating default product variant: %v", err)
			}
			if err := db.Model(&product).Update("default_variant_id", variant.ID).Error; err != nil {
				log.Fatalf("Error linking default product variant: %v", err)
			}

			fmt.Printf("✓ Product created successfully:\n")
			fmt.Printf("  ID: %d\n", product.ID)
			fmt.Printf("  SKU: %s\n", product.SKU)
			fmt.Printf("  Name: %s\n", product.Name)
			fmt.Printf("  Price: $%.2f\n", product.Price.Float64())
			fmt.Printf("  Stock: %d\n", product.Stock)
		},
	}

	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU (required)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Product name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Product description")
	cmd.Flags().Float64VarP(&price, "price", "p", 0, "Product price (required)")
	cmd.Flags().IntVar(&stock, "stock", 0, "Initial stock quantity")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a full product JSON payload")

	return cmd
}

func newEditProductCmd() *cobra.Command {
	var (
		id          string
		sku         string
		newSKU      string
		name        string
		description string
		price       float64
		stock       int
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing product",
		Long:  "Edit an existing product by ID or SKU. Only provided fields will be updated.",
		Run: func(cmd *cobra.Command, args []string) {
			if isRemoteMode() {
				product, err := findProductByIDOrSKU(nil, id, sku)
				if err != nil {
					log.Fatal(err)
				}

				input, err := loadCurrentProductUpsertInput(nil, nil, product.ID)
				if err != nil {
					log.Fatal(err)
				}
				previousSKU := input.Sku
				defaultVariantSKU := previousSKU
				if input.DefaultVariantSku != nil && strings.TrimSpace(*input.DefaultVariantSku) != "" {
					defaultVariantSKU = *input.DefaultVariantSku
				}

				if cmd.Flags().Changed("new-sku") {
					input.Sku = newSKU
					if input.DefaultVariantSku != nil && *input.DefaultVariantSku == previousSKU {
						value := newSKU
						input.DefaultVariantSku = &value
					}
					for i := range input.Variants {
						if input.Variants[i].Sku == previousSKU {
							input.Variants[i].Sku = newSKU
						}
					}
				}
				if cmd.Flags().Changed("name") {
					input.Name = name
				}
				if cmd.Flags().Changed("description") {
					input.Description = description
				}
				if cmd.Flags().Changed("price") {
					if price <= 0 {
						log.Fatal("Price must be greater than 0")
					}
					for i := range input.Variants {
						if input.Variants[i].Sku == defaultVariantSKU || len(input.Variants) == 1 {
							input.Variants[i].Price = price
						}
					}
				}
				if cmd.Flags().Changed("stock") {
					for i := range input.Variants {
						if input.Variants[i].Sku == defaultVariantSKU || len(input.Variants) == 1 {
							input.Variants[i].Stock = stock
						}
					}
				}

				updated, err := invokeRemoteJSON[apicontract.Product](http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), input)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("✓ Product updated successfully:\n")
				fmt.Printf("  ID: %d\n", updated.Id)
				fmt.Printf("  SKU: %s\n", updated.Sku)
				fmt.Printf("  Name: %s\n", updated.Name)
				fmt.Printf("  Price: $%.2f\n", updated.Price)
				fmt.Printf("  Stock: %d\n", updated.Stock)
				return
			}

			db := getDB()
			defer closeDB(db)

			var product models.Product
			var err error

			// Locate product
			if id != "" {
				productID, err := strconv.ParseUint(id, 10, 32)
				if err != nil {
					log.Fatalf("Invalid product ID: %v", err)
				}
				err = db.First(&product, productID).Error
			} else if sku != "" {
				err = db.Where("sku = ?", sku).First(&product).Error
			} else {
				log.Fatal("Either --id or --sku must be provided")
			}

			if err != nil {
				if err == gorm.ErrRecordNotFound {
					log.Fatal("Product not found")
				}
				log.Fatalf("Error finding product: %v", err)
			}

			// Apply updates only if flags were set
			if cmd.Flags().Changed("new-sku") {
				var existing models.Product
				if err := db.Where("sku = ?", newSKU).First(&existing).Error; err == nil {
					log.Fatalf("Another product with SKU '%s' already exists", newSKU)
				}
				product.SKU = newSKU
			}

			if cmd.Flags().Changed("name") {
				product.Name = name
			}

			if cmd.Flags().Changed("description") {
				product.Description = description
			}

			if cmd.Flags().Changed("price") {
				if price <= 0 {
					log.Fatal("Price must be greater than 0")
				}
				product.Price = models.MoneyFromFloat(price)
			}

			if cmd.Flags().Changed("stock") {
				product.Stock = stock
			}

			if err := db.Save(&product).Error; err != nil {
				log.Fatalf("Error updating product: %v", err)
			}

			if product.DefaultVariantID != nil {
				updates := map[string]any{
					"sku":   product.SKU,
					"title": product.Name,
				}
				if cmd.Flags().Changed("price") {
					updates["price"] = product.Price
				}
				if cmd.Flags().Changed("stock") {
					updates["stock"] = product.Stock
				}
				if err := db.Model(&models.ProductVariant{}).Where("id = ?", *product.DefaultVariantID).Updates(updates).Error; err != nil {
					log.Fatalf("Error updating default variant: %v", err)
				}
			}

			fmt.Printf("✓ Product updated successfully:\n")
			fmt.Printf("  ID: %d\n", product.ID)
			fmt.Printf("  SKU: %s\n", product.SKU)
			fmt.Printf("  Name: %s\n", product.Name)
			fmt.Printf("  Price: $%.2f\n", product.Price.Float64())
			fmt.Printf("  Stock: %d\n", product.Stock)
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")

	cmd.Flags().StringVar(&newSKU, "new-sku", "", "New product SKU")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Product name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Product description")
	cmd.Flags().Float64VarP(&price, "price", "p", 0, "Product price")
	cmd.Flags().IntVar(&stock, "stock", 0, "Stock quantity")

	cmd.MarkFlagsOneRequired("id", "sku")

	return cmd
}

func newListProductsCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List products",
		Long:  "List all products in the catalog",
		Run: func(cmd *cobra.Command, args []string) {
			if isRemoteMode() {
				path := "/api/v1/admin/products"
				if limit > 0 {
					path += "?limit=" + strconv.Itoa(limit)
				}
				page, err := invokeRemoteJSON[apicontract.ProductPage](http.MethodGet, path, nil)
				if err != nil {
					log.Fatalf("Error listing products: %v", err)
				}
				if len(page.Data) == 0 {
					fmt.Println("No products found")
					return
				}

				fmt.Printf("Found %d product(s):\n\n", len(page.Data))
				fmt.Printf("%-5s %-15s %-30s %-10s %-10s\n", "ID", "SKU", "Name", "Price", "Stock")
				fmt.Println("--------------------------------------------------------------------------------")
				for _, product := range page.Data {
					fmt.Printf("%-5d %-15s %-30s $%-9.2f %-10d\n",
						product.Id, product.Sku, product.Name, product.Price, product.Stock)
				}
				return
			}

			db := getDB()
			defer closeDB(db)

			var products []models.Product
			query := db.Model(&models.Product{})

			if limit > 0 {
				query = query.Limit(limit)
			}

			if err := query.Find(&products).Error; err != nil {
				log.Fatalf("Error listing products: %v", err)
			}

			if len(products) == 0 {
				fmt.Println("No products found")
				return
			}

			fmt.Printf("Found %d product(s):\n\n", len(products))
			fmt.Printf("%-5s %-15s %-30s %-10s %-10s\n", "ID", "SKU", "Name", "Price", "Stock")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, product := range products {
				fmt.Printf("%-5d %-15s %-30s $%-9.2f %-10d\n",
					product.ID, product.SKU, product.Name, product.Price.Float64(), product.Stock)
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit number of results")

	return cmd
}

func newDeleteProductCmd() *cobra.Command {
	var id, sku string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a product",
		Long:  "Delete a product by ID or SKU",
		Run: func(cmd *cobra.Command, args []string) {
			product, err := findProductByIDOrSKU(nil, id, sku)
			if err != nil {
				log.Fatal(err)
			}

			if !confirm {
				fmt.Printf("Are you sure you want to delete product '%s' (SKU: %s)? (yes/no): ", product.Name, product.SKU)
				var response string
				fmt.Scanln(&response)
				if response != "yes" && response != "y" {
					fmt.Println("Cancelled")
					return
				}
			}

			if isRemoteMode() {
				if _, err := invokeRemoteJSON[apicontract.MessageResponse](http.MethodDelete, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), nil); err != nil {
					log.Fatalf("Error deleting product: %v", err)
				}
				fmt.Printf("✓ Product '%s' (SKU: %s) has been deleted\n", product.Name, product.SKU)
				return
			}

			db := getDB()
			defer closeDB(db)
			if err := db.Delete(&product).Error; err != nil {
				log.Fatalf("Error deleting product: %v", err)
			}

			fmt.Printf("✓ Product '%s' (SKU: %s) has been deleted\n", product.Name, product.SKU)
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().BoolVarP(&confirm, "yes", "y", false, "Skip confirmation prompt")
	cmd.MarkFlagsOneRequired("id", "sku")

	return cmd
}

func newSetRelatedProductsCmd() *cobra.Command {
	var id, sku string
	var relatedIDs []int
	var relatedSKUs []string
	var clear bool

	cmd := &cobra.Command{
		Use:   "related-set",
		Short: "Replace related products for a product",
		Long:  "Replace the related products list using product IDs or SKUs.",
		Run: func(cmd *cobra.Command, args []string) {
			product, err := findProductByIDOrSKU(nil, id, sku)
			if err != nil {
				log.Fatal(err)
			}

			if isRemoteMode() {
				if clear {
					updated, err := invokeRemoteJSON[apicontract.Product](http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d/related", product.ID), map[string]any{
						"related_product_ids": []int{},
					})
					if err != nil {
						log.Fatalf("Error clearing related products: %v", err)
					}
					fmt.Printf("✓ Related products cleared for %s (ID: %d)\n", updated.Name, updated.Id)
					return
				}

				if len(relatedIDs) == 0 && len(relatedSKUs) == 0 {
					log.Fatal("Provide at least one --related-id or --related-sku, or use --clear")
				}

				resolvedIDs := append([]int(nil), relatedIDs...)
				for _, relatedSKU := range relatedSKUs {
					relatedProduct, err := findProductByIDOrSKU(nil, "", relatedSKU)
					if err != nil {
						log.Fatalf("Error loading related product %q: %v", relatedSKU, err)
					}
					resolvedIDs = append(resolvedIDs, int(relatedProduct.ID))
				}

				filteredIDs := resolvedIDs[:0]
				seen := make(map[int]struct{}, len(resolvedIDs))
				for _, relatedID := range resolvedIDs {
					if relatedID == int(product.ID) {
						continue
					}
					if _, ok := seen[relatedID]; ok {
						continue
					}
					seen[relatedID] = struct{}{}
					filteredIDs = append(filteredIDs, relatedID)
				}

				updated, err := invokeRemoteJSON[apicontract.Product](http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d/related", product.ID), map[string]any{
					"related_product_ids": filteredIDs,
				})
				if err != nil {
					log.Fatalf("Error setting related products: %v", err)
				}

				fmt.Printf("✓ Related products updated for %s (ID: %d)\n", updated.Name, updated.Id)
				return
			}

			db := getDB()
			defer closeDB(db)

			if clear {
				if err := db.Model(&product).Association("Related").Clear(); err != nil {
					log.Fatalf("Error clearing related products: %v", err)
				}
				fmt.Printf("✓ Related products cleared for %s (ID: %d)\n", product.Name, product.ID)
				return
			}

			if len(relatedIDs) == 0 && len(relatedSKUs) == 0 {
				log.Fatal("Provide at least one --related-id or --related-sku, or use --clear")
			}

			var relatedProducts []models.Product
			if len(relatedIDs) > 0 {
				if err := db.Where("id IN ?", relatedIDs).Find(&relatedProducts).Error; err != nil {
					log.Fatalf("Error loading related products: %v", err)
				}
			}
			if len(relatedSKUs) > 0 {
				var bySKU []models.Product
				if err := db.Where("sku IN ?", relatedSKUs).Find(&bySKU).Error; err != nil {
					log.Fatalf("Error loading related products by SKU: %v", err)
				}
				relatedProducts = append(relatedProducts, bySKU...)
			}

			filtered := relatedProducts[:0]
			for _, rel := range relatedProducts {
				if rel.ID == product.ID {
					continue
				}
				filtered = append(filtered, rel)
			}

			if err := db.Model(&product).Association("Related").Replace(filtered); err != nil {
				log.Fatalf("Error setting related products: %v", err)
			}

			fmt.Printf("✓ Related products updated for %s (ID: %d)\n", product.Name, product.ID)
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().IntSliceVar(&relatedIDs, "related-id", nil, "Related product IDs (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&relatedSKUs, "related-sku", nil, "Related product SKUs (repeatable or comma-separated)")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear all related products")
	cmd.MarkFlagsOneRequired("id", "sku")

	return cmd
}

func newUploadProductMediaCmd() *cobra.Command {
	var id, sku string
	var filePath string
	var apiBase string
	var token string

	cmd := &cobra.Command{
		Use:   "media-upload",
		Short: "Upload media and attach to a product",
		Long:  "Upload media and attach it to a product. Without --token, the CLI imports media directly using the local config and filesystem.",
		Run: func(cmd *cobra.Command, args []string) {
			if filePath == "" {
				log.Fatal("File path is required")
			}

			product, err := findProductByIDOrSKU(nil, id, sku)
			if err != nil {
				log.Fatal(err)
			}

			if isRemoteMode() {
				auth, err := currentRemoteAuth()
				if err != nil {
					log.Fatal(err)
				}
				if strings.TrimSpace(apiBase) == "" || apiBase == "http://localhost:3000" {
					apiBase = auth.APIURL
				}
				if strings.TrimSpace(token) == "" {
					token = auth.Token
				}
			}

			if token != "" {
				mediaID, err := uploadFileToTus(apiBase, token, filePath)
				if err != nil {
					log.Fatalf("Upload failed: %v", err)
				}

				if err := attachMediaToProduct(apiBase, token, product.ID, mediaID); err != nil {
					log.Fatalf("Failed to attach media: %v", err)
				}

				fmt.Printf("✓ Uploaded media %s and attached to %s (ID: %d)\n", mediaID, product.Name, product.ID)
				return
			}

			cfg := getConfig()
			db := getDBWithConfig(cfg)
			defer closeDB(db)

			if err := media.CheckDependencies(); err != nil {
				log.Fatalf("Media upload dependencies unavailable: %v", err)
			}

			mediaService := media.NewService(db, cfg.MediaRoot, cfg.MediaPublicURL, log.Default())
			if err := mediaService.EnsureDirs(); err != nil {
				log.Fatalf("Failed to initialize media directories: %v", err)
			}

			mediaObj, err := mediaService.ImportFile(filePath)
			if err != nil {
				log.Fatalf("Upload failed: %v", err)
			}

			if err := handlers.AttachProductMediaToDraft(db, mediaService, &product, []string{mediaObj.ID}); err != nil {
				_ = mediaService.DeleteIfOrphan(mediaObj.ID)
				log.Fatalf("Failed to attach media: %v", err)
			}

			fmt.Printf("✓ Uploaded media %s and attached to %s (ID: %d)\n", mediaObj.ID, product.Name, product.ID)
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to media file")
	cmd.Flags().StringVar(&apiBase, "api-base", "http://localhost:3000", "API base URL")
	cmd.Flags().StringVar(&token, "token", "", "Admin API token (JWT); when omitted, upload runs directly against the local media root and database")
	cmd.MarkFlagsOneRequired("id", "sku")
	cmd.MarkFlagRequired("file")

	return cmd
}

func findProductByIDOrSKU(db *gorm.DB, id string, sku string) (models.Product, error) {
	if isRemoteMode() {
		contract, err := findRemoteProductContractByIDOrSKU(id, sku)
		if err != nil {
			return models.Product{}, err
		}
		return contractProductToModel(contract), nil
	}

	if db == nil {
		db = getDB()
		defer closeDB(db)
	}

	var product models.Product
	var err error

	if id != "" {
		productID, parseErr := strconv.ParseUint(id, 10, 32)
		if parseErr != nil {
			return product, fmt.Errorf("invalid product ID: %w", parseErr)
		}
		err = db.First(&product, productID).Error
	} else if sku != "" {
		err = db.Where("sku = ?", sku).First(&product).Error
	} else {
		return product, fmt.Errorf("either --id or --sku must be provided")
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return product, fmt.Errorf("product not found")
		}
		return product, fmt.Errorf("error finding product: %w", err)
	}

	return product, nil
}

func findRemoteProductContractByIDOrSKU(id string, sku string) (apicontract.Product, error) {
	if strings.TrimSpace(id) != "" {
		productID, parseErr := strconv.ParseUint(id, 10, 32)
		if parseErr != nil {
			return apicontract.Product{}, fmt.Errorf("invalid product ID: %w", parseErr)
		}
		return invokeRemoteJSON[apicontract.Product](http.MethodGet, fmt.Sprintf("/api/v1/admin/products/%d", productID), nil)
	}

	if strings.TrimSpace(sku) == "" {
		return apicontract.Product{}, fmt.Errorf("either --id or --sku must be provided")
	}

	page, err := invokeRemoteJSON[apicontract.ProductPage](http.MethodGet, "/api/v1/admin/products?q="+url.QueryEscape(strings.TrimSpace(sku))+"&limit=100", nil)
	if err != nil {
		return apicontract.Product{}, err
	}
	for _, product := range page.Data {
		if product.Sku == strings.TrimSpace(sku) {
			return product, nil
		}
	}

	return apicontract.Product{}, fmt.Errorf("product not found")
}

func contractProductToModel(product apicontract.Product) models.Product {
	result := models.Product{
		SKU:         product.Sku,
		Name:        product.Name,
		Description: product.Description,
		Price:       models.MoneyFromFloat(product.Price),
		Stock:       product.Stock,
	}
	result.ID = uint(product.Id)
	return result
}

func buildSimpleProductUpsertInput(sku string, name string, description string, price float64, stock int) apicontract.ProductUpsertInput {
	defaultVariantSKU := strings.TrimSpace(sku)
	position := 1
	isPublished := false

	return apicontract.ProductUpsertInput{
		Description:       description,
		Name:              strings.TrimSpace(name),
		Sku:               defaultVariantSKU,
		DefaultVariantSku: &defaultVariantSKU,
		Variants: []apicontract.ProductVariantInput{
			{
				IsPublished: &isPublished,
				Position:    &position,
				Price:       price,
				Sku:         defaultVariantSKU,
				Stock:       stock,
				Title:       strings.TrimSpace(name),
			},
		},
	}
}

func uploadFileToTus(apiBase string, token string, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", err
	}

	base := strings.TrimRight(apiBase, "/")
	createURL := base + "/api/v1/media/uploads"

	filename := path.Base(filePath)
	metadata := "filename " + base64.StdEncoding.EncodeToString([]byte(filename))

	req, err := http.NewRequest(http.MethodPost, createURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", fmt.Sprintf("%d", info.Size()))
	req.Header.Set("Upload-Metadata", metadata)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tus create failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("tus create missing Location header")
	}

	uploadURL := location
	if strings.HasPrefix(location, "/") {
		uploadURL = base + location
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	patchReq, err := http.NewRequest(http.MethodPatch, uploadURL, file)
	if err != nil {
		return "", err
	}
	patchReq.Header.Set("Tus-Resumable", "1.0.0")
	patchReq.Header.Set("Upload-Offset", "0")
	patchReq.Header.Set("Content-Type", "application/offset+octet-stream")
	patchReq.Header.Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	patchReq.Header.Set("Authorization", "Bearer "+token)

	patchResp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		return "", err
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(patchResp.Body)
		return "", fmt.Errorf("tus patch failed: %s (%s)", patchResp.Status, strings.TrimSpace(string(body)))
	}

	mediaID, err := extractMediaID(location)
	if err != nil {
		return "", err
	}

	return mediaID, nil
}

func extractMediaID(location string) (string, error) {
	parsed, err := url.Parse(location)
	if err != nil || parsed.Path == "" {
		return "", fmt.Errorf("invalid tus location: %s", location)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid tus location: %s", location)
	}
	return parts[len(parts)-1], nil
}

func attachMediaToProduct(apiBase string, token string, productID uint, mediaID string) error {
	base := strings.TrimRight(apiBase, "/")
	target := fmt.Sprintf("%s/api/v1/admin/products/%d/media", base, productID)

	payload := map[string]any{"media_ids": []string{mediaID}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("attach failed: %s (%s)", resp.Status, strings.TrimSpace(string(respBody)))
	}

	return nil
}
