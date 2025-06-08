package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	hugoCommand "github.com/gohugoio/hugo/commands"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type cloudStoreSiteCreateActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreSiteCreateActionPerformer) Name() string {
	return "cloudstore.site.create"
}

func (d *cloudStoreSiteCreateActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Printf("Temp directory for this upload cloudStoreSiteCreateActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	resource.CheckErr(err, "Failed to create temp tempDirectoryPath for site create")
	site_type, _ := inFields["site_type"].(string)
	user_account_idStr, err := uuid.Parse(inFields["user_account_id"].(string))
	user_account_id := user_account_idStr
	cloud_store_idStr, err := uuid.Parse(inFields["cloud_store_id"].(string))
	cloud_store_id := cloud_store_idStr

	switch site_type {
	case "hugo":
		log.Printf("Starting hugo build for in cloud store create %v", tempDirectoryPath)
		hugoCommandResponse := hugoCommand.Execute([]string{"new", "site", tempDirectoryPath})
		log.Printf("Hugo command response for site create[%v]: %v", tempDirectoryPath, hugoCommandResponse)
	default:

	}

	rootPath := inFields["root_path"].(string)
	hostname, ok := inFields["hostname"].(string)
	if !ok {
		return nil, nil, []error{errors.New("hostname is missing")}
	}
	path := inFields["path"].(string)

	if path != "" {
		if !EndsWithCheck(rootPath, "/") && !resource.BeginsWith(path, "/") {
			path = "/" + path
		}
		rootPath = rootPath + path
	}

	args := []string{
		tempDirectoryPath,
		rootPath,
	}

	ur, _ := url.Parse("/site")

	plainRequest := &http.Request{
		URL: ur,
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(user_account_id),
	})
	plainRequest = plainRequest.WithContext(ctx)
	createRequest := api2go.Request{
		PlainRequest: plainRequest,
	}

	newSiteData := map[string]interface{}{
		"hostname":       hostname,
		"path":           path,
		"cloud_store_id": cloud_store_id,
		"site_type":      site_type,
		"name":           hostname,
	}
	newSite := api2go.NewApi2GoModelWithData("site", nil, 0, nil, newSiteData)
	_, err = d.cruds["site"].CreateWithoutFilter(newSite, createRequest, transaction)
	resource.CheckErr(err, "Failed to create new site")
	if err != nil {
		return nil, nil, []error{err}
	}

	log.Printf("Upload source target for site create %v %v", tempDirectoryPath, rootPath)

	storeName := strings.Split(rootPath, ":")[0]

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("File upload action from [%v]", tempDirectoryPath),
	}

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		err := sync.CopyDir(ctx, fdst, fsrc, true)
		resource.InfoErr(err, "Failed to sync files for upload to cloud after site create")
		err = os.RemoveAll(tempDirectoryPath)
		resource.InfoErr(err, "Failed to remove temp directory after upload in site create")
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

func NewCloudStoreSiteCreateActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStoreSiteCreateActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
