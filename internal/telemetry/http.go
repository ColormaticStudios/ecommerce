package telemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func HTTPMiddleware(metrics *Metrics, serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName + "/http")
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		requestContext := otel.GetTextMapPropagator().Extract(ctx.Request.Context(), propagation.HeaderCarrier(ctx.Request.Header))
		requestContext, span := tracer.Start(requestContext, ctx.Request.Method+" request", trace.WithSpanKind(trace.SpanKindServer))
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()

		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := ctx.Writer.Status()
		metrics.ObserveHTTPRequest(ctx.Request.Method, route, status, time.Since(startedAt))
		span.SetName(ctx.Request.Method + " " + route)
		span.SetAttributes(
			attribute.String("http.request.method", ctx.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		)
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
		}
		span.End()
	}
}
