package actions

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/accounting"
	"github.com/artpar/rclone/fs/filter"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sync2 "sync"
	"time"

	//"os"
	"archive/zip"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type fileUploadActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (actionPerformer *fileUploadActionPerformer) Name() string {
	return "cloudstore.file.upload"
}

func unzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		entryName, err := storagefs.ValidatePath(file.Name)
		if err != nil {
			return err
		}
		if entryName == "" || entryName == "." {
			continue
		}
		safePath, err := storagefs.ResolveLocalPath(target, entryName)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			os.MkdirAll(safePath, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		os.MkdirAll(filepath.Dir(safePath), 0755)
		targetFile, err := os.OpenFile(safePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}

	return nil
}

var cleanupmux = sync2.Mutex{}
var cleanuppath = make(map[string]bool)

func (actionPerformer *fileUploadActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	atPath, ok := inFields["path"].(string)
	files, ok := inFields["file"].([]interface{})
	if !ok {
		return nil, nil, []error{fmt.Errorf("improper file attachment, expected []interface{} got %v", inFields["file"])}
	}
	atPath, err := storagefs.ValidatePath(atPath)
	if err != nil {
		return nil, nil, []error{err}
	}
	for _, fileInterface := range files {
		file := fileInterface.(map[string]interface{})
		fileName, ok := file["name"].(string)
		if !ok {
			return nil, nil, []error{fmt.Errorf("file name is missing")}
		}
		fileName, err = storagefs.ValidatePath(fileName)
		if err != nil {
			return nil, nil, []error{err}
		}
		if fileName == "" {
			return nil, nil, []error{fmt.Errorf("file name cannot be empty")}
		}
		file["name"] = fileName
		filePath := ""
		if file["path"] != nil {
			filePath, ok = file["path"].(string)
			if !ok {
				return nil, nil, []error{fmt.Errorf("file path must be a string")}
			}
			filePath, err = storagefs.ValidatePath(filePath)
			if err != nil {
				return nil, nil, []error{err}
			}
		}
		file["path"] = filePath
	}

	storageRoot := inFields["root_path"].(string)
	localStorageRoot := ""
	rootPath := storageRoot
	isLocal, err := isLocalCloudStore(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	if isLocal {
		localStorageRoot = storageRoot
		rootPath, err = storagefs.ResolveLocalPath(storageRoot, atPath)
	} else {
		rootPath, err = storagefs.ResolvePath(storageRoot, atPath)
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	if localStorageRoot != "" {
		for _, fileInterface := range files {
			file := fileInterface.(map[string]interface{})
			if _, err := storagefs.ResolveLocalPath(localStorageRoot, path.Join(atPath, file["path"].(string), file["name"].(string))); err != nil {
				return nil, nil, []error{err}
			}
		}
	}

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Debugf("Temp directory for this upload fileUploadActionPerformer: %v", tempDirectoryPath)
	if err != nil {
		return nil, nil, []error{err}
	}

	for _, fileInterface := range files {
		file := fileInterface.(map[string]interface{})
		fileName := file["name"].(string)
		filePath := file["path"].(string)
		temproryFilePath, err := storagefs.ResolveLocalPath(tempDirectoryPath, path.Join(filePath, fileName))
		if err != nil {
			return nil, nil, []error{err}
		}

		fileContentsBase64, ok := file["file"].(string)
		if !ok {
			fileContentsBase64, ok = file["contents"].(string)
			if !ok {
				continue
			}
		}
		splitParts := strings.Split(fileContentsBase64, ",")
		encodedPart := splitParts[0]
		if len(splitParts) > 1 {
			encodedPart = splitParts[len(splitParts)-1]
		}
		fileBytes, err := base64.StdEncoding.DecodeString(encodedPart)
		log.Infof("[116] Write file [%v] for upload", temproryFilePath)
		resource.CheckErr(err, "Failed to convert base64 to []bytes")

		fileDir := filepath.Dir(temproryFilePath)
		os.MkdirAll(fileDir, 0755)

		err = os.WriteFile(temproryFilePath, fileBytes, 0666)
		resource.CheckErr(err, "[122] Failed to write file bytes to temp file for rclone upload")

		if EndsWithCheck(fileName, ".zip") {
			err = unzip(temproryFilePath, filepath.Dir(temproryFilePath))
			resource.CheckErr(err, "Failed to unzip file")
			if err != nil {
				return nil, nil, []error{err}
			}
			go func() {
				time.Sleep(5 * time.Minute)
				err = os.Remove(temproryFilePath)
				resource.CheckErr(err, "Failed to remove zip file after extraction")
			}()

		}

	}
	if localStorageRoot != "" {
		if err := storagefs.ValidateLocalTreeDestination(localStorageRoot, atPath, tempDirectoryPath); err != nil {
			_ = os.RemoveAll(tempDirectoryPath)
			return nil, nil, []error{err}
		}
	}
	args := []string{
		tempDirectoryPath,
		rootPath,
	}
	log.Infof("[183] Upload source [%v] target [%v] with [%v]", tempDirectoryPath, rootPath, inFields["credential_name"])

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := actionPerformer.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		name := strings.Split(rootPath, ":")[0]
		if cred != nil && cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(name, key, fmt.Sprintf("%s", val))
			}
		}
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("File upload action from [%v]", tempDirectoryPath),
	}
	ctx := context.Background()
	ctx = accounting.WithStatsGroup(ctx, "transfer-"+fmt.Sprintf("%d", time.Now().Unix()))
	newFilter, _ := filter.NewFilter(nil)
	ctx = filter.ReplaceConfig(ctx, newFilter)
	defaultConfig := fs.ConfigInfo{}

	defaultConfig.LogLevel = fs.LogLevelDebug

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		defaultConfig.DeleteMode = fs.DeleteModeOff
		defaultConfig.ErrorOnNoTransfer = true
		err := sync.CopyDir(ctx, fdst, fsrc, false)
		resource.InfoErr(err, "[187] Failed to sync files for upload to cloud")

		go func() {
			cleanupmux.Lock()
			_, ok := cleanuppath[tempDirectoryPath]
			if ok {
				cleanupmux.Unlock()
				return
			}
			cleanuppath[tempDirectoryPath] = true
			cleanupmux.Unlock()

			time.Sleep(10 * time.Minute)
			cleanupmux.Lock()
			delete(cleanuppath, tempDirectoryPath)
			cleanupmux.Unlock()
			err = os.RemoveAll(tempDirectoryPath)
			resource.InfoErr(err, "Failed to remove temp directory after upload")
		}()

		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewFileUploadActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := fileUploadActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
