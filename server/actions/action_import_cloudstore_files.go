package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

// importCloudStoreFilesPerformer daptin action implementation
type importCloudStoreFilesPerformer struct {
	cruds map[string]*resource.DbResource
}

// Name of the action
func (d *importCloudStoreFilesPerformer) Name() string {
	return "cloud_store.files.import"
}

// importCloudStoreFilesPerformer Imports files metadata from a cloud store
func (d *importCloudStoreFilesPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

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
			defaltValues[col.ColumnName] = resource.ColumnManager.GetFakeData(col.ColumnType)
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
		userId, err := d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId(resource.USER_ACCOUNT_TABLE_NAME, cacheFolder.CloudStore.UserId, transaction)
		resource.CheckErr(err, "Failed to get id from reference id: %v", userId)
		defaltValues["user_account_id"] = userId

		if cacheFolder.CloudStore.CredentialName != "" {
			cred, err := d.cruds["credential"].GetCredentialByName(cacheFolder.CloudStore.CredentialName, transaction)
			resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", cacheFolder.CloudStore.CredentialName))
			if cred.DataMap != nil {
				for key, val := range cred.DataMap {
					config.Data().SetValue(cacheFolder.CloudStore.Name, key, fmt.Sprintf("%s", val))
				}
			}
		}

		fsrc := cmd.NewFsDir([]string{cacheFolder.CloudStore.RootPath + "/" + colFkdata.KeyName})
		cobraCommand := &cobra.Command{
			Use: fmt.Sprintf("list files from from [%v] %v", cacheFolder.CloudStore.Name, fsrc),
		}
		defaultConfig := fs.GetConfig(nil)
		defaultConfig.LogLevel = fs.LogLevelNotice

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
				u, _ := uuid.NewV7()
				defaltValues["reference_id"] = u[:]
				defaltValues[colName] = string(fileData)

				err = d.cruds[tableName].DirectInsert(tableName, defaltValues, transaction)
				resource.CheckErr(err, "Failed to insert file record [%v]: %v", defaltValues, err)
				if err != nil {
					countFail += 1
				} else {
					countSuccess += 1
				}
				return nil
			})

			resource.InfoErr(err, "Failed to sync files for upload to cloud")
			return err
		})
	}

	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("client.notify", map[string]interface{}{
		"message": fmt.Sprintf("Imported success %d files, failed %d files", countSuccess, countFail),
	})}, nil
}

// Create a new action performer for becoming administrator action
func NewImportCloudStoreFilesPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := importCloudStoreFilesPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
