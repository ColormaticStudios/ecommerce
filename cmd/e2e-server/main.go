package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/migrations"
	"ecommerce/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	// Test-only defaults for the local E2E harness.
	defaultE2EAPIPort = 3001
	defaultE2EDBPath  = "/tmp/ecommerce-e2e.sqlite"
	e2eJWTSecret      = "e2e-test-jwt-secret"
	testRoutePrefix   = "/__test"
)

type summaryResponse struct {
	Users        int64 `json:"users"`
	Orders       int64 `json:"orders"`
	PaidOrders   int64 `json:"paid_orders"`
	CartItems    int64 `json:"cart_items"`
	ProductStock int   `json:"product_stock"`
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func ensureSeedData(db *gorm.DB) error {
	product := models.Product{
		SKU:         "e2e-product-1",
		Name:        "E2E Running Shoes",
		Description: "Stable seeded product for end-to-end tests",
		Price:       models.MoneyFromFloat(89.50),
		Stock:       8,
	}

	var existing models.Product
	if err := db.Where("sku = ?", product.SKU).First(&existing).Error; err == nil {
		return nil
	}
	if err := db.Create(&product).Error; err != nil {
		return err
	}
	return nil
}

func buildSummary(db *gorm.DB) (summaryResponse, error) {
	var summary summaryResponse

	if err := db.Model(&models.User{}).Count(&summary.Users).Error; err != nil {
		return summary, err
	}
	if err := db.Model(&models.Order{}).Count(&summary.Orders).Error; err != nil {
		return summary, err
	}
	if err := db.Model(&models.Order{}).Where("status = ?", models.StatusPaid).Count(&summary.PaidOrders).Error; err != nil {
		return summary, err
	}
	if err := db.Model(&models.CartItem{}).Count(&summary.CartItems).Error; err != nil {
		return summary, err
	}

	var product models.Product
	if err := db.Where("sku = ?", "e2e-product-1").First(&product).Error; err != nil {
		return summary, err
	}
	summary.ProductStock = product.Stock

	return summary, nil
}

func main() {
	// This binary is test-only. It is intended for local/CI integration + E2E runs.
	// Do not run this server in production environments.
	gin.SetMode(gin.ReleaseMode)

	port := envInt("E2E_API_PORT", defaultE2EAPIPort)
	dbPath := os.Getenv("E2E_DB_PATH")
	if dbPath == "" {
		dbPath = defaultE2EDBPath
	}

	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to reset e2e db: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open e2e sqlite db: %v", err)
	}

	if err := migrations.Run(db); err != nil {
		log.Fatalf("failed to run e2e migrations: %v", err)
	}

	if err := ensureSeedData(db); err != nil {
		log.Fatalf("failed to seed e2e data: %v", err)
	}
	if err := handlers.ValidateStartupDefaults(); err != nil {
		log.Fatalf("failed startup defaults validation: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://127.0.0.1:4173",
			"http://localhost:4173",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length", "Location"},
		AllowCredentials: true,
	}))

	apiServer, err := handlers.NewGeneratedAPIServer(db, nil, handlers.GeneratedAPIServerConfig{
		JWTSecret: e2eJWTSecret,
		AuthCookieConfig: handlers.AuthCookieConfig{
			Secure:   false,
			Domain:   "",
			SameSite: http.SameSiteLaxMode,
		},
	})
	if err != nil {
		log.Fatalf("failed to initialize e2e api server: %v", err)
	}
	apicontract.RegisterHandlers(r, apiServer)

	// Test-only helper endpoints consumed by Playwright E2E tests.
	r.GET(testRoutePrefix+"/summary", func(c *gin.Context) {
		summary, err := buildSummary(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, summary)
	})
	r.GET(testRoutePrefix+"/login", func(c *gin.Context) {
		email := c.Query("email")
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email query parameter is required"})
			return
		}

		var user models.User
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   user.Subject,
			"email": user.Email,
			"role":  user.Role,
			"name":  user.Name,
			"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		})
		signedToken, err := token.SignedString([]byte(e2eJWTSecret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
			return
		}

		maxAge := int((7 * 24 * time.Hour).Seconds())
		c.SetCookie("session_token", signedToken, maxAge, "/", "", false, true)
		c.SetCookie("csrf_token", uuid.NewString(), maxAge, "/", "", false, false)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST(testRoutePrefix+"/cart-item", func(c *gin.Context) {
		var payload struct {
			Email     string `json:"email"`
			ProductID uint   `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if payload.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
			return
		}
		if payload.ProductID == 0 {
			payload.ProductID = 1
		}
		if payload.Quantity < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be >= 1"})
			return
		}

		var user models.User
		if err := db.Where("email = ?", payload.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		var product models.Product
		if err := db.First(&product, payload.ProductID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}

		tx := db.Begin()
		var cart models.Cart
		if err := tx.Where("user_id = ?", user.ID).First(&cart).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				cart = models.Cart{UserID: user.ID}
				if err := tx.Create(&cart).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cart"})
					return
				}
			} else {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load cart"})
				return
			}
		}

		var item models.CartItem
		if err := tx.Where("cart_id = ? AND product_id = ?", cart.ID, product.ID).First(&item).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				item = models.CartItem{
					CartID:    cart.ID,
					ProductID: product.ID,
					Quantity:  payload.Quantity,
				}
				if err := tx.Create(&item).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cart item"})
					return
				}
			} else {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load cart item"})
				return
			}
		} else {
			item.Quantity = payload.Quantity
			if err := tx.Save(&item).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update cart item"})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit cart update"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST(testRoutePrefix+"/saved-checkout-data", func(c *gin.Context) {
		var payload struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil || payload.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
			return
		}

		var user models.User
		if err := db.Where("email = ?", payload.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		tx := db.Begin()
		if err := tx.Model(&models.SavedPaymentMethod{}).Where("user_id = ?", user.ID).Update("is_default", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear payment defaults"})
			return
		}
		if err := tx.Model(&models.SavedAddress{}).Where("user_id = ?", user.ID).Update("is_default", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear address defaults"})
			return
		}

		payment := models.SavedPaymentMethod{
			UserID:         user.ID,
			Type:           "card",
			Brand:          "Visa",
			Last4:          "1111",
			ExpMonth:       12,
			ExpYear:        2040,
			CardholderName: "E2E User",
			Nickname:       "Primary card",
			IsDefault:      true,
		}
		if err := tx.Create(&payment).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment method"})
			return
		}

		address := models.SavedAddress{
			UserID:     user.ID,
			Label:      "Primary",
			FullName:   "E2E User",
			Line1:      "500 Test Ave",
			City:       "Austin",
			PostalCode: "78701",
			Country:    "US",
			IsDefault:  true,
		}
		if err := tx.Create(&address).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create address"})
			return
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit checkout seed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("e2e api server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to run e2e api server: %v", err)
	}
}
