package orders

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	checkoutservice "ecommerce/internal/services/checkout"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderCannotBeCanceled = errors.New("order cannot be canceled")
	ErrOrderAlreadyClaimed   = errors.New("order already claimed")
	ErrInvalidClaim          = errors.New("invalid guest order claim")
	ErrCheckoutConverted     = errors.New("checkout session has already been converted")
	ErrCheckoutOrderStale    = errors.New("checkout order is no longer payable")
	ErrOrderPaymentSubmitted = errors.New("order payment already submitted")
	ErrInvalidOrderStatus    = errors.New("invalid order status")
	ErrOrderRequiresItems    = errors.New("order must contain at least one item")
	ErrInvalidOrderItem      = errors.New("invalid order item")
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func checkoutSessionFromContext(ctx context.Context) (models.CheckoutSession, bool) {
	return checkoutservice.SessionFromContext(ctx)
}

type ListInput struct {
	UserID    *uint
	Query     string
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

type Page struct {
	Orders     []models.Order
	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

type CreateItemInput struct {
	ProductVariantID uint
	Quantity         int
}

func (s *Service) Create(ctx context.Context, sessionID uint, userID *uint, guestEmail *string, items []CreateItemInput) (models.Order, error) {
	order, err := s.prepareOrderContext(ctx, sessionID, userID, guestEmail, items)
	if err != nil {
		return models.Order{}, err
	}
	if err := s.db.WithContext(ctx).Create(&order).Error; err != nil {
		return models.Order{}, err
	}
	return s.Get(ctx, order.ID, userID)
}

func (s *Service) CreateOrReplaceOpen(ctx context.Context, sessionID uint, userID *uint, guestEmail *string, items []CreateItemInput) (models.Order, bool, error) {
	candidate, err := s.prepareOrderContext(ctx, sessionID, userID, guestEmail, items)
	if err != nil {
		return models.Order{}, false, err
	}
	created := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Order
		err := tx.Where("checkout_session_id = ? AND status = ? AND TRIM(COALESCE(payment_method_display, '')) = '' AND TRIM(COALESCE(shipping_address_pretty, '')) = ''", sessionID, models.StatusPending).
			Order("id DESC").First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created = true
			return tx.Create(&candidate).Error
		}
		if err != nil {
			return err
		}
		candidate.ID = existing.ID
		candidate.CreatedAt = existing.CreatedAt
		if userID == nil {
			candidate.ConfirmationToken = existing.ConfirmationToken
		}
		if err := tx.Model(&existing).Updates(map[string]any{"user_id": userID, "guest_email": guestEmail, "confirmation_token": candidate.ConfirmationToken, "total": candidate.Total}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", existing.ID).Delete(&models.OrderItem{}).Error; err != nil {
			return err
		}
		for index := range candidate.Items {
			candidate.Items[index].OrderID = existing.ID
		}
		return tx.Create(&candidate.Items).Error
	})
	if err != nil {
		return models.Order{}, false, err
	}
	owner := userID
	if owner == nil {
		zero := uint(0)
		owner = &zero
	}
	order, err := s.Get(ctx, candidate.ID, owner)
	return order, created, err
}

func (s *Service) prepareOrderContext(ctx context.Context, sessionID uint, userID *uint, guestEmail *string, items []CreateItemInput) (models.Order, error) {
	if s == nil || s.db == nil {
		return models.Order{}, errors.New("order service is not configured")
	}
	if len(items) == 0 {
		return models.Order{}, ErrOrderRequiresItems
	}
	requested := make(map[uint]int, len(items))
	variantIDs := make([]uint, 0, len(items))
	for _, input := range items {
		if input.ProductVariantID == 0 || input.Quantity < 1 {
			return models.Order{}, ErrInvalidOrderItem
		}
		if _, exists := requested[input.ProductVariantID]; !exists {
			variantIDs = append(variantIDs, input.ProductVariantID)
		}
		requested[input.ProductVariantID] += input.Quantity
	}
	order := models.Order{CheckoutSessionID: sessionID, UserID: userID, GuestEmail: guestEmail, Status: models.StatusPending}
	for _, variantID := range variantIDs {
		quantity := requested[variantID]
		var variant models.ProductVariant
		if err := s.db.WithContext(ctx).Preload("Product").Where("id = ? AND is_published = ?", variantID, true).First(&variant).Error; err != nil {
			return models.Order{}, err
		}
		if variant.Stock < quantity {
			return models.Order{}, &InsufficientStockError{ProductVariantID: variant.ID, ProductName: variant.Product.Name, Requested: quantity, Available: variant.Stock}
		}
		order.Total += variant.Price.Mul(quantity)
		order.Items = append(order.Items, models.OrderItem{ProductVariantID: variant.ID, VariantSKU: variant.SKU, VariantTitle: variant.Title, Quantity: quantity, Price: variant.Price})
	}
	if userID == nil {
		token := uuid.NewString()
		order.ConfirmationToken = &token
	}
	return order, nil
}

func (s *Service) List(ctx context.Context, input ListInput) (Page, error) {
	if s == nil || s.db == nil {
		return Page{}, errors.New("order service is not configured")
	}
	page, limit := input.Page, input.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := s.db.WithContext(ctx).Model(&models.Order{})
	if input.UserID != nil {
		query = query.Where("user_id = ?", *input.UserID)
	}
	if status := strings.ToUpper(strings.TrimSpace(input.Status)); status != "" {
		if !models.IsValidOrderStatus(status) {
			return Page{}, ErrInvalidOrderStatus
		}
		query = query.Where("status = ?", status)
	}
	if input.StartDate != nil {
		query = query.Where("created_at >= ?", input.StartDate.UTC())
	}
	if input.EndDate != nil {
		query = query.Where("created_at <= ?", input.EndDate.UTC())
	}
	if search := strings.TrimSpace(input.Query); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("CAST(orders.id AS TEXT) LIKE ? OR LOWER(COALESCE(orders.guest_email, '')) LIKE ? OR LOWER(orders.status) LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return Page{}, err
	}
	var rows []models.Order
	if err := query.Preload("Items.ProductVariant.Product").Preload("User").Order("orders.created_at DESC, orders.id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		return Page{}, err
	}
	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}
	return Page{Orders: rows, Page: page, Limit: limit, Total: total, TotalPages: totalPages}, nil
}

func (s *Service) Get(ctx context.Context, orderID uint, userID *uint) (models.Order, error) {
	if s == nil || s.db == nil {
		return models.Order{}, errors.New("order service is not configured")
	}
	query := s.db.WithContext(ctx).Preload("Items.ProductVariant.Product").Preload("User").Where("orders.id = ?", orderID)
	if userID != nil {
		if *userID == 0 {
			session, ok := checkoutSessionFromContext(ctx)
			if !ok {
				return models.Order{}, ErrOrderNotFound
			}
			query = query.Where("orders.checkout_session_id = ?", session.ID)
		} else {
			query = query.Where("orders.user_id = ?", *userID)
		}
	}
	var order models.Order
	if err := query.First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Order{}, ErrOrderNotFound
		}
		return models.Order{}, err
	}
	return order, nil
}

func (s *Service) PayableCheckoutOrder(ctx context.Context, orderID uint, session models.CheckoutSession) (models.Order, error) {
	order, err := s.Get(ctx, orderID, func() *uint { value := uint(0); return &value }())
	if err != nil {
		return models.Order{}, err
	}
	submitted := strings.TrimSpace(order.PaymentMethodDisplay) != "" || strings.TrimSpace(order.ShippingAddressPretty) != ""
	if session.Status == models.CheckoutSessionStatusConverted && !submitted {
		return models.Order{}, ErrCheckoutConverted
	}
	if submitted {
		return models.Order{}, ErrOrderPaymentSubmitted
	}
	var current models.Order
	err = s.db.WithContext(ctx).Where("checkout_session_id = ? AND status = ? AND TRIM(COALESCE(payment_method_display, '')) = '' AND TRIM(COALESCE(shipping_address_pretty, '')) = ''", session.ID, models.StatusPending).
		Order("id DESC").First(&current).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Order{}, err
	}
	if err == nil && current.ID != order.ID {
		return models.Order{}, ErrCheckoutOrderStale
	}
	return order, nil
}

func (s *Service) UpdateStatus(ctx context.Context, orderID uint, status string) (models.Order, error) {
	if s == nil || s.db == nil {
		return models.Order{}, errors.New("order service is not configured")
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if !models.IsValidOrderStatus(status) {
		return models.Order{}, ErrInvalidOrderStatus
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
			return err
		}
		return ApplyStatusTransition(tx, &order, status)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Order{}, ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, err
	}
	return s.Get(ctx, orderID, nil)
}

func (s *Service) Cancel(ctx context.Context, orderID, userID uint) (models.Order, error) {
	if s == nil || s.db == nil {
		return models.Order{}, errors.New("order service is not configured")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
			return err
		}
		if !models.IsUserCancelableOrderStatus(order.Status) {
			return ErrOrderCannotBeCanceled
		}
		return ApplyStatusTransition(tx, &order, models.StatusCancelled)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Order{}, ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, err
	}
	return s.Get(ctx, orderID, &userID)
}

func (s *Service) ClaimGuest(ctx context.Context, userID uint, email, confirmationToken string) (models.Order, bool, error) {
	if s == nil || s.db == nil {
		return models.Order{}, false, errors.New("order service is not configured")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(strings.ToLower(email)))
	if err != nil || strings.TrimSpace(confirmationToken) == "" {
		return models.Order{}, false, ErrInvalidClaim
	}
	normalized := strings.ToLower(strings.TrimSpace(address.Address))
	var order models.Order
	if err := s.db.WithContext(ctx).Where("confirmation_token = ? AND LOWER(COALESCE(guest_email, '')) = ?", strings.TrimSpace(confirmationToken), normalized).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Order{}, false, ErrOrderNotFound
		}
		return models.Order{}, false, err
	}
	if order.UserID != nil {
		if *order.UserID != userID {
			return models.Order{}, false, ErrOrderAlreadyClaimed
		}
		loaded, err := s.Get(ctx, order.ID, &userID)
		return loaded, true, err
	}
	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Order{}).Where("id = ? AND user_id IS NULL", order.ID).Updates(map[string]any{"user_id": userID, "claimed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrOrderAlreadyClaimed
		}
		return tx.Model(&models.CheckoutSession{}).Where("id = ?", order.CheckoutSessionID).Updates(map[string]any{"user_id": userID, "guest_email": normalized, "last_seen_at": now}).Error
	})
	if err != nil {
		return models.Order{}, false, err
	}
	loaded, err := s.Get(ctx, order.ID, &userID)
	return loaded, false, err
}
