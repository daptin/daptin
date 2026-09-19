package server

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/assetcachepojo"
	storagefs "github.com/daptin/daptin/server/filesystem"
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
			_, err := writeAsset(ctx, fileName, ctx.Request.Body, assetCache)
			if !errors.Is(err, storagefs.ErrPathEscapesRoot) {
				t.Fatalf("writeAsset error = %v, want path containment rejection", err)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(outside, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside upload exists or stat failed unexpectedly: %v", err)
	}
}
