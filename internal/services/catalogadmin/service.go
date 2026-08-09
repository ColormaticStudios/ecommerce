package catalogadmin

import (
	"context"
	"errors"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/apperror"
	"ecommerce/internal/media"
	"ecommerce/internal/services/categories"
	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	maxNameLength        = 120
	maxDescriptionLength = 500
	maxCategoryDepth     = 5
)

type Service struct {
	db    *gorm.DB
	media *media.Service
}

func NewService(db *gorm.DB, mediaService *media.Service) *Service {
	return &Service{db: db, media: mediaService}
}

func (s *Service) ListBrands(ctx context.Context, activeOnly bool, query string) ([]models.Brand, error) {
	db := s.db.WithContext(ctx).Model(&models.Brand{})
	if activeOnly {
		db = db.Where("is_active = ?", true)
	}
	if query = strings.TrimSpace(query); query != "" {
		like := "%" + strings.ToLower(query) + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?", like, like, like)
	}
	var brands []models.Brand
	return brands, db.Order("name asc, id asc").Find(&brands).Error
}

func (s *Service) CreateBrand(ctx context.Context, input apicontract.BrandInput) (models.Brand, error) {
	db := s.db.WithContext(ctx)
	brand, err := validateBrand(db, input, 0)
	if err != nil {
		return models.Brand{}, err
	}
	logoID, err := s.validatedBrandLogo(ctx, input.Logo)
	if err != nil {
		return models.Brand{}, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(&brand).Error; err != nil {
			return err
		}
		return replaceBrandLogo(tx, brand.ID, logoID)
	})
	return brand, err
}

func (s *Service) UpdateBrand(ctx context.Context, id uint, input apicontract.BrandInput) (models.Brand, error) {
	db := s.db.WithContext(ctx)
	var brand models.Brand
	if err := db.First(&brand, id).Error; err != nil {
		return brand, err
	}
	normalized, err := validateBrand(db, input, id)
	if err != nil {
		return brand, err
	}
	logoID, err := s.validatedBrandLogo(ctx, input.Logo)
	if err != nil {
		return brand, err
	}
	brand.Name, brand.Slug, brand.Description, brand.IsActive = normalized.Name, normalized.Slug, normalized.Description, normalized.IsActive
	var removed []string
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&brand).Error; err != nil {
			return err
		}
		var err error
		removed, err = currentBrandLogoIDs(tx, brand.ID, logoID)
		if err != nil {
			return err
		}
		return replaceBrandLogo(tx, brand.ID, logoID)
	})
	if err == nil {
		s.cleanupMedia(removed)
	}
	return brand, err
}

func (s *Service) DeleteBrand(ctx context.Context, id uint) error {
	db := s.db.WithContext(ctx)
	var brand models.Brand
	if err := db.First(&brand, id).Error; err != nil {
		return err
	}
	var count int64
	if err := db.Model(&models.Product{}).Where("brand_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "brand_in_use", "Brand is still assigned to products.")
	}
	if err := db.Model(&models.ProductDraft{}).Where("brand_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "brand_in_use", "Brand is still assigned to product drafts.")
	}
	removed, err := currentBrandLogoIDs(db, brand.ID, "")
	if err != nil {
		return err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := replaceBrandLogo(tx, brand.ID, ""); err != nil {
			return err
		}
		return tx.Delete(&brand).Error
	})
	if err == nil {
		s.cleanupMedia(removed)
	}
	return err
}

func (s *Service) validatedBrandLogo(ctx context.Context, input *apicontract.BrandLogoInput) (string, error) {
	if input == nil {
		return "", nil
	}
	if s.media == nil {
		return "", errors.New("media service is unavailable")
	}
	mediaID := strings.TrimSpace(input.MediaId)
	if mediaID == "" {
		return "", invalidInput("invalid_media", "Logo media ID is required.")
	}
	var object models.MediaObject
	if err := s.db.WithContext(ctx).First(&object, "id = ?", mediaID).Error; err != nil {
		return "", err
	}
	if object.Status != media.StatusReady || object.OriginalPath == "" {
		return "", invalidInput("invalid_media", "Logo media is not ready.")
	}
	if !strings.HasPrefix(object.MimeType, "image/") {
		return "", invalidInput("invalid_media", "Logo media must be an image.")
	}
	return mediaID, nil
}

func currentBrandLogoIDs(db *gorm.DB, brandID uint, keep string) ([]string, error) {
	var refs []models.MediaReference
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeBrand, brandID, media.RoleBrandLogo).Find(&refs).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.MediaID != keep {
			result = append(result, ref.MediaID)
		}
	}
	return result, nil
}

func replaceBrandLogo(db *gorm.DB, brandID uint, mediaID string) error {
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeBrand, brandID, media.RoleBrandLogo).Delete(&models.MediaReference{}).Error; err != nil {
		return err
	}
	if mediaID == "" {
		return nil
	}
	return db.Create(&models.MediaReference{MediaID: mediaID, OwnerType: media.OwnerTypeBrand, OwnerID: brandID, Role: media.RoleBrandLogo}).Error
}

func (s *Service) cleanupMedia(ids []string) {
	if s.media == nil {
		return
	}
	for _, id := range ids {
		_ = s.media.DeleteIfOrphan(id)
	}
}

func validateBrand(db *gorm.DB, input apicontract.BrandInput, excludeID uint) (models.Brand, error) {
	name := strings.TrimSpace(input.Name)
	if !categories.IsValidName(name) || len(name) > maxNameLength {
		return models.Brand{}, invalidInput("invalid_brand", "Brand name must be between 2 and 120 characters.")
	}
	slug := normalizedSlug(input.Slug, name)
	if slug == "" {
		return models.Brand{}, invalidInput("invalid_brand", "Brand slug is required.")
	}
	description := normalizedOptionalString(input.Description)
	if description != nil && len(*description) > maxDescriptionLength {
		return models.Brand{}, invalidInput("invalid_brand", "Brand description must be 500 characters or fewer.")
	}
	var count int64
	if err := db.Model(&models.Brand{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&count).Error; err != nil {
		return models.Brand{}, err
	}
	if count != 0 {
		return models.Brand{}, invalidInput("duplicate_brand_slug", "Brand slug already exists.")
	}
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	return models.Brand{Name: name, Slug: slug, Description: description, IsActive: active}, nil
}

func (s *Service) ListCategories(ctx context.Context, includeInactive bool, query string) ([]models.Category, error) {
	db := s.db.WithContext(ctx).Model(&models.Category{})
	if !includeInactive {
		db = db.Where("is_active = ?", true)
	}
	if query = strings.TrimSpace(query); query != "" {
		like := "%" + strings.ToLower(query) + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?", like, like, like)
	}
	var values []models.Category
	return values, db.Order("sort_order asc, id asc").Find(&values).Error
}

func (s *Service) CreateCategory(ctx context.Context, input apicontract.CategoryInput) (models.Category, error) {
	db := s.db.WithContext(ctx)
	category, err := validateCategory(db, input, 0)
	if err != nil {
		return models.Category{}, err
	}
	return category, db.Select("*").Create(&category).Error
}

func (s *Service) UpdateCategory(ctx context.Context, id uint, input apicontract.CategoryInput) (models.Category, error) {
	db := s.db.WithContext(ctx)
	var category models.Category
	if err := db.First(&category, id).Error; err != nil {
		return category, err
	}
	normalized, err := validateCategory(db, input, id)
	if err != nil {
		return category, err
	}
	category.Name, category.Slug, category.Description = normalized.Name, normalized.Slug, normalized.Description
	if category.IsActive && !normalized.IsActive {
		referenced, err := categoryHasPublishedProductReferences(db, category.ID)
		if err != nil {
			return category, err
		}
		if referenced {
			return category, apperror.New(apperror.KindConflict, "category_in_use", "Category is assigned to published products.")
		}
	}
	category.IsActive, category.SortOrder = normalized.IsActive, normalized.SortOrder
	category.ParentID, category.Path, category.Depth = normalized.ParentID, normalized.Path, normalized.Depth
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&category).Error; err != nil {
			return err
		}
		return rebuildCategoryPaths(tx, category)
	})
	return category, err
}

func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	db := s.db.WithContext(ctx)
	var category models.Category
	if err := db.First(&category, id).Error; err != nil {
		return err
	}
	var count int64
	if err := db.Model(&models.Category{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "category_has_children", "Category has child categories.")
	}
	if err := db.Table("product_categories pc").Joins("JOIN products p ON p.id = pc.product_id").Where("pc.category_id = ? AND p.is_published = ? AND p.deleted_at IS NULL", id, true).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "category_in_use", "Category is assigned to published products.")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category_id = ?", id).Delete(&models.ProductCategory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("category_id = ?", id).Delete(&models.ProductCategoryDraft{}).Error; err != nil {
			return err
		}
		return tx.Delete(&category).Error
	})
}

func categoryHasPublishedProductReferences(db *gorm.DB, categoryID uint) (bool, error) {
	var count int64
	err := db.Table("product_categories pc").
		Joins("JOIN products p ON p.id = pc.product_id").
		Where("pc.category_id = ? AND p.is_published = ? AND p.deleted_at IS NULL", categoryID, true).
		Count(&count).Error
	return count > 0, err
}

func validateCategory(db *gorm.DB, input apicontract.CategoryInput, excludeID uint) (models.Category, error) {
	name := strings.TrimSpace(input.Name)
	if !categories.IsValidName(name) || len(name) > maxNameLength {
		return models.Category{}, invalidInput("invalid_category", "Category name must be between 2 and 120 characters.")
	}
	slug := normalizedSlug(input.Slug, name)
	if slug == "" {
		return models.Category{}, invalidInput("invalid_category", "Category slug is required.")
	}
	description := normalizedOptionalString(input.Description)
	if description != nil && len(*description) > maxDescriptionLength {
		return models.Category{}, invalidInput("invalid_category", "Category description must be 500 characters or fewer.")
	}
	var count int64
	if err := db.Model(&models.Category{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&count).Error; err != nil {
		return models.Category{}, err
	}
	if count != 0 {
		return models.Category{}, invalidInput("duplicate_category_slug", "Category slug already exists.")
	}
	active, sortOrder := true, 0
	if input.IsActive != nil {
		active = *input.IsActive
	}
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	result := models.Category{Name: name, Slug: slug, Description: description, IsActive: active, SortOrder: sortOrder, Path: "/" + slug}
	if input.ParentId != nil {
		if *input.ParentId < 1 || uint(*input.ParentId) == excludeID {
			return models.Category{}, invalidInput("invalid_category_parent", "Parent category is invalid.")
		}
		var parent models.Category
		if err := db.First(&parent, *input.ParentId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return models.Category{}, apperror.Wrap(apperror.KindInvalidInput, "invalid_category_parent", "Parent category does not exist.", err)
			}
			return models.Category{}, err
		}
		if excludeID != 0 {
			var current models.Category
			if err := db.First(&current, excludeID).Error; err != nil {
				return models.Category{}, err
			}
			if parent.ID == current.ID || parent.Path == current.Path || strings.HasPrefix(parent.Path, strings.TrimRight(current.Path, "/")+"/") {
				return models.Category{}, apperror.New(apperror.KindInvalidInput, "category_cycle", "Parent category would create a cycle.")
			}
		}
		result.Depth = parent.Depth + 1
		if result.Depth > maxCategoryDepth {
			return models.Category{}, invalidInput("category_depth_exceeded", "Category depth exceeds the maximum.")
		}
		parentID := uint(*input.ParentId)
		result.ParentID = &parentID
		result.Path = strings.TrimRight(parent.Path, "/") + "/" + slug
	}
	return result, nil
}

func rebuildCategoryPaths(db *gorm.DB, parent models.Category) error {
	var children []models.Category
	if err := db.Where("parent_id = ?", parent.ID).Find(&children).Error; err != nil {
		return err
	}
	for i := range children {
		children[i].Depth = parent.Depth + 1
		children[i].Path = strings.TrimRight(parent.Path, "/") + "/" + children[i].Slug
		if children[i].Depth > maxCategoryDepth {
			return invalidInput("category_depth_exceeded", "Category depth exceeds the maximum.")
		}
		if err := db.Save(&children[i]).Error; err != nil {
			return err
		}
		if err := rebuildCategoryPaths(db, children[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListAttributes(ctx context.Context, filterableOnly bool) ([]models.ProductAttribute, error) {
	db := s.db.WithContext(ctx).Model(&models.ProductAttribute{})
	if filterableOnly {
		db = db.Where("filterable = ?", true)
	}
	var values []models.ProductAttribute
	return values, db.Order("key asc, id asc").Find(&values).Error
}

func (s *Service) CreateAttribute(ctx context.Context, input apicontract.ProductAttributeDefinitionInput) (models.ProductAttribute, error) {
	db := s.db.WithContext(ctx)
	value, err := validateAttribute(db, input, 0)
	if err != nil {
		return value, err
	}
	return value, db.Select("*").Create(&value).Error
}

func (s *Service) UpdateAttribute(ctx context.Context, id uint, input apicontract.ProductAttributeDefinitionInput) (models.ProductAttribute, error) {
	db := s.db.WithContext(ctx)
	var value models.ProductAttribute
	if err := db.First(&value, id).Error; err != nil {
		return value, err
	}
	normalized, err := validateAttribute(db, input, id)
	if err != nil {
		return value, err
	}
	value.Key, value.Slug, value.Type = normalized.Key, normalized.Slug, normalized.Type
	value.Filterable, value.Sortable, value.EnumValues = normalized.Filterable, normalized.Sortable, normalized.EnumValues
	return value, db.Save(&value).Error
}

func (s *Service) DeleteAttribute(ctx context.Context, id uint) error {
	db := s.db.WithContext(ctx)
	var value models.ProductAttribute
	if err := db.First(&value, id).Error; err != nil {
		return err
	}
	var count int64
	if err := db.Model(&models.ProductAttributeValue{}).Where("product_attribute_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "product_attribute_in_use", "Product attribute is still assigned to products.")
	}
	if err := db.Model(&models.ProductAttributeValueDraft{}).Where("product_attribute_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return apperror.New(apperror.KindConflict, "product_attribute_in_use", "Product attribute is still assigned to product drafts.")
	}
	return db.Delete(&value).Error
}

func validateAttribute(db *gorm.DB, input apicontract.ProductAttributeDefinitionInput, excludeID uint) (models.ProductAttribute, error) {
	key := strings.TrimSpace(input.Key)
	if !categories.IsValidName(key) || len(key) > maxNameLength {
		return models.ProductAttribute{}, invalidInput("invalid_product_attribute", "Product attribute key must be between 2 and 120 characters.")
	}
	slug := normalizedSlug(input.Slug, key)
	kind := string(input.Type)
	if kind != "text" && kind != "number" && kind != "boolean" && kind != "enum" {
		return models.ProductAttribute{}, invalidInput("invalid_product_attribute", "Product attribute type is invalid.")
	}
	filterable := input.Filterable != nil && *input.Filterable
	sortable := input.Sortable != nil && *input.Sortable
	if sortable && kind != "number" {
		return models.ProductAttribute{}, invalidInput("invalid_product_attribute", "Only number attributes can be sortable.")
	}
	var enumValues models.StringArray
	if kind == "enum" {
		if input.EnumValues == nil || len(*input.EnumValues) == 0 {
			return models.ProductAttribute{}, invalidInput("invalid_product_attribute", "Enum attributes require at least one allowed value.")
		}
		enumValues = append(enumValues, (*input.EnumValues)...)
	}
	var count int64
	if err := db.Model(&models.ProductAttribute{}).Where("(slug = ? OR key = ?) AND id <> ?", slug, key, excludeID).Count(&count).Error; err != nil {
		return models.ProductAttribute{}, err
	}
	if count != 0 {
		return models.ProductAttribute{}, invalidInput("duplicate_product_attribute", "Product attribute key or slug already exists.")
	}
	return models.ProductAttribute{Key: key, Slug: slug, Type: kind, Filterable: filterable, Sortable: sortable, EnumValues: enumValues}, nil
}

func invalidInput(code, detail string) error {
	return apperror.New(apperror.KindInvalidInput, code, detail)
}

func normalizedSlug(raw *string, fallback string) string {
	if raw != nil && strings.TrimSpace(*raw) != "" {
		return categories.NormalizeSlug(*raw)
	}
	return categories.NormalizeSlug(fallback)
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
