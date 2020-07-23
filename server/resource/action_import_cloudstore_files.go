package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"strings"
	"time"
)

// ImportCloudStoreFilesPerformer daptin action implementation
type ImportCloudStoreFilesPerformer struct {
	cruds map[string]*DbResource
}

// Name of the action
func (d *ImportCloudStoreFilesPerformer) Name() string {
	return "cloud_store.files.import"
}

// ImportCloudStoreFilesPerformer Imports files metadata from a cloud store
func (d *ImportCloudStoreFilesPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	tableName := inFields["table_name"].(string)
	//columnName := inFieldMap["column_name"].(string)
	//cloudStoreReferenceid := inFieldMap["cloud_store_id"].(string)

	tableCrud, ok := d.cruds[tableName]
	if !ok {
		return nil, nil, []error{errors.New("invalid table")}
	}

	cloudStores := make(map[string]api2go.ForeignKeyData, 0)

	requiredColumns := make(map[string]interface{})
	defaltValues := make(map[string]interface{})
	for _, col := range tableCrud.TableInfo().Columns {
		if strings.Index(col.ColumnType, ".") > -1 && col.IsForeignKey && col.ForeignKeyData.DataSource == "cloud_store" {
			cloudStores[col.ColumnName] = col.ForeignKeyData
		}
		if col.DefaultValue != "" {
			defaultValue := col.DefaultValue
			if len(defaultValue) > 1 && defaultValue[0] == defaultValue[len(defaultValue)-1] {
				defaultValue = defaultValue[1 : len(defaultValue)-1]
			}
			requiredColumns[col.ColumnName] = col.DefaultValue
		} else if !col.IsNullable && col.ColumnName != "id" {
			defaltValues[col.ColumnName] = ColumnManager.GetFakeData(col.ColumnType)
		}
	}
	for key, val := range defaltValues {
		defaltValues[key] = val
	}

	countSuccess := 0
	countFail := 0
	for colName, colFkdata := range cloudStores {

		cacheFolder := d.cruds[tableName].AssetFolderCache[tableName][colName]

		defaltValues["version"] = 1
		defaltValues["created_at"] = time.Now()
		defaltValues["permission"] = cacheFolder.CloudStore.Permission.Permission.String()
		userId, err := d.cruds[USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId(USER_ACCOUNT_TABLE_NAME, cacheFolder.CloudStore.UserId)
		CheckErr(err, "Failed to get id from reference id: %v", userId)
		defaltValues["user_account_id"] = userId

		var token *oauth2.Token
		oauthConf := &oauth2.Config{}
		oauthTokenId := cacheFolder.CloudStore.OAutoTokenId

		token, oauthConf, err = d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		CheckErr(err, "Failed to get oauth2 token for store sync")

		jsonToken, err := json.Marshal(token)
		CheckErr(err, "Failed to marshal access token to json")

		config.FileSet(cacheFolder.CloudStore.StoreProvider, "client_id", oauthConf.ClientID)
		config.FileSet(cacheFolder.CloudStore.StoreProvider, "type", cacheFolder.CloudStore.StoreProvider)
		config.FileSet(cacheFolder.CloudStore.StoreProvider, "client_secret", oauthConf.ClientSecret)
		config.FileSet(cacheFolder.CloudStore.StoreProvider, "token", string(jsonToken))
		config.FileSet(cacheFolder.CloudStore.StoreProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		config.FileSet(cacheFolder.CloudStore.StoreProvider, "redirect_url", oauthConf.RedirectURL)

		fsrc := cmd.NewFsDir([]string{cacheFolder.CloudStore.StoreProvider + ":" + cacheFolder.CloudStore.RootPath + "/" + colFkdata.KeyName})
		cobraCommand := &cobra.Command{
			Use: fmt.Sprintf("list files from from [%v] %v", cacheFolder.CloudStore.StoreProvider, fsrc),
		}
		fs.Config.LogLevel = fs.LogLevelNotice

		cmd.Run(true, false, cobraCommand, func() error {
			if fsrc == nil {
				log.Errorf("Source or destination is null")
				return nil
			}

			ctx := context.Background()

			err := operations.ListJSON(ctx, fsrc, "", &operations.ListJSONOpt{
				ShowHash:  true,
				FilesOnly: true,
			}, func(item *operations.ListJSONItem) error {

				log.Printf("Import file to table [%v] %v", tableName, item.Name)
				fileData, _ := json.Marshal([]map[string]string{
					{
						"name": item.Name,
					},
				})
				u, _ := uuid.NewV4()
				newUuid := u.String()
				defaltValues["reference_id"] = newUuid
				defaltValues[colName] = string(fileData)

				err = d.cruds[tableName].DirectInsert(tableName, defaltValues)
				CheckErr(err, "Failed to insert file record [%v]: %v", defaltValues, err)
				if err != nil {
					countFail += 1
				} else {
					countSuccess += 1
				}
				return nil
			})

			InfoErr(err, "Failed to sync files for upload to cloud")
			return err
		})
	}

	return nil, []ActionResponse{NewActionResponse("client.notify", map[string]interface{}{
		"message": fmt.Sprintf("Imported success %d files, failed %d files", countSuccess, countFail),
	})}, nil
}

// Create a new action performer for becoming administrator action
func NewImportCloudStoreFilesPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := ImportCloudStoreFilesPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
