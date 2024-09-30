package resource

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/artpar/api2go"
	"github.com/buraksezer/olric"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strings"
)

type eventHandlerMiddleware struct {
	dtopicMap *map[string]*olric.PubSub
	cruds     *map[string]*DbResource
}

func (pc eventHandlerMiddleware) String() string {
	return "EventGenerator"
}

type EventMessage struct {
	MessageSource string
	EventType     string
	ObjectType    string
	EventData     []byte
}

// MarshalBinary encodes the struct into binary format manually
func (e EventMessage) MarshalBinary() (data []byte, err error) {
	buffer := new(bytes.Buffer)

	// Encode MessageSource
	if err := encodeString(buffer, e.MessageSource); err != nil {
		return nil, err
	}

	// Encode EventType
	if err := encodeString(buffer, e.EventType); err != nil {
		return nil, err
	}

	// Encode ObjectType
	if err := encodeString(buffer, e.ObjectType); err != nil {
		return nil, err
	}

	// Simplified handling for EventData: encoding just the length (this should be replaced with actual data encoding logic)
	jsonStr := string(e.EventData)
	if err := encodeString(buffer, jsonStr); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the data into the struct using manual binary decoding
func (e *EventMessage) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)

	// Decode MessageSource
	if msgSource, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.MessageSource = msgSource
	}

	// Decode EventType
	if eventType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.EventType = eventType
	}

	// Decode ObjectType
	if objectType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ObjectType = objectType
	}

	// Assume EventData is just the count of items (real logic needed to parse actual data)
	if eventDataJson, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.EventData = []byte(eventDataJson)
		return err
	}
}

// Helper functions to encode and decode strings
func encodeString(buffer *bytes.Buffer, s string) error {
	length := int32(len(s))
	if err := binary.Write(buffer, binary.BigEndian, length); err != nil {
		return err
	}
	if _, err := buffer.WriteString(s); err != nil {
		return err
	}
	return nil
}

func decodeString(buffer *bytes.Buffer) (string, error) {
	var length int32
	if err := binary.Read(buffer, binary.BigEndian, &length); err != nil {
		return "", err
	}
	data := make([]byte, length)
	if _, err := buffer.Read(data); err != nil {
		return "", err
	}
	return string(data), nil
}

func (pc *eventHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	tableName := dr.model.GetTableName()
	topic := (*pc.dtopicMap)[tableName]
	if topic == nil {
		return results, nil
	}

	switch strings.ToLower(req.PlainRequest.Method) {
	case "get":
		break
	case "post":
		messageBytes, err := json.Marshal(results[0])
		go func() {
			CheckErr(err, "Failed to serialize patch message")
			_, err = topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "create",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
			})
			CheckErr(err, "Failed to publish create message")
		}()
		break
	case "delete":
		messageBytes, err := json.Marshal(results[0])
		go func() {
			CheckErr(err, "Failed to serialize patch message")
			_, err = topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "delete",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
			})
			CheckErr(err, "Failed to delete create message")

		}()
		break
	case "patch":
		messageBytes, err := json.Marshal(results[0])
		go func() {
			CheckErr(err, "Failed to serialize patch message")
			_, err = topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "update",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
			})
			CheckErr(err, "Failed to update create message")
		}()
		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return results, nil

}

func (pc *eventHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	reqmethod := req.PlainRequest.Method
	//log.Printf("Generate events for objects", reqmethod)
	switch reqmethod {
	case "GET":
		break
	case "POST":
		break
	case "DELETE":
		break
	case "PATCH":
		break
	default:
		log.Errorf("Invalid method: %v", reqmethod)
	}

	//currentUserId := context.Get(req.PlainRequest, "user_id").(string)
	//currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

	return objects, nil

}
