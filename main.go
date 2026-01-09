package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"ecommerce/config"
	"ecommerce/handlers"
	"ecommerce/middleware"
	"ecommerce/models"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Setup logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("[INFO] Starting ecommerce API server...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}
	log.Println("[INFO] Configuration loaded successfully")

	// Connect to database
	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("[ERROR] Failed to connect to database: %v", err)
	}
	log.Println("[INFO] Database connection established")

	// Auto-migrate the schema
	if err := db.AutoMigrate(&models.User{}, &models.Product{}, &models.Order{}, &models.OrderItem{}, &models.Cart{}, &models.CartItem{}); err != nil {
		log.Fatalf("[ERROR] Failed to migrate database: %v", err)
	}
	log.Println("[INFO] Database migration completed")

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// Request logging middleware (custom format)
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s \"%s\" %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.ClientIP,
			param.Method,
			param.StatusCode,
			param.Latency,
			param.Path,
			param.ErrorMessage,
		)
	}))

	// Error recovery middleware
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("[ERROR] Panic recovered: %v", recovered)
		c.JSON(500, gin.H{"error": "Internal server error"})
	}))

	r.SetTrustedProxies(nil)

	// CORS configuration
	if cfg.DevMode {
		r.Use(cors.New(cors.Config{
			AllowOrigins: []string{
				"http://localhost:5173", // SvelteKit/Vite dev
				"http://127.0.0.1:5173",
			},
			AllowMethods: []string{
				"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
			},
			AllowHeaders: []string{
				"Origin",
				"Content-Type",
				"Authorization",
			},
			ExposeHeaders: []string{
				"Content-Length",
			},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}))
	} else {
		config := cors.DefaultConfig()
		config.AllowOrigins = []string{cfg.PublicURL}
		config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
		config.AllowHeaders = []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization", "X-CSRF-Token"}
		config.ExposeHeaders = []string{"Content-Length"}
		config.AllowCredentials = true
		r.Use(cors.New(config))
	}

	// Global rate limit (100 requests/second)
	lmt := tollbooth.NewLimiter(100, nil)
	r.Use(tollbooth_gin.LimitHandler(lmt))

	// Pass the secret key from your .env file
	jwtSecret := cfg.JWTSecret

	disableLocalSignIn, err := strconv.ParseBool(cfg.DisableLocalSignIn)
	if err != nil {
		log.Fatalf("Failed to parse variable DISABLE_LOCAL_SIGN_IN: %v", err)
	}

	api := r.Group("/api")
	{
		apiv1 := api.Group("/v1")
		{
			// PUBLIC ROUTES
			if !disableLocalSignIn {
				// Only allow these routes if they are enabled
				apiv1.POST("/auth/register", handlers.Register(db, jwtSecret))
				apiv1.POST("/auth/login", handlers.Login(db, jwtSecret))
			}

			apiv1.GET("/auth/oidc/login", handlers.OIDCLogin(cfg.OIDCProvider, cfg.OIDCClientID, cfg.OIDCRedirectURI))
			apiv1.GET("/auth/oidc/callback", handlers.OIDCCallback(db, cfg.JWTSecret, cfg.OIDCProvider, cfg.OIDCClientID))

			apiv1.GET("/products", handlers.GetProducts(db))
			apiv1.GET("/products/:id", handlers.GetProductByID(db))

			// PROTECTED USER ROUTES
			userRoutes := apiv1.Group("/me")
			userRoutes.Use(middleware.AuthMiddleware(jwtSecret, ""))
			{
				userRoutes.GET("/", handlers.GetProfile(db))
				userRoutes.PATCH("/", handlers.UpdateProfile(db))
				userRoutes.GET("/cart", handlers.GetCart(db))
				userRoutes.POST("/cart", handlers.AddCartItem(db))
				userRoutes.PATCH("/cart/:itemId", handlers.UpdateCartItem(db))
				userRoutes.DELETE("/cart/:itemId", handlers.DeleteCartItem(db))
				userRoutes.GET("/orders", handlers.GetUserOrders(db))
				userRoutes.GET("/orders/:id", handlers.GetOrderByID(db))
				userRoutes.POST("/orders", handlers.CreateOrder(db))
				userRoutes.POST("/orders/:id/pay", handlers.ProcessPayment(db))
			}

			// ADMIN ROUTES
			adminRoutes := apiv1.Group("/admin")
			adminRoutes.Use(middleware.AuthMiddleware(jwtSecret, "admin"))
			{
				adminRoutes.POST("/products", handlers.CreateProduct(db))
				adminRoutes.PATCH("/products/:id", handlers.UpdateProduct(db))
				adminRoutes.DELETE("/products/:id", handlers.DeleteProduct(db))
				adminRoutes.GET("/orders", handlers.GetAllOrders(db))
				adminRoutes.GET("/orders/:id", handlers.GetAdminOrderByID(db))
				adminRoutes.PATCH("/orders/:id/status", handlers.UpdateOrderStatus(db))
				adminRoutes.GET("/users", handlers.GetAllUsers(db))
				adminRoutes.PATCH("/users/:id/role", handlers.UpdateUserRole(db))
			}
		}
	}

	log.Printf("[INFO] Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[ERROR] Failed to start server: %v", err)
	}
}
