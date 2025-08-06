package server

import (
	"context"
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
		fileName := c.Param("filename")
		operation := c.Query("operation") // append, replace, init

		uuidDir := daptinid.InterfaceToDIR(resourceUuid)
		if uuidDir == daptinid.NullReferenceId {
			c.AbortWithStatus(404)
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

		transaction, err := dbResource.Connection().Beginx()
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer transaction.Rollback()

		user := c.Request.Context().Value("user")
		sessionUser := &auth.SessionUser{}

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		permission := dbResource.GetRowPermissionWithTransaction(originalRowReference, transaction)
		if !permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		switch operation {
		case "init":
			// Initialize upload session and return presigned URL if supported
			handleUploadInit(c, cruds, typeName, columnName, fileName, uuidDir, assetCache, transaction)
		case "stream":
			// Handle streaming upload
			handleStreamUpload(c, cruds, typeName, columnName, fileName, uuidDir, assetCache, transaction)
		case "complete":
			// Mark upload as complete
			handleUploadComplete(c, cruds, typeName, columnName, fileName, uuidDir, transaction)
		default:
			// Default to streaming upload
			handleStreamUpload(c, cruds, typeName, columnName, fileName, uuidDir, assetCache, transaction)
		}
		transaction.Commit()
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

	// Try to generate presigned URL based on storage provider
	presignedData, err := generatePresignedURL(assetCache, fileName, uploadId)

	if err == nil && presignedData != nil {
		// Presigned URL available - update database with pending upload
		cruds[typeName].UpdateAssetColumnWithPendingUpload(resourceUuid, columnName, fileName, uploadId, fileSize, fileType, transaction)

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
func handleStreamUpload(c *gin.Context, cruds map[string]*resource.DbResource, typeName,
	columnName, fileName string, resourceUuid daptinid.DaptinReferenceId, assetCache *assetcachepojo.AssetFolderCache, transaction *sqlx.Tx) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = uuid.New().String()
	}

	// Setup credentials
	configSetName := assetCache.CloudStore.Name
	if strings.Contains(assetCache.CloudStore.RootPath, ":") {
		configSetName = strings.Split(assetCache.CloudStore.RootPath, ":")[0]
	}

	if assetCache.Credentials != nil {
		for key, val := range assetCache.Credentials {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
		}
	}

	// Determine upload path
	//cloudPath := assetCache.CloudStore.RootPath + "/" + assetCache.Keyname + "/" + fileName

	// For local storage or when Rcat is not suitable, use traditional approach
	if assetCache.CloudStore.StoreProvider == "local" {
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

		// Update database
		err = cruds[typeName].UpdateAssetColumnWithFile(columnName, fileName, resourceUuid, written, c.ContentType(), transaction)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"upload_id": uploadId,
			"status":    "completed",
			"size":      written,
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

	// Update database with file info
	err = cruds[typeName].UpdateAssetColumnWithFile(columnName, fileName, resourceUuid, progressReader.bytesRead, c.ContentType(), transaction)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadId,
		"status":    "completed",
		"size":      progressReader.bytesRead,
	})
}

// handleUploadComplete marks an upload as complete after client-side upload
func handleUploadComplete(c *gin.Context, cruds map[string]*resource.DbResource,
	typeName, columnName, fileName string, resourceUuid daptinid.DaptinReferenceId, transaction *sqlx.Tx) {
	uploadId := c.Query("upload_id")
	if uploadId == "" {
		uploadId = c.PostForm("upload_id")
	}

	// Get additional metadata from client
	var metadata map[string]interface{}
	if err := c.ShouldBindJSON(&metadata); err != nil {
		metadata = make(map[string]interface{})
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

	// Update database to mark upload as complete
	err := cruds[typeName].UpdateAssetColumnStatus(resourceUuid, columnName, uploadId, "completed", metadata, transaction)
	if err != nil {
		log.Errorf("Failed to update asset column status: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadId,
		"status":    "completed",
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
func verifyFileInCloud(assetCache *assetcachepojo.AssetFolderCache, fileName string) bool {
	// For local storage, check file existence
	if assetCache.CloudStore.StoreProvider == "local" {
		localPath := filepath.Join(assetCache.LocalSyncPath, fileName)
		_, err := os.Stat(localPath)
		return err == nil
	}

	// For cloud storage, use rclone to check
	ctx := context.Background()

	// Setup credentials
	configSetName := assetCache.CloudStore.Name
	if strings.Contains(assetCache.CloudStore.RootPath, ":") {
		configSetName = strings.Split(assetCache.CloudStore.RootPath, ":")[0]
	}

	if assetCache.Credentials != nil {
		for key, val := range assetCache.Credentials {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
		}
	}

	// Check if file exists
	fsrc, err := fs.NewFs(ctx, assetCache.CloudStore.RootPath+"/"+assetCache.Keyname)
	if err != nil {
		return false
	}

	_, err = fsrc.NewObject(ctx, fileName)
	return err == nil
}
