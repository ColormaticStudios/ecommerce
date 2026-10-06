package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"ecommerce/internal/jobs"
	"ecommerce/internal/media"
	"ecommerce/internal/migrations"
	"ecommerce/internal/providerplugins"
	"ecommerce/internal/reliability"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
	accountservice "ecommerce/internal/services/account"
	"ecommerce/internal/services/accountdata"
	authservice "ecommerce/internal/services/auth"
	checkoutservice "ecommerce/internal/services/checkout"
	cmsservice "ecommerce/internal/services/cms"
	discountservice "ecommerce/internal/services/discounts"
	inventoryservice "ecommerce/internal/services/inventory"
	localizationservice "ecommerce/internal/services/localization"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/internal/telemetry"
	"ecommerce/middleware"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(rootCtx); err != nil {
		slog.Error("Ecommerce API stopped", "error", err)
		os.Exit(1)
	}
}

func run(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	slog.Info("Starting ecommerce API server")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	loggerAttributes := []any{
		"service", cfg.AlertService,
		"deployment_environment", cfg.DeploymentEnvironment,
		"release_id", cfg.ReleaseID,
		"telemetry_enabled", cfg.TelemetryEnabled,
	}
	if cfg.AlertOwner != "" {
		loggerAttributes = append(loggerAttributes, "alert_owner", cfg.AlertOwner)
	}
	applicationLogger := slog.Default().With(loggerAttributes...)
	slog.SetDefault(applicationLogger)
	legacyLogger := slog.NewLogLogger(applicationLogger.Handler(), slog.LevelInfo)
	applicationLogger.Info("Configuration loaded successfully")
	traceShutdown, err := telemetry.ConfigureTracing(ctx, telemetry.TracingConfig{
		Enabled: cfg.TracingEnabled, Endpoint: cfg.OTLPTraceEndpoint, SampleRatio: cfg.TraceSampleRatio,
		Service: cfg.AlertService, Environment: cfg.DeploymentEnvironment,
	})
	if err != nil {
		return fmt.Errorf("configure tracing: %w", err)
	}
	defer func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer shutdownCancel()
		if shutdownErr := traceShutdown(shutdownContext); shutdownErr != nil {
			applicationLogger.Error("Trace exporter shutdown failed", "error", shutdownErr)
		}
	}()
	var metrics *telemetry.Metrics
	if cfg.TelemetryEnabled {
		metrics = telemetry.NewMetrics(cfg.AlertService, cfg.DeploymentEnvironment, cfg.AlertOwner)
	}

	if err := media.CheckDependencies(); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Connect to database
	gormLogger := logger.New(
		legacyLogger,
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{
		Logger: telemetry.NewGORMLogger(gormLogger, metrics),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	applicationLogger.Info("Database connection established")

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			applicationLogger.Error("Failed to close database connection", "error", closeErr)
		}
	}()
	if metrics != nil {
		if err := metrics.RegisterDatabase(db, sqlDB); err != nil {
			return fmt.Errorf("register database metrics: %w", err)
		}
	}

	if err := migrations.EnsureReady(db, cfg.AutoApplyMigrations); err != nil {
		return fmt.Errorf("database migration readiness check failed: %w", err)
	}
	if cfg.AutoApplyMigrations {
		applicationLogger.Info("Database migration completed", "latest_version", migrations.LatestVersion())
	} else {
		applicationLogger.Info("Database migration check completed", "latest_version", migrations.LatestVersion())
	}

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{TrustRequestIDs: cfg.TrustRequestIDs}),
		telemetry.HTTPMiddleware(metrics, cfg.AlertService),
		middleware.AccessLogger(applicationLogger),
		gin.CustomRecovery(func(c *gin.Context, recovered any) {
			correlation := reliability.FromContext(c.Request.Context())
			applicationLogger.ErrorContext(c.Request.Context(), "Panic recovered",
				"request_id", correlation.RequestID,
				"correlation_id", correlation.CorrelationID,
				"panic_type", fmt.Sprintf("%T", recovered),
			)
			c.JSON(500, gin.H{"error": "Internal server error"})
		}),
	)

	if cfg.ServeMedia {
		r.Static("/media", cfg.MediaRoot)
	}

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

	var jobObserver jobs.Observer
	if metrics != nil {
		jobObserver = metrics
	}
	jobRuntime := jobs.NewRuntime(db, jobs.Config{
		WorkerConcurrency: cfg.JobWorkerConcurrency,
		PollInterval:      cfg.JobPollInterval,
		LeaseDuration:     cfg.JobLeaseDuration,
		MaxAttempts:       cfg.JobMaxAttempts,
		RetryBaseDelay:    cfg.JobRetryBaseDelay,
		RetryMaxDelay:     cfg.JobRetryMaxDelay,
		Logger:            applicationLogger,
		Observer:          jobObserver,
	})
	mediaService := media.NewService(db, cfg.MediaRoot, cfg.MediaPublicURL, legacyLogger, jobRuntime)
	if err := mediaService.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to initialize media directories: %w", err)
	}
	if err := mediaService.RegisterJobHandlers(); err != nil {
		return fmt.Errorf("register media job handlers: %w", err)
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
		applicationLogger.Info("Loaded external checkout plugins", "count", loaded, "directory", cfg.CheckoutPluginManifestsDir)
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
			applicationLogger.Info("Loaded provider-backed checkout plugins", "count", loaded, "directory", cfg.ProviderPluginManifestsDir)
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
		applicationLogger.Info("Loaded external provider plugins", "count", loadedProviders.LoadedCount, "directory", cfg.ProviderPluginManifestsDir)
	}

	checkoutCleanupWorker := func() {
		runPeriodic(ctx, 15*time.Minute, true, func(workerCtx context.Context) {
			summary, cleanupErr := checkoutservice.CleanupExpiredState(db.WithContext(workerCtx), time.Now().UTC())
			if cleanupErr != nil {
				if !errors.Is(cleanupErr, context.Canceled) {
					applicationLogger.ErrorContext(workerCtx, "Checkout cleanup failed", "error", cleanupErr)
				}
				return
			}
			if summary.ExpiredSessions > 0 || summary.DeletedIdempotencyKeys > 0 {
				applicationLogger.InfoContext(workerCtx, "Checkout cleanup completed",
					"expired_sessions", summary.ExpiredSessions,
					"deleted_idempotency_keys", summary.DeletedIdempotencyKeys,
				)
			}
		})
	}
	discountLifecycleWorker := func() {
		runPeriodic(ctx, time.Minute, true, func(workerCtx context.Context) {
			result, lifecycleErr := discountservice.RunLifecycle(db.WithContext(workerCtx), time.Now().UTC())
			if lifecycleErr != nil {
				if !errors.Is(lifecycleErr, context.Canceled) {
					applicationLogger.ErrorContext(workerCtx, "Discount lifecycle failed", "error", lifecycleErr)
				}
				return
			}
			if result.Activated > 0 || result.Deactivated > 0 || result.Archived > 0 {
				applicationLogger.InfoContext(workerCtx, "Discount lifecycle completed", "activated", result.Activated, "deactivated", result.Deactivated, "archived", result.Archived)
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
		Observer:            metrics,
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
							applicationLogger.ErrorContext(workerCtx, "Provider reconciliation failed", "error", runErr)
						}
						return
					}
					if summary.RunCount > 0 {
						applicationLogger.InfoContext(workerCtx, "Provider reconciliation completed", "runs", summary.RunCount)
					}
				})
			}
		}
	}

	providerCatalog := providerops.NewCatalogService(db, pluginManager)
	if err := providerCatalog.SyncSettings(ctx); err != nil {
		return fmt.Errorf("sync checkout provider settings: %w", err)
	}
	webhookService := webhookservice.NewService(db, paymentProviders, shippingProviders, legacyLogger)
	accountService := accountservice.NewService(db, credentialService)
	authService := authservice.NewService(db, jwtSecret, cfg.DisableLocalSignIn, accountService)
	renderer := httpapi.Renderer{Report: func(reportContext context.Context, err error, problem httpapi.Problem) {
		correlation := reliability.FromContext(reportContext)
		attributes := []any{"error_type", fmt.Sprintf("%T", err), "error_code", problem.Code, "status_code", problem.Status, "request_id", correlation.RequestID, "correlation_id", correlation.CorrelationID}
		if class, ok := reliability.ErrorClassOf(err); ok {
			attributes = append(attributes, "error_class", class)
		}
		applicationLogger.ErrorContext(reportContext, "HTTP problem", attributes...)
	}}
	accountEndpoints, err := httpapi.NewAccountEndpoints(httpapi.AccountEndpointsOptions{
		Auth: authService, Accounts: accountService, AccountData: accountdata.NewService(db),
		Renderer: renderer, JWTSecret: jwtSecret, Cookies: authCookieCfg,
	})
	if err != nil {
		return fmt.Errorf("initialize account endpoints: %w", err)
	}
	catalogEndpoints, err := httpapi.NewCatalogEndpoints(db, mediaService, jobRuntime)
	if err != nil {
		return fmt.Errorf("initialize catalog endpoints: %w", err)
	}
	if err := catalogEndpoints.RegisterSearchJobHandlers(); err != nil {
		return fmt.Errorf("register search job handlers: %w", err)
	}
	if err := catalogEndpoints.EnsureInitialSearchReindex(ctx); err != nil {
		return fmt.Errorf("schedule initial search reindex: %w", err)
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
	localizationEndpoints, err := httpapi.NewLocalizationEndpointsWithMedia(db, mediaService)
	if err != nil {
		return fmt.Errorf("initialize localization endpoints: %w", err)
	}
	apiServer, err := httpapi.NewServer(accountEndpoints, catalogEndpoints, cmsMediaEndpoints, checkoutProviderEndpoints, localizationEndpoints)
	if err != nil {
		return fmt.Errorf("compose strict API server: %w", err)
	}
	policies, err := httpapi.ContractPolicySet()
	if err != nil {
		return fmt.Errorf("build operation policies: %w", err)
	}
	if err := httpapi.RegisterStrict(r, apiServer, httpapi.RegisterStrictOptions{
		Strict: httpapi.StrictOptions{Policies: policies}, Renderer: renderer,
		Localization: &httpapi.LocalizationNegotiationOptions{
			Service: localizationservice.NewService(db),
			ResolveAccountPreference: func(resolveCtx context.Context, principal requestctx.Principal) (string, error) {
				user, resolveErr := accountService.UserBySubject(resolveCtx, principal.Subject)
				if errors.Is(resolveErr, accountservice.ErrUserNotFound) {
					return "", nil
				}
				return user.Locale, resolveErr
			},
		},
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

	startWorker(func() { jobRuntime.Run(ctx) })
	startWorker(func() {
		salesSearch := searchservice.NewService(db, nil, jobRuntime)
		runPeriodic(ctx, time.Hour, true, func(workerCtx context.Context) {
			if _, err := salesSearch.EnqueueSalesRefresh(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
				applicationLogger.ErrorContext(workerCtx, "Search sales refresh enqueue failed", "error", err)
			}
		})
	})
	startWorker(func() { webhookService.Run(ctx) })
	startWorker(checkoutCleanupWorker)
	startWorker(discountLifecycleWorker)
	startWorker(func() { providerRuntime.Recovery.Run(ctx) })
	if reconciliationWorker != nil {
		startWorker(reconciliationWorker)
	}
	inventoryservice.StartReservationExpiryWorker(ctx, db.WithContext(ctx), time.Minute, legacyLogger)
	cmsservice.StartDeliveryWorker(ctx, db.WithContext(ctx), time.Minute, legacyLogger, mediaService)
	cmsservice.StartInvalidationWorker(ctx, db.WithContext(ctx), cfg.CMSInvalidationWebhookURL, time.Minute, legacyLogger)

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

	var telemetryServer *telemetry.Server
	var telemetryErr <-chan error
	if metrics != nil {
		telemetryServer = telemetry.NewServer(cfg.TelemetryBindAddress, cfg.MetricsPath, metrics.Handler(), telemetry.ReadinessChecks{
			Pool: sqlDB, Database: db, MigrationCheck: migrations.Check, WorkersRunning: jobRuntime.Running,
		})
		telemetryServer.SetReleaseID(cfg.ReleaseID)
		telemetryErr, err = telemetryServer.Start()
		if err != nil {
			cancel()
			_ = server.Close()
			startErr := fmt.Errorf("start telemetry server on %s: %w", cfg.TelemetryBindAddress, err)
			shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
			defer shutdownCancel()
			if workerErr := waitForWorkers(shutdownContext, &workers); workerErr != nil {
				return errors.Join(startErr, workerErr)
			}
			return startErr
		}
		applicationLogger.Info("Telemetry server started", "bind_address", cfg.TelemetryBindAddress, "metrics_path", cfg.MetricsPath)
	}

	applicationLogger.Info("Server starting", "port", cfg.Port)
	select {
	case err := <-serverErr:
		if telemetryServer != nil {
			telemetryServer.SetReady(false)
		}
		cancel()
		workerCtx, workerCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer workerCancel()
		if telemetryServer != nil {
			_ = telemetryServer.Shutdown(workerCtx)
		}
		if workerErr := waitForWorkers(workerCtx, &workers); workerErr != nil {
			return workerErr
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case err := <-telemetryErr:
		telemetryServer.SetReady(false)
		cancel()
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
		if workerErr := waitForWorkers(shutdownContext, &workers); workerErr != nil {
			return workerErr
		}
		if err != nil && !telemetry.IsServerClosed(err) {
			return fmt.Errorf("telemetry server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		if telemetryServer != nil {
			telemetryServer.SetReady(false)
		}
		applicationLogger.Info("Shutdown signal received; draining HTTP server")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		_ = server.Close()
	}
	var telemetryShutdownErr error
	if telemetryServer != nil {
		telemetryShutdownErr = telemetryServer.Shutdown(shutdownCtx)
	}
	workerErr := waitForWorkers(shutdownCtx, &workers)
	if shutdownErr != nil {
		return fmt.Errorf("graceful server shutdown: %w", shutdownErr)
	}
	if workerErr != nil {
		return workerErr
	}
	if telemetryShutdownErr != nil {
		return fmt.Errorf("graceful telemetry server shutdown: %w", telemetryShutdownErr)
	}
	applicationLogger.Info("Server shutdown complete")
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
