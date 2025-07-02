package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"github.com/araddon/dateparse"
	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

const DATE_LAYOUT = "2006-01-02 15:04:05"

// IsUserActionAllowed Checks if a user identified by userReferenceId and belonging to userGroups is allowed to invoke an action `actionName` on type `typeName`
// Called before invoking an action from the /action/** api
// Checks EXECUTE on both the type and action for this user
// The permissions can come from different groups
func (dbResource *DbResource) IsUserActionAllowed(
	userReferenceId daptinid.DaptinReferenceId, userGroups auth.GroupPermissionList, typeName string, actionName string, transaction *sqlx.Tx) bool {

	permission := dbResource.GetObjectPermissionByWhereClause("world", "table_name", typeName, transaction)

	actionPermission := dbResource.GetObjectPermissionByWhereClause("action", "action_name", actionName, transaction)

	canExecuteOnType := permission.CanExecute(userReferenceId, userGroups, dbResource.AdministratorGroupId)
	canExecuteAction := actionPermission.CanExecute(userReferenceId, userGroups, dbResource.AdministratorGroupId)

	return canExecuteOnType && canExecuteAction

}

func (dbResource *DbResource) IsUserActionAllowedWithTransaction(userReferenceId daptinid.DaptinReferenceId,
	userGroups auth.GroupPermissionList, typeName string, actionName string, transaction *sqlx.Tx) bool {

	permission := dbResource.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", typeName, transaction)

	actionPermission := dbResource.GetObjectPermissionByWhereClauseWithTransaction("action", "action_name", actionName, transaction)

	canExecuteOnType := permission.CanExecute(userReferenceId, userGroups, dbResource.AdministratorGroupId)
	canExecuteAction := actionPermission.CanExecute(userReferenceId, userGroups, dbResource.AdministratorGroupId)

	return canExecuteOnType && canExecuteAction

}

// GetActionByName Gets an Action instance by `typeName` and `actionName`
// Check Action instance for usage
func (dbResource *DbResource) GetActionByName(typeName string, actionName string, transaction *sqlx.Tx) (actionresponse.Action, error) {
	var actionRow ActionRow
	var action actionresponse.Action

	cacheKey := fmt.Sprintf("action-%v-%v", typeName, actionName)
	if OlricCache != nil {
		value, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil && value != nil {

			var cachedActionRow ActionRow
			err = value.Scan(&cachedActionRow)
			action, err = ActionFromActionRow(cachedActionRow)
			return action, err
		}
	}

	sql, args, err := statementbuilder.Squirrel.Select(
		goqu.I("a.action_name").As("name"),
		goqu.I("w.table_name").As("ontype"),
		goqu.I("a.label").As("label"),
		goqu.I("action_schema").As("action_schema"),
		goqu.I("a.instance_optional").As("instance_optional"),
		goqu.I("a.reference_id").As("referenceid"),
	).Prepared(true).From(goqu.T("action").As("a")).
		Join(
			goqu.T("world").As("w"),
			goqu.On(goqu.Ex{
				"w.id": goqu.I("a.world_id"),
			}),
		).Where(goqu.Ex{"w.table_name": typeName}).Where(goqu.Ex{"a.action_name": actionName}).Limit(1).ToSQL()

	if err != nil {
		return action, err
	}

	stmt, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[72] failed to prepare statment: %v", err)
		return action, err
	}

	errScan := stmt.QueryRowx(args...).StructScan(&actionRow)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}
	if errScan != nil {
		log.Errorf("sql: %v", sql)
		log.Errorf("Failed to scan action 66: %v", err)
		return action, err
	}

	action, err = ActionFromActionRow(actionRow)
	if err != nil {
		return action, err
	}

	if OlricCache != nil {
		err = OlricCache.Put(context.Background(), cacheKey, actionRow, olric.EX(1*time.Minute), olric.NX())
		CheckInfo(err, "Failed to set action in olric cache")
	}

	return action, nil
}

func ActionFromActionRow(actionRow ActionRow) (actionresponse.Action, error) {
	var action actionresponse.Action
	err := json.Unmarshal([]byte(actionRow.ActionSchema), &action)
	CheckErr(err, "[129] failed to unmarshal ActionSchema")

	action.Name = actionRow.Name
	action.Label = actionRow.Name
	action.ReferenceId = actionRow.ReferenceId
	action.OnType = actionRow.OnType
	action.InstanceOptional = actionRow.InstanceOptional
	return action, err
}

// GetActionsByType Gets the list of all actions defined on type `typeName`
// Returns list of `Action`
func (dbResource *DbResource) GetActionsByType(typeName string, transaction *sqlx.Tx) ([]actionresponse.Action, error) {
	action := make([]actionresponse.Action, 0)

	sql, args, err := statementbuilder.Squirrel.Select(
		goqu.I("a.action_name").As("name"),
		goqu.I("w.table_name").As("ontype"),
		goqu.I("a.label"),
		goqu.I("action_schema"),
		goqu.I("instance_optional"),
		goqu.I("a.reference_id").As("referenceid"),
	).Prepared(true).From(goqu.T("action").As("a")).Join(goqu.T("world").As("w"), goqu.On(goqu.Ex{
		"w.id": goqu.I("a.world_id"),
	})).Where(goqu.Ex{
		"w.table_name": typeName,
	}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[124] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	rows, err := stmt.Queryx(args...)
	if err != nil {
		log.Errorf("[126] Failed to scan action: %v", err)
		return action, err
	}
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[132] failed to close row after value scan")
		}
	}(rows)

	for rows.Next() {

		var act actionresponse.Action
		var a ActionRow
		err := rows.StructScan(&a)
		CheckErr(err, "Failed to struct scan action row")

		if len(a.Label) < 1 {
			continue
		}
		err = json.Unmarshal([]byte(a.ActionSchema), &act)
		CheckErr(err, "failed to unmarshal ActionSchema")

		act.Name = a.Name
		act.Label = a.Label
		act.ReferenceId = a.ReferenceId
		act.OnType = a.OnType
		act.InstanceOptional = a.InstanceOptional

		action = append(action, act)

	}

	return action, nil
}

// GetActionPermissionByName Gets permission of an action by typeId and actionName
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Special utility function for actions, for other objects use GetObjectPermissionByReferenceId
func (dbResource *DbResource) GetActionPermissionByName(worldId int64, actionName string, transaction *sqlx.Tx) (permission.PermissionInstance, error) {

	refId, err := dbResource.GetReferenceIdByWhereClause("action", goqu.Ex{"action_name": actionName}, goqu.Ex{"world_id": worldId})
	if err != nil {
		return permission.PermissionInstance{}, err
	}

	if refId == nil || len(refId) < 1 {
		return permission.PermissionInstance{}, errors.New(fmt.Sprintf("Failed to find action [%v] on [%v]", actionName, worldId))
	}
	permissions := dbResource.GetObjectPermissionByReferenceId("action", refId[0], transaction)

	return permissions, nil
}

// GetObjectPermissionByReferenceId Gets permission of an Object by typeName and string referenceId
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Return a NoPermissionToAnyone if no such object exist
func (dbResource *DbResource) GetObjectPermissionByReferenceId(objectType string, referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) permission.PermissionInstance {

	var selectQuery string
	var queryParameters []interface{}
	var err error
	var perm permission.PermissionInstance
	if referenceId == daptinid.NullReferenceId {
		return perm
	}
	if objectType == "usergroup" {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select("permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	} else {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()

	}

	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(selectQuery)
	if err != nil {
		log.Errorf("[263] failed to prepare statment: %v", err)
		return perm
	}

	resultObject := make(map[string]interface{})
	errScan := stmt.QueryRowx(queryParameters...).MapScan(resultObject)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}
	if errScan != nil {
		log.Errorf("Failed to scan permission 1 [%v]: %v", referenceId, err)
	}
	//log.Printf("permi map: %v", resultObject)
	if resultObject[USER_ACCOUNT_ID_COLUMN] != nil {

		user, err := dbResource.GetIdToReferenceId(USER_ACCOUNT_TABLE_NAME, resultObject[USER_ACCOUNT_ID_COLUMN].(int64), transaction)
		if err == nil {
			perm.UserId = user
		}

	}

	i, ok := resultObject["id"].(int64)
	if !ok {
		return perm
	}
	perm.UserGroupId = dbResource.GetObjectGroupsByObjectId(objectType, i, transaction)

	perm.Permission = auth.AuthPermission(resultObject["permission"].(int64))
	if err != nil {
		log.Errorf("Failed to scan permission 2: %v", err)
	}

	//log.Printf("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

// GetObjectPermissionByReferenceId Gets permission of an Object by typeName and string referenceId
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Return a NoPermissionToAnyone if no such object exist
func GetObjectPermissionByReferenceIdWithTransaction(objectType string, referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) permission.PermissionInstance {

	cacheKey := fmt.Sprintf("object-permission-%v-%v", objectType, referenceId)

	if OlricCache != nil {

		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			var pi permission.PermissionInstance
			cachedValue.Scan(&pi)
			return pi
		}
	}

	var selectQuery string
	var queryParameters []interface{}
	var err error
	var perm permission.PermissionInstance
	if referenceId == daptinid.NullReferenceId {
		return perm
	}

	if objectType == "usergroup" {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select("permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	} else {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()

	}

	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(selectQuery)
	if err != nil {
		log.Errorf("[347] failed to prepare statment: %v", err)
		return perm
	}

	resultObject := make(map[string]interface{})
	errScan := stmt.QueryRowx(queryParameters...).MapScan(resultObject)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}
	if errScan != nil {
		log.Errorf("Failed to scan permission 1 [%v]: %v", referenceId, errScan)
	}
	//log.Printf("permi map: %v", resultObject)
	if resultObject[USER_ACCOUNT_ID_COLUMN] != nil {

		user, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, resultObject[USER_ACCOUNT_ID_COLUMN].(int64), transaction)
		if err == nil {
			perm.UserId = user
		}

	}

	i, ok := resultObject["id"].(int64)
	if !ok {
		return perm
	}
	perm.UserGroupId = GetObjectGroupsByObjectIdWithTransaction(objectType, i, transaction)

	perm.Permission = auth.AuthPermission(resultObject["permission"].(int64))
	if err != nil {
		log.Errorf("Failed to scan permission 2: %v", err)
	}

	if OlricCache != nil {
		cachePutErr := OlricCache.Put(context.Background(), cacheKey, perm, olric.EX(30*time.Minute), olric.NX())
		CheckErr(cachePutErr, "[374] failed to store cloud store in cache")
	}

	//log.Printf("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

// Get permission of an Object by typeName and string referenceId
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Return a NoPermissionToAnyone if no such object exist
func (dbResource *DbResource) GetObjectPermissionById(objectType string, id int64, transaction *sqlx.Tx) permission.PermissionInstance {

	var selectQuery string
	var queryParameters []interface{}
	var err error
	var perm permission.PermissionInstance
	if objectType == "usergroup" {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select("permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"id": id}).
			ToSQL()
	} else {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"id": id}).
			ToSQL()

	}

	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(selectQuery)
	if err != nil {
		log.Errorf("[289] failed to prepare statment: %v", err)
		return perm
	}

	resultObject := make(map[string]interface{})
	errScan := stmt.QueryRowx(queryParameters...).MapScan(resultObject)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}
	if errScan != nil {
		log.Errorf("Failed to scan permission 3 [%v]: %v", id, err)
	}
	//log.Printf("permi map: %v", resultObject)
	if resultObject[USER_ACCOUNT_ID_COLUMN] != nil {

		user, err := dbResource.GetIdToReferenceId(USER_ACCOUNT_TABLE_NAME, resultObject["user_account_id"].(int64), transaction)
		if err == nil {
			perm.UserId = user
		}
	}

	perm.UserGroupId = dbResource.GetObjectGroupsByObjectId(objectType, resultObject["id"].(int64), transaction)

	perm.Permission = auth.AuthPermission(resultObject["permission"].(int64))
	if err != nil {
		log.Errorf("Failed to scan permission 2: %v", err)
	}

	//log.Printf("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

// Get permission of an Object by typeName and string referenceId
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Return a NoPermissionToAnyone if no such object exist
func (dbResource *DbResource) GetObjectPermissionByIdWithTransaction(objectType string, id int64, transaction *sqlx.Tx) permission.PermissionInstance {

	var selectQuery string
	var queryParameters []interface{}
	var err error
	var perm permission.PermissionInstance
	if objectType == "usergroup" {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select("permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"id": id}).
			ToSQL()
	} else {
		selectQuery, queryParameters, err = statementbuilder.Squirrel.
			Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").Prepared(true).
			From(objectType).Where(goqu.Ex{"id": id}).
			ToSQL()

	}

	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(selectQuery)
	if err != nil {
		log.Errorf("[289] failed to prepare statment: %v", err)
		return perm
	}

	resultObject := make(map[string]interface{})
	errScan := stmt.QueryRowx(queryParameters...).MapScan(resultObject)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	if errScan != nil {
		log.Errorf("Failed to scan permission 3 [%v]: %v", id, err)
	}
	//log.Printf("permission map: %v", resultObject)
	if resultObject[USER_ACCOUNT_ID_COLUMN] != nil {

		user, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, resultObject["user_account_id"].(int64), transaction)
		if err == nil {
			perm.UserId = user
		}
	}

	perm.UserGroupId = GetObjectGroupsByObjectIdWithTransaction(objectType, resultObject["id"].(int64), transaction)

	perm.Permission = auth.AuthPermission(resultObject["permission"].(int64))
	if err != nil {
		log.Errorf("Failed to scan permission 2: %v", err)
	}

	//log.Printf("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

var OlricCache olric.DMap

// GetObjectPermissionByWhereClause Gets permission of an Object by typeName and string referenceId with a simple where clause colName = colValue
// Use carefully
// Loads the owner, usergroup and guest permission of the action from the database
// Return a PermissionInstance
// Return a NoPermissionToAnyone if no such object exist
func (dbResource *DbResource) GetObjectPermissionByWhereClause(objectType string, colName string, colValue string, transaction *sqlx.Tx) permission.PermissionInstance {
	if OlricCache == nil {
		OlricCache, _ = dbResource.OlricDb.NewDMap("default-cache")
	}

	cacheKey := ""
	if OlricCache != nil {
		cacheKey = fmt.Sprintf("%s_%s_%s", objectType, colName, colValue)
		cachedPermission, err := OlricCache.Get(context.Background(), cacheKey)
		if cachedPermission != nil && err == nil {
			var perm permission.PermissionInstance
			err := cachedPermission.Scan(&perm)
			if err == nil {
				return perm
			}
		}
	}

	var perm permission.PermissionInstance
	s, q, err := statementbuilder.Squirrel.
		Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").From(objectType).Prepared(true).
		Where(goqu.Ex{colName: colValue}).ToSQL()
	if err != nil {
		log.Errorf("[542] Failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[548] failed to prepare statment: %v", err)
		return perm
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("[554] failed to close prepared statement: %v", err)
		}
	}(stmt)

	m := make(map[string]interface{})
	errScan := stmt.QueryRowx(q...).MapScan(m)
	err = stmt.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}
	if errScan != nil {

		log.Errorf("[566] Failed to scan permission: %v", err)
		return perm
	}

	//log.Printf("permi map: %v", m)
	if m["user_account_id"] != nil {

		user, err := dbResource.GetIdToReferenceId(USER_ACCOUNT_TABLE_NAME, m[USER_ACCOUNT_ID_COLUMN].(int64), transaction)
		if err == nil {
			perm.UserId = user
		}

	}

	perm.UserGroupId = dbResource.GetObjectGroupsByObjectId(objectType, m["id"].(int64), transaction)

	perm.Permission = auth.AuthPermission(m["permission"].(int64))

	//log.Printf("PermissionInstance for [%v]: %v", typeName, perm)

	if OlricCache != nil {
		err = OlricCache.Put(context.Background(), cacheKey, perm, olric.EX(10*time.Second), olric.NX())
		CheckErr(err, "[2099] Failed to set object permission id in olric cache")
	}
	return perm
}

func (dbResource *DbResource) GetObjectPermissionByWhereClauseWithTransaction(objectType string, colName string, colValue string, transaction *sqlx.Tx) permission.PermissionInstance {
	if OlricCache == nil {
		OlricCache, _ = dbResource.OlricDb.NewDMap("default-cache")
	}

	cacheKey := ""
	cacheKey = fmt.Sprintf("object-permission-%s_%s_%s", objectType, colName, colValue)
	if OlricCache != nil {
		cachedPermission, err := OlricCache.Get(context.Background(), cacheKey)
		if cachedPermission != nil && err == nil {
			var pi permission.PermissionInstance
			err = cachedPermission.Scan(&pi)
			if err == nil {
				return pi
			} else {
				log.Errorf("Failed to GetObjectPermissionByWhereClauseWithTransaction [%s][%s][%s]: %v", objectType, colName, colValue, err)
			}
		}
	}

	var perm permission.PermissionInstance
	s, q, err := statementbuilder.Squirrel.
		Select(USER_ACCOUNT_ID_COLUMN, "permission", "id").From(objectType).Prepared(true).
		Where(goqu.Ex{colName: colValue}).ToSQL()
	if err != nil {
		log.Errorf("[618 failed to create sql: %v", err)
		return perm
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[624] failed to prepare statment: %v", err)
		return perm
	}

	m := make(map[string]interface{})
	errScan := stmt.QueryRowx(q...).MapScan(m)
	err = stmt.Close()
	if err != nil {
		log.Errorf("[632] failed to close prepared statement: %v", err)
	}
	if errScan != nil {

		log.Errorf("[636] failed to scan permission: %v", err)
		return perm
	}

	//log.Printf("permi map: %v", m)
	if m["user_account_id"] != nil {

		user, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, m[USER_ACCOUNT_ID_COLUMN].(int64), transaction)
		if err == nil {
			perm.UserId = user
		} else {
			log.Errorf("Failed GetIdToReferenceIdWithTransaction [%v] [%v]", m, err)
		}

	}

	perm.UserGroupId = GetObjectGroupsByObjectIdWithTransaction(objectType, m["id"].(int64), transaction)

	perm.Permission = auth.AuthPermission(m["permission"].(int64))

	log.Debugf("PermissionInstance for [%v]: %v", objectType, perm)

	if OlricCache != nil {
		OlricCache.Put(context.Background(), cacheKey, perm, olric.EX(10*time.Minute), olric.NX())
		CheckErr(err, "[617] Failed to set id to reference id in olric cache")
	}
	return perm
}

//// GetObjectUserGroupsByWhere Get list of group permissions for objects of typeName where colName=colValue
//// Utility method which makes a join query to load a lot of permissions quickly
//// Used by GetRowPermission
//func (dbResource *DbResource) GetObjectUserGroupsByWhere(objectType string, colName string, colValue interface{}) []auth.GroupPermission {
//
//	//if OlricCache == nil {
//	//	OlricCache, _ = dbResource.OlricDb.NewDMap("default-cache")
//	//}
//	//
//	//cacheKey := ""
//	//if OlricCache != nil {
//	//	cacheKey = fmt.Sprintf("groups-%s_%s_%s", objectType, colName, colValue)
//	//	cachedPermission, err := OlricCache.Get(cacheKey)
//	//	if cachedPermission != nil && err == nil {
//	//		return cachedPermission.([]auth.GroupPermission)
//	//	}
//	//}
//
//	s := make([]auth.GroupPermission, 0)
//
//	rel := api2go.TableRelation{}
//	rel.Subject = objectType
//	rel.SubjectName = objectType + "_id"
//	rel.Object = "usergroup"
//	rel.ObjectName = "usergroup_id"
//	rel.Relation = "has_many_and_belongs_to_many"
//
//	//log.Printf("Join string: %v: ", rel.GetJoinString())
//
//	sql, args, err := statementbuilder.Squirrel.Select(
//		goqu.I("usergroup_id.reference_id").As("groupreferenceid"),
//		goqu.I(rel.GetJoinTableName()+".reference_id").As("relationreferenceid"),
//		goqu.I(rel.GetJoinTableName()+".permission").As("permission"),
//	).From(goqu.T(rel.GetSubject())).
//		// rel.GetJoinString()
//		Join(goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
//			goqu.On(goqu.Ex{
//				fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetSubject(), "id")),
//			})).
//		Join(goqu.T(rel.GetObject()).As(rel.GetObjectName()),
//			goqu.On(goqu.Ex{
//				fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObjectName(), "id")),
//			})).
//		Where(goqu.Ex{
//			fmt.Sprintf("%s.%s", rel.Subject, colName): colValue,
//		}).ToSQL()
//	if err != nil {
//		log.Errorf("Failed to create permission select query: %v", err)
//		return s
//	}
//
//	stmt, err := dbResource.connection.Preparex(sql)
//	if err != nil {
//		log.Errorf("[720] failed to prepare statment: %v", err)
//		return nil
//	}
//	defer func(stmt1 *sqlx.Stmt) {
//		err := stmt1.Close()
//		if err != nil {
//			log.Errorf("failed to close prepared statement: %v", err)
//		}
//	}(stmt)
//
//	res, err := stmt.Queryx(args...)
//	//log.Printf("Group select sql: %v", sql)
//	if err != nil {
//
//		log.Errorf("Failed to get object groups by where clause: %v", err)
//		log.Errorf("Query: %s == [%v]", sql, args)
//		return s
//	}
//	defer res.Close()
//
//	for res.Next() {
//		var g auth.GroupPermission
//		err = res.StructScan(&g)
//		if err != nil {
//			log.Errorf("Failed to scan group permission 1: %v", err)
//		}
//		s = append(s, g)
//	}
//
//	//if OlricCache != nil {
//	//	_ = OlricCache.Put(cacheKey, s, olric.EX(10*time.Second), olric.NX())
//	//}
//
//	return s
//
//}

// GetObjectUserGroupsByWhere Get list of group permissions for objects of typeName where colName=colValue
// Utility method which makes a join query to load a lot of permissions quickly
// Used by GetRowPermission
func (dbResource *DbResource) GetObjectUserGroupsByWhereWithTransaction(objectType string, transaction *sqlx.Tx,
	colName string, colValue interface{}) auth.GroupPermissionList {

	//if OlricCache == nil {
	//	OlricCache, _ = dbResource.OlricDb.NewDMap("default-cache")
	//}
	//
	//cacheKey := ""
	//if OlricCache != nil {
	//	cacheKey = fmt.Sprintf("groups-%s_%s_%s", objectType, colName, colValue)
	//	cachedPermission, err := OlricCache.Get(cacheKey)
	//	if cachedPermission != nil && err == nil {
	//		return cachedPermission.([]auth.GroupPermission)
	//	}
	//}

	s := make(auth.GroupPermissionList, 0)

	rel := api2go.TableRelation{}
	rel.Subject = objectType
	rel.SubjectName = objectType + "_id"
	rel.Object = "usergroup"
	rel.ObjectName = "usergroup_id"
	rel.Relation = "has_many_and_belongs_to_many"

	//log.Printf("Join string: %v: ", rel.GetJoinString())

	sql, args, err := statementbuilder.Squirrel.Select(
		goqu.I("usergroup_id.reference_id").As("groupreferenceid"),
		goqu.I(rel.GetJoinTableName()+".reference_id").As("relationreferenceid"),
		goqu.I(rel.GetJoinTableName()+".permission").As("permission"),
	).Prepared(true).From(goqu.T(rel.GetSubject())).
		// rel.GetJoinString()
		Join(goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
			goqu.On(goqu.Ex{
				fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetSubject(), "id")),
			})).
		Join(goqu.T(rel.GetObject()).As(rel.GetObjectName()),
			goqu.On(goqu.Ex{
				fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObjectName(), "id")),
			})).
		Where(goqu.Ex{
			fmt.Sprintf("%s.%s", rel.Subject, colName): colValue,
		}).ToSQL()
	if err != nil {
		log.Errorf("Failed to create permission select query: %v", err)
		return s
	}

	stmt, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[811] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("[814] failed to close prepared statement: %v", err)
		}
	}(stmt)

	res, err := stmt.Queryx(args...)
	//log.Printf("Group select sql: %v", sql)
	if err != nil {

		log.Errorf("[822] failed to get object groups by where clause: %v", err)
		log.Errorf("[823] query: %s == [%v]", sql, args)
		return s
	}
	defer res.Close()

	for res.Next() {
		var g auth.GroupPermission
		err = res.StructScan(&g)
		if err != nil {
			log.Errorf("Failed to scan group permission 1: %v", err)
		}
		s = append(s, g)
	}

	//if OlricCache != nil {
	//	_ = OlricCache.Put(cacheKey, s, olric.EX(10*time.Second), olric.NX())
	//}

	return s

}

func (dbResource *DbResource) GetObjectGroupsByObjectId(objType string, objectId int64, transaction *sqlx.Tx) auth.GroupPermissionList {
	s := make([]auth.GroupPermission, 0)

	refId, err := dbResource.GetIdToReferenceId(objType, objectId, transaction)

	if objType == "usergroup" {

		if err != nil {
			log.Printf("Failed to get id to reference id [%v][%v] == %v", objType, objectId, err)
			return s
		}
		s = append(s, auth.GroupPermission{
			GroupReferenceId:    refId,
			ObjectReferenceId:   refId,
			RelationReferenceId: refId,
			Permission:          auth.AuthPermission(dbResource.Cruds["usergroup"].model.GetDefaultPermission()),
		})
		return s
	}

	sql, args, err := statementbuilder.Squirrel.Select(
		goqu.I("ug.reference_id").As("groupreferenceid"),
		goqu.I("uug.reference_id").As("relationreferenceid"),
		goqu.I("uug.permission").As("permission"),
	).Prepared(true).From(goqu.T("usergroup").As("ug")).
		Join(
			goqu.T(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", objType, objType)).As("uug"),
			goqu.On(goqu.Ex{"uug.usergroup_id": goqu.I("ug.id")})).
		Where(goqu.Ex{
			fmt.Sprintf("uug.%s_id", objType): objectId,
		}).ToSQL()

	stmt, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[501] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res, err := stmt.Queryx(args...)

	if err != nil {
		log.Errorf("Failed to query object group by object id 403 [%v][%v] == %v", objType, objectId, err)
		return s
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("[478] failed to close result after value scan in defer")
		}
	}(res)

	for res.Next() {
		var g auth.GroupPermission
		vals := make(map[string]interface{})
		err := res.MapScan(vals)
		g.GroupReferenceId = daptinid.DaptinReferenceId(vals["groupreferenceid"].([]uint8))
		g.RelationReferenceId = daptinid.DaptinReferenceId(vals["relationreferenceid"].([]uint8))
		g.Permission = auth.AuthPermission(vals["permission"].(int64))
		g.ObjectReferenceId = refId
		if err != nil {
			log.Errorf("Failed to scan group permission 3: %v", err)
		}
		s = append(s, g)
	}
	return s

}

func GetObjectGroupsByObjectIdWithTransaction(objectType string, objectId int64, transaction *sqlx.Tx) auth.GroupPermissionList {

	cacheKey := fmt.Sprintf("object-groups-%v-%v", objectType, objectId)

	if OlricCache != nil {

		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			var res []auth.GroupPermission
			err = cachedValue.Scan(&res)
			if err != nil {
				log.Errorf("[933] Failed to scan permission from cache: %v", err)
			} else {
				return res
			}
		}
	}
	groupPermissionList := make(auth.GroupPermissionList, 0)

	refId, err := GetIdToReferenceIdWithTransaction(objectType, objectId, transaction)

	if objectType == "usergroup" {

		if err != nil {
			log.Printf("Failed to get id to reference id [%v][%v] == %v", objectType, objectId, err)
			return groupPermissionList
		}
		groupPermissionList = append(groupPermissionList, auth.GroupPermission{
			GroupReferenceId:    refId,
			ObjectReferenceId:   refId,
			RelationReferenceId: refId,
			Permission:          auth.DEFAULT_PERMISSION,
		})
		return groupPermissionList
	}

	sql, args, err := statementbuilder.Squirrel.Select(
		goqu.I("ug.reference_id").As("groupreferenceid"),
		goqu.I("uug.reference_id").As("relationreferenceid"),
		goqu.I("uug.permission").As("permission"),
	).Prepared(true).From(goqu.T("usergroup").As("ug")).
		Join(
			goqu.T(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", objectType, objectType)).As("uug"),
			goqu.On(goqu.Ex{"uug.usergroup_id": goqu.I("ug.id")})).
		Where(goqu.Ex{
			fmt.Sprintf("uug.%s_id", objectType): objectId,
		}).ToSQL()

	stmt, err := transaction.Preparex(sql)
	if err != nil {
		log.Errorf("[501] failed to prepare statment: %v", err)
		return nil
	}
	defer stmt.Close()
	res, err := stmt.Queryx(args...)

	if err != nil {
		log.Errorf("Failed to query object group by object id 403 [%v][%v] == %v", objectType, objectId, err)
		return groupPermissionList
	}
	defer res.Close()
	for res.Next() {
		var g auth.GroupPermission
		err = res.StructScan(&g)
		g.ObjectReferenceId = refId
		if err != nil {
			log.Errorf("Failed to scan group permission 2: %v", err)

		}
		groupPermissionList = append(groupPermissionList, g)
	}

	if OlricCache != nil {
		cachePutErr := OlricCache.Put(context.Background(), cacheKey, groupPermissionList, olric.EX(30*time.Second), olric.NX())
		CheckErr(cachePutErr, "[996] failed to store config value in cache [%v]", cacheKey)
	}

	return groupPermissionList

}

// CanBecomeAdmin Checks if the context user can invoke the become admin action
// checks if there is only 1 real user in the system
// No one can become admin once we have an adminstrator
func (dbResource *DbResource) CanBecomeAdmin(transaction *sqlx.Tx) bool {

	adminRefId := GetAdminReferenceIdWithTransaction(transaction)
	if adminRefId == nil || len(adminRefId) == 0 {
		return true
	}

	return false

}

// GetUserAccountRowByEmail Returns the user account row of a user by looking up on email
func (dbResource *DbResource) GetUserAccountRowByEmail(email string, transaction *sqlx.Tx) (map[string]interface{}, error) {

	user, _, err := dbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClauseWithTransaction("user_account",
		nil, transaction, goqu.Ex{"email": email})

	if len(user) > 0 {

		return user[0], err
	}

	return nil, errors.New("no such user")

}

// GetUserAccountRowByEmail Returns the user account row of a user by looking up on email
func (dbResource *DbResource) GetUserAccountRowByEmailWithTransaction(email string, transaction *sqlx.Tx) (map[string]interface{}, error) {

	user, _, err := dbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClauseWithTransaction(
		"user_account", nil, transaction, goqu.Ex{"email": email})

	if err != nil {
		return nil, err
	}

	if len(user) > 0 {
		return user[0], err
	}

	return nil, errors.New("no such user")

}

func (dbResource *DbResource) GetUserPassword(email string, transaction *sqlx.Tx) (string, error) {
	passwordHash := ""

	existingUsers, _, err := dbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClause("user_account", nil, transaction, goqu.Ex{"email": email})
	if err != nil {
		return passwordHash, err
	}
	if len(existingUsers) < 1 {
		return passwordHash, errors.New("user not found")
	}

	passwordHash = existingUsers[0]["password"].(string)

	return passwordHash, err
}

// UserGroupNameToId Converts group name to the internal integer id
func (dbResource *DbResource) UserGroupNameToId(groupName string) (uint64, error) {

	query, arg, err := statementbuilder.Squirrel.
		Select("id").From("usergroup").Prepared(true).Where(goqu.Ex{"name": groupName}).ToSQL()
	if err != nil {
		return 0, err
	}
	stmt, err := dbResource.Connection().Preparex(query)
	if err != nil {
		log.Errorf("[592] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res := stmt.QueryRowx(arg...)
	if res.Err() != nil {
		return 0, res.Err()
	}

	var id uint64
	err = res.Scan(&id)

	return id, err
}

// UserGroupNameToId Converts group name to the internal integer id
func (dbResource *DbResource) UserGroupNameToIdWithTransaction(groupName string, transaction *sqlx.Tx) (uint64, error) {

	query, arg, err := statementbuilder.Squirrel.Select("id").Prepared(true).
		From("usergroup").Where(goqu.Ex{"name": groupName}).ToSQL()
	if err != nil {
		return 0, err
	}
	stmt, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[592] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res := stmt.QueryRowx(arg...)
	if res.Err() != nil {
		return 0, res.Err()
	}

	var id uint64
	err = res.Scan(&id)

	return id, err
}

// BecomeAdmin make user the administrator and owner of everything
// Check CanBecomeAdmin before invoking this
func (dbResource *DbResource) BecomeAdmin(userId int64, transaction *sqlx.Tx) bool {
	log.Printf("User: %d is going to become admin", userId)
	if !dbResource.CanBecomeAdmin(transaction) {
		return false
	}

	for _, crud := range dbResource.Cruds {

		if crud.model.GetName() == "user_account_user_account_id_has_usergroup_usergroup_id" {
			continue
		}

		if crud.model.HasColumn(USER_ACCOUNT_ID_COLUMN) {

			q, v, err := statementbuilder.Squirrel.
				Update(crud.model.GetName()).Prepared(true).
				Set(goqu.Record{
					USER_ACCOUNT_ID_COLUMN: userId,
					"permission":           auth.DEFAULT_PERMISSION,
				}).ToSQL()
			if err != nil {
				log.Errorf("Query: %v", q)
				log.Errorf("Failed to create query to update to become admin: %v == %v", crud.model.GetName(), err)
				continue
			}

			_, err = transaction.Exec(q, v...)
			if err != nil {
				log.Errorf("Query: %v", q)
				log.Errorf("	Failed to execute become admin update query: %v", err)
				continue
			}

		}
	}

	adminUsergroupId, err := dbResource.UserGroupNameToIdWithTransaction("administrators", transaction)
	referenceId, _ := uuid.NewV7()

	query, args, err := statementbuilder.Squirrel.Insert("user_account_user_account_id_has_usergroup_usergroup_id").
		Cols(USER_ACCOUNT_ID_COLUMN, "usergroup_id", "permission", "reference_id").Prepared(true).
		Vals([]interface{}{userId, adminUsergroupId, int64(auth.DEFAULT_PERMISSION), referenceId[:]}).
		ToSQL()

	_, err = transaction.Exec(query, args...)
	CheckErr(err, "Failed to add user to administrator usergroup: %v == %v", query, args)
	if err != nil {
		return false
	}

	query, args, err = statementbuilder.Squirrel.Update("world").Prepared(true).
		Set(goqu.Record{
			"permission":         int64(auth.DEFAULT_PERMISSION),
			"default_permission": int64(auth.DEFAULT_PERMISSION),
		}).
		Where(goqu.Ex{
			"table_name": goqu.Op{"notlike": "%_audit"},
		}).
		ToSQL()
	if err != nil {
		log.Errorf("Failed to create sql for updating world permissions: %v", err)
		return false
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to update world permissions: %v", err)
		return false
	}

	query, args, err = statementbuilder.Squirrel.Update("world").Prepared(true).
		Set(goqu.Record{
			"permission":         int64(auth.UserCreate | auth.GroupCreate),
			"default_permission": int64(auth.UserRead | auth.GroupRead),
		}).
		Where(goqu.Ex{
			"table_name": goqu.Op{"like": "%_audit"},
		}).ToSQL()
	if err != nil {
		log.Errorf("Failed to create sql for update world audit permissions: %v", err)
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to world update audit permissions: %v", err)
	}

	query, args, err = statementbuilder.Squirrel.Update("action").Prepared(true).
		Set(goqu.Record{"permission": int64(auth.UserRead | auth.UserExecute | auth.GroupCRUD | auth.GroupExecute | auth.GroupRefer)}).
		ToSQL()
	if err != nil {
		log.Errorf("Failed to create update action permission sql : %v", err)
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to update action permissions : %v", err)
	}

	query, args, err = statementbuilder.Squirrel.Update("action").Prepared(true).
		Set(goqu.Record{"permission": int64(auth.GuestPeek | auth.GuestExecute | auth.UserRead | auth.UserExecute | auth.GroupRead | auth.GroupExecute)}).
		Where(goqu.Ex{
			"action_name": "signin",
		}).
		ToSQL()
	if err != nil {
		log.Errorf("Failed to create update sign in action permission sql : %v", err)
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to world update signin action  permissions: %v", err)
	}

	return true
}

func (dbResource *DbResource) GetRowPermission(row map[string]interface{}, transaction *sqlx.Tx) permission.PermissionInstance {

	refId := daptinid.InterfaceToDIR(row["reference_id"])
	if refId == daptinid.NullReferenceId {
		refId = daptinid.InterfaceToDIR(row["id"])
	}
	rowType := row["__type"].(string)

	var perm permission.PermissionInstance

	if rowType != "usergroup" {
		if row[USER_ACCOUNT_ID_COLUMN] != nil {
			perm.UserId = daptinid.InterfaceToDIR(row[USER_ACCOUNT_ID_COLUMN])
		} else {
			u, _ := dbResource.GetReferenceIdToObjectColumnWithTransaction(rowType, refId, USER_ACCOUNT_ID_COLUMN, transaction)
			if u != nil {
				perm.UserId = daptinid.InterfaceToDIR(u)
			}
		}

	}

	loc := strings.Index(rowType, "_has_")
	//log.Printf("Location [%v]: %v", dbResource.model.GetName(), loc)

	if BeginsWith(rowType, "file.") || rowType == "none" {
		perm.UserGroupId = auth.GroupPermissionList{
			{
				GroupReferenceId:    daptinid.NullReferenceId,
				ObjectReferenceId:   daptinid.NullReferenceId,
				RelationReferenceId: daptinid.NullReferenceId,
				Permission:          auth.GuestRead,
			},
		}
		return perm
	}

	if loc == -1 && dbResource.Cruds[rowType].model.HasMany("usergroup") {

		perm.UserGroupId = dbResource.GetObjectUserGroupsByWhereWithTransaction(rowType, transaction, "reference_id", refId[:])

	} else if rowType == "usergroup" {
		originalGroupId, _ := row["reference_id"]
		originalGroupIdStr := refId
		if originalGroupId != nil {
			originalGroupIdStr = daptinid.InterfaceToDIR(originalGroupId)
		}

		perm.UserGroupId = auth.GroupPermissionList{
			{
				GroupReferenceId:    originalGroupIdStr,
				ObjectReferenceId:   refId,
				RelationReferenceId: refId,
				Permission:          auth.AuthPermission(dbResource.Cruds["usergroup"].model.GetDefaultPermission()),
			},
		}
	} else if loc > -1 {
		// this is a something belongs to a usergroup row
		//for colName, colValue := range row {
		//	if EndsWithCheck(colName, "_id") && colName != "reference_id" {
		//		if colName != "usergroup_id" {
		//			return dbResource.GetObjectPermissionByReferenceId(strings.Split(rowType, "_"+colName)[0], colValue.(string))
		//		}
		//	}
		//}

	}

	rowPermission := row["permission"]
	if rowPermission != nil {

		var err error
		i64, ok := rowPermission.(int64)
		if !ok {
			f64, ok := rowPermission.(float64)
			if !ok {
				i64, err = strconv.ParseInt(rowPermission.(string), 10, 64)
				//p, err := int64(row["permission"].(int))
				if err != nil {
					log.Errorf("Invalid cast :%v", err)
				}
			} else {
				i64 = int64(f64)
			}
		}

		perm.Permission = auth.AuthPermission(i64)
	} else {
		pe := dbResource.GetObjectPermissionByReferenceId(rowType, refId, transaction)
		perm.Permission = pe.Permission
	}
	//log.Printf("Row permission: %v  ---------------- %v", perm, row)
	return perm
}

func (dbResource *DbResource) GetRowPermissionWithTransaction(row map[string]interface{}, transaction *sqlx.Tx) permission.PermissionInstance {

	var referenceId daptinid.DaptinReferenceId
	referenceId = daptinid.InterfaceToDIR(row["reference_id"])
	rowType := row["__type"].(string)

	cacheKey := fmt.Sprintf("row-permission-%v-%v", rowType, referenceId)

	if OlricCache != nil {

		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			var cv permission.PermissionInstance
			cachedValue.Scan(&cv)
			return cv
		}
	}

	var perm permission.PermissionInstance

	loc := strings.Index(rowType, "_has_")
	if rowType != "usergroup" && loc == -1 {
		if row[USER_ACCOUNT_ID_COLUMN] != nil {
			perm.UserId = daptinid.InterfaceToDIR(row[USER_ACCOUNT_ID_COLUMN])
		} else {
			u, _ := dbResource.GetReferenceIdToObjectColumnWithTransaction(rowType, referenceId, USER_ACCOUNT_ID_COLUMN, transaction)
			perm.UserId = daptinid.InterfaceToDIR(u)
		}

	}

	//log.Printf("Location [%v]: %v", dbResource.model.GetName(), loc)

	if BeginsWith(rowType, "file.") || rowType == "none" {
		perm.UserGroupId = auth.GroupPermissionList{
			{
				GroupReferenceId:    daptinid.NullReferenceId,
				ObjectReferenceId:   daptinid.NullReferenceId,
				RelationReferenceId: daptinid.NullReferenceId,
				Permission:          auth.GuestRead,
			},
		}
		return perm
	}

	if loc == -1 && dbResource.Cruds[rowType].model.HasMany("usergroup") {

		perm.UserGroupId = dbResource.GetObjectUserGroupsByWhereWithTransaction(rowType, transaction, "reference_id", referenceId[:])

	} else if rowType == "usergroup" {
		originalGroupId, _ := row["reference_id"]
		originalGroupIdStr := referenceId
		originalGroupIdStr = daptinid.InterfaceToDIR(originalGroupId)

		perm.UserGroupId = auth.GroupPermissionList{
			{
				GroupReferenceId:    originalGroupIdStr,
				ObjectReferenceId:   referenceId,
				RelationReferenceId: referenceId,
				Permission:          auth.AuthPermission(dbResource.Cruds["usergroup"].model.GetDefaultPermission()),
			},
		}
	} else if loc > -1 {
		// this is a something belongs to a usergroup row
		//for colName, colValue := range row {
		//	if EndsWithCheck(colName, "_id") && colName != "reference_id" {
		//		if colName != "usergroup_id" {
		//			return dbResource.GetObjectPermissionByReferenceId(strings.Split(rowType, "_"+colName)[0], colValue.(string))
		//		}
		//	}
		//}

	}

	rowPermission := row["permission"]
	if rowPermission != nil {

		var err error
		i64, ok := rowPermission.(int64)
		if !ok {
			f64, ok := rowPermission.(float64)
			if !ok {
				i64, err = strconv.ParseInt(rowPermission.(string), 10, 64)
				//p, err := int64(row["permission"].(int))
				if err != nil {
					log.Errorf("Invalid cast :%v", err)
				}
			} else {
				i64 = int64(f64)
			}
		}

		perm.Permission = auth.AuthPermission(i64)
	} else {
		pe := GetObjectPermissionByReferenceIdWithTransaction(rowType, referenceId, transaction)
		perm.Permission = pe.Permission
	}
	//log.Printf("Row permission: %v  ---------------- %v", perm, row)

	if OlricCache != nil {
		cachePutErr := OlricCache.Put(context.Background(), cacheKey, perm, olric.EX(1*time.Minute), olric.NX())
		CheckErr(cachePutErr, "failed to store object permission in cache [%v]", cacheKey)
	}

	return perm
}

func (dbResource *DbResource) GetRowsByWhereClause(typeName string, includedRelations map[string]bool, transaction *sqlx.Tx, where ...goqu.Ex) (
	[]map[string]interface{}, [][]map[string]interface{}, error) {

	stmt := statementbuilder.Squirrel.Select("*").Prepared(true).From(typeName)

	for _, w := range where {
		stmt = stmt.Where(w)
	}

	s, q, err := stmt.ToSQL()

	//log.Printf("GetRowsByWhereClause: %v == [%v]", s)

	stmt1, err := transaction.Preparex(s)

	if err != nil {
		log.Errorf("[839] failed to prepare statment - [%v]: %v", s, err)
		return nil, nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)

	if err != nil {
		return nil, nil, err
	}
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[802] failed to close rows after scanning values in defer")
		}
	}(rows)

	start := time.Now()
	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	err = stmt1.Close()
	err = rows.Close()

	m1, include, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), includedRelations, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetRowsByWhere ResultToArray: %v", duration)

	return m1, include, err

}

/////////////

func (dbResource *DbResource) GetRowsByWhereClauseWithTransaction(typeName string,
	includedRelations map[string]bool, transaction *sqlx.Tx, where ...goqu.Ex) (
	[]map[string]interface{}, [][]map[string]interface{}, error) {

	stmt := statementbuilder.Squirrel.Select("*").Prepared(true).From(typeName)

	for _, w := range where {
		stmt = stmt.Where(w)
	}

	s, q, err := stmt.ToSQL()

	//log.Printf("GetRowsByWhereClause: %v == [%v]", s)

	stmt1, err := transaction.Preparex(s)

	if err != nil {
		log.Errorf("[839] failed to prepare statment - [%v]: %v", s, err)
		return nil, nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)

	if err != nil {
		return nil, nil, err
	}
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[802] failed to close rows after scanning values in defer")
		}
	}(rows)

	start := time.Now()
	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	err = stmt1.Close()
	err = rows.Close()

	m1, include, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), includedRelations, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetRowsByWhere ResultToArray: %v", duration)

	return m1, include, err

}

func (dbResource *DbResource) GetRandomRow(typeName string, count uint, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	randomFunc := "RANDOM() * "

	if dbResource.Connection().DriverName() == "mysql" {
		randomFunc = "RAND() * "
	}

	// select id from world where id > RANDOM() * (SELECT MAX(id) FROM world) limit 15;
	maxSql, _, _ := goqu.Select(goqu.L("max(id)")).From(typeName).ToSQL()
	stmt := statementbuilder.Squirrel.Select("*").From(typeName).Prepared(true).Where(goqu.Ex{
		"id": goqu.Op{"gte": goqu.L(randomFunc + " ( " + maxSql + " ) ")},
	}).Limit(count)

	s, q, err := stmt.ToSQL()

	//log.Printf("Select query: %v == [%v]", s, q)

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[885] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)
	if err != nil {
		return nil, err
	}
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[843] failed to close rows after value scan in defer")
		}
	}(rows)

	start := time.Now()
	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	err = stmt1.Close()
	err = rows.Close()

	m1, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetRandomRow ResultToArray: %v", duration)

	return m1, err

}

func (dbResource *DbResource) GetUserMembersByGroupName(groupName string, transaction *sqlx.Tx) []daptinid.DaptinReferenceId {

	s, q, err := statementbuilder.Squirrel.
		Select("u.reference_id").Prepared(true).
		From(goqu.T("user_account_user_account_id_has_usergroup_usergroup_id").As("uu")).
		LeftJoin(
			goqu.T("user_account").As("u"), goqu.On(goqu.Ex{
				"uu.user_account_id": goqu.I("u.id"),
			})).
		LeftJoin(
			goqu.T("usergroup").As("g"), goqu.On(goqu.Ex{
				"uu.usergroup_id": goqu.I("g.id"),
			})).
		Where(goqu.Ex{"g.name": groupName}).
		Order(goqu.I("uu.created_at").Asc()).ToSQL()
	if err != nil {
		log.Errorf("Failed to create sql query 749: %v", err)
		return []daptinid.DaptinReferenceId{}
	}

	refIds := make([]daptinid.DaptinReferenceId, 0)

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[936] failed to prepare statment: %v", err)
		return nil
	}
	defer stmt1.Close()
	rows, err := stmt1.Queryx(q...)
	if err != nil {
		log.Errorf("Failed to create sql query 757: %v", err)
		return []daptinid.DaptinReferenceId{}
	}
	for rows.Next() {
		var refId daptinid.DaptinReferenceId
		err = rows.Scan(&refId)
		CheckErr(err, "failed to scan ref id")
		refIds = append(refIds, refId)
	}

	return refIds

}

func GetUserMembersByGroupNameWithTransaction(groupName string, transaction *sqlx.Tx) []daptinid.DaptinReferenceId {

	s, q, err := statementbuilder.Squirrel.
		Select("u.reference_id").Prepared(true).
		From(goqu.T("user_account_user_account_id_has_usergroup_usergroup_id").As("uu")).
		LeftJoin(
			goqu.T("user_account").As("u"), goqu.On(goqu.Ex{
				"uu.user_account_id": goqu.I("u.id"),
			})).
		LeftJoin(
			goqu.T("usergroup").As("g"), goqu.On(goqu.Ex{
				"uu.usergroup_id": goqu.I("g.id"),
			})).
		Where(goqu.Ex{"g.name": groupName}).
		Order(goqu.I("uu.created_at").Asc()).ToSQL()
	if err != nil {
		log.Errorf("Failed to create sql query 749: %v", err)
		return []daptinid.DaptinReferenceId{}
	}

	refIds := make([]daptinid.DaptinReferenceId, 0)

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[936] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)
	if err != nil {
		log.Errorf("Failed to create sql query 757: %v", err)
		return []daptinid.DaptinReferenceId{}
	}
	for rows.Next() {
		var refId daptinid.DaptinReferenceId
		err = rows.Scan(&refId)
		CheckErr(err, "failed to scan ref id")
		refIds = append(refIds, refId)
	}

	return refIds

}

func (dbResource *DbResource) GetUserEmailIdByUsergroupId(usergroupId int64, transaction *sqlx.Tx) string {

	s, q, err := statementbuilder.Squirrel.Select("u.email").Prepared(true).From(goqu.T("user_account_user_account_id_has_usergroup_usergroup_id").As("uu")).
		LeftJoin(
			goqu.T(USER_ACCOUNT_TABLE_NAME).As("u"),
			goqu.On(goqu.Ex{
				"uu." + USER_ACCOUNT_ID_COLUMN: goqu.I("u.id"),
			}),
		).Prepared(true).Where(goqu.Ex{"uu.usergroup_id": usergroupId}).
		Order(goqu.I("uu.created_at").Asc()).Limit(1).ToSQL()
	if err != nil {
		log.Errorf("Failed to create sql query 781: %v", err)
		return ""
	}

	var email string

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[981] failed to prepare statment: %v", err)
		return ""
	}
	defer stmt1.Close()
	err = stmt1.QueryRowx(q...).Scan(&email)
	if err != nil {
		log.Warnf("Failed to execute query 789: %v == %v", s, q)
		log.Warnf("Failed to scan user group id from the result 830: %v", err)
	}

	return email

}

func (dbResource *DbResource) GetUserById(userId int64, transaction *sqlx.Tx) (map[string]interface{}, error) {

	user, _, err := dbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowById("user_account", userId, nil, transaction)

	if len(user) > 0 {
		return user, err
	}

	return nil, errors.New("no such user")

}

func (dbResource *DbResource) GetSingleRowByReferenceIdWithTransaction(typeName string, referenceId daptinid.DaptinReferenceId,
	includedRelations map[string]bool, transaction *sqlx.Tx) (map[string]interface{}, []map[string]interface{}, error) {
	log.Tracef("Get single row by id: [%v][%v]", typeName, referenceId)
	s, q, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		log.Errorf("failed to create select query by ref id: %v", referenceId)
		return nil, nil, err
	}

	start := time.Now()
	stmt1, err := transaction.Preparex(s)
	duration := time.Since(start)
	log.Tracef("[TIMING] SingleRowSelect Preparex: %v", duration)
	if err != nil {
		log.Errorf("[1841] failed to prepare statment - [%v]: %v", s, err)
		return nil, nil, err
	}
	defer stmt1.Close()

	start = time.Now()
	rows, err := stmt1.Queryx(q...)
	duration = time.Since(start)
	log.Tracef("[TIMING] SingleRowSelect Queryx [1824]: %v", duration)

	defer func() {
		if rows == nil {
			log.Printf("rows is already closed in get single row by reference id")
			return
		}
		err = rows.Close()
		CheckErr(err, "Failed to close rows after db query [%v]", s)
	}()

	if err != nil {
		log.Errorf("[940] failed to query single row by ref id: %v", err)
		return nil, nil, err
	}

	start = time.Now()
	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	err = stmt1.Close()
	err = rows.Close()

	resultRows, includeRows, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), includedRelations, transaction)
	duration = time.Since(start)
	log.Tracef("[TIMING] GetSingleRowByReferenceId ResultToArray [1843]: %v", duration)

	if err != nil {
		log.Printf("failed to ResultToArrayOfMap: %v", err)
		return nil, nil, err
	}

	if len(resultRows) < 1 {
		return nil, nil, fmt.Errorf("[897] no such entity [%v][%v]", typeName, referenceId)
	}

	m := resultRows[0]
	n := includeRows[0]

	log.Tracef("Completed  GetSingleRowByReferenceIdWithTransaction ResultToArray")
	return m, n, err

}

func (dbResource *DbResource) GetSingleRowById(typeName string, id int64, includedRelations map[string]bool, transaction *sqlx.Tx) (map[string]interface{}, []map[string]interface{}, error) {
	//log.Printf("Get single row by id: [%v][%v]", typeName, referenceId)
	s, q, err := statementbuilder.Squirrel.Select("*").
		Prepared(true).From(typeName).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		log.Errorf("Failed to create select query by id: %v", id)
		return nil, nil, err
	}

	stmt1, err := transaction.Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1063] failed to prepare statment - [%v]: %v", s, err)
		return nil, nil, err
	}

	rows, err := stmt1.Queryx(q...)
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[989] failed to close rows after value scan in defer")
		}
	}(rows)
	start := time.Now()
	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	err = stmt1.Close()
	err = rows.Close()

	resultRows, includeRows, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), includedRelations, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetSingleRowById ResultToArray: %v", duration)

	if err != nil {
		return nil, nil, err
	}

	if len(resultRows) < 1 {
		return nil, nil, fmt.Errorf("923 no such entity [%v][%v]", typeName, id)
	}

	m := resultRows[0]
	n := includeRows[0]

	return m, n, err

}

func (dbResource *DbResource) GetObjectByWhereClause(typeName string, column string, val interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {
	s, q, err := statementbuilder.Squirrel.Select("*").
		Prepared(true).From(typeName).Where(goqu.Ex{column: val}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)

	if err != nil {
		log.Errorf("[1106] failed to prepare statment - [%v]: %v", s, err)
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row, err := stmt1.Queryx(q...)

	if err != nil {
		return nil, err
	}

	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1029] failed to close result after value scan in defer")
		}
	}(row)

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt1.Close()
	err = row.Close()

	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetObjectByWhere ResultToArray [1946]: %v", duration)

	if len(m) == 0 {
		log.Printf("[1976] No result found for [%v] [%v][%v]", typeName, column, val)
		return nil, errors.New(fmt.Sprintf("no [%v=%v] object found", column, val))
	}

	return m[0], err
}

func (dbResource *DbResource) GetObjectByWhereClauseWithTransaction(
	typeName string, column string, val interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {
	s, q, err := statementbuilder.Squirrel.Select("*").Prepared(true).From(typeName).Where(goqu.Ex{column: val}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1106] failed to prepare statment - [%v]: %v", s, err)
		return nil, err
	}

	row, err := stmt1.Queryx(q...)
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1029] failed to close result after value scan in defer")
		}
	}(row)

	if err != nil {
		return nil, err
	}
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt1.Close()
	err = row.Close()

	start := time.Now()
	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetObjectByWhere ResultToArray [1991]: %v", duration)

	if len(m) == 0 {
		log.Printf("[2021] No result found for [%v] [%v][%v]", typeName, column, val)
		return nil, errors.New(fmt.Sprintf("no [%v=%v] object found", column, val))
	}

	return m[0], err
}

func (dbResource *DbResource) GetIdToObject(typeName string, id int64, transaction *sqlx.Tx) (map[string]interface{}, error) {
	//key := fmt.Sprintf("ito-%v-%v", typeName, id)
	//if OlricCache != nil {
	//	val, err := OlricCache.Get(key)
	//	if err == nil && val != nil {
	//		return val.(map[string]interface{}), nil
	//	}
	//}
	s, q, err := statementbuilder.Squirrel.Select(goqu.C("*")).Prepared(true).From(typeName).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1146] failed to prepare statment - [%v]: %v", s, err)
		return nil, err
	}

	row, err := stmt1.Queryx(q...)

	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1029] failed to close result after value scan in defer")
		}
	}(row)

	if err != nil {
		return nil, err
	}

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = row.Close()
	if err != nil {
		log.Errorf("[1064] failed to close result after value scan in defer")
	}
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetIdToObject ResultToArray: %v", duration)

	if len(m) == 0 {
		log.Printf("[2082] No result found for [%v][%v]", typeName, id)
		return nil, err
	}
	//if OlricCache != nil {
	//	err = OlricCache.Put(key, m[0], olric.EX(1*time.Minute), olric.NX())
	//	CheckErr(err, "[2034[ Failed to set id to object in olric cache")
	//}

	return m[0], nil
}

func (dbResource *DbResource) GetIdToObjectWithTransaction(typeName string, id int64, transaction *sqlx.Tx) (map[string]interface{}, error) {
	key := fmt.Sprintf("ito-%v-%v", typeName, id)
	if OlricCache != nil {
		val, err := OlricCache.Get(context.Background(), key)
		if err == nil && val != nil {
			var res map[string]interface{}
			err = val.Scan(res)
			return res, err
		}
	}
	s, q, err := statementbuilder.Squirrel.Select(goqu.C("*")).Prepared(true).From(typeName).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)

	if err != nil {
		log.Errorf("[1146] failed to prepare statment - [%v]: %v", s, err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row, err := stmt1.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1029] failed to close result after value scan in defer")
		}
	}(row)

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = row.Close()
	if err != nil {
		log.Errorf("[1064] failed to close result after value scan in defer")
		return nil, err
	}
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
		return nil, err
	}
	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)
	log.Tracef("[TIMING] GetIdToObject ResultToArray: %v", duration)

	if len(m) == 0 {
		log.Printf("[2151] No result found for [%v][%v]", typeName, id)
		return nil, fmt.Errorf("no such item %v-%v", typeName, id)
	}
	if OlricCache != nil {
		err = OlricCache.Put(context.Background(), key, m[0], olric.EX(5*time.Minute), olric.NX())
		//CheckErr(err, "[2099] Failed to set id to object in olric cache")
	}

	return m[0], nil
}

func (dbResource *DbResource) TruncateTable(typeName string, skipRelations bool, transaction *sqlx.Tx) error {
	log.Printf("Truncate table: %v", typeName)

	if !skipRelations {

		var err error
		for _, rel := range dbResource.tableInfo.Relations {

			if rel.Relation == "belongs_to" {
				if rel.Subject == dbResource.tableInfo.TableName {
					// err = dbResource.TruncateTable(rel.Object, true)
				} else {
					err = dbResource.TruncateTable(rel.Object, true, transaction)
				}
			}
			if rel.Relation == "has_many" {
				err = dbResource.TruncateTable(rel.GetJoinTableName(), true, transaction)
			}
			if rel.Relation == "has_many_and_belongs_to_many" {
				err = dbResource.TruncateTable(rel.GetJoinTableName(), true, transaction)
			}
			if rel.Relation == "has_one" {
				if rel.Subject == dbResource.tableInfo.TableName {
					// err = dbResource.TruncateTable(rel.Object, true)
				} else {
					err = dbResource.TruncateTable(rel.Object, true, transaction)
				}
			}

			CheckErr(err, "Failed to truncate related table before truncate table [%v] [%v]", typeName, rel)
			err = nil
		}
	}

	s, q, err := statementbuilder.Squirrel.Delete(typeName).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(s, q...)

	return err

}

// Update the data and set the values using the data map without an validation or transformations
// Invoked by data import action
func (dbResource *DbResource) DirectInsert(typeName string, data map[string]interface{}, transaction *sqlx.Tx) error {
	var err error

	columnMap := dbResource.Cruds[typeName].model.GetColumnMap()

	cols := make([]interface{}, 0)
	vals := make([]interface{}, 0)

	for columnName := range columnMap {
		colInfo, ok := dbResource.tableInfo.GetColumnByName(columnName)
		if !ok {
			log.Printf("No column named [%v]", columnName)
			continue
		}
		value := data[columnName]
		switch colInfo.ColumnType {
		case "datetime":
			if value != nil {
				valStr, ok := value.(string)
				if !ok {

				} else {

					value, err = dateparse.ParseLocal(valStr)
					if err != nil {
						log.Errorf("Failed to parse value as time, insert will fail [%v][%v]: %v", columnName, value, err)
						continue
					}
				}
			}
		}

		if columnName == "permission" {
			value = dbResource.tableInfo.DefaultPermission
		}

		cols = append(cols, columnName)
		vals = append(vals, value)

	}

	sqlString, args, err := statementbuilder.Squirrel.Insert(typeName).Prepared(true).Cols(cols...).Vals(vals).ToSQL()

	if err != nil {
		return err
	}

	_, err = transaction.Exec(sqlString, args...)
	if err != nil {
		log.Errorf("Failed SQL  [%v] [%v]", sqlString, args)
	}
	return err
}

// GetAllObjects Gets all rows from the table `typeName`
// Returns an array of Map object, each object has the column name to value mapping
// Utility method for loading all objects having low count
// Can be used by actions
func (dbResource *DbResource) GetAllObjects(typeName string, transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	s, q, err := statementbuilder.Squirrel.Select(goqu.L("*")).Prepared(true).From(typeName).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[2190] failed to prepare statment: %v", err)
		return nil, err
	}

	row, err := stmt1.Queryx(q...)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = row.Close()
	if err != nil {
		log.Errorf("[2203] failed to close result after value scan in defer")
	}
	err = stmt1.Close()
	if err != nil {
		log.Errorf("[2207] failed to close result after value scan in defer")
	}

	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetAllObjects ResultToArray: %v", duration)

	return m, err
}

// GetAllObjectsWithWhere Get all rows from the table `typeName`
// Returns an array of Map object, each object has the column name to value mapping
// Utility method for loading all objects having low count
// Can be used by actions
func (dbResource *DbResource) GetAllObjectsWithWhereWithTransaction(typeName string, transaction *sqlx.Tx, where ...goqu.Ex) ([]map[string]interface{}, error) {
	query := statementbuilder.Squirrel.Select(goqu.L("*")).Prepared(true).From(typeName)

	for _, w := range where {
		query = query.Where(w)
	}

	s, q, err := query.ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1336] failed to prepare statment [%v]: %v", s, err)
		if stmt1 != nil {
			err = stmt1.Close()
			CheckErr(err, "failed to close statement after prepare error")
		}
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row, err := stmt1.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1244] failed to close result after value scan in defer")
		}
	}(row)

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = row.Close()
	err = stmt1.Close()

	m, _, err := dbResource.Cruds[typeName].ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetAllObjectWhere ResultToArray: %v", duration)

	return m, err
}

// GetAllRawObjects Get all rows from the table `typeName` without any processing of the response
// expect no "__type" column on the returned instances
// Returns an array of Map object, each object has the column name to value mapping
// Utility method for loading all objects having low count
// Can be used by actions
func (dbResource *DbResource) GetAllRawObjectsByRawQuery(typeName string, query string, args []interface{}) ([]map[string]interface{}, error) {

	stmt1, err := dbResource.Connection().Preparex(query)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1376] failed to prepare statment [%v]: %v", query, err)
		return nil, err
	}

	row, err := stmt1.Queryx(args...)
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1279] failed to close result after value scan in defer")
		}
	}(row)

	if err != nil {
		return nil, err
	}

	m, err := RowsToMap(row, typeName)

	return m, err
}

// GetAllRawObjects Get all rows from the table `typeName` without any processing of the response
// expect no "__type" column on the returned instances
// Returns an array of Map object, each object has the column name to value mapping
// Utility method for loading all objects having low count
// Can be used by actions
func (dbResource *DbResource) GetAllRawObjects(typeName string) ([]map[string]interface{}, error) {
	s, q, err := statementbuilder.Squirrel.Select(goqu.L("*")).Prepared(true).From(typeName).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := dbResource.Connection().Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1376] failed to prepare statment [%v]: %v", s, err)
		return nil, err
	}

	row, err := stmt1.Queryx(q...)
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1279] failed to close result after value scan in defer")
		}
	}(row)

	if err != nil {
		return nil, err
	}

	m, err := RowsToMap(row, typeName)

	return m, err
}

func (dbResource *DbResource) GetAllRawObjectsWithTransaction(typeName string, transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	s, q, err := statementbuilder.Squirrel.Select(goqu.L("*")).Prepared(true).From(typeName).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1376] failed to prepare statment [%v]: %v", s, err)
		return nil, err
	}

	row, err := stmt1.Queryx(q...)
	defer func(row *sqlx.Rows) {
		err := row.Close()
		if err != nil {
			log.Errorf("[1279] failed to close result after value scan in defer")
		}
	}(row)

	if err != nil {
		return nil, err
	}

	m, err := RowsToMap(row, typeName)

	return m, err
}

// GetReferenceIdToObject Loads an object of type `typeName` using a reference_id
// Used internally, can be used by actions
func (dbResource *DbResource) GetReferenceIdToObjectWithTransaction(typeName string, referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (map[string]interface{}, error) {

	// cache is converting value types from int64 -> float64

	//cacheKey := fmt.Sprintf("rio-%v-%v", typeName, referenceId)
	//if OlricCache != nil {
	//	cachedMarshaledValue, err := OlricCache.Get(cacheKey)
	//	if err == nil && cachedMarshaledValue != nil {
	//		var cachedResult map[string]interface{}
	//		err := json.Unmarshal(cachedMarshaledValue.([]byte), &cachedResult)
	//		CheckErr(err, "Failed to unmarshal cached result")
	//		if err == nil {
	//			return cachedResult, nil
	//		}
	//	}
	//}

	//log.Printf("Get Object by reference id [%v][%v]", typeName, referenceId)
	s, q, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	if err != nil {
		log.Errorf("[1423] failed to prepare statment - [%v]: %v", s, err)
		return nil, err
	}

	//log.Printf("Get object by reference id sql: %v", s)
	row, err := stmt1.Queryx(q...)
	defer func() {
		err = row.Close()
		CheckErr(err, "[1314] Failed to close row after querying single row")
	}()

	if err != nil {
		return nil, err
	}

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt1.Close()
	err = row.Close()

	results, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetReferenceIdToObject ResultToArray: %v", duration)

	if err != nil {
		return nil, err
	}

	//log.Printf("Have to return first of %d results", len(results))
	if len(results) == 0 {
		return nil, fmt.Errorf("no such object 1161 [%v][%v]", typeName, referenceId)
	}
	//if OlricCache != nil {
	//	marshalledResult, err := json.Marshal(results[0])
	//	CheckErr(err, "Failed to marshal result to cache")
	//	if err == nil {
	//		err = OlricCache.Put(cacheKey, marshalledResult, olric.EX(5*time.Second), olric.NX())
	//		CheckErr(err, "[2552] Failed to set reference id to object id in olric cache")
	//	}
	//}

	return results[0], err
}

// GetReferenceIdToObjectColumn Loads an object of type `typeName` using a reference_id
// Used internally, can be used by actions
func (dbResource *DbResource) GetReferenceIdToObjectColumnWithTransaction(typeName string, referenceId daptinid.DaptinReferenceId,
	columnToSelect string, transaction *sqlx.Tx) (interface{}, error) {
	//log.Printf("Get Object by reference id [%v][%v]", typeName, referenceId)
	s, q, err := statementbuilder.Squirrel.Select(columnToSelect).Prepared(true).
		Prepared(true).From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		return nil, err
	}

	//log.Printf("Get object by reference id sql: %v", s)

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1473] failed to prepare statment for get object by reference id[%s][%s]: %v", typeName, referenceId, err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement[%s][%s]: %v", typeName, referenceId, err)
		}
	}(stmt)

	row, err := stmt.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func() {
		err = row.Close()
		CheckErr(err, "Failed to close row after querying single row")
	}()

	start := time.Now()
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt.Close()
	err = row.Close()

	results, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds[typeName].model.GetColumnMap(), nil, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetReferenceIdToColumn ResultToArray: %v", duration)

	if err != nil {
		return nil, err
	}

	//log.Printf("Have to return first of %d results", len(results))
	if len(results) == 0 {
		return nil, fmt.Errorf("no such object 1197 [%v][%v]", typeName, referenceId)
	}

	return results[0][columnToSelect], err
}

// Load rows from the database of `typeName` with a where clause to filter rows
// Converts the queries to sql and run query with where clause
// Returns list of reference_ids
func (dbResource *DbResource) GetReferenceIdByWhereClause(typeName string, queries ...goqu.Ex) ([]daptinid.DaptinReferenceId, error) {
	builder := statementbuilder.Squirrel.Select("reference_id").Prepared(true).From(typeName)

	for _, qu := range queries {
		builder = builder.Where(qu)
	}

	s, q, err := builder.ToSQL()
	//log.Debugf("reference id by where query: %v", s)

	if err != nil {
		return nil, err
	}

	stmt, err := dbResource.Connection().Preparex(s)
	if err != nil {
		log.Errorf("[1525] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res, err := stmt.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("[1296] Failed to close rows after query")
		}
	}(res)

	ret := make([]daptinid.DaptinReferenceId, 0)
	var sRef daptinid.DaptinReferenceId
	for res.Next() {
		err := res.Scan(&sRef)
		if err != nil {
			log.Errorf("[1305] failed to scan result into variable")
			return nil, err
		}
		ret = append(ret, sRef)
	}

	return ret, err

}

// Load rows from the database of `typeName` with a where clause to filter rows
// Converts the queries to sql and run query with where clause
// Returns list of reference_ids
func GetReferenceIdByWhereClauseWithTransaction(typeName string, transaction *sqlx.Tx, queries ...goqu.Ex) ([]daptinid.DaptinReferenceId, error) {
	builder := statementbuilder.Squirrel.Select("reference_id").Prepared(true).From(typeName)

	for _, qu := range queries {
		builder = builder.Where(qu)
	}

	s, q, err := builder.ToSQL()
	//log.Debugf("reference id by where query: %v", s)

	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1525] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res, err := stmt.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("[1296] Failed to close rows after query")
		}
	}(res)

	ret := make([]daptinid.DaptinReferenceId, 0)
	var sRef daptinid.DaptinReferenceId
	for res.Next() {
		err := res.Scan(&sRef)
		if err != nil {
			log.Errorf("[1305] failed to scan result into variable")
			return nil, err
		}
		ret = append(ret, sRef)
	}

	return ret, err

}

// GetIdByWhereClause Loads rows from the database of `typeName` with a where clause to filter rows
// Converts the queries to sql and run query with where clause
// Returns  list of internal database integer ids
func (dbResource *DbResource) GetIdByWhereClause(typeName string, transaction *sqlx.Tx, queries ...goqu.Ex) ([]int64, error) {
	builder := statementbuilder.Squirrel.Select("id").Prepared(true).From(typeName)

	for _, qu := range queries {
		builder = builder.Where(qu)
	}

	s, q, err := builder.ToSQL()
	//log.Debugf("reference id by where query: %v", s)

	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1581] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	res, err := stmt.Queryx(q...)

	if err != nil {
		return nil, err
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("[1454] failed to close rows after value scan in defer")
		}
	}(res)

	ret := make([]int64, 0)
	for res.Next() {
		var s int64
		err := res.Scan(&s)
		if err != nil {
			log.Errorf("[1463] failed to scan value after query")
			return nil, err
		}
		ret = append(ret, s)
	}

	return ret, err

}

// GetIdToReferenceId Looks up an integer id and return a string reference id of an object of type `typeName`
func (dbResource *DbResource) GetIdToReferenceId(typeName string, id int64, transaction *sqlx.Tx) (daptinid.DaptinReferenceId, error) {

	var idd daptinid.DaptinReferenceId
	k := fmt.Sprintf("itr-%v-%v", typeName, id)
	if OlricCache != nil {
		v, err := OlricCache.Get(context.Background(), k)
		if err == nil {
			var bytes [16]byte
			err = v.Scan(bytes)
			return bytes, err
		}
	}

	s, q, err := statementbuilder.Squirrel.Select("reference_id").
		Prepared(true).From(typeName).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return idd, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1636] failed to prepare statment: %v", err)
		return idd, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	var str daptinid.DaptinReferenceId
	row := stmt.QueryRowx(q...)
	err = row.Scan(&str)
	if OlricCache != nil {
		err1 := OlricCache.Put(context.Background(), k, str, olric.EX(60*time.Minute), olric.NX())
		CheckErr(err1, "[2856] Failed to set if to reference id in olric cache")
	}
	return str, err

}

// GetIdToReferenceIdWithTransaction Looks up an integer id and return a string reference id of an object of type `typeName`
func GetIdToReferenceIdWithTransaction(typeName string, id int64, transaction *sqlx.Tx) (daptinid.DaptinReferenceId, error) {

	k := fmt.Sprintf("itr-%v-%v", typeName, id)
	if OlricCache != nil {
		v, err := OlricCache.Get(context.Background(), k)
		if err == nil {
			var dri daptinid.DaptinReferenceId
			err := v.Scan(&dri)
			return dri, err
		}
	}

	s, q, err := statementbuilder.Squirrel.Select("reference_id").
		Prepared(true).From(typeName).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return [16]byte{}, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1636] failed to prepare statment: %v", err)
		return [16]byte{}, err
	}
	defer stmt.Close()
	var str []byte
	row := stmt.QueryRowx(q...)
	err = row.Scan(&str)
	if OlricCache != nil {
		OlricCache.Put(context.Background(), k, str, olric.EX(30*time.Minute), olric.NX())
		//CheckErr(cacheErr, "[2897] Failed to set id to reference id in olric cache")
	}
	if len(str) == 0 {
		return daptinid.NullReferenceId, err
	}
	return daptinid.DaptinReferenceId(str), err

}

// GetReferenceIdToId Lookup an string reference id and return a internal integer id of an object of type `typeName`
func (dbResource *DbResource) GetReferenceIdToId(typeName string, referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (int64, error) {

	cacheKey := fmt.Sprintf("riti-%v-%v", typeName, referenceId)

	if OlricCache != nil {

		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			return cachedValue.Int64()
		}
	}

	var id int64
	s, q, err := statementbuilder.Squirrel.Select("id").
		Prepared(true).From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		return 0, err
	}
	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1666] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	err = stmt.QueryRowx(q...).Scan(&id)

	if OlricCache != nil {
		cachePutErr := OlricCache.Put(context.Background(), cacheKey, id, olric.EX(30*time.Minute), olric.NX())
		CheckErr(cachePutErr, "failed to cache reference id to id for [%v][%v]", typeName, referenceId)
	}

	return id, err

}

// GetReferenceIdToIdWithTransaction Looks up a string reference id and return an internal integer id of an object of type `typeName`
func GetReferenceIdToIdWithTransaction(typeName string, referenceId daptinid.DaptinReferenceId, updateTransaction *sqlx.Tx) (int64, error) {

	cacheKey := fmt.Sprintf("riti-%v-%v", typeName, referenceId)

	if OlricCache != nil {

		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil && cachedValue != nil {
			return cachedValue.Int64()
		}
	}

	var id int64
	s, q, err := statementbuilder.Squirrel.Select("id").
		Prepared(true).From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		return 0, err
	}
	stmt, err := updateTransaction.Preparex(s)
	if err != nil {
		log.Errorf("[1666] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	err = stmt.QueryRowx(q...).Scan(&id)

	if OlricCache != nil && err == nil {
		cachePutErr := OlricCache.Put(context.Background(), cacheKey, id, olric.EX(1*time.Hour), olric.NX())
		CheckErr(cachePutErr, "failed to cache reference id to id for [%v][%v]", typeName, referenceId)
	}

	return id, err

}

// Lookup an string reference id and return a internal integer id of an object of type `typeName`
func (dbResource *DbResource) GetReferenceIdListToIdList(typeName string, referenceId []string) (map[string]int64, error) {

	idMap := make(map[string]int64)
	s, q, err := statementbuilder.Squirrel.Select("id", "reference_id").Prepared(true).
		From(typeName).Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()
	if err != nil {
		return idMap, err
	}

	stmt1, err := dbResource.Connection().Preparex(s)
	if err != nil {
		log.Errorf("[1694] failed to prepare statment: %v", err)
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)
	if err != nil {
		return idMap, err
	}
	for rows.Next() {
		var id1 int64
		var id2 string
		err = rows.Scan(&id1, &id2)
		idMap[id2] = id1
	}

	return idMap, err
}

// Lookup an string reference id and return a internal integer id of an object of type `typeName`
func GetReferenceIdListToIdListWithTransaction(typeName string, referenceId []daptinid.DaptinReferenceId,
	transaction *sqlx.Tx) (map[daptinid.DaptinReferenceId]int64, error) {
	log.Tracef("GetReferenceIdListToIdListWithTransaction: [%v] => [%v]", typeName, referenceId)

	var refVals [][]byte

	for _, i := range referenceId {
		copied := make([]uint8, len(i))
		copy(copied, i[:])
		refVals = append(refVals, copied)
	}

	idMap := make(map[daptinid.DaptinReferenceId]int64)
	s, q, err := statementbuilder.Squirrel.Select("id", "reference_id").Prepared(true).
		From(typeName).Where(goqu.Ex{"reference_id": refVals}).ToSQL()
	if err != nil {
		return idMap, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1694] failed to prepare statment: %v", err)
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)
	if err != nil {
		return idMap, err
	}
	rowStr := make(map[string]interface{})
	err = rows.MapScan(rowStr)
	for rows.Next() {
		//var id1 int64
		//var id2 daptinid.DaptinReferenceId
		err = rows.MapScan(rowStr)
		idMap[daptinid.InterfaceToDIR(rowStr["reference_id"])] = rowStr["id"].(int64)
	}

	return idMap, err
}

type IdReferenceIdRow struct {
	reference_id daptinid.DaptinReferenceId `db:"reference_id"`
	id           int64                      `db:"id"`
}

// GetIdListToReferenceIdList Lookups an string internal integer id and return a reference id of an object of type `typeName`
func (dbResource *DbResource) GetIdListToReferenceIdList(typeName string, ids []int64, transaction *sqlx.Tx) (map[int64]string, error) {

	idMap := make(map[int64]string)
	s, q, err := statementbuilder.Squirrel.Select("reference_id", "id").Prepared(true).
		From(typeName).Where(goqu.Ex{"id": ids}).ToSQL()
	if err != nil {
		return idMap, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1731] failed to prepare statment: %v", err)
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(q...)
	if err != nil {
		return idMap, err
	}
	for rows.Next() {
		var id1 string
		var id2 int64
		err = rows.Scan(&id1, &id2)
		CheckErr(err, "[1581] failed to scan value after query: %v[%v]", typeName, ids)
		idMap[id2] = id1
	}

	return idMap, err
}

// GetSingleColumnValueByReferenceId select "column" from "typeName" where matchColumn in (values)
// returns list of values of the column
func (dbResource *DbResource) GetSingleColumnValueByReferenceId(
	typeName string, selectColumn []interface{}, matchColumn string, values []string) ([]interface{}, error) {

	s, q, err := statementbuilder.Squirrel.
		Select(selectColumn...).Prepared(true).From(typeName).Where(goqu.Ex{matchColumn: values}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt, err := dbResource.Connection().Preparex(s)
	if err != nil {
		log.Errorf("[1768] failed to prepare statment for permission select: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	rows, err := stmt.Queryx(q...)
	if err != nil {
		return nil, err
	}

	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[1483] failed to close result after value scan")
		}
	}(rows)
	returnValues := make([]interface{}, 0)

	for rows.Next() {
		var val interface{}
		err = rows.Scan(&val)
		if err != nil {
			log.Errorf("[1620] failed to scan value after query")
			break
		}
		returnValues = append(returnValues, val)
	}

	return returnValues, nil
}

// GetSingleColumnValueByReferenceId select "column" from "typeName" where matchColumn in (values)
// returns list of values of the column
func GetSingleColumnValueByReferenceIdWithTransaction(
	typeName string, selectColumn []interface{}, matchColumn string, values []string, transaction *sqlx.Tx) ([]interface{}, error) {
	log.Tracef("GetSingleColumnValueByReferenceIdWithTransaction: [%v] => [%v]", typeName, selectColumn)

	actualValues := make([]interface{}, 0)
	for _, val := range values {
		asUuid, err := uuid.Parse(val)
		if err != nil {
			return nil, err
		}
		actualValues = append(actualValues, asUuid[:])
	}
	s, q, err := statementbuilder.Squirrel.
		Select(selectColumn...).Prepared(true).From(typeName).Where(goqu.Ex{matchColumn: actualValues}).ToSQL()
	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[1768] failed to prepare statment for permission select: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	rows, err := stmt.Queryx(q...)
	if err != nil {
		return nil, err
	}

	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[1483] failed to close result after value scan")
		}
	}(rows)
	returnValues := make([]interface{}, 0)

	for rows.Next() {
		var val interface{}
		err = rows.Scan(&val)
		if err != nil {
			log.Errorf("[1620] failed to scan value after query")
			break
		}
		returnValues = append(returnValues, val)
	}

	return returnValues, nil
}

// RowsToMap converts the result of db.QueryRowx => rows to array of data
// can be used on any *sqlx.Rows and assign a typeName
func RowsToMap(rows *sqlx.Rows, typeName string) ([]map[string]interface{}, error) {

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	responseArray := make([]map[string]interface{}, 0)

	for rows.Next() {

		rc := NewMapStringScan(columns)
		err := rc.Update(rows)
		if err != nil {
			return responseArray, err
		}

		dbRow := rc.Get()
		dbRow["__type"] = typeName
		responseArray = append(responseArray, dbRow)
	}

	return responseArray, nil
}

// ResultToArrayOfMap converts the result of db.QueryRowx => rows to array of data
// fetches the related objects also
// expects columnMap to be fetched from rows
// check usage in exiting source for example
// includeRelationMap can be nil to include none or map[string]bool{"*": true} to include all relations
// can be used on any *sqlx.Rows
func (dbResource *DbResource) ResultToArrayOfMapWithTransaction(
	responseArray []map[string]interface{}, columnMap map[string]api2go.ColumnInfo,
	includedRelationMap map[string]bool, transaction *sqlx.Tx) ([]map[string]interface{}, [][]map[string]interface{}, error) {

	//finalArray := make([]map[string]interface{}, 0)
	if includedRelationMap == nil {
		includedRelationMap = make(map[string]bool)
	}

	var err error
	objectCache := make(map[string]interface{})
	referenceIdCache := make(map[string]daptinid.DaptinReferenceId)
	includes := make([][]map[string]interface{}, 0)

	for _, row := range responseArray {
		localInclude := make([]map[string]interface{}, 0)

		for key, val := range row {
			//log.Printf("Key: [%v] == %v", key, val)

			columnInfo, ok := columnMap[key]
			if !ok {
				continue
			}

			if val != nil && columnInfo.ColumnType == "datetime" {
				stringVal, ok := val.(string)
				if ok {
					parsedValue, _, err := fieldtypes.GetTime(stringVal)
					if err != nil {
						parsedValue, _, err := fieldtypes.GetDateTime(stringVal)
						if InfoErr(err, "Failed to parse date time from [%v]: %v", columnInfo.ColumnName, stringVal) {
							row[key] = nil
						} else {
							row[key] = parsedValue
						}
					} else {
						row[key] = parsedValue
					}
				}
			}

			if !columnInfo.IsForeignKey {
				continue
			}

			if val == "" || val == nil {
				continue
			}

			namespace := columnInfo.ForeignKeyData.Namespace
			//log.Printf("Resolve foreign key from [%v][%v][%v]", columnInfo.ForeignKeyData.DataSource, namespace, val)
			switch columnInfo.ForeignKeyData.DataSource {
			case "self":

				referenceIdInt, ok := val.(int64)
				if !ok {
					stringIntId := val.(string)
					referenceIdInt, err = strconv.ParseInt(stringIntId, 10, 64)
					CheckErr(err, "Failed to convert string id to int id")
				}
				cacheKey := fmt.Sprintf("%v-%v", namespace, referenceIdInt)
				objCached, ok := objectCache[cacheKey]
				if ok {
					localInclude = append(localInclude, objCached.(map[string]interface{}))
					continue
				}

				idCacheKey := fmt.Sprintf("%s_%d", namespace, referenceIdInt)
				refId, ok := referenceIdCache[idCacheKey]

				if !ok {
					start := time.Now()
					refId, err = GetIdToReferenceIdWithTransaction(namespace, referenceIdInt, transaction)
					duration := time.Since(start)
					log.Tracef("[TIMING] RowsToMap IdToReferenceId: %v", duration)

					referenceIdCache[idCacheKey] = refId
				}

				if err != nil {
					log.Errorf("Failed to get ref id for [%v][%v]: %v", namespace, val, err)
					continue
				}
				row[key] = refId

				if includedRelationMap != nil && (includedRelationMap[namespace] || includedRelationMap[columnInfo.ColumnName] || includedRelationMap["*"]) {
					start := time.Now()
					obj, err := dbResource.GetIdToObjectWithTransaction(namespace, referenceIdInt, transaction)
					if err != nil || obj == nil {
						return nil, nil, fmt.Errorf("failed to get related object [%v][%v][%v]", namespace, referenceIdInt, err)
					}
					duration := time.Since(start)
					log.Tracef("[TIMING] RowsToMap IdToObject: %v", duration)

					obj["__type"] = namespace

					localInclude = append(localInclude, obj)
				}

			case "cloud_store":
				referenceStorageInformation := val.(string)
				//log.Printf("Resolve files from cloud store: %v", referenceStorageInformation)
				foreignFilesList := make([]map[string]interface{}, 0)
				err := json.Unmarshal([]byte(referenceStorageInformation), &foreignFilesList)
				CheckErr(err, "Failed to obtain list of file information")
				if err != nil {
					continue
				}

				returnFileList := make([]map[string]interface{}, 0)

				for _, file := range foreignFilesList {

					if file["type"] == "x-crdt/yjs" && !includedRelationMap["x-crdt/yjs"] {
						continue
					}

					if file["path"] != nil && file["name"] != nil && len(file["path"].(string)) > 0 {
						file["src"] = file["path"].(string) + "/" + file["name"].(string)
					} else if file["name"] != nil {
						file["src"] = file["name"].(string)
					} else {
						log.Errorf("File entry is missing name and path [%v][%v]", dbResource.TableInfo().TableName, key)
					}
					returnFileList = append(returnFileList, file)
				}

				row[key] = returnFileList
				//log.Printf("set row[%v]  == %v", key, foreignFilesList)
				if includedRelationMap[columnInfo.ColumnName] || includedRelationMap["*"] {

					resolvedFilesList, err := dbResource.GetFileFromLocalCloudStore(dbResource.TableInfo().TableName, columnInfo.ColumnName, returnFileList)
					CheckErr(err, "Failed to resolve file from cloud store")
					row[key] = resolvedFilesList
					for _, file := range resolvedFilesList {
						file["__type"] = columnInfo.ColumnType
						localInclude = append(localInclude, file)
					}

				}
			default:
				log.Errorf("Undefined data source: %v", columnInfo.ForeignKeyData.DataSource)
				continue
			}

		}

		for _, relation := range dbResource.tableInfo.Relations {

			if !(includedRelationMap[relation.GetObjectName()] || includedRelationMap[relation.GetSubjectName()]) {
				continue
			}

			if relation.Subject == dbResource.tableInfo.TableName {
				// fetch objects

				switch relation.Relation {
				case "has_one":
					// nothing to do here
					break
				case "belongs_to":
					// nothing to do here
					break
				case "has_many":

					fallthrough
				case "has_many_and_belongs_to_many":
					dir := daptinid.InterfaceToDIR(row["reference_id"])
					query, args, err := statementbuilder.Squirrel.
						Select(goqu.I(relation.GetObjectName()+".id")).Prepared(true).
						From(goqu.T(relation.GetSubject()).As(relation.GetSubjectName())).
						Join(
							goqu.T(relation.GetJoinTableName()).As(relation.GetJoinTableName()),
							goqu.On(goqu.Ex{
								relation.GetJoinTableName() + "." + relation.GetSubjectName(): goqu.I(relation.GetSubjectName() + ".id"),
							}),
						).
						Join(
							goqu.T(relation.GetObject()).As(relation.GetObjectName()),
							goqu.On(goqu.Ex{
								fmt.Sprintf("%v.%v", relation.GetJoinTableName(), relation.GetObjectName()): goqu.I(relation.GetObjectName() + ".id"),
							}),
						).
						Where(goqu.Ex{
							relation.GetSubjectName() + ".reference_id": dir[:],
						}).Order(goqu.I(relation.GetJoinTableName() + ".created_at").Desc()).Limit(50).ToSQL()
					if err != nil {
						log.Printf("Failed to build query 1474: %v", err)
						return nil, nil, err
					}

					stmt1, err := transaction.Preparex(query)
					if err != nil {
						log.Errorf("[2023] failed to prepare statment: %v", err)
						return nil, nil, err
					}
					defer func(stmt1 *sqlx.Stmt) {
						err := stmt1.Close()
						if err != nil {
							log.Errorf("failed to close prepared statement: %v", err)
						}
					}(stmt1)

					rows, err := stmt1.Queryx(args...)
					if err != nil {
						log.Printf("Failed to query 1482: %v", err)
						return nil, nil, err
					}

					ids := make([]int64, 0)

					for rows.Next() {
						includeRow := int64(0)
						err = rows.Scan(&includeRow)
						if err != nil {
							log.Printf("[1857] failed to scan include row: %v", err)
							break
						}
						ids = append(ids, includeRow)
					}

					rows.Close()
					stmt1.Close()
					if len(ids) < 1 {
						continue
					}

					includes1, err := dbResource.Cruds[relation.GetObject()].GetAllObjectsWithWhereWithTransaction(relation.GetObject(), transaction, goqu.Ex{
						"id": ids,
					})

					_, ok := row[relation.GetObjectName()]
					if !ok {
						row[relation.GetObjectName()] = make([]string, 0)
					}

					for _, incl := range includes1 {
						row[relation.GetObjectName()] = append(row[relation.GetObjectName()].([]string), incl["reference_id"].(daptinid.DaptinReferenceId).String())
					}

					localInclude = append(localInclude, includes1...)

					break
				}

			} else {
				// fetch subjects

				switch relation.Relation {
				case "has_one":

					fallthrough
				case "belongs_to":

					asDir := daptinid.InterfaceToDIR(row["reference_id"])
					query, args, err := statementbuilder.Squirrel.
						Select(goqu.I(relation.GetSubjectName()+".id")).Prepared(true).
						From(goqu.T(relation.GetObject()).As(relation.GetObjectName())).
						Join(
							goqu.T(relation.GetSubject()).As(relation.GetSubjectName()),
							goqu.On(goqu.Ex{
								fmt.Sprintf("%v.%v", relation.GetSubjectName(), relation.GetObjectName()): goqu.I(relation.GetObjectName() + ".id"),
							}),
						).
						Where(goqu.Ex{
							relation.GetObjectName() + ".reference_id": asDir[:],
						}).Order(goqu.I(relation.GetSubjectName() + ".created_at").Desc()).Limit(50).ToSQL()

					if err != nil {
						log.Printf("Failed to build query 1533: %v", err)
					}

					stmt1, err := transaction.Preparex(query)
					if err != nil {
						log.Errorf("[2097] failed to prepare statment: %v", err)
						return nil, nil, err
					}
					defer func(stmt1 *sqlx.Stmt) {
						err := stmt1.Close()
						if err != nil {
							log.Errorf("failed to close prepared statement: %v", err)
						}
					}(stmt1)

					includedSubjectRow, err := stmt1.Queryx(args...)
					if err != nil {
						log.Printf("Failed to query 1538: %v", includedSubjectRow.Err())
						return nil, nil, err
					}
					includedSubjectId := []int64{}

					for includedSubjectRow.Next() {
						var subId int64
						err = includedSubjectRow.Scan(&subId)
						includedSubjectId = append(includedSubjectId, subId)
					}
					CheckErr(err, "[2133] failed to scan included subject id")
					err = includedSubjectRow.Close()
					CheckErr(err, "[2135] failed to close rows")
					stmt1.Close()

					if len(includedSubjectId) < 1 {
						continue
					}

					localSubjectInclude, err := dbResource.Cruds[relation.GetSubject()].GetAllObjectsWithWhereWithTransaction(relation.GetSubject(), transaction, goqu.Ex{
						"id": includedSubjectId,
					})
					CheckErr(err, "[1923] failed to get object by od")

					_, ok := row[relation.GetSubjectName()]
					if !ok {
						row[relation.GetSubjectName()] = make([]string, 0)
					}

					for _, incl := range localSubjectInclude {
						row[relation.GetSubjectName()] = append(row[relation.GetSubjectName()].([]string), incl["reference_id"].(string))
					}

					localInclude = append(localInclude, localSubjectInclude...)

					break
				case "has_many":

					fallthrough
				case "has_many_and_belongs_to_many":
					asDir := daptinid.InterfaceToDIR(row["reference_id"])
					query, args, err := statementbuilder.Squirrel.
						Select(goqu.I(relation.GetSubjectName()+".id")).Prepared(true).
						From(goqu.T(relation.GetObject()).As(relation.GetObjectName())).
						Join(
							goqu.T(relation.GetJoinTableName()).As(relation.GetJoinTableName()),
							goqu.On(goqu.Ex{
								relation.GetJoinTableName() + "." + relation.GetObjectName(): goqu.I(relation.GetObjectName() + ".id"),
							}),
						).
						Join(
							goqu.T(relation.GetSubject()).As(relation.GetSubjectName()),
							goqu.On(goqu.Ex{
								fmt.Sprintf("%v.%v", relation.GetJoinTableName(), relation.GetSubjectName()): goqu.I(relation.GetSubjectName() + ".id"),
							}),
						).
						Where(goqu.Ex{
							relation.GetObjectName() + ".reference_id": asDir[:],
						}).Order(goqu.I(relation.GetJoinTableName() + ".created_at").Desc()).Limit(50).ToSQL()
					if err != nil {
						log.Printf("Failed to build query 1474: %v", err)
					}

					stmt1, err := transaction.Preparex(query)
					if err != nil {
						log.Errorf("[2155] failed to prepare statment: %v", err)
						return nil, nil, err
					}
					defer func(stmt1 *sqlx.Stmt) {
						err := stmt1.Close()
						if err != nil {
							log.Errorf("failed to close prepared statement: %v", err)
						}
					}(stmt1)

					rows, err := stmt1.Queryx(args...)

					if err != nil {
						log.Printf("Failed to query 1482: %v", err)
						return nil, nil, err
					}

					ids := make([]int64, 0)

					for rows.Next() {
						includeRow := int64(0)
						err = rows.Scan(&includeRow)
						if err != nil {
							log.Printf("[1966] failed to scan include row: %v", err)
							return nil, nil, err
						}
						ids = append(ids, includeRow)
					}
					rows.Close()
					stmt1.Close()

					if len(ids) < 1 {
						continue
					}

					includes1, err := dbResource.Cruds[relation.GetSubject()].GetAllObjectsWithWhereWithTransaction(relation.GetSubject(), transaction, goqu.Ex{
						"id": ids,
					})
					if err != nil {
						log.Errorf("Failed to get objects by where clause: %v", err)
						return nil, nil, err
					}

					_, ok := row[relation.GetSubjectName()]
					if !ok {
						row[relation.GetSubjectName()] = make([]string, 0)
					}

					for _, incl := range includes1 {
						row[relation.GetSubjectName()] = append(row[relation.GetSubjectName()].([]string), incl["reference_id"].(daptinid.DaptinReferenceId).String())
					}

					localInclude = append(localInclude, includes1...)

					break
				}

			}

		}

		includes = append(includes, localInclude)

	}

	return responseArray, includes, nil
}

// convert the result of db.QueryRowx => rows to array of data
// can be used on any *sqlx.Rows and assign a typeName
// calls RowsToMap with the current model name
func (dbResource *DbResource) ResultToArrayOfMapRaw(rows *sqlx.Rows, columnMap map[string]api2go.ColumnInfo) ([]map[string]interface{}, error) {

	//finalArray := make([]map[string]interface{}, 0)

	responseArray, err := RowsToMap(rows, dbResource.model.GetName())
	if err != nil {
		return responseArray, err
	}

	return responseArray, nil
}

// resolve a file column from data in column to actual file on a cloud store
// returns a map containing the metadata of the file and the file contents as base64 encoded
// can be sent to browser to invoke downloading js and data urls
func (dbResource *DbResource) GetFileFromLocalCloudStore(tableName string, columnName string, filesList []map[string]interface{}) (resp []map[string]interface{}, err error) {

	assetFolder, ok := dbResource.AssetFolderCache[tableName][columnName]
	if !ok {
		return nil, fmt.Errorf("not a synced folder [%v][%v]", tableName, columnName)
	}

	for _, fileItem := range filesList {
		newFileItem := make(map[string]interface{})

		for key, val := range fileItem {
			newFileItem[key] = val
		}

		if fileItem["src"] == nil {
			log.Printf("file has no source: [%v][%v]", tableName, columnName)
			continue
		}

		filePath := fileItem["src"].(string)
		filePath = strings.ReplaceAll(filePath, "/", string(os.PathSeparator))
		if filePath[0] != os.PathSeparator {
			filePath = string(os.PathSeparator) + filePath
		}
		bytes, err := os.ReadFile(assetFolder.LocalSyncPath + filePath)
		CheckErr(err, "Failed to read file on storage [%v]: %v", assetFolder.LocalSyncPath, filePath)
		if err != nil {
			continue
		}
		newFileItem["reference_id"] = fileItem["name"]
		newFileItem["contents"] = base64.StdEncoding.EncodeToString(bytes)
		resp = append(resp, newFileItem)
	}
	return resp, nil
}
