package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T, migrateModels ...any) *gorm.DB {
	t.Helper()

	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(migrateModels...))
	return db
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{})

	r := gin.New()
	r.POST("/api/v1/auth/register", Register(db, "test-secret", AuthCookieConfig{}))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"username":"new-user","password":"supersecret"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body["error"])

	var count int64
	require.NoError(t, db.Model(&models.User{}).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestUpdateUserRoleRejectsUnsupportedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{})

	user := models.User{
		Subject:  "sub-1",
		Username: "alice",
		Email:    "alice@example.com",
		Role:     "customer",
		Currency: "USD",
	}
	require.NoError(t, db.Create(&user).Error)

	r := gin.New()
	r.PUT("/api/v1/admin/users/:id/role", UpdateUserRole(db))

	req := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/v1/admin/users/%d/role", user.ID),
		strings.NewReader(`{"role":"superadmin"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var reloaded models.User
	require.NoError(t, db.First(&reloaded, user.ID).Error)
	assert.Equal(t, "customer", reloaded.Role)
}
