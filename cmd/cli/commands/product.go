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

	return productCmd
}

func newCreateProductCmd() *cobra.Command {
	var sku, name, description string
	var price float64
	var stock int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new product",
		Long:  "Create a new product in the catalog",
		Run: func(cmd *cobra.Command, args []string) {
			db := getDB()
			defer closeDB(db)

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
				Price:       price,
				Stock:       stock,
			}

			if err := db.Create(&product).Error; err != nil {
				log.Fatalf("Error creating product: %v", err)
			}

			fmt.Printf("✓ Product created successfully:\n")
			fmt.Printf("  ID: %d\n", product.ID)
			fmt.Printf("  SKU: %s\n", product.SKU)
			fmt.Printf("  Name: %s\n", product.Name)
			fmt.Printf("  Price: $%.2f\n", product.Price)
			fmt.Printf("  Stock: %d\n", product.Stock)
		},
	}

	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU (required)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Product name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Product description")
	cmd.Flags().Float64VarP(&price, "price", "p", 0, "Product price (required)")
	cmd.Flags().IntVar(&stock, "stock", 0, "Initial stock quantity")
	cmd.MarkFlagRequired("sku")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("price")

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
				product.Price = price
			}

			if cmd.Flags().Changed("stock") {
				product.Stock = stock
			}

			if err := db.Save(&product).Error; err != nil {
				log.Fatalf("Error updating product: %v", err)
			}

			fmt.Printf("✓ Product updated successfully:\n")
			fmt.Printf("  ID: %d\n", product.ID)
			fmt.Printf("  SKU: %s\n", product.SKU)
			fmt.Printf("  Name: %s\n", product.Name)
			fmt.Printf("  Price: $%.2f\n", product.Price)
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
					product.ID, product.SKU, product.Name, product.Price, product.Stock)
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
			db := getDB()
			defer closeDB(db)

			var product models.Product
			var err error

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
					log.Fatalf("Product not found")
				}
				log.Fatalf("Error finding product: %v", err)
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
			db := getDB()
			defer closeDB(db)

			product, err := findProductByIDOrSKU(db, id, sku)
			if err != nil {
				log.Fatal(err)
			}

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
		Long:  "Upload media via TUS and attach it to a product (admin only).",
		Run: func(cmd *cobra.Command, args []string) {
			if filePath == "" {
				log.Fatal("File path is required")
			}
			if token == "" {
				log.Fatal("API token is required")
			}

			db := getDB()
			defer closeDB(db)

			product, err := findProductByIDOrSKU(db, id, sku)
			if err != nil {
				log.Fatal(err)
			}

			mediaID, err := uploadFileToTus(apiBase, token, filePath)
			if err != nil {
				log.Fatalf("Upload failed: %v", err)
			}

			if err := attachMediaToProduct(apiBase, token, product.ID, mediaID); err != nil {
				log.Fatalf("Failed to attach media: %v", err)
			}

			fmt.Printf("✓ Uploaded media %s and attached to %s (ID: %d)\n", mediaID, product.Name, product.ID)
		},
	}

	cmd.Flags().StringVarP(&id, "id", "i", "", "Product ID")
	cmd.Flags().StringVarP(&sku, "sku", "s", "", "Product SKU")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to media file")
	cmd.Flags().StringVar(&apiBase, "api-base", "http://localhost:3000", "API base URL")
	cmd.Flags().StringVar(&token, "token", "", "Admin API token (JWT)")
	cmd.MarkFlagsOneRequired("id", "sku")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("token")

	return cmd
}

func findProductByIDOrSKU(db *gorm.DB, id string, sku string) (models.Product, error) {
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
