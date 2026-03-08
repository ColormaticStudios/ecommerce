package handlers

import (
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	categoriesservice "ecommerce/internal/services/categories"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxBrandNameLength        = 120
	maxBrandDescriptionLength = 500
	maxAttributeKeyLength     = 120
	maxSEOTitleLength         = 70
	maxSEODescriptionLength   = 160
	maxSEOCanonicalPathLength = 255
)

var supportedAttributeTypes = map[string]struct{}{
	"text":    {},
	"number":  {},
	"boolean": {},
	"enum":    {},
}

func normalizeCatalogSlug(raw *string, fallback string) string {
	if raw != nil && strings.TrimSpace(*raw) != "" {
		return categoriesservice.NormalizeSlug(*raw)
	}
	return categoriesservice.NormalizeSlug(fallback)
}

func normalizeCanonicalPath(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return &trimmed
}

func validateCatalogSEO(tx *gorm.DB, seo productSEODraftData, productID uint) error {
	normalized := normalizeProductSEO(seo)
	normalized.CanonicalPath = normalizeCanonicalPath(normalized.CanonicalPath)

	if normalized.Title != nil && len(strings.TrimSpace(*normalized.Title)) > maxSEOTitleLength {
		return errors.New("SEO title must be 70 characters or fewer")
	}
	if normalized.Description != nil && len(strings.TrimSpace(*normalized.Description)) > maxSEODescriptionLength {
		return errors.New("SEO description must be 160 characters or fewer")
	}
	if normalized.CanonicalPath != nil {
		if len(*normalized.CanonicalPath) > maxSEOCanonicalPathLength {
			return errors.New("SEO canonical path must be 255 characters or fewer")
		}
		if !strings.HasPrefix(*normalized.CanonicalPath, "/") {
			return errors.New("SEO canonical path must start with /")
		}
		var existing models.SEOMetadata
		err := tx.
			Where("canonical_path = ?", *normalized.CanonicalPath).
			Where("NOT (entity_type = ? AND entity_id = ?)", seoEntityTypeProduct, productID).
			First(&existing).Error
		if err == nil {
			return errors.New("SEO canonical path already exists")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	return nil
}

func brandToContract(brand models.Brand) apicontract.Brand {
	return apicontract.Brand{
		Description: brand.Description,
		Id:          int(brand.ID),
		IsActive:    brand.IsActive,
		LogoMediaId: brand.LogoMediaID,
		Name:        brand.Name,
		Slug:        brand.Slug,
	}
}

func productAttributeToContract(attribute models.ProductAttribute) apicontract.ProductAttributeDefinition {
	return apicontract.ProductAttributeDefinition{
		Filterable: attribute.Filterable,
		Id:         int(attribute.ID),
		Key:        attribute.Key,
		Slug:       attribute.Slug,
		Sortable:   attribute.Sortable,
		Type:       apicontract.ProductAttributeDefinitionType(attribute.Type),
	}
}

func listBrandsQuery(db *gorm.DB, activeOnly bool, searchTerm string) *gorm.DB {
	query := db.Model(&models.Brand{})
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	if searchTerm != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(searchTerm)) + "%"
		query = query.Where(
			`LOWER(name) LIKE ? OR
			 LOWER(slug) LIKE ? OR
			 LOWER(COALESCE(description, '')) LIKE ?`,
			like, like, like,
		)
	}
	return query.Order("name asc, id asc")
}

func ListBrands(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var brands []models.Brand
		if err := listBrandsQuery(db, true, "").Find(&brands).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch brands"})
			return
		}

		response := apicontract.BrandListResponse{Data: make([]apicontract.Brand, 0, len(brands))}
		for _, brand := range brands {
			response.Data = append(response.Data, brandToContract(brand))
		}
		c.JSON(http.StatusOK, response)
	}
}

func ListAdminBrands(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		searchTerm := strings.TrimSpace(c.Query("q"))
		var brands []models.Brand
		if err := listBrandsQuery(db, false, searchTerm).Find(&brands).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch brands"})
			return
		}

		response := apicontract.BrandListResponse{Data: make([]apicontract.Brand, 0, len(brands))}
		for _, brand := range brands {
			response.Data = append(response.Data, brandToContract(brand))
		}
		c.JSON(http.StatusOK, response)
	}
}

func validateBrandInput(tx *gorm.DB, input apicontract.BrandInput, excludeID uint) (models.Brand, error) {
	name := strings.TrimSpace(input.Name)
	if !categoriesservice.IsValidName(name) || len(name) > maxBrandNameLength {
		return models.Brand{}, errors.New("Brand name must be between 2 and 120 characters")
	}

	slug := normalizeCatalogSlug(input.Slug, name)
	if slug == "" {
		return models.Brand{}, errors.New("Brand slug is required")
	}

	description := trimOptionalString(input.Description)
	if description != nil && len(*description) > maxBrandDescriptionLength {
		return models.Brand{}, errors.New("Brand description must be 500 characters or fewer")
	}

	logoMediaID := trimOptionalString(input.LogoMediaId)
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	var count int64
	if err := tx.Model(&models.Brand{}).
		Where("slug = ? AND id <> ?", slug, excludeID).
		Count(&count).Error; err != nil {
		return models.Brand{}, err
	}
	if count > 0 {
		return models.Brand{}, errors.New("Brand slug already exists")
	}

	return models.Brand{
		Name:        name,
		Slug:        slug,
		Description: description,
		LogoMediaID: logoMediaID,
		IsActive:    isActive,
	}, nil
}

func CreateAdminBrand(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.BrandInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		brand, err := validateBrandInput(db, req, 0)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Create(&brand).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create brand"})
			return
		}
		c.JSON(http.StatusCreated, brandToContract(brand))
	}
}

func UpdateAdminBrand(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.BrandInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var brand models.Brand
		if err := db.First(&brand, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Brand not found"})
			return
		}

		normalized, err := validateBrandInput(db, req, brand.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		brand.Name = normalized.Name
		brand.Slug = normalized.Slug
		brand.Description = normalized.Description
		brand.LogoMediaID = normalized.LogoMediaID
		brand.IsActive = normalized.IsActive

		if err := db.Save(&brand).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update brand"})
			return
		}
		c.JSON(http.StatusOK, brandToContract(brand))
	}
}

func DeleteAdminBrand(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var brand models.Brand
		if err := db.First(&brand, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Brand not found"})
			return
		}

		var liveCount int64
		if err := db.Model(&models.Product{}).Where("brand_id = ?", brand.ID).Count(&liveCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete brand"})
			return
		}
		if liveCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Brand is still assigned to products"})
			return
		}

		var draftCount int64
		if err := db.Model(&models.ProductDraft{}).Where("brand_id = ?", brand.ID).Count(&draftCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete brand"})
			return
		}
		if draftCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Brand is still assigned to product drafts"})
			return
		}

		if err := db.Delete(&brand).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete brand"})
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Brand deleted"})
	}
}

func listProductAttributeDefinitionsQuery(db *gorm.DB, filterableOnly bool) *gorm.DB {
	query := db.Model(&models.ProductAttribute{})
	if filterableOnly {
		query = query.Where("filterable = ?", true)
	}
	return query.Order("key asc, id asc")
}

func ListProductAttributes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var attributes []models.ProductAttribute
		if err := listProductAttributeDefinitionsQuery(db, true).Find(&attributes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product attributes"})
			return
		}

		response := apicontract.ProductAttributeDefinitionListResponse{
			Data: make([]apicontract.ProductAttributeDefinition, 0, len(attributes)),
		}
		for _, attribute := range attributes {
			response.Data = append(response.Data, productAttributeToContract(attribute))
		}
		c.JSON(http.StatusOK, response)
	}
}

func ListAdminProductAttributes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var attributes []models.ProductAttribute
		if err := listProductAttributeDefinitionsQuery(db, false).Find(&attributes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product attributes"})
			return
		}

		response := apicontract.ProductAttributeDefinitionListResponse{
			Data: make([]apicontract.ProductAttributeDefinition, 0, len(attributes)),
		}
		for _, attribute := range attributes {
			response.Data = append(response.Data, productAttributeToContract(attribute))
		}
		c.JSON(http.StatusOK, response)
	}
}

func validateProductAttributeDefinitionInput(
	tx *gorm.DB,
	input apicontract.ProductAttributeDefinitionInput,
	excludeID uint,
) (models.ProductAttribute, error) {
	key := strings.TrimSpace(input.Key)
	if !categoriesservice.IsValidName(key) || len(key) > maxAttributeKeyLength {
		return models.ProductAttribute{}, errors.New("Product attribute key must be between 2 and 120 characters")
	}

	slug := normalizeCatalogSlug(input.Slug, key)
	if slug == "" {
		return models.ProductAttribute{}, errors.New("Product attribute slug is required")
	}

	attrType := strings.TrimSpace(string(input.Type))
	if _, ok := supportedAttributeTypes[attrType]; !ok {
		return models.ProductAttribute{}, errors.New("Product attribute type is invalid")
	}

	filterable := input.Filterable != nil && *input.Filterable
	sortable := input.Sortable != nil && *input.Sortable

	var slugCount int64
	if err := tx.Model(&models.ProductAttribute{}).
		Where("slug = ? AND id <> ?", slug, excludeID).
		Count(&slugCount).Error; err != nil {
		return models.ProductAttribute{}, err
	}
	if slugCount > 0 {
		return models.ProductAttribute{}, errors.New("Product attribute slug already exists")
	}

	var keyCount int64
	if err := tx.Model(&models.ProductAttribute{}).
		Where("key = ? AND id <> ?", key, excludeID).
		Count(&keyCount).Error; err != nil {
		return models.ProductAttribute{}, err
	}
	if keyCount > 0 {
		return models.ProductAttribute{}, errors.New("Product attribute key already exists")
	}

	return models.ProductAttribute{
		Key:        key,
		Slug:       slug,
		Type:       attrType,
		Filterable: filterable,
		Sortable:   sortable,
	}, nil
}

func CreateAdminProductAttribute(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.ProductAttributeDefinitionInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		attribute, err := validateProductAttributeDefinitionInput(db, req, 0)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Create(&attribute).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product attribute"})
			return
		}
		c.JSON(http.StatusCreated, productAttributeToContract(attribute))
	}
}

func UpdateAdminProductAttribute(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.ProductAttributeDefinitionInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var attribute models.ProductAttribute
		if err := db.First(&attribute, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product attribute not found"})
			return
		}

		normalized, err := validateProductAttributeDefinitionInput(db, req, attribute.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		attribute.Key = normalized.Key
		attribute.Slug = normalized.Slug
		attribute.Type = normalized.Type
		attribute.Filterable = normalized.Filterable
		attribute.Sortable = normalized.Sortable

		if err := db.Save(&attribute).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product attribute"})
			return
		}
		c.JSON(http.StatusOK, productAttributeToContract(attribute))
	}
}

func DeleteAdminProductAttribute(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var attribute models.ProductAttribute
		if err := db.First(&attribute, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product attribute not found"})
			return
		}

		var liveCount int64
		if err := db.Model(&models.ProductAttributeValue{}).
			Where("product_attribute_id = ?", attribute.ID).
			Count(&liveCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product attribute"})
			return
		}
		if liveCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Product attribute is still assigned to products"})
			return
		}

		var draftCount int64
		if err := db.Model(&models.ProductAttributeValueDraft{}).
			Where("product_attribute_id = ?", attribute.ID).
			Count(&draftCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product attribute"})
			return
		}
		if draftCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Product attribute is still assigned to product drafts"})
			return
		}

		if err := db.Delete(&attribute).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product attribute"})
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Product attribute deleted"})
	}
}
