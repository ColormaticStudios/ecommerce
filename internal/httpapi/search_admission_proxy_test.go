package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSearchClientMetadataTrustsOnlyConfiguredProxyPeers(t *testing.T) {
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies([]string{"192.0.2.10/32"}))
	router.Use(RequestContextMiddleware(RequestContextOptions{}))
	router.GET("/client", func(c *gin.Context) {
		metadata, ok := requestctx.MetadataFrom(c.Request.Context())
		require.True(t, ok)
		c.String(http.StatusOK, metadata.ClientIP)
	})
	for _, tc := range []struct{ peer, expected string }{
		{"192.0.2.10:4321", "198.51.100.2"},
		{"203.0.113.4:4321", "203.0.113.4"},
	} {
		request := httptest.NewRequest(http.MethodGet, "/client", nil)
		request.RemoteAddr = tc.peer
		request.Header.Set("X-Forwarded-For", "198.51.100.2")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, tc.expected, response.Body.String())
	}
}

func TestSSRClientsHaveSeparateSearchBudgets(t *testing.T) {
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies([]string{"192.0.2.10/32"}))
	router.Use(RequestContextMiddleware(RequestContextOptions{}))
	config := DefaultSearchAdmissionConfig()
	config.ClientRequestsPerMinute = 2
	endpoints := &CatalogEndpoints{searchClients: newSearchClientLimiter(config)}
	router.GET("/search", func(c *gin.Context) {
		release, err := endpoints.admitSearchClient(c.Request.Context(), false)
		if err != nil {
			c.Status(http.StatusTooManyRequests)
			return
		}
		defer release()
		c.Status(http.StatusOK)
	})
	call := func(client string) int {
		request := httptest.NewRequest(http.MethodGet, "/search", nil)
		request.RemoteAddr = "192.0.2.10:4321"
		request.Header.Set("X-Forwarded-For", client)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}
	for _, client := range []string{"198.51.100.1", "198.51.100.2"} {
		require.Equal(t, http.StatusOK, call(client), "SSR result request")
		require.Equal(t, http.StatusOK, call(client), "SSR suggestions request")
		require.Equal(t, http.StatusTooManyRequests, call(client))
	}
	require.Equal(t, http.StatusOK, call("198.51.100.3"))
	require.Equal(t, http.StatusOK, call("2001:db8:1:2::1"))
	require.Equal(t, http.StatusOK, call("2001:db8:1:2::2"))
	require.Equal(t, http.StatusTooManyRequests, call("2001:db8:1:2::3"), "IPv6 address rotation shares a /64 budget")
	require.Equal(t, http.StatusOK, call("2001:db8:1:3::1"))
}
