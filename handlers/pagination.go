package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const maxPageSize = 100

func parsePagination(c *gin.Context, defaultLimit int) (page int, limit int, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	offset = (page - 1) * limit
	return page, limit, offset
}
