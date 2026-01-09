package commands

import (
	"ecommerce/models"
	"fmt"
	"log"
	"strconv"

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
