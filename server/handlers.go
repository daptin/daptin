package server

import (
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

func CreateEventHandler(initConfig *resource.CmsConfig, fsmManager resource.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		sessionUser := gincontext.Request.Context().Value("user").(*auth.SessionUser)

		pr := &http.Request{}
		pr.Method = "GET"
		req := api2go.Request{
			PlainRequest: gincontext.Request,
			QueryParams:  map[string][]string{},
		}

		objectStateMachineId := gincontext.Param("objectStateId")
		typename := gincontext.Param("typename")

		objectStateMachineResponse, err := cruds[typename+"_state"].FindOne(objectStateMachineId, req)
		if err != nil {
			log.Errorf("Failed to get object state machine: %v", err)
			gincontext.AbortWithError(400, err)
			return
		}

		objectStateMachine := objectStateMachineResponse.Result().(*api2go.Api2GoModel)

		stateObject := objectStateMachine.Data

		var subjectInstanceModel *api2go.Api2GoModel
		//var stateMachineDescriptionInstance *api2go.Api2GoModel

		for _, included := range objectStateMachine.Includes {
			casted := included.(*api2go.Api2GoModel)
			if casted.GetTableName() == typename {
				subjectInstanceModel = casted
			}

		}

		stateMachineId := objectStateMachine.GetID()
		eventName := gincontext.Param("eventName")

		stateMachinePermission := cruds["smd"].GetRowPermission(objectStateMachine.GetAllAsAttributes())

		if !stateMachinePermission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups) {
			gincontext.AbortWithStatus(403)
			return
		}

		nextState, err := fsmManager.ApplyEvent(subjectInstanceModel.GetAllAsAttributes(), resource.NewStateMachineEvent(stateMachineId, eventName))
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}

		stateAudit := objectStateMachine.GetAuditModel()
		creator, ok := cruds[stateAudit.GetTableName()]
		if ok {

			newRequest := &http.Request{
				Method: "POST",
			}
			newRequest = newRequest.WithContext(gincontext.Request.Context())

			req := api2go.Request{
				PlainRequest: newRequest,
				QueryParams:  map[string][]string{},
			}

			stateAudit.Data["source_reference_id"] = objectStateMachine.GetReferenceId()

			_, err := creator.Create(stateAudit, req)
			resource.CheckErr(err, "Failed to create audit for [%v]", objectStateMachine.GetTableName())
		}

		s, v, err := statementbuilder.Squirrel.Update(typename+"_state").
			Set("current_state", nextState).
			Set("version", stateObject["version"].(int64)+1).
			Where(squirrel.Eq{"reference_id": stateMachineId}).ToSql()

		_, err = db.Exec(s, v...)
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}

		gincontext.AbortWithStatus(200)

	}

}

func CreateEventStartHandler(fsmManager resource.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		user := gincontext.Request.Context().Value("user")
		var sessionUser *auth.SessionUser

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		jsBytes, err := ioutil.ReadAll(gincontext.Request.Body)
		if err != nil {
			log.Errorf("Failed to read post body: %v", err)
			gincontext.AbortWithError(400, err)
			return
		}

		m := make(map[string]interface{})
		json.Unmarshal(jsBytes, &m)

		typename := m["typeName"].(string)
		refId := m["referenceId"].(string)
		stateMachineId := gincontext.Param("stateMachineId")

		pr := &http.Request{}
		pr.Method = "GET"
		pr = pr.WithContext(gincontext.Request.Context())
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  map[string][]string{},
		}

		response, err := cruds["smd"].FindOne(stateMachineId, req)
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}

		stateMachineInstance := response.Result().(*api2go.Api2GoModel)
		stateMachineInstanceProperties := stateMachineInstance.GetAttributes()
		stateMachinePermission := cruds["smd"].GetRowPermission(stateMachineInstance.GetAllAsAttributes())

		if !stateMachinePermission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups) {
			gincontext.AbortWithStatus(403)
			return
		}

		subjectInstanceResponse, err := cruds[typename].FindOne(refId, req)
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}
		subjectInstanceModel := subjectInstanceResponse.Result().(*api2go.Api2GoModel).GetAttributes()

		newStateMachine := make(map[string]interface{})

		newStateMachine["current_state"] = stateMachineInstanceProperties["initial_state"]
		newStateMachine[typename+"_smd"] = stateMachineInstanceProperties["reference_id"]
		newStateMachine["is_state_of_"+typename] = subjectInstanceModel["reference_id"]
		newStateMachine["permission"] = int64(auth.None | auth.UserRead | auth.UserExecute | auth.GroupCreate | auth.GroupExecute)

		req.PlainRequest.Method = "POST"

		resp, err := cruds[typename+"_state"].Create(api2go.NewApi2GoModelWithData(typename+"_state", nil, 0, nil, newStateMachine), req)

		if err != nil {
			log.Errorf("Failed to execute state insert query: %v", err)
			gincontext.AbortWithError(500, err)
			return
		}

		gincontext.JSON(200, resp)

	}

}
