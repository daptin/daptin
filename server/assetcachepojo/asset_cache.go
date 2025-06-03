package assetcachepojo

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AssetFolderCache struct {
	LocalSyncPath string
	Keyname       string
	CloudStore    rootpojo.CloudStore
	Credentials   map[string]interface{} // Store credentials to avoid repeated lookups
}

func (afc *AssetFolderCache) GetFileByName(fileName string) (*os.File, error) {
	localFilePath := afc.LocalSyncPath + string(os.PathSeparator) + fileName

	// Try to open the file from local cache first
	file, err := os.Open(localFilePath)
	if err == nil {
		return file, nil
	}

	// If file not found in local cache and cloud store is not local, try to download it
	if os.IsNotExist(err) && afc.CloudStore.StoreProvider != "local" {
		log.Infof("File [%v] not found in local cache, attempting to download from cloud storage", fileName)

		// Download the file from cloud storage
		err = afc.downloadFileFromCloudStore(fileName)
		if err != nil {
			log.Errorf("[42] Failed to download file from cloud storage: %v", err)
			return nil, err
		}

		// Try opening the file again after download
		return os.Open(localFilePath)
	}

	return nil, err
}

// downloadFileFromCloudStore downloads a specific file from cloud storage to local cache
func (afc *AssetFolderCache) downloadFileFromCloudStore(fileName string) error {
	// Setup credentials if available
	configSetName := afc.CloudStore.Name
	if strings.Index(afc.CloudStore.RootPath, ":") > -1 {
		configSetName = strings.Split(afc.CloudStore.RootPath, ":")[0]
	}
	if afc.Credentials != nil {
		for key, val := range afc.Credentials {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
		}
	} else if afc.CloudStore.StoreParameters != nil {
		for key, val := range afc.CloudStore.StoreParameters {
			config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
		}
	}

	// Prepare source and destination paths
	sourcePath := afc.CloudStore.RootPath + string(os.PathSeparator) + afc.Keyname
	destPathFolder := afc.LocalSyncPath + string(os.PathSeparator)
	destFilePath := destPathFolder + string(os.PathSeparator) + fileName

	// Ensure destination directory exists
	destDir := filepath.Dir(destPathFolder)
	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to create destination directory")
	}

	// Create a temporary file for download
	tmpFile := destPathFolder + string(os.PathSeparator) + fileName + ".tmp"
	defer func() {
		// Clean up temp file if it exists
		if _, err := os.Stat(tmpFile); err == nil {
			os.Remove(tmpFile)
		}
	}()

	// Download the file
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Parse remote filesystem
	fsrc, err := fs.NewFs(ctx, sourcePath)
	if err != nil {
		return errors.Wrap(err, "failed to create source filesystem")
	}

	// Get the file object
	srcObj, err := fsrc.NewObject(ctx, fileName)
	if err != nil {
		return errors.Wrap(err, "failed to create source object")
	}

	// Open destination file
	dst, err := os.Create(tmpFile)
	if err != nil {
		return errors.Wrap(err, "failed to create destination file")
	}
	defer dst.Close()

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
	dst.Close()

	// Move temp file to final destination
	err = os.Rename(tmpFile, destFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to move downloaded file to final location")
	}

	log.Debugf("Successfully downloaded file [%v] from cloud storage[%v] to cache", sourcePath, fileName)
	return nil
}
func (afc *AssetFolderCache) DeleteFileByName(fileName string) error {

	return os.Remove(afc.LocalSyncPath + string(os.PathSeparator) + fileName)

}

func (afc *AssetFolderCache) GetPathContents(path string) ([]map[string]interface{}, error) {

	fileInfo, err := os.ReadDir(afc.LocalSyncPath + string(os.PathSeparator) + path)
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

func (afc *AssetFolderCache) UploadFiles(files []interface{}) error {

	for i := range files {
		file := files[i].(map[string]interface{})
		contents, ok := file["file"]
		if !ok {
			contents = file["contents"]
		}
		if contents != nil {

			contentString, ok := contents.(string)
			if ok && len(contentString) > 4 {

				if strings.Index(contentString, ",") > -1 {
					contentParts := strings.Split(contentString, ",")
					contentString = contentParts[len(contentParts)-1]
				}
				fileBytes, e := base64.StdEncoding.DecodeString(contentString)
				if e != nil {
					continue
				}
				if file["name"] == nil {
					return errors.WithMessage(errors.New("file name cannot be null"), "File name is null")
				}
				filePath := string(os.PathSeparator)
				if file["path"] != nil {
					filePath = strings.Replace(file["path"].(string), "/", string(os.PathSeparator), -1) + string(os.PathSeparator)
				}
				localPath := afc.LocalSyncPath + string(os.PathSeparator) + filePath
				createDirIfNotExist(localPath)
				localFilePath := localPath + file["name"].(string)
				err := os.WriteFile(localFilePath, fileBytes, os.ModePerm)
				if err != nil {
					log.Error("Failed to write data to local file store asset cache folder")
					return errors.WithMessage(err, "Failed to write data to local file store ")
				}
			}
		}
	}

	return nil

}

func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}
