package server

import (
	"encoding/base64"
	"github.com/artpar/ydb"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

// PathExistsAndIsFolder checks if a path exists and is a folder
func PathExistsAndIsFolder(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false // Path does not exist
	}
	if err != nil {
		return false // Other errors
	}
	return info.IsDir() // Check if it's a directory
}

func CreateYjsDocumentProvider(configStore *resource.ConfigStore, transaction *sqlx.Tx, localStoragePath string, documentProvider ydb.DocumentProvider, cruds map[string]*resource.DbResource) ydb.DocumentProvider {
	logrus.Infof("YJS endpoint is enabled in config")
	yjs_temp_directory, err := configStore.GetConfigValueFor("yjs.storage.path", "backend", transaction)
	//if err != nil {
	yjs_temp_directory = localStoragePath + "/yjs-documents"
	configStore.SetConfigValueFor("yjs.storage.path", yjs_temp_directory, "backend", transaction)
	//}

	if !PathExistsAndIsFolder(yjs_temp_directory) {
		err = os.Mkdir(yjs_temp_directory, 0777)
		if err != nil {
			resource.CheckErr(err, "Failed to create yjs storage directory")
		}
	}

	documentProvider = ydb.NewDiskDocumentProvider(yjs_temp_directory, 10000, ydb.DocumentListener{
		GetDocumentInitialContent: func(documentPath string, transaction *sqlx.Tx) []byte {
			logrus.Debugf("Get initial content for document: %v", documentPath)
			pathParts := strings.Split(documentPath, ".")
			typeName := pathParts[0]
			referenceId := pathParts[1]
			columnName := pathParts[2]
			if transaction == nil {
				logrus.Tracef("start transaction for GetDocumentInitialContent")
				transaction, err = cruds[typeName].Connection().Beginx()
				if err != nil {
					return nil
				}
				defer transaction.Rollback()
			}

			object, _, _ := cruds[typeName].GetSingleRowByReferenceIdWithTransaction(typeName,
				daptinid.DaptinReferenceId(uuid.MustParse(referenceId)), map[string]bool{
					columnName: true,
				}, transaction)
			logrus.Tracef("Completed NewDiskDocumentProvider GetSingleRowByReferenceIdWithTransaction")

			originalFile := object[columnName]
			if originalFile == nil {
				return []byte{}
			}
			columnValueArray := originalFile.([]map[string]interface{})

			fileContentsJson := []byte{}
			for _, file := range columnValueArray {
				if file["type"] != "x-crdt/yjs" {
					continue
				}

				fileContentsJson, _ = base64.StdEncoding.DecodeString(file["contents"].(string))

			}

			logrus.Debugf("Completed get initial content for document: %v", documentPath)
			return fileContentsJson
		},
		SetDocumentInitialContent: nil,
	})
	return documentProvider
}
