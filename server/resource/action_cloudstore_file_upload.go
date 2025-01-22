package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	uuid "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sync2 "sync"
	"time"

	//"os"
	"archive/zip"
	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type fileUploadActionPerformer struct {
	cruds map[string]*DbResource
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
		path := filepath.Join(target, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
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

func EndsWithCheck(str string, endsWith string) bool {
	if len(endsWith) > len(str) {
		return false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return false
	}

	suffix := str[len(str)-len(endsWith):]
	i := suffix == endsWith
	return i

}
func EndsWith(str string, endsWith string) (string, bool) {
	if len(endsWith) > len(str) {
		return "", false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return "", false
	}

	suffix := str[len(str)-len(endsWith):]
	prefix := str[:len(str)-len(endsWith)]
	i := suffix == endsWith
	return prefix, i

}

var cleanupmux = sync2.Mutex{}
var cleanuppath = make(map[string]bool)

func (actionPerformer *fileUploadActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Debugf("Temp directory for this upload fileUploadActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	atPath, ok := inFields["path"].(string)
	files, ok := inFields["file"].([]interface{})
	if ok {

		for _, fileInterface := range files {
			file := fileInterface.(map[string]interface{})
			fileName, ok := file["name"].(string)
			if !ok {
				log.Errorf("Name is missing for file")
				continue
			}
			temproryFilePath := filepath.Join(tempDirectoryPath, fileName)

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
				encodedPart = splitParts[1]
			}
			fileBytes, err := base64.StdEncoding.DecodeString(encodedPart)
			log.Printf("Write file [%v] for upload", temproryFilePath)
			CheckErr(err, "Failed to convert base64 to []bytes")

			os.MkdirAll(tempDirectoryPath, 0600)

			err = os.WriteFile(temproryFilePath, fileBytes, 0666)
			CheckErr(err, "Failed to write file bytes to temp file for rclone upload")

			if EndsWithCheck(fileName, ".zip") {
				err = unzip(temproryFilePath, tempDirectoryPath)
				CheckErr(err, "Failed to unzip file")
				go func() {
					time.Sleep(5 * time.Minute)
					err = os.Remove(temproryFilePath)
					CheckErr(err, "Failed to remove zip file after extraction")
				}()

			}

		}
		CheckErr(err, "Failed to remove cache folder: %s", tempDirectoryPath)
	} else {
		return nil, nil, []error{errors.New("improper file attachment")}
	}

	rootPath := inFields["root_path"].(string)
	if atPath != "" {

		if !EndsWithCheck(rootPath, "/") && len(atPath) > 0 && atPath[0] != '/' {
			rootPath = rootPath + "/"
		}
		rootPath = rootPath + atPath
	}
	args := []string{
		tempDirectoryPath,
		rootPath,
	}
	log.Infof("[183] Upload source [%v] target [%v]", tempDirectoryPath, rootPath)

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := actionPerformer.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		name := inFields["name"].(string)
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(name, key, fmt.Sprintf("%s", val))
			}
		}
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("File upload action from [%v]", tempDirectoryPath),
	}
	defaultConfig := fs.GetConfig(nil)

	defaultConfig.LogLevel = fs.LogLevelDebug

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		ctx := context.Background()

		defaultConfig.DeleteMode = fs.DeleteModeOff
		err := sync.CopyDir(ctx, fdst, fsrc, true)
		InfoErr(err, "Failed to sync files for upload to cloud")

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
			InfoErr(err, "Failed to remove temp directory after upload")
		}()

		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewFileUploadActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := fileUploadActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
