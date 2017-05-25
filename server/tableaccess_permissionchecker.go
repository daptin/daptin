package server

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "errors"
  "strconv"
)

type TableAccessPermissionChecker struct {
}

func (pc *TableAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

  returnMap := make([]map[string]interface{}, 0)

  var err error
  currentUser, okUser := req.Context.Get("user")
  var currentUserId string
  //currentUserGroup, okGroup := req.Context.Get("usergroup")

  log.Infof("Check after read permission on table [%v]", dr.model.GetName())
  if !okUser || currentUser == nil {
    authToken := req.Header.Get("Authorization")

    users, err := dr.GetRowsByWhereClause("user", squirrel.Eq{"email": authToken})
    if err != nil {
      return nil, err
    }

    if len(users) < 1 {
      m := make(map[string]interface{})
      dr.db.QueryRowx("select * from user order by id limit 1").MapScan(m)
      currentUser = m
      currentUserId = string(m["reference_id"].([]byte))
    } else {
      currentUser = users[0]
      currentUserId = string(users[0]["reference_id"].([]byte))
    }
  } else {
    currentUserId = string(currentUser.(map[string]interface{})["reference_id"].([]byte))
  }

  log.Infof("Current user: %v", currentUser)
  currentUserMap := currentUser.(map[string]interface{})
  currentUserGroupId := string(currentUserMap["usergroup_id"].([]byte))
  okGroup := false
  var currentUserGroup []map[string]interface{}

  log.Infof("Current user group id: %v", currentUserGroupId)
  if !okGroup || currentUserGroup == nil {
    currentUserGroup, err = dr.GetRowsByWhereClause("usergroup", squirrel.Eq{"id": currentUserGroupId})
    if len(currentUserGroup) < 1 {
      log.Errorf("No group from user group id [%v]", currentUserGroupId)
      return nil, errors.New("failed")
    }
    currentUserGroupId = currentUserGroup[0]["reference_id"].(string)
    if err != nil {
      log.Errorf("Failed to load user group of user [%v]", currentUser)
    }
  }

  for _, result := range results {
    log.Infof("Result: %v", result)
    permission := dr.GetRowPermission(result)
    log.Infof("Row Permission for [%v] for [%v]", permission, result)
    if permission.CanRead(currentUserId, currentUserGroupId) {
      returnMap = append(returnMap, result)
    }
  }

  return returnMap, nil

}
func (pc *TableAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request) (api2go.Responder, error) {

  //var err error
  currentUser, okUser := req.Context.Get("user")
  var currentUserId string
  //currentUserGroup, okGroup := req.Context.Get("usergroup")

  log.Infof("Check read permission on table [%v]", dr.model.GetName())
  if !okUser || currentUser == nil {
    authToken := req.Header.Get("Authorization")

    users, err := dr.GetRowsByWhereClause("user", squirrel.Eq{"email": authToken})
    if err != nil {
      return nil, err
    }

    if len(users) < 1 {
      m := make(map[string]interface{})
      dr.db.QueryRowx("select * from user order by id limit 1").MapScan(m)
      currentUser = m
      currentUserId = string(m["reference_id"].([]byte))
    } else {
      currentUser = users[0]
      currentUserId = string(users[0]["reference_id"].([]byte))
    }
  } else {
    currentUserId = string(currentUser.(map[string]interface{})["reference_id"].([]byte))
  }

  log.Infof("Current user: %v", currentUser)
  currentUserMap := currentUser.(map[string]interface{})
  currentUserGroupId := string(currentUserMap["usergroup_id"].([]byte))
  okGroup := false
  var currentUserGroup []map[string]interface{}

  log.Infof("Current user group id: %v", currentUserGroupId)
  if !okGroup || currentUserGroup == nil {
    i, err := strconv.ParseInt(currentUserGroupId, 10, 64)
    currentUserGroup, err = dr.GetRowsByWhereClause("usergroup", squirrel.Eq{"id": i})
    if len(currentUserGroup) < 1 {
      log.Errorf("No group from user group id [%v]", currentUserGroupId)
      return nil, errors.New("failed")
    }
    currentUserGroupId = currentUserGroup[0]["reference_id"].(string)
    if err != nil {
      log.Errorf("Failed to load user group of user [%v]", currentUser)
    }
  }

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

  req.Context.Set("user", currentUser)
  req.Context.Set("usergroup", currentUserGroup)

  return nil, nil

}
