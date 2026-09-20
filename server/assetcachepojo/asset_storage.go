package assetcachepojo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
	storagefs "github.com/daptin/daptin/server/filesystem"
)

// StoredFileInfo is the common file metadata returned by local and rclone stores.
type StoredFileInfo struct {
	FileName  string
	FileSize  int64
	Modified  time.Time
	Directory bool
}

func (info StoredFileInfo) Name() string { return info.FileName }
func (info StoredFileInfo) Size() int64  { return info.FileSize }
func (info StoredFileInfo) Mode() os.FileMode {
	if info.Directory {
		return os.ModeDir | 0755
	}
	return 0600
}
func (info StoredFileInfo) ModTime() time.Time { return info.Modified }
func (info StoredFileInfo) IsDir() bool        { return info.Directory }
func (info StoredFileInfo) Sys() interface{}   { return nil }

func (afc *AssetFolderCache) storedPath(name string) (string, string, error) {
	name, err := storagefs.ValidatePath(name)
	if err != nil {
		return "", "", err
	}
	root, err := afc.CloudStore.ResolvePath(afc.Keyname)
	return root, name, err
}

func (afc *AssetFolderCache) storedFs(ctx context.Context) (fs.Fs, error) {
	root, _, err := afc.storedPath("")
	if err != nil {
		return nil, err
	}
	configName := afc.CloudStore.Name
	if index := indexRemoteSeparator(afc.CloudStore.RootPath); index >= 0 {
		configName = afc.CloudStore.RootPath[:index]
	}
	return afc.newCloudFilesystem(ctx, root, configName)
}

func indexRemoteSeparator(root string) int {
	for i := 0; i < len(root); i++ {
		if root[i] == ':' {
			return i
		}
	}
	return -1
}

// PutStoredFile writes to the configured cloud store, not the local site cache.
func (afc *AssetFolderCache) PutStoredFile(ctx context.Context, name string, reader io.Reader) (int64, error) {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return 0, err
	}
	if name == "" {
		return 0, errors.New("stored file path is empty")
	}
	if afc.CloudStore.StoreType == "local" {
		target, err := afc.resolveLocalPath(name)
		if err != nil {
			return 0, err
		}
		previous, statErr := os.Stat(target)
		if statErr != nil && !os.IsNotExist(statErr) {
			return 0, statErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return 0, err
		}
		temporary, err := os.CreateTemp(filepath.Dir(target), ".daptin-upload-*")
		if err != nil {
			return 0, err
		}
		defer os.Remove(temporary.Name())
		size, copyErr := io.Copy(temporary, reader)
		closeErr := temporary.Close()
		if copyErr != nil {
			return 0, copyErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		if previous != nil {
			if err := os.Chmod(temporary.Name(), previous.Mode().Perm()); err != nil {
				return 0, err
			}
		}
		if err := os.Rename(temporary.Name(), target); err != nil {
			return 0, err
		}
		return size, nil
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return 0, err
	}
	counted := &countingStoredReader{reader: reader}
	_, err = operations.Rcat(ctx, store, name, io.NopCloser(counted), time.Now(), fs.Metadata{})
	if err != nil {
		return 0, err
	}
	if err := afc.evictStoredCache(name); err != nil {
		return 0, err
	}
	return counted.count, nil
}

type countingStoredReader struct {
	reader io.Reader
	count  int64
}

func (reader *countingStoredReader) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	reader.count += int64(n)
	return n, err
}

func (afc *AssetFolderCache) StatStoredFile(ctx context.Context, name string) (os.FileInfo, error) {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return nil, err
	}
	if afc.CloudStore.StoreType == "local" {
		filePath, err := afc.resolveLocalPath(name)
		if err != nil {
			return nil, err
		}
		return os.Stat(filePath)
	}
	if name == "" {
		return StoredFileInfo{FileName: ".", Directory: true}, nil
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return nil, err
	}
	object, err := store.NewObject(ctx, name)
	if err == nil {
		return StoredFileInfo{FileName: path.Base(name), FileSize: object.Size(), Modified: object.ModTime(ctx)}, nil
	}
	if !errors.Is(err, fs.ErrorObjectNotFound) && !errors.Is(err, fs.ErrorIsDir) {
		return nil, err
	}
	entries, err := store.List(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 && !store.Features().CanHaveEmptyDirectories {
		return nil, fs.ErrorDirNotFound
	}
	return StoredFileInfo{FileName: path.Base(name), Directory: true}, nil
}

func (afc *AssetFolderCache) ListStoredFiles(ctx context.Context, name string) ([]os.FileInfo, error) {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return nil, err
	}
	if afc.CloudStore.StoreType == "local" {
		filePath, err := afc.resolveLocalPath(name)
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(filePath)
		if err != nil {
			return nil, err
		}
		files := make([]os.FileInfo, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			files = append(files, info)
		}
		return files, nil
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := store.List(ctx, name)
	if err != nil {
		return nil, err
	}
	if name != "" && len(entries) == 0 && !store.Features().CanHaveEmptyDirectories {
		return nil, fs.ErrorDirNotFound
	}
	files := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		_, directory := entry.(fs.Directory)
		files = append(files, StoredFileInfo{FileName: path.Base(entry.Remote()), FileSize: entry.Size(), Modified: entry.ModTime(ctx), Directory: directory})
	}
	return files, nil
}

func (afc *AssetFolderCache) MakeStoredDirectory(ctx context.Context, name string) error {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("stored directory path is empty")
	}
	if afc.CloudStore.StoreType == "local" {
		filePath, err := afc.resolveLocalPath(name)
		if err != nil {
			return err
		}
		return os.Mkdir(filePath, 0750)
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return err
	}
	if !store.Features().CanHaveEmptyDirectories {
		return fs.ErrorNotImplemented
	}
	return store.Mkdir(ctx, name)
}

func (afc *AssetFolderCache) RemoveStoredFile(ctx context.Context, name string) error {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("stored path is empty")
	}
	if afc.CloudStore.StoreType == "local" {
		filePath, err := afc.resolveLocalPath(name)
		if err != nil {
			return err
		}
		return os.Remove(filePath)
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return err
	}
	object, err := store.NewObject(ctx, name)
	if err == nil {
		err = object.Remove(ctx)
	} else if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorIsDir) {
		if entries, listErr := store.List(ctx, name); listErr != nil {
			return listErr
		} else if len(entries) != 0 {
			return fs.ErrorDirectoryNotEmpty
		} else if !store.Features().CanHaveEmptyDirectories {
			return fs.ErrorDirNotFound
		}
		err = store.Rmdir(ctx, name)
	}
	if err != nil {
		return err
	}
	return afc.evictStoredCache(name)
}

func (afc *AssetFolderCache) MoveStoredFile(ctx context.Context, from, to string) error {
	_, from, err := afc.storedPath(from)
	if err != nil {
		return err
	}
	if from == "" {
		return errors.New("source path is empty")
	}
	_, to, err = afc.storedPath(to)
	if err != nil {
		return err
	}
	if to == "" {
		return errors.New("destination path is empty")
	}
	if strings.HasPrefix(to, from+"/") {
		return errors.New("cannot move a path into itself")
	}
	if afc.CloudStore.StoreType == "local" {
		fromPath, err := afc.resolveLocalPath(from)
		if err != nil {
			return err
		}
		toPath, err := afc.resolveLocalPath(to)
		if err != nil {
			return err
		}
		return os.Rename(fromPath, toPath)
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return err
	}
	_, err = store.NewObject(ctx, from)
	if err == nil {
		err = operations.MoveFile(ctx, store, store, to, from)
	} else if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorIsDir) {
		entries, listErr := store.List(ctx, from)
		if listErr != nil {
			return fmt.Errorf("list move source %q: %w", from, listErr)
		}
		if len(entries) == 0 && !store.Features().CanHaveEmptyDirectories {
			return fs.ErrorDirNotFound
		}
		err = nil
		fromRoot, err := afc.CloudStore.ResolvePath(path.Join(afc.Keyname, from))
		if err != nil {
			return err
		}
		toRoot, err := afc.CloudStore.ResolvePath(path.Join(afc.Keyname, to))
		if err != nil {
			return err
		}
		configName := afc.CloudStore.Name
		if index := indexRemoteSeparator(afc.CloudStore.RootPath); index >= 0 {
			configName = afc.CloudStore.RootPath[:index]
		}
		fromStore, err := afc.newCloudFilesystem(ctx, fromRoot, configName)
		if err != nil {
			return err
		}
		toStore, err := afc.newCloudFilesystem(ctx, toRoot, configName)
		if err != nil {
			return err
		}
		err = sync.MoveDir(ctx, toStore, fromStore, true, true)
		if err != nil {
			return fmt.Errorf("move directory %q to %q: %w", from, to, err)
		}
	}
	if err != nil {
		return fmt.Errorf("move object %q to %q: %w", from, to, err)
	}
	if err := afc.evictStoredCache(from); err != nil {
		return err
	}
	return afc.evictStoredCache(to)
}

func (afc *AssetFolderCache) SetStoredModTime(ctx context.Context, name string, modified time.Time) error {
	_, name, err := afc.storedPath(name)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("stored path is empty")
	}
	if afc.CloudStore.StoreType == "local" {
		filePath, err := afc.resolveLocalPath(name)
		if err != nil {
			return err
		}
		return os.Chtimes(filePath, modified, modified)
	}
	store, err := afc.storedFs(ctx)
	if err != nil {
		return err
	}
	object, err := store.NewObject(ctx, name)
	if err != nil {
		return err
	}
	if err := object.SetModTime(ctx, modified); err != nil {
		return err
	}
	return afc.evictStoredCache(name)
}

func (afc *AssetFolderCache) ChmodStoredFile(name string, mode os.FileMode) error {
	if afc.CloudStore.StoreType != "local" {
		return fs.ErrorNotImplemented
	}
	filePath, err := afc.resolveLocalPath(name)
	if err != nil {
		return err
	}
	return os.Chmod(filePath, mode)
}

func (afc *AssetFolderCache) evictStoredCache(name string) error {
	if afc.CloudStore.StoreType == "local" {
		return nil
	}
	cachePath, err := storagefs.ResolveLocalPath(afc.LocalSyncPath, name)
	if err != nil {
		return err
	}
	return os.RemoveAll(cachePath)
}
