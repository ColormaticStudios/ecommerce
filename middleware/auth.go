package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

func AuthMiddleware(secretKey string, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't authenticate OPTIONS requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// 1. Extract the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is missing"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Parse and validate the token (try MapClaims first, then CustomClaims)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			// Ensure the signing method is what we expect (e.g., HS256)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secretKey), nil
		})

		// 3. Handle validation errors
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// 4. Extract claims (support both MapClaims and CustomClaims)
		var subject, email, role string

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// MapClaims format (from our auth handlers)
			if sub, ok := claims["sub"].(string); ok {
				subject = sub
			}
			if em, ok := claims["email"].(string); ok {
				email = em
			}
			if r, ok := claims["role"].(string); ok {
				role = r
			}
		} else if customClaims, ok := token.Claims.(*CustomClaims); ok {
			// CustomClaims format (for OIDC compatibility)
			subject = customClaims.Subject
			email = customClaims.Email
			role = customClaims.Role
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		if subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: missing subject"})
			return
		}

		// 5. Check role requirement
		if requiredRole != "" && role != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied: insufficient permissions"})
			return
		}

		// 6. Store user data in Gin context for handlers to use
		c.Set("userID", subject)
		c.Set("userEmail", email)
		c.Set("userRole", role)

		c.Next()
	}
}
