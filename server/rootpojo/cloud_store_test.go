package rootpojo

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	storagefs "github.com/daptin/daptin/server/filesystem"
)

func TestCloudStoreResolvePathUsesPersistedStoreType(t *testing.T) {
	if _, err := (CloudStore{RootPath: t.TempDir()}).ResolvePath("file.txt"); err == nil {
		t.Fatal("cloud store without a type was accepted")
	}

	remote := CloudStore{RootPath: "remote:bucket", StoreType: "cloud", StoreProvider: "s3"}
	if got, err := remote.ResolvePath("/nested/file.txt"); err != nil || got != "remote:bucket/nested/file.txt" {
		t.Fatalf("remote path = %q, %v", got, err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	local := CloudStore{RootPath: root, StoreType: "local", StoreProvider: "localstore"}
	if _, err := local.ResolvePath("escape/file.txt"); !errors.Is(err, storagefs.ErrPathEscapesRoot) {
		t.Fatalf("local symlink escape error = %v", err)
	}
}
