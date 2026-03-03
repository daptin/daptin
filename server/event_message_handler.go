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
		err = json.Unmarshal(eventMessage.EventData, &eventDataMap)
		resource.CheckErr(err, "Failed to unmarshal message ["+eventMessage.ObjectType+"]")
		stringReferenceId := eventDataMap["reference_id"]
		if stringReferenceId == nil {
			logrus.Warnf("no reference id in event data map %v", eventDataMap)
			return nil
		}

		stringRefId, ok := stringReferenceId.(string)
		if !ok {
			logrus.Warnf("reference_id is not a string: %v", stringReferenceId)
			return nil
		}
		referenceId, parseErr := uuid.Parse(stringRefId)
		if parseErr != nil {
			logrus.Warnf("failed to parse reference_id as UUID: %v", stringRefId)
			return nil
		}

		colData, ok := eventDataMap[columnInfo.ColumnName]
		if ok && colData != nil {
			colDataMap, ok := colData.([]interface{})
			if ok {
				for _, file := range colDataMap {
					fileMap, ok := file.(map[string]interface{})
					if !ok {
						continue
					}
					if fileMap["type"] != "x-crdt/yjs" {
						return nil
					}
				}
			}
		}

		transaction1, txErr := cruds[typename].Connection().Beginx()
		if txErr != nil {
			resource.CheckErr(txErr, "Failed to begin transaction [788]")
			return nil
		}
		defer transaction1.Rollback()

		object, _, getErr := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename, daptinid.DaptinReferenceId(referenceId), map[string]bool{
			columnInfo.ColumnName: true,
		}, transaction1)
		logrus.Tracef("Completed dtopicMapListener GetSingleRowByReferenceIdWithTransaction")
		if getErr != nil {
			logrus.Warnf("failed to get row by reference id: %v", getErr)
			return nil
		}
		if object == nil {
			logrus.Warnf("object not found for reference id: %v", referenceId)
			return nil
		}

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

		documentName := fmt.Sprintf("%v.%v.%v", typename, referenceId, columnInfo.ColumnName)
		document := documentProvider.GetDocument(ydb.YjsRoomName(documentName), transaction1)
		if document != nil && len(fileContentsJson) > 0 {
			document.SetInitialContent(fileContentsJson)
		}

	}
	return err
}
