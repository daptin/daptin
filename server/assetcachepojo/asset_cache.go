package assetcachepojo

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const maxConcurrentCloudDownloads = 16

var (
	cloudDownloads        singleflight.Group
	cloudDownloadSlots    = make(chan struct{}, maxConcurrentCloudDownloads)
	rcloneConfigurationMu sync.Mutex
)

var ErrAssetNotFound = errors.New("asset not found")

type AssetFolderCache struct {
	LocalSyncPath string
	Keyname       string
	CloudStore    rootpojo.CloudStore
	Credentials   map[string]interface{} // Store credentials to avoid repeated lookups
}

func (afc *AssetFolderCache) GetFileByName(fileName string) (*os.File, error) {
	return afc.GetFileByNameContext(context.Background(), fileName)
}

// GetFileByNameContext opens a cached file, downloading it once when concurrent
// requests miss the same cloud-backed object. Every caller receives its own file
// descriptor; only the cache fill is shared.
func (afc *AssetFolderCache) GetFileByNameContext(ctx context.Context, fileName string) (*os.File, error) {
	fileName, err := storagefs.ValidatePath(fileName)
	if err != nil {
		return nil, err
	}
	if fileName == "" {
		return nil, fmt.Errorf("file name cannot be empty")
	}
	localFilePath, err := afc.resolveLocalPath(fileName)
	if err != nil {
		return nil, err
	}

	// Try to open the file from local cache first
	file, err := os.Open(localFilePath)
	if err == nil {
		return file, nil
	}

	// If file not found in local cache and cloud store is not local, try to download it
	if os.IsNotExist(err) && afc.CloudStore.StoreType != "local" {
		// Download the file from cloud storage
		key, keyErr := filepath.Abs(localFilePath)
		if keyErr != nil {
			key = filepath.Clean(localFilePath)
		}
		result := cloudDownloads.DoChan(key, func() (interface{}, error) {
			// Another process may have completed the cache fill before this
			// singleflight leader started.
			if existing, openErr := os.Open(localFilePath); openErr == nil {
				_ = existing.Close()
				return nil, nil
			} else if !os.IsNotExist(openErr) {
				return nil, openErr
			}
			log.Infof("File [%v] not found in local cache, attempting to download from cloud storage", fileName)

			downloadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			select {
			case cloudDownloadSlots <- struct{}{}:
				defer func() { <-cloudDownloadSlots }()
			case <-downloadCtx.Done():
				return nil, downloadCtx.Err()
			}
			downloadErr := afc.downloadFileFromCloudStore(downloadCtx, fileName)
			if downloadErr != nil {
				log.Errorf("[42] Failed to download file[%s] from cloud storage: %v", fileName, downloadErr)
			}
			return nil, downloadErr
		})

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case downloadResult := <-result:
			err = downloadResult.Err
		}
		if err != nil {
			return nil, err
		}

		// Try opening the file again after download
		return os.Open(localFilePath)
	}

	return nil, err
}

func IsAssetNotFound(err error) bool {
	return errors.Is(err, ErrAssetNotFound) || os.IsNotExist(err)
}

// downloadFileFromCloudStore downloads a specific file from cloud storage to local cache
func (afc *AssetFolderCache) downloadFileFromCloudStore(ctx context.Context, fileName string) error {
	// Setup credentials if available
	fileName, err := storagefs.ValidatePath(fileName)
	if err != nil {
		return err
	}
	configSetName := afc.CloudStore.Name
	if strings.Index(afc.CloudStore.RootPath, ":") > -1 {
		configSetName = strings.Split(afc.CloudStore.RootPath, ":")[0]
	}
	// Prepare source and destination paths
	sourcePath, err := afc.CloudStore.ResolvePath(afc.Keyname)
	if err != nil {
		return err
	}
	destFilePath, err := storagefs.ResolveLocalPath(afc.LocalSyncPath, fileName)
	if err != nil {
		return err
	}

	// Ensure the final file and its temporary sibling share a directory so the
	// rename is atomic.
	destDir := filepath.Dir(destFilePath)
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to create destination directory")
	}

	// rclone's configuration is process-global. Protect configuration and
	// filesystem construction so one store cannot observe another's credentials.
	fsrc, err := afc.newCloudFilesystem(ctx, sourcePath, configSetName)
	if err != nil {
		return errors.Wrapf(err, "failed to create source filesystem [%s]", sourcePath)
	}

	// Get the file object
	srcObj, err := fsrc.NewObject(ctx, fileName)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return errors.Wrapf(ErrAssetNotFound, "source object [%s][%s]", sourcePath, fileName)
		}
		return errors.Wrapf(err, "failed to create source object [%s][%s]", sourcePath, fileName)
	}

	// Each download owns its temporary file. This also makes the operation safe
	// across multiple daptin processes sharing the same cache directory.
	dst, err := os.CreateTemp(destDir, "."+filepath.Base(fileName)+".download-*")
	if err != nil {
		return errors.Wrapf(err, "failed to create temporary file in [%s]", destDir)
	}
	tmpFile := dst.Name()
	removeTemporary := true
	defer func() {
		_ = dst.Close()
		if removeTemporary {
			_ = os.Remove(tmpFile)
		}
	}()

	// Open source for reading
	srcReader, err := srcObj.Open(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open source for reading")
	}
	defer srcReader.Close()

	// Copy the content
	_, err = io.Copy(dst, srcReader)
	if err != nil {
		return errors.Wrap(err, "failed to copy file content")
	}

	// Close the destination file before rename
	if err := dst.Close(); err != nil {
		return errors.Wrap(err, "failed to close downloaded file")
	}
	if err := os.Chmod(tmpFile, 0644); err != nil {
		return errors.Wrap(err, "failed to set downloaded file permissions")
	}

	// Move temp file to final destination
	err = os.Rename(tmpFile, destFilePath)
	if err != nil {
		// Another process may have won the same atomic cache fill. Its completed
		// file is equivalent and safe to use.
		if _, statErr := os.Stat(destFilePath); statErr == nil {
			return nil
		}
		return errors.Wrap(err, "failed to move downloaded file to final location")
	}
	removeTemporary = false

	log.Debugf("Successfully downloaded file [%v] from cloud storage[%v] to cache", sourcePath, fileName)
	return nil
}

func (afc *AssetFolderCache) newCloudFilesystem(ctx context.Context, sourcePath, configSetName string) (fs.Fs, error) {
	rcloneConfigurationMu.Lock()
	defer rcloneConfigurationMu.Unlock()

	if afc.Credentials != nil {
		for key, val := range afc.Credentials {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%v", val))
		}
	} else if afc.CloudStore.StoreParameters != nil {
		for key, val := range afc.CloudStore.StoreParameters {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%v", val))
		}
	}
	return fs.NewFs(ctx, sourcePath)
}
func (afc *AssetFolderCache) DeleteFileByName(fileName string) error {
	fileName, err := storagefs.ValidatePath(fileName)
	if err != nil {
		return err
	}
	if fileName == "" {
		return nil
	}
	localPath, err := afc.resolveLocalPath(fileName)
	if err != nil {
		return err
	}
	return os.Remove(localPath)
}

func (afc *AssetFolderCache) GetPathContents(path string) ([]map[string]interface{}, error) {
	localPath, err := afc.resolveLocalPath(path)
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.ReadDir(localPath)
	if err != nil {
		return nil, err
	}

	//files, err := filepath.Glob(afc.LocalSyncPath + string(os.PathSeparator) + path + "*")
	//fmt.Println(files)
	var files []map[string]interface{}
	for _, file := range fileInfo {
		//files[i] = strings.Replace(file, afc.LocalSyncPath, "", 1)
		info, err := file.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, map[string]interface{}{
			"name":     file.Name(),
			"is_dir":   file.IsDir(),
			"mod_time": info.ModTime(),
			"size":     info.Size(),
		})
	}

	return files, err

}

func (afc *AssetFolderCache) storageBaseDir() (string, error) {
	if afc.CloudStore.StoreType == "local" {
		return afc.CloudStore.ResolvePath(afc.Keyname)
	}
	return storagefs.ResolveLocalPath(afc.LocalSyncPath, "")
}

func (afc *AssetFolderCache) resolveLocalPath(fileName string) (string, error) {
	baseDir, err := afc.storageBaseDir()
	if err != nil {
		return "", err
	}
	return storagefs.ResolveLocalPath(baseDir, fileName)
}
