package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errInvalidStatusFilter = errors.New("invalid status filter")
	errInvalidStartDate    = errors.New("invalid start date")
	errInvalidEndDate      = errors.New("invalid end date")
	errInvalidDateRange    = errors.New("invalid date range")
)

func parseUserOrderFilters(c *gin.Context) (userOrderFilters, error) {
	filters := userOrderFilters{}
	status := strings.ToUpper(c.Query("status"))
	if status != "" && !models.IsValidOrderStatus(status) {
		return filters, errInvalidStatusFilter
	}
	filters.status = status

	var err error
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		filters.startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return filters, fmt.Errorf("%w: expected YYYY-MM-DD", errInvalidStartDate)
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		filters.endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return filters, fmt.Errorf("%w: expected YYYY-MM-DD", errInvalidEndDate)
		}
		filters.endDate = filters.endDate.Add(24*time.Hour - time.Nanosecond)
	}

	if !filters.startDate.IsZero() && !filters.endDate.IsZero() && filters.endDate.Before(filters.startDate) {
		return filters, errInvalidDateRange
	}

	return filters, nil
}

// GetUserOrders retrieves all orders for the authenticated user.
func GetUserOrders(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		filters, err := parseUserOrderFilters(c)
		if err != nil {
			switch {
			case errors.Is(err, errInvalidStatusFilter):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
			case errors.Is(err, errInvalidStartDate):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date, expected YYYY-MM-DD"})
			case errors.Is(err, errInvalidEndDate):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date, expected YYYY-MM-DD"})
			case errors.Is(err, errInvalidDateRange):
				c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be on or after start_date"})
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order filters"})
			}
			return
		}

		page, limit, offset := parsePagination(c, 20)
		query := db.Model(&models.Order{}).Where("user_id = ?", user.ID)
		if filters.status != "" {
			query = query.Where("status = ?", filters.status)
		}
		if !filters.startDate.IsZero() {
			query = query.Where("created_at >= ?", filters.startDate)
		}
		if !filters.endDate.IsZero() {
			query = query.Where("created_at <= ?", filters.endDate)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		var orders []models.Order
		if err := query.Preload("Items.Product").Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}
		applyOrderMedia(orders, mediaService)
		applyOrderCapabilitiesToList(orders, &user.ID)

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

// GetOrderByID retrieves a specific order by ID (only if it belongs to the user).
func GetOrderByID(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).Preload("Items.Product").First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, &user.ID)

		c.JSON(http.StatusOK, order)
	}
}

// GetAllOrders retrieves all orders (admin only).
func GetAllOrders(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		page, limit, offset := parsePagination(c, 10)
		searchTerm := strings.TrimSpace(c.Query("q"))

		query := db.Model(&models.Order{}).Joins("LEFT JOIN users ON users.id = orders.user_id")
		if searchTerm != "" {
			like := "%" + strings.ToLower(searchTerm) + "%"
			query = query.Where(
				`CAST(orders.id AS TEXT) LIKE ? OR
				 CAST(orders.user_id AS TEXT) LIKE ? OR
				 LOWER(orders.status) LIKE ? OR
				 LOWER(COALESCE(orders.payment_method_display, '')) LIKE ? OR
				 LOWER(COALESCE(orders.shipping_address_pretty, '')) LIKE ? OR
				 LOWER(COALESCE(users.username, '')) LIKE ? OR
				 LOWER(COALESCE(users.name, '')) LIKE ? OR
				 LOWER(COALESCE(users.email, '')) LIKE ? OR
				 EXISTS (
				 	SELECT 1
				 	FROM order_items
				 	JOIN products ON products.id = order_items.product_id
				 	WHERE order_items.order_id = orders.id
				 	  AND (
				 		LOWER(COALESCE(products.name, '')) LIKE ? OR
				 		LOWER(COALESCE(products.sku, '')) LIKE ?
				 	  )
				 )`,
				like, like, like, like, like, like, like, like, like, like,
			)
		}
		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		var orders []models.Order
		if err := query.Preload("Items.Product").Preload("User").Order("orders.created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}
		applyOrderMedia(orders, mediaService)
		applyOrderCapabilitiesToList(orders, nil)

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

// GetAdminOrderByID retrieves any order by ID (admin only).
func GetAdminOrderByID(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var order models.Order
		if err := db.Preload("Items.Product").Preload("User").First(&order, orderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, nil)

		c.JSON(http.StatusOK, order)
	}
}
