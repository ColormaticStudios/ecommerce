package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ecommerce/config"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/httpcors"
	"ecommerce/internal/media"
	"ecommerce/internal/migrations"
	"ecommerce/internal/providerplugins"
	accountservice "ecommerce/internal/services/account"
	"ecommerce/internal/services/accountdata"
	authservice "ecommerce/internal/services/auth"
	checkoutservice "ecommerce/internal/services/checkout"
	cmsservice "ecommerce/internal/services/cms"
	inventoryservice "ecommerce/internal/services/inventory"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(rootCtx); err != nil {
		log.Printf("[ERROR] %v", err)
		os.Exit(1)
	}
}

func run(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	log.Println("[INFO] Starting ecommerce API server...")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	log.Println("[INFO] Configuration loaded successfully")

	if err := media.CheckDependencies(); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
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
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Println("[INFO] Database connection established")

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("[ERROR] Failed to close database connection: %v", closeErr)
		}
	}()

	if err := migrations.EnsureReady(db, cfg.AutoApplyMigrations); err != nil {
		return fmt.Errorf("database migration readiness check failed: %w", err)
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
	authCookieCfg := httpapi.CookieConfig{
		Secure:   cookieSecure,
		Domain:   "",
		SameSite: cookieSameSite,
	}

	mediaService := media.NewService(db, cfg.MediaRoot, cfg.MediaPublicURL, log.Default())
	if err := mediaService.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to initialize media directories: %w", err)
	}

	var workers sync.WaitGroup
	startWorker := func(worker func()) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			worker()
		}()
	}

	pluginManager := checkoutplugins.NewDefaultManager()
	if cfg.CheckoutPluginManifestsDir != "" {
		loaded, loadErr := pluginManager.LoadExternalPluginsFromDir(cfg.CheckoutPluginManifestsDir)
		if loadErr != nil {
			return fmt.Errorf("failed to load checkout plugins: %w", loadErr)
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
				return fmt.Errorf("failed to load provider-backed checkout plugins: %w", loadErr)
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
			return fmt.Errorf("failed to load provider plugins: %w", loadErr)
		}
		paymentProviders = loadedProviders.PaymentProviders
		shippingProviders = loadedProviders.ShippingProviders
		taxProviders = loadedProviders.TaxProviders
		log.Printf("[INFO] Loaded %d external provider plugins from %s", loadedProviders.LoadedCount, cfg.ProviderPluginManifestsDir)
	}

	checkoutCleanupWorker := func() {
		runPeriodic(ctx, 15*time.Minute, true, func(workerCtx context.Context) {
			summary, cleanupErr := checkoutservice.CleanupExpiredState(db.WithContext(workerCtx), time.Now().UTC())
			if cleanupErr != nil {
				if !errors.Is(cleanupErr, context.Canceled) {
					log.Printf("[ERROR] Checkout cleanup failed: %v", cleanupErr)
				}
				return
			}
			if summary.ExpiredSessions > 0 || summary.DeletedIdempotencyKeys > 0 {
				log.Printf(
					"[INFO] Checkout cleanup expired_sessions=%d deleted_idempotency_keys=%d",
					summary.ExpiredSessions,
					summary.DeletedIdempotencyKeys,
				)
			}
		})
	}

	keyring, err := providerops.ParseKeyringConfig(cfg.ProviderCredentialsKeys)
	if err != nil {
		return fmt.Errorf("failed to parse provider credential keys: %w", err)
	}
	credentialService, err := providerops.NewCredentialService(keyring, cfg.ProviderCredentialsKeyVersion)
	if err != nil {
		return fmt.Errorf("failed to initialize provider credential service: %w", err)
	}
	providerRuntime := providerops.NewRuntime(db, providerops.RuntimeConfig{
		Environment:         cfg.ProviderRuntimeEnvironment,
		Credentials:         credentialService,
		PaymentProviders:    paymentProviders,
		ShippingProviders:   shippingProviders,
		TaxProviders:        taxProviders,
		ExecutionTimeout:    cfg.ProviderExecutionTimeout,
		QueryTimeout:        cfg.ProviderQueryTimeout,
		CompensationTimeout: cfg.ProviderCompensationTimeout,
		LeaseDuration:       cfg.ProviderLeaseDuration,
	})

	var reconciliationWorker func()
	if intervalText := cfg.ProviderReconciliationInterval; intervalText != "" {
		interval, parseErr := time.ParseDuration(intervalText)
		if parseErr != nil {
			return fmt.Errorf("failed to parse provider reconciliation interval: %w", parseErr)
		}
		if interval > 0 {
			reconciliationWorker = func() {
				runPeriodic(ctx, interval, false, func(workerCtx context.Context) {
					summary, runErr := providerRuntime.Reconciliation.RunScheduled(workerCtx)
					if runErr != nil {
						if !errors.Is(runErr, context.Canceled) {
							log.Printf("[ERROR] Provider reconciliation failed: %v", runErr)
						}
						return
					}
					if summary.RunCount > 0 {
						log.Printf("[INFO] Provider reconciliation completed runs=%d", summary.RunCount)
					}
				})
			}
		}
	}

	providerCatalog := providerops.NewCatalogService(db, pluginManager)
	if err := providerCatalog.SyncSettings(ctx); err != nil {
		return fmt.Errorf("sync checkout provider settings: %w", err)
	}
	webhookService := webhookservice.NewService(db, paymentProviders, shippingProviders, log.Default())
	accountService := accountservice.NewService(db, credentialService)
	authService := authservice.NewService(db, jwtSecret, cfg.DisableLocalSignIn, accountService)
	renderer := httpapi.Renderer{Report: func(_ context.Context, err error, problem httpapi.Problem) {
		log.Printf("[ERROR] HTTP problem code=%s status=%d: %v", problem.Code, problem.Status, err)
	}}
	accountEndpoints, err := httpapi.NewAccountEndpoints(httpapi.AccountEndpointsOptions{
		Auth: authService, Accounts: accountService, AccountData: accountdata.NewService(db),
		Renderer: renderer, JWTSecret: jwtSecret, Cookies: authCookieCfg,
	})
	if err != nil {
		return fmt.Errorf("initialize account endpoints: %w", err)
	}
	catalogEndpoints, err := httpapi.NewCatalogEndpoints(db, mediaService)
	if err != nil {
		return fmt.Errorf("initialize catalog endpoints: %w", err)
	}
	cmsMediaEndpoints, err := httpapi.NewCmsMediaEndpoints(db, mediaService)
	if err != nil {
		return fmt.Errorf("initialize CMS/media endpoints: %w", err)
	}
	checkoutProviderEndpoints, err := httpapi.NewCheckoutProviderEndpoints(httpapi.CheckoutProviderEndpointsOptions{
		DB: db, Media: mediaService, CheckoutPlugins: pluginManager, ProviderRuntime: providerRuntime, Webhooks: webhookService, Renderer: renderer,
	})
	if err != nil {
		return fmt.Errorf("initialize checkout/provider endpoints: %w", err)
	}
	apiServer, err := httpapi.NewServer(accountEndpoints, catalogEndpoints, cmsMediaEndpoints, checkoutProviderEndpoints)
	if err != nil {
		return fmt.Errorf("compose strict API server: %w", err)
	}
	policies, err := httpapi.ContractPolicySet()
	if err != nil {
		return fmt.Errorf("build operation policies: %w", err)
	}
	if err := httpapi.RegisterStrict(r, apiServer, httpapi.RegisterStrictOptions{
		Strict: httpapi.StrictOptions{Policies: policies}, Renderer: renderer,
		Security: httpapi.SecurityOptions{PreviewSecret: jwtSecret, Authenticator: httpapi.JWTAuthenticator{
			Secret: []byte(jwtSecret), ResolveAccountID: func(resolveCtx context.Context, subject string) (uint, error) {
				user, resolveErr := accountService.UserBySubject(resolveCtx, subject)
				if errors.Is(resolveErr, accountservice.ErrUserNotFound) {
					return 0, nil
				}
				return user.ID, resolveErr
			},
		}},
	}); err != nil {
		return fmt.Errorf("register strict API server: %w", err)
	}

	mediaService.StartProcessor()
	startWorker(func() { webhookService.Run(ctx) })
	startWorker(checkoutCleanupWorker)
	startWorker(func() { providerRuntime.Recovery.Run(ctx) })
	if reconciliationWorker != nil {
		startWorker(reconciliationWorker)
	}
	inventoryservice.StartReservationExpiryWorker(ctx, db.WithContext(ctx), time.Minute, log.Default())
	cmsservice.StartDeliveryWorker(ctx, db.WithContext(ctx), time.Minute, log.Default(), mediaService)
	cmsservice.StartInvalidationWorker(ctx, db.WithContext(ctx), cfg.CMSInvalidationWebhookURL, time.Minute, log.Default())

	requestRootCtx := context.WithoutCancel(ctx)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return requestRootCtx
		},
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	log.Printf("[INFO] Server starting on port %s", cfg.Port)
	select {
	case err := <-serverErr:
		cancel()
		workerCtx, workerCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer workerCancel()
		if workerErr := waitForWorkers(workerCtx, &workers); workerErr != nil {
			return workerErr
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Println("[INFO] Shutdown signal received; draining HTTP server")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		_ = server.Close()
	}
	workerErr := waitForWorkers(shutdownCtx, &workers)
	if shutdownErr != nil {
		return fmt.Errorf("graceful server shutdown: %w", shutdownErr)
	}
	if workerErr != nil {
		return workerErr
	}
	log.Println("[INFO] Server shutdown complete")
	return nil
}

func runPeriodic(ctx context.Context, interval time.Duration, runImmediately bool, run func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if runImmediately {
		select {
		case <-ctx.Done():
			return
		default:
			run(ctx)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run(ctx)
		}
	}
}

func waitForWorkers(ctx context.Context, workers *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("background worker shutdown: %w", ctx.Err())
	}
}
