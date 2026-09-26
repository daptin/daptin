package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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

	cloudStores := make([]string, 0)

	requiredColumns := make(map[string]interface{})
	defaltValues := make(map[string]interface{})
	for _, col := range tableCrud.TableInfo().Columns {
		if col.IsForeignKey && col.ForeignKeyData.DataSource == "cloud_store" {
			cloudStores = append(cloudStores, col.ColumnName)
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
	for _, colName := range cloudStores {

		cacheFolder := d.cruds[tableName].AssetFolderCache[tableName][colName]
		if cacheFolder == nil {
			return nil, nil, []error{fmt.Errorf("cloud store is not configured for [%s][%s]", tableName, colName)}
		}

		defaltValues["version"] = 1
		defaltValues["created_at"] = time.Now()
		defaltValues["permission"] = cacheFolder.CloudStore.Permission.Permission.String()
		userId, err := d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId(resource.USER_ACCOUNT_TABLE_NAME, cacheFolder.CloudStore.UserId, transaction)
		if err != nil {
			return nil, nil, []error{err}
		}
		defaltValues["user_account_id"] = userId

		files, err := cacheFolder.ListStoredFiles(context.Background(), "")
		if err != nil {
			return nil, nil, []error{err}
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fileData, _ := json.Marshal([]map[string]string{{"name": file.Name()}})
			u, _ := uuid.NewV7()
			defaltValues["reference_id"] = u[:]
			defaltValues[colName] = string(fileData)
			if err := d.cruds[tableName].DirectInsert(tableName, defaltValues, transaction); err != nil {
				countFail++
			} else {
				countSuccess++
			}
		}
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
