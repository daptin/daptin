package resource

import (
  "github.com/artpar/api2go"
  log "github.com/Sirupsen/logrus"
)

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dr *DbResource) FindOne(referenceId string, req api2go.Request) (api2go.Responder, error) {

  for _, bf := range dr.ms.BeforeFindOne {
    log.Infof("Invoke BeforeFindOne [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
    r, err := bf.InterceptBefore(dr, &req)
    if err != nil {
      log.Errorf("Error from BeforeFindOne middleware: %v", err)
      return nil, err
    }
    if r != nil {
      return r, err
    }
  }

  log.Infof("Find [%s] by id [%s]", dr.model.GetName(), referenceId)

  data, include, err := dr.GetSingleRowByReferenceId(dr.model.GetName(), referenceId)

  for _, bf := range dr.ms.AfterFindOne {
    log.Infof("Invoke AfterFindOne [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

    results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{data})
    if len(results) != 0 {
      data = results[0]
    } else {
      data = nil
    }
    if err != nil {
      log.Errorf("Error from AfterFindOne middleware: %v", err)
    }
    include, err = bf.InterceptAfter(dr, &req, include)

    if err != nil {
      log.Errorf("Error from AfterFindOne middleware: %v", err)
    }
  }

  delete(data, "id")
  delete(data, "deleted_at")

  infos := dr.model.GetColumns()
  var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
  a.Data = data

  for _, inc := range include {
    p, ok := inc["permission"].(int64)
    if !ok {
      log.Errorf("Failed to convert [%v] to permission: %v", ok)
      continue
    }
    a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(inc["__type"].(string), nil, int(p), nil, inc))
  }

  return NewResponse(nil, a, 200, nil), err
}
