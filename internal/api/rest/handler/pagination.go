package handler

import (
	"math"

	"github.com/gin-gonic/gin"
)

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Page     int
	PageSize int
}

// Offset returns the database offset
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Pagination defaults and bounds.
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ParsePagination parses pagination parameters from context. It responds 400
// and returns false if either parameter is not an integer or is out of range;
// invalid values used to be coerced to the defaults, so a client sending
// page=abc or page_size=1000 got a 200 for a page it never asked for.
func ParsePagination(c *gin.Context) (PaginationParams, bool) {
	page, ok := parseIntQuery(c, queryPage, DefaultPage, 1, math.MaxInt32)
	if !ok {
		return PaginationParams{}, false
	}

	pageSize, ok := parseIntQuery(c, queryPageSize, DefaultPageSize, 1, MaxPageSize)
	if !ok {
		return PaginationParams{}, false
	}

	return PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, true
}

// NewPaginationResponse creates a standardized pagination response
func NewPaginationResponse(data any, params PaginationParams, total int64) gin.H {
	totalPages := (int(total) + params.PageSize - 1) / params.PageSize

	return gin.H{
		"data": data,
		"pagination": gin.H{
			"page":        params.Page,
			"page_size":   params.PageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	}
}
