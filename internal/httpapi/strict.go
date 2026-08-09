package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
)

type StrictOptions struct {
	Policies    PolicySet
	Renderer    Renderer
	Middlewares []apicontract.StrictMiddlewareFunc
}

// ComposeStrict validates policy completeness before exposing the generated
// non-strict adapter required by generated Gin registration.
func ComposeStrict(server apicontract.StrictServerInterface, options StrictOptions) (apicontract.ServerInterface, error) {
	if interfaceIsNil(server) {
		return nil, errors.New("strict server is required")
	}
	if err := options.Policies.ValidateContract(); err != nil {
		return nil, fmt.Errorf("validate strict operation policies: %w", err)
	}
	middlewares := append([]apicontract.StrictMiddlewareFunc{}, options.Middlewares...)
	if checkoutServer, ok := server.(interface {
		CheckoutSessionMiddleware() apicontract.StrictMiddlewareFunc
	}); ok {
		middlewares = append(middlewares, checkoutServer.CheckoutSessionMiddleware())
	}
	middlewares = append(middlewares, StrictPolicyMiddleware(options.Policies), StrictErrorMiddleware(options.Renderer))
	return apicontract.NewStrictHandler(server, middlewares), nil
}

// StrictPolicyMiddleware records generated operation metadata and enforces the
// principal/role portion of a policy. Per-operation body limits and CSRF must be
// enforced before generated body binding; they are deliberately not simulated
// here.
func StrictErrorMiddleware(renderer Renderer) apicontract.StrictMiddlewareFunc {
	return func(next apicontract.StrictHandlerFunc, _ string) apicontract.StrictHandlerFunc {
		return func(ctx *gin.Context, request interface{}) (interface{}, error) {
			response, err := next(ctx, request)
			if err == nil {
				return response, nil
			}
			status := http.StatusInternalServerError
			var typed *ProblemError
			if errors.As(err, &typed) && typed.Problem.Status >= 400 && typed.Problem.Status <= 599 {
				status = typed.Problem.Status
			}
			renderer.Render(ctx.Writer, ctx.Request.Context(), status, err)
			ctx.Abort()
			return nil, nil
		}
	}
}

func StrictPolicyMiddleware(set PolicySet) apicontract.StrictMiddlewareFunc {
	return func(next apicontract.StrictHandlerFunc, operationID string) apicontract.StrictHandlerFunc {
		return func(ctx *gin.Context, request interface{}) (interface{}, error) {
			policy, ok := set.Lookup(operationID)
			if !ok {
				return nil, ErrorProblem(Problem{
					Type:   TypeInternal,
					Title:  "Internal Server Error",
					Status: 500,
					Code:   "operation_policy_missing",
					Detail: "The operation is not configured.",
				}, fmt.Errorf("missing policy for generated operation %q", operationID))
			}

			requestContext := requestctx.WithOperation(ctx.Request.Context(), operationID)
			ctx.Request = ctx.Request.WithContext(requestContext)
			if err := authorizePolicy(requestContext, policy); err != nil {
				return nil, err
			}
			return next(ctx, request)
		}
	}
}

type RegisterStrictOptions struct {
	Strict              StrictOptions
	Renderer            Renderer
	RequestContext      RequestContextOptions
	Security            SecurityOptions
	BaseURL             string
	GeneratedMiddleware []apicontract.MiddlewareFunc
}

// RegisterStrict installs request metadata, recovery/error rendering, generated
// binding handling, and generated strict composition as one boundary. Call it on
// a dedicated router group if legacy routes must retain legacy middleware.
func RegisterStrict(router *gin.Engine, server apicontract.StrictServerInterface, options RegisterStrictOptions) error {
	if router == nil {
		return errors.New("strict router is required")
	}
	// oapi-codegen's Gin strict adapter passes *gin.Context as context.Context.
	// Delegate Value lookups to Request.Context so requestctx metadata and the
	// authenticated principal remain visible to transport-neutral endpoints.
	router.ContextWithFallback = true
	strictOptions := options.Strict
	strictOptions.Renderer = options.Renderer
	composed, err := ComposeStrict(server, strictOptions)
	if err != nil {
		return err
	}
	securityOptions := options.Security
	securityOptions.BaseURL = options.BaseURL
	security, err := OperationSecurityMiddleware(options.Strict.Policies, options.Renderer, securityOptions)
	if err != nil {
		return fmt.Errorf("initialize strict operation security: %w", err)
	}
	generatedMiddleware := append([]apicontract.MiddlewareFunc(nil), options.GeneratedMiddleware...)
	if providerServer, ok := server.(interface {
		ProviderBindingMiddleware() apicontract.MiddlewareFunc
	}); ok {
		generatedMiddleware = append(generatedMiddleware, providerServer.ProviderBindingMiddleware())
	}
	router.Use(RequestContextMiddleware(options.RequestContext), Boundary(options.Renderer), security)
	apicontract.RegisterHandlersWithOptions(router, composed, apicontract.GinServerOptions{
		BaseURL:      options.BaseURL,
		Middlewares:  generatedMiddleware,
		ErrorHandler: GeneratedBindingErrorHandler(options.Renderer),
	})
	return nil
}
