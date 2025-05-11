package server

import (
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"time"
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

		// Read file content
		fileContents, err := io.ReadAll(file)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		// Generate ETag for better caching
		fileInfo, _ := file.Stat()
		etag := generateETag(fileContents, fileInfo.ModTime())
		c.Header("ETag", etag)

		// Check if client has this version cached
		if match := c.Request.Header.Get("If-None-Match"); match != "" && match == etag {
			c.AbortWithStatus(http.StatusNotModified) // 304
			return
		}

		// Set content type based on format
		c.Header("Content-Type", contentType)

		// Set last modified
		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

		// Write response
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favicon."+format)
	}
}
