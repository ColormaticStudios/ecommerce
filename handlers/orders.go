package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateOrderRequest struct {
	Items []OrderItemRequest `json:"items" binding:"required"`
}

type OrderItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

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

// CreateOrder creates a new order for the authenticated user
func CreateOrder(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		// Get user subject from middleware
		subject := c.GetString("userID")
		if subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Find user by subject
		var user models.User
		if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Parse request
		var req CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order must contain at least one item"})
			return
		}

		// Calculate total and validate products
		var total float64
		var orderItems []models.OrderItem

		for _, itemReq := range req.Items {
			// Get product
			var product models.Product
			if err := db.First(&product, itemReq.ProductID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found", "product_id": itemReq.ProductID})
				return
			}

			// Check stock availability
			if product.Stock < itemReq.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   itemReq.ProductID,
					"product_name": product.Name,
					"requested":    itemReq.Quantity,
					"available":    product.Stock,
				})
				return
			}

			// Calculate item total
			itemTotal := product.Price * float64(itemReq.Quantity)
			total += itemTotal

			// Create order item
			orderItem := models.OrderItem{
				ProductID: itemReq.ProductID,
				Quantity:  itemReq.Quantity,
				Price:     product.Price, // Snapshot price at time of order
			}
			orderItems = append(orderItems, orderItem)
		}

		// Create order
		order := models.Order{
			UserID: user.ID,
			Total:  total,
			Status: models.StatusPending,
			Items:  orderItems,
		}

		if err := db.Create(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}

		// Preload related data for response
		db.Preload("Items.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)

		c.JSON(http.StatusCreated, order)
	}
}

// GetUserOrders retrieves all orders for the authenticated user
func GetUserOrders(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		// Get user subject from middleware
		subject := c.GetString("userID")
		if subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Find user by subject
		var user models.User
		if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		status := strings.ToUpper(c.Query("status"))
		validStatuses := map[string]bool{
			models.StatusPending: true,
			models.StatusPaid:    true,
			models.StatusFailed:  true,
		}
		if status != "" && !validStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
			return
		}

		startDateStr := c.Query("start_date")
		endDateStr := c.Query("end_date")
		var startDate, endDate time.Time
		var err error

		if startDateStr != "" {
			startDate, err = time.Parse("2006-01-02", startDateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date, expected YYYY-MM-DD"})
				return
			}
		}
		if endDateStr != "" {
			endDate, err = time.Parse("2006-01-02", endDateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date, expected YYYY-MM-DD"})
				return
			}
			// Make end date inclusive.
			endDate = endDate.Add(24*time.Hour - time.Nanosecond)
		}
		if !startDate.IsZero() && !endDate.IsZero() && endDate.Before(startDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be on or after start_date"})
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		offset := (page - 1) * limit

		query := db.Model(&models.Order{}).Where("user_id = ?", user.ID)
		if status != "" {
			query = query.Where("status = ?", status)
		}
		if !startDate.IsZero() {
			query = query.Where("created_at >= ?", startDate)
		}
		if !endDate.IsZero() {
			query = query.Where("created_at <= ?", endDate)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		var orders []models.Order
		if err := query.
			Preload("Items.Product").
			Order("created_at DESC").
			Offset(offset).
			Limit(limit).
			Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}
		applyOrderMedia(orders, mediaService)

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}

		c.JSON(http.StatusOK, gin.H{
			"data": orders,
			"pagination": gin.H{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": totalPages,
			},
		})
	}
}

// GetOrderByID retrieves a specific order by ID (only if it belongs to the user)
func GetOrderByID(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		// Get user subject from middleware
		subject := c.GetString("userID")
		if subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Find user by subject
		var user models.User
		if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order (only if it belongs to the user)
		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).
			Preload("Items.Product").
			First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		applyOrderMediaToOrder(&order, mediaService)

		c.JSON(http.StatusOK, order)
	}
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=PENDING PAID FAILED"`
}

type insufficientStockError struct {
	ProductID   uint
	ProductName string
	Requested   int
	Available   int
}

func (e *insufficientStockError) Error() string {
	return "insufficient stock"
}

func deductStockForItems(tx *gorm.DB, items []models.OrderItem) error {
	for _, item := range items {
		var product models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, item.ProductID).Error; err != nil {
			return err
		}

		if product.Stock < item.Quantity {
			return &insufficientStockError{
				ProductID:   item.ProductID,
				ProductName: product.Name,
				Requested:   item.Quantity,
				Available:   product.Stock,
			}
		}

		if err := tx.Model(&models.Product{}).
			Where("id = ? AND stock >= ?", item.ProductID, item.Quantity).
			Update("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
			return err
		}
	}
	return nil
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
	PaymentMethodID *uint                       `json:"payment_method_id"`
	AddressID       *uint                       `json:"address_id"`
	PaymentMethod   *ProcessPaymentInputMethod  `json:"payment_method"`
	Address         *ProcessPaymentInputAddress `json:"address"`
}

// ProcessPayment processes payment for an order (mock implementation)
func ProcessPayment(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req ProcessPaymentRequest
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order (only if it belongs to the user)
		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).
			Preload("Items.Product").
			First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		// Check if order is already paid
		if order.Status == models.StatusPaid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order is already paid"})
			return
		}

		paymentDisplay, err := resolvePaymentDisplayForOrder(db, user.ID, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		shippingAddress, err := resolveShippingAddressForOrder(db, user.ID, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Mock payment processing - in a real app, this would call a payment provider
		// For now, we'll simulate a successful payment
		// In production, you'd validate payment details, process with Stripe/PayPal/etc.

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Update order status
		order.Status = models.StatusPaid
		order.PaymentMethodDisplay = paymentDisplay
		order.ShippingAddressPretty = shippingAddress
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		if err := deductStockForItems(tx, order.Items); err != nil {
			tx.Rollback()
			var stockErr *insufficientStockError
			if errors.As(err, &stockErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   stockErr.ProductID,
					"product_name": stockErr.ProductName,
					"requested":    stockErr.Requested,
					"available":    stockErr.Available,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
			return
		}

		// Clear user's cart after successful payment
		var cart models.Cart
		if err := tx.Where("user_id = ?", user.ID).First(&cart).Error; err == nil {
			if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cart"})
				return
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		// Reload order
		db.Preload("Items.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)

		c.JSON(http.StatusOK, gin.H{
			"message": "Payment processed successfully",
			"order":   order,
		})
	}
}

func resolvePaymentDisplayForOrder(db *gorm.DB, userID uint, req ProcessPaymentRequest) (string, error) {
	if req.PaymentMethodID != nil {
		var method models.SavedPaymentMethod
		if err := db.Where("id = ? AND user_id = ?", *req.PaymentMethodID, userID).First(&method).Error; err != nil {
			return "", fmt.Errorf("payment method not found")
		}
		return paymentMethodDisplay(method.Brand, method.Last4), nil
	}

	if req.PaymentMethod != nil {
		cardDigits := digitsOnly(req.PaymentMethod.CardNumber)
		if len(cardDigits) < 12 || len(cardDigits) > 19 {
			return "", fmt.Errorf("card number must be 12 to 19 digits")
		}
		brand := detectCardBrand(cardDigits)
		last4 := cardDigits[len(cardDigits)-4:]
		return paymentMethodDisplay(brand, last4), nil
	}

	var method models.SavedPaymentMethod
	if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&method).Error; err == nil {
		return paymentMethodDisplay(method.Brand, method.Last4), nil
	}

	return "", fmt.Errorf("payment method is required")
}

func resolveShippingAddressForOrder(db *gorm.DB, userID uint, req ProcessPaymentRequest) (string, error) {
	if req.AddressID != nil {
		var address models.SavedAddress
		if err := db.Where("id = ? AND user_id = ?", *req.AddressID, userID).First(&address).Error; err != nil {
			return "", fmt.Errorf("address not found")
		}
		return addressPretty(address), nil
	}

	if req.Address != nil {
		country := strings.ToUpper(strings.TrimSpace(req.Address.Country))
		if len(country) != 2 {
			return "", fmt.Errorf("country must be a 2-letter code")
		}
		address := models.SavedAddress{
			FullName:   strings.TrimSpace(req.Address.FullName),
			Line1:      strings.TrimSpace(req.Address.Line1),
			Line2:      strings.TrimSpace(req.Address.Line2),
			City:       strings.TrimSpace(req.Address.City),
			State:      strings.TrimSpace(req.Address.State),
			PostalCode: strings.TrimSpace(req.Address.PostalCode),
			Country:    country,
		}
		return addressPretty(address), nil
	}

	var address models.SavedAddress
	if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error; err == nil {
		return addressPretty(address), nil
	}

	return "", fmt.Errorf("shipping address is required")
}

// UpdateOrderStatus updates the status of an order (for webhooks or admin)
func UpdateOrderStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Parse request
		var req UpdateOrderStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get order
		var order models.Order
		if err := db.First(&order, orderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		previousStatus := order.Status

		// Update status
		order.Status = req.Status
		if err := db.Save(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		// If status transitioned to PAID, deduct stock
		if req.Status == models.StatusPaid && previousStatus != models.StatusPaid {
			tx := db.Begin()
			defer func() {
				if r := recover(); r != nil {
					tx.Rollback()
				}
			}()

			// Load order items
			if err := tx.Preload("Items").First(&order, orderID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load order items"})
				return
			}

			if err := deductStockForItems(tx, order.Items); err != nil {
				tx.Rollback()
				var stockErr *insufficientStockError
				if errors.As(err, &stockErr) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":        "Insufficient stock",
						"product_id":   stockErr.ProductID,
						"product_name": stockErr.ProductName,
						"requested":    stockErr.Requested,
						"available":    stockErr.Available,
					})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
				return
			}

			if err := tx.Commit().Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process stock update"})
				return
			}
		}

		// Reload order
		db.Preload("Items.Product").First(&order, order.ID)

		c.JSON(http.StatusOK, order)
	}
}

// GetAllOrders retrieves all orders (admin only)
func GetAllOrders(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		offset := (page - 1) * limit

		query := db.Model(&models.Order{})
		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		var orders []models.Order
		if err := query.
			Preload("Items.Product").
			Preload("User").
			Order("created_at DESC").
			Offset(offset).
			Limit(limit).
			Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}
		applyOrderMedia(orders, mediaService)

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}

		c.JSON(http.StatusOK, gin.H{
			"data": orders,
			"pagination": gin.H{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": totalPages,
			},
		})
	}
}

// GetAdminOrderByID retrieves any order by ID (admin only)
func GetAdminOrderByID(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order
		var order models.Order
		if err := db.Preload("Items.Product").
			Preload("User").
			First(&order, orderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		applyOrderMediaToOrder(&order, mediaService)

		c.JSON(http.StatusOK, order)
	}
}
