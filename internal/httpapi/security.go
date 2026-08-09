package httpapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidAuthenticationToken = errors.New("invalid authentication token")

// Authenticator resolves optional credentials. present is false only when the
// request supplied no credential at all; malformed or expired credentials
// return present=true and an error.
type Authenticator interface {
	Authenticate(*http.Request) (principal requestctx.Principal, present bool, err error)
}

type JWTAuthenticator struct {
	Secret           []byte
	CookieName       string
	ResolveAccountID func(context.Context, string) (uint, error)
}

func (auth JWTAuthenticator) Authenticate(request *http.Request) (requestctx.Principal, bool, error) {
	tokenText, method := bearerToken(request)
	if tokenText == "" {
		cookieName := auth.CookieName
		if cookieName == "" {
			cookieName = "session_token"
		}
		cookie, err := request.Cookie(cookieName)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				return requestctx.Principal{}, false, nil
			}
			return requestctx.Principal{}, true, ErrInvalidAuthenticationToken
		}
		tokenText = cookie.Value
		method = "cookie"
	}
	if tokenText == "" || len(auth.Secret) == 0 {
		return requestctx.Principal{}, true, ErrInvalidAuthenticationToken
	}

	token, err := jwt.Parse(tokenText, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidAuthenticationToken
		}
		return auth.Secret, nil
	}, jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Alg(),
		jwt.SigningMethodHS384.Alg(),
		jwt.SigningMethodHS512.Alg(),
	}))
	if err != nil || !token.Valid {
		return requestctx.Principal{}, true, ErrInvalidAuthenticationToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return requestctx.Principal{}, true, ErrInvalidAuthenticationToken
	}
	subject, _ := claims["sub"].(string)
	if subject == "" {
		return requestctx.Principal{}, true, ErrInvalidAuthenticationToken
	}
	principal := requestctx.Principal{Subject: subject, AuthMethod: method}
	principal.Email, _ = claims["email"].(string)
	if role, _ := claims["role"].(string); role != "" {
		principal.Roles = []string{role}
	}
	if auth.ResolveAccountID != nil {
		accountID, err := auth.ResolveAccountID(request.Context(), subject)
		if err != nil {
			return requestctx.Principal{}, true, err
		}
		principal.AccountID = accountID
	}
	return principal, true, nil
}

type SecurityOptions struct {
	Authenticator  Authenticator
	CSRFCookieName string
	CSRFHeaderName string
	BaseURL        string
	PreviewSecret  string
}

// OperationSecurityMiddleware executes before generated binding. It applies the
// operation body limit, resolves optional credentials, authorizes access/roles,
// and enforces CSRF for cookie-based unsafe requests.
func OperationSecurityMiddleware(set PolicySet, renderer Renderer, options SecurityOptions) (gin.HandlerFunc, error) {
	routes, err := contractOperationRoutes(options.BaseURL)
	if err != nil {
		return nil, err
	}
	csrfCookieName := options.CSRFCookieName
	if csrfCookieName == "" {
		csrfCookieName = "csrf_token"
	}
	csrfHeaderName := options.CSRFHeaderName
	if csrfHeaderName == "" {
		csrfHeaderName = "X-CSRF-Token"
	}

	return func(ctx *gin.Context) {
		routePath := ctx.FullPath()
		if routePath == "" {
			routePath = ctx.Request.URL.Path
		}
		operationID, ok := routes[routeKey{method: ctx.Request.Method, path: routePath}]
		if !ok {
			renderer.Render(ctx.Writer, ctx.Request.Context(), http.StatusInternalServerError, ErrorProblem(Problem{
				Type: TypeInternal, Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError,
				Code: "operation_route_missing", Detail: "The operation route is not configured.",
			}, fmt.Errorf("generated route has no OpenAPI operation: %s %s", ctx.Request.Method, routePath)))
			ctx.Abort()
			return
		}
		policy, ok := set.Lookup(operationID)
		if !ok {
			renderer.Render(ctx.Writer, ctx.Request.Context(), http.StatusInternalServerError, ErrorProblem(Problem{
				Type: TypeInternal, Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError,
				Code: "operation_policy_missing", Detail: fmt.Sprintf("The operation policy for %s is not configured.", operationID),
			}, fmt.Errorf("missing operation policy for %q", operationID)))
			ctx.Abort()
			return
		}

		requestContext := requestctx.WithOperation(ctx.Request.Context(), operationID)
		if options.PreviewSecret != "" {
			metadata, _ := requestctx.MetadataFrom(requestContext)
			if token := metadata.Cookies[draftPreviewCookieName]; token != "" {
				metadata.DraftPreview = validDraftPreviewToken(token, options.PreviewSecret)
				requestContext = requestctx.WithMetadata(requestContext, metadata)
			}
		}
		ctx.Request = ctx.Request.WithContext(requestContext)
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, policy.MaxBodyBytes)
		}
		if operationID == "ReceiveWebhookEvent" && ctx.Request.Body != nil {
			rawBody, readErr := io.ReadAll(ctx.Request.Body)
			if readErr != nil {
				renderer.Render(ctx.Writer, requestContext, http.StatusBadRequest, readErr)
				ctx.Abort()
				return
			}
			ctx.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
			metadata, _ := requestctx.MetadataFrom(requestContext)
			metadata.RawBody = rawBody
			requestContext = requestctx.WithMetadata(requestContext, metadata)
			ctx.Request = ctx.Request.WithContext(requestContext)
		}

		if options.Authenticator != nil {
			principal, present, authErr := options.Authenticator.Authenticate(ctx.Request)
			if authErr != nil {
				renderer.Render(ctx.Writer, requestContext, http.StatusUnauthorized, ErrorProblem(Problem{
					Type: TypeAuthenticationRequired, Title: http.StatusText(http.StatusUnauthorized),
					Status: http.StatusUnauthorized, Code: "authentication_invalid", Detail: "The authentication credential is invalid or expired.",
				}, authErr))
				ctx.Abort()
				return
			}
			if present {
				requestContext = requestctx.WithPrincipal(requestContext, principal)
				ctx.Request = ctx.Request.WithContext(requestContext)
			}
		}
		if err := authorizePolicy(requestContext, policy); err != nil {
			renderer.Render(ctx.Writer, requestContext, http.StatusInternalServerError, err)
			ctx.Abort()
			return
		}
		if requiresOperationCSRF(ctx.Request, policy) {
			cookie, cookieErr := ctx.Request.Cookie(csrfCookieName)
			header := ctx.GetHeader(csrfHeaderName)
			if cookieErr != nil || cookie.Value == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				renderer.Render(ctx.Writer, requestContext, http.StatusForbidden, ErrorProblem(Problem{
					Type: TypeForbidden, Title: http.StatusText(http.StatusForbidden), Status: http.StatusForbidden,
					Code: "csrf_failed", Detail: "The CSRF token is missing or invalid.",
				}, errors.New("CSRF validation failed")))
				ctx.Abort()
				return
			}
		}
		ctx.Next()
	}, nil
}

func bearerToken(request *http.Request) (string, string) {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if header == "" {
		return "", ""
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", "bearer"
	}
	return strings.TrimSpace(token), "bearer"
}

func requiresCSRF(request *http.Request) bool {
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	token, method := bearerToken(request)
	return method != "bearer" || token == ""
}

func requiresOperationCSRF(request *http.Request, policy OperationPolicy) bool {
	if !requiresCSRF(request) {
		return false
	}
	if policy.CSRF == CSRFRequired {
		return true
	}
	// Public guest-checkout mutations allow the first request to establish a
	// session, then require double-submit CSRF protection on every continuation.
	cookie, err := request.Cookie(checkoutSessionCookieName)
	return err == nil && cookie.Value != "" && strings.HasPrefix(request.URL.Path, "/api/v1/checkout/")
}

type routeKey struct {
	method string
	path   string
}

func contractOperationRoutes(baseURL string) (map[routeKey]string, error) {
	spec, err := apicontract.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load generated OpenAPI contract: %w", err)
	}
	routes := make(map[routeKey]string)
	baseURL = strings.TrimRight(baseURL, "/")
	for path, item := range spec.Paths.Map() {
		ginPath := baseURL + openAPIPathToGin(path)
		for method, operation := range item.Operations() {
			key := routeKey{method: strings.ToUpper(method), path: ginPath}
			if previous, duplicate := routes[key]; duplicate {
				return nil, fmt.Errorf("duplicate operation route %s %s (%s and %s)", key.method, key.path, previous, operation.OperationID)
			}
			routes[key] = operation.OperationID
		}
	}
	return routes, nil
}

func openAPIPathToGin(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[index] = ":" + strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		}
	}
	return strings.Join(parts, "/")
}
