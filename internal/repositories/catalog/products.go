package catalog

import (
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
)

type ProductListFilters struct {
	SearchTerm string
	MinPrice   *float64
	MaxPrice   *float64
	SortField  string
	SortOrder  string
	Page       int
	Limit      int
	Preview    bool
}

type ProductListResult struct {
	Products []models.Product
	Total    int64
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func normalizeSort(sortField, sortOrder string) (string, string) {
	validSortFields := map[string]bool{"price": true, "name": true, "created_at": true}
	if !validSortFields[sortField] {
		sortField = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortField, sortOrder
}

func (r *Repository) ListProducts(filters ProductListFilters) (ProductListResult, error) {
	query := r.db.Model(&models.Product{})
	if filters.Preview {
		query = query.Where("is_published = ? OR (draft_data IS NOT NULL AND draft_data <> '')", true)
	} else {
		query = query.Where("is_published = ?", true)
	}

	if term := strings.TrimSpace(filters.SearchTerm); term != "" {
		query = query.Where("name ILIKE ?", "%"+term+"%")
	}
	if filters.MinPrice != nil {
		query = query.Where("price >= ?", *filters.MinPrice)
	}
	if filters.MaxPrice != nil {
		query = query.Where("price <= ?", *filters.MaxPrice)
	}

	sortField, sortOrder := normalizeSort(filters.SortField, filters.SortOrder)
	query = query.Order(sortField + " " + sortOrder)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ProductListResult{}, err
	}

	offset := (filters.Page - 1) * filters.Limit
	var products []models.Product
	if err := query.Offset(offset).Limit(filters.Limit).Find(&products).Error; err != nil {
		return ProductListResult{}, err
	}

	return ProductListResult{Products: products, Total: total}, nil
}

func (r *Repository) GetPublicProductByID(id string) (models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Related", "is_published = ?", true).First(&product, id).Error; err != nil {
		return models.Product{}, err
	}
	return product, nil
}

func (r *Repository) GetPreviewProductByID(id string) (models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Related").First(&product, id).Error; err != nil {
		return models.Product{}, err
	}
	return product, nil
}
