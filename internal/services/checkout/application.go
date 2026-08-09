package checkout

import (
	"context"
	"errors"
	"time"

	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const SessionTTL = 30 * 24 * time.Hour

var (
	ErrCartItemNotFound      = errors.New("cart item not found")
	ErrGuestCheckoutDisabled = errors.New("guest checkout is disabled")
	ErrInvalidQuantity       = errors.New("quantity must be greater than zero")
	ErrVariantNotFound       = errors.New("product variant not found")
)

type Service struct{ db *gorm.DB }

type sessionContextKey struct{}

type ResolveSessionInput struct {
	UserID         uint
	Token          string
	AllowConverted bool
	Create         bool
}

type ResolveSessionResult struct {
	Session   *models.CheckoutSession
	SetCookie bool
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func WithSession(ctx context.Context, session models.CheckoutSession) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

func SessionFromContext(ctx context.Context) (models.CheckoutSession, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(models.CheckoutSession)
	return session, ok
}

func (s *Service) ResolveSession(ctx context.Context, input ResolveSessionInput) (ResolveSessionResult, error) {
	if s == nil || s.db == nil {
		return ResolveSessionResult{}, errors.New("checkout service is not configured")
	}
	now := time.Now().UTC()
	if input.UserID == 0 {
		var settings models.WebsiteSettings
		err := s.db.WithContext(ctx).First(&settings, models.WebsiteSettingsSingletonID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = models.WebsiteSettings{ID: models.WebsiteSettingsSingletonID, AllowGuestCheckout: true}
			if err = s.db.WithContext(ctx).Select("*").Create(&settings).Error; err != nil {
				return ResolveSessionResult{}, err
			}
		} else if err != nil {
			return ResolveSessionResult{}, err
		}
		if !settings.AllowGuestCheckout {
			return ResolveSessionResult{}, ErrGuestCheckoutDisabled
		}
	}

	statuses := []string{models.CheckoutSessionStatusActive}
	if input.AllowConverted {
		statuses = append(statuses, models.CheckoutSessionStatusConverted)
	}
	var session *models.CheckoutSession
	if input.Token != "" {
		var byToken models.CheckoutSession
		err := s.db.WithContext(ctx).Where("public_token = ? AND status IN ? AND expires_at > ?", input.Token, statuses, now).First(&byToken).Error
		if err == nil {
			session = &byToken
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolveSessionResult{}, err
		}
	}
	if session != nil && input.UserID != 0 && session.UserID != nil && *session.UserID != input.UserID {
		session = nil
	}

	var linked *models.CheckoutSession
	if input.UserID != 0 {
		var byUser models.CheckoutSession
		err := s.db.WithContext(ctx).Where("user_id = ? AND status = ? AND expires_at > ?", input.UserID, models.CheckoutSessionStatusActive, now).
			Order("last_seen_at DESC, id DESC").First(&byUser).Error
		if err == nil {
			linked = &byUser
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolveSessionResult{}, err
		}
		if linked != nil {
			session = linked
		}
	}

	setCookie := session != nil && session.PublicToken != input.Token
	if session == nil && input.Create {
		created := models.CheckoutSession{PublicToken: uuid.NewString(), Status: models.CheckoutSessionStatusActive, ExpiresAt: now.Add(SessionTTL), LastSeenAt: now}
		if input.UserID != 0 {
			created.UserID = &input.UserID
		}
		if err := s.db.WithContext(ctx).Create(&created).Error; err != nil {
			return ResolveSessionResult{}, err
		}
		session, setCookie = &created, true
	}
	if session == nil {
		return ResolveSessionResult{}, nil
	}

	updates := map[string]any{"last_seen_at": now}
	if session.Status == models.CheckoutSessionStatusActive && input.UserID != 0 && linked == nil && session.UserID == nil {
		updates["user_id"] = input.UserID
		session.UserID = &input.UserID
	}
	if session.Status == models.CheckoutSessionStatusActive {
		expiresAt := now.Add(SessionTTL)
		if session.ExpiresAt.Before(expiresAt.Add(-12 * time.Hour)) {
			updates["expires_at"] = expiresAt
			session.ExpiresAt = expiresAt
		}
	}
	if err := s.db.WithContext(ctx).Model(&models.CheckoutSession{}).Where("id = ?", session.ID).Updates(updates).Error; err != nil {
		return ResolveSessionResult{}, err
	}
	session.LastSeenAt = now
	return ResolveSessionResult{Session: session, SetCookie: setCookie}, nil
}

func (s *Service) SessionForUser(ctx context.Context, userID uint, allowConverted bool) (models.CheckoutSession, error) {
	if session, ok := SessionFromContext(ctx); ok {
		if session.Status == models.CheckoutSessionStatusActive || (allowConverted && session.Status == models.CheckoutSessionStatusConverted) {
			return session, nil
		}
	}
	if s == nil || s.db == nil {
		return models.CheckoutSession{}, errors.New("checkout service is not configured")
	}
	now := time.Now().UTC()
	statuses := []string{models.CheckoutSessionStatusActive}
	if allowConverted {
		statuses = append(statuses, models.CheckoutSessionStatusConverted)
	}
	var session models.CheckoutSession
	err := s.db.WithContext(ctx).Where("user_id = ? AND status IN ? AND expires_at > ?", userID, statuses, now).Order("last_seen_at DESC, id DESC").First(&session).Error
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) || allowConverted {
		return models.CheckoutSession{}, err
	}
	session = models.CheckoutSession{PublicToken: uuid.NewString(), UserID: &userID, Status: models.CheckoutSessionStatusActive, ExpiresAt: now.Add(SessionTTL), LastSeenAt: now}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return models.CheckoutSession{}, err
	}
	return session, nil
}

func (s *Service) Cart(ctx context.Context, userID uint) (models.Cart, error) {
	session, err := s.SessionForUser(ctx, userID, false)
	if err != nil {
		return models.Cart{}, err
	}
	return s.cartForSessionContext(ctx, session.ID, true)
}

func (s *Service) cartForSessionContext(ctx context.Context, sessionID uint, create bool) (models.Cart, error) {
	var cart models.Cart
	db := s.db.WithContext(ctx)
	err := db.Where("checkout_session_id = ?", sessionID).Preload("CheckoutSession").Preload("Items.ProductVariant.Product").First(&cart).Error
	if err == nil || !create || !errors.Is(err, gorm.ErrRecordNotFound) {
		return cart, err
	}
	cart = models.Cart{CheckoutSessionID: sessionID}
	if err := db.Create(&cart).Error; err != nil {
		return models.Cart{}, err
	}
	return s.cartForSessionContext(ctx, sessionID, false)
}

func (s *Service) AddCartItem(ctx context.Context, userID, variantID uint, quantity int) (models.Cart, error) {
	if quantity < 1 {
		return models.Cart{}, ErrInvalidQuantity
	}
	cart, err := s.Cart(ctx, userID)
	if err != nil {
		return models.Cart{}, err
	}
	var variant models.ProductVariant
	if err := s.db.WithContext(ctx).Where("id = ? AND is_published = ?", variantID, true).First(&variant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Cart{}, ErrVariantNotFound
		}
		return models.Cart{}, err
	}
	if quantity > variant.Stock {
		return models.Cart{}, ErrInvalidQuantity
	}
	var item models.CartItem
	err = s.db.WithContext(ctx).Where("cart_id = ? AND product_variant_id = ?", cart.ID, variantID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = models.CartItem{CartID: cart.ID, ProductVariantID: variantID, Quantity: quantity}
		err = s.db.WithContext(ctx).Create(&item).Error
	} else if err == nil {
		item.Quantity += quantity
		if item.Quantity > variant.Stock {
			return models.Cart{}, ErrInvalidQuantity
		}
		err = s.db.WithContext(ctx).Save(&item).Error
	}
	if err != nil {
		return models.Cart{}, err
	}
	return s.cartForSessionContext(ctx, cart.CheckoutSessionID, false)
}

func (s *Service) UpdateCartItem(ctx context.Context, userID, itemID uint, quantity int) (models.CartItem, error) {
	if quantity < 1 {
		return models.CartItem{}, ErrInvalidQuantity
	}
	cart, err := s.Cart(ctx, userID)
	if err != nil {
		return models.CartItem{}, err
	}
	var item models.CartItem
	if err := s.db.WithContext(ctx).Where("id = ? AND cart_id = ?", itemID, cart.ID).Preload("ProductVariant.Product").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.CartItem{}, ErrCartItemNotFound
		}
		return models.CartItem{}, err
	}
	if quantity > item.ProductVariant.Stock {
		return models.CartItem{}, ErrInvalidQuantity
	}
	if err := s.db.WithContext(ctx).Model(&item).Update("quantity", quantity).Error; err != nil {
		return models.CartItem{}, err
	}
	item.Quantity = quantity
	return item, nil
}

func (s *Service) DeleteCartItem(ctx context.Context, userID, itemID uint) error {
	cart, err := s.Cart(ctx, userID)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Where("id = ? AND cart_id = ?", itemID, cart.ID).Delete(&models.CartItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCartItemNotFound
	}
	return nil
}

func (s *Service) CartItemCount(ctx context.Context, userID uint) (int, error) {
	session, ok := SessionFromContext(ctx)
	if !ok {
		return 0, nil
	}
	var total int64
	err := s.db.WithContext(ctx).Model(&models.CartItem{}).
		Joins("JOIN carts ON carts.id = cart_items.cart_id").
		Where("carts.checkout_session_id = ?", session.ID).
		Count(&total).Error
	return int(total), err
}
