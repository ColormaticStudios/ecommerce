package handlers

import (
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"
)

func resolveMediaService(mediaServices ...*media.Service) *media.Service {
	if len(mediaServices) == 0 {
		return nil
	}
	return mediaServices[0]
}

func applyOrderMedia(orders []models.Order, mediaService *media.Service) {
	if mediaService == nil || len(orders) == 0 {
		return
	}

	productIDs := make([]uint, 0)
	seen := map[uint]struct{}{}
	for i := range orders {
		for j := range orders[i].Items {
			productID := orders[i].Items[j].ProductID
			if productID == 0 {
				continue
			}
			if _, ok := seen[productID]; ok {
				continue
			}
			seen[productID] = struct{}{}
			productIDs = append(productIDs, productID)
		}
	}

	mediaByProduct, err := mediaService.ProductMediaURLsByProductIDs(productIDs)
	if err != nil {
		return
	}

	for i := range orders {
		for j := range orders[i].Items {
			product := &orders[i].Items[j].Product
			if len(product.Images) > 0 && product.CoverImage == nil {
				product.CoverImage = &product.Images[0]
			}

			mediaURLs := mediaByProduct[orders[i].Items[j].ProductID]
			if len(mediaURLs) > 0 {
				product.Images = mediaURLs
				product.CoverImage = &mediaURLs[0]
			}
		}
	}
}

func applyOrderMediaToOrder(order *models.Order, mediaService *media.Service) {
	if order == nil {
		return
	}
	orders := []models.Order{*order}
	applyOrderMedia(orders, mediaService)
	*order = orders[0]
}

func applyOrderCapabilities(order *models.Order, userID *uint) {
	if order == nil {
		return
	}
	if userID == nil {
		order.CanCancel = false
		return
	}
	order.CanCancel = order.UserID == *userID && models.IsUserCancelableOrderStatus(order.Status)
}

func applyOrderCapabilitiesToList(orders []models.Order, userID *uint) {
	for i := range orders {
		applyOrderCapabilities(&orders[i], userID)
	}
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type ProcessPaymentInputMethod struct {
	CardholderName string `json:"cardholder_name" binding:"required"`
	CardNumber     string `json:"card_number" binding:"required"`
	ExpMonth       int    `json:"exp_month" binding:"required,min=1,max=12"`
	ExpYear        int    `json:"exp_year" binding:"required,min=2000,max=2200"`
}

type ProcessPaymentInputAddress struct {
	FullName   string `json:"full_name" binding:"required"`
	Line1      string `json:"line1" binding:"required"`
	Line2      string `json:"line2"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code" binding:"required"`
	Country    string `json:"country" binding:"required,len=2"`
}

type ProcessPaymentRequest struct {
	PaymentMethodID    *uint                       `json:"payment_method_id"`
	AddressID          *uint                       `json:"address_id"`
	PaymentMethod      *ProcessPaymentInputMethod  `json:"payment_method"`
	Address            *ProcessPaymentInputAddress `json:"address"`
	PaymentProviderID  string                      `json:"payment_provider_id"`
	ShippingProviderID string                      `json:"shipping_provider_id"`
	TaxProviderID      string                      `json:"tax_provider_id"`
	PaymentData        map[string]string           `json:"payment_data"`
	ShippingData       map[string]string           `json:"shipping_data"`
	TaxData            map[string]string           `json:"tax_data"`
}

type userOrderFilters struct {
	status    string
	startDate time.Time
	endDate   time.Time
}
