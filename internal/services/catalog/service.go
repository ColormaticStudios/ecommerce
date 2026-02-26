package catalog

import (
	"strconv"

	"ecommerce/internal/media"
	catalogrepo "ecommerce/internal/repositories/catalog"
	"ecommerce/models"

	"gorm.io/gorm"
)

type Service struct {
	repo *catalogrepo.Repository
}

type ListProductsInput struct {
	SearchTerm string
	MinPrice   *float64
	MaxPrice   *float64
	SortField  string
	SortOrder  string
	Page       int
	Limit      int
	Preview    bool
}

type ListProductsOutput struct {
	Products   []models.Product
	Total      int64
	TotalPages int
}

func NewService(db *gorm.DB, mediaService *media.Service) *Service {
	_ = mediaService
	return &Service{repo: catalogrepo.NewRepository(db)}
}

func (s *Service) ListProducts(input ListProductsInput) (ListProductsOutput, error) {
	result, err := s.repo.ListProducts(catalogrepo.ProductListFilters{
		SearchTerm: input.SearchTerm,
		MinPrice:   input.MinPrice,
		MaxPrice:   input.MaxPrice,
		SortField:  input.SortField,
		SortOrder:  input.SortOrder,
		Page:       input.Page,
		Limit:      input.Limit,
		Preview:    input.Preview,
	})
	if err != nil {
		return ListProductsOutput{}, err
	}

	totalPages := int(result.Total) / input.Limit
	if int(result.Total)%input.Limit > 0 {
		totalPages++
	}

	return ListProductsOutput{Products: result.Products, Total: result.Total, TotalPages: totalPages}, nil
}

func (s *Service) GetProductByID(id string, preview bool) (models.Product, error) {
	if preview {
		product, err := s.repo.GetPreviewProductByID(id)
		if err != nil {
			return models.Product{}, err
		}
		return product, nil
	}

	product, err := s.repo.GetPublicProductByID(id)
	if err != nil {
		return models.Product{}, err
	}
	if !product.IsPublished {
		return models.Product{}, gorm.ErrRecordNotFound
	}
	return product, nil
}

func ParsePrice(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
