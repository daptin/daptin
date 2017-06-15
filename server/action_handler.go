package server

import (
  "encoding/base64"
  "encoding/json"
  "errors"
  "fmt"
  log "github.com/sirupsen/logrus"
  "github.com/artpar/api2go"
  "github.com/artpar/goms/server/auth"
  "github.com/artpar/goms/server/resource"
  "github.com/dgrijalva/jwt-go"
  "github.com/gorilla/context"
  "github.com/dop251/goja"
  "github.com/satori/go.uuid"
  "gopkg.in/Masterminds/squirrel.v1"
  "gopkg.in/gin-gonic/gin.v1"
  //"io"
  "io/ioutil"
  "net/http"
  "os"
  "strings"
  "syscall"
  "time"
  "crypto/md5"
  "encoding/hex"
)

var guestActions = map[string]resource.Action{}

func CreateGuestActionListHandler(initConfig *CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {

  actionMap := make(map[string]resource.Action)

  for _, ac := range initConfig.Actions {
    actionMap[ac.OnType+":"+ac.Name] = ac
  }

  guestActions["user:signup"] = actionMap["user:signup"]
  guestActions["user:signin"] = actionMap["user:signin"]

  return func(c *gin.Context) {

    c.JSON(200, guestActions)
  }
}

func CreateActionEventHandler(initConfig *CmsConfig, configStore *ConfigStore, cruds map[string]*resource.DbResource) func(*gin.Context) {

  actionMap := make(map[string]resource.Action)

  for _, ac := range initConfig.Actions {
    actionMap[ac.OnType+":"+ac.Name] = ac
  }

  return func(ginContext *gin.Context) {

    actionName := ginContext.Param("actionName")

    bytes, err := ioutil.ReadAll(ginContext.Request.Body)
    if err != nil {
      ginContext.Error(err)
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
    userid := context.Get(ginContext.Request, "user_id")
    var userReferenceId string
    userGroupReferenceIds := make([]auth.GroupPermission, 0)
    if userid != nil {
      userReferenceId = userid.(string)
      userGroupReferenceIds = context.Get(ginContext.Request, "usergroup_id").([]auth.GroupPermission)
    }

    var subjectInstance *api2go.Api2GoModel
    var subjectInstanceMap map[string]interface{}

    subjectInstanceReferenceId, ok := actionRequest.Attributes[actionRequest.Type+"_id"]
    if ok {
      referencedObject, err := cruds[actionRequest.Type].FindOne(subjectInstanceReferenceId.(string), req)
      if err != nil {
        ginContext.AbortWithError(400, err)
        return
      }
      subjectInstance = referencedObject.Result().(*api2go.Api2GoModel)

      subjectInstanceMap = subjectInstance.Data
      subjectInstanceMap["__type"] = subjectInstance.GetName()
      permission := cruds[actionRequest.Type].GetRowPermission(subjectInstanceMap)

      if !permission.CanExecute(userReferenceId, userGroupReferenceIds) {
        ginContext.AbortWithError(403, errors.New("Forbidden"))
        return
      }
    }

    if !cruds["world"].IsUserActionAllowed(userReferenceId, userGroupReferenceIds, actionRequest.Type, actionRequest.Action) {
      ginContext.AbortWithError(403, errors.New("Forbidden"))
      return
    }

    log.Infof("Handle event for [%v]", actionName)

    action, err := cruds["action"].GetActionByName(actionRequest.Type, actionRequest.Action)

    if err != nil {
      ginContext.AbortWithError(400, err)
      return
    }

    inFieldMap, err := GetValidatedInFields(actionRequest, action)

    if err != nil {
      ginContext.AbortWithError(400, err)
      return
    }

    if userReferenceId != "" {
      user, err := cruds["user"].GetReferenceIdToObject("user", userReferenceId)
      if err != nil {
        log.Errorf("Failed to load user: %v", err)
        return
      }
      inFieldMap["user"] = user
    }

    if subjectInstanceMap != nil {
      inFieldMap[actionRequest.Type+"_id"] = subjectInstanceMap["reference_id"]
      inFieldMap["subject"] = subjectInstanceMap
    }

    responses := make([]ActionResponse, 0)

    var res api2go.Responder
    for _, outcome := range action.OutFields {

      var actionResponse ActionResponse

      model, request, err := BuildOutcome(inFieldMap, outcome)
      if err != nil {
        log.Errorf("Failed to build outcome: %v", err)
        responses = append(responses, NewActionResponse("error", "Failed to build outcome "+outcome.Type))
        continue
      }

      context.Set(request.PlainRequest, "user_id", context.Get(ginContext.Request, "user_id"))
      context.Set(request.PlainRequest, "user_id_integer", context.Get(ginContext.Request, "user_id_integer"))
      context.Set(request.PlainRequest, "usergroup_id", context.Get(ginContext.Request, "usergroup_id"))

      dbResource, ok := cruds[outcome.Type]
      if !ok {
        //log.Errorf("No DbResource for type [%v]", outcome.Type)
      }

      switch outcome.Method {
      case "POST":
        res, err = dbResource.Create(model, request)
        if err != nil {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to create "+model.GetName()))
        } else {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("success", "Created "+model.GetName()))
        }
        responses = append(responses, actionResponse)
        break
      case "UPDATE":
        res, err = dbResource.Update(model, request)
        if err != nil {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to update "+model.GetName()))
        } else {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("success", "Created "+model.GetName()))
        }
        responses = append(responses, actionResponse)
        break
      case "DELETE":
        res, err = dbResource.Delete(model.Data["reference_id"].(string), request)
        if err != nil {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to delete "+model.GetName()))
        } else {
          actionResponse = NewActionResponse("client.notify", NewClientNotification("success", "Created "+model.GetName()))
        }
        responses = append(responses, actionResponse)
        break
      case "EXECUTE":
        //res, err = cruds[outcome.Type].Create(model, request)

        if model.GetName() == "__restart" {

          restartAttrs := make(map[string]interface{})
          restartAttrs["type"] = "success"
          restartAttrs["message"] = "Initiating system update."
          actionResponse = NewActionResponse("client.notify", restartAttrs)
          responses = append(responses, actionResponse)

          // new response
          restartAttrs = make(map[string]interface{})
          restartAttrs["location"] = "/"
          restartAttrs["window"] = "self"
          restartAttrs["delay"] = 5000
          actionResponse = NewActionResponse("client.redirect", restartAttrs)
          responses = append(responses, actionResponse)

          go restart()
        } else if model.GetName() == "__download_init_config" {

          js, err := json.Marshal(*initConfig)
          if err != nil {
            log.Errorf("Failed to marshal initconfig: %v", err)
            return
          }

          responseAttrs := make(map[string]interface{})
          responseAttrs["content"] = base64.StdEncoding.EncodeToString(js)
          responseAttrs["name"] = "schema.json"
          responseAttrs["contentType"] = "application/json"
          responseAttrs["message"] = "Downloading system schema"
          actionResponse = NewActionResponse("client.file.download", responseAttrs)
          responses = append(responses, actionResponse)

          //io.Copy(ginContext.Writer, strings.NewReader(string()))

          //ginContext.JSON(200, *initConfig)
        } else if model.GetName() == "__become_admin" {

          if !cruds["world"].CanBecomeAdmin() {
            ginContext.AbortWithStatus(400)
            return
          }
          user := inFieldMap["user"].(map[string]interface{})

          responseAttrs := make(map[string]interface{})

          if cruds["world"].BecomeAdmin(user["id"].(int64)) {
            responseAttrs["location"] = "/"
            responseAttrs["window"] = "self"
            responseAttrs["delay"] = 0
          }

          actionResponse = NewActionResponse("client.redirect", responseAttrs)
          responses = append(responses, actionResponse)

        } else if model.GetName() == "generate.jwt.token" {

          email := inFieldMap["email"]
          password := inFieldMap["password"]

          existingUsers, _, err := cruds["user"].GetRowsByWhereClause("user", squirrel.Eq{"email": email})

          responseAttrs := make(map[string]interface{})
          if err != nil || len(existingUsers) < 1 {
            responseAttrs["type"] = "error"
            responseAttrs["message"] = "Invalid username or password"
            actionResponse = NewActionResponse("client.notify", responseAttrs)
            responses = append(responses, actionResponse)
          } else {
            existingUser := existingUsers[0]
            if resource.BcryptCheckStringHash(password.(string), existingUser["password"].(string)) {

              // Create a new token object, specifying signing method and the claims
              // you would like it to contain.
              token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
                "email":   existingUser["email"],
                "name":    existingUser["name"],
                "nbf":     time.Now().Unix(),
                "exp":     time.Now().Add(60 * time.Minute).Unix(),
                "iss":     "goms",
                "picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", GetMD5Hash(strings.ToLower(existingUser["email"].(string)))),
                "iat":     time.Now(),
                "jti":     uuid.NewV4().String(),
              })

              // Sign and get the complete encoded token as a string using the secret
              secret, err := configStore.GetConfigValueFor("jwt.secret", "backend")
              if err != nil {
                newSecret := uuid.NewV4().String()
                configStore.SetConfigValueFor("jwt.secret", newSecret, "backend")
                secret = newSecret
              }
              tokenString, err := token.SignedString([]byte(secret))
              fmt.Printf("%v %v", tokenString, err)
              if err != nil {
                log.Errorf("Failed to sign string: %v", err)
                break
              }

              responseAttrs = make(map[string]interface{})
              responseAttrs["value"] = string(tokenString)
              responseAttrs["key"] = "token"
              actionResponse = NewActionResponse("client.store.set", responseAttrs)
              responses = append(responses, actionResponse)

              notificationAttrs := make(map[string]string)
              notificationAttrs["message"] = "Logged in"
              notificationAttrs["type"] = "success"
              responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

              responseAttrs = make(map[string]interface{})
              responseAttrs["location"] = "/"
              responseAttrs["window"] = "self"
              responseAttrs["delay"] = 2000

              responses = append(responses, NewActionResponse("client.redirect", responseAttrs))

            } else {
              responseAttrs = make(map[string]interface{})
              responseAttrs["type"] = "error"
              responseAttrs["message"] = "Invalid username or password"
              responses = append(responses, NewActionResponse("client.notify", responseAttrs))

            }

          }

        }

        break

      default:
        log.Errorf("Invalid outcome method: %v", outcome.Method)
        ginContext.AbortWithError(500, errors.New("Invalid outcome"))
        return
      }
      if res != nil && res.Result() != nil {
        inFieldMap[outcome.Reference] = res.Result().(*api2go.Api2GoModel).Data
      }

      if err != nil {
        ginContext.AbortWithError(500, err)
        return
      }
    }

    ginContext.JSON(200, responses)

  }
}
func NewClientNotification(notificationType string, message string) map[string]interface{} {

  m := make(map[string]interface{})

  m["type"] = notificationType
  m["message"] = message
  return m

}

func GetMD5Hash(text string) string {
  hasher := md5.New()
  hasher.Write([]byte(text))
  return hex.EncodeToString(hasher.Sum(nil))
}

type ActionResponse struct {
  ResponseType string
  Attributes   interface{}
}

func NewActionResponse(responseType string, attrs interface{}) ActionResponse {

  ar := ActionResponse{
    ResponseType: responseType,
    Attributes:   attrs,
  }

  return ar

}

func restart() {
  log.Infof("Sleeping for 3 seconds before restart")
  time.Sleep(100 * time.Millisecond)
  log.Infof("Kill")
  syscall.Kill(os.Getpid(), syscall.SIGUSR2)

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
    break

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
  case "jwt.token":

    respopnseModel := api2go.NewApi2GoModel("generate.jwt.token", nil, 0, nil)
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

var halt = errors.New("Stahp")

func runUnsafe(unsafe string, contextMap map[string]interface{}) string {

  vm := goja.New()

  //vm.ToValue(contextMap)
  for key, val := range contextMap {
    vm.Set(key, val)
  }
  v, err := vm.RunString(unsafe) // Here be dragons (risky code)
  if err != nil {
    log.Errorf("failed to execute: %v", err)
  }

  return v.String()
}

func buildActionContext(outcome resource.Outcome, inFieldMap map[string]interface{}) map[string]interface{} {

  data := make(map[string]interface{})
  for key, field := range outcome.Attributes {

    if field[0] == '$' {

      fieldParts := strings.Split(field[1:], ".")

      if fieldParts[0] == "" {
        fieldParts[0] = "subject"
      }

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
    } else if field[0] == '!' {

      res := runUnsafe(field[1:], inFieldMap)
      data[key] = res

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
