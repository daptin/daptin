package resource

import (
	"encoding/base64"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/cmd"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	//"os"
	"archive/zip"
	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"golang.org/x/oauth2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"encoding/json"
)

type FileUploadActionPerformer struct {
	cruds map[string]*DbResource
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

func (d *FileUploadActionPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV4()
	sourceDirectoryName := u.String()
	tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)
	log.Infof("Temp directory for this upload: %v", tempDirectoryPath)

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

	//targetInformation := inFields["subject"]
	//targetInformationMap := targetInformation.(map[string]interface{})
	rootPath := inFields["root_path"].(string)
	args := []string{
		tempDirectoryPath,
		rootPath,
	}

	var token *oauth2.Token
	oauthConf := &oauth2.Config{}
	oauthTokenId1 := inFields["oauth_token_id"]
	if oauthTokenId1 == nil {
		log.Infof("No oauth token set for target store")
	} else {
		oauthTokenId := oauthTokenId1.(string)
		token, oauthConf, err = d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		CheckErr(err, "Failed to get oauth2 token for store sync")
	}

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to marshal access token to json")

	storeProvider := inFields["store_provider"].(string)
	config.FileSet(storeProvider, "client_id", oauthConf.ClientID)
	config.FileSet(storeProvider, "type", storeProvider)
	config.FileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
	config.FileSet(storeProvider, "token", string(jsonToken))
	config.FileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
	config.FileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

	fsrc, fdst := cmd.NewFsSrcDst(args)

	go cmd.Run(true, true, nil, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}
		err := sync.CopyDir(fdst, fsrc)
		os.RemoveAll(tempDirectoryPath)
		InfoErr(err, "Failed to sync files for upload to cloud")
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

	handler := FileUploadActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
