package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
	"os"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	//"io"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"io"
	"net/url"
	"strconv"

	"github.com/artpar/conform"
	"gopkg.in/go-playground/validator.v9"
)

var guestActions = map[string]actionresponse.Action{}

func CreateGuestActionListHandler(initConfig *CmsConfig) func(*gin.Context) {

	actionMap := make(map[string]actionresponse.Action)

	for _, ac := range initConfig.Actions {
		actionMap[ac.OnType+":"+ac.Name] = ac
	}

	guestActions["user:signup"] = actionMap["user_account:signup"]
	guestActions["user:signin"] = actionMap["user_account:signin"]

	return func(c *gin.Context) {

		c.JSON(200, guestActions)
	}
}

type DaptinError struct {
	Message string
	Code    string
}

func (de *DaptinError) Error() string {
	return de.Message
}

func NewDaptinError(str string, code string) *DaptinError {
	return &DaptinError{
		Message: str,
		Code:    code,
	}
}

func CreatePostActionHandler(initConfig *CmsConfig,
	cruds map[string]*DbResource, actionPerformers []actionresponse.ActionPerformerInterface) func(*gin.Context) {

	actionMap := make(map[string]actionresponse.Action)

	for _, ac := range initConfig.Actions {
		actionMap[ac.OnType+":"+ac.Name] = ac
	}

	actionHandlerMap := make(map[string]actionresponse.ActionPerformerInterface)

	for _, actionPerformer := range actionPerformers {
		if actionPerformer == nil {
			continue
		}
		actionHandlerMap[actionPerformer.Name()] = actionPerformer
	}

	return func(ginContext *gin.Context) {

		actionName := ginContext.Param("actionName")
		actionType := ginContext.Param("typename")

		actionRequest, err := BuildActionRequest(ginContext.Request.Body, actionType, actionName,
			ginContext.Params, ginContext.Request.URL.Query())

		if err != nil {
			ginContext.Error(err)
			return
		}
		//log.Printf("Action Request body: %v", actionRequest)

		req := api2go.Request{
			PlainRequest: &http.Request{
				Method: "POST",
				URL:    ginContext.Request.URL,
				Header: ginContext.Request.Header,
			},
		}

		req.PlainRequest = req.PlainRequest.WithContext(ginContext.Request.Context())

		actionCrudResource, ok := cruds[actionType]
		if !ok {
			actionCrudResource = cruds["world"]
		}

		transaction, err := cruds["world"].Connection().Beginx()
		if err != nil {
			CheckErr(err, "Failed to begin transaction [121]")
		}

		responses, err := actionCrudResource.HandleActionRequest(actionRequest, req, transaction)
		if err != nil {
			transaction.Rollback()
		} else {
			transaction.Commit()
		}

		responseStatus := 200
		for _, response := range responses {
			if response.ResponseType == "render" {
				attrs := response.Attributes.(map[string]interface{})
				var content = attrs["content"].(string)
				var mimeType = attrs["mime_type"].(string)
				var headers = attrs["headers"].(map[string]string)

				ginContext.Writer.WriteHeader(http.StatusOK)
				ginContext.Writer.Header().Set("Content-Type", mimeType)
				for hKey, hValue := range headers {
					ginContext.Writer.Header().Set(hKey, hValue)
				}
				ginContext.Writer.Flush()

				// Render the rest of the DATA
				ginContext.Writer.WriteString(Atob(content))
				ginContext.Writer.Flush()
				return
			}
			if response.ResponseType == "client.header.set" {
				attrs := response.Attributes.(map[string]string)

				for key, value := range attrs {
					if strings.ToLower(key) == "status" {
						responseStatusCode, err := strconv.ParseInt(value, 10, 32)
						if err != nil {
							log.Errorf("invalid status code value set in response: %v", value)
						} else {
							responseStatus = int(responseStatusCode)
						}
					} else {
						ginContext.Header(key, value)
					}
				}
			}
		}

		if err != nil {
			if httpErr, ok := err.(api2go.HTTPError); ok {
				if len(responses) > 0 {
					ginContext.AbortWithStatusJSON(httpErr.Status(), responses)
				} else {
					ginContext.AbortWithStatusJSON(httpErr.Status(), []actionresponse.ActionResponse{
						{
							ResponseType: "client.notify",
							Attributes: map[string]interface{}{
								"message": err.Error(),
								"title":   "failed",
								"type":    "error",
							},
						},
					})

				}
			} else {
				if len(responses) > 0 {
					ginContext.AbortWithStatusJSON(400, responses)
				} else {
					ginContext.AbortWithStatusJSON(500, []actionresponse.ActionResponse{
						{
							ResponseType: "client.notify",
							Attributes: map[string]interface{}{
								"message": err.Error(),
								"title":   "failed",
								"type":    "error",
							},
						},
					})
				}

			}
			return
		}

		//log.Printf("Final responses: %v", responses)

		ginContext.JSON(responseStatus, responses)

	}
}

func (dbResource *DbResource) HandleActionRequest(actionRequest actionresponse.ActionRequest,
	req api2go.Request, transaction *sqlx.Tx) ([]actionresponse.ActionResponse, error) {

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	var err error
	//adminUserGroupIds, err := dbResource.GetIdByWhereClause("usergroup", transaction, goqu.Ex{
	//	"name": "administrators",
	//})
	//adminUsergroupId := adminUserGroupIds[0]
	//
	var subjectInstance api2go.Api2GoModel
	var subjectInstanceMap map[string]interface{}

	action, err := dbResource.GetActionByName(actionRequest.Type, actionRequest.Action, transaction)
	CheckErr(err, "Failed to get action by Type/action [%v][%v]", actionRequest.Type, actionRequest.Action)
	if err != nil {
		log.Warnf("invalid action: %v - %v", actionRequest.Action, actionRequest.Type)
		//CheckErr(rollbackErr, "failed to rollback")
		return nil, api2go.NewHTTPError(err, "no such action", 400)
	}

	isAdmin := IsAdminWithTransaction(sessionUser, transaction)

	subjectInstanceReferenceString, _ := actionRequest.Attributes[actionRequest.Type+"_id"]

	subjectInstanceReferenceUuid := daptinid.InterfaceToDIR(subjectInstanceReferenceString)

	if subjectInstanceReferenceUuid != daptinid.NullReferenceId {
		req.PlainRequest.Method = "GET"
		req.QueryParams = make(map[string][]string)
		req.QueryParams["included_relations"] = action.RequestSubjectRelations
		referencedObject, err := dbResource.FindOneWithTransaction(subjectInstanceReferenceUuid, req, transaction)
		if err != nil {
			log.Warnf("failed to load subject for action: %v - [%v][%v]", actionRequest.Action, actionRequest.Type, subjectInstanceReferenceString)
			return nil, api2go.NewHTTPError(err, "failed to load subject", 400)
		}
		subjectInstance = referencedObject.Result().(api2go.Api2GoModel)

		subjectInstanceMap = subjectInstance.GetAllAsAttributes()
		subjectInstanceMap["reference_id"] = subjectInstance.GetID()

		if subjectInstanceMap == nil {
			log.Warnf("subject is empty: %v - %v", actionRequest.Action, subjectInstanceReferenceString)
			return nil, api2go.NewHTTPError(errors.New("subject not found"), "subject not found", 400)
		}

		subjectInstanceMap["__type"] = subjectInstance.GetName()
		permission := dbResource.GetRowPermissionWithTransaction(subjectInstanceMap, transaction)

		if !permission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
			log.Warnf("user[%v] not allowed action on this object: %v - %v", sessionUser, actionRequest.Action, subjectInstanceReferenceString)
			return nil, api2go.NewHTTPError(errors.New("forbidden"), "forbidden", 403)
		}
	}

	if !isAdmin && !dbResource.IsUserActionAllowedWithTransaction(sessionUser.UserReferenceId, sessionUser.Groups, actionRequest.Type, actionRequest.Action, transaction) {
		log.Warnf("user[%v] not allowed action: %v - %v", sessionUser, actionRequest.Action, subjectInstanceReferenceString)
		return nil, api2go.NewHTTPError(errors.New("forbidden"), "forbidden", 403)
	}

	log.Debugf("Handle event for action [%v] by user [%v]", actionRequest.Action, sessionUser)

	if !action.InstanceOptional && (subjectInstanceReferenceString == "" || subjectInstance.GetID() != subjectInstanceReferenceString) {
		log.Warnf("subject is unidentified: %v - %v", actionRequest.Action, actionRequest.Type)
		return nil, api2go.NewHTTPError(errors.New("required reference id not provided or incorrect"), "no reference id", 400)
	}

	if actionRequest.Attributes == nil {
		actionRequest.Attributes = make(map[string]interface{})
	}

	for _, field := range action.InFields {
		_, ok := actionRequest.Attributes[field.ColumnName]
		if !ok {
			actionRequest.Attributes[field.ColumnName] = req.PlainRequest.Form.Get(field.ColumnName)
		}
	}

	for _, validation := range action.Validations {
		errs := ValidatorInstance.VarWithValue(actionRequest.Attributes[validation.ColumnName], actionRequest.Attributes, validation.Tags)
		if errs != nil {
			log.Warnf("validation on input fields failed: %v - %v", actionRequest.Action, actionRequest.Type)
			validationErrors := errs.(validator.ValidationErrors)
			firstError := validationErrors[0]
			return nil, api2go.NewHTTPError(errors.New(fmt.Sprintf("invalid value for %s", validation.ColumnName)), firstError.Tag(), 400)
		}
	}

	for _, conformations := range action.Conformations {

		val, ok := actionRequest.Attributes[conformations.ColumnName]
		if !ok {
			continue
		}
		valStr, ok := val.(string)
		if !ok {
			continue
		}
		newVal := conform.TransformString(valStr, conformations.Tags)
		actionRequest.Attributes[conformations.ColumnName] = newVal
	}

	inFieldMap, err := GetValidatedInFields(actionRequest, action)
	if err != nil {
		log.Errorf("Action Input Validation Failed: [%v]", err)
		return nil, err
	}
	inFieldMap["httpRequest"] = req.PlainRequest
	inFieldMap["httpRequestHeaders"] = map[string][]string(req.PlainRequest.Header)
	inFieldMap["attributes"] = actionRequest.Attributes
	inFieldMap["env"] = dbResource.envMap
	inFieldMap["__url"] = req.PlainRequest.URL.String()
	inFieldMap["rawBodyString"] = actionRequest.RawBodyString
	inFieldMap["rawBodyBytes"] = actionRequest.RawBodyBytes
	inFieldMap["encryptionSecret"] = dbResource.EncryptionSecret

	if sessionUser.UserReferenceId != daptinid.NullReferenceId {
		user, err := dbResource.GetReferenceIdToObjectWithTransaction(USER_ACCOUNT_TABLE_NAME, sessionUser.UserReferenceId, transaction)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "failed to identify user", 401)
		}
		inFieldMap["user"] = user
	}

	if subjectInstanceMap != nil {
		inFieldMap[actionRequest.Type+"_id"] = subjectInstanceMap["reference_id"]
		inFieldMap["subject"] = subjectInstanceMap
	}

	responses := make([]actionresponse.ActionResponse, 0)

	sessionUser.Groups = append(sessionUser.Groups, auth.GroupPermission{
		GroupReferenceId: dbResource.AdministratorGroupId,
	})

OutFields:
	for _, outcome := range action.OutFields {
		var responseObjects interface{}
		responseObjects = nil
		var responses1 []actionresponse.ActionResponse
		var errors1 []error
		var actionResponse actionresponse.ActionResponse

		log.Debugf("Action [%v][%v] => Outcome [%v][%v] ", actionRequest.Action, subjectInstanceReferenceString, outcome.Type, outcome.Method)

		if len(outcome.Condition) > 0 {
			var outcomeResult interface{}
			outcomeResult, err = EvaluateString(outcome.Condition, inFieldMap)
			CheckErr(err, "[%s][%s]Failed to evaluate condition, assuming false by default", action.OnType, action.Name)
			if err != nil {
				continue
			}

			log.Tracef("Evaluated condition [%v] result: %v", outcome.Condition, outcomeResult)
			boolValue, ok := outcomeResult.(bool)
			if !ok {

				strVal := fmt.Sprintf("%v", outcomeResult)
				if strVal == "1" || strings.ToLower(strings.TrimSpace(strVal)) == "true" {
					log.Tracef("Condition is true [%s]", outcome.Condition)
					// condition is true
				} else {
					// condition isn't true
					log.Tracef("Condition is false, skipping outcome [%s]", outcome.Condition)
					continue
				}

			} else if !boolValue {
				log.Debugf("Outcome [%v][%v] skipped because condition failed [%v]", outcome.Method, outcome.Type, outcome.Condition)
				continue
			}
		}

		var model api2go.Api2GoModel
		var modelPointer *api2go.Api2GoModel
		var request api2go.Request
		modelPointer, request, err = BuildOutcome(inFieldMap, outcome, sessionUser)
		if err != nil {
			log.Errorf("Failed to build outcome: %v on action[%s] in outcome [%v]", err,
				action.Name, outcome)
			responses = append(responses, NewActionResponse("error", "Failed to build outcome "+outcome.Type))
			if outcome.ContinueOnError {
				continue
			} else {
				return []actionresponse.ActionResponse{}, fmt.Errorf("invalid input for %v => %v", outcome.Type, err)
			}
		}
		model = *modelPointer

		requestContext := req.PlainRequest.Context()
		//var adminUserReferenceId daptinid.DaptinReferenceId
		//adminUserReferenceIds := GetAdminReferenceIdWithTransaction(transaction)
		//for id := range adminUserReferenceIds {
		//	adminUserReferenceId = daptinid.DaptinReferenceId(id)
		//	break
		//}
		//
		//if adminUserReferenceId != daptinid.NullReferenceId {
		requestContext = context.WithValue(requestContext, "user", sessionUser)
		//}
		//request.PlainRequest = request.PlainRequest.WithContext(requestContext)

		actionResponses := make([]actionresponse.ActionResponse, 0)
		//log.Printf("Next outcome method: [%v][%v]", outcome.Method, outcome.Type)
		switch outcome.Method {
		case "SWITCH_USER":
			attrs := model.GetAttributes()
			var userIdAsDir daptinid.DaptinReferenceId
			refIdAttr := attrs["user_reference_id"]
			userIdAsDir = daptinid.InterfaceToDIR(refIdAttr)
			if userIdAsDir == daptinid.NullReferenceId {

			}

			user, _, err := dbResource.Cruds["user_account"].GetSingleRowByReferenceIdWithTransaction(
				"user_account", userIdAsDir, nil, transaction)

			if err != nil {

				actionResponse = NewActionResponse("client.notify", NewClientNotification("error",
					"No such user user_reference_id ["+fmt.Sprintf("%v", refIdAttr)+"] "+err.Error(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			}

			userGroups := dbResource.GetObjectUserGroupsByWhereWithTransaction("user_account", transaction, "id", user["id"].(int64))
			userGroups = append(sessionUser.Groups, auth.GroupPermission{
				GroupReferenceId: dbResource.AdministratorGroupId,
			})

			*sessionUser = auth.SessionUser{
				UserId:          user["id"].(int64),
				UserReferenceId: daptinid.InterfaceToDIR(user["reference_id"]),
				Groups:          userGroups,
			}

			updatedCtx := context.WithValue(request.PlainRequest.Context(), "user", sessionUser)
			request.PlainRequest = request.PlainRequest.WithContext(updatedCtx)

		case "POST":
			responseObjects, err = dbResource.Cruds[outcome.Type].CreateWithTransaction(model, request, transaction)
			CheckErr(err, "Failed to post from action")
			if err != nil {

				actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to create "+model.GetName()+". "+err.Error(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			} else {
				createdRow := responseObjects.(api2go.Response).Result().(api2go.Api2GoModel).GetAttributes()
				actionResponse = NewActionResponse(createdRow["__type"].(string), createdRow)
			}
			actionResponses = append(actionResponses, actionResponse)
		case "GET":

			request.QueryParams = make(map[string][]string)

			for k, val := range model.GetAttributes() {
				if k == "query" {
					valStr, isStr := val.(string)
					if isStr {
						request.QueryParams[k] = []string{valStr}
					} else {
						request.QueryParams[k] = []string{ToJson(val)}
					}
				} else {
					request.QueryParams[k] = []string{fmt.Sprintf("%v", val)}
				}
			}

			responseObjects, _, _, _, err = dbResource.Cruds[outcome.Type].PaginatedFindAllWithoutFilters(request, transaction)
			CheckErr(err, "Failed to get inside action")
			if err != nil {
				actionResponse = NewActionResponse("client.notify",
					NewClientNotification("error", "Failed to get "+model.GetName()+". "+err.Error(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			} else {
				actionResponse = NewActionResponse(outcome.Type, responseObjects)
			}
			actionResponses = append(actionResponses, actionResponse)
		case "GET_BY_ID":

			referenceIdString, ok := model.GetAttributes()["reference_id"]
			referenceIdDir := daptinid.InterfaceToDIR(referenceIdString)
			if referenceIdDir == daptinid.NullReferenceId || !ok {
				err = api2go.NewHTTPError(err, "no reference id provided for GET_BY_ID", 400)
				break OutFields
			}
			includedRelations := make(map[string]bool, 0)
			if model.GetAttributes()["included_relations"] != nil {
				//included := req.QueryParams["included_relations"][0]
				//includedRelationsList := strings.Split(included, ",")
				for _, incl := range strings.Split(model.GetAttributes()["included_relations"].(string), ",") {
					includedRelations[incl] = true
				}

			} else {
				includedRelations = nil
			}

			responseObjects, _, err = dbResource.Cruds[outcome.Type].GetSingleRowByReferenceIdWithTransaction(outcome.Type, referenceIdDir, nil, transaction)
			CheckErr(err, "Failed to get by id")

			if err != nil {
				actionResponse = NewActionResponse("client.notify",
					NewClientNotification("error", "Failed to create "+model.GetName()+". "+err.Error(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			} else {
				actionResponse = NewActionResponse(outcome.Type, responseObjects)
			}
			actionResponses = append(actionResponses, actionResponse)
		case "PATCH":
			responseObjects, err = dbResource.Cruds[outcome.Type].UpdateWithTransaction(model, request, transaction)
			CheckErr(err, "Failed to update inside action")
			if err != nil {
				actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to update "+model.GetName()+". "+err.Error(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			} else {
				createdRow := responseObjects.(api2go.Response).Result().(api2go.Api2GoModel).GetAttributes()
				actionResponse = NewActionResponse(createdRow["__type"].(string), createdRow)
			}
			actionResponses = append(actionResponses, actionResponse)
		case "DELETE":
			idString := model.GetID()
			idUUid := uuid.MustParse(idString)
			err = dbResource.Cruds[outcome.Type].DeleteWithoutFilters(daptinid.DaptinReferenceId(idUUid), request, transaction)
			CheckErr(err, "Failed to delete inside action")
			if err != nil {
				actionResponse = NewActionResponse("client.notify", NewClientNotification("error", "Failed to delete "+model.GetName(), "Failed"))
				responses = append(responses, actionResponse)
				break OutFields
			} else {
				actionResponse = NewActionResponse("client.notify", NewClientNotification("success", "Deleted "+model.GetName(), "Success"))
			}
			actionResponses = append(actionResponses, actionResponse)
		case "EXECUTE":
			//res, err = Cruds[outcome.Type].Create(model, actionRequest)

			actionName := model.GetName()
			performer, ok := dbResource.ActionHandlerMap[actionName]
			if !ok {
				log.Errorf("Invalid outcome method: [%v]%v", outcome.Method, actionName)
			} else {
				var responder api2go.Responder
				outcome.Attributes["user"] = sessionUser
				responder, responses1, errors1 = performer.DoAction(outcome, model.GetAttributes(), transaction)

				actionResponses = append(actionResponses, responses1...)
				if len(errors1) > 0 {
					err = errors1[0]
					break OutFields
				}
				if responder != nil {
					api2GoResult := responder.Result().(api2go.Api2GoModel)
					if api2GoResult.GetName() == "render" {
						return []actionresponse.ActionResponse{
							{
								ResponseType: "render",
								Attributes:   api2GoResult.GetAttributes(),
							},
						}, nil
					}
					responseObjects = api2GoResult.GetAttributes()
				}
			}

		case "ACTIONRESPONSE":
			//res, err = Cruds[outcome.Type].Create(model, actionRequest)
			log.Debugf("Create action response: %v", model.GetName())
			var actionResponse actionresponse.ActionResponse
			actionResponse = NewActionResponse(model.GetName(), model.GetAttributes())
			responseObjects = api2go.Response{
				Res: model,
			}
			actionResponses = append(actionResponses, actionResponse)
		default:
			handler, ok := dbResource.ActionHandlerMap[outcome.Type]

			if !ok {
				log.Errorf("Unknown method invoked on [%v]: [%v] by session user [%v]",
					outcome.Type, outcome.Method, sessionUser.UserReferenceId)
				continue
			}
			responder, responses1, err1 := handler.DoAction(outcome, model.GetAttributes(), transaction)
			if err1 != nil {
				err = err1[0]
			} else {
				actionResponses = append(actionResponses, responses1...)
				responseObjects = responder
			}

		}

		if outcome.LogToConsole {
			for i, response := range actionResponses {

				attrsAsJson, _ := json.Marshal(response.Attributes)

				log.Infof("[%s][%s] by user [%s] OutcomeResponse[%d]: [%s] => %s",
					actionRequest.Type,
					actionRequest.Action,
					sessionUser.UserReferenceId,
					i,
					response.ResponseType,
					attrsAsJson)
			}

		}

		if err != nil {
			log.Errorf("failed to execute outcome [%v] => %v", outcome.Type, err)
			return nil, err
		}

		if !outcome.SkipInResponse {
			for _, ar := range actionResponses {
				attrs, yes := ar.Attributes.(map[string]interface{})
				if yes {
					for key, val := range attrs {
						refId, isRef := val.(daptinid.DaptinReferenceId)
						if isRef {
							attrs[key] = refId.String()
						}
					}
				} else {
					attrsArray, yes := ar.Attributes.([]map[string]interface{})
					if yes {
						for _, atr := range attrsArray {
							for key, val := range atr {
								refId, isRef := val.(daptinid.DaptinReferenceId)
								if isRef {
									atr[key] = refId.String()
								}
							}
						}
					}
				}
			}
			responses = append(responses, actionResponses...)
		}

		if len(actionResponses) > 0 && outcome.Reference != "" {
			lst := make([]interface{}, 0)
			for i, res := range actionResponses {
				inFieldMap[fmt.Sprintf("response.%v[%v]", outcome.Reference, i)] = res.Attributes
				lst = append(lst, res.Attributes)
			}
			inFieldMap[fmt.Sprintf("%v", outcome.Reference)] = lst
		}

		if responseObjects != nil && outcome.Reference != "" {

			api2goModel, ok := responseObjects.(api2go.Response)
			if ok {
				responseObjects = api2goModel.Result().(api2go.Api2GoModel).GetAttributes()
			}

			singleResult, isSingleResult := responseObjects.(map[string]interface{})

			if isSingleResult {
				inFieldMap[outcome.Reference] = singleResult
			} else {
				resultArray, ok := responseObjects.([]map[string]interface{})

				finalArray := make([]map[string]interface{}, 0)
				if ok {
					for i, item := range resultArray {
						finalArray = append(finalArray, item)
						inFieldMap[fmt.Sprintf("%v[%v]", outcome.Reference, i)] = item
					}
				}
				inFieldMap[outcome.Reference] = finalArray

			}
		}

	}
	if err != nil {
		return nil, err
	}

	return responses, nil
}

func BuildActionRequest(closer io.ReadCloser, actionType, actionName string,
	params gin.Params, queryParams url.Values) (actionresponse.ActionRequest, error) {
	bytes, err := io.ReadAll(closer)
	actionRequest := actionresponse.ActionRequest{}
	actionRequest.RawBodyBytes = bytes
	actionRequest.RawBodyString = string(bytes)
	if err != nil {
		return actionRequest, err
	}
	closer.Close()

	err = json.Unmarshal(bytes, &actionRequest)
	if err != nil {
		values, err := url.ParseQuery(string(bytes))
		CheckErr(err, "Failed to parse body as query values")
		if err == nil {

			attributesMap := make(map[string]interface{})
			actionRequest.Attributes = make(map[string]interface{})
			for key, val := range values {
				if len(val) > 1 {
					attributesMap[key] = val
					actionRequest.Attributes[key] = val
				} else {
					attributesMap[key] = val[0]
					actionRequest.Attributes[key] = val[0]
				}
			}
			attributesMap["__body"] = string(bytes)
			actionRequest.Attributes = attributesMap
		}
	}

	if actionRequest.Attributes == nil {
		actionRequest.Attributes = make(map[string]interface{})
	}

	var data map[string]interface{}
	err = json.Unmarshal(bytes, &data)
	//CheckErr(err, "Failed to read body as json", data)
	for k, v := range data {
		if k == "attributes" {
			continue
		}
		actionRequest.Attributes[k] = v
	}

	actionRequest.Type = actionType
	actionRequest.Action = actionName

	if actionRequest.Attributes == nil {
		actionRequest.Attributes = make(map[string]interface{})
	}
	for _, param := range params {
		actionRequest.Attributes[param.Key] = param.Value
	}
	for key, valueArray := range queryParams {

		if len(valueArray) == 1 {
			actionRequest.Attributes[key] = valueArray[0]
		} else {
			actionRequest.Attributes[key] = valueArray
		}
	}

	return actionRequest, nil
}

func NewClientNotification(notificationType string, message string, title string) map[string]interface{} {

	m := make(map[string]interface{})

	m["type"] = notificationType
	m["message"] = message
	m["title"] = title
	return m

}

func GetMD5HashString(text string) string {
	return GetMD5Hash([]byte(text))
}

func GetMD5Hash(text []byte) string {
	hasher := md5.New()
	hasher.Write(text)
	return hex.EncodeToString(hasher.Sum(nil))
}

func NewActionResponse(responseType string, attrs interface{}) actionresponse.ActionResponse {

	ar := actionresponse.ActionResponse{
		ResponseType: responseType,
		Attributes:   attrs,
	}

	return ar

}

func BuildOutcome(inFieldMap map[string]interface{},
	outcome actionresponse.Outcome, sessionUser *auth.SessionUser) (*api2go.Api2GoModel,
	api2go.Request, error) {

	attrInterface, err := BuildActionContext(outcome.Attributes, inFieldMap)
	if err != nil {
		return nil, api2go.Request{}, err
	}
	attrs := attrInterface.(map[string]interface{})

	switch outcome.Type {
	case "system_json_schema_update":

		ur, _ := url.Parse("/")
		responseModel := api2go.NewApi2GoModel("__restart", nil, 0, nil)
		returnRequest := api2go.Request{
			PlainRequest: &http.Request{
				Method: "EXECUTE",
				URL:    ur,
			},
		}

		files1, ok := attrs["json_schema"]
		if !ok {
			return nil, returnRequest, errors.New("no files uploaded")
		}
		log.Printf("Files [%v]: %v", attrs, files1)
		files, ok := files1.([]interface{})
		if !ok || len(files) < 1 {
			return nil, returnRequest, errors.New("no files uploaded")
		}
		for _, file := range files {
			f := file.(map[string]interface{})
			fileName := f["name"].(string)
			log.Printf("File name: %v", fileName)
			fileNameParts := strings.Split(fileName, ".")
			fileFormat := fileNameParts[len(fileNameParts)-1]
			contents := f["file"]
			if contents == nil {
				contents = f["contents"]
			}
			if contents == nil {
				log.Printf("Contents are missing in the update schema request: %v", f)
				continue
			}
			fileContentsBase64 := contents.(string)
			var fileBytes []byte
			contentParts := strings.Split(fileContentsBase64, ",")
			if len(contentParts) > 1 {
				fileBytes, err = base64.StdEncoding.DecodeString(contentParts[1])
			} else {
				fileBytes, err = base64.StdEncoding.DecodeString(fileContentsBase64)
			}
			if err != nil {
				return nil, returnRequest, err
			}

			jsonFileName := fmt.Sprintf("schema_uploaded_%v_daptin.%v", fileName, fileFormat)
			err = os.WriteFile(jsonFileName, fileBytes, 0644)
			if err != nil {
				log.Errorf("Failed to write json file: %v", jsonFileName)
				return nil, returnRequest, err
			}

		}

		log.Printf("Written all json files. Attempting restart")

		return &responseModel, returnRequest, nil

	case "__download_cms_config":
		fallthrough
	case "__become_admin":

		ur, _ := url.Parse("/")
		returnRequest := api2go.Request{
			PlainRequest: &http.Request{
				Method: "EXECUTE",
				URL:    ur,
			},
		}

		model := api2go.NewApi2GoModelWithData(outcome.Type, nil, int64(auth.DEFAULT_PERMISSION), nil, attrs)

		return &model, returnRequest, nil
	case "__as_user":

		ur, _ := url.Parse("/")
		returnRequest := api2go.Request{
			PlainRequest: &http.Request{
				Method: "SWITCH_USER",
				URL:    ur,
			},
		}

		model := api2go.NewApi2GoModelWithData(outcome.Type, nil, int64(auth.DEFAULT_PERMISSION), nil, attrs)

		return &model, returnRequest, nil

	case "action.response":
		fallthrough
	case "client.redirect":
		fallthrough
	case "client.store.set":
		fallthrough
	case "client.notify":
		//respopnseModel := NewActionResponse(attrs["responseType"].(string), attrs)
		ur, _ := url.Parse("/")
		returnRequest := api2go.Request{
			PlainRequest: &http.Request{
				Method: "ACTIONRESPONSE",
				URL:    ur,
			},
		}
		ctxWithUser := context.WithValue(returnRequest.PlainRequest.Context(), "user", sessionUser)
		returnRequest.PlainRequest = returnRequest.PlainRequest.WithContext(ctxWithUser)

		model := api2go.NewApi2GoModelWithData(outcome.Type, nil, int64(auth.DEFAULT_PERMISSION), nil, attrs)

		return &model, returnRequest, err

	default:

		ur, _ := url.Parse("/" + outcome.Type)
		model := api2go.NewApi2GoModelWithData(outcome.Type, nil, int64(auth.DEFAULT_PERMISSION), nil, attrs)
		returnRequest := api2go.Request{
			PlainRequest: &http.Request{
				Method: outcome.Method,
				URL:    ur,
			},
		}

		ctxWithUser := context.WithValue(returnRequest.PlainRequest.Context(), "user", sessionUser)
		returnRequest.PlainRequest = returnRequest.PlainRequest.WithContext(ctxWithUser)

		return &model, returnRequest, err

	}

	//return nil, api2go.Request{}, errors.New(fmt.Sprintf("Unidentified outcome: %v", outcome.Type))

}

func runUnsafeJavascript(unsafe string, contextMap map[string]interface{}) (interface{}, error) {

	vm := goja.New()

	//vm.ToValue(contextMap)
	for key, val := range contextMap {
		vm.Set(key, val)
	}

	for key, function := range CryptoFuncMap {
		vm.Set(key, function)
	}

	for key, function := range EncodingFuncMap {
		vm.Set(key, function)
	}

	vm.Set("uuid", func() string {
		u, _ := uuid.NewV7()
		return u.String()
	})
	v, err := vm.RunString(unsafe) // Here be dragons (risky code)

	if err != nil {
		return nil, err
	}

	return v.Export(), nil
}

func BuildActionContext(outcomeAttributes interface{}, inFieldMap map[string]interface{}) (interface{}, error) {

	var data interface{}

	kindOfOutcome := reflect.TypeOf(outcomeAttributes).Kind()

	if kindOfOutcome == reflect.Map {

		dataMap := make(map[string]interface{})

		outcomeMap := outcomeAttributes.(map[string]interface{})
		for key, field := range outcomeMap {

			typeOfField := reflect.TypeOf(field).Kind()
			//log.Printf("Outcome attribute [%v] == %v [%v]", key, field, typeOfField)

			if typeOfField == reflect.String {

				fieldString := field.(string)

				val, err := EvaluateString(fieldString, inFieldMap)
				//log.Printf("Value of [%v] == [%v]", key, val)
				if err != nil {
					return data, err
				}
				if val != nil {
					dataMap[key] = val
				}

			} else if typeOfField == reflect.Map || typeOfField == reflect.Slice || typeOfField == reflect.Array {

				val, err := BuildActionContext(field, inFieldMap)
				if err != nil {
					return data, err
				}
				if val != nil {
					dataMap[key] = val
				}
			} else {
				dataMap[key] = field
			}

		}

		data = dataMap

	} else if kindOfOutcome == reflect.Array || kindOfOutcome == reflect.Slice {

		outcomeArray, ok := outcomeAttributes.([]interface{})

		if !ok {
			outcomeArray = make([]interface{}, 0)
			outcomeArrayString := outcomeAttributes.([]string)
			for _, o := range outcomeArrayString {
				outcomeArray = append(outcomeArray, o)
			}
		}

		outcomes := make([]interface{}, 0)

		for _, outcome := range outcomeArray {

			outcomeKind := reflect.TypeOf(outcome).Kind()

			if outcomeKind == reflect.Map || outcomeKind == reflect.Array || outcomeKind == reflect.Slice {
				outc, err := BuildActionContext(outcome, inFieldMap)
				//log.Printf("Outcome is: %v", outc)
				if err != nil {
					return data, err
				}
				outcomes = append(outcomes, outc)
			} else {

				outcomeString, isString := outcome.(string)

				if isString {
					evtStr, err := EvaluateString(outcomeString, inFieldMap)
					if err != nil {
						return data, err
					}
					outcomes = append(outcomes, evtStr)
				} else {
					outcomes = append(outcomes, outcome)
				}

			}

		}
		data = outcomes

	}

	return data, nil
}

func EvaluateString(fieldString string, inFieldMap map[string]interface{}) (interface{}, error) {

	var valueToReturn interface{}

	if fieldString == "" {
		return "", nil
	}

	if fieldString[0] == '!' {

		res, err := runUnsafeJavascript(fieldString[1:], inFieldMap)
		if err != nil {
			return nil, fmt.Errorf("[1084] failed to evaluate JS in outcome attribute for key %s: %v", fieldString, err)
		}
		valueToReturn = res

	} else if len(fieldString) > 3 && BeginsWith(fieldString, "{{") && EndsWithCheck(fieldString, "}}") {

		jsString := fieldString[2 : len(fieldString)-2]
		res, err := runUnsafeJavascript(jsString, inFieldMap)
		if err != nil {
			return nil, fmt.Errorf("[1093] failed to evaluate JS in outcome attribute for key %s: %v", fieldString, err)
		}
		valueToReturn = res

	} else if len(fieldString) > 3 && fieldString[0:3] == "js:" {

		res, err := runUnsafeJavascript(fieldString[1:], inFieldMap)
		if err != nil {
			return nil, fmt.Errorf("[1101] failed to evaluate JS in outcome attribute for key %s: %v", fieldString, err)
		}
		valueToReturn = res

	} else if fieldString[0] == '~' {

		fieldParts := strings.Split(fieldString[1:], ".")

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
		if finalValue == nil {
			return nil, nil
		}

		castMap := finalValue.(map[string]interface{})
		finalValue = castMap[fieldParts[len(fieldParts)-1]]
		valueToReturn = finalValue

	} else {
		//log.Printf("Get [%v] from infields: %v", fieldString, ToJson(inFieldMap))

		rex := regexp.MustCompile(`\$([a-zA-Z0-9_\[\]]+)?(\.[a-zA-Z0-9_\[\]]+)*`)
		matches := rex.FindAllStringSubmatch(fieldString, -1)

		for _, match := range matches {

			fieldParts := strings.Split(match[0][1:], ".")

			if fieldParts[0] == "" {
				fieldParts[0] = "subject"
			}

			var finalValue interface{}

			// it looks confusing but it does whats its supposed to do
			// todo: add helpful comment

			finalValue = inFieldMap
			for i := 0; i < len(fieldParts)-1; i++ {

				fieldPart := fieldParts[i]

				fieldIndexParts := strings.Split(fieldPart, "[")
				if len(fieldIndexParts) > 1 {
					right := strings.Split(fieldIndexParts[1], "]")
					index, err := strconv.ParseInt(right[0], 10, 64)
					if err == nil {
						finalValMap := finalValue.(map[string]interface{})
						mapPart := finalValMap[fieldIndexParts[0]]
						mapPartArray, ok := mapPart.([]map[string]interface{})
						if !ok {
							mapPartArrayInterface, ok := mapPart.([]interface{})
							if ok {
								mapPartArray = make([]map[string]interface{}, 0)
								for _, ar := range mapPartArrayInterface {
									mapPartArray = append(mapPartArray, ar.(map[string]interface{}))
								}
							}
						}

						if int(index) > len(mapPartArray)-1 {
							return nil, fmt.Errorf("failed to evaluate value from array in outcome attribute for key %s, index [%d] is out of range [%d values]: %v", fieldString, index, len(mapPartArray), err)
						}
						finalValue = mapPartArray[index]
					} else {
						finalValue = finalValue.(map[string]interface{})[fieldPart]
					}
				} else {
					var ok bool
					finalValue, ok = finalValue.(map[string]interface{})[fieldPart]
					if !ok {
						return nil, fmt.Errorf("failed to evaluate value from array in outcome attribute for key %s, value is nil", fieldString)
					}

				}

			}
			if finalValue == nil {
				return nil, nil
			}

			castMap, ok := finalValue.(map[string]interface{})
			if ok {
				lastFieldPart := fieldParts[len(fieldParts)-1]
				finalValue = castMap[lastFieldPart]
				fieldString = strings.Replace(fieldString, fmt.Sprintf("%v", match[0]), fmt.Sprintf("%v", finalValue), -1)
			} else {
				castArray, arrayOk := finalValue.([]interface{})
				ok = arrayOk
				if arrayOk {
					lastFieldPart := fieldParts[len(fieldParts)-1]
					lastFieldPartIndex, err := strconv.ParseInt(lastFieldPart, 10, 32)
					if err != nil {
						log.Errorf("Non-Integer access to array index: %s", lastFieldPart)
					}
					finalValue = castArray[lastFieldPartIndex]
					fieldString = strings.Replace(fieldString, fmt.Sprintf("%v", match[0]), fmt.Sprintf("%v", finalValue), -1)
				}
			}
			if !ok {
				log.Errorf("Value at [%v] is %v", fieldString, castMap)
				return valueToReturn, errors.New(fmt.Sprintf("unable to evaluate value for [%v]", fieldString))
			}
		}
		valueToReturn = fieldString

	}
	//log.Printf("Evaluated string path [%v] => %v", fieldString, valueToReturn)

	return valueToReturn, nil
}

func GetValidatedInFields(actionRequest actionresponse.ActionRequest, action actionresponse.Action) (map[string]interface{}, error) {

	dataMap := actionRequest.Attributes
	finalDataMap := make(map[string]interface{})

	for _, inField := range action.InFields {
		if len(inField.ColumnName) == 0 && len(inField.Name) > 0 {
			inField.ColumnName = inField.Name
		}
		val, ok := dataMap[inField.ColumnName]
		if ok {
			finalDataMap[inField.ColumnName] = val

		} else if inField.DefaultValue != "" {
		} else if inField.IsNullable {

		} else {
			return nil, errors.New(fmt.Sprintf("Field %s cannot be blank", inField.Name))
		}
	}

	return finalDataMap, nil
}
