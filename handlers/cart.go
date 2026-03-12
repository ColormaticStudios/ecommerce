package handlers

import (
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

type cartSummaryResponse struct {
	ItemCount int `json:"item_count"`
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

// AddCartItem adds an item to the checkout session cart.
func AddCartItem(db *gorm.DB, mediaService *media.Service, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
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

		// Check if item already exists in cart.
		var existingItem models.CartItem
		err = db.Where("cart_id = ? AND product_variant_id = ?", requestCtx.Cart.ID, req.ProductVariantID).First(&existingItem).Error

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
				CartID:           requestCtx.Cart.ID,
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
		db.Preload("CheckoutSession").
			Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(requestCtx.Cart, requestCtx.Cart.ID)
		applyCartMedia(requestCtx.Cart, mediaService)
		response, err := buildCartResponse(db, mediaService, *requestCtx.Cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render cart"})
			return
		}
		c.JSON(http.StatusOK, response)
	}
}

// GetCart retrieves the checkout session cart.
func GetCart(db *gorm.DB, mediaService *media.Service, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		applyCartMedia(requestCtx.Cart, mediaService)
		response, err := buildCartResponse(db, mediaService, *requestCtx.Cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render cart"})
			return
		}
		c.JSON(http.StatusOK, response)
	}
}

// GetCartSummary retrieves the current checkout cart item count without creating checkout state.
func GetCartSummary(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerCtx, ok := resolveExistingCheckoutSessionOwnerContext(
			db,
			c,
			jwtSecret,
			cookieCfg,
			checkoutSessionResolveOptions{},
		)
		if !ok {
			return
		}

		if ownerCtx.Session == nil {
			c.JSON(http.StatusOK, cartSummaryResponse{ItemCount: 0})
			return
		}

		var itemCount int64
		if err := db.Model(&models.CartItem{}).
			Joins("JOIN carts ON carts.id = cart_items.cart_id").
			Where("carts.checkout_session_id = ?", ownerCtx.Session.ID).
			Count(&itemCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart summary"})
			return
		}

		c.JSON(http.StatusOK, cartSummaryResponse{ItemCount: int(itemCount)})
	}
}

// UpdateCartItem updates the quantity of a cart item.
func UpdateCartItem(db *gorm.DB, mediaService *media.Service, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
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
			Where("cart_items.id = ? AND carts.checkout_session_id = ?", itemID, requestCtx.Session.ID).
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

// DeleteCartItem removes an item from the cart.
func DeleteCartItem(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
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
			Where("cart_items.id = ? AND carts.checkout_session_id = ?", itemID, requestCtx.Session.ID).
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
