package handlers

import (
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
		page, limit, offset := parsePagination(c, 10)
		_ = offset

		minPrice, _ := catalogservice.ParsePrice(c.Query("min_price"))
		maxPrice, _ := catalogservice.ParsePrice(c.Query("max_price"))
		list, err := catalog.ListProducts(catalogservice.ListProductsInput{
			SearchTerm: c.Query("q"),
			MinPrice:   minPrice,
			MaxPrice:   maxPrice,
			SortField:  c.DefaultQuery("sort", "created_at"),
			SortOrder:  c.DefaultQuery("order", "desc"),
			Page:       page,
			Limit:      limit,
			Preview:    preview,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}
		products := list.Products

		for i := range products {
			if preview {
				view, err := materializeAdminProduct(db, mediaService, products[i], false)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
					return
				}
				products[i] = view
				continue
			}

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
		publicProducts := make([]PublicProduct, 0, len(products))
		for _, product := range products {
			if preview {
				contractProducts = append(contractProducts, toContractProduct(product))
				continue
			}
			publicProducts = append(publicProducts, toPublicProduct(product))
		}

		if !preview {
			c.JSON(http.StatusOK, PublicProductPage{
				Data: publicProducts,
				Pagination: apicontract.Pagination{
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
			view, err := materializeAdminProduct(db, mediaService, product, true)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
				return
			}
			c.JSON(http.StatusOK, toContractProduct(view))
			return
		}

		if !productIsPubliclyVisible(product) {
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

		c.JSON(http.StatusOK, toPublicProduct(product))
	}
}
