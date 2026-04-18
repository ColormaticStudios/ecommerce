package handlers

import (
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"gorm.io/gorm"
)

type cartItemResponse struct {
	ID               int                        `json:"id"`
	CartID           int                        `json:"cart_id"`
	ProductVariantID int                        `json:"product_variant_id"`
	Quantity         int                        `json:"quantity"`
	ProductVariant   apicontract.ProductVariant `json:"product_variant"`
	Product          apicontract.Product        `json:"product"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	DeletedAt        *time.Time                 `json:"deleted_at,omitempty"`
}

type cartResponse struct {
	ID        int                `json:"id"`
	UserID    int                `json:"user_id"`
	Items     []cartItemResponse `json:"items"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	DeletedAt *time.Time         `json:"deleted_at,omitempty"`
}

type variantSelectionRow struct {
	ProductVariantID     uint
	ProductOptionValueID uint
	OptionName           string
	OptionValue          string
	Position             int
}

func buildCartResponse(db *gorm.DB, mediaService *media.Service, cart models.Cart) (cartResponse, error) {
	variantContracts, productContracts, err := loadCartOrderContracts(db, mediaService, cart.Items)
	if err != nil {
		return cartResponse{}, err
	}

	items := make([]cartItemResponse, 0, len(cart.Items))
	for _, item := range cart.Items {
		product := item.ProductVariant.Product
		items = append(items, cartItemResponse{
			ID:               int(item.ID),
			CartID:           int(item.CartID),
			ProductVariantID: int(item.ProductVariantID),
			Quantity:         item.Quantity,
			ProductVariant:   variantContracts[item.ProductVariantID],
			Product:          productContracts[product.ID],
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
			DeletedAt:        toContractDeletedAt(item.DeletedAt),
		})
	}

	return cartResponse{
		ID:        int(cart.ID),
		UserID:    contractCartUserID(cart),
		Items:     items,
		CreatedAt: cart.CreatedAt,
		UpdatedAt: cart.UpdatedAt,
		DeletedAt: toContractDeletedAt(cart.DeletedAt),
	}, nil
}

func contractCartUserID(cart models.Cart) int {
	if cart.CheckoutSession.UserID == nil {
		return 0
	}
	return int(*cart.CheckoutSession.UserID)
}

func buildCartItemResponse(
	db *gorm.DB,
	mediaService *media.Service,
	item models.CartItem,
) (cartItemResponse, error) {
	variantContracts, productContracts, err := loadCartOrderContracts(db, mediaService, []models.CartItem{item})
	if err != nil {
		return cartItemResponse{}, err
	}

	product := item.ProductVariant.Product
	return cartItemResponse{
		ID:               int(item.ID),
		CartID:           int(item.CartID),
		ProductVariantID: int(item.ProductVariantID),
		Quantity:         item.Quantity,
		ProductVariant:   variantContracts[item.ProductVariantID],
		Product:          productContracts[product.ID],
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		DeletedAt:        toContractDeletedAt(item.DeletedAt),
	}, nil
}

func buildOrderResponse(
	db *gorm.DB,
	mediaService *media.Service,
	order models.Order,
) (apicontract.Order, error) {
	variantContracts, productContracts, err := loadCartOrderContracts(db, mediaService, order.Items)
	if err != nil {
		return apicontract.Order{}, err
	}

	items := make([]apicontract.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		product := item.ProductVariant.Product
		items = append(items, apicontract.OrderItem{
			CreatedAt:        item.CreatedAt,
			DeletedAt:        toContractDeletedAt(item.DeletedAt),
			Id:               int(item.ID),
			OrderId:          int(item.OrderID),
			Price:            item.Price.Float64(),
			Product:          productContracts[product.ID],
			ProductVariant:   variantContracts[item.ProductVariantID],
			ProductVariantId: int(item.ProductVariantID),
			Quantity:         item.Quantity,
			UpdatedAt:        item.UpdatedAt,
			VariantSku:       item.VariantSKU,
			VariantTitle:     item.VariantTitle,
		})
	}

	return apicontract.Order{
		CanCancel:             order.CanCancel,
		CheckoutSessionId:     int(order.CheckoutSessionID),
		ConfirmationToken:     order.ConfirmationToken,
		CreatedAt:             order.CreatedAt,
		DeletedAt:             toContractDeletedAt(order.DeletedAt),
		GuestEmail:            contractGuestEmail(order.GuestEmail),
		Id:                    int(order.ID),
		Items:                 items,
		PaymentMethodDisplay:  optionalString(order.PaymentMethodDisplay),
		ShippingAddressPretty: optionalString(order.ShippingAddressPretty),
		Status:                apicontract.OrderStatus(order.Status),
		Total:                 order.Total.Float64(),
		UpdatedAt:             order.UpdatedAt,
		UserId:                contractOrderUserID(order.UserID),
	}, nil
}

func contractOrderUserID(userID *uint) *int {
	if userID == nil {
		return nil
	}
	value := int(*userID)
	return &value
}

func contractGuestEmail(email *string) *openapi_types.Email {
	if email == nil {
		return nil
	}
	value := openapi_types.Email(*email)
	return &value
}

func loadCartOrderContracts(
	db *gorm.DB,
	mediaService *media.Service,
	items any,
) (map[uint]apicontract.ProductVariant, map[uint]apicontract.Product, error) {
	var (
		cartItems  []models.CartItem
		orderItems []models.OrderItem
	)

	switch typed := items.(type) {
	case []models.CartItem:
		cartItems = typed
	case []models.OrderItem:
		orderItems = typed
	}

	variantIDs := make([]uint, 0)
	productByID := make(map[uint]models.Product)
	seenVariants := map[uint]struct{}{}
	for _, item := range cartItems {
		if _, exists := seenVariants[item.ProductVariantID]; !exists {
			seenVariants[item.ProductVariantID] = struct{}{}
			variantIDs = append(variantIDs, item.ProductVariantID)
		}
		product := item.ProductVariant.Product
		productByID[product.ID] = product
	}
	for _, item := range orderItems {
		if _, exists := seenVariants[item.ProductVariantID]; !exists {
			seenVariants[item.ProductVariantID] = struct{}{}
			variantIDs = append(variantIDs, item.ProductVariantID)
		}
		product := item.ProductVariant.Product
		productByID[product.ID] = product
	}

	selectionMap, err := loadVariantSelections(db, variantIDs)
	if err != nil {
		return nil, nil, err
	}

	variantContracts := make(map[uint]apicontract.ProductVariant, len(variantIDs))
	for _, item := range cartItems {
		variantContracts[item.ProductVariantID] = buildVariantContract(item.ProductVariant, selectionMap[item.ProductVariantID])
	}
	for _, item := range orderItems {
		variantContracts[item.ProductVariantID] = buildVariantContract(item.ProductVariant, selectionMap[item.ProductVariantID])
	}

	productContracts := make(map[uint]apicontract.Product, len(productByID))
	for productID, product := range productByID {
		contract, err := buildProductContract(db, mediaService, product, false, false, false)
		if err != nil {
			return nil, nil, err
		}
		productContracts[productID] = contract
	}

	return variantContracts, productContracts, nil
}

func loadVariantSelections(db *gorm.DB, variantIDs []uint) (map[uint][]apicontract.ProductVariantSelection, error) {
	if len(variantIDs) == 0 {
		return map[uint][]apicontract.ProductVariantSelection{}, nil
	}

	var rows []variantSelectionRow
	if err := db.Table("product_variant_option_values").
		Select(
			"product_variant_option_values.product_variant_id",
			"product_variant_option_values.product_option_value_id",
			"product_options.name AS option_name",
			"product_option_values.value AS option_value",
			"product_option_values.position",
		).
		Joins("JOIN product_option_values ON product_option_values.id = product_variant_option_values.product_option_value_id").
		Joins("JOIN product_options ON product_options.id = product_option_values.product_option_id").
		Where("product_variant_option_values.product_variant_id IN ?", variantIDs).
		Order("product_options.position ASC").
		Order("product_option_values.position ASC").
		Order("product_variant_option_values.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint][]apicontract.ProductVariantSelection, len(variantIDs))
	for _, row := range rows {
		id := int(row.ProductOptionValueID)
		result[row.ProductVariantID] = append(result[row.ProductVariantID], apicontract.ProductVariantSelection{
			OptionName:           row.OptionName,
			OptionValue:          row.OptionValue,
			Position:             row.Position,
			ProductOptionValueId: &id,
		})
	}
	return result, nil
}

func buildVariantContract(
	variant models.ProductVariant,
	selections []apicontract.ProductVariantSelection,
) apicontract.ProductVariant {
	var compareAtPrice *float64
	if variant.CompareAtPrice != nil {
		value := variant.CompareAtPrice.Float64()
		compareAtPrice = &value
	}

	response := apicontract.ProductVariant{
		CompareAtPrice: compareAtPrice,
		HeightCm:       variant.HeightCm,
		IsPublished:    variant.IsPublished,
		LengthCm:       variant.LengthCm,
		Position:       variant.Position,
		Price:          variant.Price.Float64(),
		Selections:     selections,
		Sku:            variant.SKU,
		Stock:          variant.Stock,
		Title:          variant.Title,
		WeightGrams:    variant.WeightGrams,
		WidthCm:        variant.WidthCm,
	}
	id := int(variant.ID)
	response.Id = &id
	return response
}
