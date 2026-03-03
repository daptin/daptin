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

			crud, ok := cruds[typeName]
			if !ok || crud == nil {
				logrus.Warnf("no crud for type %v in document provider", typeName)
				return []byte{}
			}

			if transaction == nil {
				logrus.Tracef("start transaction for GetDocumentInitialContent")
				var txErr error
				transaction, txErr = crud.Connection().Beginx()
				if txErr != nil {
					return nil
				}
				defer transaction.Rollback()
			}

			object, _, getErr := crud.GetSingleRowByReferenceIdWithTransaction(typeName,
				daptinid.DaptinReferenceId(uuid.MustParse(referenceId)), map[string]bool{
					columnName: true,
				}, transaction)
			logrus.Tracef("Completed NewDiskDocumentProvider GetSingleRowByReferenceIdWithTransaction")
			if getErr != nil {
				logrus.Warnf("failed to get row in document provider: %v", getErr)
				return []byte{}
			}
			if object == nil {
				return []byte{}
			}

			originalFile := object[columnName]
			if originalFile == nil {
				return []byte{}
			}
			columnValueArray, ok := originalFile.([]map[string]interface{})
			if !ok {
				logrus.Warnf("column value is not []map[string]interface{}: %v", originalFile)
				return []byte{}
			}

			fileContentsJson := []byte{}
			for _, file := range columnValueArray {
				if file["type"] != "x-crdt/yjs" {
					continue
				}

				contentsStr, ok := file["contents"].(string)
				if !ok {
					logrus.Warnf("file contents is not a string: %v", file["contents"])
					continue
				}
				decoded, decodeErr := base64.StdEncoding.DecodeString(contentsStr)
				if decodeErr != nil {
					logrus.Warnf("failed to base64 decode file contents: %v", decodeErr)
					continue
				}
				fileContentsJson = decoded

			}

			logrus.Debugf("Completed get initial content for document: %v", documentPath)
			return fileContentsJson
		},
		SetDocumentInitialContent: nil,
	})
	return documentProvider
}
