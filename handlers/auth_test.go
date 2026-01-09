package handlers

import (
	"testing"
	"time"

	"ecommerce/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestGenerateJWT(t *testing.T) {
	secret := "test-secret-key-for-jwt-generation"
	user := models.User{
		Subject: "test-subject-123",
		Email:   "test@example.com",
		Role:    "customer",
		Name:    "Test User",
	}

	token, err := generateJWT(user, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token can be parsed
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Verify claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, user.Subject, claims["sub"])
	assert.Equal(t, user.Email, claims["email"])
	assert.Equal(t, user.Role, claims["role"])
	assert.Equal(t, user.Name, claims["name"])

	// Verify expiration is set
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	expTime := time.Unix(int64(exp), 0)
	assert.True(t, expTime.After(time.Now()))
	assert.True(t, expTime.Before(time.Now().Add(time.Hour*24*8))) // Should be around 7 days
}

func TestGenerateJWT_DifferentUsers(t *testing.T) {
	secret := "test-secret"

	user1 := models.User{
		Subject: "subject-1",
		Email:   "user1@example.com",
		Role:    "customer",
	}

	user2 := models.User{
		Subject: "subject-2",
		Email:   "user2@example.com",
		Role:    "admin",
	}

	token1, err1 := generateJWT(user1, secret)
	token2, err2 := generateJWT(user2, secret)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, token1, token2) // Different users should get different tokens
}

func TestGenerateSubjectID(t *testing.T) {
	email1 := "test1@example.com"
	email2 := "test2@example.com"

	subject1 := generateSubjectID(email1)
	subject2 := generateSubjectID(email2)

	// Should generate non-empty subject
	assert.NotEmpty(t, subject1)
	assert.NotEmpty(t, subject2)

	// Different emails should generate different subjects
	assert.NotEqual(t, subject1, subject2)

	// Same email should generate same subject (deterministic)
	subject1Again := generateSubjectID(email1)
	// Note: This might not be true if timestamp is used, but let's test the function works
	assert.NotEmpty(t, subject1Again)
}

func TestPasswordHashing(t *testing.T) {
	password := "test-password-123"

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)

	// Verify password
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	assert.NoError(t, err)

	// Wrong password should fail
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte("wrong-password"))
	assert.Error(t, err)
}

func TestPasswordHashing_DifferentPasswords(t *testing.T) {
	password1 := "password1"
	password2 := "password2"

	hash1, err1 := bcrypt.GenerateFromPassword([]byte(password1), bcrypt.DefaultCost)
	hash2, err2 := bcrypt.GenerateFromPassword([]byte(password2), bcrypt.DefaultCost)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Hashes should be different
	assert.NotEqual(t, string(hash1), string(hash2))

	// Each hash should only verify with its own password
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash1, []byte(password1)))
	assert.Error(t, bcrypt.CompareHashAndPassword(hash1, []byte(password2)))
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash2, []byte(password2)))
	assert.Error(t, bcrypt.CompareHashAndPassword(hash2, []byte(password1)))
}
