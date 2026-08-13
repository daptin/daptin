package server

import (
	"bytes"
	"context"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/assetcachepojo"
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
				} else {
					row, authz, err := loadAuthorizedAssetRow(cruds, typeName, resourceUuid, c)
					if err != nil {
						fileCache.RemoveAsync(cacheKey)
						abortAssetError(c, err)
						return
					}
					if !assetAuthzAllowed(authz, c) {
						fileCache.RemoveAsync(cacheKey)
						c.AbortWithStatus(http.StatusForbidden)
						return
					}
					if colInfo.ColumnType != "markdown" && !cachedAssetFileStillCurrent(cachedFile, row, cruds, typeName, columnName, c) {
						fileCache.RemoveAsync(cacheKey)
						c.AbortWithStatus(http.StatusNotFound)
						return
					}
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

			serveCachedAsset(c, cachedMarkdown)
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
		assetFileByName, err := assetCache.GetFileByNameContext(c.Request.Context(), fileNameToServe)
		if err != nil {
			log.Errorf("[239] Failed to get file [%s] from asset cache: %v", filePath, err)
			abortAssetStorageError(c, err)
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
			HandleImageProcessing(c, assetFileByName)
			return
		}

		// Check if it's a video or audio file that should be streamed
		isVideo := strings.HasPrefix(fileType, "video/")
		isAudio := strings.HasPrefix(fileType, "audio/")
		if isVideo || isAudio {
			// Serve the handle returned by GetFileByName. It may point beneath a
			// cloud-store key or may have just been downloaded into the cache.
			serveResolvedMediaAsset(c, fileNameToServe, fileType, assetFileByName, fileInfo)
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

			serveCachedAsset(c, newCachedFile)
			return
		}

		// Stream larger files from the handle already resolved by the asset cache,
		// using the same reusable gzip sidecar strategy as hosted sites.
		serveResolvedAssetWithCompression(c, fileNameToServe, assetFileByName, fileInfo, assetCache.LocalSyncPath, cache.ShouldCompress(fileType))
	}
}

func serveResolvedMediaAsset(c *gin.Context, fileName, fileType string, file *os.File, fileInfo os.FileInfo) {
	c.Header("Content-Type", fileType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", fileName))
	setPrivateAssetCacheHeaders(c, 3600)

	serveResolvedAsset(c, fileName, file, fileInfo)
}

func serveResolvedAsset(c *gin.Context, fileName string, file *os.File, fileInfo os.FileInfo) {
	etag := generateETagFromStat(fileInfo)
	c.Header("ETag", etag)
	if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	http.ServeContent(c.Writer, c.Request, fileName, fileInfo.ModTime(), file)
}

func serveResolvedAssetWithCompression(c *gin.Context, fileName string, file *os.File, fileInfo os.FileInfo, cacheRoot string, compressible bool) {
	etag := generateETagFromStat(fileInfo)
	servedFile := file
	compressible = compressible && fileInfo.Size() > cache.CompressionThreshold
	if compressible {
		c.Header("Vary", appendVary(c.Writer.Header().Get("Vary"), "Accept-Encoding"))
	}
	if compressible && c.GetHeader("Range") == "" && acceptsGzip(c.GetHeader("Accept-Encoding")) {
		if compressed, err := openPrecompressedSubsiteFile(file, fileInfo, cacheRoot, fileName); err == nil {
			servedFile = compressed
			defer compressed.Close()
			etag = gzipETag(etag)
			c.Header("Content-Encoding", "gzip")
		}
	}

	c.Header("ETag", etag)
	if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}
	http.ServeContent(c.Writer, c.Request, fileName, fileInfo.ModTime(), servedFile)
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
	if cachedFile == nil {
		return false
	}
	return assetAuthzAllowed(assetAuthzSnapshot{
		tablePermission: cachedFile.TablePermission,
		rowPermission:   cachedFile.RowPermission,
		adminGroupId:    cachedFile.AdminGroupId,
	}, c)
}

func assetAuthzAllowed(authz assetAuthzSnapshot, c *gin.Context) bool {
	sessionUser := &auth.SessionUser{}
	if user := c.Request.Context().Value("user"); user != nil {
		if typedUser, ok := user.(*auth.SessionUser); ok && typedUser != nil {
			sessionUser = typedUser
		}
	}

	for _, group := range sessionUser.Groups {
		if group.GroupReferenceId == authz.adminGroupId {
			return true
		}
	}

	return authz.tablePermission.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups, authz.adminGroupId) &&
		authz.rowPermission.CanRead(sessionUser.UserReferenceId, sessionUser.Groups, authz.adminGroupId)
}

func cachedAssetFileStillCurrent(cachedFile *cache.CachedFile, row map[string]interface{}, cruds map[string]*resource.DbResource, typeName, columnName string, c *gin.Context) bool {
	if cachedFile == nil || row == nil {
		return false
	}

	worldCrud, ok := cruds["world"]
	if !ok || worldCrud == nil || worldCrud.AssetFolderCache == nil {
		return false
	}
	tableAssetCaches, ok := worldCrud.AssetFolderCache[typeName]
	if !ok {
		return false
	}
	assetCache, ok := tableAssetCaches[columnName]
	if !ok || assetCache == nil {
		return false
	}

	files, ok := assetColumnFileList(row[columnName])
	if !ok {
		return false
	}

	indexByQueryInt := -1
	if indexByQuery := c.Query("index"); indexByQuery != "" {
		if parsedIndex, err := strconv.Atoi(indexByQuery); err == nil {
			indexByQueryInt = parsedIndex
		}
	}
	fileNameToServe, _ := GetFileToServe(indexByQueryInt, files, c.Query("file"))
	if fileNameToServe == "" {
		return false
	}

	expectedPath := filepath.Clean(assetCache.LocalSyncPath + string(os.PathSeparator) + fileNameToServe)
	return filepath.Clean(cachedFile.Path) == expectedPath
}

func assetColumnFileList(colData interface{}) ([]map[string]interface{}, bool) {
	switch typed := colData.(type) {
	case []map[string]interface{}:
		return typed, true
	case []interface{}:
		files := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			file, ok := item.(map[string]interface{})
			if !ok {
				return nil, false
			}
			files = append(files, file)
		}
		return files, true
	default:
		return nil, false
	}
}

func serveCachedAsset(c *gin.Context, cachedFile *cache.CachedFile) {
	useGzip := cachedFile.GzipData != nil && len(cachedFile.GzipData) > 0 && c.GetHeader("Range") == "" && acceptsGzip(c.GetHeader("Accept-Encoding"))
	etag := cachedFile.ETag
	if cachedFile.GzipData != nil && len(cachedFile.GzipData) > 0 {
		c.Header("Vary", appendVary(c.Writer.Header().Get("Vary"), "Accept-Encoding"))
	}
	if useGzip {
		etag = gzipETag(etag)
		c.Header("Content-Encoding", "gzip")
	}

	if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
		setPrivateAssetCacheHeaders(c, int(time.Until(cachedFile.ExpiresAt).Seconds()))
		c.Header("ETag", etag)
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	c.Header("Content-Type", cachedFile.MimeType)
	c.Header("ETag", etag)
	setPrivateAssetCacheHeaders(c, int(time.Until(cachedFile.ExpiresAt).Seconds()))

	if cachedFile.IsDownload {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(cachedFile.Path)))
	} else {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", filepath.Base(cachedFile.Path)))
	}

	if useGzip {
		c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.GzipData)
		return
	}

	http.ServeContent(c.Writer, c.Request, filepath.Base(cachedFile.Path), cachedFile.Modtime, bytes.NewReader(cachedFile.Data))
}

func setPrivateAssetCacheHeaders(c *gin.Context, maxAge int) {
	if maxAge <= 0 {
		maxAge = 60
	}
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	c.Header("Vary", appendVary(c.Writer.Header().Get("Vary"), "Authorization"))
}

func abortAssetError(c *gin.Context, err error) {
	if httpErr, ok := err.(api2go.HTTPError); ok {
		c.AbortWithStatus(httpErr.Status())
		return
	}
	c.AbortWithStatus(http.StatusInternalServerError)
}

func abortAssetStorageError(c *gin.Context, err error) {
	if assetcachepojo.IsAssetNotFound(err) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.AbortWithStatus(http.StatusBadGateway)
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
