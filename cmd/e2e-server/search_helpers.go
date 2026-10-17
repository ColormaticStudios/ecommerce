package main

import (
	"ecommerce/internal/httpapi"
	"ecommerce/internal/search"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

// This route exists only in the dedicated E2E binary. It simulates an indexed
// query failure while leaving canonical catalog browsing available.
func registerSearchTestHelpers(routes *gin.RouterGroup, db *gorm.DB, endpoints *httpapi.CatalogEndpoints) {
	routes.POST("/search-index-availability", func(c *gin.Context) {
		var body struct {
			Available *bool `json:"available"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Available == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "available boolean is required"})
			return
		}
		from, to := "search_product_ranks", "search_product_ranks_e2e_paused"
		if *body.Available {
			from, to = to, from
		}
		if db.Migrator().HasTable(from) {
			if err := db.WithContext(c.Request.Context()).Exec("ALTER TABLE " + from + " RENAME TO " + to).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		config := search.DefaultHardeningConfig()
		config.CircuitFailureThreshold = 1
		if err := endpoints.ConfigureSearchHardening(config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if *body.Available {
			if err := endpoints.ReindexSearchNow(c.Request.Context()); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"available": *body.Available})
	})
}
