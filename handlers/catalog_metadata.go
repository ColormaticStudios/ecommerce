package handlers

import (
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	categoriesservice "ecommerce/internal/services/categories"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxBrandNameLength           = 120
	maxBrandDescriptionLength    = 500
	maxCategoryDescriptionLength = 500
	maxCategoryDepth             = 5
	maxAttributeKeyLength        = 120
	maxSEOTitleLength            = 70
	maxSEODescriptionLength      = 160
	maxSEOCanonicalPathLength    = 255
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
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	return nil
}

func brandToContract(brand models.Brand, mediaService *media.Service) apicontract.Brand {
	contract := apicontract.Brand{
		Description: brand.Description,
		Id:          int(brand.ID),
		IsActive:    brand.IsActive,
		Name:        brand.Name,
		Slug:        brand.Slug,
	}
	if mediaService != nil {
		if logoURL, err := mediaService.BrandLogoURL(brand.ID); err == nil {
			contract.LogoUrl = &logoURL
		}
	}
	return contract
}

func productAttributeToContract(attribute models.ProductAttribute) apicontract.ProductAttributeDefinition {
	enumValues := []string(attribute.EnumValues)
	if enumValues == nil {
		enumValues = []string{}
	}
	return apicontract.ProductAttributeDefinition{
		Filterable: attribute.Filterable,
		EnumValues: enumValues,
		Id:         int(attribute.ID),
		Key:        attribute.Key,
		Slug:       attribute.Slug,
		Sortable:   attribute.Sortable,
		Type:       apicontract.ProductAttributeDefinitionType(attribute.Type),
	}
}

func categoryToContract(category models.Category) apicontract.Category {
	return apicontract.Category{
		Depth:       category.Depth,
		Description: category.Description,
		Id:          int(category.ID),
		IsActive:    category.IsActive,
		Name:        category.Name,
		ParentId:    uintPtrToIntPtr(category.ParentID),
		Path:        category.Path,
		Slug:        category.Slug,
		SortOrder:   category.SortOrder,
	}
}

func uintPtrToIntPtr(value *uint) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
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

func listCategoriesQuery(db *gorm.DB, includeInactive bool, searchTerm string) *gorm.DB {
	query := db.Model(&models.Category{})
	if !includeInactive {
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
	return query.Order("sort_order asc, id asc")
}

func ListAdminCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		searchTerm := strings.TrimSpace(c.Query("q"))
		includeInactive := strings.EqualFold(c.Query("include_inactive"), "true")
		var categories []models.Category
		if err := listCategoriesQuery(db, includeInactive, searchTerm).Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
			return
		}

		response := apicontract.CategoryListResponse{Data: make([]apicontract.Category, 0, len(categories))}
		for _, category := range categories {
			response.Data = append(response.Data, categoryToContract(category))
		}
		c.JSON(http.StatusOK, response)
	}
}

func ListCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var categories []models.Category
		if err := listCategoriesQuery(db, false, "").Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
			return
		}

		response := apicontract.CategoryListResponse{Data: make([]apicontract.Category, 0, len(categories))}
		for _, category := range categories {
			response.Data = append(response.Data, categoryToContract(category))
		}
		c.JSON(http.StatusOK, response)
	}
}

func validateCategoryInput(tx *gorm.DB, input apicontract.CategoryInput, excludeID uint) (models.Category, error) {
	name := strings.TrimSpace(input.Name)
	if !categoriesservice.IsValidName(name) || len(name) > maxBrandNameLength {
		return models.Category{}, errors.New("Category name must be between 2 and 120 characters")
	}

	slug := normalizeCatalogSlug(input.Slug, name)
	if slug == "" {
		return models.Category{}, errors.New("Category slug is required")
	}

	description := trimOptionalString(input.Description)
	if description != nil && len(*description) > maxCategoryDescriptionLength {
		return models.Category{}, errors.New("Category description must be 500 characters or fewer")
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}

	var slugCount int64
	if err := tx.Model(&models.Category{}).
		Where("slug = ? AND id <> ?", slug, excludeID).
		Count(&slugCount).Error; err != nil {
		return models.Category{}, err
	}
	if slugCount > 0 {
		return models.Category{}, errors.New("Category slug already exists")
	}

	var parentID *uint
	path := "/" + slug
	depth := 0
	if input.ParentId != nil {
		if *input.ParentId <= 0 {
			return models.Category{}, errors.New("Parent category is invalid")
		}
		candidateParentID := uint(*input.ParentId)
		if candidateParentID == excludeID && excludeID != 0 {
			return models.Category{}, errors.New("Category cannot be its own parent")
		}
		parent, err := loadCategoryParent(tx, candidateParentID)
		if err != nil {
			return models.Category{}, err
		}
		if excludeID != 0 && categoryParentCreatesCycle(tx, parent, excludeID) {
			return models.Category{}, errors.New("Category parent would create a cycle")
		}
		depth = parent.Depth + 1
		if depth > maxCategoryDepth {
			return models.Category{}, errors.New("Category depth exceeds the maximum")
		}
		parentID = &candidateParentID
		path = strings.TrimRight(parent.Path, "/") + "/" + slug
	}

	return models.Category{
		Name:        name,
		Slug:        slug,
		Description: description,
		IsActive:    isActive,
		SortOrder:   sortOrder,
		ParentID:    parentID,
		Path:        path,
		Depth:       depth,
	}, nil
}

func loadCategoryParent(tx *gorm.DB, parentID uint) (models.Category, error) {
	var parent models.Category
	if err := tx.First(&parent, parentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Category{}, errors.New("Parent category not found")
		}
		return models.Category{}, err
	}
	return parent, nil
}

func categoryParentCreatesCycle(tx *gorm.DB, parent models.Category, categoryID uint) bool {
	for {
		if parent.ID == categoryID {
			return true
		}
		if parent.ParentID == nil {
			return false
		}
		if err := tx.First(&parent, *parent.ParentID).Error; err != nil {
			return true
		}
	}
}

func CreateAdminCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CategoryInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		category, err := validateCategoryInput(db, req, 0)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Select("*").Create(&category).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
			return
		}
		c.JSON(http.StatusCreated, categoryToContract(category))
	}
}

func UpdateAdminCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CategoryInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var category models.Category
		if err := db.First(&category, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}

		normalized, err := validateCategoryInput(db, req, category.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if category.IsActive && !normalized.IsActive {
			referenced, err := categoryHasPublishedProductReferences(db, category.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
				return
			}
			if referenced {
				c.JSON(http.StatusConflict, gin.H{"error": "Category is assigned to published products"})
				return
			}
		}

		category.Name = normalized.Name
		category.Slug = normalized.Slug
		category.Description = normalized.Description
		category.IsActive = normalized.IsActive
		category.SortOrder = normalized.SortOrder
		category.ParentID = normalized.ParentID
		category.Path = normalized.Path
		category.Depth = normalized.Depth

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&category).Error; err != nil {
				return err
			}
			return rebuildCategoryDescendantPaths(tx, category)
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
			return
		}
		c.JSON(http.StatusOK, categoryToContract(category))
	}
}

func rebuildCategoryDescendantPaths(tx *gorm.DB, parent models.Category) error {
	var children []models.Category
	if err := tx.Where("parent_id = ?", parent.ID).Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		child.Depth = parent.Depth + 1
		child.Path = strings.TrimRight(parent.Path, "/") + "/" + child.Slug
		if child.Depth > maxCategoryDepth {
			return errors.New("Category depth exceeds the maximum")
		}
		if err := tx.Save(&child).Error; err != nil {
			return err
		}
		if err := rebuildCategoryDescendantPaths(tx, child); err != nil {
			return err
		}
	}
	return nil
}

func DeleteAdminCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var category models.Category
		if err := db.First(&category, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}

		var childCount int64
		if err := db.Model(&models.Category{}).Where("parent_id = ?", category.ID).Count(&childCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
			return
		}
		if childCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Category has child categories"})
			return
		}

		referenced, err := categoryHasPublishedProductReferences(db, category.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
			return
		}
		if referenced {
			c.JSON(http.StatusConflict, gin.H{"error": "Category is assigned to published products"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("category_id = ?", category.ID).Delete(&models.ProductCategory{}).Error; err != nil {
				return err
			}
			if err := tx.Where("category_id = ?", category.ID).Delete(&models.ProductCategoryDraft{}).Error; err != nil {
				return err
			}
			return tx.Delete(&category).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Category deleted"})
	}
}

func categoryHasPublishedProductReferences(db *gorm.DB, categoryID uint) (bool, error) {
	var count int64
	err := db.Table("product_categories pc").
		Joins("JOIN products p ON p.id = pc.product_id").
		Where("pc.category_id = ? AND p.is_published = ? AND p.deleted_at IS NULL", categoryID, true).
		Count(&count).Error
	return count > 0, err
}

func ListBrands(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var brands []models.Brand
		if err := listBrandsQuery(db, true, "").Find(&brands).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch brands"})
			return
		}

		response := apicontract.BrandListResponse{Data: make([]apicontract.Brand, 0, len(brands))}
		for _, brand := range brands {
			response.Data = append(response.Data, brandToContract(brand, mediaService))
		}
		c.JSON(http.StatusOK, response)
	}
}

func ListAdminBrands(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		searchTerm := strings.TrimSpace(c.Query("q"))
		var brands []models.Brand
		if err := listBrandsQuery(db, false, searchTerm).Find(&brands).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch brands"})
			return
		}

		response := apicontract.BrandListResponse{Data: make([]apicontract.Brand, 0, len(brands))}
		for _, brand := range brands {
			response.Data = append(response.Data, brandToContract(brand, mediaService))
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
		IsActive:    isActive,
	}, nil
}

func brandLogoMediaID(mediaService *media.Service, input apicontract.BrandInput) (string, error) {
	if input.Logo == nil {
		return "", nil
	}
	if mediaService == nil {
		return "", errors.New("media service is unavailable")
	}
	mediaID := strings.TrimSpace(input.Logo.MediaId)
	if mediaID == "" {
		return "", errors.New("logo media ID is required")
	}
	object, err := mediaService.WaitUntilReady(mediaID, mediaReadyTimeout)
	if err != nil {
		return "", mediaLookupStatusError(err, "Logo media not found", "Failed to load logo media", "Logo media processing failed", "Logo media is still processing")
	}
	if !strings.HasPrefix(object.MimeType, "image/") {
		return "", errors.New("logo media must be an image")
	}
	return mediaID, nil
}

func replaceBrandLogoReference(tx *gorm.DB, brandID uint, mediaID string) ([]string, error) {
	var existing []models.MediaReference
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeBrand, brandID, media.RoleBrandLogo).Find(&existing).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeBrand, brandID, media.RoleBrandLogo).Delete(&models.MediaReference{}).Error; err != nil {
		return nil, err
	}
	if mediaID != "" {
		if err := tx.Create(&models.MediaReference{MediaID: mediaID, OwnerType: media.OwnerTypeBrand, OwnerID: brandID, Role: media.RoleBrandLogo}).Error; err != nil {
			return nil, err
		}
	}
	removed := make([]string, 0, len(existing))
	for _, ref := range existing {
		if ref.MediaID != mediaID {
			removed = append(removed, ref.MediaID)
		}
	}
	return removed, nil
}

func CreateAdminBrand(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
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
		logoMediaID, err := brandLogoMediaID(mediaService, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&brand).Error; err != nil {
				return err
			}
			_, err := replaceBrandLogoReference(tx, brand.ID, logoMediaID)
			return err
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create brand"})
			return
		}
		c.JSON(http.StatusCreated, brandToContract(brand, mediaService))
	}
}

func UpdateAdminBrand(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
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
		logoMediaID, err := brandLogoMediaID(mediaService, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		brand.Name = normalized.Name
		brand.Slug = normalized.Slug
		brand.Description = normalized.Description
		brand.IsActive = normalized.IsActive

		var cleanupIDs []string
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&brand).Error; err != nil {
				return err
			}
			var err error
			cleanupIDs, err = replaceBrandLogoReference(tx, brand.ID, logoMediaID)
			return err
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update brand"})
			return
		}
		cleanupMediaIDs(mediaService, cleanupIDs)
		c.JSON(http.StatusOK, brandToContract(brand, mediaService))
	}
}

func DeleteAdminBrand(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
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

		var cleanupIDs []string
		if err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			cleanupIDs, err = replaceBrandLogoReference(tx, brand.ID, "")
			if err != nil {
				return err
			}
			return tx.Delete(&brand).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete brand"})
			return
		}
		cleanupMediaIDs(mediaService, cleanupIDs)
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
	if sortable && attrType != "number" {
		return models.ProductAttribute{}, errors.New("Only number attributes can be sortable")
	}

	enumValues, err := normalizeProductAttributeEnumValues(attrType, input.EnumValues)
	if err != nil {
		return models.ProductAttribute{}, err
	}

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
		EnumValues: enumValues,
	}, nil
}

func normalizeProductAttributeEnumValues(attrType string, values *[]string) (models.StringArray, error) {
	if attrType != "enum" {
		return nil, nil
	}
	if values == nil || len(*values) == 0 {
		return nil, errors.New("Enum attributes require at least one allowed value")
	}

	result := make(models.StringArray, 0, len(*values))
	seen := map[string]struct{}{}
	for _, raw := range *values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, errors.New("Enum attribute values cannot be blank")
		}
		if len(value) > maxAttributeKeyLength {
			return nil, errors.New("Enum attribute values must be 120 characters or less")
		}
		lookup := strings.ToLower(value)
		if _, exists := seen[lookup]; exists {
			return nil, errors.New("Enum attribute values must be unique")
		}
		seen[lookup] = struct{}{}
		result = append(result, value)
	}
	return result, nil
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
		attribute.EnumValues = normalized.EnumValues

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
