package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const sessionCookieName = "session_token"

var (
	ErrAuthTokenMissing = errors.New("authentication token missing")
	ErrAuthTokenInvalid = errors.New("authentication token invalid")
)

type AuthIdentity struct {
	Subject string
	Email   string
	Role    string
}

type CustomClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

func ResolveAuthIdentity(c *gin.Context, secretKey string) (*AuthIdentity, error) {
	authHeader := c.GetHeader("Authorization")
	tokenString := ""
	if authHeader != "" {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if tokenString == "" {
		if cookieToken, err := c.Cookie(sessionCookieName); err == nil {
			tokenString = cookieToken
		}
	}
	if tokenString == "" {
		return nil, ErrAuthTokenMissing
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrAuthTokenInvalid
	}

	var subject, email, role string
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
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
		subject = customClaims.Subject
		email = customClaims.Email
		role = customClaims.Role
	} else {
		return nil, ErrAuthTokenInvalid
	}

	if subject == "" {
		return nil, ErrAuthTokenInvalid
	}

	return &AuthIdentity{
		Subject: subject,
		Email:   email,
		Role:    role,
	}, nil
}

func AuthMiddleware(secretKey string, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't authenticate OPTIONS requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		identity, err := ResolveAuthIdentity(c, secretKey)
		if err != nil {
			if errors.Is(err, ErrAuthTokenMissing) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if requiredRole != "" && identity.Role != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied: insufficient permissions"})
			return
		}

		c.Set("userID", identity.Subject)
		c.Set("userEmail", identity.Email)
		c.Set("userRole", identity.Role)

		c.Next()
	}
}
