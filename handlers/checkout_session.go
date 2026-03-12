package handlers

import (
	"errors"
	"net/http"
	"time"

	"ecommerce/middleware"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	checkoutSessionCookieName   = "checkout_session"
	checkoutSessionTTL          = 30 * 24 * time.Hour
	guestCheckoutDisabledCode   = "guest_checkout_disabled"
	guestCheckoutDisabledReason = "Guest checkout is disabled"
)

type checkoutRequestContext struct {
	User    *models.User
	Session *models.CheckoutSession
	Cart    *models.Cart
}

type checkoutSessionResolveOptions struct {
	AllowConverted bool
}

type checkoutSessionOwnerContext struct {
	User    *models.User
	Session *models.CheckoutSession
}

func setCheckoutSessionCookie(c *gin.Context, token string, cfg AuthCookieConfig) {
	maxAge := int(checkoutSessionTTL.Seconds())
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		checkoutSessionCookieName,
		token,
		maxAge,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)

	if csrfToken, err := c.Cookie(csrfCookieName); err != nil || csrfToken == "" {
		setCSRFCookie(c, uuid.NewString(), cfg)
	}
}

func findAuthenticatedUserIfPresent(db *gorm.DB, c *gin.Context, jwtSecret string) (*models.User, error) {
	identity, err := middleware.ResolveAuthIdentity(c, jwtSecret)
	if err != nil {
		if errors.Is(err, middleware.ErrAuthTokenMissing) || errors.Is(err, middleware.ErrAuthTokenInvalid) {
			return nil, nil
		}
		return nil, err
	}

	var user models.User
	if err := db.Where("subject = ?", identity.Subject).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func isGuestCheckoutEnabled(db *gorm.DB) (bool, error) {
	_, settings, err := loadOrCreateStorefrontSettings(db, nil, false)
	if err != nil {
		return false, err
	}
	return settings.Checkout.AllowGuestCheckout, nil
}

func rejectGuestCheckoutDisabled(c *gin.Context) bool {
	c.JSON(http.StatusForbidden, gin.H{
		"error": guestCheckoutDisabledReason,
		"code":  guestCheckoutDisabledCode,
	})
	return false
}

func resolveCheckoutRequestContext(
	db *gorm.DB,
	c *gin.Context,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
) (*checkoutRequestContext, bool) {
	ownerCtx, ok := resolveCheckoutSessionOwnerContext(
		db,
		c,
		jwtSecret,
		cookieCfg,
		checkoutSessionResolveOptions{},
	)
	if !ok {
		return nil, false
	}

	cart, err := getOrCreateCartByCheckoutSession(db, ownerCtx.Session.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cart"})
		return nil, false
	}

	return &checkoutRequestContext{
		User:    ownerCtx.User,
		Session: ownerCtx.Session,
		Cart:    cart,
	}, true
}

func resolveCheckoutOrderRequestContext(
	db *gorm.DB,
	c *gin.Context,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
) (*checkoutSessionOwnerContext, bool) {
	return resolveCheckoutSessionOwnerContext(
		db,
		c,
		jwtSecret,
		cookieCfg,
		checkoutSessionResolveOptions{AllowConverted: true},
	)
}

func resolveCheckoutSessionOwnerContext(
	db *gorm.DB,
	c *gin.Context,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
	options checkoutSessionResolveOptions,
) (*checkoutSessionOwnerContext, bool) {
	user, err := findAuthenticatedUserIfPresent(db, c, jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve checkout session"})
		return nil, false
	}

	if user == nil {
		enabled, err := isGuestCheckoutEnabled(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
			return nil, false
		}
		if !enabled {
			return nil, rejectGuestCheckoutDisabled(c)
		}
	}

	session, setCookie, err := resolveOrCreateCheckoutSessionWithOptions(db, c, user, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve checkout session"})
		return nil, false
	}
	if setCookie {
		setCheckoutSessionCookie(c, session.PublicToken, cookieCfg)
	}

	return &checkoutSessionOwnerContext{
		User:    user,
		Session: session,
	}, true
}

func resolveExistingCheckoutSessionOwnerContext(
	db *gorm.DB,
	c *gin.Context,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
	options checkoutSessionResolveOptions,
) (*checkoutSessionOwnerContext, bool) {
	user, ok := requireCheckoutAccess(db, c, jwtSecret)
	if !ok {
		return nil, false
	}

	session, setCookie, err := resolveExistingCheckoutSessionWithOptions(db, c, user, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve checkout session"})
		return nil, false
	}
	if session != nil && setCookie {
		setCheckoutSessionCookie(c, session.PublicToken, cookieCfg)
	}

	return &checkoutSessionOwnerContext{
		User:    user,
		Session: session,
	}, true
}

func requireCheckoutAccess(
	db *gorm.DB,
	c *gin.Context,
	jwtSecret string,
) (*models.User, bool) {
	user, err := findAuthenticatedUserIfPresent(db, c, jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve checkout session"})
		return nil, false
	}
	if user != nil {
		return user, true
	}

	enabled, err := isGuestCheckoutEnabled(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load storefront settings"})
		return nil, false
	}
	if !enabled {
		return nil, rejectGuestCheckoutDisabled(c)
	}
	return nil, true
}

func resolveOrCreateCheckoutSession(db *gorm.DB, c *gin.Context, user *models.User) (*models.CheckoutSession, bool, error) {
	return resolveOrCreateCheckoutSessionWithOptions(db, c, user, checkoutSessionResolveOptions{})
}

func resolveExistingCheckoutSession(
	db *gorm.DB,
	c *gin.Context,
	user *models.User,
) (*models.CheckoutSession, bool, error) {
	return resolveExistingCheckoutSessionWithOptions(db, c, user, checkoutSessionResolveOptions{})
}

func resolveOrCreateCheckoutSessionWithOptions(
	db *gorm.DB,
	c *gin.Context,
	user *models.User,
	options checkoutSessionResolveOptions,
) (*models.CheckoutSession, bool, error) {
	return resolveCheckoutSessionWithOptions(db, c, user, options, true)
}

func resolveExistingCheckoutSessionWithOptions(
	db *gorm.DB,
	c *gin.Context,
	user *models.User,
	options checkoutSessionResolveOptions,
) (*models.CheckoutSession, bool, error) {
	return resolveCheckoutSessionWithOptions(db, c, user, options, false)
}

func resolveCheckoutSessionWithOptions(
	db *gorm.DB,
	c *gin.Context,
	user *models.User,
	options checkoutSessionResolveOptions,
	createIfMissing bool,
) (*models.CheckoutSession, bool, error) {
	now := time.Now().UTC()
	token, _ := c.Cookie(checkoutSessionCookieName)

	session, err := lookupCheckoutSessionByTokenWithOptions(db, token, now, options)
	if err != nil {
		return nil, false, err
	}
	setCookie := false

	if session != nil && user != nil && session.UserID != nil && *session.UserID != user.ID {
		session = nil
	}

	var userSession *models.CheckoutSession
	if user != nil {
		userSession, err = lookupCheckoutSessionByUser(db, user.ID, now)
		if err != nil {
			return nil, false, err
		}
		if userSession != nil {
			if session == nil || session.ID != userSession.ID || token != userSession.PublicToken {
				setCookie = true
			}
			session = userSession
		}
	}

	if session == nil && createIfMissing {
		session, err = createCheckoutSession(db, user, now)
		if err != nil {
			return nil, false, err
		}
		setCookie = true
	}

	if session == nil {
		return nil, false, nil
	}

	updates := map[string]any{}
	if session.Status == models.CheckoutSessionStatusActive &&
		user != nil &&
		userSession == nil &&
		(session.UserID == nil || *session.UserID != user.ID) {
		updates["user_id"] = user.ID
	}
	if session.Status == models.CheckoutSessionStatusActive {
		expiresAt := now.Add(checkoutSessionTTL)
		if session.ExpiresAt.Before(expiresAt.Add(-12 * time.Hour)) {
			updates["expires_at"] = expiresAt
		}
	}
	updates["last_seen_at"] = now

	if len(updates) > 0 {
		if err := db.Model(&models.CheckoutSession{}).
			Where("id = ?", session.ID).
			Updates(updates).Error; err != nil {
			return nil, false, err
		}
		if user != nil {
			session.UserID = &user.ID
		}
		session.LastSeenAt = now
		if expires, ok := updates["expires_at"].(time.Time); ok {
			session.ExpiresAt = expires
		}
	}

	return session, setCookie, nil
}

func lookupCheckoutSessionByToken(db *gorm.DB, token string, now time.Time) (*models.CheckoutSession, error) {
	return lookupCheckoutSessionByTokenWithOptions(db, token, now, checkoutSessionResolveOptions{})
}

func lookupCheckoutSessionByTokenWithOptions(
	db *gorm.DB,
	token string,
	now time.Time,
	options checkoutSessionResolveOptions,
) (*models.CheckoutSession, error) {
	if token == "" {
		return nil, nil
	}

	statuses := []string{models.CheckoutSessionStatusActive}
	if options.AllowConverted {
		statuses = append(statuses, models.CheckoutSessionStatusConverted)
	}

	var session models.CheckoutSession
	if err := db.Where("public_token = ? AND status IN ? AND expires_at > ?", token, statuses, now).
		First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func lookupCheckoutSessionByUser(db *gorm.DB, userID uint, now time.Time) (*models.CheckoutSession, error) {
	var session models.CheckoutSession
	if err := db.Where("user_id = ? AND status = ? AND expires_at > ?", userID, models.CheckoutSessionStatusActive, now).
		Order("last_seen_at DESC").
		First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func createCheckoutSession(db *gorm.DB, user *models.User, now time.Time) (*models.CheckoutSession, error) {
	session := models.CheckoutSession{
		PublicToken: uuid.NewString(),
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   now.Add(checkoutSessionTTL),
		LastSeenAt:  now,
	}
	if user != nil {
		session.UserID = &user.ID
	}

	if err := db.Create(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func getOrCreateCartByCheckoutSession(db *gorm.DB, checkoutSessionID uint) (*models.Cart, error) {
	cart, err := getCartByCheckoutSession(db, checkoutSessionID)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	cart = &models.Cart{CheckoutSessionID: checkoutSessionID}
	if err := db.Create(cart).Error; err != nil {
		if lookupErr := db.Where("checkout_session_id = ?", checkoutSessionID).
			Preload("CheckoutSession").
			Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(cart).Error; lookupErr == nil {
			return cart, nil
		}
		return nil, err
	}
	if err := db.Where("id = ?", cart.ID).
		Preload("CheckoutSession").
		Preload("Items.ProductVariant").
		Preload("Items.ProductVariant.Product").
		First(cart).Error; err != nil {
		return nil, err
	}
	return cart, nil
}

func getCartByCheckoutSession(db *gorm.DB, checkoutSessionID uint) (*models.Cart, error) {
	var cart models.Cart
	err := db.Where("checkout_session_id = ?", checkoutSessionID).
		Preload("CheckoutSession").
		Preload("Items.ProductVariant").
		Preload("Items.ProductVariant.Product").
		First(&cart).Error

	if err != nil {
		return nil, err
	}
	return &cart, nil
}
