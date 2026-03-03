package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/ydb"
	"github.com/buraksezer/olric"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strings"
)

type yjsHandlerMiddleware struct {
	dtopicMap *map[string]*olric.PubSub
	cruds     *map[string]*DbResource
	store     ydb.Store
}

func (pc yjsHandlerMiddleware) String() string {
	return "EventGenerator"
}

func NewYJSHandlerMiddleware(store ydb.Store) DatabaseRequestInterceptor {
	return &yjsHandlerMiddleware{
		dtopicMap: nil,
		cruds:     nil,
		store:     store,
	}
}

func (pc *yjsHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	return results, nil

}

func (pc *yjsHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	requestMethod := strings.ToLower(req.PlainRequest.Method)
	switch requestMethod {
	case "get":
		break
	case "post":
		break

	case "patch":

		for _, obj := range objects {
			var referenceId = daptinid.InterfaceToDIR(obj["reference_id"])

			for _, column := range dr.TableInfo().Columns {
				if BeginsWith(column.ColumnType, "file.") {
					fileColumnValue, ok := obj[column.ColumnName]
					if !ok || fileColumnValue == nil {
						log.Debugf("[57] File column value missing [%v]", column.ColumnName)
						continue
					}
					log.Infof("[60] Process file column with YJS [%s]", column.ColumnName)
					fileColumnValueArray, ok := fileColumnValue.([]interface{})
					if !ok {
						continue
					}
					log.Infof("[66] yjs middleware for column [%v][%v]", dr.tableInfo.TableName, column.ColumnName)

					existingYjsDocument := false
					// there should be only 2 files at max if the column
					if len(fileColumnValueArray) > 1 {
						existingYjsDocument = true
					}

					stateFileExists := make(map[string]bool)

					for _, fileInterface := range fileColumnValueArray {

						file, ok := fileInterface.(map[string]interface{})
						if !ok {
							continue
						}

						if file["type"] == "x-crdt/yjs" {
							filename, ok := file["name"]
							if !ok {
								continue
							}
							filenameStr, ok := filename.(string)
							if !ok {
								continue
							}

							stateFileExists[strings.Split(filenameStr, ".yjs")[0]] = true
						}

					}

					for i, fileInterface := range fileColumnValueArray {

						file, ok := fileInterface.(map[string]interface{})
						if !ok {
							continue
						}
						if file["type"] != "x-crdt/yjs" {
							continue
						}
						filename, ok := file["name"]
						if !ok {
							filename = column.ColumnName + "_" + referenceId.String() + ".txt"
						}
						filenamestring, ok := filename.(string)
						if !ok {
							continue
						}
						if stateFileExists[filenamestring] {
							continue
						}

						var documentName = fmt.Sprintf("%v.%v.%v", dr.tableInfo.TableName, referenceId, column.ColumnName)
						documentHistory, _, readErr := pc.store.ReadFrom(ydb.YjsRoomName(documentName), 0)
						if readErr != nil {
							continue
						}

						if len(documentHistory) < 1 {
							continue
						}

						otherIdx := 1 - i
						if !existingYjsDocument {
							fileColumnValueArray = append(fileColumnValueArray, map[string]interface{}{
								"contents": "x-crdt/yjs," + base64.StdEncoding.EncodeToString(documentHistory),
								"name":     filenamestring + ".yjs",
								"type":     "x-crdt/yjs",
								"path":     file["path"],
							})

						} else if otherIdx >= 0 && otherIdx < len(fileColumnValueArray) {
							fileColumnValueArray[otherIdx] = map[string]interface{}{
								"contents": "x-crdt/yjs," + base64.StdEncoding.EncodeToString(documentHistory),
								"name":     filenamestring + ".yjs",
								"type":     "x-crdt/yjs",
								"path":     file["path"],
							}
						}

						obj[column.ColumnName] = fileColumnValueArray

					}
				}
			}
		}

		break
	case "delete":

		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return objects, nil

}
