package handlers

import (
	"net/http"
	"strconv"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func toContractDeletedAt(deletedAt gorm.DeletedAt) *time.Time {
	if !deletedAt.Valid {
		return nil
	}
	value := deletedAt.Time
	return &value
}

func toContractRelatedProduct(product models.Product) apicontract.RelatedProduct {
	var price *float64
	priceValue := product.Price.Float64()
	price = &priceValue

	description := product.Description
	return apicontract.RelatedProduct{
		Id:          int(product.ID),
		Sku:         product.SKU,
		Name:        product.Name,
		Description: &description,
		Price:       price,
		CoverImage:  product.CoverImage,
		Stock:       product.Stock,
	}
}

func toContractProduct(product models.Product) apicontract.Product {
	related := make([]apicontract.RelatedProduct, 0, len(product.Related))
	for _, relatedProduct := range product.Related {
		related = append(related, toContractRelatedProduct(relatedProduct))
	}

	return apicontract.Product{
		Id:              int(product.ID),
		Sku:             product.SKU,
		Name:            product.Name,
		Description:     product.Description,
		Price:           product.Price.Float64(),
		Stock:           product.Stock,
		Images:          product.Images,
		CoverImage:      product.CoverImage,
		RelatedProducts: related,
		CreatedAt:       product.CreatedAt,
		UpdatedAt:       product.UpdatedAt,
		DeletedAt:       toContractDeletedAt(product.DeletedAt),
	}
}

// GetProducts handles search, filtering, sorting, and pagination
// Query parameters:
//   - q: search term (searches in name)
//   - min_price: minimum price filter
//   - max_price: maximum price filter
//   - sort: sort field (price, name, created_at) - default: created_at
//   - order: sort order (asc, desc) - default: desc
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func GetProducts(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.Product{})

		// Search by name if query parameter 'q' is present
		if searchTerm := c.Query("q"); searchTerm != "" {
			query = query.Where("name ILIKE ?", "%"+searchTerm+"%")
		}

		// Price range filtering
		if minPriceStr := c.Query("min_price"); minPriceStr != "" {
			if minPrice, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
				query = query.Where("price >= ?", minPrice)
			}
		}
		if maxPriceStr := c.Query("max_price"); maxPriceStr != "" {
			if maxPrice, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
				query = query.Where("price <= ?", maxPrice)
			}
		}

		// Sorting
		sortField := c.DefaultQuery("sort", "created_at")
		sortOrder := c.DefaultQuery("order", "desc")

		// Validate sort field
		validSortFields := map[string]bool{
			"price": true, "name": true, "created_at": true,
		}
		if !validSortFields[sortField] {
			sortField = "created_at"
		}

		// Validate sort order
		if sortOrder != "asc" && sortOrder != "desc" {
			sortOrder = "desc"
		}

		query = query.Order(sortField + " " + sortOrder)

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		offset := (page - 1) * limit

		// Get total count for pagination metadata
		var total int64
		query.Count(&total)

		// Fetch products
		var products []models.Product
		if err := query.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}

		// Calculate pagination metadata
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}

		for i := range products {
			if mediaService != nil {
				mediaURLs, err := mediaService.ProductMediaURLs(products[i].ID)
				if err == nil && len(mediaURLs) > 0 {
					products[i].Images = mediaURLs
					products[i].CoverImage = &mediaURLs[0]
					continue
				}
			}
			if len(products[i].Images) > 0 {
				products[i].CoverImage = &products[i].Images[0]
			}
		}

		contractProducts := make([]apicontract.Product, 0, len(products))
		for _, product := range products {
			contractProducts = append(contractProducts, toContractProduct(product))
		}

		c.JSON(http.StatusOK, apicontract.ProductPage{
			Data: contractProducts,
			Pagination: apicontract.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      int(total),
				TotalPages: totalPages,
			},
		})
	}
}

// GetProductByID retrieves a specific product and its related items
func GetProductByID(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		// Preload "Related" items to populate the related_products field
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if mediaService != nil {
			mediaURLs, err := mediaService.ProductMediaURLs(product.ID)
			if err == nil && len(mediaURLs) > 0 {
				product.Images = mediaURLs
				product.CoverImage = &mediaURLs[0]
			}
		}
		if product.CoverImage == nil && len(product.Images) > 0 {
			product.CoverImage = &product.Images[0]
		}

		for i := range product.Related {
			if mediaService != nil {
				mediaURLs, err := mediaService.ProductMediaURLs(product.Related[i].ID)
				if err == nil && len(mediaURLs) > 0 {
					product.Related[i].Images = mediaURLs
					product.Related[i].CoverImage = &mediaURLs[0]
					continue
				}
			}
			if len(product.Related[i].Images) > 0 {
				product.Related[i].CoverImage = &product.Related[i].Images[0]
			}
		}

		c.JSON(http.StatusOK, toContractProduct(product))
	}
}
