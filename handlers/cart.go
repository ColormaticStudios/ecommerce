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
	ProductVariantID uint `json:"product_variant_id" binding:"required"`
	Quantity         int  `json:"quantity" binding:"required,min=1"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// getOrCreateCart gets the user's cart or creates one if it doesn't exist
func getOrCreateCart(db *gorm.DB, userID uint) (*models.Cart, error) {
	var cart models.Cart
	err := db.Where("user_id = ?", userID).
		Preload("Items.ProductVariant").
		Preload("Items.ProductVariant.Product").
		First(&cart).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new cart
		cart = models.Cart{UserID: userID}
		if err := db.Create(&cart).Error; err != nil {
			// Another request may have created the cart first.
			if lookupErr := db.Where("user_id = ?", userID).
				Preload("Items.ProductVariant").
				Preload("Items.ProductVariant.Product").
				First(&cart).Error; lookupErr == nil {
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
		productIDs = append(productIDs, cart.Items[i].ProductVariant.ProductID)
	}

	mediaByProduct, err := mediaService.ProductMediaURLsByProductIDs(productIDs)
	if err != nil {
		return
	}

	for i := range cart.Items {
		mediaURLs := mediaByProduct[cart.Items[i].ProductVariant.ProductID]
		if len(mediaURLs) > 0 {
			cart.Items[i].ProductVariant.Product.Images = mediaURLs
		}
	}
}

func loadPublicVariant(db *gorm.DB, variantID uint) (models.ProductVariant, error) {
	var variant models.ProductVariant
	if err := db.Preload("Product").First(&variant, variantID).Error; err != nil {
		return models.ProductVariant{}, err
	}
	if !variant.IsPublished || !productIsPubliclyVisible(variant.Product) {
		return models.ProductVariant{}, gorm.ErrRecordNotFound
	}
	return variant, nil
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

		// Verify variant exists and is publicly purchasable.
		variant, err := loadPublicVariant(db, req.ProductVariantID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product variant not found"})
			return
		}

		if variant.Stock < req.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":              "Insufficient stock",
				"product_variant_id": req.ProductVariantID,
				"product_name":       variant.Product.Name,
				"requested":          req.Quantity,
				"available":          variant.Stock,
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
		err = db.Where("cart_id = ? AND product_variant_id = ?", cart.ID, req.ProductVariantID).First(&existingItem).Error

		switch err {
		case nil:
			// Update quantity
			newQuantity := existingItem.Quantity + req.Quantity
			if variant.Stock < newQuantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":              "Insufficient stock for updated quantity",
					"product_variant_id": req.ProductVariantID,
					"requested":          newQuantity,
					"available":          variant.Stock,
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
				CartID:           cart.ID,
				ProductVariantID: req.ProductVariantID,
				Quantity:         req.Quantity,
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
		db.Preload("Items.ProductVariant").Preload("Items.ProductVariant.Product").First(cart, cart.ID)
		applyCartMedia(cart, mediaService)
		response, err := buildCartResponse(db, mediaService, *cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render cart"})
			return
		}
		c.JSON(http.StatusOK, response)
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
		response, err := buildCartResponse(db, mediaService, *cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render cart"})
			return
		}
		c.JSON(http.StatusOK, response)
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

		// Check variant stock.
		variant, err := loadPublicVariant(db, cartItem.ProductVariantID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product variant not found"})
			return
		}

		if variant.Stock < req.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":              "Insufficient stock",
				"product_variant_id": cartItem.ProductVariantID,
				"requested":          req.Quantity,
				"available":          variant.Stock,
			})
			return
		}

		// Update quantity
		cartItem.Quantity = req.Quantity
		if err := db.Save(&cartItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart item"})
			return
		}

		// Reload with variant and product
		db.Preload("ProductVariant").Preload("ProductVariant.Product").First(&cartItem, cartItem.ID)
		response, err := buildCartItemResponse(db, mediaService, cartItem)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render cart item"})
			return
		}
		c.JSON(http.StatusOK, response)
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
