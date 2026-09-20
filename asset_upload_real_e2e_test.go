package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const assetUploadE2ESchema = `
Tables:
  - TableName: asset_upload_probe
    Permission: 2097151
    DefaultPermission: 0
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false
      - Name: attachment
        DataType: text
        ColumnType: file
        IsNullable: true
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: asset-upload-e2e-store
          KeyName: uploaded
`

const assetUploadLocalE2ESchema = `
Tables:
  - TableName: asset_upload_probe
    Permission: 2097151
    DefaultPermission: 0
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false
      - Name: attachment
        DataType: text
        ColumnType: file
        IsNullable: true
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: asset-upload-local-e2e-store
          KeyName: uploaded
`

func TestAssetUploadLocalRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	usedPorts := map[int]bool{}
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	olricPort := freeTransportE2EPortPair(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	root := t.TempDir()
	options := transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "asset-upload-local.db"), olricPort: olricPort, schema: assetUploadLocalE2ESchema}
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	client := &http.Client{Timeout: 30 * time.Second}
	token := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	accessGroupsE2ECreateRecord(t, client, baseURL, token, "cloud_store", map[string]interface{}{
		"name": "asset-upload-local-e2e-store", "store_type": "local", "store_provider": "local",
		"root_path": root, "store_parameters": "{}",
	})
	process.stopProcess()
	process = startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer process.stopProcess()
	signin := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/user_account/signin", "",
		map[string]interface{}{"attributes": map[string]interface{}{"email": "admin@test.local", "password": "testpass123"}}, http.StatusOK)
	token, _ = accessGroupsE2EFindString(signin, "value")
	invalidCreate, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{
		"type": "asset_upload_probe", "attributes": map[string]interface{}{
			"title": "invalid asset", "attachment": []map[string]interface{}{{"name": "invalid.txt", "file": "data:text/plain;base64,@@@not-base64@@@"}},
		},
	}})
	requestAssetE2E(t, client, http.MethodPost, baseURL+"/api/asset_upload_probe", token,
		bytes.NewReader(invalidCreate), "application/vnd.api+json", http.StatusBadRequest)
	traversalCreate, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{
		"type": "asset_upload_probe", "attributes": map[string]interface{}{
			"title": "traversal asset", "attachment": []map[string]interface{}{{"name": "../../escape.txt", "file": "data:text/plain;base64,dGVzdA=="}},
		},
	}})
	requestAssetE2E(t, client, http.MethodPost, baseURL+"/api/asset_upload_probe", token,
		bytes.NewReader(traversalCreate), "application/vnd.api+json", http.StatusBadRequest)
	if rows := requestAssetE2E(t, client, http.MethodGet, baseURL+"/api/asset_upload_probe", token, nil, "", http.StatusOK); strings.Contains(string(rows), "invalid asset") || strings.Contains(string(rows), "traversal asset") {
		t.Fatalf("rejected create committed a row: %s", rows)
	}
	if _, err := os.Stat(filepath.Join(root, "uploaded", "invalid.txt")); !os.IsNotExist(err) {
		t.Fatalf("invalid create wrote an object: %v", err)
	}
	validID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "asset_upload_probe", map[string]interface{}{
		"title": "valid inline asset", "attachment": []map[string]interface{}{{"name": "inline.txt", "file": "data:text/plain;base64,aGVsbG8="}},
	})
	validRow := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/asset_upload_probe/"+validID, token, nil, http.StatusOK)
	validJSON, _ := json.Marshal(validRow)
	if !strings.Contains(string(validJSON), `"size":5`) || !strings.Contains(string(validJSON), `"md5":"5d41402abc4b2a76b9719d911017c592"`) {
		t.Fatalf("valid asset metadata was not computed from decoded bytes: %s", validJSON)
	}
	waitAssetE2E(t, func() bool {
		content, err := os.ReadFile(filepath.Join(root, "uploaded", "inline.txt"))
		return err == nil && string(content) == "hello"
	})
	rowID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "asset_upload_probe", map[string]interface{}{"title": "local asset"})
	invalidUpdate, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{
		"type": "asset_upload_probe", "id": rowID, "attributes": map[string]interface{}{
			"attachment": []map[string]interface{}{{"name": "invalid.txt", "file": "data:text/plain;base64,@@@not-base64@@@"}},
		},
	}})
	requestAssetE2E(t, client, http.MethodPatch, baseURL+"/api/asset_upload_probe/"+rowID, token,
		bytes.NewReader(invalidUpdate), "application/vnd.api+json", http.StatusBadRequest)
	row := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/asset_upload_probe/"+rowID, token, nil, http.StatusOK)
	rowJSON, _ := json.Marshal(row)
	if strings.Contains(string(rowJSON), "invalid.txt") {
		t.Fatalf("invalid update committed metadata: %s", rowJSON)
	}
	assetURL := baseURL + "/asset/asset_upload_probe/" + rowID + "/attachment"
	init := requestAssetE2E(t, client, http.MethodPost, assetURL+"/upload?operation=init&filename=local.txt", token, nil, "", http.StatusOK)
	if !strings.Contains(string(init), `"upload_type":"stream"`) {
		t.Fatalf("local init did not select stream: %s", init)
	}
	requestAssetE2E(t, client, http.MethodPost, assetURL+"/upload?operation=stream&filename=local.txt", token,
		strings.NewReader("local content"), "text/plain", http.StatusOK)
	if body := requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=local.txt", token, nil, "", http.StatusOK); string(body) != "local content" {
		t.Fatalf("local GET body = %q", body)
	}
	filePath := filepath.Join(root, "uploaded", "local.txt")
	if body, err := os.ReadFile(filePath); err != nil || string(body) != "local content" {
		t.Fatalf("local stored file = %q, %v", body, err)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=local.txt", token, nil, "", http.StatusAccepted)
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=local.txt", token, nil, "", http.StatusNoContent)
	requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=local.txt", token, nil, "", http.StatusNotFound)
	waitAssetE2E(t, func() bool { _, err := os.Stat(filePath); return os.IsNotExist(err) })
}

func TestAssetUploadRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	endpoint := os.Getenv("DAPTIN_ASSET_E2E_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set DAPTIN_ASSET_E2E_S3_ENDPOINT to an isolated S3-compatible store")
	}
	accessKey := os.Getenv("DAPTIN_ASSET_E2E_S3_ACCESS_KEY")
	secretKey := os.Getenv("DAPTIN_ASSET_E2E_S3_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		t.Skip("set isolated S3 test credentials")
	}
	internalEndpoint := os.Getenv("DAPTIN_ASSET_E2E_S3_INTERNAL_ENDPOINT")
	if internalEndpoint == "" {
		internalEndpoint = endpoint
	}
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	if err != nil {
		t.Fatal(err)
	}
	s3Client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	bucket := fmt.Sprintf("asset-upload-e2e-%d", time.Now().UnixNano())
	if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, key := range []string{"uploaded/multipart.txt", "uploaded/presigned.txt", "uploaded/large.txt", "uploaded/unattached.txt", "uploaded/a/shared.txt", "uploaded/b/shared.txt", "uploaded/extensionless"} {
			_, _ = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		}
		_, _ = s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	}()

	usedPorts := map[int]bool{}
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	olricPort := freeTransportE2EPortPair(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	databasePath := filepath.Join(t.TempDir(), "asset-upload.db")
	options := transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort, schema: assetUploadE2ESchema}
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	client := &http.Client{Timeout: 30 * time.Second}
	token := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	credentialContent, _ := json.Marshal(map[string]interface{}{
		"type": "s3", "provider": "Minio", "env_auth": "false", "access_key_id": accessKey,
		"secret_access_key": secretKey, "endpoint": internalEndpoint, "public_endpoint": endpoint, "region": "us-east-1",
	})
	accessGroupsE2ECreateRecord(t, client, baseURL, token, "credential", map[string]interface{}{
		"name": "asset-upload-e2e-credential", "content": string(credentialContent),
	})
	accessGroupsE2ECreateRecord(t, client, baseURL, token, "cloud_store", map[string]interface{}{
		"name": "asset-upload-e2e-store", "store_type": "s3", "store_provider": "s3",
		"credential_name": "asset-upload-e2e-credential", "root_path": "asset-upload-e2e-store:" + bucket,
		"store_parameters": "{}",
	})
	process.stopProcess()
	process = startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer process.stopProcess()
	signin := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/user_account/signin", "",
		map[string]interface{}{"attributes": map[string]interface{}{"email": "admin@test.local", "password": "testpass123"}}, http.StatusOK)
	token, _ = accessGroupsE2EFindString(signin, "value")
	if token == "" {
		t.Fatal("admin token missing after restart")
	}
	rowID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "asset_upload_probe", map[string]interface{}{"title": "asset"})
	assetURL := baseURL + "/asset/asset_upload_probe/" + rowID + "/attachment"

	requestAssetE2E(t, client, http.MethodPost, assetURL+"/upload", "", assetMultipartBody(t, "multipart.txt", "multipart content"),
		"multipart/form-data", http.StatusForbidden)
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/multipart.txt")}); err == nil {
		t.Fatal("denied upload wrote an object")
	}
	requestAssetE2E(t, client, http.MethodPost, assetURL+"/upload", token, assetMultipartBody(t, "multipart.txt", "multipart content"),
		"multipart/form-data", http.StatusOK)
	if body := requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=multipart.txt", token, nil, "", http.StatusOK); string(body) != "multipart content" {
		t.Fatalf("multipart GET body = %q", body)
	}
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/multipart.txt")}); err != nil {
		t.Fatalf("multipart object missing: %v", err)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?operation=abort&file=multipart.txt", token, nil, "", http.StatusBadRequest)
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/multipart.txt")}); err != nil {
		t.Fatalf("obsolete abort operation affected stored asset: %v", err)
	}

	initURL := assetURL + "/upload?operation=init&filename=presigned.txt"
	initResponse := requestAssetE2E(t, client, http.MethodPost, initURL, token, nil, "", http.StatusOK)
	var upload struct {
		PresignedData struct {
			PresignedURL string `json:"presigned_url"`
		} `json:"presigned_data"`
		CompleteURL string `json:"complete_url"`
	}
	if err := json.Unmarshal(initResponse, &upload); err != nil {
		t.Fatal(err)
	}
	if upload.PresignedData.PresignedURL == "" || upload.CompleteURL == "" {
		t.Fatalf("incomplete presigned response: %s", initResponse)
	}
	if !strings.HasPrefix(upload.PresignedData.PresignedURL, endpoint+"/") {
		t.Fatalf("presigned URL uses the wrong client endpoint: %q", upload.PresignedData.PresignedURL)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=presigned.txt", token, nil, "", http.StatusBadRequest)
	requestAssetE2E(t, client, http.MethodPut, upload.PresignedData.PresignedURL, "", strings.NewReader("presigned content"),
		"", http.StatusOK)
	requestAssetE2E(t, client, http.MethodPost, baseURL+upload.CompleteURL, token, nil, "", http.StatusOK)
	requestAssetE2E(t, client, http.MethodPost, baseURL+upload.CompleteURL, token, nil, "", http.StatusNotFound)
	if body := requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=presigned.txt", token, nil, "", http.StatusOK); string(body) != "presigned content" {
		t.Fatalf("presigned GET body = %q", body)
	}
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/presigned.txt")}); err != nil {
		t.Fatalf("presigned object missing: %v", err)
	}
	largeInit, err := http.NewRequest(http.MethodPost, assetURL+"/upload?operation=init&filename=large.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	largeInit.Header.Set("Authorization", "Bearer "+token)
	largeInit.Header.Set("X-File-Size", "104857601")
	largeResponse, err := client.Do(largeInit)
	if err != nil {
		t.Fatal(err)
	}
	largeBody, _ := io.ReadAll(largeResponse.Body)
	_ = largeResponse.Body.Close()
	if largeResponse.StatusCode != http.StatusOK {
		t.Fatalf("multipart init returned %d: %s", largeResponse.StatusCode, largeBody)
	}
	var largeUpload struct {
		UploadType  string `json:"upload_type"`
		GetPartURL  string `json:"get_part_url"`
		CompleteURL string `json:"complete_url"`
	}
	if err := json.Unmarshal(largeBody, &largeUpload); err != nil {
		t.Fatal(err)
	}
	if largeUpload.UploadType != "multipart" || largeUpload.GetPartURL == "" {
		t.Fatalf("multipart initiation was incomplete: %s", largeBody)
	}
	partBody := requestAssetE2E(t, client, http.MethodGet, baseURL+largeUpload.GetPartURL+"&part_number=1", token, nil, "", http.StatusOK)
	var part struct {
		PresignedURL string `json:"presigned_url"`
	}
	if err := json.Unmarshal(partBody, &part); err != nil {
		t.Fatal(err)
	}
	partRequest, err := http.NewRequest(http.MethodPut, part.PresignedURL, strings.NewReader("multipart part"))
	if err != nil {
		t.Fatal(err)
	}
	partResponse, err := client.Do(partRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, partResponse.Body)
	_ = partResponse.Body.Close()
	if partResponse.StatusCode != http.StatusOK {
		t.Fatalf("multipart part PUT returned %d", partResponse.StatusCode)
	}
	etag := partResponse.Header.Get("ETag")
	if etag == "" {
		t.Fatal("multipart part response has no ETag")
	}
	completeBody, _ := json.Marshal(map[string]interface{}{"parts": []map[string]interface{}{{"part_number": 1, "etag": etag}}})
	requestAssetE2E(t, client, http.MethodPost, baseURL+largeUpload.CompleteURL, token, bytes.NewReader(completeBody), "application/json", http.StatusOK)
	if body := requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=large.txt", token, nil, "", http.StatusOK); string(body) != "multipart part" {
		t.Fatalf("S3 multipart GET body = %q", body)
	}

	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=multipart.txt", "", nil, "", http.StatusForbidden)
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/multipart.txt")}); err != nil {
		t.Fatalf("denied delete removed object: %v", err)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=multipart.txt", token, nil, "", http.StatusAccepted)
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=multipart.txt", token, nil, "", http.StatusNoContent)
	requestAssetE2E(t, client, http.MethodGet, assetURL+"?file=multipart.txt", token, nil, "", http.StatusNotFound)
	row := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/asset_upload_probe/"+rowID, token, nil, http.StatusOK)
	rowJSON, _ := json.Marshal(row)
	if strings.Contains(string(rowJSON), "multipart.txt") || !strings.Contains(string(rowJSON), "presigned.txt") {
		t.Fatalf("row attachment metadata was not updated by DELETE: %s", rowJSON)
	}
	waitAssetE2E(t, func() bool {
		_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/multipart.txt")})
		return err != nil
	})
	if _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/unattached.txt"), Body: strings.NewReader("not attached")}); err != nil {
		t.Fatal(err)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=unattached.txt", token, nil, "", http.StatusNoContent)
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/unattached.txt")}); err != nil {
		t.Fatalf("delete touched object not attached to row: %v", err)
	}
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=presigned.txt", token, nil, "", http.StatusAccepted)
	requestAssetE2E(t, client, http.MethodDelete, assetURL+"/upload?file=large.txt", token, nil, "", http.StatusAccepted)
	for _, key := range []string{"uploaded/presigned.txt", "uploaded/large.txt"} {
		key := key
		waitAssetE2E(t, func() bool {
			_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
			return err != nil
		})
	}
	requestAssetE2E(t, client, http.MethodGet, assetURL, token, nil, "", http.StatusNotFound)
	for _, key := range []string{"uploaded/a/shared.txt", "uploaded/b/shared.txt"} {
		if _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: strings.NewReader(key)}); err != nil {
			t.Fatal(err)
		}
	}
	duplicateID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "asset_upload_probe", map[string]interface{}{
		"title": "same names", "attachment": []map[string]interface{}{
			{"name": "shared.txt", "path": "a", "type": "text/plain", "size": 1},
			{"name": "shared.txt", "path": "b", "type": "text/plain", "size": 1},
		},
	})
	duplicateURL := baseURL + "/asset/asset_upload_probe/" + duplicateID + "/attachment"
	requestAssetE2E(t, client, http.MethodDelete, duplicateURL+"/upload?file=shared.txt", token, nil, "", http.StatusBadRequest)
	requestAssetE2E(t, client, http.MethodDelete, duplicateURL+"/upload?file=shared.txt&path=a&index=0", token, nil, "", http.StatusAccepted)
	requestAssetE2E(t, client, http.MethodDelete, duplicateURL+"/upload?file=shared.txt&path=a&index=0", token, nil, "", http.StatusNoContent)
	waitAssetE2E(t, func() bool {
		_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/a/shared.txt")})
		return err != nil
	})
	if _, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/b/shared.txt")}); err != nil {
		t.Fatalf("other duplicate object was deleted: %v", err)
	}
	requestAssetE2E(t, client, http.MethodDelete, duplicateURL+"/upload?file=shared.txt&path=b", token, nil, "", http.StatusAccepted)
	waitAssetE2E(t, func() bool {
		_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/b/shared.txt")})
		return err != nil
	})
	if _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/extensionless"), Body: strings.NewReader("content")}); err != nil {
		t.Fatal(err)
	}
	extensionlessID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "asset_upload_probe", map[string]interface{}{
		"title": "extensionless", "attachment": []map[string]interface{}{{"name": "extensionless", "type": "text/plain", "size": 7}},
	})
	extensionlessURL := baseURL + "/asset/asset_upload_probe/" + extensionlessID + "/attachment"
	requestAssetE2E(t, client, http.MethodDelete, extensionlessURL+"/upload?file=extensionless", token, nil, "", http.StatusAccepted)
	waitAssetE2E(t, func() bool {
		_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("uploaded/extensionless")})
		return err != nil
	})
}

func waitAssetE2E(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("background asset deletion did not complete")
}

func assetMultipartBody(t *testing.T, name, content string) io.Reader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	header.Set("Content-Type", "text/plain")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, content)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &assetE2EMultipartReader{Reader: &body, contentType: writer.FormDataContentType()}
}

type assetE2EMultipartReader struct {
	io.Reader
	contentType string
}

func requestAssetE2E(t *testing.T, client *http.Client, method, requestURL, token string, body io.Reader, contentType string, want int) []byte {
	t.Helper()
	request, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if multipartBody, ok := body.(*assetE2EMultipartReader); ok {
		contentType = multipartBody.contentType
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != want {
		t.Fatalf("%s %s returned %d, want %d: %s\n%s", method, requestURL, response.StatusCode, want, responseBody, response.Header)
	}
	return responseBody
}
