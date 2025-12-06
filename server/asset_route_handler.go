package server

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/cache"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func AssetRouteHandler(cruds map[string]*resource.DbResource) func(c *gin.Context) {
	return func(c *gin.Context) {
		typeName := c.Param("typename")
		resourceUuid := c.Param("resource_id")
		columnNameWithExt := c.Param("columnname")
		columnNameWithoutExt := columnNameWithExt

		if strings.Index(columnNameWithoutExt, ".") > -1 {
			columnNameWithoutExt = columnNameWithoutExt[:strings.LastIndex(columnNameWithoutExt, ".")]
		}

		// Generate a cache key for this request
		cacheKey := fmt.Sprintf("%s:%s:%s:%s:%s",
			typeName,
			resourceUuid,
			columnNameWithoutExt,
			c.Query("index"),
			c.Query("file"))

		// Check if we have a cached file for this request
		if cachedFile, found := fileCache.Get(cacheKey); found {
			// Check if client has fresh copy using ETag
			if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == cachedFile.ETag {
				c.Header("Cache-Control", "public, max-age=31536000") // 1 year for 304 responses
				c.Header("ETag", cachedFile.ETag)
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Set basic headers from cache
			c.Header("Content-Type", cachedFile.MimeType)
			c.Header("ETag", cachedFile.ETag)

			// Set cache control based on expiry time
			maxAge := int(time.Until(cachedFile.ExpiresAt).Seconds())
			if maxAge <= 0 {
				maxAge = 60 // Minimum 1 minute for almost expired resources
			}
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))

			// Add content disposition if needed
			if cachedFile.IsDownload {
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(cachedFile.Path)))
			} else {
				c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", filepath.Base(cachedFile.Path)))
			}

			// Check if client accepts gzip and we have compressed data
			if cachedFile.GzipData != nil && len(cachedFile.GzipData) > 0 && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
				c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.GzipData)
				return
			}

			// Serve uncompressed data
			c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.Data)
			return
		}

		// Parse column name and extension
		//parts := strings.SplitN(columnNameWithExt, ".", 2)
		//if len(parts) == 0 {
		//	c.AbortWithStatus(http.StatusBadRequest)
		//	return
		//}
		columnName := columnNameWithoutExt

		// Fast path: check if the table exists
		table, ok := cruds[typeName]
		if !ok || table == nil {
			log.Errorf("table not found [%v]", typeName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Fast path: check if the column exists
		colInfo, ok := table.TableInfo().GetColumnByName(columnName)
		if !ok || colInfo == nil || (!colInfo.IsForeignKey && colInfo.ColumnType != "markdown") {
			log.Errorf("column [%v] info not found", columnName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Handle markdown directly (simple case)
		if colInfo.ColumnType == "markdown" {
			// Fetch data
			pr := &http.Request{
				Method: "GET",
				URL:    c.Request.URL,
			}
			pr = pr.WithContext(c.Request.Context())

			req := api2go.Request{
				PlainRequest: pr,
			}

			obj, err := cruds[typeName].FindOne(resourceUuid, req)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			row := obj.Result().(api2go.Api2GoModel)
			colData := row.GetAttributes()[columnName]
			if colData == nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			markdownContent := colData.(string)

			// Generate ETag
			etag := cache.GenerateETag([]byte(markdownContent), time.Now())

			// Check if client has fresh copy
			if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
				c.Header("ETag", etag)
				c.Header("Cache-Control", "public, max-age=86400") // 1 day
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Cache the markdown content
			htmlContent := fmt.Sprintf("<pre>%s</pre>", markdownContent)
			cachedMarkdown := &cache.CachedFile{
				Data:       []byte(htmlContent),
				ETag:       etag,
				Modtime:    time.Now(),
				MimeType:   "text/html; charset=utf-8",
				Size:       len(htmlContent),
				Path:       fmt.Sprintf("%s/%s/%s", typeName, resourceUuid, columnNameWithExt),
				IsDownload: false,
				ExpiresAt:  cache.CalculateExpiry("text/html", ""),
			}

			// Create compressed version if large enough
			if len(htmlContent) > cache.CompressionThreshold {
				if compressedData, err := cache.CompressData([]byte(htmlContent)); err == nil {
					cachedMarkdown.GzipData = compressedData
				}
			}

			fileCache.Set(cacheKey, cachedMarkdown)

			// Return markdown as HTML with appropriate headers
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(time.Until(cachedMarkdown.ExpiresAt).Seconds())))
			c.Header("ETag", etag)

			// Use compression if client accepts it and we have compressed data
			if cachedMarkdown.GzipData != nil && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
				c.Data(http.StatusOK, "text/html; charset=utf-8", cachedMarkdown.GzipData)
				return
			}

			c.Data(http.StatusOK, "text/html; charset=utf-8", cachedMarkdown.Data)
			return
		}

		if !colInfo.IsForeignKey {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Get cache for this path
		assetCache, ok := cruds["world"].AssetFolderCache[typeName][columnName]
		if !ok {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Find the file to serve
		pr := &http.Request{
			Method: "GET",
			URL:    c.Request.URL,
		}
		pr = pr.WithContext(c.Request.Context())

		req := api2go.Request{
			PlainRequest: pr,
		}

		obj, err := cruds[typeName].FindOne(resourceUuid, req)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		row := obj.Result().(api2go.Api2GoModel)
		colData := row.GetAttributes()[columnName]
		if colData == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Find the correct file
		colDataMapArray := colData.([]map[string]interface{})

		indexByQuery := c.Query("index")
		var indexByQueryInt = -1
		indexByQueryInt, err = strconv.Atoi(indexByQuery)
		if err != nil {
			indexByQueryInt = -1
		}
		nameByQuery := c.Query("file")

		// Logic to find the right file based on index or name
		fileNameToServe, fileType := GetFileToServe(indexByQueryInt, colDataMapArray, nameByQuery)

		if fileNameToServe == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Get file path
		filePath := assetCache.LocalSyncPath + string(os.PathSeparator) + fileNameToServe
		assetFileByName, err := assetCache.GetFileByName(fileNameToServe)
		if err != nil {
			log.Errorf("[239] Failed to get file [%s] from asset cache: %v", filePath, err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		fileInfo, err := assetFileByName.Stat()
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer assetFileByName.Close() // Close the file after stat

		// Check if it's an image that needs processing
		if isImage := strings.HasPrefix(fileType, "image/"); isImage && c.Query("processImage") == "true" {
			// Use separate function for image processing
			file, err := cruds["world"].AssetFolderCache[typeName][columnName].GetFileByName(fileNameToServe)
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			defer file.Close()
			HandleImageProcessing(c, file)
			return
		}

		// Check if it's a video or audio file that should be streamed
		isVideo := strings.HasPrefix(fileType, "video/")
		isAudio := strings.HasPrefix(fileType, "audio/")
		if isVideo || isAudio {
			// For video/audio files, always use streaming with http.ServeContent for range request support
			file, err := os.Open(filePath)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Set media-specific headers for optimal streaming
			c.Header("Content-Type", fileType)
			c.Header("Accept-Ranges", "bytes")
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", fileNameToServe))

			// Set cache control for media files (shorter cache time due to size)
			c.Header("Cache-Control", "public, max-age=3600") // 1 hour

			// Generate ETag for media files
			etag := fmt.Sprintf("\"%x-%x\"", fileInfo.ModTime().Unix(), fileInfo.Size())
			c.Header("ETag", etag)

			// Check if client has fresh copy
			if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Use http.ServeContent for efficient video streaming with range request support
			http.ServeContent(c.Writer, c.Request, fileNameToServe, fileInfo.ModTime(), file)
			return
		}

		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Determine if this should be a download
		isDownload := cache.ShouldBeDownloaded(fileType, fileNameToServe)

		// Set response headers for all cases
		c.Header("Content-Type", fileType)

		// For downloads, add content disposition
		if isDownload {
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", fileNameToServe))
		} else {
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", fileNameToServe))
		}

		// Calculate expiry time
		expiryTime := cache.CalculateExpiry(fileType, filePath)

		// Set cache control header based on expiry
		maxAge := int(time.Until(expiryTime).Seconds())
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))

		// Use optimized file serving for small files that can be cached
		if fileInfo.Size() <= cache.MaxFileCacheSize {
			// Read file into memory with size limit protection
			data, err := readFileWithLimit(assetFileByName, cache.MaxFileCacheSize)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			// Generate ETag for client-side caching
			etag := cache.GenerateETag(data, fileInfo.ModTime())

			// Check if client has fresh copy before we do anything else
			if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
				c.Header("ETag", etag)
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Create cache entry
			newCachedFile := &cache.CachedFile{
				Data:       data,
				ETag:       etag,
				Modtime:    fileInfo.ModTime(),
				MimeType:   fileType,
				Size:       len(data),
				Path:       filePath,
				IsDownload: isDownload,
				ExpiresAt:  expiryTime,
			}

			// Pre-compress text files for better performance
			needsCompression := cache.ShouldCompress(fileType) && len(data) > cache.CompressionThreshold
			if needsCompression {
				if compressedData, err := cache.CompressData(data); err == nil {
					newCachedFile.GzipData = compressedData
				}
			}

			// Get file stat for validation
			if fileStat, err := cache.GetFileStat(filePath); err == nil {
				newCachedFile.FileStat = fileStat
			}

			// Add to cache for future requests
			fileCache.Set(cacheKey, newCachedFile)

			// Set ETag header
			c.Header("ETag", etag)

			// Use compression if client accepts it and we have compressed data
			if newCachedFile.GzipData != nil && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
				c.Data(http.StatusOK, fileType, newCachedFile.GzipData)
				return
			}

			// Serve uncompressed data
			c.Data(http.StatusOK, fileType, data)
			return
		}

		// For larger files, use http.ServeContent for efficient range requests
		// This is important for video/audio streaming
		file, err := os.Open(filePath)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Set ETag for large files too
		// Instead of reading the entire file, use file info to generate ETag
		etag := fmt.Sprintf("\"%x-%x\"", fileInfo.ModTime().Unix(), fileInfo.Size())
		c.Header("ETag", etag)

		// Add streaming-specific headers for video/audio files
		if isVideo || isAudio {
			c.Header("Accept-Ranges", "bytes")
		}

		// Check if client has fresh copy
		if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
			c.AbortWithStatus(http.StatusNotModified)
			return
		}

		http.ServeContent(c.Writer, c.Request, fileNameToServe, fileInfo.ModTime(), file)
	}
}

func GetFileToServe(indexByQueryInt int, colDataMapArray []map[string]interface{}, nameByQuery string) (string, string) {
	fileNameToServe := ""
	fileType := "application/octet-stream"

	if indexByQueryInt > -1 && indexByQueryInt < len(colDataMapArray) {
		fileData := colDataMapArray[indexByQueryInt]
		fileName := fileData["name"].(string)
		queryFile := nameByQuery

		if queryFile == fileName || queryFile == "" {
			// Determine filename
			if fileData["path"] != nil && len(fileData["path"].(string)) > 0 {
				fileNameToServe = fileData["path"].(string) + "/" + fileName
			} else {
				fileNameToServe = fileName
			}

			// Determine mime type
			if typFromData, ok := fileData["type"]; ok {
				if typeStr, isStr := typFromData.(string); isStr {
					fileType = typeStr
				} else {
					fileType = cache.GetMimeType(fileNameToServe)
				}
			} else {
				fileType = cache.GetMimeType(fileNameToServe)
			}
		}
	} else {
		for _, fileData := range colDataMapArray {
			fileName := fileData["name"].(string)
			queryFile := nameByQuery

			if queryFile == fileName || queryFile == "" {
				// Determine filename
				if fileData["path"] != nil && len(fileData["path"].(string)) > 0 {
					fileNameToServe = fileData["path"].(string) + "/" + fileName
				} else {
					fileNameToServe = fileName
				}

				// Determine mime type
				if typFromData, ok := fileData["type"]; ok {
					if typeStr, isStr := typFromData.(string); isStr {
						fileType = typeStr
					} else {
						fileType = cache.GetMimeType(fileNameToServe)
					}
				} else {
					fileType = cache.GetMimeType(fileNameToServe)
				}

				break
			}
		}
	}
	return fileNameToServe, fileType
}
