package server

import (
	"errors"
	"fmt"
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
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"reflect"
)

func canExecuteStateMachine(cruds map[string]*resource.DbResource, smd map[string]interface{}, user *auth.SessionUser, transaction *sqlx.Tx) bool {
	adminGroupID := cruds["usergroup"].AdministratorGroupId
	tablePermission := cruds["smd"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "smd", transaction)
	if !tablePermission.CanExecute(user.UserReferenceId, user.Groups, adminGroupID) {
		return false
	}
	rowPermission := cruds["smd"].GetRowPermission(smd, transaction)
	return rowPermission.CanExecute(user.UserReferenceId, user.Groups, adminGroupID)
}

func CreateEventHandler(initConfig *resource.CmsConfig, fsmManager fsm.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		userValue := gincontext.Request.Context().Value("user")
		if userValue == nil {
			gincontext.AbortWithStatus(401)
			return
		}
		sessionUser := userValue.(*auth.SessionUser)

		objectStateMachineUuidString := gincontext.Param("objectStateId")
		typename := gincontext.Param("typename")
		typename_state := typename + "_state"

		req := api2go.Request{
			PlainRequest: gincontext.Request,
			QueryParams: map[string][]string{
				"included_relations": []string{
					"is_state_of_" + typename,
				},
			},
		}

		if cruds[typename_state] == nil {
			log.Errorf("State transition failed: cruds['%s'] is nil. Available tables: %v", typename_state, func() []string {
				keys := make([]string, 0, len(cruds))
				for k := range cruds {
					keys = append(keys, k)
				}
				return keys
			}())
			gincontext.AbortWithStatus(500)
			return
		}

		objectStateMachineResponse, err := cruds[typename_state].FindOne(objectStateMachineUuidString, req)
		if err != nil {
			log.Errorf("Failed to get object state machine: %v", err)
			gincontext.AbortWithError(400, err)
			return
		}

		objectStateMachine := objectStateMachineResponse.Result().(api2go.Api2GoModel)

		// Validate includes were loaded
		if len(objectStateMachine.Includes) == 0 {
			log.Errorf("No includes loaded for state transition. Required: is_state_of_%s", typename)
			gincontext.AbortWithStatus(500)
			return
		}

		var subjectInstanceModel api2go.Api2GoModel
		//var stateMachineDescriptionInstance *api2go.Api2GoModel

		for _, included := range objectStateMachine.Includes {
			casted := included.(api2go.Api2GoModel)
			if casted.GetTableName() == typename {
				subjectInstanceModel = casted
			}

		}

		// Verify subject instance was found in includes
		if reflect.ValueOf(subjectInstanceModel).IsZero() {
			log.Errorf("Subject instance not found in includes. Expected typename: %s", typename)
			gincontext.AbortWithStatus(500)
			return
		}

		stateMachineId := uuid.MustParse(objectStateMachine.GetID())
		eventName := gincontext.Param("eventName")
		transaction, err := db.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [59]")
			return
		}
		defer transaction.Rollback()

		smdReferenceID := daptinid.InterfaceToDIR(objectStateMachine.GetAttributes()[typename+"_smd"])
		if smdReferenceID == daptinid.NullReferenceId {
			gincontext.AbortWithStatus(500)
			return
		}
		smd, err := cruds["smd"].GetReferenceIdToObjectWithTransaction("smd", smdReferenceID, transaction)
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}

		if !canExecuteStateMachine(cruds, smd, sessionUser, transaction) {
			gincontext.AbortWithStatus(403)
			return
		}

		// Commit transaction BEFORE calling FSM to avoid deadlock
		err = transaction.Commit()
		if err != nil {
			resource.CheckErr(err, "Failed to commit permission check transaction")
			return
		}

		nextState, err := fsmManager.ApplyEvent(subjectInstanceModel.GetAllAsAttributes(),
			fsm.NewStateMachineEvent(daptinid.DaptinReferenceId(stateMachineId), eventName))
		if err != nil {
			if errors.Is(err, fsm.ErrInvalidTransition) {
				gincontext.AbortWithStatusJSON(400, gin.H{"errors": []gin.H{{
					"status": "400", "title": "Invalid Transition",
					"detail": fmt.Sprintf("event '%s' inappropriate in current state '%s'", eventName, objectStateMachine.GetAttributes()["current_state"]),
				}}})
			} else {
				gincontext.AbortWithError(500, err)
			}
			return
		}

		// Start new transaction for audit and state update
		transaction, err = db.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction for state update")
			return
		}
		defer transaction.Rollback()

		s, v, err := statementbuilder.Squirrel.Update(typename_state).Prepared(true).
			Set(goqu.Record{
				"current_state": nextState,
				"version":       goqu.L("COALESCE(version, 0) + 1"),
			}).
			Where(goqu.Ex{"reference_id": stateMachineId[:], "current_state": objectStateMachine.GetAttributes()["current_state"]}).ToSQL()
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}

		result, err := transaction.Exec(s, v...)
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}
		if rowsAffected == 0 {
			gincontext.AbortWithStatus(409)
			return
		}
		if rowsAffected != 1 {
			gincontext.AbortWithError(500, fmt.Errorf("state transition updated %d rows", rowsAffected))
			return
		}
		stateAudit := objectStateMachine.GetAuditModel()
		creator, ok := cruds[stateAudit.GetTableName()]
		if !ok {
			gincontext.AbortWithError(500, fmt.Errorf("state audit resource %s is unavailable", stateAudit.GetTableName()))
			return
		}

		newRequest := &http.Request{Method: "POST", URL: gincontext.Request.URL}
		newRequest = newRequest.WithContext(gincontext.Request.Context())
		auditRequest := api2go.Request{PlainRequest: newRequest, QueryParams: map[string][]string{}}

		stateAudit.Set("source_reference_id", objectStateMachine.GetReferenceId())
		stateAudit.Set("operation", resource.AuditOperationStateTransition)
		for column, value := range stateAudit.GetAttributes() {
			if referenceID, ok := value.(daptinid.DaptinReferenceId); ok {
				stateAudit.Set(column, referenceID.String())
			}
		}

		_, err = creator.CreateWithoutFilter(stateAudit, auditRequest, transaction)
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}
		var state map[string]interface{}
		if cruds[typename_state].PubSub != nil {
			state, err = cruds[typename_state].GetReferenceIdToObjectWithTransaction(typename_state, daptinid.DaptinReferenceId(stateMachineId), transaction)
			if err != nil {
				gincontext.AbortWithError(500, err)
				return
			}
		}
		// Commit transaction before returning
		err = transaction.Commit()
		if err != nil {
			gincontext.AbortWithError(500, err)
			return
		}
		if topic := cruds[typename_state].PubSub; topic != nil {
			resource.PublishUpdateEvent(topic, typename_state, state)
		}

		gincontext.AbortWithStatus(200)

	}

}

func CreateEventStartHandler(fsmManager fsm.FsmManager, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(gincontext *gin.Context) {

		user := gincontext.Request.Context().Value("user")
		sessionUser := &auth.SessionUser{}

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
		if !canExecuteStateMachine(cruds, stateMachineInstance.GetAllAsAttributes(), sessionUser, transaction) {
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
