package resource

import (
	"context"
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
	EventData     map[string]interface{}
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
		go func() {
			_, err := topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "create",
				ObjectType:    dr.model.GetTableName(),
				EventData:     results[0],
			})
			CheckErr(err, "Failed to publish create message")
		}()
		break
	case "delete":
		go func() {
			_, err := topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "delete",
				ObjectType:    dr.model.GetTableName(),
				EventData:     results[0],
			})
			CheckErr(err, "Failed to delete create message")

		}()
		break
	case "patch":
		go func() {
			_, err := topic.Publish(context.Background(), tableName, EventMessage{
				MessageSource: "database",
				EventType:     "update",
				ObjectType:    dr.model.GetTableName(),
				EventData:     results[0],
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
