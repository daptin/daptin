package resource

import (
	"github.com/artpar/rclone/cmd"
	"encoding/base64"
	log "github.com/sirupsen/logrus"
	"github.com/artpar/rclone/fs"
	"io/ioutil"
	"github.com/satori/go.uuid"
	//"os"
	_ "github.com/artpar/rclone/fs/all" // import all fs
	"path/filepath"
	"strings"
	"github.com/gin-gonic/gin/json"
	"archive/zip"
	"io"
	"os"
	"context"
)

type FileUploadActionPerformer struct {
	cruds     map[string]*DbResource
	cmsConfig *CmsConfig
}

func (d *FileUploadActionPerformer) Name() string {
	return "__external_file_upload"
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

func (d *FileUploadActionPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) ([]ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	sourceDirectoryName := uuid.NewV4().String()
	tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)
	log.Infof("Temp directory for this upload: %v", tempDirectoryPath)
	targetStorageDetails := inFields["subject"].(map[string]interface{})

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	files := inFields["file"].([]interface{})
	for _, fileInterface := range files {
		file := fileInterface.(map[string]interface{})
		fileName := file["name"].(string)
		temproryFilePath := filepath.Join(tempDirectoryPath, fileName)

		fileContentsBase64 := file["file"].(string)
		fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
		log.Infof("Write file [%v] for upload", temproryFilePath)
		CheckErr(err, "Failed to convert base64 to []bytes")

		err = ioutil.WriteFile(temproryFilePath, fileBytes, 0666)
		CheckErr(err, "Failed to write file bytes to temp file for rclone upload")

		if EndsWithCheck(fileName, ".zip") {
			unzip(temproryFilePath, tempDirectoryPath)
			err = os.Remove(temproryFilePath)
			CheckErr(err, "Failed to remove zip file after extraction")
		}

	}

	targetInformation := inFields["subject"]
	targetInformationMap := targetInformation.(map[string]interface{})
	rootPath := targetInformationMap["root_path"].(string)
	args := []string{
		tempDirectoryPath,
		rootPath,
	}

	oauthTokenId := targetStorageDetails["oauth_token_id"].(string)
	token, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
	oauthConf, err := d.cruds["oauth_token"].GetOauthDescriptionByTokenReferenceId(oauthTokenId)
	if err != nil {
		log.Errorf("Failed to get oauth token for store sync: %v", err)
		return nil, []error{err}
	}

	if !token.Valid() {
		ctx := context.Background()
		tokenSource := oauthConf.TokenSource(ctx, token)
		token, err = tokenSource.Token()
		CheckErr(err, "Failed to get new access token")
		err = d.cruds["oauth_token"].UpdateAccessTokenByTokenReferenceId(oauthTokenId, token.AccessToken, token.Expiry.Unix())
		CheckErr(err, "failed to update access token")
	}

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to marshal access token to json")
	configFile := filepath.Join(tempDirectoryPath, "upload.conf")
	fs.LoadConfig()
	fs.ConfigPath = configFile
	fs.Config.DryRun = false
	fs.Config.LogLevel = 200
	fs.Config.StatsLogLevel = 200
	storeProvider := targetStorageDetails["store_provider"].(string)
	fs.ConfigFileSet(storeProvider, "client_id", oauthConf.ClientID)
	fs.ConfigFileSet(storeProvider, "type", targetInformationMap["store_provider"].(string))
	fs.ConfigFileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
	fs.ConfigFileSet(storeProvider, "token", string(jsonToken))
	fs.ConfigFileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
	fs.ConfigFileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

	fsrc, fdst := cmd.NewFsSrcDst(args)

	go cmd.Run(true, true, nil, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}
		dir := fs.CopyDir(fdst, fsrc)
		os.RemoveAll(tempDirectoryPath)
		return dir
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Initiating system update."
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return responses, nil
}

func NewFileUploadActionPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := FileUploadActionPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil

}
