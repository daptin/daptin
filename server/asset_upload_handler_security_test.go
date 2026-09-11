package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/gin-gonic/gin"
)

func TestStreamUploadCannotEscapeLocalAssetRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "assets", "escape")); err != nil {
		t.Fatal(err)
	}
	assetCache := &assetcachepojo.AssetFolderCache{
		Keyname: "assets",
		CloudStore: rootpojo.CloudStore{
			StoreType:     "local",
			StoreProvider: "localstore",
			RootPath:      root,
		},
	}

	for _, fileName := range []string{"../outside.txt", "escape/outside.txt"} {
		t.Run(fileName, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest("POST", "/?upload_id=test", strings.NewReader("outside"))
			handleStreamUpload(ctx, fileName, assetCache)
			if response.Code != 400 {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(outside, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside upload exists or stat failed unexpectedly: %v", err)
	}
}
