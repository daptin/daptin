package server

import (
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"net/http"
)

func CreateFaviconEndpoint(boxRoot http.FileSystem) gin.HandlerFunc {
	return func(c *gin.Context) {
		format := c.Param("format")
		if format != "ico" && format != "png" {
			c.AbortWithStatus(404)
			return
		}

		// Set aggressive caching headers
		c.Header("Cache-Control", "public, max-age=31536000, immutable") // 1 year
		c.Header("Pragma", "public")

		// Try to get file from primary location
		var file http.File
		var err error
		var contentType string

		if format == "ico" {
			file, err = boxRoot.Open("static/img/favicon.ico")
			contentType = "image/x-icon"
			if err != nil {
				// Try fallback location for .ico
				file, err = boxRoot.Open("favicon.ico")
				if err != nil {
					c.AbortWithStatus(404)
					return
				}
			}
		} else { // png
			file, err = boxRoot.Open("static/img/favicon.png")
			contentType = "image/png"
			if err != nil {
				c.AbortWithStatus(404)
				return
			}
		}

		// Get file info for consistent caching
		fileInfo, err := file.Stat()
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		// Check client cache first (consistent with other handlers)
		if checkClientCache(c, fileInfo) {
			return
		}

		// Read file content with size limit protection
		fileContents, err := readFileWithLimit(file, DefaultFileServingConfig.MaxMemoryReadSize)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		// Set optimal cache headers (already includes ETag, Last-Modified)
		setOptimalCacheHeaders(c, fileInfo, DefaultFileServingConfig)

		// Set content type based on format
		c.Header("Content-Type", contentType)

		// Write response
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favicon."+format)
	}
}
