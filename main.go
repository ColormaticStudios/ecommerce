package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ecommerce/config"
	"ecommerce/handlers"
	"ecommerce/internal/media"
	"ecommerce/middleware"
	"ecommerce/models"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	tusdfilestore "github.com/tus/tusd/v2/pkg/filestore"
	tusdhandler "github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	if err := media.CheckDependencies(); err != nil {
		log.Fatalf("[ERROR] Dependency check failed: %v", err)
	}

	// Connect to database
	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		log.Fatalf("[ERROR] Failed to connect to database: %v", err)
	}
	log.Println("[INFO] Database connection established")

	// Auto-migrate the schema
	if err := db.AutoMigrate(
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
		&models.Cart{},
		&models.CartItem{},
		&models.MediaObject{},
		&models.MediaVariant{},
		&models.MediaReference{},
		&models.SavedPaymentMethod{},
		&models.SavedAddress{},
	); err != nil {
		log.Fatalf("[ERROR] Failed to migrate database: %v", err)
	}
	log.Println("[INFO] Database migration completed")

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	if cfg.ServeMedia {
		r.Static("/media", cfg.MediaRoot)
	}

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
				"X-CSRF-Token",
				"Tus-Resumable",
				"Upload-Length",
				"Upload-Metadata",
				"Upload-Offset",
			},
			ExposeHeaders: []string{
				"Content-Length",
				"Location",
			},
			AllowCredentials: true,
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

	cookieSameSite := http.SameSiteLaxMode
	cookieSecure := false
	if !cfg.DevMode {
		cookieSameSite = http.SameSiteNoneMode
		cookieSecure = true
	}
	authCookieCfg := handlers.AuthCookieConfig{
		Secure:   cookieSecure,
		Domain:   "",
		SameSite: cookieSameSite,
	}

	mediaService := media.NewService(db, cfg.MediaRoot, cfg.MediaPublicURL, log.Default())
	if err := mediaService.EnsureDirs(); err != nil {
		log.Fatalf("[ERROR] Failed to initialize media directories: %v", err)
	}
	mediaService.StartProcessor()

	api := r.Group("/api")
	{
		apiv1 := api.Group("/v1")
		{
			// PUBLIC ROUTES
			if !disableLocalSignIn {
				// Only allow these routes if they are enabled
				apiv1.POST("/auth/register", handlers.Register(db, jwtSecret, authCookieCfg))
				apiv1.POST("/auth/login", handlers.Login(db, jwtSecret, authCookieCfg))
			}
			apiv1.POST("/auth/logout", handlers.Logout(authCookieCfg))

			apiv1.GET("/auth/oidc/login", handlers.OIDCLogin(cfg.OIDCProvider, cfg.OIDCClientID, cfg.OIDCRedirectURI, authCookieCfg))
			apiv1.GET("/auth/oidc/callback", handlers.OIDCCallback(db, cfg.JWTSecret, cfg.OIDCProvider, cfg.OIDCClientID, cfg.OIDCRedirectURI, authCookieCfg))

			apiv1.GET("/products", handlers.GetProducts(db, mediaService))
			apiv1.GET("/products/:id", handlers.GetProductByID(db, mediaService))

			mediaRoutes := apiv1.Group("/media")
			mediaRoutes.Use(middleware.AuthMiddleware(jwtSecret, ""))
			mediaRoutes.Use(middleware.CSRFMiddleware())
			{
				composer := tusdhandler.NewStoreComposer()
				store := tusdfilestore.New(mediaService.TusDir())
				store.UseIn(composer)

				tusd, err := tusdhandler.NewHandler(tusdhandler.Config{
					BasePath:              "/api/v1/media/uploads",
					StoreComposer:         composer,
					NotifyCompleteUploads: true,
				})
				if err != nil {
					log.Fatalf("[ERROR] Failed to initialize tusd: %v", err)
				}

				go func() {
					for event := range tusd.CompleteUploads {
						if err := mediaService.HandleTusdComplete(event.Upload); err != nil {
							log.Printf("[ERROR] Media upload completion failed: %v", err)
						}
					}
				}()

				mediaRoutes.Any("/uploads", gin.WrapH(http.StripPrefix("/api/v1/media/uploads", tusd)))
				mediaRoutes.Any("/uploads/*path", gin.WrapH(http.StripPrefix("/api/v1/media/uploads", tusd)))
			}

			// PROTECTED USER ROUTES
			userRoutes := apiv1.Group("/me")
			userRoutes.Use(middleware.AuthMiddleware(jwtSecret, ""))
			userRoutes.Use(middleware.CSRFMiddleware())
			{
				userRoutes.GET("/", handlers.GetProfile(db, mediaService))
				userRoutes.PATCH("/", handlers.UpdateProfile(db))
				userRoutes.POST("/profile-photo", handlers.SetProfilePhoto(db, mediaService))
				userRoutes.DELETE("/profile-photo", handlers.DeleteProfilePhoto(db, mediaService))
				userRoutes.GET("/cart", handlers.GetCart(db, mediaService))
				userRoutes.POST("/cart", handlers.AddCartItem(db, mediaService))
				userRoutes.PATCH("/cart/:itemId", handlers.UpdateCartItem(db, mediaService))
				userRoutes.DELETE("/cart/:itemId", handlers.DeleteCartItem(db))
				userRoutes.GET("/orders", handlers.GetUserOrders(db, mediaService))
				userRoutes.GET("/orders/:id", handlers.GetOrderByID(db, mediaService))
				userRoutes.POST("/orders", handlers.CreateOrder(db, mediaService))
				userRoutes.POST("/orders/:id/pay", handlers.ProcessPayment(db, mediaService))
				userRoutes.GET("/payment-methods", handlers.GetSavedPaymentMethods(db))
				userRoutes.POST("/payment-methods", handlers.CreateSavedPaymentMethod(db))
				userRoutes.DELETE("/payment-methods/:id", handlers.DeleteSavedPaymentMethod(db))
				userRoutes.PATCH("/payment-methods/:id/default", handlers.SetDefaultPaymentMethod(db))
				userRoutes.GET("/addresses", handlers.GetSavedAddresses(db))
				userRoutes.POST("/addresses", handlers.CreateSavedAddress(db))
				userRoutes.DELETE("/addresses/:id", handlers.DeleteSavedAddress(db))
				userRoutes.PATCH("/addresses/:id/default", handlers.SetDefaultAddress(db))
			}

			// ADMIN ROUTES
			adminRoutes := apiv1.Group("/admin")
			adminRoutes.Use(middleware.AuthMiddleware(jwtSecret, "admin"))
			adminRoutes.Use(middleware.CSRFMiddleware())
			{
				adminRoutes.POST("/products", handlers.CreateProduct(db))
				adminRoutes.PATCH("/products/:id", handlers.UpdateProduct(db))
				adminRoutes.DELETE("/products/:id", handlers.DeleteProduct(db, mediaService))
				adminRoutes.POST("/products/:id/media", handlers.AttachProductMedia(db, mediaService))
				adminRoutes.PATCH("/products/:id/media/order", handlers.UpdateProductMediaOrder(db, mediaService))
				adminRoutes.DELETE("/products/:id/media/:mediaId", handlers.DetachProductMedia(db, mediaService))
				adminRoutes.PATCH("/products/:id/related", handlers.UpdateProductRelated(db, mediaService))
				adminRoutes.GET("/orders", handlers.GetAllOrders(db, mediaService))
				adminRoutes.GET("/orders/:id", handlers.GetAdminOrderByID(db, mediaService))
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
