package auth

import (
	"context"
	"fmt"
	"testing"

	accountservice "ecommerce/internal/services/account"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAuthTestService(t *testing.T, disabled bool) (*Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.WebsiteSettings{}))
	website := accountservice.NewService(db, nil)
	return NewService(db, "test-secret", disabled, website), db
}

func TestRegisterAndLogin(t *testing.T) {
	service, _ := newAuthTestService(t, false)
	registered, err := service.Register(context.Background(), RegisterInput{Username: "ada", Email: "ADA@example.com", Password: "secret12", Name: "Ada"})
	require.NoError(t, err)
	assert.NotEmpty(t, registered.Token)
	assert.Empty(t, registered.User.PasswordHash)
	assert.Equal(t, "ada@example.com", registered.User.Email)

	_, err = service.Register(context.Background(), RegisterInput{Username: "other", Email: "ada@example.com", Password: "secret12"})
	require.ErrorIs(t, err, ErrEmailConflict)

	loggedIn, err := service.Login(context.Background(), LoginInput{Email: "ada@example.com", Password: "secret12"})
	require.NoError(t, err)
	assert.Equal(t, registered.User.ID, loggedIn.User.ID)

	_, err = service.Login(context.Background(), LoginInput{Email: "ada@example.com", Password: "wrong"})
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLocalAuthenticationCanBeDisabled(t *testing.T) {
	service, _ := newAuthTestService(t, true)
	config, err := service.Config(context.Background())
	require.NoError(t, err)
	assert.False(t, config.LocalSignInEnabled)

	_, err = service.Register(context.Background(), RegisterInput{Username: "ada", Email: "ada@example.com", Password: "secret12"})
	require.ErrorIs(t, err, ErrLocalSignInDisabled)
	_, err = service.Login(context.Background(), LoginInput{Email: "ada@example.com", Password: "secret12"})
	require.ErrorIs(t, err, ErrLocalSignInDisabled)
}

func TestAuthServiceUsesCallerContext(t *testing.T) {
	service, _ := newAuthTestService(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Register(ctx, RegisterInput{Username: "ada", Email: "ada@example.com", Password: "secret12"})
	require.ErrorIs(t, err, context.Canceled)
}
