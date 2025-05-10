package server

import (
	"github.com/daptin/daptin/server/database"
	"github.com/gin-gonic/gin"
	"net/http"
)

func CreateStatisticsHandler(db database.DatabaseConnection) func(*gin.Context) {
	return func(c *gin.Context) {
		stats := make(map[string]interface{})
		stats["web"] = Stats.Data()
		stats["db"] = db.Stats()
		stats["cpu"] = nil     // TODO
		stats["disk"] = nil    // TODO
		stats["network"] = nil // TODO
		c.JSON(http.StatusOK, stats)
	}
}
