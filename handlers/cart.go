package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AddCartItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// getOrCreateCart gets the user's cart or creates one if it doesn't exist
func getOrCreateCart(db *gorm.DB, userID uint) (*models.Cart, error) {
	var cart models.Cart
	err := db.Where("user_id = ?", userID).Preload("Items.Product").First(&cart).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new cart
		cart = models.Cart{UserID: userID}
		if err := db.Create(&cart).Error; err != nil {
			// Another request may have created the cart first.
			if lookupErr := db.Where("user_id = ?", userID).Preload("Items.Product").First(&cart).Error; lookupErr == nil {
				return &cart, nil
			}
			return nil, err
		}
		return &cart, nil
	}

	if err != nil {
		return nil, err
	}

	return &cart, nil
}

func applyCartMedia(cart *models.Cart, mediaService *media.Service) {
	if mediaService == nil {
		return
	}
	productIDs := make([]uint, 0, len(cart.Items))
	for i := range cart.Items {
		productIDs = append(productIDs, cart.Items[i].ProductID)
	}

	mediaByProduct, err := mediaService.ProductMediaURLsByProductIDs(productIDs)
	if err != nil {
		return
	}

	for i := range cart.Items {
		mediaURLs := mediaByProduct[cart.Items[i].ProductID]
		if len(mediaURLs) > 0 {
			cart.Items[i].Product.Images = mediaURLs
		}
	}
}

// AddCartItem adds an item to the user's cart
func AddCartItem(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		// Parse request
		var req AddCartItemRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify product exists
		var product models.Product
		if err := db.First(&product, req.ProductID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if !productIsPubliclyVisible(product) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		// Check stock availability
		if product.Stock < req.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":        "Insufficient stock",
				"product_id":   req.ProductID,
				"product_name": product.Name,
				"requested":    req.Quantity,
				"available":    product.Stock,
			})
			return
		}

		// Get or create cart
		cart, err := getOrCreateCart(db, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cart"})
			return
		}

		// Check if item already exists in cart
		var existingItem models.CartItem
		err = db.Where("cart_id = ? AND product_id = ?", cart.ID, req.ProductID).First(&existingItem).Error

		switch err {
		case nil:
			// Update quantity
			newQuantity := existingItem.Quantity + req.Quantity
			if product.Stock < newQuantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Insufficient stock for updated quantity",
					"product_id": req.ProductID,
					"requested":  newQuantity,
					"available":  product.Stock,
				})
				return
			}
			existingItem.Quantity = newQuantity
			if err := db.Save(&existingItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart item"})
				return
			}
		case gorm.ErrRecordNotFound:
			// Create new cart item
			cartItem := models.CartItem{
				CartID:    cart.ID,
				ProductID: req.ProductID,
				Quantity:  req.Quantity,
			}
			if err := db.Create(&cartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item to cart"})
				return
			}
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check cart"})
			return
		}

		// Reload cart with items
		db.Preload("Items.Product").First(cart, cart.ID)
		applyCartMedia(cart, mediaService)
		c.JSON(http.StatusOK, cart)
	}
}

// GetCart retrieves the user's cart
func GetCart(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		// Get or create cart
		cart, err := getOrCreateCart(db, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cart"})
			return
		}

		applyCartMedia(cart, mediaService)
		c.JSON(http.StatusOK, cart)
	}
}

// UpdateCartItem updates the quantity of a cart item
func UpdateCartItem(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		// Get cart item ID
		itemID, err := strconv.ParseUint(c.Param("itemId"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cart item ID"})
			return
		}

		// Parse request
		var req UpdateCartItemRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get cart item and verify it belongs to user's cart
		var cartItem models.CartItem
		if err := db.Joins("JOIN carts ON cart_items.cart_id = carts.id").
			Where("cart_items.id = ? AND carts.user_id = ?", itemID, user.ID).
			First(&cartItem).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cart item not found"})
			return
		}

		// Check product stock
		var product models.Product
		if err := db.First(&product, cartItem.ProductID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if !productIsPubliclyVisible(product) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if product.Stock < req.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Insufficient stock",
				"product_id": cartItem.ProductID,
				"requested":  req.Quantity,
				"available":  product.Stock,
			})
			return
		}

		// Update quantity
		cartItem.Quantity = req.Quantity
		if err := db.Save(&cartItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart item"})
			return
		}

		// Reload with product
		db.Preload("Product").First(&cartItem, cartItem.ID)
		if mediaService != nil {
			mediaURLs, err := mediaService.ProductMediaURLs(cartItem.ProductID)
			if err == nil && len(mediaURLs) > 0 {
				cartItem.Product.Images = mediaURLs
			}
		}
		c.JSON(http.StatusOK, cartItem)
	}
}

// DeleteCartItem removes an item from the cart
func DeleteCartItem(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		// Get cart item ID
		itemID, err := strconv.ParseUint(c.Param("itemId"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cart item ID"})
			return
		}

		// Get cart item and verify it belongs to user's cart
		var cartItem models.CartItem
		if err := db.Joins("JOIN carts ON cart_items.cart_id = carts.id").
			Where("cart_items.id = ? AND carts.user_id = ?", itemID, user.ID).
			First(&cartItem).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cart item not found"})
			return
		}

		// Delete cart item
		if err := db.Delete(&cartItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete cart item"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Cart item deleted successfully"})
	}
}
