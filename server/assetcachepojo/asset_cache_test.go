package assetcachepojo

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	_ "github.com/artpar/rclone/backend/local"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/daptin/daptin/server/rootpojo"
)

func TestConcurrentColdCacheRequestsShareDownload(t *testing.T) {
	remoteRoot := t.TempDir()
	localCache := t.TempDir()
	keyName := "assets"
	fileName := filepath.Join("nested", "same.txt")
	want := bytes.Repeat([]byte("concurrent asset contents\n"), 4096)
	remotePath := filepath.Join(remoteRoot, keyName, fileName)
	if err := os.MkdirAll(filepath.Dir(remotePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remotePath, want, 0o644); err != nil {
		t.Fatal(err)
	}

	assetCache := &AssetFolderCache{
		LocalSyncPath: localCache,
		Keyname:       keyName,
		CloudStore: rootpojo.CloudStore{
			RootPath:      remoteRoot,
			StoreType:     "cloud",
			StoreProvider: "remote-for-test",
		},
	}

	const callers = 32
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	var wait sync.WaitGroup
	wait.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wait.Done()
			<-start
			file, err := assetCache.GetFileByName(fileName)
			if err != nil {
				errorsByCaller <- err
				return
			}
			defer file.Close()
			got, err := io.ReadAll(file)
			if err != nil {
				errorsByCaller <- err
				return
			}
			if !bytes.Equal(got, want) {
				errorsByCaller <- errors.New("downloaded contents differ")
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByCaller)
	for err := range errorsByCaller {
		t.Errorf("concurrent cache request failed: %v", err)
	}

	if matches, err := filepath.Glob(filepath.Join(localCache, "nested", ".same.txt.download-*")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("temporary downloads remain: %v", matches)
	}
	if got, err := os.ReadFile(filepath.Join(localCache, fileName)); err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(got, want) {
		t.Fatal("final cached file differs from remote object")
	}
}

func TestCloudObjectNotFoundIsClassified(t *testing.T) {
	assetCache := &AssetFolderCache{
		LocalSyncPath: t.TempDir(),
		Keyname:       "assets",
		CloudStore: rootpojo.CloudStore{
			RootPath:      t.TempDir(),
			StoreType:     "cloud",
			StoreProvider: "remote-for-test",
		},
	}

	_, err := assetCache.GetFileByName("missing.txt")
	if err == nil {
		t.Fatal("expected missing cloud object error")
	}
	if !IsAssetNotFound(err) {
		t.Fatalf("IsAssetNotFound(%v) = false", err)
	}
}

func TestLocalAssetOperationsCannotEscapeStoreRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}

	root := t.TempDir()
	outside := t.TempDir()
	keyName := "assets"
	assetRoot := filepath.Join(root, keyName)
	if err := os.MkdirAll(assetRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "canary.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(assetRoot, "escape")); err != nil {
		t.Fatal(err)
	}

	assetCache := &AssetFolderCache{
		LocalSyncPath: assetRoot,
		Keyname:       keyName,
		CloudStore: rootpojo.CloudStore{
			RootPath:      root,
			StoreType:     "local",
			StoreProvider: "localstore",
		},
	}

	if _, err := assetCache.GetFileByName("../canary.txt"); !errors.Is(err, storagefs.ErrPathEscapesRoot) {
		t.Fatalf("lexical read escape error = %v", err)
	}
	if _, err := assetCache.GetFileByName("escape/canary.txt"); !errors.Is(err, storagefs.ErrPathEscapesRoot) {
		t.Fatalf("symlink read escape error = %v", err)
	}
	if _, err := assetCache.GetPathContents("escape"); !errors.Is(err, storagefs.ErrPathEscapesRoot) {
		t.Fatalf("symlink list escape error = %v", err)
	}
	if err := assetCache.DeleteFileByName("escape/canary.txt"); !errors.Is(err, storagefs.ErrPathEscapesRoot) {
		t.Fatalf("symlink delete escape error = %v", err)
	}
	got, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside file changed to %q", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside upload exists or stat failed unexpectedly: %v", err)
	}
}
