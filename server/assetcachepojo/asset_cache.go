package assetcachepojo

import (
	"encoding/base64"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

type AssetFolderCache struct {
	LocalSyncPath string
	Keyname       string
	CloudStore    rootpojo.CloudStore
}

func (afc *AssetFolderCache) GetFileByName(fileName string) (*os.File, error) {

	return os.Open(afc.LocalSyncPath + string(os.PathSeparator) + fileName)

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
