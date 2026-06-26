package handlers

import (
	"fmt"
	"net/http"
	"strconv"

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

func (s *GeneratedAPIServer) ListAdminCategories(c *gin.Context, params apicontract.ListAdminCategoriesParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminCategories(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCategory(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminCategory(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminCategory(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", DeleteAdminCategory(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCategory(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateAdminCategory(s.db))
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

func (s *GeneratedAPIServer) ListAdminDiscountCampaigns(c *gin.Context, params apicontract.ListAdminDiscountCampaignsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminDiscountCampaigns(s.db))
}

func (s *GeneratedAPIServer) CreateAdminDiscountCampaign(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminDiscountCampaign(s.db))
}

func (s *GeneratedAPIServer) CreateAdminPromotionCampaign(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminPromotionCampaign(s.db))
}

func (s *GeneratedAPIServer) PreviewAdminPromotion(c *gin.Context) {
	s.runProtected(c, "admin", PreviewAdminPromotion(s.db))
}

func (s *GeneratedAPIServer) ListAdminPromotionTemplates(c *gin.Context, params apicontract.ListAdminPromotionTemplatesParams) {
	if params.Active != nil {
		c.Request.URL.RawQuery = "active=" + strconv.FormatBool(*params.Active)
	}
	s.runProtected(c, "admin", ListAdminPromotionTemplates(s.db))
}

func (s *GeneratedAPIServer) CreateAdminPromotionTemplate(c *gin.Context) {
	s.runProtected(c, "admin", CreateAdminPromotionTemplate(s.db))
}

func (s *GeneratedAPIServer) InstantiateAdminPromotionTemplate(c *gin.Context, id int) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprint(id)})
	s.runProtected(c, "admin", InstantiateAdminPromotionTemplate(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminDiscountCampaign(c *gin.Context, id int) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprint(id)})
	s.runProtected(c, "admin", UpdateAdminDiscountCampaign(s.db))
}

func (s *GeneratedAPIServer) DisableAdminDiscountCampaign(c *gin.Context, id int) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprint(id)})
	s.runProtected(c, "admin", DisableAdminDiscountCampaign(s.db))
}

func (s *GeneratedAPIServer) ScheduleAdminDiscountCampaign(c *gin.Context, id int) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprint(id)})
	s.runProtected(c, "admin", ScheduleAdminDiscountCampaign(s.db))
}

func (s *GeneratedAPIServer) ArchiveAdminDiscountCampaign(c *gin.Context, id int) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprint(id)})
	s.runProtected(c, "admin", ArchiveAdminDiscountCampaign(s.db))
}

func (s *GeneratedAPIServer) RunAdminDiscountLifecycle(c *gin.Context) {
	s.runProtected(c, "admin", RunAdminDiscountLifecycle(s.db))
}

func (s *GeneratedAPIServer) ListAdminDiscountHistory(c *gin.Context, params apicontract.ListAdminDiscountHistoryParams) {
	if params.CampaignId != nil {
		c.Request.URL.RawQuery = "campaign_id=" + fmt.Sprint(*params.CampaignId)
	}
	s.runProtected(c, "admin", ListAdminDiscountHistory(s.db))
}

func (s *GeneratedAPIServer) ListAdminDiscountAudit(c *gin.Context, params apicontract.ListAdminDiscountAuditParams) {
	if params.CampaignId != nil {
		c.Request.URL.RawQuery = "campaign_id=" + fmt.Sprint(*params.CampaignId)
	}
	s.runProtected(c, "admin", ListAdminDiscountAudit(s.db))
}

func (s *GeneratedAPIServer) GetAdminDiscountMetrics(c *gin.Context) {
	s.runProtected(c, "admin", GetAdminDiscountMetrics())
}

func (s *GeneratedAPIServer) RunAdminDiscountReconciliation(c *gin.Context) {
	s.runProtected(c, "admin", RunAdminDiscountReconciliation(s.db))
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

func (s *GeneratedAPIServer) ListAdminCmsPages(c *gin.Context, params apicontract.ListAdminCmsPagesParams) {
	_ = params
	s.runProtected(c, "admin", listAdminCMSPages(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCmsPage(c *gin.Context) {
	s.runProtected(c, "admin", createAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminCmsLocales(c *gin.Context) {
	s.runProtected(c, "admin", getAdminCMSLocales(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsLocales(c *gin.Context) {
	s.runProtected(c, "admin", updateAdminCMSLocales(s.db))
}

func (s *GeneratedAPIServer) ListAdminCmsPageVariants(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", listAdminCMSPageVariants(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCmsPageVariant(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", createAdminCMSPageVariant(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateAdminCmsPageVariant(c *gin.Context, id int, variantId int) {
	_, _ = id, variantId
	s.runProtected(c, "admin", updateAdminCMSPageVariant(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DeleteAdminCmsPageVariant(c *gin.Context, id int, variantId int) {
	_, _ = id, variantId
	s.runProtected(c, "admin", deleteAdminCMSPageVariant(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) TransitionAdminCmsPageVariant(c *gin.Context, id int, variantId int, action apicontract.TransitionAdminCmsPageVariantParamsAction) {
	_, _, _ = id, variantId, action
	s.runProtected(c, "admin", transitionAdminCMSPageVariant(s.db))
}

func (s *GeneratedAPIServer) ListAdminCmsAuditEvents(c *gin.Context, params apicontract.ListAdminCmsAuditEventsParams) {
	_ = params
	s.runProtected(c, "admin", listAdminCMSAuditEvents(s.db))
}

func (s *GeneratedAPIServer) GetAdminCmsGovernance(c *gin.Context) {
	s.runProtected(c, "admin", getAdminCMSGovernance(s.db))
}
func (s *GeneratedAPIServer) UpdateAdminCmsGovernance(c *gin.Context) {
	s.runProtected(c, "admin", updateAdminCMSGovernance(s.db))
}
func (s *GeneratedAPIServer) GetAdminCmsEntryWorkflow(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSEntryWorkflow(s.db))
}
func (s *GeneratedAPIServer) TransitionAdminCmsEntryWorkflow(c *gin.Context, id int, action apicontract.TransitionAdminCmsEntryWorkflowParamsAction) {
	_, _ = id, action
	s.runProtected(c, "admin", transitionAdminCMSEntryWorkflow(s.db))
}
func (s *GeneratedAPIServer) CreateAdminCmsEntryComment(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", createAdminCMSEntryComment(s.db))
}
func (s *GeneratedAPIServer) ResolveAdminCmsComment(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", resolveAdminCMSComment(s.db))
}
func (s *GeneratedAPIServer) ListAdminCmsEntryVariants(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", listAdminCMSEntryVariants(s.db))
}
func (s *GeneratedAPIServer) CreateAdminCmsEntryVariant(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", createAdminCMSEntryVariant(s.db))
}
func (s *GeneratedAPIServer) UpdateAdminCmsEntryVariant(c *gin.Context, id, variantId int) {
	_, _ = id, variantId
	s.runProtected(c, "admin", updateAdminCMSEntryVariant(s.db))
}
func (s *GeneratedAPIServer) DeleteAdminCmsEntryVariant(c *gin.Context, id, variantId int) {
	_, _ = id, variantId
	s.runProtected(c, "admin", deleteAdminCMSEntryVariant(s.db))
}
func (s *GeneratedAPIServer) TransitionAdminCmsEntryVariant(c *gin.Context, id, variantId int, action apicontract.TransitionAdminCmsEntryVariantParamsAction) {
	_, _, _ = id, variantId, action
	s.runProtected(c, "admin", transitionAdminCMSEntryVariant(s.db))
}
func (s *GeneratedAPIServer) GetAdminCmsOperations(c *gin.Context) {
	s.runProtected(c, "admin", getAdminCMSOperations(s.db))
}
func (s *GeneratedAPIServer) RetryAdminCmsInvalidation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", retryAdminCMSInvalidation(s.db))
}
func (s *GeneratedAPIServer) PreviewAdminCmsRestore(c *gin.Context) {
	s.runProtected(c, "admin", previewAdminCMSRestore())
}

func (s *GeneratedAPIServer) ExportAdminCmsContent(c *gin.Context) {
	s.runProtected(c, "admin", exportAdminCMSContent(s.db))
}

func (s *GeneratedAPIServer) RestoreAdminCmsContent(c *gin.Context) {
	s.runProtected(c, "admin", restoreAdminCMSContent(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSPage(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DeleteAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", deleteAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) PublishAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", publishAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UnpublishAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", unpublishAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DiscardAdminCmsPageDraft(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", discardAdminCMSPageDraft(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) RollbackAdminCmsPage(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", rollbackAdminCMSPage(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminCmsPageDelivery(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSPageDelivery(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsPageDelivery(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSPageDelivery(s.db))
}

func (s *GeneratedAPIServer) GetAdminCmsPageSeo(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSPageSEO(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsPageSeo(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSPageSEO(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListAdminCmsRedirects(c *gin.Context) {
	s.runProtected(c, "admin", listAdminCMSRedirects(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCmsRedirect(c *gin.Context) {
	s.runProtected(c, "admin", createAdminCMSRedirect(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsRedirect(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSRedirect(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminCmsRedirect(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", deleteAdminCMSRedirect(s.db))
}

func (s *GeneratedAPIServer) PreviewAdminCmsPayload(c *gin.Context) {
	s.runProtected(c, "admin", previewAdminCMSPayload(s.db))
}

func (s *GeneratedAPIServer) ListAdminCmsNavigation(c *gin.Context, params apicontract.ListAdminCmsNavigationParams) {
	_ = params
	s.runProtected(c, "admin", listAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCmsNavigation(c *gin.Context) {
	s.runProtected(c, "admin", createAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) GetAdminCmsNavigation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsNavigation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) DeleteAdminCmsNavigation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", deleteAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) PublishAdminCmsNavigation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", publishAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) UnpublishAdminCmsNavigation(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", unpublishAdminCMSNavigation(s.db))
}

func (s *GeneratedAPIServer) DiscardAdminCmsNavigationDraft(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", discardAdminCMSNavigationDraft(s.db))
}

func (s *GeneratedAPIServer) ListAdminCmsGlobalRegions(c *gin.Context, params apicontract.ListAdminCmsGlobalRegionsParams) {
	_ = params
	s.runProtected(c, "admin", listAdminCMSGlobalRegions(s.db))
}

func (s *GeneratedAPIServer) CreateAdminCmsGlobalRegion(c *gin.Context) {
	s.runProtected(c, "admin", createAdminCMSGlobalRegion(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminCmsGlobalRegion(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", getAdminCMSGlobalRegion(s.db))
}

func (s *GeneratedAPIServer) UpdateAdminCmsGlobalRegion(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", updateAdminCMSGlobalRegion(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DeleteAdminCmsGlobalRegion(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", deleteAdminCMSGlobalRegion(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) PublishAdminCmsGlobalRegion(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", publishAdminCMSGlobalRegion(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UnpublishAdminCmsGlobalRegion(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", unpublishAdminCMSGlobalRegion(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DiscardAdminCmsGlobalRegionDraft(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", discardAdminCMSGlobalRegionDraft(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) GetAdminWebsiteSettings(c *gin.Context) {
	s.runProtected(c, "admin", GetAdminWebsiteSettings(s.db))
}

func (s *GeneratedAPIServer) UpdateWebsiteSettings(c *gin.Context) {
	s.runProtected(c, "admin", UpsertWebsiteSettingsWithCredentials(s.db, s.providerRuntime.Credentials))
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
	GetAuthConfig(s.db, s.disableLocalSignIn)(c)
}

func (s *GeneratedAPIServer) OidcCallback(c *gin.Context, params apicontract.OidcCallbackParams) {
	_ = params
	OIDCCallbackWithCredentials(s.db, s.jwtSecret, s.authCookieCfg, s.providerRuntime.Credentials)(c)
}

func (s *GeneratedAPIServer) OidcLogin(c *gin.Context, params apicontract.OidcLoginParams) {
	_ = params
	OIDCLoginWithCredentials(s.db, s.authCookieCfg, s.providerRuntime.Credentials)(c)
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

func (s *GeneratedAPIServer) ListCategories(c *gin.Context) {
	ListCategories(s.db)(c)
}

func (s *GeneratedAPIServer) ListProductAttributes(c *gin.Context) {
	ListProductAttributes(s.db)(c)
}

func (s *GeneratedAPIServer) GetProduct(c *gin.Context, id int) {
	_ = id
	s.applyDraftPreview(c)
	GetProductByID(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) ResolveContentPage(c *gin.Context, path string, params apicontract.ResolveContentPageParams) {
	_ = path
	_ = params
	s.applyDraftPreview(c)
	if user, err := findAuthenticatedUserIfPresent(s.db, c, s.jwtSecret); err == nil && user != nil {
		c.Set("cms_authenticated", true)
		c.Set("cms_customer_assignment", strconv.FormatUint(uint64(user.ID), 10))
	}
	resolveContentPage(s.db)(c)
}

func (s *GeneratedAPIServer) ResolveContentHomepage(c *gin.Context, params apicontract.ResolveContentHomepageParams) {
	_ = params
	c.Params = append(c.Params, gin.Param{Key: "path", Value: "/"})
	s.applyDraftPreview(c)
	if user, err := findAuthenticatedUserIfPresent(s.db, c, s.jwtSecret); err == nil && user != nil {
		c.Set("cms_authenticated", true)
		c.Set("cms_customer_assignment", strconv.FormatUint(uint64(user.ID), 10))
	}
	resolveContentPage(s.db)(c)
}

func (s *GeneratedAPIServer) RecordContentEvent(c *gin.Context) {
	s.runPublic(c, recordContentEvent(s.db))
}

func (s *GeneratedAPIServer) ResolveContentRedirect(c *gin.Context, params apicontract.ResolveContentRedirectParams) {
	_ = params
	s.runPublic(c, resolveContentRedirect(s.db))
}

func (s *GeneratedAPIServer) GetContentSitemap(c *gin.Context) {
	s.runPublic(c, getContentSitemap(s.db))
}

func (s *GeneratedAPIServer) GetContentNavigation(c *gin.Context, location string) {
	_ = location
	s.applyDraftPreview(c)
	resolveContentNavigation(s.db)(c)
}

func (s *GeneratedAPIServer) GetContentGlobalRegion(c *gin.Context, region string) {
	_ = region
	s.applyDraftPreview(c)
	resolveContentGlobalRegion(s.db)(c)
}
