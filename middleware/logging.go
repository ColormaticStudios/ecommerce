package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
)

// AccessLogger emits one structured completion event for every request. It
// deliberately reads request context after the handler chain so authentication
// and generated operation metadata are available to the log record.
func AccessLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		requestContext := ctx.Request.Context()
		metadata, _ := requestctx.MetadataFrom(requestContext)
		route := ctx.FullPath()
		if route == "" {
			route = ctx.Request.URL.Path
		}
		attributes := []any{
			"event", "http_request_completed",
			"request_id", metadata.RequestID,
			"correlation_id", metadata.CorrelationID,
			"method", ctx.Request.Method,
			"route", route,
			"status_code", ctx.Writer.Status(),
			"latency_ms", float64(time.Since(startedAt).Microseconds()) / 1000,
			"client_ip", ctx.ClientIP(),
		}
		if metadata.OperationID != "" {
			attributes = append(attributes, "operation_id", metadata.OperationID)
		}
		if principal, ok := requestctx.PrincipalFrom(requestContext); ok {
			attributes = append(attributes, "actor_subject", principal.Subject)
			if principal.AccountID != 0 {
				attributes = append(attributes, "actor_account_id", principal.AccountID)
			}
			if len(principal.Roles) > 0 {
				attributes = append(attributes, "actor_roles", principal.Roles)
			}
			if principal.AuthMethod != "" {
				attributes = append(attributes, "auth_method", principal.AuthMethod)
			}
		}
		if len(ctx.Errors) > 0 {
			attributes = append(attributes, "error_count", len(ctx.Errors))
		}

		switch status := ctx.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			logger.ErrorContext(requestContext, "HTTP request completed", attributes...)
		case status >= http.StatusBadRequest:
			logger.WarnContext(requestContext, "HTTP request completed", attributes...)
		default:
			logger.InfoContext(requestContext, "HTTP request completed", attributes...)
		}
	}
}
