package server

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/cache"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestServeCachedAssetUsesRepresentationSpecificGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data := []byte("abcdefghijklmnopqrstuvwxyz")
	gzipData, err := cache.CompressData(data)
	if err != nil {
		t.Fatal(err)
	}
	cachedFile := &cache.CachedFile{
		Data:      data,
		GzipData:  gzipData,
		ETag:      `"plain"`,
		Modtime:   time.Unix(1_700_000_000, 0),
		MimeType:  "text/plain; charset=utf-8",
		Path:      "asset.txt",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	t.Run("gzip", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = request

		serveCachedAsset(ctx, cachedFile)

		if got := response.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		if got := response.Header().Get("ETag"); got != `"plain-gzip"` {
			t.Fatalf("ETag = %q, want gzip representation ETag", got)
		}
		if got := response.Header().Get("Vary"); got != "Accept-Encoding, Authorization" {
			t.Fatalf("Vary = %q", got)
		}
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(data) {
			t.Fatalf("body = %q", got)
		}
	})

	t.Run("q zero", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
		request.Header.Set("Accept-Encoding", "gzip;q=0")
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = request

		serveCachedAsset(ctx, cachedFile)

		if got := response.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if got := response.Header().Get("ETag"); got != cachedFile.ETag {
			t.Fatalf("ETag = %q, want %q", got, cachedFile.ETag)
		}
		if got := response.Body.Bytes(); string(got) != string(data) {
			t.Fatalf("body = %q", got)
		}
	})

	t.Run("range bypasses gzip", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Range", "bytes=2-5")
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = request

		serveCachedAsset(ctx, cachedFile)

		if response.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusPartialContent)
		}
		if got := response.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if got := response.Body.String(); got != "cdef" {
			t.Fatalf("body = %q, want cdef", got)
		}
	})

	t.Run("gzip conditional", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("If-None-Match", `"plain-gzip"`)
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = request

		serveCachedAsset(ctx, cachedFile)

		if response.Code != http.StatusNotModified {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNotModified)
		}
	})
}

func TestServeResolvedAssetCompressionBypassesRanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	filePath := filepath.Join(root, "large.txt")
	data := []byte(strings.Repeat("compressible asset data\n", 512))
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	serve := func(rangeHeader string) *httptest.ResponseRecorder {
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		if rangeHeader != "" {
			request.Header.Set("Range", rangeHeader)
		}
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = request
		setPrivateAssetCacheHeaders(ctx, 3600)
		serveResolvedAssetWithCompression(ctx, "large.txt", file, info, root, true)
		return response
	}

	compressed := serve("")
	if got := compressed.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if !strings.HasSuffix(compressed.Header().Get("ETag"), `-gzip"`) {
		t.Fatalf("ETag = %q, want gzip representation", compressed.Header().Get("ETag"))
	}

	partial := serve("bytes=2-5")
	if partial.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want %d", partial.Code, http.StatusPartialContent)
	}
	if got := partial.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("range Content-Encoding = %q, want empty", got)
	}
	if got := partial.Body.String(); got != string(data[2:6]) {
		t.Fatalf("range body = %q, want %q", got, data[2:6])
	}
}

func TestServeResolvedMediaAssetFromLocalCloudStoreKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rootDir := t.TempDir()
	localSyncDir := t.TempDir()
	keyName := filepath.Join("assets", "media")
	fileName := "sample.bin"
	contents := []byte("0123456789")
	filePath := filepath.Join(rootDir, keyName, fileName)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, contents, 0644); err != nil {
		t.Fatal(err)
	}

	assetCache := &assetcachepojo.AssetFolderCache{
		LocalSyncPath: localSyncDir,
		Keyname:       keyName,
		CloudStore: rootpojo.CloudStore{
			RootPath:      rootDir,
			StoreProvider: "local",
		},
	}

	for _, fileType := range []string{"audio/wav", "video/mp4"} {
		t.Run(fileType, func(t *testing.T) {
			file, err := assetCache.GetFileByName(fileName)
			if err != nil {
				t.Fatalf("GetFileByName: %v", err)
			}
			defer file.Close()
			fileInfo, err := file.Stat()
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}

			request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
			request.Header.Set("Range", "bytes=2-5")
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = request

			serveResolvedMediaAsset(ctx, fileName, fileType, file, fileInfo)

			if response.Code != http.StatusPartialContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusPartialContent)
			}
			if got, want := response.Body.String(), "2345"; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
			if got := response.Header().Get("Content-Type"); got != fileType {
				t.Fatalf("Content-Type = %q, want %q", got, fileType)
			}
		})
	}
}

func TestCachedAssetAllowedUsesPermissionSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	otherRef := daptinid.DaptinReferenceId(uuid.New())
	groupRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())

	cachedFile := &cache.CachedFile{
		AuthzVersion: cachedAssetAuthzVersion,
		TablePermission: permission.PermissionInstance{
			Permission: auth.UserPeek,
			UserId:     ownerRef,
		},
		RowPermission: permission.PermissionInstance{
			Permission: auth.UserRead,
			UserId:     ownerRef,
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupRead,
			}},
		},
		AdminGroupId: adminGroupRef,
	}

	tests := []struct {
		name    string
		user    *auth.SessionUser
		allowed bool
	}{
		{
			name:    "guest denied",
			user:    &auth.SessionUser{},
			allowed: false,
		},
		{
			name:    "owner allowed",
			user:    &auth.SessionUser{UserReferenceId: ownerRef},
			allowed: true,
		},
		{
			name:    "other user denied",
			user:    &auth.SessionUser{UserReferenceId: otherRef},
			allowed: false,
		},
		{
			name: "group member denied without table peek",
			user: &auth.SessionUser{
				UserReferenceId: otherRef,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: groupRef,
				}},
			},
			allowed: false,
		},
		{
			name: "admin group allowed",
			user: &auth.SessionUser{
				UserReferenceId: otherRef,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: adminGroupRef,
				}},
			},
			allowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
			request = request.WithContext(context.WithValue(request.Context(), "user", tt.user))
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = request

			if got := cachedAssetAllowed(cachedFile, ctx); got != tt.allowed {
				t.Fatalf("cachedAssetAllowed = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestCachedAssetAllowedForGroupSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())

	cachedFile := &cache.CachedFile{
		AuthzVersion: cachedAssetAuthzVersion,
		TablePermission: permission.PermissionInstance{
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupPeek,
			}},
		},
		RowPermission: permission.PermissionInstance{
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupRead,
			}},
		},
		AdminGroupId: adminGroupRef,
	}

	request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
	request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
		Groups: auth.GroupPermissionList{{
			GroupReferenceId: groupRef,
		}},
	}))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	if !cachedAssetAllowed(cachedFile, ctx) {
		t.Fatal("expected group member with table peek and row read to be allowed")
	}
}

func TestCachedAssetRequiresAuthzSnapshot(t *testing.T) {
	if cachedAssetHasAuthz(&cache.CachedFile{}) {
		t.Fatal("expected cached file without authz snapshot to be treated as unsafe")
	}
	if !cachedAssetHasAuthz(&cache.CachedFile{AuthzVersion: cachedAssetAuthzVersion}) {
		t.Fatal("expected current authz snapshot version to be accepted")
	}
}

func TestAssetAuthzAllowedUsesFreshSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
	request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{}))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	stalePrivateCachedFile := &cache.CachedFile{
		AuthzVersion: cachedAssetAuthzVersion,
		TablePermission: permission.PermissionInstance{
			Permission: auth.UserPeek,
			UserId:     daptinid.DaptinReferenceId(uuid.New()),
		},
		RowPermission: permission.PermissionInstance{
			Permission: auth.UserRead,
			UserId:     daptinid.DaptinReferenceId(uuid.New()),
		},
	}
	if cachedAssetAllowed(stalePrivateCachedFile, ctx) {
		t.Fatal("expected stale private cached snapshot to deny guest")
	}

	currentPublicAuthz := assetAuthzSnapshot{
		tablePermission: permission.PermissionInstance{Permission: auth.GuestPeek},
		rowPermission:   permission.PermissionInstance{Permission: auth.GuestRead},
	}
	if !assetAuthzAllowed(currentPublicAuthz, ctx) {
		t.Fatal("expected current public authorization to allow guest")
	}
}

func TestCachedAssetFileStillCurrentRequiresCurrentRowReference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	cachedFile := &cache.CachedFile{
		Path: filepath.Join(tempDir, "folder", "image.png"),
	}
	cruds := map[string]*resource.DbResource{
		"world": {
			AssetFolderCache: map[string]map[string]*assetcachepojo.AssetFolderCache{
				"asset": {
					"file": {
						LocalSyncPath: tempDir,
					},
				},
			},
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file?index=0&file=image.png", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	currentRow := map[string]interface{}{
		"file": []map[string]interface{}{
			{
				"path": "folder",
				"name": "image.png",
				"type": "image/png",
			},
		},
	}
	if !cachedAssetFileStillCurrent(cachedFile, currentRow, cruds, "asset", "file", ctx) {
		t.Fatal("expected cached file to match current row file reference")
	}

	updatedRow := map[string]interface{}{
		"file": []map[string]interface{}{
			{
				"path": "folder",
				"name": "replacement.png",
				"type": "image/png",
			},
		},
	}
	if cachedAssetFileStillCurrent(cachedFile, updatedRow, cruds, "asset", "file", ctx) {
		t.Fatal("expected cached file to be stale after current row references a different file")
	}

	deletedFileRow := map[string]interface{}{
		"file": []map[string]interface{}{},
	}
	if cachedAssetFileStillCurrent(cachedFile, deletedFileRow, cruds, "asset", "file", ctx) {
		t.Fatal("expected cached file to be stale after current row no longer references files")
	}
}
