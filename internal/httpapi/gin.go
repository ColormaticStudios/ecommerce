package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
)

// PrincipalResolver adapts transport authentication state to requestctx.
type PrincipalResolver func(*gin.Context) (requestctx.Principal, bool)

type RequestContextOptions struct {
	NewID            func() string
	TrustRequestIDs  bool
	ResolvePrincipal PrincipalResolver
}

func RequestContextMiddleware(options RequestContextOptions) gin.HandlerFunc {
	newID := options.NewID
	if newID == nil {
		newID = func() string { return uuid.NewString() }
	}
	return func(ctx *gin.Context) {
		requestID := ""
		correlationID := ""
		if options.TrustRequestIDs {
			requestID = validExternalID(ctx.GetHeader(RequestIDHeader))
			correlationID = validExternalID(ctx.GetHeader(CorrelationIDHeader))
		}
		if requestID == "" {
			requestID = newID()
		}
		if correlationID == "" {
			correlationID = requestID
		}

		cookies := make(map[string]string)
		for _, cookie := range ctx.Request.Cookies() {
			cookies[cookie.Name] = cookie.Value
		}
		headers := make(map[string]string, len(ctx.Request.Header))
		for name := range ctx.Request.Header {
			headers[name] = ctx.Request.Header.Get(name)
		}
		requestContext := requestctx.WithMetadata(ctx.Request.Context(), requestctx.Metadata{
			RequestID:     requestID,
			CorrelationID: correlationID,
			Method:        ctx.Request.Method,
			Path:          ctx.Request.URL.Path,
			StartedAt:     time.Now().UTC(),
			Cookies:       cookies,
			Headers:       headers,
		})
		if options.ResolvePrincipal != nil {
			if principal, ok := options.ResolvePrincipal(ctx); ok {
				requestContext = requestctx.WithPrincipal(requestContext, principal)
			}
		}
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header(RequestIDHeader, requestID)
		ctx.Header(CorrelationIDHeader, correlationID)
		ctx.Next()
	}
}

// LegacyGinPrincipal bridges the keys populated by the current auth middleware.
// It can be removed after authentication writes requestctx.Principal directly.
func LegacyGinPrincipal(ctx *gin.Context) (requestctx.Principal, bool) {
	subject := ctx.GetString("userID")
	if subject == "" {
		return requestctx.Principal{}, false
	}
	principal := requestctx.Principal{
		Subject:    subject,
		Email:      ctx.GetString("userEmail"),
		AuthMethod: "legacy-gin",
	}
	if role := ctx.GetString("userRole"); role != "" {
		principal.Roles = []string{role}
	}
	return principal, true
}

// Boundary converts errors left by generated strict binding/dispatch and panics
// into one problem response. It must be registered before generated routes.
func Boundary(renderer Renderer) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := errors.New("panic in HTTP handler")
				if ctx.Writer.Written() {
					renderer.FromError(ctx.Request.Context(), http.StatusInternalServerError, err)
					ctx.Abort()
					return
				}
				renderer.Render(ctx.Writer, ctx.Request.Context(), http.StatusInternalServerError, err)
				ctx.Abort()
			}
		}()

		ctx.Next()
		if len(ctx.Errors) == 0 || ctx.Writer.Written() {
			return
		}
		status := ctx.Writer.Status()
		if status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}
		renderer.Render(ctx.Writer, ctx.Request.Context(), status, ctx.Errors.Last().Err)
		ctx.Abort()
	}
}

// GeneratedBindingErrorHandler handles path/query/header binding failures from
// RegisterHandlersWithOptions. Strict JSON body errors are handled by Boundary.
func GeneratedBindingErrorHandler(renderer Renderer) func(*gin.Context, error, int) {
	return func(ctx *gin.Context, err error, status int) {
		if status == http.StatusBadRequest && strings.Contains(err.Error(), "Idempotency-Key") {
			err = problemError(status, "invalid_request", err.Error(), err)
		}
		renderer.Render(ctx.Writer, ctx.Request.Context(), status, err)
		ctx.Abort()
	}
}

func validExternalID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ""
		}
	}
	return value
}
