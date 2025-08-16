package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/resource"
	log "github.com/sirupsen/logrus"
)

// generatePresignedURL generates presigned URLs for different cloud providers
func generatePresignedURL(assetCache *assetcachepojo.AssetFolderCache, fileName string, uploadId string) (map[string]interface{}, error) {
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

	// Check if this is S3 storage based on credentials
	if assetCache.Credentials != nil {
		if providerType, ok := assetCache.Credentials["type"].(string); ok && providerType == "s3" {
			// Extract bucket and key from RootPath
			rootPath := assetCache.CloudStore.RootPath
			keyPath := assetCache.Keyname + "/" + fileName

			// Parse bucket name from rootPath (format: "s3:bucket" or "bucket:")
			bucketName := ""
			if strings.Contains(rootPath, ":") {
				parts := strings.Split(rootPath, ":")
				if len(parts) >= 2 {
					bucketName = strings.TrimPrefix(parts[1], "/")
					// If there's a path after bucket, add it to keyPath
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
				return nil, fmt.Errorf("could not extract bucket name from root path: %s", rootPath)
			}

			return generateS3PresignedURL(assetCache.Credentials, bucketName, keyPath, uploadId)
		}
	}

	ctx := context.Background()

	// Create filesystem to determine provider type
	cloudPath := assetCache.CloudStore.RootPath + "/" + assetCache.Keyname
	_, err := fs.NewFs(ctx, cloudPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem: %v", err)
	}

	// For non-S3 providers, presigned URLs are not implemented
	// Return error to fallback to streaming upload
	return nil, fmt.Errorf("presigned URLs not yet implemented for this cloud storage provider")
}

// generateS3PresignedURL generates presigned URLs for S3-compatible storage
func generateS3PresignedURL(credentials map[string]interface{}, bucketName string, keyPath string, uploadId string) (map[string]interface{}, error) {
	// Extract S3 credentials from the credential map (rclone format)
	accessKeyID, ok := credentials["access_key_id"].(string)
	if !ok || accessKeyID == "" {
		return nil, fmt.Errorf("missing access_key_id in S3 credentials")
	}

	secretAccessKey, ok := credentials["secret_access_key"].(string)
	if !ok || secretAccessKey == "" {
		return nil, fmt.Errorf("missing secret_access_key in S3 credentials")
	}

	region, ok := credentials["region"].(string)
	if !ok || region == "" {
		region = "us-east-1" // Default region
	}

	endpoint, _ := credentials["endpoint"].(string)

	// Create AWS config with static credentials
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			awscredentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %v", err)
	}

	// Create S3 client options
	s3Options := func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // Use path-style for custom endpoints
		}
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg, s3Options)

	// Create S3 presign client
	presignClient := s3.NewPresignClient(s3Client)

	// Check if this is a multipart upload request
	if uploadId != "" {
		// For multipart uploads, we need to return multiple presigned URLs for parts
		// For now, we'll create a single part upload
		// In production, you'd want to calculate the number of parts based on file size

		// Generate presigned URL for uploading a part
		partNumber := int32(1)
		uploadPartRequest := &s3.UploadPartInput{
			Bucket:     aws.String(bucketName),
			Key:        aws.String(keyPath),
			UploadId:   aws.String(uploadId),
			PartNumber: aws.Int32(partNumber),
		}

		presignedReq, err := presignClient.PresignUploadPart(ctx, uploadPartRequest,
			func(opts *s3.PresignOptions) {
				opts.Expires = time.Duration(3600 * time.Second) // 1 hour expiry
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create presigned URL for multipart upload: %v", err)
		}

		return map[string]interface{}{
			"upload_type": "multipart",
			"upload_id":   uploadId,
			"parts": []map[string]interface{}{
				{
					"part_number":   partNumber,
					"presigned_url": presignedReq.URL,
					"headers":       presignedReq.SignedHeader,
				},
			},
			"complete_multipart_endpoint": fmt.Sprintf("/s3/complete-multipart"),
		}, nil
	}

	// Generate standard presigned PUT URL for single file upload
	putObjectRequest := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(keyPath),
	}

	presignedReq, err := presignClient.PresignPutObject(ctx, putObjectRequest,
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(3600 * time.Second) // 1 hour expiry
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create presigned URL: %v", err)
	}

	log.Infof("Generated S3 presigned URL for bucket: %s, key: %s", bucketName, keyPath)

	return map[string]interface{}{
		"upload_type":   "presigned",
		"presigned_url": presignedReq.URL,
		"method":        presignedReq.Method,
		"headers":       presignedReq.SignedHeader,
		"expires_at":    time.Now().Add(3600 * time.Second).Unix(),
	}, nil
}

// generateGCSSignedURL would generate signed URLs for Google Cloud Storage
// Requires GCS client library integration - not yet implemented
func generateGCSSignedURL(cloudPath string, fileName string, uploadId string) (map[string]interface{}, error) {
	// To implement GCS signed URLs:
	// 1. Import cloud.google.com/go/storage
	// 2. Extract service account credentials from rclone config
	// 3. Create storage client
	// 4. Generate signed URL using bucket.SignedURL

	return nil, fmt.Errorf("GCS signed URL generation not yet implemented")
}

// generateAzureSASURL would generate SAS tokens for Azure Blob Storage
// Requires Azure SDK integration - not yet implemented
func generateAzureSASURL(cloudPath string, fileName string, uploadId string) (map[string]interface{}, error) {
	// To implement Azure SAS URLs:
	// 1. Import github.com/Azure/azure-storage-blob-go
	// 2. Extract account key from rclone config
	// 3. Create SAS token with upload permissions
	// 4. Construct blob URL with SAS token

	return nil, fmt.Errorf("Azure SAS URL generation not yet implemented")
}

// CompleteMultipartUpload completes a multipart upload on S3
func CompleteMultipartUpload(cruds map[string]*resource.DbResource, bucket, key, awsUploadId string, parts []map[string]interface{}) error {
	// Get credentials from cruds - you'll need to pass the correct credential name
	// For now, this is a placeholder - you'd need to retrieve the appropriate credentials
	// based on the bucket/configuration

	// This would need to be enhanced to get the proper credentials
	// For now returning an error until credential retrieval is implemented
	return fmt.Errorf("S3 multipart upload completion requires credential retrieval implementation")
}

// CompleteS3MultipartUpload completes a multipart upload on S3 with provided credentials
func CompleteS3MultipartUpload(credentials map[string]interface{}, bucket, key, awsUploadId string, parts []map[string]interface{}) error {
	// Extract S3 credentials
	accessKeyID, ok := credentials["access_key_id"].(string)
	if !ok || accessKeyID == "" {
		return fmt.Errorf("missing access_key_id in S3 credentials")
	}

	secretAccessKey, ok := credentials["secret_access_key"].(string)
	if !ok || secretAccessKey == "" {
		return fmt.Errorf("missing secret_access_key in S3 credentials")
	}

	region, ok := credentials["region"].(string)
	if !ok || region == "" {
		region = "us-east-1"
	}

	endpoint, _ := credentials["endpoint"].(string)

	// Create AWS config
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			awscredentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %v", err)
	}

	// Create S3 client
	s3Options := func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	}
	s3Client := s3.NewFromConfig(cfg, s3Options)

	// Convert parts to CompletedPart format
	var completedParts []types.CompletedPart
	for _, part := range parts {
		partNumber, ok := part["part_number"].(int32)
		if !ok {
			if pn, ok := part["part_number"].(float64); ok {
				partNumber = int32(pn)
			} else {
				continue
			}
		}

		etag, ok := part["etag"].(string)
		if !ok {
			continue
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       aws.String(etag),
			PartNumber: aws.Int32(partNumber),
		})
	}

	// Complete the multipart upload
	_, err = s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(awsUploadId),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %v", err)
	}

	log.Infof("Successfully completed S3 multipart upload for bucket: %s, key: %s, uploadId: %s", bucket, key, awsUploadId)
	return nil
}

// AbortMultipartUpload aborts a multipart upload on S3
func AbortMultipartUpload(bucket, key, awsUploadId string) error {
	// This would need credentials to be passed in or retrieved
	return fmt.Errorf("S3 multipart upload abort requires credential retrieval implementation")
}

// AbortS3MultipartUpload aborts a multipart upload on S3 with provided credentials
func AbortS3MultipartUpload(credentials map[string]interface{}, bucket, key, awsUploadId string) error {
	// Extract S3 credentials
	accessKeyID, ok := credentials["access_key_id"].(string)
	if !ok || accessKeyID == "" {
		return fmt.Errorf("missing access_key_id in S3 credentials")
	}

	secretAccessKey, ok := credentials["secret_access_key"].(string)
	if !ok || secretAccessKey == "" {
		return fmt.Errorf("missing secret_access_key in S3 credentials")
	}

	region, ok := credentials["region"].(string)
	if !ok || region == "" {
		region = "us-east-1"
	}

	endpoint, _ := credentials["endpoint"].(string)

	// Create AWS config
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			awscredentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %v", err)
	}

	// Create S3 client
	s3Options := func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	}
	s3Client := s3.NewFromConfig(cfg, s3Options)

	// Abort the multipart upload
	_, err = s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(awsUploadId),
	})

	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %v", err)
	}

	log.Infof("Successfully aborted S3 multipart upload for bucket: %s, key: %s, uploadId: %s", bucket, key, awsUploadId)
	return nil
}
