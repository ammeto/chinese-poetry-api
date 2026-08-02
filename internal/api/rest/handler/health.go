package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/palemoky/chinese-poetry-api/internal/database"
)

// HealthHandler handles health check requests
func HealthHandler(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check database connection
		sqlDB, err := db.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "failed to get database connection",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	}
}

// StatsHandler returns overall statistics
//
// Takes no query parameters - notably not lang, since GetStatistics counts rows
// that are the same in both variants. /health is deliberately left permissive
// instead: probes routinely append cache-busting params, and rejecting those
// would turn a healthy service into a failing health check.
func StatsHandler(repo *database.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkQueryParams(c) {
			return
		}

		stats, err := repo.GetStatistics()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to get statistics",
			})
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}
