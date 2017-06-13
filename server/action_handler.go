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
  //"github.com/fatih/structs"
  "syscall"
  "github.com/gorilla/context"
  "net/http"
  "errors"
  "encoding/base64"
  "io"
)

func CreateActionEventHandler(initConfig *CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {

  actionMap := make(map[string]resource.Action)

  for _, ac := range initConfig.Actions {
    actionMap[ac.OnType+":"+ac.Name] = ac
  }

  return func(c *gin.Context) {

    actionName := c.Param("actionName")

    bytes, err := ioutil.ReadAll(c.Request.Body)
    if err != nil {
      c.Error(err)
      return
    }

    actionRequest := resource.ActionRequest{}
    json.Unmarshal(bytes, &actionRequest)

    //log.Infof("Request body: %v", actionRequest)

    req := api2go.Request{
      PlainRequest: &http.Request{
        Method: "GET",
      },
    }
    userReferenceId := context.Get(c.Request, "user_id").(string)
    userGroupReferenceIds := context.Get(c.Request, "usergroup_id").([]auth.GroupPermission)

    var subjectInstance *api2go.Api2GoModel
    var subjectInstanceMap map[string]interface{}

    subjectInstanceReferenceId, ok := actionRequest.Attributes[actionRequest.Type+"_id"]
    if ok {
      referencedObject, err := cruds[actionRequest.Type].FindOne(subjectInstanceReferenceId.(string), req)
      if err != nil {
        c.AbortWithError(400, err)
        return
      }
      subjectInstance = referencedObject.Result().(*api2go.Api2GoModel)

      subjectInstanceMap = subjectInstance.Data
      subjectInstanceMap["__type"] = subjectInstance.GetName()
      permission := cruds[actionRequest.Type].GetRowPermission(subjectInstanceMap)

      if !permission.CanExecute(userReferenceId, userGroupReferenceIds) {
        c.AbortWithError(403, errors.New("Forbidden"))
        return
      }
    }

    if !cruds["world"].IsUserActionAllowed(userReferenceId, userGroupReferenceIds, actionRequest.Type, actionRequest.Action) {
      c.AbortWithError(403, errors.New("Forbidden"))
      return
    }

    log.Infof("Handle event for [%v]", actionName)

    action, err := cruds["action"].GetActionByName(actionRequest.Type, actionRequest.Action)

    if err != nil {
      c.AbortWithError(400, err)
      return
    }

    inFieldMap, err := GetValidatedInFields(actionRequest, action)

    if err != nil {
      c.AbortWithError(400, err)
      return
    }

    user, err := cruds["user"].GetReferenceIdToObject("user", userReferenceId)
    if err != nil {
      log.Errorf("Failed to load user: %v", err)
      return
    }

    if subjectInstanceMap != nil {
      inFieldMap[actionRequest.Type+"_id"] = subjectInstanceMap["reference_id"]
      inFieldMap[""] = subjectInstanceMap
    }

    inFieldMap["user"] = user

    var res api2go.Responder

    for _, outcome := range action.OutFields {

      model, request, err := BuildOutcome(inFieldMap, outcome)
      if err != nil {
        log.Errorf("Failed to build outcome: %v", err)
        continue
      }

      context.Set(request.PlainRequest, "user_id", context.Get(c.Request, "user_id"))
      context.Set(request.PlainRequest, "user_id_integer", context.Get(c.Request, "user_id_integer"))
      context.Set(request.PlainRequest, "usergroup_id", context.Get(c.Request, "usergroup_id"))

      dbResource, ok := cruds[outcome.Type]
      if !ok {
        //log.Errorf("No DbResource for type [%v]", outcome.Type)
      }

      switch outcome.Method {
      case "POST":
        res, err = dbResource.Create(model, request)
        break
      case "UPDATE":
        res, err = dbResource.Update(model, request)
        break
      case "DELETE":
        res, err = dbResource.Delete(model.Data["reference_id"].(string), request)
        break
      case "EXECUTE":
        //res, err = cruds[outcome.Type].Create(model, request)

        if model.GetName() == "__restart" {
          go restart()
        } else if model.GetName() == "__download_init_config" {

          c.Header("Content-Disposition", "attachment; filename=schema.json")
          c.Header("Content-Type", "text/json;charset=utf-8")

          js, err := json.Marshal(*initConfig)
          if err != nil {
            log.Errorf("Failed to marshal initconfig: %v", err)
            return
          }
          io.Copy(c.Writer, strings.NewReader(string(js)))

          //c.JSON(200, *initConfig)
          return
        } else if model.GetName() == "__become_admin" {

          if !cruds["world"].CanBecomeAdmin() {
            c.AbortWithStatus(400)
            return
          }

          cruds["world"].BecomeAdmin(user["id"].(int64))

        }

        break

      default:
        log.Errorf("Invalid outcome method: %v", outcome.Method)
        c.AbortWithError(500, errors.New("Invalid outcome"))
        return
      }
      if res != nil && res.Result() != nil {
        inFieldMap[outcome.Reference] = res.Result().(*api2go.Api2GoModel).Data
      }

    }

    if err != nil {
      c.AbortWithError(500, err)
      return
    }

    c.JSON(200, res)

  }
}

func restart() {
  log.Infof("Sleeping for 3 seconds before restart")
  time.Sleep(100 * time.Millisecond)
  log.Infof("Kill")
  syscall.Kill(os.Getpid(), syscall.SIGUSR2)

}

func isSystemAction(actionName string) bool {

  switch actionName {
  case "upload_system_schema":
    return true
  case "down_system_schema":
    return true
  case "become_admin":
    return true
  }
  return false
}

func BuildOutcome(inFieldMap map[string]interface{}, outcome resource.Outcome) (*api2go.Api2GoModel, api2go.Request, error) {

  attrs := buildActionContext(outcome, inFieldMap)

  switch outcome.Type {
  case "system_json_schema_update":
    responseModel := api2go.NewApi2GoModel("__restart", nil, 0, nil)
    returnRequest := api2go.Request{
      PlainRequest: &http.Request{
        Method: "EXECUTE",
      },
    }

    files1, ok := attrs["json_schema"]
    log.Infof("Files [%v]: %v", attrs, files1)
    files := files1.([]interface{})
    if !ok || len(files) < 1 {
      return nil, returnRequest, errors.New("No files uploaded")
    }
    for _, file := range files {
      f := file.(map[string]interface{})
      fileName := f["name"].(string)
      log.Infof("File name: %v", fileName)
      fileContentsBase64 := f["file"].(string)
      fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
      if err != nil {
        return nil, returnRequest, err
      }

      jsonFileName := fmt.Sprintf("schema_%v_gocms.json", fileName)
      err = ioutil.WriteFile(jsonFileName, fileBytes, 0644)
      if err != nil {
        log.Errorf("Failed to write json file: %v", jsonFileName)
        return nil, returnRequest, err
      }

    }

    log.Infof("Written all json files. Attempting restart")

    return responseModel, returnRequest, nil
    break;

  case "system_json_schema_download":

    respopnseModel := api2go.NewApi2GoModel("__download_init_config", nil, 0, nil)
    returnRequest := api2go.Request{
      PlainRequest: &http.Request{
        Method: "EXECUTE",
      },
    }

    return respopnseModel, returnRequest, nil

    break
  case "become_admin":

    respopnseModel := api2go.NewApi2GoModel("__become_admin", nil, 0, nil)
    returnRequest := api2go.Request{
      PlainRequest: &http.Request{
        Method: "EXECUTE",
      },
    }

    return respopnseModel, returnRequest, nil

    break

  default:

    model := api2go.NewApi2GoModelWithData(outcome.Type, nil, auth.DEFAULT_PERMISSION, nil, attrs)

    req := api2go.Request{
      PlainRequest: &http.Request{
        Method: "POST",
      },
    }
    return model, req, nil

  }

  return nil, api2go.Request{}, errors.New(fmt.Sprintf("Unidentified outcome: %v", outcome.Type))

}

func buildActionContext(outcome resource.Outcome, inFieldMap map[string]interface{}) (map[string]interface{}) {

  data := make(map[string]interface{})
  for key, field := range outcome.Attributes {

    if field[0] == '$' {

      fieldParts := strings.Split(field[1:], ".")

      var finalValue interface{}

      // it looks confusing but it does whats its supposed to do
      // todo: add helpful comment

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
  return data
}

func GetValidatedInFields(actionRequest resource.ActionRequest, action resource.Action) (map[string]interface{}, error) {

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

  return finalDataMap, nil
}
