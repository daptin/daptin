package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
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
	"github.com/jlaffaye/ftp"
)

func TestFTPStorageS3RealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	endpoint := os.Getenv("DAPTIN_ASSET_E2E_S3_ENDPOINT")
	accessKey := os.Getenv("DAPTIN_ASSET_E2E_S3_ACCESS_KEY")
	secretKey := os.Getenv("DAPTIN_ASSET_E2E_S3_SECRET_KEY")
	if endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("set isolated S3 E2E endpoint and credentials")
	}
	internalEndpoint := os.Getenv("DAPTIN_ASSET_E2E_S3_INTERNAL_ENDPOINT")
	if internalEndpoint == "" {
		internalEndpoint = endpoint
	}
	ctx := context.Background()
	config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	if err != nil {
		t.Fatal(err)
	}
	store := s3.NewFromConfig(config, func(options *s3.Options) { options.BaseEndpoint = aws.String(endpoint); options.UsePathStyle = true })
	bucket := fmt.Sprintf("ftp-e2e-%d", time.Now().UnixNano())
	if _, err := store.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, key := range []string{"ftp-site/seed.txt", "ftp-site/proof.txt", "ftp-site/renamed.txt", "ftp-site/nested/child.txt", "ftp-site/moved/child.txt"} {
			_, _ = store.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		}
		_, _ = store.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	}()
	if _, err := store.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String("ftp-site/seed.txt"), Body: strings.NewReader("seed")}); err != nil {
		t.Fatal(err)
	}
	readBacking := func(key string) string {
		t.Helper()
		response, err := store.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String("ftp-site/" + key)})
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	used := map[int]bool{}
	databasePath := filepath.Join(t.TempDir(), "ftp-s3.db")
	ftpPort := freeTransportE2EPort(t, used)
	client := &http.Client{Timeout: 30 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, used)
		httpsPort := freeTransportE2EPort(t, used)
		olricPort := freeTransportE2EPortPair(t, used)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		return base, startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort})
	}
	base, process := start()
	token := accessGroupsE2ESignupSigninAdmin(t, client, base)
	credentialContent, _ := json.Marshal(map[string]interface{}{
		"type": "s3", "provider": "Minio", "env_auth": "false", "access_key_id": accessKey, "secret_access_key": secretKey,
		"endpoint": internalEndpoint, "public_endpoint": endpoint, "region": "us-east-1",
	})
	accessGroupsE2ECreateRecord(t, client, base, token, "credential", map[string]interface{}{"name": "ftp-e2e-credential", "content": string(credentialContent)})
	storeID := accessGroupsE2ECreateRecord(t, client, base, token, "cloud_store", map[string]interface{}{
		"name": "ftp-e2e-store", "store_type": "s3", "store_provider": "s3", "credential_name": "ftp-e2e-credential",
		"root_path": "ftp-e2e-store:" + bucket, "store_parameters": "{}",
	})
	siteID := accessGroupsE2ECreateRecord(t, client, base, token, "site", map[string]interface{}{
		"name": "ftp-s3-site", "hostname": "ftp-s3.example.test", "path": "ftp-site", "enable": true, "ftp_enabled": true, "site_type": "static",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/site/"+siteID, token, map[string]interface{}{
		"data": map[string]interface{}{"type": "site", "id": siteID, "relationships": map[string]interface{}{
			"cloud_store_id": map[string]interface{}{"data": map[string]interface{}{"type": "cloud_store", "id": storeID}},
		}},
	}, http.StatusOK)
	ftpE2ESetConfig(t, client, base, token, "ftp.listen_interface", fmt.Sprintf("127.0.0.1:%d", ftpPort))
	ftpE2ESetConfig(t, client, base, token, "ftp.enable", "true")
	process.stopProcess()
	base, process = start()
	connection := ftpE2EConnectSite(t, ftpPort, "ftp-s3.example.test")
	if err := connection.ChangeDir("missing-directory"); err == nil {
		t.Fatal("S3 CWD accepted a nonexistent directory")
	}
	ftpE2EAssertRetr(t, connection, "seed.txt", "seed")
	ftpE2EPrimeSiteMiss(t, client, base, "ftp-s3.example.test", "/proof.txt")
	if err := connection.Stor("proof.txt", strings.NewReader("remote")); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("proof.txt"); got != "remote" {
		t.Fatalf("S3 upload=%q", got)
	}
	if size, err := connection.FileSize("proof.txt"); err != nil || size != 6 {
		t.Fatalf("S3 SIZE=%d, %v", size, err)
	}
	ftpE2EAssertRetrFrom(t, connection, "proof.txt", 2, "mote")
	ftpE2EAssertSiteHTTP(t, client, base, "ftp-s3.example.test", "/proof.txt", "remote")
	if err := connection.Append("proof.txt", strings.NewReader(" append")); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("proof.txt"); got != "remote append" {
		t.Fatalf("S3 append=%q", got)
	}
	ftpE2EAssertSiteHTTP(t, client, base, "ftp-s3.example.test", "/proof.txt", "remote append")
	if err := connection.StorFrom("proof.txt", strings.NewReader("FTP"), 7); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("proof.txt"); got != "remote FTPend" {
		t.Fatalf("S3 resume=%q", got)
	}
	if err := connection.Rename("proof.txt", "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("renamed.txt"); got != "remote FTPend" {
		t.Fatalf("S3 renamed=%q", got)
	}
	if err := connection.Delete("renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("ftp-site/renamed.txt")}); err == nil {
		t.Fatal("deleted S3 object remains")
	}
	if err := connection.MakeDir("empty"); err == nil {
		t.Fatal("S3 accepted an empty directory it cannot preserve")
	}
	if err := connection.NoOp(); err != nil {
		t.Fatalf("FTP after rejected mkdir: %v", err)
	}
	if err := connection.Stor("nested/child.txt", strings.NewReader("nested")); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("nested/child.txt"); got != "nested" {
		t.Fatalf("nested S3 upload=%q", got)
	}
	entries, err := connection.List("nested")
	if err != nil || !ftpE2EHasEntry(entries, "child.txt") {
		t.Fatalf("S3 directory listing: %#v, %v", entries, err)
	}
	if err := connection.RemoveDir("nested"); err == nil {
		t.Fatal("S3 removed a nonempty directory")
	}
	if got := readBacking("nested/child.txt"); got != "nested" {
		t.Fatalf("failed S3 RMD changed child=%q", got)
	}
	if err := connection.Rename("nested", "nested/inside"); err == nil {
		t.Fatal("S3 moved a directory into itself")
	}
	if got := readBacking("nested/child.txt"); got != "nested" {
		t.Fatalf("failed S3 rename changed child=%q", got)
	}
	if err := connection.Rename("nested", "moved"); err != nil {
		t.Fatal(err)
	}
	if got := readBacking("moved/child.txt"); got != "nested" {
		t.Fatalf("moved S3 directory=%q", got)
	}
	if _, err := store.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("ftp-site/nested/child.txt")}); err == nil {
		t.Fatal("directory move left source object")
	}
	_ = connection.Quit()
	process.stopProcess()
	base, process = start()
	defer process.stopProcess()
	connection = ftpE2EConnectSite(t, ftpPort, "ftp-s3.example.test")
	defer connection.Quit()
	ftpE2EAssertRetr(t, connection, "moved/child.txt", "nested")
	ftpE2EAssertSiteHTTP(t, client, base, "ftp-s3.example.test", "/moved/child.txt", "nested")
	if got := readBacking("moved/child.txt"); got != "nested" {
		t.Fatalf("S3 object after restart=%q", got)
	}
}

func TestFTPStorageLocalRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	used := map[int]bool{}
	databasePath := filepath.Join(t.TempDir(), "ftp.db")
	storageRoot := t.TempDir()
	siteRoot := filepath.Join(storageRoot, "ftp-site")
	if err := os.Mkdir(siteRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteRoot, "seed.txt"), []byte("seed"), 0644); err != nil {
		t.Fatal(err)
	}
	ftpPort := freeTransportE2EPort(t, used)
	client := &http.Client{Timeout: 30 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, used)
		httpsPort := freeTransportE2EPort(t, used)
		olricPort := freeTransportE2EPortPair(t, used)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		return base, startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort})
	}
	base, process := start()
	token := accessGroupsE2ESignupSigninAdmin(t, client, base)
	storeID := accessGroupsE2ECreateRecord(t, client, base, token, "cloud_store", map[string]interface{}{
		"name": "ftp-local-store", "store_type": "local", "store_provider": "local", "root_path": storageRoot, "store_parameters": "{}",
	})
	siteID := accessGroupsE2ECreateRecord(t, client, base, token, "site", map[string]interface{}{
		"name": "ftp-local-site", "hostname": "ftp.example.test", "path": "ftp-site", "enable": true, "ftp_enabled": true, "site_type": "static",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/site/"+siteID, token, map[string]interface{}{
		"data": map[string]interface{}{"type": "site", "id": siteID, "relationships": map[string]interface{}{
			"cloud_store_id": map[string]interface{}{"data": map[string]interface{}{"type": "cloud_store", "id": storeID}},
		}},
	}, http.StatusOK)
	ftpE2ESetConfig(t, client, base, token, "ftp.listen_interface", fmt.Sprintf("127.0.0.1:%d", ftpPort))
	ftpE2ESetConfig(t, client, base, token, "ftp.enable", "true")
	process.stopProcess()
	base, process = start()
	conn := ftpE2EConnect(t, ftpPort)
	defer conn.Quit()
	ftpE2EAssertNoPassiveReply(t, ftpPort)
	seedInfo, err := os.Stat(filepath.Join(siteRoot, "seed.txt"))
	if err != nil || seedInfo.Mode().Perm() != 0640 {
		t.Fatalf("FTP CHMOD backing mode: %v, %v", seedInfo, err)
	}
	entries, err := conn.List(".")
	if err != nil {
		t.Fatal(err)
	}
	if !ftpE2EHasEntry(entries, "seed.txt") {
		t.Fatalf("FTP did not list backing file: %#v", entries)
	}
	ftpE2EAssertRetr(t, conn, "seed.txt", "seed")
	ftpE2EPrimeSiteMiss(t, client, base, "ftp.example.test", "/proof.txt")
	if err := conn.Stor("proof.txt", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	if err := conn.Append("append-new.txt", strings.NewReader("created by APPE")); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "append-new.txt"), "created by APPE")
	if err := conn.Delete("append-new.txt"); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "proof.txt"), "hello")
	ftpE2EAssertRetr(t, conn, "proof.txt", "hello")
	ftpE2EAssertRetrFrom(t, conn, "proof.txt", 2, "llo")
	modified := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	if err := conn.SetTime("proof.txt", modified); err != nil {
		t.Fatalf("FTP MFMT: %v", err)
	}
	info, err := os.Stat(filepath.Join(siteRoot, "proof.txt"))
	if err != nil || !info.ModTime().Equal(modified) {
		t.Fatalf("FTP MFMT backing time: %v, %v", info, err)
	}
	if err := conn.Append("proof.txt", strings.NewReader(" world")); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "proof.txt"), "hello world")
	if err := conn.StorFrom("proof.txt", strings.NewReader("FTP"), 6); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "proof.txt"), "hello FTPld")
	if err := conn.MakeDir("folder"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Stor("folder/nested.txt", strings.NewReader("nested")); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "folder", "nested.txt"), "nested")
	if err := conn.Rename("folder/nested.txt", "folder/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "folder", "renamed.txt"), "nested")
	if _, err := os.Stat(filepath.Join(siteRoot, "folder", "nested.txt")); !os.IsNotExist(err) {
		t.Fatalf("rename left source: %v", err)
	}
	if err := conn.Delete("folder/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if err := conn.RemoveDir("folder"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(siteRoot, "folder")); !os.IsNotExist(err) {
		t.Fatalf("removed directory remains: %v", err)
	}
	ftpE2EAssertSiteHTTP(t, client, base, "ftp.example.test", "/proof.txt", "hello FTPld")
	blockedRoot := siteRoot + "-blocked"
	if err := os.Rename(siteRoot, blockedRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siteRoot, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := conn.Stor("must-fail.txt", strings.NewReader("never stored")); err == nil {
		t.Fatal("FTP acknowledged a failed backing-store write")
	}
	if err := conn.NoOp(); err != nil {
		t.Fatalf("FTP reply stream desynchronized after failed write: %v", err)
	}
	if err := os.Remove(siteRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(blockedRoot, siteRoot); err != nil {
		t.Fatal(err)
	}
	_ = conn.Quit()
	process.stopProcess()
	base, process = start()
	defer process.stopProcess()
	conn = ftpE2EConnect(t, ftpPort)
	ftpE2EAssertRetr(t, conn, "proof.txt", "hello FTPld")
	ftpE2EAssertSiteHTTP(t, client, base, "ftp.example.test", "/proof.txt", "hello FTPld")
	ftpE2EAssertStored(t, filepath.Join(siteRoot, "proof.txt"), "hello FTPld")
}

func ftpE2EAssertSiteHTTP(t *testing.T, client *http.Client, base, host, filePath, want string) {
	t.Helper()
	response, err := http.NewRequest(http.MethodGet, base+filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Host = host
	httpResponse, err := client.Do(response)
	if err != nil {
		t.Fatal(err)
	}
	httpBody, _ := io.ReadAll(httpResponse.Body)
	_ = httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK || string(httpBody) != want {
		t.Fatalf("site HTTP: %d %q", httpResponse.StatusCode, httpBody)
	}
}

func ftpE2EPrimeSiteMiss(t *testing.T, client *http.Client, base, host, filePath string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, base+filePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = host
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
}

func ftpE2EHasEntry(entries []*ftp.Entry, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func ftpE2EConnect(t *testing.T, port int) *ftp.ServerConn {
	return ftpE2EConnectSite(t, port, "ftp.example.test")
}

func ftpE2EConnectSite(t *testing.T, port int, hostname string) *ftp.ServerConn {
	t.Helper()
	var connection *ftp.ServerConn
	var err error
	for i := 0; i < 30; i++ {
		connection, err = ftp.Dial(fmt.Sprintf("127.0.0.1:%d", port), ftp.DialWithTimeout(3*time.Second))
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("FTP unavailable: %v", err)
	}
	if err := connection.Login("admin@test.local", "testpass123"); err != nil {
		t.Fatal(err)
	}
	if err := connection.ChangeDir("/" + hostname); err != nil {
		t.Fatal(err)
	}
	return connection
}

func ftpE2ESetConfig(t *testing.T, client *http.Client, base, token, key, value string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, base+"/_config/backend/"+key, strings.NewReader(value))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("config %s: %d %s", key, response.StatusCode, body)
	}
}

func ftpE2EAssertRetr(t *testing.T, connection *ftp.ServerConn, name, want string) {
	t.Helper()
	response, err := connection.Retr(name)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response)
	_ = response.Close()
	if err != nil || string(body) != want {
		t.Fatalf("FTP RETR %s: %q, %v", name, body, err)
	}
}

func ftpE2EAssertRetrFrom(t *testing.T, connection *ftp.ServerConn, name string, offset uint64, want string) {
	t.Helper()
	response, err := connection.RetrFrom(name, offset)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response)
	_ = response.Close()
	if err != nil || string(body) != want {
		t.Fatalf("FTP REST RETR %s: %q, %v", name, body, err)
	}
}

func ftpE2EAssertNoPassiveReply(t *testing.T, port int) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	protocol := textproto.NewConn(connection)
	if _, _, err := protocol.ReadResponse(220); err != nil {
		t.Fatal(err)
	}
	for _, command := range []struct {
		line string
		code int
	}{
		{"USER admin@test.local", 331},
		{"PASS testpass123", 230},
		{"STOR /ftp.example.test/no-passive.txt", 550},
		{"NOOP", 200},
		{"SITE CHMOD 0640 /ftp.example.test/seed.txt", 200},
	} {
		if err := protocol.PrintfLine("%s", command.line); err != nil {
			t.Fatal(err)
		}
		if _, _, err := protocol.ReadResponse(command.code); err != nil {
			t.Fatalf("%s reply: %v", command.line, err)
		}
	}
}

func ftpE2EAssertStored(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("backing file %s: %q, %v", path, body, err)
	}
}
