package handlers

import (
	"errors"
	"net/http"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	catalogservice "ecommerce/internal/services/catalog"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UpdateProductRequest struct {
	SKU         *string   `json:"sku"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Price       *float64  `json:"price"`
	Stock       *int      `json:"stock"`
	Images      *[]string `json:"images"`
}

type UpdateRelatedRequest struct {
	RelatedIDs []uint `json:"related_ids"`
}

func ensureUniqueProductSKU(db *gorm.DB, sku string, excludedID uint) error {
	var existing models.Product
	err := db.Where("sku = ? AND id <> ?", sku, excludedID).First(&existing).Error
	if err == nil {
		return errors.New("Product with this SKU already exists")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func mediaIDsFromRefs(refs []models.MediaReference) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.MediaID)
	}
	return ids
}

func diffMediaIDs(previous []models.MediaReference, keep []models.MediaReference) []string {
	active := make(map[string]struct{}, len(keep))
	for _, ref := range keep {
		active[ref.MediaID] = struct{}{}
	}

	removed := make([]string, 0, len(previous))
	for _, ref := range previous {
		if _, exists := active[ref.MediaID]; exists {
			continue
		}
		removed = append(removed, ref.MediaID)
	}
	return removed
}

func relatedProductsByIDs(db *gorm.DB, ids []uint) ([]models.Product, error) {
	if len(ids) == 0 {
		return []models.Product{}, nil
	}

	var raw []models.Product
	if err := db.Where("id IN ?", ids).Find(&raw).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]models.Product, len(raw))
	for _, product := range raw {
		if productHasDraft(product) {
			draft, hasDraft, err := loadNormalizedProductDraft(db, product)
			if err != nil {
				return nil, err
			}
			if hasDraft {
				if derived, deriveErr := deriveCatalogMerchandising(draft); deriveErr == nil {
					draft = derived
				}
				product = applyCatalogDraftToProduct(product, draft)
			}
		}
		byID[product.ID] = product
	}

	ordered := make([]models.Product, 0, len(ids))
	for _, id := range ids {
		product, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, product)
	}
	return ordered, nil
}

func applyCatalogDraftToProduct(product models.Product, draft productCatalogDraft) models.Product {
	product.SKU = draft.SKU
	product.Name = draft.Name
	product.Subtitle = draft.Subtitle
	product.Description = draft.Description
	product.Price = models.MoneyFromFloat(draft.Price)
	product.Stock = draft.Stock
	product.Images = append([]string(nil), draft.Images...)
	product.BrandID = draft.BrandID
	return product
}

func applyProductMediaWithRole(product *models.Product, mediaService *media.Service, role string, fallbackImages []string) {
	if product == nil {
		return
	}

	product.Images = append([]string(nil), fallbackImages...)
	product.CoverImage = nil

	if mediaService != nil {
		mediaURLs, err := mediaService.ProductMediaURLsByRole(product.ID, role)
		if err == nil && len(mediaURLs) > 0 {
			product.Images = mediaURLs
		}
	}
	if len(product.Images) > 0 {
		product.CoverImage = &product.Images[0]
	}
}

func ListAdminProducts(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	catalog := catalogservice.NewService(db, mediaService)
	return func(c *gin.Context) {
		input := buildCatalogListInput(c, true, 10)
		list, err := catalog.ListProducts(input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}

		contractProducts := make([]apicontract.Product, 0, len(list.Products))
		for _, product := range list.Products {
			view, viewErr := buildProductContract(db, mediaService, product, true, false, true)
			if viewErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
				return
			}
			contractProducts = append(contractProducts, view)
		}

		c.JSON(http.StatusOK, apicontract.ProductPage{
			Data: contractProducts,
			Pagination: apicontract.Pagination{
				Page:       input.Page,
				Limit:      input.Limit,
				Total:      int(list.Total),
				TotalPages: list.TotalPages,
			},
		})
	}
}

func GetAdminProductByID(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		view, err := buildProductContract(db, mediaService, product, true, true, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusOK, view)
	}
}

func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.ProductUpsertInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		draft := catalogDraftFromUpsertInput(req)
		derived, err := deriveCatalogMerchandising(draft)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		product := models.Product{
			SKU:            derived.SKU,
			Name:           derived.Name,
			Subtitle:       derived.Subtitle,
			Description:    derived.Description,
			Price:          models.MoneyFromFloat(derived.Price),
			Stock:          derived.Stock,
			Images:         append([]string(nil), derived.Images...),
			BrandID:        derived.BrandID,
			IsPublished:    false,
			DraftUpdatedAt: ptrTimeNow(),
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := validateProductCatalogDraft(tx, draft, 0); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&product).Error; err != nil {
				return err
			}
			if err := tx.Model(&product).Updates(map[string]any{
				"is_published": false,
			}).Error; err != nil {
				return err
			}
			return saveEditableProductCatalogDraft(tx, &product, draft)
		}); err != nil {
			if err.Error() == "Product with this SKU already exists" || err.Error() == "Variant SKU already exists" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Preload("Related").First(&product, product.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load created product"})
			return
		}
		view, err := buildProductContract(db, nil, product, true, true, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusCreated, view)
	}
}

func UpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req apicontract.ProductUpsertInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			draft := catalogDraftFromUpsertInput(req)
			if err := validateProductCatalogDraft(tx, draft, product.ID); err != nil {
				return err
			}
			return saveEditableProductCatalogDraft(tx, &product, draft)
		}); err != nil {
			if err.Error() == "Product with this SKU already exists" || err.Error() == "Variant SKU already exists" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		respondAdminProduct(c, db, nil, product.ID)
	}
}

func UpdateProductRelated(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	_ = mediaService
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var req UpdateRelatedRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		for _, relatedID := range req.RelatedIDs {
			if relatedID == product.ID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product cannot be related to itself"})
				return
			}
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			_, err := ensureProductCatalogDraft(tx, &product)
			if err != nil {
				return err
			}
			draft, _, err := loadNormalizedProductDraft(tx, product)
			if err != nil {
				return err
			}
			draft.RelatedIDs = append([]uint(nil), req.RelatedIDs...)
			return saveEditableProductCatalogDraft(tx, &product, draft)
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update related products"})
			return
		}

		respondAdminProduct(c, db, mediaService, product.ID)
	}
}

func PublishProduct(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cleanupIDs := []string{}
		publishedProductID := uint(0)
		err := db.Transaction(func(tx *gorm.DB) error {
			var product models.Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Related").First(&product, id).Error; err != nil {
				return err
			}
			publishedProductID = product.ID

			if productHasDraft(product) {
				var draftHeader models.ProductDraft
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", product.ID).First(&draftHeader).Error; err != nil {
					return err
				}
				if draftHeader.Version <= 0 {
					return errors.New("product draft version is invalid")
				}
			}

			draft, hasDraft, err := loadNormalizedProductDraft(tx, product)
			if err != nil {
				return err
			}
			if err := validateProductCatalogDraft(tx, draft, product.ID); err != nil {
				return err
			}
			normalized, err := deriveCatalogMerchandising(draft)
			if err != nil {
				return err
			}

			oldLiveRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductImage)
			if err != nil {
				return err
			}
			draftRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductDraftImage)
			if err != nil {
				return err
			}

			if err := publishNormalizedProductDraft(tx, &product, normalized); err != nil {
				return err
			}

			if hasDraft {
				if err := replaceProductMediaReferences(tx, product.ID, media.RoleProductImage, draftRefs); err != nil {
					return err
				}
				if hasMediaReferenceTable(tx) {
					if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
						media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
						Delete(&models.MediaReference{}).Error; err != nil {
						return err
					}
				}
			}

			activeLiveRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductImage)
			if err != nil {
				return err
			}
			cleanupIDs = append(cleanupIDs, diffMediaIDs(oldLiveRefs, activeLiveRefs)...)
			if hasDraft {
				cleanupIDs = append(cleanupIDs, diffMediaIDs(draftRefs, activeLiveRefs)...)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
				return
			}
			if err.Error() == "Product with this SKU already exists" || err.Error() == "Variant SKU already exists" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cleanupMediaIDs(mediaService, cleanupIDs)
		respondAdminProduct(c, db, mediaService, publishedProductID)
	}
}

func UnpublishProduct(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			hadDraft, err := ensureProductCatalogDraft(tx, &product)
			if err != nil {
				return err
			}

			if !hadDraft {
				if err := copyProductMediaRole(
					tx,
					product.ID,
					media.RoleProductImage,
					media.RoleProductDraftImage,
				); err != nil {
					return err
				}
			}

			return tx.Model(&product).Update("is_published", false).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish product"})
			return
		}

		respondAdminProduct(c, db, mediaService, product.ID)
	}
}

func DiscardProductDraft(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if !productHasDraft(product) {
			respondAdminProduct(c, db, mediaService, product.ID)
			return
		}

		cleanupIDs := []string{}
		if err := db.Transaction(func(tx *gorm.DB) error {
			draftRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductDraftImage)
			if err != nil {
				return err
			}

			if product.IsPublished {
				if err := discardNormalizedProductDraft(tx, &product); err != nil {
					return err
				}
				if hasMediaReferenceTable(tx) {
					if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
						media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
						Delete(&models.MediaReference{}).Error; err != nil {
						return err
					}
				}
				cleanupIDs = append(cleanupIDs, mediaIDsFromRefs(draftRefs)...)
				return nil
			}

			if err := discardNormalizedProductDraft(tx, &product); err != nil {
				return err
			}
			if err := copyProductMediaRole(tx, product.ID, media.RoleProductImage, media.RoleProductDraftImage); err != nil {
				return err
			}
			currentDraftRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductDraftImage)
			if err != nil {
				return err
			}
			cleanupIDs = append(cleanupIDs, diffMediaIDs(draftRefs, currentDraftRefs)...)
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to discard draft"})
			return
		}

		cleanupMediaIDs(mediaService, cleanupIDs)
		respondAdminProduct(c, db, mediaService, product.ID)
	}
}

func DeleteProduct(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var refs []models.MediaReference
		if mediaService != nil && hasMediaReferenceTable(db) {
			if err := db.Where("owner_type = ? AND owner_id = ? AND role IN ?",
				media.OwnerTypeProduct, product.ID, []string{media.RoleProductImage, media.RoleProductDraftImage}).
				Find(&refs).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product media"})
				return
			}
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&product).Error; err != nil {
				return err
			}
			if mediaService != nil && hasMediaReferenceTable(tx) {
				if err := tx.Where("owner_type = ? AND owner_id = ? AND role IN ?",
					media.OwnerTypeProduct, product.ID, []string{media.RoleProductImage, media.RoleProductDraftImage}).
					Delete(&models.MediaReference{}).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
			return
		}

		if mediaService != nil {
			cleanupMediaIDs(mediaService, mediaIDsFromRefs(refs))
		}

		c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
	}
}

func ptrTimeNow() *time.Time {
	now := time.Now()
	return &now
}
