package resource

import (
  "github.com/artpar/api2go"
  //"errors"
  log "github.com/Sirupsen/logrus"
  "github.com/gorilla/context"
  "strings"
)

type eventHandlerMiddleware struct {
}

func (pc *eventHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

  log.Infof("Request to intercept: %v", req)

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

  return nil, nil

}

func (pc *eventHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request) (api2go.Responder, error) {

  var err error = nil
  log.Infof("context: %v", context.GetAll(req.PlainRequest))

  log.Infof("Request to intercept: %v", req)
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

  //currentUserId := context.Get(req.PlainRequest, "user_id").(string)
  //currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

  return nil, err

}