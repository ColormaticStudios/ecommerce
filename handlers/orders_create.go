package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateOrderRequest struct {
	Items []OrderItemRequest `json:"items" binding:"required"`
}

type CreateCheckoutOrderRequest struct {
	GuestEmail string `json:"guest_email"`
}

type OrderItemRequest struct {
	ProductVariantID uint `json:"product_variant_id" binding:"required"`
	Quantity         int  `json:"quantity" binding:"required,min=1"`
}

type orderVariantQuantity struct {
	ProductVariantID uint
	Quantity         int
}

type orderStockError struct {
	ProductVariantID uint
	ProductName      string
	Requested        int
	Available        int
}

func (e *orderStockError) Error() string {
	return "insufficient stock"
}

var errOrderRequiresItems = errors.New("order must contain at least one item")

func collectOrderVariantQuantities(items []orderVariantQuantity) (map[uint]int, []uint, error) {
	if len(items) == 0 {
		return nil, nil, errOrderRequiresItems
	}

	requestedByVariant := make(map[uint]int, len(items))
	orderedVariantIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.ProductVariantID == 0 || item.Quantity < 1 {
			return nil, nil, errOrderRequiresItems
		}
		if _, exists := requestedByVariant[item.ProductVariantID]; !exists {
			orderedVariantIDs = append(orderedVariantIDs, item.ProductVariantID)
		}
		requestedByVariant[item.ProductVariantID] += item.Quantity
	}
	return requestedByVariant, orderedVariantIDs, nil
}

func buildOrderItemsForVariants(
	db *gorm.DB,
	items []orderVariantQuantity,
) (models.Money, []models.OrderItem, error) {
	requestedByVariant, orderedVariantIDs, err := collectOrderVariantQuantities(items)
	if err != nil {
		return 0, nil, err
	}

	var total models.Money
	orderItems := make([]models.OrderItem, 0, len(orderedVariantIDs))
	for _, variantID := range orderedVariantIDs {
		quantity := requestedByVariant[variantID]
		variant, err := loadPublicVariant(db, variantID)
		if err != nil {
			return 0, nil, err
		}
		if variant.Stock < quantity {
			return 0, nil, &orderStockError{
				ProductVariantID: variantID,
				ProductName:      variant.Product.Name,
				Requested:        quantity,
				Available:        variant.Stock,
			}
		}

		total += variant.Price.Mul(quantity)
		orderItems = append(orderItems, models.OrderItem{
			ProductVariantID: variantID,
			VariantSKU:       variant.SKU,
			VariantTitle:     variant.Title,
			Quantity:         quantity,
			Price:            variant.Price,
		})
	}

	return total, orderItems, nil
}

func createOrderRecord(
	db *gorm.DB,
	session *models.CheckoutSession,
	userID *uint,
	guestEmail *string,
	items []orderVariantQuantity,
) (models.Order, error) {
	total, orderItems, err := buildOrderItemsForVariants(db, items)
	if err != nil {
		return models.Order{}, err
	}

	order := models.Order{
		CheckoutSessionID: session.ID,
		GuestEmail:        guestEmail,
		Total:             total,
		Status:            models.StatusPending,
		Items:             orderItems,
	}
	if userID != nil {
		order.UserID = userID
	} else {
		token := uuid.NewString()
		order.ConfirmationToken = &token
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updates := map[string]any{"last_seen_at": now}
		if guestEmail != nil {
			updates["guest_email"] = *guestEmail
		}
		if err := tx.Model(&models.CheckoutSession{}).
			Where("id = ?", session.ID).
			Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&order).Error
	})
	if err != nil {
		return models.Order{}, err
	}

	if err := db.Preload("Items.ProductVariant").
		Preload("Items.ProductVariant.Product").
		First(&order, order.ID).Error; err != nil {
		return models.Order{}, err
	}
	return order, nil
}

func writeCreateOrderError(c *gin.Context, err error) {
	status, payload := createOrderErrorResponse(err)
	if payload == nil {
		return
	}
	c.JSON(status, payload)
}

func createOrderErrorResponse(err error) (int, gin.H) {
	switch typed := err.(type) {
	case nil:
		return http.StatusOK, nil
	case *orderStockError:
		return http.StatusBadRequest, gin.H{
			"error":              "Insufficient stock",
			"product_variant_id": typed.ProductVariantID,
			"product_name":       typed.ProductName,
			"requested":          typed.Requested,
			"available":          typed.Available,
		}
	default:
		switch {
		case errors.Is(err, errOrderRequiresItems):
			return http.StatusBadRequest, gin.H{"error": err.Error()}
		case errors.Is(err, gorm.ErrRecordNotFound):
			return http.StatusBadRequest, gin.H{"error": "Product variant not found"}
		default:
			return http.StatusInternalServerError, gin.H{"error": "Failed to create order"}
		}
	}
}

func normalizeGuestEmail(value string) (*string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return nil, nil
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(addr.Address))
	return &email, nil
}

func orderQuantitiesFromRequest(items []OrderItemRequest) []orderVariantQuantity {
	quantities := make([]orderVariantQuantity, 0, len(items))
	for _, item := range items {
		quantities = append(quantities, orderVariantQuantity{
			ProductVariantID: item.ProductVariantID,
			Quantity:         item.Quantity,
		})
	}
	return quantities
}

func orderQuantitiesFromCartItems(items []models.CartItem) []orderVariantQuantity {
	quantities := make([]orderVariantQuantity, 0, len(items))
	for _, item := range items {
		quantities = append(quantities, orderVariantQuantity{
			ProductVariantID: item.ProductVariantID,
			Quantity:         item.Quantity,
		})
	}
	return quantities
}

func checkoutOrderPaymentSubmitted(order models.Order) bool {
	return strings.TrimSpace(order.PaymentMethodDisplay) != "" ||
		strings.TrimSpace(order.ShippingAddressPretty) != ""
}

func findCurrentCheckoutOpenOrder(db *gorm.DB, sessionID uint) (*models.Order, error) {
	var order models.Order
	err := db.Where(
		`checkout_session_id = ? AND status = ? AND TRIM(COALESCE(payment_method_display, '')) = '' AND TRIM(COALESCE(shipping_address_pretty, '')) = ''`,
		sessionID,
		models.StatusPending,
	).
		Order("id DESC").
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func createOrUpdateCheckoutOrderRecord(
	db *gorm.DB,
	session *models.CheckoutSession,
	userID *uint,
	guestEmail *string,
	items []orderVariantQuantity,
) (models.Order, bool, error) {
	total, orderItems, err := buildOrderItemsForVariants(db, items)
	if err != nil {
		return models.Order{}, false, err
	}

	var order models.Order
	created := false
	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updates := map[string]any{"last_seen_at": now}
		if guestEmail != nil {
			updates["guest_email"] = *guestEmail
		}
		if err := tx.Model(&models.CheckoutSession{}).
			Where("id = ?", session.ID).
			Updates(updates).Error; err != nil {
			return err
		}

		existing, err := findCurrentCheckoutOpenOrder(tx, session.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			order = *existing
			order.Total = total
			order.GuestEmail = guestEmail
			if userID != nil {
				order.UserID = userID
				order.ConfirmationToken = nil
			}
			if err := tx.Model(&models.Order{}).
				Where("id = ?", order.ID).
				Updates(map[string]any{
					"total":              total,
					"guest_email":        guestEmail,
					"user_id":            userID,
					"confirmation_token": order.ConfirmationToken,
				}).Error; err != nil {
				return err
			}
			if err := tx.Where("order_id = ?", order.ID).Delete(&models.OrderItem{}).Error; err != nil {
				return err
			}
			for i := range orderItems {
				orderItems[i].OrderID = order.ID
			}
			if len(orderItems) > 0 {
				if err := tx.Create(&orderItems).Error; err != nil {
					return err
				}
			}
			return nil
		}

		order = models.Order{
			CheckoutSessionID: session.ID,
			GuestEmail:        guestEmail,
			Total:             total,
			Status:            models.StatusPending,
			Items:             orderItems,
		}
		if userID != nil {
			order.UserID = userID
		} else {
			token := uuid.NewString()
			order.ConfirmationToken = &token
		}
		created = true
		return tx.Create(&order).Error
	})
	if err != nil {
		return models.Order{}, false, err
	}

	if err := db.Preload("Items.ProductVariant").
		Preload("Items.ProductVariant.Product").
		First(&order, order.ID).Error; err != nil {
		return models.Order{}, false, err
	}
	return order, created, nil
}

func renderCreatedOrder(c *gin.Context, db *gorm.DB, mediaService *media.Service, order models.Order, userID *uint) {
	response, err := buildCreatedOrderResponse(db, mediaService, order, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
		return
	}
	c.JSON(http.StatusCreated, response)
}

func buildCreatedOrderResponse(
	db *gorm.DB,
	mediaService *media.Service,
	order models.Order,
	userID *uint,
) (orderResponse, error) {
	applyOrderMediaToOrder(&order, mediaService)
	applyOrderCapabilities(&order, userID)
	return buildOrderResponse(db, mediaService, order)
}

// CreateOrder creates a new order for the authenticated user.
func CreateOrder(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req CreateOrderRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		session, setCookie, err := resolveOrCreateCheckoutSession(db, c, user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve checkout session"})
			return
		}
		if setCookie {
			setCheckoutSessionCookie(c, session.PublicToken, cookieCfg)
		}

		order, err := createOrderRecord(
			db,
			session,
			&user.ID,
			nil,
			orderQuantitiesFromRequest(req.Items),
		)
		if err != nil {
			writeCreateOrderError(c, err)
			return
		}

		renderCreatedOrder(c, db, mediaService, order, &user.ID)
	}
}

// CreateCheckoutOrder creates an order from the current checkout session cart.
func CreateCheckoutOrder(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		var req CreateCheckoutOrderRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var guestEmail *string
		if requestCtx.User == nil {
			var err error
			guestEmail, err = normalizeGuestEmail(req.GuestEmail)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Guest email must be valid"})
				return
			}
			if guestEmail == nil {
				guestEmail = requestCtx.Session.GuestEmail
			}
			if guestEmail == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Guest email is required"})
				return
			}
			if normalized, err := normalizeGuestEmail(*guestEmail); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Guest email must be valid"})
				return
			} else {
				guestEmail = normalized
			}
		}

		items := orderQuantitiesFromCartItems(requestCtx.Cart.Items)
		idempotencyRequest := struct {
			GuestEmail *string                `json:"guest_email,omitempty"`
			Items      []orderVariantQuantity `json:"items"`
		}{
			GuestEmail: guestEmail,
			Items:      items,
		}
		replayedRecord, handled, err := replayCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			"checkout_order_create",
			idempotencyRequest,
		)
		if err != nil {
			correlationID := checkoutCorrelationID(c, "")
			log.Printf(
				"checkout_order_create result=failure correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q reason=%q",
				correlationID,
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
				err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process checkout request"})
			return
		}
		if handled {
			log.Printf(
				"checkout_order_create result=replay correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q",
				checkoutCorrelationID(c, func() string {
					if replayedRecord == nil {
						return ""
					}
					return replayedRecord.CorrelationID
				}()),
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
			)
			return
		}

		if !enforceCheckoutSubmissionRateLimit(c, requestCtx.Session, "create_order") {
			log.Printf(
				"checkout_order_create result=failure mode=%s session_id=%d user_id=%v guest_email=%q reason=%q",
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
				checkoutRateLimitedCode,
			)
			return
		}

		correlationID := checkoutCorrelationID(c, "")
		idempotencyRecord, handled, err := beginCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			"checkout_order_create",
			idempotencyRequest,
			correlationID,
		)
		if err != nil {
			log.Printf(
				"checkout_order_create result=failure correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q reason=%q",
				correlationID,
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
				err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process checkout request"})
			return
		}
		if handled {
			log.Printf(
				"checkout_order_create result=replay correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q",
				checkoutCorrelationID(c, func() string {
					if idempotencyRecord == nil {
						return ""
					}
					return idempotencyRecord.CorrelationID
				}()),
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
			)
			return
		}

		order, created, err := createOrUpdateCheckoutOrderRecord(
			db,
			requestCtx.Session,
			func() *uint {
				if requestCtx.User == nil {
					return nil
				}
				return &requestCtx.User.ID
			}(),
			guestEmail,
			items,
		)
		if err != nil {
			status, payload := createOrderErrorResponse(err)
			log.Printf(
				"checkout_order_create result=failure correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q reason=%q",
				correlationID,
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
				payload["error"],
			)
			writeCheckoutJSON(db, c, idempotencyRecord, status, payload)
			return
		}

		response, err := buildCreatedOrderResponse(db, mediaService, order, func() *uint {
			if requestCtx.User == nil {
				return nil
			}
			return &requestCtx.User.ID
		}())
		if err != nil {
			log.Printf(
				"checkout_order_create result=failure correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reason=%q",
				correlationID,
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(guestEmail),
				order.ID,
				err.Error(),
			)
			writeCheckoutJSON(db, c, idempotencyRecord, http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		log.Printf(
			"checkout_order_create result=success correlation_id=%s mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reused=%t",
			correlationID,
			checkoutMode(requestCtx.User),
			requestCtx.Session.ID,
			checkoutUserID(requestCtx.User),
			checkoutGuestEmail(guestEmail),
			order.ID,
			!created,
		)
		status := http.StatusCreated
		if !created {
			status = http.StatusOK
		}
		writeCheckoutJSON(db, c, idempotencyRecord, status, response)
	}
}
