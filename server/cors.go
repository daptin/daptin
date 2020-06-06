package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
)

// Possible improvements:
// If AllowedMethods["*"] then Access-Control-Allow-Methods is set to the requested methods
// If AllowedHeaderss["*"] then Access-Control-Allow-Headers is set to the requested headers
// Put some presets in AllowedHeaders
// Put some presets in AccessControlExposeHeaders

// CorsMiddleware provides a configurable CORS implementation.
type CorsMiddleware struct {
	allowedMethods    map[string]bool
	allowedMethodsCsv string
	allowedHeaders    map[string]bool
	allowedHeadersCsv string

	// Reject non CORS requests if true. See CorsInfo.IsCors.
	RejectNonCorsRequests bool

	// Function executed for every CORS requests to validate the Origin. (Required)
	// Must return true if valid, false if invalid.
	// For instance: simple equality, regexp, DB lookup, ...
	OriginValidator func(origin string, request *http.Request) bool

	// List of allowed HTTP methods. Note that the comparison will be made in
	// uppercase to avoid common mistakes. And that the
	// Access-Control-Allow-Methods response header also uses uppercase.
	// (see CorsInfo.AccessControlRequestMethod)
	AllowedMethods []string

	// List of allowed HTTP Headers. Note that the comparison will be made with
	// noarmalized names (http.CanonicalHeaderKey). And that the response header
	// also uses normalized names.
	// (see CorsInfo.AccessControlRequestHeaders)
	AllowedHeaders []string

	// List of headers used to set the Access-Control-Expose-Headers header.
	AccessControlExposeHeaders []string

	// User to se the Access-Control-Allow-Credentials response header.
	AccessControlAllowCredentials bool

	// Used to set the Access-Control-Max-Age response header, in seconds.
	AccessControlMaxAge int
}

func NewCorsMiddleware() *CorsMiddleware {
	return &CorsMiddleware{
		allowedMethods:                nil,
		allowedMethodsCsv:             "",
		allowedHeaders:                nil,
		allowedHeadersCsv:             "",
		RejectNonCorsRequests:         false,
		OriginValidator:               nil,
		AllowedMethods:                nil,
		AllowedHeaders:                nil,
		AccessControlExposeHeaders:    nil,
		AccessControlAllowCredentials: false,
		AccessControlMaxAge:           0,
	}
}

func (cm *CorsMiddleware) CorsMiddlewareFunc(c *gin.Context) {
	//log.Infof("middleware ")

	c.Header("Access-Control-Allow-Origin", c.Request.Header.Get("Origin"))
	c.Header("Access-Control-Allow-Methods", "POST,GET,DELETE,PUT,OPTIONS,PATCH")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With,Accept-Language,Access-Control-Allow-Origin")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(200)
	}

	return
}

type CorsInfo struct {
	IsCors      bool
	IsPreflight bool
	Origin      string
	OriginUrl   *url.URL

	// The header value is converted to uppercase to avoid common mistakes.
	AccessControlRequestMethod string

	// The header values are normalized with http.CanonicalHeaderKey.
	AccessControlRequestHeaders []string
}
