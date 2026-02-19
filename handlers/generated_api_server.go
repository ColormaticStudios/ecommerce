package handlers

import (
	"net/http"

	"ecommerce/internal/apicontract"
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
}

// GeneratedAPIServer adapts generated OpenAPI routes to existing handler implementations.
type GeneratedAPIServer struct {
	db                 *gorm.DB
	mediaService       *media.Service
	jwtSecret          string
	disableLocalSignIn bool
	authCookieCfg      AuthCookieConfig
	oidcProvider       string
	oidcClientID       string
	oidcRedirectURI    string
	mediaUploads       http.Handler
}

func NewGeneratedAPIServer(db *gorm.DB, mediaService *media.Service, cfg GeneratedAPIServerConfig) *GeneratedAPIServer {
	return &GeneratedAPIServer{
		db:                 db,
		mediaService:       mediaService,
		jwtSecret:          cfg.JWTSecret,
		disableLocalSignIn: cfg.DisableLocalSignIn,
		authCookieCfg:      cfg.AuthCookieConfig,
		oidcProvider:       cfg.OIDCProvider,
		oidcClientID:       cfg.OIDCClientID,
		oidcRedirectURI:    cfg.OIDCRedirectURI,
		mediaUploads:       cfg.MediaUploads,
	}
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

func (s *GeneratedAPIServer) ListAdminOrders(c *gin.Context, params apicontract.ListAdminOrdersParams) {
	_ = params
	s.runProtected(c, "admin", GetAllOrders(s.db, s.mediaService))
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

func (s *GeneratedAPIServer) GetAdminStorefrontSettings(c *gin.Context) {
	s.runProtected(c, "admin", GetAdminStorefrontSettings(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) UpdateStorefrontSettings(c *gin.Context) {
	s.runProtected(c, "admin", UpsertStorefrontSettings(s.db, s.mediaService))
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

func (s *GeneratedAPIServer) GetUserOrder(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", GetOrderByID(s.db, s.mediaService))
}

func (s *GeneratedAPIServer) ProcessPayment(c *gin.Context, id int) {
	_ = id
	s.runProtected(c, "", ProcessPayment(s.db, s.mediaService))
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
	GetProducts(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) GetProduct(c *gin.Context, id int) {
	_ = id
	GetProductByID(s.db, s.mediaService)(c)
}

func (s *GeneratedAPIServer) GetStorefrontSettings(c *gin.Context) {
	GetStorefrontSettings(s.db, s.mediaService)(c)
}
