package server

import (
  "github.com/artpar/api2go"
  "gopkg.in/gin-gonic/gin.v1"
  log "github.com/Sirupsen/logrus"
  "github.com/artpar/goms/server/resource"
  "io/ioutil"
  "encoding/json"
  "github.com/artpar/goms/server/auth"
  "strings"
  "fmt"
  "time"
  "os"
  "syscall"
  "github.com/gorilla/context"
  "net/http"
  "errors"
  "encoding/base64"
)

func CreateActionEventHandler(initConfig *CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {

  actionMap := make(map[string]resource.Action)

  for _, ac := range initConfig.Actions {
    actionMap[ac.OnType+":"+ac.Name] = ac
  }

  return func(c *gin.Context) {

    onEntity := c.Param("actionName")

    bytes, err := ioutil.ReadAll(c.Request.Body)
    if err != nil {
      c.Error(err)
      return
    }

    actionRequest := resource.ActionRequest{}
    json.Unmarshal(bytes, &actionRequest)

    if isSystemAction(actionRequest.Action) {
      err = handleSystemAction(actionRequest, cruds)
      if err != nil {
        log.Errorf("Failed to complete system action: %v", err)
        c.AbortWithError(400, err)
        return
      }
      c.AbortWithStatus(200);
      return

    }

    log.Infof("Request body: %v", actionRequest)

    userReferenceId := context.Get(c.Request, "user_id").(string)
    userGroupReferenceIds := context.Get(c.Request, "usergroup_id").([]auth.GroupPermission)

    req := api2go.Request{
      PlainRequest: &http.Request{
        Method: "GET",
      },
    }

    referencedObject, err := cruds[actionRequest.Type].FindOne(actionRequest.Attributes[actionRequest.Type+"_id"].(string), req)

    if err != nil {
      c.Error(err)
      return
    }

    goModel := referencedObject.Result().(*api2go.Api2GoModel)
    obj := goModel.Data
    obj["__type"] = goModel.GetName()
    permission := cruds[actionRequest.Type].GetRowPermission(obj)

    if !permission.CanExecute(userReferenceId, userGroupReferenceIds) {
      c.AbortWithError(403, errors.New("Forbidden"))
      return
    }

    if !cruds["world"].IsUserActionAllowed(userReferenceId, userGroupReferenceIds, actionRequest.Type, actionRequest.Action) {
      c.AbortWithError(403, errors.New("Forbidden"))
      return
    }

    log.Infof("Handle event for [%v]", onEntity)

    action, err := cruds["action"].GetActionByName(actionRequest.Type, actionRequest.Action)

    if err != nil {
      c.AbortWithError(400, err)
      return
    }

    inFieldMap, err := GetValidatedInFields(actionRequest, action, obj)

    if err != nil {
      c.AbortWithError(400, err)
      return
    }

    inFieldMap[""] = obj

    var res api2go.Responder

    for _, outcome := range action.OutFields {

      req, model := BuildOutcome(action.OnType, inFieldMap, outcome)

      context.Set(model.PlainRequest, "user_id", context.Get(c.Request, "user_id"))
      context.Set(model.PlainRequest, "user_id_integer", context.Get(c.Request, "user_id_integer"))
      context.Set(model.PlainRequest, "usergroup_id", context.Get(c.Request, "usergroup_id"))

      dbResource, ok := cruds[outcome.Type]
      if !ok {
        log.Errorf("No DbResource for type [%v]", outcome.Type)
        continue
      }

      switch outcome.Method {
      case "POST":
        res, err = dbResource.Create(req, model)
        break
      case "UPDATE":
        res, err = dbResource.Update(req, model)
        break
      case "DELETE":
        res, err = dbResource.Delete(req.Data["reference_id"].(string), model)
        break
      case "EXECUTE":
        //res, err = cruds[outcome.Type].Create(req, model)
        break

      default:
        log.Errorf("Invalid outcome method: %v", outcome.Method)
        c.AbortWithError(500, errors.New("Invalid outcome"))
        return
      }
      inFieldMap[outcome.Reference] = res.Result().(*api2go.Api2GoModel).Data

    }

    if err != nil {
      c.AbortWithError(500, err)
      return
    }

    c.JSON(200, res)

  }
}

func handleSystemAction(request resource.ActionRequest, cruds map[string]*resource.DbResource) error {

  log.Infof("Handle system action: %v", request.Action)

  action, err := cruds["action"].GetActionByName(request.Type, request.Action)
  if err != nil {
    return err
  }

  attrs := request.Attributes

  switch action.Name {
  case "upload_system_schema":
    files1, ok := attrs["schema_json_file"]
    log.Infof("Files [%v]: %v", attrs, files1)
    files := files1.([]interface{})
    if !ok || len(files) < 1 {
      return errors.New("No files uploaded")
    }
    for _, file := range files {
      f := file.(map[string]interface{})
      fileName := f["name"].(string)
      log.Infof("File name: %v", fileName)
      fileContentsBase64 := f["file"].(string)
      fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
      if err != nil {
        return err
      }

      err = ioutil.WriteFile(fmt.Sprintf("schema_%v_gocms.json", fileName), fileBytes, 0644)
      if err != nil {
        return err
      }

    }

    log.Infof("Written all json files. Attempting restart")

    go restart()
    break;
  }

  return nil

}

func restart() {
  log.Infof("Sleeping for 3 seconds before restart")
  time.Sleep(3 * time.Second)
  log.Infof("Kill")
  //workingDirectory, err := os.Getwd()
  //if err != nil {
  //  log.Errorf("Failed to get working directory: %v", err)
  //}
  //attrs := os.ProcAttr{
  //  Dir: workingDirectory,
  //
  //}
  //os.StartProcess("kill", []string{"-SIGUSR2", fmt.Sprintf("%v", syscall.Getppid())}, &attrs)
  syscall.Kill(os.Getpid(), syscall.SIGUSR2)
  //goagain.Kill()
}

func isSystemAction(actionName string) bool {

  switch actionName {
  case "upload_system_schema":
    return true
  }
  return false
}

func BuildOutcome(onType string, inFieldMap map[string]interface{}, outcome resource.Outcome) (*api2go.Api2GoModel, api2go.Request) {

  data := make(map[string]interface{})

  for key, field := range outcome.Attributes {

    if field[0] == '$' {

      fieldParts := strings.Split(field[1:], ".")

      var finalValue interface{}

      finalValue = inFieldMap
      for i := 0; i < len(fieldParts)-1; i++ {
        fieldPart := fieldParts[i]
        finalValue = finalValue.(map[string]interface{})[fieldPart]
      }
      finalValue = finalValue.(map[string]interface{})[fieldParts[len(fieldParts)-1]]
      data[key] = finalValue
    } else {
      data[key] = inFieldMap[field]
    }

  }

  model := api2go.NewApi2GoModelWithData(outcome.Type, nil, 755, nil, data)

  req := api2go.Request{
    PlainRequest: &http.Request{
      Method: "POST",
    },
  }

  return model, req

}

func GetValidatedInFields(actionRequest resource.ActionRequest, action resource.Action, referencedObject map[string]interface{}) (map[string]interface{}, error) {

  dataMap := actionRequest.Attributes
  finalDataMap := make(map[string]interface{})
  for _, inField := range action.InFields {
    val, ok := dataMap[inField.ColumnName]
    if ok {
      finalDataMap[inField.ColumnName] = val
    } else if inField.DefaultValue != "" {

    } else {
      return nil, errors.New(fmt.Sprintf("Field %s cannot be blank", inField.Name))
    }
  }

  finalDataMap[actionRequest.Type+"_id"] = referencedObject["reference_id"]
  return finalDataMap, nil
}
