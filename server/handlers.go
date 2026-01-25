package server

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/fsm"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func CreateEventHandler(initConfig *resource.CmsConfig, fsmManager fsm.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		userValue := gincontext.Request.Context().Value("user")
		if userValue == nil {
			gincontext.AbortWithStatus(401)
			return
		}
		sessionUser := userValue.(*auth.SessionUser)

		pr := &http.Request{
			URL: gincontext.Request.URL,
		}
		pr.Method = "GET"
		req := api2go.Request{
			PlainRequest: gincontext.Request,
			QueryParams:  map[string][]string{},
		}

		objectStateMachineUuidString := gincontext.Param("objectStateId")
		typename := gincontext.Param("typename")

		if cruds[typename] == nil {
			log.Errorf("State transition failed: cruds['%s'] is nil. Available tables: %v", typename, func() []string {
				keys := make([]string, 0, len(cruds))
				for k := range cruds {
					keys = append(keys, k)
				}
				return keys
			}())
			gincontext.AbortWithStatus(500)
			return
		}

		objectStateMachineResponse, err := cruds[typename].FindOne(objectStateMachineUuidString, req)
		if err != nil {
			log.Errorf("Failed to get object state machine: %v", err)
			gincontext.AbortWithError(400, err)
			return
		}

		objectStateMachine := objectStateMachineResponse.Result().(api2go.Api2GoModel)

		stateObject := objectStateMachine.GetAttributes()

		var subjectInstanceModel api2go.Api2GoModel
		//var stateMachineDescriptionInstance *api2go.Api2GoModel

		for _, included := range objectStateMachine.Includes {
			casted := included.(api2go.Api2GoModel)
			if casted.GetTableName() == typename {
				subjectInstanceModel = casted
			}

		}

		stateMachineId := uuid.MustParse(objectStateMachine.GetID())
		eventName := gincontext.Param("eventName")

		transaction, err := db.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [59]")
			return
		}

		defer transaction.Commit()
		stateMachinePermission := cruds["smd"].GetRowPermission(objectStateMachine.GetAllAsAttributes(), transaction)

		if !stateMachinePermission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, cruds["usergroup"].AdministratorGroupId) {
			gincontext.AbortWithStatus(403)
			return
		}

		nextState, err := fsmManager.ApplyEvent(subjectInstanceModel.GetAllAsAttributes(),
			fsm.NewStateMachineEvent(daptinid.DaptinReferenceId(stateMachineId), eventName))
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}

		stateAudit := objectStateMachine.GetAuditModel()
		creator, ok := cruds[stateAudit.GetTableName()]
		if ok {

			newRequest := &http.Request{
				Method: "POST",
				URL:    gincontext.Request.URL,
			}
			newRequest = newRequest.WithContext(gincontext.Request.Context())

			req := api2go.Request{
				PlainRequest: newRequest,
				QueryParams:  map[string][]string{},
			}

			stateAudit.Set("source_reference_id", objectStateMachine.GetReferenceId())

			_, err := creator.CreateWithTransaction(stateAudit, req, transaction)
			resource.CheckErr(err, "Failed to create audit for [%v]", objectStateMachine.GetTableName())
		}

		s, v, err := statementbuilder.Squirrel.Update(typename + "_state").
			Set(goqu.Record{
				"current_state": nextState,
				"version":       stateObject["version"].(int64) + 1,
			}).
			Where(goqu.Ex{"reference_id": stateMachineId}).ToSQL()

		_, err = transaction.Exec(s, v...)
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}

		gincontext.AbortWithStatus(200)

	}

}

func CreateEventStartHandler(fsmManager fsm.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		user := gincontext.Request.Context().Value("user")
		var sessionUser *auth.SessionUser

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		requestBodyBytes, err := io.ReadAll(gincontext.Request.Body)
		if err != nil {
			log.Errorf("Failed to read post body: %v", err)
			gincontext.AbortWithError(400, err)
			return
		}

		requestBodyMap := make(map[string]interface{})
		json.Unmarshal(requestBodyBytes, &requestBodyMap)

		typename := requestBodyMap["typeName"].(string)
		refId := uuid.MustParse(requestBodyMap["referenceId"].(string))
		stateMachineUuidString := gincontext.Param("stateMachineId")

		pr := &http.Request{
			URL: gincontext.Request.URL,
		}
		pr.Method = "GET"
		pr = pr.WithContext(gincontext.Request.Context())
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  map[string][]string{},
		}

		response, err := cruds["smd"].FindOne(stateMachineUuidString, req)
		log.Tracef("Found one from smd")
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}

		stateMachineInstance := response.Result().(api2go.Api2GoModel)
		stateMachineInstanceProperties := stateMachineInstance.GetAttributes()
		transaction, err := db.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [160]")
			return
		}

		defer transaction.Commit()
		stateMachinePermission := cruds["smd"].GetRowPermission(stateMachineInstance.GetAllAsAttributes(), transaction)

		if !stateMachinePermission.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, cruds["usergroup"].AdministratorGroupId) {
			gincontext.AbortWithStatus(403)
			return
		}

		subjectInstanceResponse, err := cruds[typename].FindOneWithTransaction(daptinid.DaptinReferenceId(refId), req, transaction)
		if err != nil {
			gincontext.AbortWithError(400, err)
			return
		}
		subjectInstanceModel := subjectInstanceResponse.Result().(api2go.Api2GoModel).GetAttributes()

		newStateMachine := make(map[string]interface{})

		newStateMachine["current_state"] = stateMachineInstanceProperties["initial_state"]
		newStateMachine[typename+"_smd"] = stateMachineInstanceProperties["reference_id"]
		newStateMachine["is_state_of_"+typename] = subjectInstanceModel["reference_id"]
		newStateMachine["permission"] = int64(auth.None | auth.UserRead | auth.UserExecute | auth.GroupCreate | auth.GroupExecute)

		req.PlainRequest.Method = "POST"

		resp, err := cruds[typename+"_state"].CreateWithTransaction(
			api2go.NewApi2GoModelWithData(typename+"_state", nil, 0, nil, newStateMachine), req, transaction)

		if err != nil {
			log.Errorf("Failed to execute state insert query: %v", err)
			gincontext.AbortWithError(500, err)
			return
		}

		gincontext.JSON(200, resp)

	}

}
