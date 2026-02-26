package handlers

import (
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/media"
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
}

func NewGeneratedAPIServer(db *gorm.DB, mediaService *media.Service, cfg GeneratedAPIServerConfig) (*GeneratedAPIServer, error) {
	pluginManager := cfg.CheckoutPlugins
	if pluginManager == nil {
		pluginManager = checkoutplugins.NewDefaultManager()
	}
	if err := syncCheckoutProviderSettings(db, pluginManager); err != nil {
		return nil, fmt.Errorf("sync checkout provider settings: %w", err)
	}

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

func (s *GeneratedAPIServer) GetAdminOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", GetAdminOrderByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateOrderStatus(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "admin", UpdateOrderStatus(s.db))
}

func (s *GeneratedAPIServer) CreateProduct(c *gin.Context) {
	s.runProtected(c, "admin", CreateProduct(s.db))
}

func (s *GeneratedAPIServer) ListAdminProducts(c *gin.Context, params apicontract.ListAdminProductsParams) {
	_ = params
	s.runProtected(c, "admin", ListAdminProducts(s.db, s.mediaService))
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

func (s *GeneratedAPIServer) OidcCallback(c *gin.Context, params apicontract.OidcCallbackParams) {
	_ = params
	OIDCCallback(s.db, s.jwtSecret, s.oidcProvider, s.oidcClientID, s.oidcRedirectURI, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) OidcLogin(c *gin.Context, params apicontract.OidcLoginParams) {
	_ = params
	OIDCLogin(s.oidcProvider, s.oidcClientID, s.oidcRedirectURI, s.authCookieCfg)(c)
}

func (s *GeneratedAPIServer) Register(c *gin.Context) {
	if s.disableLocalSignIn {
		c.Status(http.StatusNotFound)
		return
	}
	Register(s.db, s.jwtSecret, s.authCookieCfg)(c)
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
	s.runProtected(c, "", GetCart(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) AddCartItem(c *gin.Context) {
	s.runProtected(c, "", AddCartItem(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) DeleteCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runProtected(c, "", DeleteCartItem(s.db))
}

func (s *GeneratedAPIServer) UpdateCartItem(c *gin.Context, itemId int) {
	_ = itemId
	s.runProtected(c, "", UpdateCartItem(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListUserOrders(c *gin.Context, params apicontract.ListUserOrdersParams) {
	_ = params
	s.runProtected(c, "", GetUserOrders(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) CreateOrder(c *gin.Context) {
	s.runProtected(c, "", CreateOrder(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ListCheckoutPlugins(c *gin.Context) {
	s.runProtected(c, "", ListCheckoutPlugins(s.pluginManager))
}

func (s *GeneratedAPIServer) QuoteCheckout(c *gin.Context) {
	s.runProtected(c, "", QuoteCheckout(s.db, s.pluginManager))
}

func (s *GeneratedAPIServer) GetUserOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", GetOrderByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ProcessPayment(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", ProcessPayment(s.db, s.pluginManager, s.mediaService))
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

func (s *GeneratedAPIServer) GetProduct(c *gin.Context, id int) {
	_ = id
	s.applyDraftPreview(c)
	GetProductByID(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) GetStorefrontSettings(c *gin.Context) {
	s.applyDraftPreview(c)
	GetStorefrontSettings(s.db, s.mediaService)(c)
}
