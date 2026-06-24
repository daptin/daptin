package server

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/cache"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
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

		if fileCache != nil {
			if cachedFile, found := fileCache.Get(cacheKey); found {
				if !cachedAssetHasAuthz(cachedFile) {
					fileCache.RemoveAsync(cacheKey)
				} else if !cachedAssetAllowed(cachedFile, c) {
					c.AbortWithStatus(http.StatusForbidden)
					return
				} else {
					serveCachedAsset(c, cachedFile)
					return
				}
			}
		}

		// Handle markdown directly (simple case)
		if colInfo.ColumnType == "markdown" {
			// Fetch data
			row, authz, err := loadAuthorizedAssetRow(cruds, typeName, resourceUuid, c)
			if err != nil {
				abortAssetError(c, err)
				return
			}

			colData := row[columnName]
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
				setPrivateAssetCacheHeaders(c, 86400)
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Cache the markdown content
			htmlContent := fmt.Sprintf("<pre>%s</pre>", markdownContent)
			cachedMarkdown := &cache.CachedFile{
				Data:            []byte(htmlContent),
				ETag:            etag,
				Modtime:         time.Now(),
				MimeType:        "text/html; charset=utf-8",
				Size:            len(htmlContent),
				Path:            fmt.Sprintf("%s/%s/%s", typeName, resourceUuid, columnNameWithExt),
				IsDownload:      false,
				ExpiresAt:       cache.CalculateExpiry("text/html", ""),
				AuthzVersion:    cachedAssetAuthzVersion,
				TablePermission: authz.tablePermission,
				RowPermission:   authz.rowPermission,
				AdminGroupId:    authz.adminGroupId,
			}

			// Create compressed version if large enough
			if len(htmlContent) > cache.CompressionThreshold {
				if compressedData, err := cache.CompressData([]byte(htmlContent)); err == nil {
					cachedMarkdown.GzipData = compressedData
				}
			}

			if fileCache != nil {
				fileCache.Set(cacheKey, cachedMarkdown)
			}

			// Return markdown as HTML with appropriate headers
			c.Header("Content-Type", "text/html; charset=utf-8")
			setPrivateAssetCacheHeaders(c, int(time.Until(cachedMarkdown.ExpiresAt).Seconds()))
			c.Header("ETag", etag)

			// Use compression if client accepts it and we have compressed data
			if cachedMarkdown.GzipData != nil && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Authorization, Accept-Encoding")
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
		row, authz, err := loadAuthorizedAssetRow(cruds, typeName, resourceUuid, c)
		if err != nil {
			abortAssetError(c, err)
			return
		}

		colData := row[columnName]
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
			setPrivateAssetCacheHeaders(c, 3600)

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
		setPrivateAssetCacheHeaders(c, maxAge)

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
				Data:            data,
				ETag:            etag,
				Modtime:         fileInfo.ModTime(),
				MimeType:        fileType,
				Size:            len(data),
				Path:            filePath,
				IsDownload:      isDownload,
				ExpiresAt:       expiryTime,
				AuthzVersion:    cachedAssetAuthzVersion,
				TablePermission: authz.tablePermission,
				RowPermission:   authz.rowPermission,
				AdminGroupId:    authz.adminGroupId,
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
			if fileCache != nil {
				fileCache.Set(cacheKey, newCachedFile)
			}

			// Set ETag header
			c.Header("ETag", etag)

			// Use compression if client accepts it and we have compressed data
			if newCachedFile.GzipData != nil && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Authorization, Accept-Encoding")
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

const cachedAssetAuthzVersion byte = 1

type assetAuthzSnapshot struct {
	tablePermission permission.PermissionInstance
	rowPermission   permission.PermissionInstance
	adminGroupId    daptinid.DaptinReferenceId
}

func loadAuthorizedAssetRow(cruds map[string]*resource.DbResource, typeName, resourceUuid string, c *gin.Context) (map[string]interface{}, assetAuthzSnapshot, error) {
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
		return nil, assetAuthzSnapshot{}, err
	}

	row := obj.Result().(api2go.Api2GoModel).GetAttributes()
	referenceId := daptinid.InterfaceToDIR(resourceUuid)

	transaction, err := cruds[typeName].Connection().Beginx()
	if err != nil {
		return nil, assetAuthzSnapshot{}, err
	}
	defer transaction.Rollback()

	rowReference := map[string]interface{}{
		"__type":                typeName,
		"reference_id":          referenceId,
		"relation_reference_id": daptinid.NullReferenceId,
	}
	authz := assetAuthzSnapshot{
		tablePermission: cruds[typeName].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", typeName, transaction),
		rowPermission:   cruds[typeName].GetRowPermissionWithTransaction(rowReference, transaction),
		adminGroupId:    cruds[typeName].AdministratorGroupId,
	}
	return row, authz, nil
}

func cachedAssetHasAuthz(cachedFile *cache.CachedFile) bool {
	return cachedFile != nil && cachedFile.AuthzVersion == cachedAssetAuthzVersion
}

func cachedAssetAllowed(cachedFile *cache.CachedFile, c *gin.Context) bool {
	sessionUser := &auth.SessionUser{}
	if user := c.Request.Context().Value("user"); user != nil {
		if typedUser, ok := user.(*auth.SessionUser); ok && typedUser != nil {
			sessionUser = typedUser
		}
	}

	for _, group := range sessionUser.Groups {
		if group.GroupReferenceId == cachedFile.AdminGroupId {
			return true
		}
	}

	return cachedFile.TablePermission.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups, cachedFile.AdminGroupId) &&
		cachedFile.RowPermission.CanRead(sessionUser.UserReferenceId, sessionUser.Groups, cachedFile.AdminGroupId)
}

func serveCachedAsset(c *gin.Context, cachedFile *cache.CachedFile) {
	if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == cachedFile.ETag {
		setPrivateAssetCacheHeaders(c, int(time.Until(cachedFile.ExpiresAt).Seconds()))
		c.Header("ETag", cachedFile.ETag)
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	c.Header("Content-Type", cachedFile.MimeType)
	c.Header("ETag", cachedFile.ETag)
	setPrivateAssetCacheHeaders(c, int(time.Until(cachedFile.ExpiresAt).Seconds()))

	if cachedFile.IsDownload {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(cachedFile.Path)))
	} else {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", filepath.Base(cachedFile.Path)))
	}

	if cachedFile.GzipData != nil && len(cachedFile.GzipData) > 0 && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Authorization, Accept-Encoding")
		c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.GzipData)
		return
	}

	c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.Data)
}

func setPrivateAssetCacheHeaders(c *gin.Context, maxAge int) {
	if maxAge <= 0 {
		maxAge = 60
	}
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	c.Header("Vary", "Authorization")
}

func abortAssetError(c *gin.Context, err error) {
	if httpErr, ok := err.(api2go.HTTPError); ok {
		c.AbortWithStatus(httpErr.Status())
		return
	}
	c.AbortWithStatus(http.StatusInternalServerError)
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
