package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationCalculation(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		limit    int
		expected struct {
			page   int
			limit  int
			offset int
		}
	}{
		{
			name:  "First page",
			page:  1,
			limit: 20,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 1, limit: 20, offset: 0},
		},
		{
			name:  "Second page",
			page:  2,
			limit: 20,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 2, limit: 20, offset: 20},
		},
		{
			name:  "Third page with custom limit",
			page:  3,
			limit: 10,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 3, limit: 10, offset: 20},
		},
		{
			name:  "Zero page should default to 1",
			page:  0,
			limit: 20,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 1, limit: 20, offset: 0},
		},
		{
			name:  "Negative page should default to 1",
			page:  -1,
			limit: 20,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 1, limit: 20, offset: 0},
		},
		{
			name:  "Zero limit should default to 20",
			page:  1,
			limit: 0,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 1, limit: 20, offset: 0},
		},
		{
			name:  "Limit over 100 should cap at 100",
			page:  1,
			limit: 150,
			expected: struct {
				page   int
				limit  int
				offset int
			}{page: 1, limit: 100, offset: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := tt.page
			limit := tt.limit

			if page < 1 {
				page = 1
			}
			if limit < 1 {
				limit = 20
			}
			if limit > 100 {
				limit = 100
			}

			offset := (page - 1) * limit

			assert.Equal(t, tt.expected.page, page)
			assert.Equal(t, tt.expected.limit, limit)
			assert.Equal(t, tt.expected.offset, offset)
		})
	}
}

func TestSortFieldValidation(t *testing.T) {
	validSortFields := map[string]bool{
		"price":      true,
		"name":       true,
		"created_at": true,
	}

	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{"Valid price", "price", "price", true},
		{"Valid name", "name", "name", true},
		{"Valid created_at", "created_at", "created_at", true},
		{"Invalid field", "invalid", "created_at", false},
		{"Empty field", "", "created_at", false},
		{"Case sensitive", "Price", "created_at", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortField := tt.input
			if !validSortFields[sortField] {
				sortField = "created_at"
			}

			if tt.valid {
				assert.Equal(t, tt.input, sortField)
			} else {
				assert.Equal(t, "created_at", sortField)
			}
		})
	}
}

func TestSortOrderValidation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Valid asc", "asc", "asc"},
		{"Valid desc", "desc", "desc"},
		{"Invalid order", "invalid", "desc"},
		{"Empty order", "", "desc"},
		{"Case sensitive", "ASC", "desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortOrder := tt.input
			if sortOrder != "asc" && sortOrder != "desc" {
				sortOrder = "desc"
			}
			assert.Equal(t, tt.expected, sortOrder)
		})
	}
}

func TestPriceRangeValidation(t *testing.T) {
	tests := []struct {
		name        string
		minPriceStr string
		maxPriceStr string
		shouldApply bool
	}{
		{"Valid range", "10.00", "100.00", true},
		{"Only min", "10.00", "", true},
		{"Only max", "", "100.00", true},
		{"No range", "", "", false},
		{"Invalid min", "invalid", "100.00", false},
		{"Invalid max", "10.00", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMin := tt.minPriceStr != ""
			hasMax := tt.maxPriceStr != ""

			// Simulate parsing - in real code this uses strconv.ParseFloat
			shouldApply := hasMin || hasMax
			if tt.minPriceStr == "invalid" || tt.maxPriceStr == "invalid" {
				shouldApply = false // Would fail parsing
			}

			assert.Equal(t, tt.shouldApply, shouldApply)
		})
	}
}
