package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func validateProductDraft(draft productDraftData) error {
	if strings.TrimSpace(draft.SKU) == "" {
		return errors.New("Product SKU is required")
	}
	if strings.TrimSpace(draft.Name) == "" {
		return errors.New("Product name is required")
	}
	if draft.Price <= 0 {
		return errors.New("Product price must be greater than 0")
	}
	return nil
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
		draft, hasDraft, err := editableProductDraftData(product)
		if err != nil {
			return nil, err
		}
		if hasDraft {
			product = applyDraftDataToProduct(product, draft)
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

func materializeAdminProduct(db *gorm.DB, mediaService *media.Service, source models.Product, includeRelated bool) (models.Product, error) {
	draft, hasDraft, err := editableProductDraftData(source)
	if err != nil {
		return models.Product{}, err
	}

	view := source
	if hasDraft {
		view = applyDraftDataToProduct(view, draft)
	}
	if includeRelated {
		if hasDraft {
			related, relatedErr := relatedProductsByIDs(db, draft.RelatedIDs)
			if relatedErr != nil {
				return models.Product{}, relatedErr
			}
			view.Related = related
		} else {
			view.Related = append([]models.Product(nil), source.Related...)
		}
	}

	imageRole := media.RoleProductImage
	fallbackImages := view.Images
	if hasDraft {
		imageRole = media.RoleProductDraftImage
		fallbackImages = draft.Images
	}
	applyProductMediaWithRole(&view, mediaService, imageRole, fallbackImages)

	if includeRelated {
		for i := range view.Related {
			role := media.RoleProductImage
			fallbackImages := view.Related[i].Images
			if productHasDraft(view.Related[i]) {
				role = media.RoleProductDraftImage
			}
			applyProductMediaWithRole(&view.Related[i], mediaService, role, fallbackImages)
		}
	}

	view.IsPublished = source.IsPublished
	view.DraftData = source.DraftData
	view.DraftUpdatedAt = source.DraftUpdatedAt
	return view, nil
}

func ListAdminProducts(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.Product{})

		if searchTerm := strings.TrimSpace(c.Query("q")); searchTerm != "" {
			query = query.Where("name ILIKE ? OR sku ILIKE ?", "%"+searchTerm+"%", "%"+searchTerm+"%")
		}
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

		sortField := c.DefaultQuery("sort", "created_at")
		sortOrder := c.DefaultQuery("order", "desc")
		validSortFields := map[string]bool{
			"price": true, "name": true, "created_at": true,
		}
		if !validSortFields[sortField] {
			sortField = "created_at"
		}
		if sortOrder != "asc" && sortOrder != "desc" {
			sortOrder = "desc"
		}
		query = query.Order(sortField + " " + sortOrder)

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
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

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}

		var products []models.Product
		if err := query.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}

		contractProducts := make([]apicontract.Product, 0, len(products))
		for _, product := range products {
			view, viewErr := materializeAdminProduct(db, mediaService, product, false)
			if viewErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
				return
			}
			contractProducts = append(contractProducts, toContractProduct(view))
		}

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
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

func GetAdminProductByID(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
	}
}

func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpdateProductRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		draft := normalizeProductDraftData(productDraftData{})
		if req.SKU != nil {
			draft.SKU = strings.TrimSpace(*req.SKU)
		}
		if req.Name != nil {
			draft.Name = strings.TrimSpace(*req.Name)
		}
		if req.Description != nil {
			draft.Description = strings.TrimSpace(*req.Description)
		}
		if req.Price != nil {
			draft.Price = *req.Price
		}
		if req.Stock != nil {
			draft.Stock = *req.Stock
		}
		if req.Images != nil {
			draft.Images = append([]string(nil), (*req.Images)...)
		}
		draft = normalizeProductDraftData(draft)

		if err := validateProductDraft(draft); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := ensureUniqueProductSKU(db, draft.SKU, 0); err != nil {
			if err.Error() == "Product with this SKU already exists" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check SKU uniqueness"})
			return
		}

		payload, err := encodeProductDraftData(draft)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product draft"})
			return
		}

		product := models.Product{
			SKU:            draft.SKU,
			Name:           draft.Name,
			Description:    draft.Description,
			Price:          models.MoneyFromFloat(draft.Price),
			Stock:          draft.Stock,
			Images:         append([]string(nil), draft.Images...),
			IsPublished:    false,
			DraftData:      payload,
			DraftUpdatedAt: ptrTimeNow(),
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Select("*").Create(&product).Error; err != nil {
				return err
			}
			return tx.Model(&product).Updates(map[string]any{
				"is_published":     false,
				"draft_data":       payload,
				"draft_updated_at": product.DraftUpdatedAt,
			}).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
			return
		}
		if err := db.Preload("Related").First(&product, product.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load created product"})
			return
		}

		view, viewErr := materializeAdminProduct(db, nil, product, true)
		if viewErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load created product"})
			return
		}
		c.JSON(http.StatusCreated, toContractProduct(view))
	}
}

func UpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req UpdateProductRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.SKU == nil && req.Name == nil && req.Description == nil && req.Price == nil && req.Stock == nil && req.Images == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
			return
		}

		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			draft, _, err := ensureProductDraft(tx, &product)
			if err != nil {
				return err
			}

			if req.SKU != nil {
				draft.SKU = strings.TrimSpace(*req.SKU)
			}
			if req.Name != nil {
				draft.Name = strings.TrimSpace(*req.Name)
			}
			if req.Description != nil {
				draft.Description = strings.TrimSpace(*req.Description)
			}
			if req.Price != nil {
				draft.Price = *req.Price
			}
			if req.Stock != nil {
				draft.Stock = *req.Stock
			}
			if req.Images != nil {
				draft.Images = append([]string(nil), (*req.Images)...)
			}
			draft = normalizeProductDraftData(draft)

			if err := validateProductDraft(draft); err != nil {
				return err
			}
			if err := ensureUniqueProductSKU(tx, draft.SKU, product.ID); err != nil {
				return err
			}
			return upsertProductDraft(tx, &product, draft)
		}); err != nil {
			switch err.Error() {
			case "Product SKU is required", "Product name is required", "Product price must be greater than 0":
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			case "Product with this SKU already exists":
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
				return
			}
		}

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}
		view, err := materializeAdminProduct(db, nil, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
			draft, _, err := ensureProductDraft(tx, &product)
			if err != nil {
				return err
			}
			draft.RelatedIDs = append([]uint(nil), req.RelatedIDs...)
			draft = normalizeProductDraftData(draft)
			return upsertProductDraft(tx, &product, draft)
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update related products"})
			return
		}

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}
		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
	}
}

func PublishProduct(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		draft, hasDraft, err := editableProductDraftData(product)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product draft"})
			return
		}
		if !hasDraft {
			draft = productDraftFromPublished(product)
		}
		if err := validateProductDraft(draft); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := ensureUniqueProductSKU(db, draft.SKU, product.ID); err != nil {
			if err.Error() == "Product with this SKU already exists" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check SKU uniqueness"})
			return
		}

		cleanupIDs := []string{}
		if err := db.Transaction(func(tx *gorm.DB) error {
			var related []models.Product
			if len(draft.RelatedIDs) > 0 {
				if err := tx.Where("id IN ?", draft.RelatedIDs).Find(&related).Error; err != nil {
					return err
				}
			}

			oldLiveRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductImage)
			if err != nil {
				return err
			}
			draftRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductDraftImage)
			if err != nil {
				return err
			}

			updates := map[string]any{
				"sku":              draft.SKU,
				"name":             draft.Name,
				"description":      draft.Description,
				"price":            models.MoneyFromFloat(draft.Price),
				"stock":            draft.Stock,
				"images":           draft.Images,
				"is_published":     true,
				"draft_data":       "",
				"draft_updated_at": nil,
			}
			if err := tx.Model(&product).Updates(updates).Error; err != nil {
				return err
			}

			if err := tx.Model(&product).Association("Related").Replace(related); err != nil {
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
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish product"})
			return
		}

		cleanupMediaIDs(mediaService, cleanupIDs)

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load published product"})
			return
		}
		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
			_, hadDraft, err := ensureProductDraft(tx, &product)
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

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load unpublished product"})
			return
		}

		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
			view, err := materializeAdminProduct(db, mediaService, product, true)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
				return
			}
			c.JSON(http.StatusOK, toContractProduct(view))
			return
		}

		cleanupIDs := []string{}
		if err := db.Transaction(func(tx *gorm.DB) error {
			draftRefs, err := loadProductMediaReferences(tx, product.ID, media.RoleProductDraftImage)
			if err != nil {
				return err
			}

			if product.IsPublished {
				if err := tx.Model(&product).Updates(map[string]any{
					"draft_data":       "",
					"draft_updated_at": nil,
				}).Error; err != nil {
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

			baseDraft := productDraftFromPublished(product)
			if err := upsertProductDraft(tx, &product, baseDraft); err != nil {
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

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product"})
			return
		}
		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
