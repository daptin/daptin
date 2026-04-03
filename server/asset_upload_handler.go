package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	"github.com/jmoiron/sqlx"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
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

		// Determine operation based on HTTP method and query param
		operation := c.Query("operation")
		if operation == "" {
			// Default operations based on HTTP method
			switch c.Request.Method {
			case "GET":
				operation = "get_part_url"
			case "DELETE":
				operation = "abort"
			case "POST":
				// Check if it's init or complete based on presence of upload_id
				if c.Query("upload_id") != "" || c.PostForm("upload_id") != "" {
					operation = "complete"
				} else {
					operation = "init"
				}
			default:
				operation = "stream"
			}
		}

		// Get filename from query param (required for all operations)
		fileName := c.Query("filename")

		uuidDir := daptinid.InterfaceToDIR(resourceUuid)
		if uuidDir == daptinid.NullReferenceId {
			c.AbortWithStatus(404)
			return
		}

		// Filename is required for all operations
		if fileName == "" && operation != "complete" {
			c.AbortWithError(400, errors.New("filename query parameter is required"))
			return
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

		user := c.Request.Context().Value("user")
		sessionUser := &auth.SessionUser{}

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		// For stream operation, we only need permission check, not full transaction
		if operation == "stream" || operation == "" {
			// Create a minimal transaction just for permission check
			tx, err := dbResource.Connection().Beginx()
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

			permission := dbResource.GetRowPermissionWithTransaction(originalRowReference, tx)
			tx.Rollback() // Immediately rollback as we only needed to check permissions

			if !permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			// Handle streaming upload without maintaining transaction
			handleStreamUpload(c, fileName, assetCache)
		} else if operation == "init" || operation == "complete" || operation == "get_part_url" || operation == "abort" {
			// For init and complete operations, we need full transaction
			transaction, err := dbResource.Connection().Beginx()
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			defer transaction.Rollback()

			permission := dbResource.GetRowPermissionWithTransaction(originalRowReference, transaction)
			if !permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			switch operation {
			case "init":
				// Initialize upload session and return presigned URL if supported
				handleUploadInit(c, cruds, typeName, columnName, fileName, uuidDir, assetCache, transaction)
			case "complete":
				// Mark upload as complete
				handleUploadComplete(c, cruds, typeName, columnName, fileName, uuidDir, transaction)
			case "get_part_url":
				// Get presigned URL for a specific part in multipart upload
				handleGetPartPresignedURL(c, assetCache)
			case "abort":
				// Abort multipart upload
				handleAbortMultipartUpload(c, assetCache)
			}
			transaction.Commit()
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
						err = cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, transaction)
						if err != nil {
							log.Errorf("Failed to update asset column with pending multipart upload: %v", err)
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
							"get_part_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=get_part_url&upload_id=%s&filename=%s",
								typeName, resourceUuid, columnName, s3UploadId, fileName),
							"complete_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=complete&upload_id=%s",
								typeName, resourceUuid, columnName, uploadId),
							"abort_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=abort&upload_id=%s&filename=%s",
								typeName, resourceUuid, columnName, s3UploadId, fileName),
						})
						return
					}
					// If multipart init failed, fall through to regular presigned URL
					log.Warnf("Failed to initiate multipart upload, falling back to regular upload: %v", err)
				}
			}
		}
	}

	// Try to generate presigned URL based on storage provider
	presignedData, err := generatePresignedURL(assetCache, fileName, uploadId)

	if err == nil && presignedData != nil {
		// Presigned URL available - update database with pending upload
		err = cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, transaction)
		if err != nil {
			log.Errorf("[147] Failed to update asset column with pending upload: %v", err)
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

	// For streaming upload, also track in database as pending
	err = cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, transaction)
	if err != nil {
		log.Errorf("[166] Failed to update asset column with pending upload: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Fallback to streaming upload
	c.JSON(http.StatusOK, gin.H{
		"upload_id":   uploadId,
		"upload_type": "stream",
		"upload_url": fmt.Sprintf("/asset/%s/%s/%s/%s/upload?operation=stream&upload_id=%s",
			typeName, resourceUuid, columnName, fileName, uploadId),
		"complete_url": fmt.Sprintf("/asset/%s/%s/%s/upload?operation=complete&upload_id=%s",
			typeName, resourceUuid, columnName, uploadId),
	})
}

// handleStreamUpload handles direct streaming upload to cloud storage
func handleStreamUpload(c *gin.Context, fileName string, assetCache *assetcachepojo.AssetFolderCache) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = uuid.New().String()
	}

	// Setup credentials using helper function
	setupCloudStorageCredentials(assetCache)

	// For local storage or when Rcat is not suitable, use traditional approach
	if isLocalStorage(assetCache) {
		// Write to local file
		localPath := filepath.Join(assetCache.LocalSyncPath, fileName)

		// Ensure directory exists
		os.MkdirAll(filepath.Dir(localPath), 0755)

		// Create file
		file, err := os.Create(localPath)
		if err != nil {
			log.Errorf("Failed to create local file: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Stream from request body to file
		written, err := io.Copy(file, c.Request.Body)
		if err != nil {
			log.Errorf("Failed to write file: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// Return upload info without DB update
		// DB will be updated in complete phase
		c.JSON(http.StatusOK, gin.H{
			"upload_id": uploadId,
			"status":    "uploaded",
			"size":      written,
			"message":   "File uploaded, call complete endpoint to finalize",
		})
		return
	}

	// Use rclone Rcat for streaming to cloud storage
	ctx := context.Background()

	// Parse destination filesystem
	fdst, err := fs.NewFs(ctx, assetCache.CloudStore.RootPath+"/"+assetCache.Keyname)
	if err != nil {
		log.Errorf("Failed to create destination filesystem: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Stream directly to cloud storage
	modTime := time.Now()

	// Track upload progress
	progressReader := &progressReader{
		reader:   c.Request.Body,
		total:    c.Request.ContentLength,
		uploadId: uploadId,
	}

	// Use Rcat to stream upload - wrap reader in io.NopCloser to satisfy ReadCloser interface
	// and provide empty metadata
	metadata := fs.Metadata{}
	_, err = operations.Rcat(ctx, fdst, fileName, io.NopCloser(progressReader), modTime, metadata)
	if err != nil {
		log.Errorf("[206] Failed to upload file via Rcat: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Return upload info without DB update
	// DB will be updated in complete phase
	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadId,
		"status":    "uploaded",
		"size":      progressReader.bytesRead,
		"message":   "File uploaded, call complete endpoint to finalize",
	})
}

// handleUploadComplete marks an upload as complete after client-side upload
func handleUploadComplete(c *gin.Context, cruds map[string]*resource.DbResource,
	typeName, columnName, fileName string, resourceUuid daptinid.DaptinReferenceId, transaction *sqlx.Tx) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = c.PostForm("upload_id")
	}

	// Get file size and type from request or metadata
	fileSize, _ := strconv.ParseInt(c.GetHeader("X-File-Size"), 10, 64)
	fileType := c.GetHeader("X-File-Type")
	if fileType == "" {
		fileType = c.GetHeader("Content-Type")
		if fileType == "" {
			fileType = "application/octet-stream"
		}
	}

	// Get additional metadata from client
	var metadata map[string]interface{}
	if err := c.ShouldBindJSON(&metadata); err != nil {
		metadata = make(map[string]interface{})
	}

	// Check if this is a multipart upload completion
	if parts, ok := metadata["parts"].([]interface{}); ok && len(parts) > 0 {
		// This is a multipart upload completion
		s3UploadId, _ := metadata["s3_upload_id"].(string)
		if s3UploadId == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "s3_upload_id is required for multipart completion",
			})
			return
		}

		// Get filename from metadata if not in query param
		if fileName == "" {
			if fn, ok := metadata["fileName"].(string); ok {
				fileName = fn
			}
		}

		if fileName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "fileName is required for multipart completion",
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

		// Update database with completed status
		err = cruds[typeName].UpdateAssetColumnStatus(resourceUuid, columnName, uploadId, "completed", metadata, transaction)
		if err != nil {
			log.Errorf("Failed to update asset column status: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"upload_id": uploadId,
			"status":    "completed",
			"fileName":  fileName,
			"size":      fileSize,
			"multipart": true,
		})
		return
	}

	// Extract file info from metadata if available
	if size, ok := metadata["size"].(float64); ok && fileSize == 0 {
		fileSize = int64(size)
	}
	if fType, ok := metadata["type"].(string); ok && fileType == "application/octet-stream" {
		fileType = fType
	}
	if fName, ok := metadata["fileName"].(string); ok && fileName == "" {
		fileName = fName
	}

	// Verify file exists in cloud storage (optional based on provider)
	assetCache := cruds["world"].AssetFolderCache[typeName][columnName]
	fileExists := verifyFileInCloud(assetCache, fileName)

	if !fileExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "file not found in cloud storage",
			"upload_id": uploadId,
		})
		return
	}

	// Update database with file information
	if uploadId != "" {
		// If we have an upload ID, update the pending upload
		err := cruds[typeName].UpdateAssetColumnStatus(resourceUuid, columnName, uploadId, "completed", metadata, transaction)
		if err != nil {
			log.Errorf("Failed to update asset column status: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	} else {
		// Direct update without upload ID (for stream uploads)
		err := cruds[typeName].UpdateAssetColumnWithFile(columnName, fileName, resourceUuid, fileSize, fileType, transaction)
		if err != nil {
			log.Errorf("Failed to update asset column: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

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

// verifyFileInCloud checks if a file exists in cloud storage
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

func verifyFileInCloud(assetCache *assetcachepojo.AssetFolderCache, fileName string) bool {
	// For local storage, check file existence
	if assetCache.CloudStore.StoreProvider == "local" {
		localPath := filepath.Join(assetCache.LocalSyncPath, fileName)
		_, err := os.Stat(localPath)
		return err == nil
	}

	// For cloud storage, use rclone to check
	ctx := context.Background()

	// Setup credentials using helper function
	setupCloudStorageCredentials(assetCache)

	// Check if file exists
	fsrc, err := fs.NewFs(ctx, assetCache.CloudStore.RootPath+"/"+assetCache.Keyname)
	if err != nil {
		return false
	}

	_, err = fsrc.NewObject(ctx, fileName)
	return err == nil
}

// getUploadPath constructs the full path for upload destination
func getUploadPath(assetCache *assetcachepojo.AssetFolderCache, fileName string) string {
	return assetCache.CloudStore.RootPath + "/" + assetCache.Keyname + "/" + fileName
}

// isLocalStorage checks if the storage provider is local filesystem
func isLocalStorage(assetCache *assetcachepojo.AssetFolderCache) bool {
	return assetCache.CloudStore.StoreProvider == "local"
}

// handleGetPartPresignedURL generates a presigned URL for a specific part in multipart upload
func handleGetPartPresignedURL(c *gin.Context, assetCache *assetcachepojo.AssetFolderCache) {
	uploadId := c.Query("upload_id")
	partNumberStr := c.Query("part_number")

	if uploadId == "" || partNumberStr == "" {
		c.AbortWithError(400, fmt.Errorf("upload_id and part_number are required"))
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
	fileName := c.Query("filename")
	if fileName == "" {
		c.AbortWithError(400, fmt.Errorf("filename is required"))
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
	log.Infof("Generating presigned URL for part %d - uploadId from query: %s, bucket: %s, key: %s",
		partNumber, uploadId, bucketName, keyPath)
	presignedUrl, err := GetS3PartPresignedURL(assetCache.Credentials, bucketName, keyPath, uploadId, int32(partNumber))
	if err != nil {
		log.Errorf("Failed to generate part presigned URL: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	log.Infof("Generated presigned URL: %s", presignedUrl)

	c.JSON(http.StatusOK, gin.H{
		"presigned_url": presignedUrl,
		"part_number":   partNumber,
		"expires_at":    time.Now().Add(3600 * time.Second).Unix(),
	})
}

// handleAbortMultipartUpload aborts an in-progress multipart upload
func handleAbortMultipartUpload(c *gin.Context, assetCache *assetcachepojo.AssetFolderCache) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = c.PostForm("upload_id")
	}

	if uploadId == "" {
		c.AbortWithError(400, fmt.Errorf("upload_id is required"))
		return
	}

	// Check if this is S3 storage
	if assetCache.Credentials == nil {
		c.AbortWithError(400, fmt.Errorf("abort not supported for this storage type"))
		return
	}

	providerType, ok := assetCache.Credentials["type"].(string)
	if !ok || providerType != "s3" {
		c.AbortWithError(400, fmt.Errorf("abort only supported for S3 storage"))
		return
	}

	// Extract bucket and key
	rootPath := assetCache.CloudStore.RootPath
	fileName := c.Query("filename")
	if fileName == "" {
		c.AbortWithError(400, fmt.Errorf("filename is required"))
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

	// Abort the multipart upload
	err := AbortS3MultipartUpload(assetCache.Credentials, bucketName, keyPath, uploadId)
	if err != nil {
		log.Errorf("Failed to abort multipart upload: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "aborted",
		"upload_id": uploadId,
	})
}
