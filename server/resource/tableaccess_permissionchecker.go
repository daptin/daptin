package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  //"gopkg.in/Masterminds/squirrel.v1"
  "errors"

  "github.com/gorilla/context"
  "github.com/artpar/goms/server/auth"
)

type TableAccessPermissionChecker struct {
}

func (pc *TableAccessPermissionChecker) String() string {
  return "TableAccessPermissionChecker"
}

func (pc *TableAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

  if results == nil || len(results) < 1 {
    return results, nil
  }

  returnMap := make([]map[string]interface{}, 0)

  userIdString := context.Get(req.PlainRequest, "user_id")
  userGroupId := context.Get(req.PlainRequest, "usergroup_id")

  currentUserId := ""
  if userIdString != nil {
    currentUserId = userIdString.(string)

  }

  currentUserGroupId := []auth.GroupPermission{}
  if userGroupId != nil {
    currentUserGroupId = userGroupId.([]auth.GroupPermission)
  }

  for _, result := range results {
    //log.Infof("Result: %v", result)
    permission := dr.GetRowPermission(result)
    //log.Infof("Row Permission for [%v] for [%v]", permission, result)
    if permission.CanRead(currentUserId, currentUserGroupId) {
      returnMap = append(returnMap, result)
    } else {
      log.Errorf("Result not to be included: %v", result)
    }
  }

  return returnMap, nil

}

func (pc *TableAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request) (api2go.Responder, error) {

  //var err error
  //log.Infof("context: %v", context.GetAll(req.PlainRequest))
  userIdString := context.Get(req.PlainRequest, "user_id")
  userGroupId := context.Get(req.PlainRequest, "usergroup_id")

  currentUserId := ""
  if userIdString != nil {
    currentUserId = userIdString.(string)

  }

  currentUserGroupId := []auth.GroupPermission{}
  if userGroupId != nil {
    currentUserGroupId = userGroupId.([]auth.GroupPermission)
  }

  tableOwnership := dr.GetObjectPermissionByWhereClause("world", "table_name", dr.model.GetName())

  log.Infof("Permission check for action type: [%v] on [%v]", req.PlainRequest.Method, dr.model.GetName())
  if req.PlainRequest.Method == "GET" {
    if !tableOwnership.CanRead(currentUserId, currentUserGroupId) {
      return api2go.Response{
        Code: 403,
      }, errors.New("unauthorized")
    }
  } else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" || req.PlainRequest.Method == "POST" || req.PlainRequest.Method == "DELETE" {
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
