package resource

import (
	"github.com/artpar/api2go"
	//"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/context"
	"strings"
	//"github.com/lann/ps"
)

type eventHandlerMiddleware struct {
}

func (pc eventHandlerMiddleware) String() string {
	return "EventGenerator"
}

func (pc *eventHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	switch strings.ToLower(req.PlainRequest.Method) {
	case "get":
		break
	case "post":
		break
	case "update":
		break
	case "delete":
		break
	case "patch":
		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return results, nil

}

func (pc *eventHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request) (api2go.Responder, error) {

	var err error = nil
	log.Infof("%v: %v", pc.String(), context.GetAll(req.PlainRequest))

	reqmethod := req.PlainRequest.Method
	log.Infof("Request to intercept: %v", reqmethod)
	switch reqmethod {
	case "GET":
		break
	case "POST":
		break
	case "UPDATE":
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

	return nil, err

}
