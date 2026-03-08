package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	catalogservice "ecommerce/internal/services/catalog"
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

type PublicProduct struct {
	Id              int                          `json:"id"`
	Sku             string                       `json:"sku"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description"`
	Price           float64                      `json:"price"`
	Stock           int                          `json:"stock"`
	Images          []string                     `json:"images"`
	CoverImage      *string                      `json:"cover_image,omitempty"`
	RelatedProducts []apicontract.RelatedProduct `json:"related_products,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	DeletedAt       *time.Time                   `json:"deleted_at,omitempty"`
}

type PublicProductPage struct {
	Data       []PublicProduct        `json:"data"`
	Pagination apicontract.Pagination `json:"pagination"`
}

func toContractProduct(product models.Product) apicontract.Product {
	related := make([]apicontract.RelatedProduct, 0, len(product.Related))
	for _, relatedProduct := range product.Related {
		related = append(related, toContractRelatedProduct(relatedProduct))
	}

	hasDraft := productHasDraft(product)
	published := productIsPubliclyVisible(product)

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
		IsPublished:     &published,
		HasDraftChanges: &hasDraft,
		DraftUpdatedAt:  product.DraftUpdatedAt,
	}
}

func toPublicProduct(product models.Product) PublicProduct {
	related := make([]apicontract.RelatedProduct, 0, len(product.Related))
	for _, relatedProduct := range product.Related {
		related = append(related, toContractRelatedProduct(relatedProduct))
	}
	return PublicProduct{
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

func publicProductPayload(product apicontract.Product) (map[string]any, error) {
	payload, err := json.Marshal(product)
	if err != nil {
		return nil, err
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	delete(decoded, "has_draft_changes")
	delete(decoded, "draft_updated_at")
	delete(decoded, "is_published")
	return decoded, nil
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
	catalog := catalogservice.NewService(db, mediaService)
	return func(c *gin.Context) {
		preview := isDraftPreviewActive(c)
		input := buildCatalogListInput(c, preview, 10)
		list, err := catalog.ListProducts(input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}
		products := list.Products

		contractProducts := make([]apicontract.Product, 0, len(products))
		publicProducts := make([]map[string]any, 0, len(products))
		for _, product := range products {
			contractProduct, err := buildProductContract(db, mediaService, product, preview, false, preview)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
				return
			}
			if !preview {
				publicProduct, err := publicProductPayload(contractProduct)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
					return
				}
				publicProducts = append(publicProducts, publicProduct)
			}
			contractProducts = append(contractProducts, contractProduct)
		}
		page := input.Page
		limit := input.Limit

		if !preview {
			c.JSON(http.StatusOK, gin.H{
				"data": publicProducts,
				"pagination": apicontract.Pagination{
					Page:       page,
					Limit:      limit,
					Total:      int(list.Total),
					TotalPages: list.TotalPages,
				},
			})
			return
		}

		c.JSON(http.StatusOK, apicontract.ProductPage{
			Data: contractProducts,
			Pagination: apicontract.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      int(list.Total),
				TotalPages: list.TotalPages,
			},
		})
	}
}

// GetProductByID retrieves a specific product and its related items
func GetProductByID(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	catalog := catalogservice.NewService(db, mediaService)
	return func(c *gin.Context) {
		id := c.Param("id")
		preview := isDraftPreviewActive(c)
		product, err := catalog.GetProductByID(id, preview)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product"})
			return
		}

		if preview {
			if !productIsPubliclyVisible(product) && !productHasDraft(product) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
				return
			}
		}

		if !preview && !productIsPubliclyVisible(product) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		contractProduct, err := buildProductContract(db, mediaService, product, preview, true, preview)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
			return
		}
		if !preview {
			publicProduct, err := publicProductPayload(contractProduct)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
				return
			}
			c.JSON(http.StatusOK, publicProduct)
			return
		}
		c.JSON(http.StatusOK, contractProduct)
	}
}
