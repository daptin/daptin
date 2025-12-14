package server

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/ydb"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func ProcessEventMessage(eventMessage resource.EventMessage, msg *redis.Message, typename string, cruds map[string]*resource.DbResource, columnInfo api2go.ColumnInfo, documentProvider ydb.DocumentProvider) error {
	var err error
	err = eventMessage.UnmarshalBinary([]byte(msg.Payload))
	if err != nil {
		resource.CheckErr(err, "Failed to read message on channel "+typename)
		return nil
	}
	if eventMessage.EventType == "update" && eventMessage.ObjectType == typename {
		eventDataMap := make(map[string]interface{})
		err := json.Unmarshal(eventMessage.EventData, &eventDataMap)
		resource.CheckErr(err, "Failed to unmarshal message ["+eventMessage.ObjectType+"]")
		referenceId := uuid.MustParse(eventDataMap["reference_id"].(string))

		colData, ok := eventDataMap[columnInfo.ColumnName]
		if ok && colData != nil {
			colDataMap, ok := colData.([]interface{})
			if ok {
				for _, file := range colDataMap {
					fileMap := file.(map[string]interface{})
					if fileMap["type"] != "x-crdt/yjs" {
						return nil
					}
				}
			}
		}

		transaction1, err := cruds[typename].Connection().Beginx()
		defer transaction1.Rollback()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [788]")
			return nil
		}

		object, _, _ := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename, daptinid.DaptinReferenceId(referenceId), map[string]bool{
			columnInfo.ColumnName: true,
		}, transaction1)
		logrus.Tracef("Completed dtopicMapListener GetSingleRowByReferenceIdWithTransaction")

		colValue := object[columnInfo.ColumnName]
		if colValue == nil {
			return nil
		}
		columnValueArray, ok := colValue.([]map[string]interface{})
		if !ok {
			logrus.Warnf("value is not of type array - %v", colValue)
			return nil
		}

		fileContentsJson := []byte{}
		for _, file := range columnValueArray {
			if file["type"] != "x-crdt/yjs" {
				continue
			}
			fileContentsJson, _ = base64.StdEncoding.DecodeString(file["contents"].(string))
		}

		documentName := fmt.Sprintf("%v.%v.%v", typename, referenceId, columnInfo.ColumnName)
		document := documentProvider.GetDocument(ydb.YjsRoomName(documentName), transaction1)
		if document != nil && len(fileContentsJson) > 0 {
			document.SetInitialContent(fileContentsJson)
		}

	}
	return err
}
