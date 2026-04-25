package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"ecommerce/config"
	"ecommerce/handlers"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/httpcors"
	"ecommerce/internal/media"
	"ecommerce/internal/migrations"
	"ecommerce/internal/providerplugins"
	checkoutservice "ecommerce/internal/services/checkout"
	inventoryservice "ecommerce/internal/services/inventory"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"

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
	if err := handlers.ValidateStartupDefaults(); err != nil {
		log.Fatalf("[ERROR] Startup defaults validation failed: %v", err)
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

	if err := migrations.EnsureReady(db, cfg.AutoApplyMigrations); err != nil {
		log.Fatalf("[ERROR] Database migration readiness check failed: %v", err)
	}
	if cfg.AutoApplyMigrations {
		log.Printf("[INFO] Database migration completed (latest=%s)", migrations.LatestVersion())
	} else {
		log.Printf("[INFO] Database migration check completed (latest=%s)", migrations.LatestVersion())
	}

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
			AllowMethods:     httpcors.AllowMethods(),
			AllowHeaders:     httpcors.AllowHeaders(),
			ExposeHeaders:    httpcors.ExposeHeaders(),
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	} else {
		config := cors.DefaultConfig()
		config.AllowOrigins = []string{cfg.PublicURL}
		config.AllowMethods = httpcors.AllowMethods()
		config.AllowHeaders = httpcors.AllowHeaders()
		config.ExposeHeaders = httpcors.ExposeHeaders()
		config.AllowCredentials = true
		r.Use(cors.New(config))
	}

	// Global rate limit (100 requests/second)
	lmt := tollbooth.NewLimiter(100, nil)
	r.Use(tollbooth_gin.LimitHandler(lmt))

	// Pass the secret key from your .env file
	jwtSecret := cfg.JWTSecret

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

	pluginManager := checkoutplugins.NewDefaultManager()
	if cfg.CheckoutPluginManifestsDir != "" {
		loaded, loadErr := pluginManager.LoadExternalPluginsFromDir(cfg.CheckoutPluginManifestsDir)
		if loadErr != nil {
			log.Fatalf("[ERROR] Failed to load checkout plugins: %v", loadErr)
		}
		log.Printf("[INFO] Loaded %d external checkout plugins from %s", loaded, cfg.CheckoutPluginManifestsDir)
	}

	var paymentProviders paymentservice.ProviderRegistry = paymentservice.NewDefaultProviderRegistry()
	var shippingProviders shippingservice.ProviderRegistry = shippingservice.NewDefaultProviderRegistry()
	var taxProviders taxservice.ProviderRegistry = taxservice.NewDefaultProviderRegistry()

	if cfg.ProviderPluginManifestsDir != "" {
		if cfg.ProviderPluginManifestsDir != cfg.CheckoutPluginManifestsDir {
			loaded, loadErr := pluginManager.LoadExternalPluginsFromDir(cfg.ProviderPluginManifestsDir)
			if loadErr != nil {
				log.Fatalf("[ERROR] Failed to load provider-backed checkout plugins: %v", loadErr)
			}
			log.Printf("[INFO] Loaded %d provider-backed checkout plugins from %s", loaded, cfg.ProviderPluginManifestsDir)
		}

		loadedProviders, loadErr := providerplugins.LoadRegistriesFromDir(
			cfg.ProviderPluginManifestsDir,
			paymentProviders,
			shippingProviders,
			taxProviders,
		)
		if loadErr != nil {
			log.Fatalf("[ERROR] Failed to load provider plugins: %v", loadErr)
		}
		paymentProviders = loadedProviders.PaymentProviders
		shippingProviders = loadedProviders.ShippingProviders
		taxProviders = loadedProviders.TaxProviders
		log.Printf("[INFO] Loaded %d external provider plugins from %s", loadedProviders.LoadedCount, cfg.ProviderPluginManifestsDir)
	}

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		for {
			summary, cleanupErr := checkoutservice.CleanupExpiredState(db, time.Now().UTC())
			if cleanupErr != nil {
				log.Printf("[ERROR] Checkout cleanup failed: %v", cleanupErr)
			} else if summary.ExpiredSessions > 0 || summary.DeletedIdempotencyKeys > 0 {
				log.Printf(
					"[INFO] Checkout cleanup expired_sessions=%d deleted_idempotency_keys=%d",
					summary.ExpiredSessions,
					summary.DeletedIdempotencyKeys,
				)
			}

			<-ticker.C
		}
	}()
	inventoryservice.StartReservationExpiryWorker(context.Background(), db, time.Minute, log.Default())

	keyring, err := providerops.ParseKeyringConfig(cfg.ProviderCredentialsKeys)
	if err != nil {
		log.Fatalf("[ERROR] Failed to parse provider credential keys: %v", err)
	}
	credentialService, err := providerops.NewCredentialService(keyring, cfg.ProviderCredentialsKeyVersion)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize provider credential service: %v", err)
	}
	providerRuntime := providerops.NewRuntime(db, providerops.RuntimeConfig{
		Environment:       cfg.ProviderRuntimeEnvironment,
		Credentials:       credentialService,
		PaymentProviders:  paymentProviders,
		ShippingProviders: shippingProviders,
		TaxProviders:      taxProviders,
	})

	if intervalText := cfg.ProviderReconciliationInterval; intervalText != "" {
		interval, parseErr := time.ParseDuration(intervalText)
		if parseErr != nil {
			log.Fatalf("[ERROR] Failed to parse provider reconciliation interval: %v", parseErr)
		}
		if interval > 0 {
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for range ticker.C {
					summary, runErr := providerRuntime.Reconciliation.RunScheduled(context.Background())
					if runErr != nil {
						log.Printf("[ERROR] Provider reconciliation failed: %v", runErr)
						continue
					}
					if summary.RunCount > 0 {
						log.Printf("[INFO] Provider reconciliation completed runs=%d", summary.RunCount)
					}
				}
			}()
		}
	}

	apiServer, err := handlers.NewGeneratedAPIServer(db, mediaService, handlers.GeneratedAPIServerConfig{
		JWTSecret:          jwtSecret,
		DisableLocalSignIn: cfg.DisableLocalSignIn,
		AuthCookieConfig:   authCookieCfg,
		OIDCProvider:       cfg.OIDCProvider,
		OIDCClientID:       cfg.OIDCClientID,
		OIDCRedirectURI:    cfg.OIDCRedirectURI,
		MediaUploads:       http.StripPrefix("/api/v1/media/uploads", tusd),
		CheckoutPlugins:    pluginManager,
		ProviderRuntime:    providerRuntime,
	})
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize API server: %v", err)
	}
	apicontract.RegisterHandlers(r, apiServer)

	log.Printf("[INFO] Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[ERROR] Failed to start server: %v", err)
	}
}
