package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  //"gopkg.in/Masterminds/squirrel.v1"
  "errors"

  "github.com/gorilla/context"
)

type TableAccessPermissionChecker struct {
}

func (pc *TableAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

  if results == nil || len(results) < 1 {
    return results, nil
  }

  returnMap := make([]map[string]interface{}, 0)

  //var err error
  currentUserId := context.Get(req.PlainRequest, "user_id").(string)
  currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

  for _, result := range results {
    //log.Infof("Result: %v", result)
    permission := dr.GetRowPermission(result)
    //log.Infof("Row Permission for [%v] for [%v]", permission, result)
    if permission.CanRead(currentUserId, currentUserGroupId) {
      returnMap = append(returnMap, result)
    }
  }

  return returnMap, nil

}

func (pc *TableAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request) (api2go.Responder, error) {

  //var err error
  log.Infof("context: %v", context.GetAll(req.PlainRequest))
  currentUserId := context.Get(req.PlainRequest, "user_id").(string)
  currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

  tableOwnership := dr.GetTablePermission(dr.model.GetName())

  if req.PlainRequest.Method == "GET" {
    if !tableOwnership.CanRead(currentUserId, currentUserGroupId) {
      return api2go.Response{
        Code: 403,
      }, errors.New("unauthorized")
    }
  } else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "POST" || req.PlainRequest.Method == "DELETE" {
    if !tableOwnership.CanWrite(currentUserId, currentUserGroupId) {
      return api2go.Response{
        Code: 403,
      }, errors.New("unauthorized")

    }
  } else {
    return api2go.Response{
      Code: 403,
    }, errors.New("unauthorized")

  }

  return nil, nil

}
