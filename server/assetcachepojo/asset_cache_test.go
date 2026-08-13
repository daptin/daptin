package assetcachepojo

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	_ "github.com/artpar/rclone/backend/local"
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
