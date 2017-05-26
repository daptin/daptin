package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
  //sq "gopkg.in/Masterminds/squirrel.v1"
  //"reflect"
  //"github.com/satori/go.uuid"
  "time"
  "gopkg.in/Masterminds/squirrel.v1"
)

// Delete an object
// Possible Responder status codes are:
// - 200 OK: Deletion was a success, returns meta information, currently not implemented! Do not use this
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Deletion was successful, return nothing


func (dr *DbResource) Delete(id string, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeDelete {
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from before delete middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }

  m := dr.model
  //log.Infof("Get all resource type: %v\n", m)

  queryBuilder := squirrel.Update(m.GetTableName()).Set("deleted_at", time.Now()).Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil})

  sql1, args, err := queryBuilder.ToSql()
  if err != nil {
    log.Infof("Error: %v", err)
    return nil, err
  }

  //log.Infof("Sql: %v\n", sql1)

  _, err = dr.db.Exec(sql1, args...)

  for _, bf := range dr.ms.AfterDelete {
    _, err = bf.InterceptAfter(dr, &req, nil)
    if err != nil {
      log.Errorf("Error from after create middleware: %v", err)
    }
  }

  if err != nil {
    log.Infof("Error: %v", err)
    return nil, err
  }

  return NewResponse(nil, nil, 200, nil), nil
}

