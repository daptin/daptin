package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/jmoiron/sqlx"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// AssetUploadHandler handles direct asset uploads with streaming support
func AssetUploadHandler(cruds map[string]*resource.DbResource) func(c *gin.Context) {
	return func(c *gin.Context) {
		typeName := c.Param("typename")
		resourceUuid := c.Param("resource_id")
		columnName := c.Param("columnname")

		operation := c.Query("operation")
		switch c.Request.Method {
		case http.MethodPost:
			if operation != "" && operation != "init" && operation != "complete" && operation != "stream" {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		case http.MethodGet:
			if operation != "get_part_url" {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		case http.MethodDelete:
			if operation != "" {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		default:
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		fileName := c.Query("filename")

		uuidDir := daptinid.InterfaceToDIR(resourceUuid)
		if uuidDir == daptinid.NullReferenceId {
			c.AbortWithStatus(404)
			return
		}

		if operation == "init" || operation == "stream" {
			var err error
			fileName, err = storagefs.ValidatePath(fileName)
			if err != nil || fileName == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "filename is required and must be a valid asset path"})
				return
			}
		}
		// Validate table and column
		dbResource, ok := cruds[typeName]
		if !ok || dbResource == nil {
			log.Errorf("table not found [%v]", typeName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		colInfo, ok := dbResource.TableInfo().GetColumnByName(columnName)
		if !ok || colInfo == nil || !colInfo.IsForeignKey || colInfo.ForeignKeyData.DataSource != "cloud_store" {
			log.Errorf("column [%v] is not a cloud_store asset column", columnName)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Get asset cache
		assetCache, ok := cruds["world"].AssetFolderCache[typeName][columnName]
		if !ok {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		originalRowReference := map[string]interface{}{
			"__type":                typeName,
			"reference_id":          uuidDir,
			"relation_reference_id": daptinid.NullReferenceId,
		}

		sessionUser := &auth.SessionUser{}
		if user, ok := c.Request.Context().Value("user").(*auth.SessionUser); ok && user != nil {
			sessionUser = user
		}

		canUpdate := func(tx *sqlx.Tx) bool {
			tablePermission := dbResource.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", typeName, tx)
			rowPermission := dbResource.GetRowPermissionWithTransaction(originalRowReference, tx)
			return tablePermission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) &&
				rowPermission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId)
		}

		if c.Request.Method == http.MethodPost && (operation == "" || operation == "stream") {
			tx, err := dbResource.Connection().Beginx()
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			allowed := canUpdate(tx)
			_ = tx.Rollback()
			if !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			var reader io.Reader = c.Request.Body
			fileType := c.ContentType()
			var multipartFile multipart.File
			if operation == "" {
				if !strings.HasPrefix(c.ContentType(), "multipart/form-data") {
					c.JSON(http.StatusBadRequest, gin.H{"error": "multipart file field is required"})
					return
				}
				part, err := c.FormFile("file")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "multipart file field is required"})
					return
				}
				fileName, err = storagefs.ValidatePath(part.Filename)
				if err != nil || fileName == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
					return
				}
				multipartFile, err = part.Open()
				if err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				defer multipartFile.Close()
				reader = multipartFile
				fileType = part.Header.Get("Content-Type")
			}
			if fileType == "" {
				fileType = "application/octet-stream"
			}
			size, err := writeAsset(c, fileName, reader, assetCache)
			if err != nil {
				log.Errorf("asset upload failed: %v", err)
				if errors.Is(err, storagefs.ErrPathEscapesRoot) {
					c.AbortWithStatus(http.StatusBadRequest)
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
				return
			}
			tx, err = dbResource.Connection().Beginx()
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()
			if err := dbResource.UpdateAssetColumnWithFile(columnName, fileName, uuidDir, size, fileType, tx); err != nil {
				log.Errorf("failed to attach uploaded asset: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			fileIndex, err := assetFileIndex(dbResource, uuidDir, columnName, fileName, tx)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if err := tx.Commit(); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			invalidateAssetCache(typeName, resourceUuid, columnName, fileName, fileIndex)
			c.JSON(http.StatusOK, gin.H{"status": "completed", "fileName": fileName, "size": size})
			return
		}

		transaction, err := dbResource.Connection().Beginx()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer transaction.Rollback()
		if !canUpdate(transaction) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		switch c.Request.Method {
		case http.MethodDelete:
			handleAssetDelete(c, dbResource, typeName, resourceUuid, columnName, uuidDir, assetCache, transaction)
		case http.MethodGet:
			handleGetPartPresignedURL(c, dbResource, uuidDir, columnName, assetCache, transaction)
		case http.MethodPost:
			if operation == "init" {
				handleUploadInit(c, cruds, typeName, columnName, fileName, uuidDir, assetCache, transaction)
			} else {
				handleUploadComplete(c, cruds, typeName, columnName, uuidDir, transaction, canUpdate)
			}
		}
	}
}

// handleUploadInit initializes an upload session and returns presigned URL if available
func handleUploadInit(c *gin.Context, cruds map[string]*resource.DbResource, typeName, columnName, fileName string,
	resourceUuid daptinid.DaptinReferenceId, assetCache *assetcachepojo.AssetFolderCache, transaction *sqlx.Tx) {
	fileSize, _ := strconv.ParseInt(c.GetHeader("X-File-Size"), 10, 64)
	fileType := c.GetHeader("X-File-Type")
	if fileType == "" {
		fileType = "application/octet-stream"
	}

	// Generate upload ID
	uploadId := uuid.New().String()

	// Check if this is a large file that requires multipart upload (>100MB)
	const multipartThreshold = 100 * 1024 * 1024 // 100MB
	if fileSize > multipartThreshold {
		// Check if this is S3 storage which supports multipart
		if assetCache.Credentials != nil {
			if providerType, ok := assetCache.Credentials["type"].(string); ok && providerType == "s3" {
				// Extract bucket and key
				rootPath := assetCache.CloudStore.RootPath
				keyPath := assetCache.Keyname + "/" + fileName

				// Parse bucket name
				bucketName := ""
				if strings.Contains(rootPath, ":") {
					parts := strings.Split(rootPath, ":")
					if len(parts) >= 2 {
						bucketName = strings.TrimPrefix(parts[1], "/")
						if strings.Contains(bucketName, "/") {
							pathParts := strings.SplitN(bucketName, "/", 2)
							bucketName = pathParts[0]
							if len(pathParts) > 1 {
								keyPath = pathParts[1] + "/" + keyPath
							}
						}
					}
				}

				if bucketName != "" {
					// Initiate S3 multipart upload
					s3UploadId, err := InitiateS3MultipartUpload(assetCache.Credentials, bucketName, keyPath)
					if err == nil {
						// Update database with pending multipart upload
						err = cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, s3UploadId, transaction)
						if err != nil {
							log.Errorf("Failed to update asset column with pending multipart upload: %v", err)
							c.AbortWithStatus(http.StatusInternalServerError)
							return
						}
						if err := transaction.Commit(); err != nil {
							c.AbortWithStatus(http.StatusInternalServerError)
							return
						}

						// Return multipart upload details to client
						c.JSON(http.StatusOK, gin.H{
							"upload_id":     uploadId,
							"s3_upload_id":  s3UploadId,
							"upload_type":   "multipart",
							"min_part_size": 5 * 1024 * 1024, // 5MB minimum part size for S3
							"max_parts":     10000,           // S3 limit
							"get_part_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=get_part_url&upload_id=%s",
								typeName, resourceUuid, columnName, uploadId),
							"complete_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=complete&upload_id=%s",
								typeName, resourceUuid, columnName, uploadId),
						})
						return
					}
					log.Errorf("Failed to initiate S3 multipart asset upload: %v", err)
					c.AbortWithStatus(http.StatusBadGateway)
					return
				}
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
	}

	// Try to generate presigned URL based on storage provider
	presignedData, err := generatePresignedURL(assetCache, fileName, uploadId)

	if err == nil && presignedData != nil {
		// Presigned URL available - update database with pending upload
		err = cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, "", transaction)
		if err != nil {
			log.Errorf("[147] Failed to update asset column with pending upload: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if err := transaction.Commit(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// Return presigned URL to client
		c.JSON(http.StatusOK, gin.H{
			"upload_id":      uploadId,
			"upload_type":    "presigned",
			"presigned_data": presignedData,
			"complete_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=complete&upload_id=%s",
				typeName, resourceUuid, columnName, uploadId),
		})
		return
	}
	if providerType, _ := assetCache.Credentials["type"].(string); providerType == "s3" {
		log.Errorf("failed to generate presigned asset URL: %v", err)
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}

	// Non-S3 stores use the same direct streaming path as a multipart POST.
	c.JSON(http.StatusOK, gin.H{
		"upload_type": "stream",
		"upload_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=stream&filename=%s",
			typeName, resourceUuid, columnName, url.QueryEscape(fileName)),
	})
}

// writeAsset is the storage write used by raw and multipart HTTP uploads.
func writeAsset(c *gin.Context, fileName string, reader io.Reader, assetCache *assetcachepojo.AssetFolderCache) (int64, error) {
	var err error
	fileName, err = storagefs.ValidatePath(fileName)
	if err != nil {
		return 0, err
	}
	if fileName == "" {
		return 0, fmt.Errorf("filename is required")
	}
	setupCloudStorageCredentials(assetCache)
	if isLocalStorage(assetCache) {
		localPath, err := assetCache.CloudStore.ResolvePath(path.Join(assetCache.Keyname, fileName))
		if err != nil {
			return 0, err
		}
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return 0, err
		}
		file, err := os.Create(localPath)
		if err != nil {
			return 0, err
		}
		size, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return 0, copyErr
		}
		return size, closeErr
	}

	destination, err := assetCache.CloudStore.ResolvePath(assetCache.Keyname)
	if err != nil {
		return 0, err
	}
	ctx := c.Request.Context()
	fdst, err := fs.NewFs(ctx, destination)
	if err != nil {
		return 0, err
	}
	progress := &progressReader{reader: reader, total: c.Request.ContentLength}
	_, err = operations.Rcat(ctx, fdst, fileName, io.NopCloser(progress), time.Now(), fs.Metadata{})
	if err != nil {
		return 0, err
	}
	return progress.bytesRead, nil
}

// handleUploadComplete marks an upload as complete after client-side upload
func handleUploadComplete(c *gin.Context, cruds map[string]*resource.DbResource,
	typeName, columnName string, resourceUuid daptinid.DaptinReferenceId, transaction *sqlx.Tx, canUpdate func(*sqlx.Tx) bool) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = c.PostForm("upload_id")
	}
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload_id is required"})
		return
	}
	pending, err := cruds[typeName].GetPendingAssetUpload(resourceUuid, columnName, uploadId, transaction)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pending upload not found"})
		return
	}
	fileName, ok := pending["name"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pending upload has no filename"})
		return
	}
	fileName, err = storagefs.ValidatePath(fileName)
	if err != nil || fileName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pending upload has invalid filename"})
		return
	}

	// Get additional metadata from client
	var metadata map[string]interface{}
	if err := c.ShouldBindJSON(&metadata); err != nil {
		metadata = make(map[string]interface{})
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Check if this is a multipart upload completion
	if parts, ok := metadata["parts"].([]interface{}); ok && len(parts) > 0 {
		// This is a multipart upload completion
		s3UploadId, _ := pending["s3_upload_id"].(string)
		if s3UploadId == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "upload is not a multipart upload",
			})
			return
		}

		// Get asset cache to access credentials
		assetCache := cruds["world"].AssetFolderCache[typeName][columnName]

		// Check if this is S3 storage
		if assetCache.Credentials == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "multipart completion not supported for this storage type",
			})
			return
		}

		providerType, ok := assetCache.Credentials["type"].(string)
		if !ok || providerType != "s3" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "multipart completion only supported for S3 storage",
			})
			return
		}

		// Extract bucket and key
		rootPath := assetCache.CloudStore.RootPath
		keyPath := assetCache.Keyname + "/" + fileName

		// Parse bucket name
		bucketName := ""
		if strings.Contains(rootPath, ":") {
			partsList := strings.Split(rootPath, ":")
			if len(partsList) >= 2 {
				bucketName = strings.TrimPrefix(partsList[1], "/")
				if strings.Contains(bucketName, "/") {
					pathParts := strings.SplitN(bucketName, "/", 2)
					bucketName = pathParts[0]
					if len(pathParts) > 1 {
						keyPath = pathParts[1] + "/" + keyPath
					}
				}
			}
		}

		if bucketName == "" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not extract bucket name",
			})
			return
		}

		// Convert parts to the format expected by CompleteS3MultipartUpload
		var s3Parts []map[string]interface{}
		for _, part := range parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				s3Parts = append(s3Parts, partMap)
			}
		}

		// S3 completion and object inspection must not hold the metadata transaction.
		if err := transaction.Rollback(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		// Complete the multipart upload on S3
		err := CompleteS3MultipartUpload(assetCache.Credentials, bucketName, keyPath, s3UploadId, s3Parts)
		if err != nil {
			log.Errorf("Failed to complete S3 multipart upload: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to complete multipart upload: %v", err),
				"details": map[string]interface{}{
					"bucket":      bucketName,
					"key":         keyPath,
					"upload_id":   s3UploadId,
					"parts_count": len(s3Parts),
				},
			})
			return
		}

		fileSize, err := assetObjectSize(c.Request.Context(), assetCache, fileName)
		if err != nil {
			c.AbortWithStatus(http.StatusBadGateway)
			return
		}
		metadata["size"] = fileSize
		delete(metadata, "type")
		completionTx, err := cruds[typeName].Connection().Beginx()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer completionTx.Rollback()
		if !canUpdate(completionTx) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if _, err := cruds[typeName].GetPendingAssetUpload(resourceUuid, columnName, uploadId, completionTx); err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		err = cruds[typeName].UpdateAssetColumnStatus(resourceUuid, columnName, uploadId, "completed", metadata, completionTx)
		if err != nil {
			log.Errorf("Failed to update asset column status: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		fileIndex, err := assetFileIndex(cruds[typeName], resourceUuid, columnName, fileName, completionTx)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if err := completionTx.Commit(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		invalidateAssetCache(typeName, resourceUuid.String(), columnName, fileName, fileIndex)

		c.JSON(http.StatusOK, gin.H{
			"upload_id": uploadId,
			"status":    "completed",
			"fileName":  fileName,
			"size":      fileSize,
			"multipart": true,
		})
		return
	}

	if pending["s3_upload_id"] != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart parts are required"})
		return
	}
	assetCache := cruds["world"].AssetFolderCache[typeName][columnName]
	fileSize, err := assetObjectSize(c.Request.Context(), assetCache, fileName)
	if err != nil {
		log.Errorf("uploaded asset verification failed: %v", err)
		if errors.Is(err, fs.ErrorObjectNotFound) || os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file not found in cloud storage", "upload_id": uploadId})
		} else {
			c.AbortWithStatus(http.StatusBadGateway)
		}
		return
	}
	metadata["size"] = fileSize
	delete(metadata, "type")
	if err := cruds[typeName].UpdateAssetColumnStatus(resourceUuid, columnName, uploadId, "completed", metadata, transaction); err != nil {
		log.Errorf("Failed to update asset column status: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	fileIndex, err := assetFileIndex(cruds[typeName], resourceUuid, columnName, fileName, transaction)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := transaction.Commit(); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	invalidateAssetCache(typeName, resourceUuid.String(), columnName, fileName, fileIndex)

	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadId,
		"status":    "completed",
		"fileName":  fileName,
		"size":      fileSize,
	})
}

// progressReader wraps io.Reader to track upload progress
type progressReader struct {
	reader    io.Reader
	bytesRead int64
	total     int64
	uploadId  string
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.bytesRead += int64(n)

	// Could emit progress events here if needed
	if pr.total > 0 {
		progress := float64(pr.bytesRead) / float64(pr.total) * 100
		if int(progress)%10 == 0 {
			log.Debugf("Upload progress [%s]: %.0f%%", pr.uploadId, progress)
		}
	}

	return n, err
}

// setupCloudStorageCredentials configures rclone credentials for the given asset cache
func setupCloudStorageCredentials(assetCache *assetcachepojo.AssetFolderCache) string {
	configSetName := assetCache.CloudStore.Name
	if strings.Contains(assetCache.CloudStore.RootPath, ":") {
		configSetName = strings.Split(assetCache.CloudStore.RootPath, ":")[0]
	}

	if assetCache.Credentials != nil {
		for key, val := range assetCache.Credentials {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
		}
	}

	return configSetName
}

func assetObjectSize(ctx context.Context, assetCache *assetcachepojo.AssetFolderCache, fileName string) (int64, error) {
	fileName, err := storagefs.ValidatePath(fileName)
	if err != nil || fileName == "" {
		return 0, fmt.Errorf("invalid asset path")
	}
	if isLocalStorage(assetCache) {
		localPath, err := assetCache.CloudStore.ResolvePath(path.Join(assetCache.Keyname, fileName))
		if err != nil {
			return 0, err
		}
		info, err := os.Stat(localPath)
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}
	setupCloudStorageCredentials(assetCache)
	root, err := assetCache.CloudStore.ResolvePath(assetCache.Keyname)
	if err != nil {
		return 0, err
	}
	store, err := fs.NewFs(ctx, root)
	if err != nil {
		return 0, err
	}
	object, err := store.NewObject(ctx, fileName)
	if err != nil {
		return 0, err
	}
	return object.Size(), nil
}

func invalidateAssetCache(typeName, resourceUuid, columnName, fileName string, index int) {
	if fileCache == nil {
		return
	}
	for _, selector := range []string{"", fileName} {
		fileCache.Remove(fmt.Sprintf("%s:%s:%s::%s", typeName, resourceUuid, columnName, selector))
		if index >= 0 {
			fileCache.Remove(fmt.Sprintf("%s:%s:%s:%d:%s", typeName, resourceUuid, columnName, index, selector))
		}
	}
}

func assetFileIndex(dbResource *resource.DbResource, rowID daptinid.DaptinReferenceId, columnName, fileName string, transaction *sqlx.Tx) (int, error) {
	files, err := dbResource.GetAssetColumnFiles(rowID, columnName, transaction)
	if err != nil {
		return -1, err
	}
	for i, file := range files {
		if file["name"] == fileName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("attached asset not found")
}

func handleAssetDelete(c *gin.Context, dbResource *resource.DbResource, typeName, resourceUuid, columnName string,
	rowID daptinid.DaptinReferenceId, assetCache *assetcachepojo.AssetFolderCache, transaction *sqlx.Tx) {
	files, err := dbResource.GetAssetColumnFiles(rowID, columnName, transaction)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	selectedName := c.Query("file")
	selectedPath := c.Query("path")
	hasPath := c.Request.URL.Query().Has("path")
	indexValue := c.Query("index")
	if selectedName == "" && (indexValue != "" || hasPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required with path or index"})
		return
	}
	if indexValue != "" && !hasPath {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required with index for a repeatable deletion"})
		return
	}
	index := -1
	if indexValue != "" {
		index, err = strconv.Atoi(indexValue)
		if err != nil || index < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid index"})
			return
		}
	}
	if selectedName == "" {
		if len(files) == 0 {
			c.Status(http.StatusNoContent)
			return
		}
		if len(files) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file selector is required"})
			return
		}
		index = 0
	} else if index >= 0 {
		if index >= len(files) || files[index]["name"] != selectedName || assetEntryPath(files[index]) != selectedPath {
			c.Status(http.StatusNoContent)
			return
		}
	} else {
		for i, file := range files {
			if file["name"] == selectedName && (!hasPath || assetEntryPath(file) == selectedPath) {
				if index >= 0 && !hasPath {
					c.JSON(http.StatusBadRequest, gin.H{"error": "file selector is ambiguous; include path"})
					return
				}
				index = i
			}
		}
		if index < 0 {
			c.Status(http.StatusNoContent)
			return
		}
	}
	storedName, ok := files[index]["name"].(string)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if files[index]["status"] == "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pending upload is not a stored artifact"})
		return
	}
	storedPath, _ := files[index]["path"].(string)
	fileName, err := storagefs.ValidatePath(storedName)
	if err != nil || fileName == "" {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	filePath, err := storagefs.ValidatePath(storedPath)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	objectName := path.Join(filePath, fileName)
	storageRoot, err := assetCache.CloudStore.ResolvePath(assetCache.Keyname)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	deleteAction, ok := resource.GetGlobalActionHandler("site.file.delete")
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	_, _, actionErrors := deleteAction.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"credential_name": assetCache.CloudStore.CredentialName,
		"store_type":      assetCache.CloudStore.StoreType,
		"root_path":       storageRoot,
		"path":            objectName,
		"is_file":         true,
	}, transaction)
	if len(actionErrors) > 0 {
		log.Errorf("failed to queue attached asset deletion: %v", actionErrors[0])
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := dbResource.RemoveAssetColumnFile(rowID, columnName, index, storedName, storedPath, transaction); err != nil {
		log.Errorf("failed to detach deleted asset: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := transaction.Commit(); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if !isLocalStorage(assetCache) {
		if err := assetCache.DeleteFileByName(objectName); err != nil && !os.IsNotExist(err) {
			log.Warnf("failed to evict asset cache: %v", err)
		}
	}
	invalidateAssetCache(typeName, resourceUuid, columnName, storedName, index)
	c.Status(http.StatusAccepted)
}

func assetEntryPath(file map[string]interface{}) string {
	storedPath, _ := file["path"].(string)
	return storedPath
}

// isLocalStorage checks if the storage provider is local filesystem
func isLocalStorage(assetCache *assetcachepojo.AssetFolderCache) bool {
	return assetCache.CloudStore.StoreType == "local"
}

// handleGetPartPresignedURL generates a presigned URL for a specific part in multipart upload
func handleGetPartPresignedURL(c *gin.Context, dbResource *resource.DbResource, rowID daptinid.DaptinReferenceId, columnName string,
	assetCache *assetcachepojo.AssetFolderCache, transaction *sqlx.Tx) {
	uploadId := c.Query("upload_id")
	partNumberStr := c.Query("part_number")

	if uploadId == "" || partNumberStr == "" {
		c.AbortWithError(400, fmt.Errorf("upload_id and part_number are required"))
		return
	}
	pending, err := dbResource.GetPendingAssetUpload(rowID, columnName, uploadId, transaction)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	s3UploadId, _ := pending["s3_upload_id"].(string)
	fileName, _ := pending["name"].(string)
	if s3UploadId == "" || fileName == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	partNumber, err := strconv.ParseInt(partNumberStr, 10, 32)
	if err != nil {
		c.AbortWithError(400, fmt.Errorf("invalid part_number: %v", err))
		return
	}

	// Check if this is S3 storage
	if assetCache.Credentials == nil {
		c.AbortWithError(400, fmt.Errorf("presigned URLs not supported for this storage type"))
		return
	}

	providerType, ok := assetCache.Credentials["type"].(string)
	if !ok || providerType != "s3" {
		c.AbortWithError(400, fmt.Errorf("presigned URLs only supported for S3 storage"))
		return
	}

	// Extract bucket and key
	rootPath := assetCache.CloudStore.RootPath
	fileName, err = storagefs.ValidatePath(fileName)
	if err != nil || fileName == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid filename"))
		return
	}

	keyPath := assetCache.Keyname + "/" + fileName

	// Parse bucket name
	bucketName := ""
	if strings.Contains(rootPath, ":") {
		parts := strings.Split(rootPath, ":")
		if len(parts) >= 2 {
			bucketName = strings.TrimPrefix(parts[1], "/")
			if strings.Contains(bucketName, "/") {
				pathParts := strings.SplitN(bucketName, "/", 2)
				bucketName = pathParts[0]
				if len(pathParts) > 1 {
					keyPath = pathParts[1] + "/" + keyPath
				}
			}
		}
	}

	if bucketName == "" {
		c.AbortWithError(500, fmt.Errorf("could not extract bucket name"))
		return
	}

	// Generate presigned URL for this part
	presignedUrl, err := GetS3PartPresignedURL(assetCache.Credentials, bucketName, keyPath, s3UploadId, int32(partNumber))
	if err != nil {
		log.Errorf("Failed to generate part presigned URL: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"presigned_url": presignedUrl,
		"part_number":   partNumber,
		"expires_at":    time.Now().Add(3600 * time.Second).Unix(),
	})
}
