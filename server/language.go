package server

import (
	"github.com/gin-gonic/gin"
)

// Possible improvements:
// If AllowedMethods["*"] then Access-Control-Allow-Methods is set to the requested methods
// If AllowedHeaderss["*"] then Access-Control-Allow-Headers is set to the requested headers
// Put some presets in AllowedHeaders
// Put some presets in AccessControlExposeHeaders

// CorsMiddleware provides a configurable CORS implementation.
type LanguageMiddleware struct {
}

func NewLanguageMiddleware() *LanguageMiddleware {
	return &LanguageMiddleware{

	}
}

func (lm *LanguageMiddleware) LanguageMiddlewareFunc(c *gin.Context) {
	//log.Infof("middleware ")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(200)
	}
	return
}

type LanguageInfo struct {
}
