package handlers

import (
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/media"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GeneratedAPIServerConfig contains runtime dependencies for generated endpoint handlers.
type GeneratedAPIServerConfig struct {
	JWTSecret          string
	DisableLocalSignIn bool
	AuthCookieConfig   AuthCookieConfig
	OIDCProvider       string
	OIDCClientID       string
	OIDCRedirectURI    string
	MediaUploads       http.Handler
	CheckoutPlugins    *checkoutplugins.Manager
	WebhookService     *webhookservice.Service
	ProviderRuntime    *providerops.Runtime
}

// GeneratedAPIServer adapts generated OpenAPI routes to existing handler implementations.
type GeneratedAPIServer struct {
	db                 *gorm.DB
	mediaService       *media.Service
	pluginManager      *checkoutplugins.Manager
	jwtSecret          string
	disableLocalSignIn bool
	authCookieCfg      AuthCookieConfig
	oidcProvider       string
	oidcClientID       string
	oidcRedirectURI    string
	mediaUploads       http.Handler
	webhookService     *webhookservice.Service
	providerRuntime    *providerops.Runtime
	providerOverview   *providerops.OverviewService
}

func NewGeneratedAPIServer(db *gorm.DB, mediaService *media.Service, cfg GeneratedAPIServerConfig) (*GeneratedAPIServer, error) {
	pluginManager := cfg.CheckoutPlugins
	if pluginManager == nil {
		pluginManager = checkoutplugins.NewDefaultManager()
	}
	if err := syncCheckoutProviderSettings(db, pluginManager); err != nil {
		return nil, fmt.Errorf("sync checkout provider settings: %w", err)
	}
	runtime := cfg.ProviderRuntime
	if runtime == nil {
		runtime = providerops.NewRuntime(db, providerops.RuntimeConfig{
			Environment:       "sandbox",
			PaymentProviders:  paymentservice.NewDefaultProviderRegistry(),
			ShippingProviders: shippingservice.NewDefaultProviderRegistry(),
			TaxProviders:      taxservice.NewDefaultProviderRegistry(),
		})
	}
	webhookSvc := cfg.WebhookService
	if webhookSvc == nil {
		webhookSvc = webhookservice.NewService(
			db,
			runtime.PaymentProviders,
			runtime.ShippingProviders,
			nil,
		)
	}
	webhookSvc.StartProcessor()
	providerOverview := providerops.NewOverviewService(
		db,
		runtime.Environment,
		runtime.Credentials,
		webhookSvc.MaxAttempts,
	)

	return &GeneratedAPIServer{
		db:                 db,
		mediaService:       mediaService,
		pluginManager:      pluginManager,
		jwtSecret:          cfg.JWTSecret,
		disableLocalSignIn: cfg.DisableLocalSignIn,
		authCookieCfg:      cfg.AuthCookieConfig,
		oidcProvider:       cfg.OIDCProvider,
		oidcClientID:       cfg.OIDCClientID,
		oidcRedirectURI:    cfg.OIDCRedirectURI,
		mediaUploads:       cfg.MediaUploads,
		webhookService:     webhookSvc,
		providerRuntime:    runtime,
		providerOverview:   providerOverview,
	}, nil
}

func (s *GeneratedAPIServer) requireAuthenticatedUser(c *gin.Context, requiredRole string) bool {
	middleware.AuthMiddleware(s.jwtSecret, requiredRole)(c)
	return !c.IsAborted()
}

func (s *GeneratedAPIServer) requireCSRF(c *gin.Context) bool {
	middleware.CSRFMiddleware()(c)
	return !c.IsAborted()
}

func (s *GeneratedAPIServer) runProtected(c *gin.Context, requiredRole string, handler gin.HandlerFunc) {
	if !s.requireAuthenticatedUser(c, requiredRole) {
		return
	}
	if !s.requireCSRF(c) {
		return
	}
	handler(c)
}

func (s *GeneratedAPIServer) runWithCSRF(c *gin.Context, handler gin.HandlerFunc) {
	if !s.requireCSRF(c) {
		return
	}
	handler(c)
}

func (s *GeneratedAPIServer) runPublic(c *gin.Context, handler gin.HandlerFunc) {
	handler(c)
}

func (s *GeneratedAPIServer) runWithCheckoutCartBootstrapCSRF(c *gin.Context, handler gin.HandlerFunc) {
	if s.shouldAllowCheckoutCartBootstrapWithoutCSRF(c) {
		handler(c)
		return
	}
	s.runWithCSRF(c, handler)
}

func (s *GeneratedAPIServer) shouldAllowCheckoutCartBootstrapWithoutCSRF(c *gin.Context) bool {
	for _, cookieName := range []string{SessionCookieName, checkoutSessionCookieName, csrfCookieName} {
		if _, err := c.Cookie(cookieName); err == nil {
			return false
		}
	}
	return true
}

func (s *GeneratedAPIServer) applyDraftPreview(c *gin.Context) {
	_ = enableDraftPreviewContext(c, s.jwtSecret)
}

func (s *GeneratedAPIServer) ListAdminOrders(c *gin.Context, params apicontract.ListAdminOrdersParams) {
	_ = params
	s.runProtected(c, "admin", GetAllOrders(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListAdminCheckoutPlugins(c *gin.Context) {
	s.runProtected(c, "admin", ListAdminCheckoutPlugins(s.pluginManager))
}

func (s *GeneratedAPIServer) UpdateAdminCheckoutPlugin(c *gin.Context, pType apicontract.UpdateAdminCheckoutPluginParamsType, id string) {
	_ = pType
	_ = id
	s.runProtected(c, "admin", UpdateAdminCheckoutPlugin(s.db, s.pluginManager))
}

func (s *GeneratedAPIServer) ListAdminInventoryReservations(c *gin.Context, params apicontract.ListAdminInventoryReservationsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminInventoryReservations(s.db))
}

func (s *GeneratedAPIServer) ListAdminInventoryAlerts(c *gin.Context, params apicontract.ListAdminInventoryAlertsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminInventoryAlerts(s.db))
}

func (s *GeneratedAPIServer) AckAdminInventoryAlert(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", AckAdminInventoryAlert(s.db))
}

func (s *GeneratedAPIServer) ResolveAdminInventoryAlert(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", ResolveAdminInventoryAlert(s.db))
}

func (s *GeneratedAPIServer) ListAdminInventoryThresholds(c *gin.Context, params apicontract.ListAdminInventoryThresholdsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminInventoryThresholds(s.db))
}

func (s *GeneratedAPIServer) UpsertAdminInventoryThreshold(c *gin.Context) {
	s.runProtected(c, "admin", UpsertAdminInventoryThreshold(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminInventoryThreshold(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DeleteAdminInventoryThreshold(s.db))
}

func (s *GeneratedAPIServer) CreateAdminInventoryAdjustment(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminInventoryAdjustment(s.db))
}

func (s *GeneratedAPIServer) RunAdminInventoryReconciliation(c *gin.Context) {
	s.runProtected(c, "admin", RunAdminInventoryReconciliation(s.db))
}

func (s *GeneratedAPIServer) GetAdminInventoryTimeline(c *gin.Context, productVariantId int, params apicontract.GetAdminInventoryTimelineParams) {
	_ = productVariantId
	_ = params
	s.runProtected(c, "admin", GetAdminInventoryTimeline(s.db))
}

func (s *GeneratedAPIServer) ListAdminPurchaseOrders(c *gin.Context, params apicontract.ListAdminPurchaseOrdersParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminPurchaseOrders(s.db))
}

func (s *GeneratedAPIServer) CreateAdminPurchaseOrder(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminPurchaseOrder(s.db))
}

func (s *GeneratedAPIServer) IssueAdminPurchaseOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", IssueAdminPurchaseOrder(s.db))
}

func (s *GeneratedAPIServer) CancelAdminPurchaseOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", CancelAdminPurchaseOrder(s.db))
}

func (s *GeneratedAPIServer) ReceiveAdminPurchaseOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", ReceiveAdminPurchaseOrder(s.db))
}

func (s *GeneratedAPIServer) GetAdminOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", GetAdminOrderByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminOrderPayments(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", GetAdminOrderPayments(s.db))
}

func (s *GeneratedAPIServer) CreateAdminOrderShippingLabel(c *gin.Context, id int, params apicontract.CreateAdminOrderShippingLabelParams) {
	_ = id
	_ = params
	s.runProtected(c, "admin", CreateAdminOrderShippingLabel(s.db, s.providerRuntime.ShippingProviders))
}

func (s *GeneratedAPIServer) CaptureAdminOrderPayment(c *gin.Context, id int, intentId int, params apicontract.CaptureAdminOrderPaymentParams) {
	_ = id
	_ = intentId
	_ = params
	s.runProtected(c, "admin", CaptureAdminOrderPayment(s.db, s.providerRuntime.PaymentProviders, s.mediaService))
}

func (s *GeneratedAPIServer) VoidAdminOrderPayment(c *gin.Context, id int, intentId int, params apicontract.VoidAdminOrderPaymentParams) {
	_ = id
	_ = intentId
	_ = params
	s.runProtected(c, "admin", VoidAdminOrderPayment(s.db, s.providerRuntime.PaymentProviders, s.mediaService))
}

func (s *GeneratedAPIServer) RefundAdminOrderPayment(c *gin.Context, id int, intentId int, params apicontract.RefundAdminOrderPaymentParams) {
	_ = id
	_ = intentId
	_ = params
	s.runProtected(c, "admin", RefundAdminOrderPayment(s.db, s.providerRuntime.PaymentProviders, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateOrderStatus(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateOrderStatus(s.db))
}

func (s *GeneratedAPIServer) ListAdminWebhookEvents(c *gin.Context, params apicontract.ListAdminWebhookEventsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminWebhookEvents(s.db, s.webhookService))
}

func (s *GeneratedAPIServer) ExportAdminTaxReport(c *gin.Context, params apicontract.ExportAdminTaxReportParams) {
	_ = params
	s.runProtected(c, "admin", ExportAdminTaxReport(s.db, s.providerRuntime.TaxProviders))
}

func (s *GeneratedAPIServer) ListAdminProviderCredentials(c *gin.Context, params apicontract.ListAdminProviderCredentialsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminProviderCredentials(s.db, s.providerRuntime.Credentials))
}

func (s *GeneratedAPIServer) UpsertAdminProviderCredential(c *gin.Context) {
	s.runProtected(c, "admin", UpsertAdminProviderCredential(s.db, s.providerRuntime.Credentials))
}

func (s *GeneratedAPIServer) RotateAdminProviderCredential(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", RotateAdminProviderCredential(s.db, s.providerRuntime.Credentials))
}

func (s *GeneratedAPIServer) GetAdminProviderOperationsOverview(c *gin.Context) {
	s.runProtected(c, "admin", GetAdminProviderOperationsOverview(s.providerOverview))
}

func (s *GeneratedAPIServer) ListAdminProviderReconciliationRuns(c *gin.Context, params apicontract.ListAdminProviderReconciliationRunsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminProviderReconciliationRuns(s.providerRuntime.Reconciliation))
}

func (s *GeneratedAPIServer) CreateAdminProviderReconciliationRun(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminProviderReconciliationRun(s.providerRuntime.Reconciliation))
}

func (s *GeneratedAPIServer) GetAdminProviderReconciliationRun(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", GetAdminProviderReconciliationRun(s.providerRuntime.Reconciliation))
}

func (s *GeneratedAPIServer) ReceiveWebhookEvent(c *gin.Context, provider string) {
	_ = provider
	s.runPublic(c, ReceiveWebhookEvent(s.db, s.webhookService))
}

func (s *GeneratedAPIServer) CreateProduct(c *gin.Context) {
	s.runProtected(c, "admin", CreateProduct(s.db))
}

func (s *GeneratedAPIServer) CreateAdminBrand(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminBrand(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminBrand(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DeleteAdminBrand(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminBrand(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateAdminBrand(s.db))
}

func (s *GeneratedAPIServer) ListAdminProducts(c *gin.Context, params apicontract.ListAdminProductsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminProducts(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListAdminBrands(c *gin.Context, params apicontract.ListAdminBrandsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminBrands(s.db))
}

func (s *GeneratedAPIServer) ListAdminProductAttributes(c *gin.Context) {
	s.runProtected(c, "admin", ListAdminProductAttributes(s.db))
}

func (s *GeneratedAPIServer) CreateAdminProductAttribute(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminProductAttribute(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminProductAttribute(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateAdminProductAttribute(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminProductAttribute(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DeleteAdminProductAttribute(s.db))
}

func (s *GeneratedAPIServer) GetAdminProduct(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", GetAdminProductByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DeleteProduct(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DeleteProduct(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateProduct(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateProduct(s.db))
}

func (s *GeneratedAPIServer) AttachProductMedia(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", AttachProductMedia(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateProductMediaOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateProductMediaOrder(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DetachProductMedia(c *gin.Context, id int, mediaId string) {
	_ = id
	_ = mediaId
	s.runProtected(c, "admin", DetachProductMedia(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateProductRelated(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateProductRelated(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) PublishProduct(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", PublishProduct(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UnpublishProduct(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UnpublishProduct(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DiscardProductDraft(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DiscardProductDraft(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminStorefrontSettings(c *gin.Context) {
	s.runProtected(c, "admin", GetAdminStorefrontSettings(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateStorefrontSettings(c *gin.Context) {
	s.runProtected(c, "admin", UpsertStorefrontSettings(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) PublishStorefrontSettings(c *gin.Context) {
	s.runProtected(c, "admin", PublishStorefrontSettings(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DiscardStorefrontDraft(c *gin.Context) {
	s.runProtected(c, "admin", DiscardStorefrontDraft(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminPreview(c *gin.Context) {
	s.runProtected(c, "admin", GetDraftPreviewSessionStatus(s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) StartAdminPreview(c *gin.Context) {
	s.runProtected(c, "admin", StartDraftPreviewSession(s.jwtSecret, s.authCookieCfg, defaultDraftPreviewTTL))
}

func (s *GeneratedAPIServer) StopAdminPreview(c *gin.Context) {
	s.runProtected(c, "admin", StopDraftPreviewSession(s.authCookieCfg))
}

func (s *GeneratedAPIServer) ListUsers(c *gin.Context, params apicontract.ListUsersParams) {
	_ = params
	s.runProtected(c, "admin", GetAllUsers(s.db))
}

func (s *GeneratedAPIServer) UpdateUserRole(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateUserRole(s.db))
}

func (s *GeneratedAPIServer) Login(c *gin.Context) {
	if s.disableLocalSignIn {
		c.Status(http.StatusNotFound)
		return
	}
	Login(s.db, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) Logout(c *gin.Context) {
	Logout(s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) GetAuthConfig(c *gin.Context) {
	GetAuthConfig(s.disableLocalSignIn, s.oidcProvider, s.oidcClientID, s.oidcRedirectURI)(c)
}

func (s *GeneratedAPIServer) OidcCallback(c *gin.Context, params apicontract.OidcCallbackParams) {
	_ = params
	if !oidcConfigured(s.oidcProvider, s.oidcClientID, s.oidcRedirectURI) {
		c.Status(http.StatusNotFound)
		return
	}
	OIDCCallback(s.db, s.jwtSecret, s.oidcProvider, s.oidcClientID, s.oidcRedirectURI, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) OidcLogin(c *gin.Context, params apicontract.OidcLoginParams) {
	_ = params
	if !oidcConfigured(s.oidcProvider, s.oidcClientID, s.oidcRedirectURI) {
		c.Status(http.StatusNotFound)
		return
	}
	OIDCLogin(s.oidcProvider, s.oidcClientID, s.oidcRedirectURI, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) Register(c *gin.Context) {
	if s.disableLocalSignIn {
		c.Status(http.StatusNotFound)
		return
	}
	Register(s.db, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) GetCheckoutCart(c *gin.Context) {
	GetCart(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) GetCheckoutCartSummary(c *gin.Context) {
	GetCartSummary(s.db, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) AddCheckoutCartItem(c *gin.Context) {
	s.runWithCheckoutCartBootstrapCSRF(c, AddCartItem(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) DeleteCheckoutCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runWithCSRF(c, DeleteCartItem(s.db, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) UpdateCheckoutCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runWithCSRF(c, UpdateCartItem(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) ListCheckoutSessionPlugins(c *gin.Context) {
	ListCheckoutPluginsWithAccess(s.db, s.pluginManager, s.jwtSecret)(c)
}

func (s *GeneratedAPIServer) QuoteCheckoutSession(c *gin.Context) {
	QuoteCheckout(s.db, s.pluginManager, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) CreateCheckoutOrder(c *gin.Context, params apicontract.CreateCheckoutOrderParams) {
	_ = params
	s.runWithCSRF(c, CreateCheckoutOrder(s.db, s.jwtSecret, s.authCookieCfg, s.mediaService))
}

func (s *GeneratedAPIServer) AuthorizeCheckoutOrderPayment(c *gin.Context, id int, params apicontract.AuthorizeCheckoutOrderPaymentParams) {
	_ = id
	_ = params
	s.runWithCSRF(c, AuthorizeCheckoutOrderPayment(s.db, s.providerRuntime.PaymentProviders, s.pluginManager, s.jwtSecret, s.authCookieCfg, s.mediaService))
}

func (s *GeneratedAPIServer) QuoteCheckoutOrderShippingRates(c *gin.Context, id int, params apicontract.QuoteCheckoutOrderShippingRatesParams) {
	_ = id
	_ = params
	s.runWithCSRF(c, QuoteCheckoutOrderShippingRates(s.db, s.providerRuntime.ShippingProviders, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) GetCheckoutOrderShippingTracking(c *gin.Context, id int) {
	_ = id
	GetCheckoutOrderShippingTracking(s.db, s.jwtSecret, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) FinalizeCheckoutOrderTax(c *gin.Context, id int, params apicontract.FinalizeCheckoutOrderTaxParams) {
	_ = id
	_ = params
	s.runWithCSRF(c, FinalizeCheckoutOrderTax(s.db, s.providerRuntime.TaxProviders, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) GetProfile(c *gin.Context) {
	s.runProtected(c, "", GetProfile(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateProfile(c *gin.Context) {
	s.runProtected(c, "", UpdateProfile(s.db))
}

func (s *GeneratedAPIServer) ListSavedAddresses(c *gin.Context) {
	s.runProtected(c, "", GetSavedAddresses(s.db))
}

func (s *GeneratedAPIServer) CreateSavedAddress(c *gin.Context) {
	s.runProtected(c, "", CreateSavedAddress(s.db))
}

func (s *GeneratedAPIServer) DeleteSavedAddress(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", DeleteSavedAddress(s.db))
}

func (s *GeneratedAPIServer) SetDefaultAddress(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", SetDefaultAddress(s.db))
}

func (s *GeneratedAPIServer) GetCart(c *gin.Context) {
	s.runProtected(c, "", GetCart(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) AddCartItem(c *gin.Context) {
	s.runProtected(c, "", AddCartItem(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) DeleteCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runProtected(c, "", DeleteCartItem(s.db, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) UpdateCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runProtected(c, "", UpdateCartItem(s.db, s.mediaService, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) ListUserOrders(c *gin.Context, params apicontract.ListUserOrdersParams) {
	_ = params
	s.runProtected(c, "", GetUserOrders(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ClaimGuestOrder(c *gin.Context) {
	s.runProtected(c, "", ClaimGuestOrder(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) CreateOrder(c *gin.Context) {
	s.runProtected(c, "", CreateOrder(s.db, s.jwtSecret, s.authCookieCfg, s.mediaService))
}

func (s *GeneratedAPIServer) ListCheckoutPlugins(c *gin.Context) {
	s.runProtected(c, "", ListCheckoutPluginsWithAccess(s.db, s.pluginManager, s.jwtSecret))
}

func (s *GeneratedAPIServer) QuoteCheckout(c *gin.Context) {
	s.runProtected(c, "", QuoteCheckout(s.db, s.pluginManager, s.jwtSecret, s.authCookieCfg))
}

func (s *GeneratedAPIServer) GetUserOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", GetOrderByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) CancelUserOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", CancelUserOrder(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListSavedPaymentMethods(c *gin.Context) {
	s.runProtected(c, "", GetSavedPaymentMethods(s.db))
}

func (s *GeneratedAPIServer) CreateSavedPaymentMethod(c *gin.Context) {
	s.runProtected(c, "", CreateSavedPaymentMethod(s.db))
}

func (s *GeneratedAPIServer) DeleteSavedPaymentMethod(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", DeleteSavedPaymentMethod(s.db))
}

func (s *GeneratedAPIServer) SetDefaultPaymentMethod(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", SetDefaultPaymentMethod(s.db))
}

func (s *GeneratedAPIServer) DeleteProfilePhoto(c *gin.Context) {
	s.runProtected(c, "", DeleteProfilePhoto(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) SetProfilePhoto(c *gin.Context) {
	s.runProtected(c, "", SetProfilePhoto(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) CreateMediaUpload(c *gin.Context, params apicontract.CreateMediaUploadParams) {
	_ = params
	if s.mediaUploads == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Media uploads unavailable"})
		return
	}
	s.runProtected(c, "", func(c *gin.Context) {
		s.mediaUploads.ServeHTTP(c.Writer, c.Request)
	})
}

func (s *GeneratedAPIServer) HeadMediaUpload(c *gin.Context, path string) {
	_ = path
	if s.mediaUploads == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	s.runProtected(c, "", func(c *gin.Context) {
		s.mediaUploads.ServeHTTP(c.Writer, c.Request)
	})
}

func (s *GeneratedAPIServer) PatchMediaUpload(c *gin.Context, path string) {
	_ = path
	if s.mediaUploads == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Media uploads unavailable"})
		return
	}
	s.runProtected(c, "", func(c *gin.Context) {
		s.mediaUploads.ServeHTTP(c.Writer, c.Request)
	})
}

func (s *GeneratedAPIServer) ListProducts(c *gin.Context, params apicontract.ListProductsParams) {
	_ = params
	s.applyDraftPreview(c)
	GetProducts(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) ListBrands(c *gin.Context) {
	ListBrands(s.db)(c)
}

func (s *GeneratedAPIServer) ListProductAttributes(c *gin.Context) {
	ListProductAttributes(s.db)(c)
}

func (s *GeneratedAPIServer) GetProduct(c *gin.Context, id int) {
	_ = id
	s.applyDraftPreview(c)
	GetProductByID(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) GetStorefrontSettings(c *gin.Context) {
	s.applyDraftPreview(c)
	GetStorefrontSettings(s.db, s.mediaService)(c)
}
