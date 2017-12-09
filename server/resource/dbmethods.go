package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"reflect"
	"strconv"
	"strings"
)

func (dr *DbResource) IsUserActionAllowed(userReferenceId string, userGroups []auth.GroupPermission, typeName string, actionName string) bool {
	permission := dr.GetObjectPermissionByWhereClause("world", "table_name", typeName)
	return permission.CanExecute(userReferenceId, userGroups)

}

func (dr *DbResource) GetActionByName(typeName string, actionName string) (Action, error) {
	var a ActionRow

	err := dr.db.QueryRowx("select a.action_name as name, w.table_name as ontype, a.label, action_schema as action_schema, a.reference_id as referenceid from action a join world w on w.id = a.world_id where w.table_name = ? and a.action_name = ? limit 1", typeName, actionName).StructScan(&a)
	var action Action
	if err != nil {
		log.Errorf("Failed to scan action: %v", err)
		return action, err
	}

	err = json.Unmarshal([]byte(a.ActionSchema), &action)
	CheckError(err, "failed to unmarshal infields")

	action.Name = a.Name
	action.Label = a.Name
	action.ReferenceId = a.ReferenceId
	action.OnType = a.OnType

	return action, nil
}

func (dr *DbResource) GetActionsByType(typeName string) ([]Action, error) {
	action := make([]Action, 0)

	rows, err := dr.db.Queryx("select a.action_name as name, w.table_name as ontype, a.label, action_schema as action_schema,"+
			" a.instance_optional as instance_optional, a.reference_id as referenceid from action a"+
			" join world w on w.id = a.world_id"+
			" where w.table_name = ? ", typeName)
	if err != nil {
		log.Errorf("Failed to scan action: %v", err)
		return action, err
	}
	defer rows.Close()

	for rows.Next() {

		var act Action
		var a ActionRow
		err := rows.StructScan(&a)
		CheckErr(err, "Failed to struct scan action row")

		if len(a.Label) < 1 {
			continue
		}
		err = json.Unmarshal([]byte(a.ActionSchema), &act)
		CheckError(err, "failed to unmarshal infields")

		act.Name = a.Name
		act.Label = a.Label
		act.ReferenceId = a.ReferenceId
		act.OnType = a.OnType
		act.InstanceOptional = a.InstanceOptional

		action = append(action, act)

	}

	return action, nil
}

func CheckError(err error, msg string) {
	if err != nil {
		log.Errorf(msg+" : %v", err)
	}
}

func (dr *DbResource) GetActionPermissionByName(worldId int64, actionName string) (PermissionInstance, error) {

	refId, err := dr.GetReferenceIdByWhereClause("action", squirrel.Eq{"action_name": actionName}, squirrel.Eq{"world_id": worldId})
	if err != nil {
		return PermissionInstance{}, err
	}

	if refId == nil || len(refId) < 1 {
		return PermissionInstance{}, errors.New(fmt.Sprintf("Failed to find action [%v] on [%v]", actionName, worldId))
	}
	permissions := dr.GetObjectPermission("action", refId[0])

	return permissions, nil
}

func (dr *DbResource) GetObjectPermission(objectType string, referenceId string) PermissionInstance {

	var selectQuery string
	var queryParameters []interface{}
	var err error
	if objectType == "usergroup" {
		selectQuery, queryParameters, err = squirrel.
		Select("permission", "id").
				From(objectType).Where(squirrel.Eq{"reference_id": referenceId}).
				ToSql()
	} else {
		selectQuery, queryParameters, err = squirrel.
		Select("user_id", "permission", "id").
				From(objectType).Where(squirrel.Eq{"reference_id": referenceId}).
				ToSql()

	}

	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return PermissionInstance{
			"", []auth.GroupPermission{}, auth.NewPermission(auth.None, auth.None, auth.None),
		}
	}

	resultObject := make(map[string]interface{})
	err = dr.db.QueryRowx(selectQuery, queryParameters...).MapScan(resultObject)
	if err != nil {
		log.Errorf("Failed to scan permission 1: %v", err)
	}
	//log.Infof("permi map: %v", resultObject)
	var perm PermissionInstance
	if resultObject["user_id"] != nil {

		user, err := dr.GetIdToReferenceId("user", resultObject["user_id"].(int64))
		if err == nil {
			perm.UserId = user
		}

	}

	perm.UserGroupId = dr.GetObjectGroupsByObjectId(objectType, resultObject["id"].(int64))

	perm.Permission = auth.ParsePermission(resultObject["permission"].(int64))
	if err != nil {
		log.Errorf("Failed to scan permission 2: %v", err)
	}

	//log.Infof("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

func (dr *DbResource) GetObjectPermissionByWhereClause(objectType string, colName string, colValue string) PermissionInstance {
	var perm PermissionInstance
	s, q, err := squirrel.Select("user_id", "permission", "id").From(objectType).Where(squirrel.Eq{colName: colValue}).ToSql()
	if err != nil {
		log.Errorf("Failed to create sql: %v", err)
		return perm
	}

	m := make(map[string]interface{})
	err = dr.db.QueryRowx(s, q...).MapScan(m)

	if err != nil {
		log.Errorf("Failed to scan permission: %v", err)
		return perm
	}

	//log.Infof("permi map: %v", m)
	if m["user_id"] != nil {

		user, err := dr.GetIdToReferenceId("user", m["user_id"].(int64))
		if err == nil {
			perm.UserId = user
		}

	}

	perm.UserGroupId = dr.GetObjectGroupsByObjectId(objectType, m["id"].(int64))

	perm.Permission = auth.ParsePermission(m["permission"].(int64))

	//log.Infof("PermissionInstance for [%v]: %v", typeName, perm)
	return perm
}

//func (dr *DbResource) GetTablePermission(typeName string) (PermissionInstance) {
//
//  s, q, err := squirrel.Select("user_id", "permission", "id").From("world").Where(squirrel.Eq{"table_name": typeName}).ToSql()
//  if err != nil {
//    log.Errorf("Failed to create sql: %v", err)
//    return PermissionInstance{
//      "", []auth.GroupPermission{}, 0,
//    }
//  }
//
//  m := make(map[string]interface{})
//  err = dr.db.QueryRowx(s, q...).MapScan(m)
//  //log.Infof("permi map: %v", m)
//  var perm PermissionInstance
//  if m["user_id"] != nil {
//
//    user, err := dr.GetIdToReferenceId("user", m["user_id"].(int64))
//    if err == nil {
//      perm.UserId = user
//    }
//
//  }
//
//  perm.UserGroupId = dr.GetObjectGroups("world", m["id"])
//
//  perm.PermissionInstance = m["permission"].(int64)
//  if err != nil {
//    log.Errorf("Failed to scan permission: %v", err)
//  }
//
//  //log.Infof("PermissionInstance for [%v]: %v", typeName, perm)
//  return perm
//}

func (dr *DbResource) GetObjectUserGroupsByWhere(objType string, colName string, colvalue string) []auth.GroupPermission {

	s := make([]auth.GroupPermission, 0)

	rel := api2go.TableRelation{}
	rel.Subject = objType
	rel.SubjectName = objType + "_id"
	rel.Object = "usergroup"
	rel.ObjectName = "usergroup_id"
	rel.Relation = "has_many_and_belongs_to_many"

	//log.Infof("Join string: %v: ", rel.GetJoinString())

	sql := fmt.Sprintf("select usergroup.reference_id as referenceid, j1.permission from %s join %s  where %s.%s = ?", rel.Subject, rel.GetJoinString(), rel.Subject, colName)
	//log.Infof("Group select sql: %v", sql)
	res, err := dr.db.Queryx(sql, colvalue)
	if err != nil {
		log.Errorf("Failed to get object groups by where clause: %v", err)
		return s
	}
	defer res.Close()

	for res.Next() {
		var g auth.GroupPermission
		err = res.StructScan(&g)
		if err != nil {
			log.Errorf("Failed to scan group permission : %v", err)
		}
		s = append(s, g)
	}
	return s

}
func (dr *DbResource) GetObjectGroupsByObjectId(objType string, objectId int64) []auth.GroupPermission {
	s := make([]auth.GroupPermission, 0)

	if objType == "usergroup" {

		refId, err := dr.GetIdToReferenceId(objType, objectId)
		if err != nil {
			log.Infof("Failed to get id to reference id [%v][%v] == %v", objType, objectId, err)
			return s
		}
		s = append(s, auth.GroupPermission{
			ReferenceId: refId,
			Permission:  auth.ParsePermission(dr.cruds["usergroup"].model.GetDefaultPermission()),
		})
		return s
	}

	res, err := dr.db.Queryx(
		fmt.Sprintf("select ug.reference_id as referenceid, uug.permission "+
				"from usergroup ug "+
				"join %s_%s_id_has_usergroup_usergroup_id uug on uug.usergroup_id = ug.id and uug.%s_id = ?", objType, objType, objType), objectId)
	if err != nil {
		log.Errorf("Failed to query object group by object id [%v][%v] == %v", objType, objectId, err)
		return s
	}
	defer res.Close()

	for res.Next() {
		var g auth.GroupPermission
		err = res.StructScan(&g)
		if err != nil {
			log.Errorf("Failed to scan group permission : %v", err)
		}
		s = append(s, g)
	}
	return s

}

func (dbResource *DbResource) CanBecomeAdmin() bool {

	var count int

	err := dbResource.db.QueryRow("select count(*) from user where email != 'guest@cms.go'").Scan(&count)
	if err != nil {
		return false
	}

	return count < 2

}

func (dbResource *DbResource) BecomeAdmin(userId int64) bool {

	if !dbResource.CanBecomeAdmin() {
		return false
	}

	for _, crud := range dbResource.cruds {

		if crud.model.HasColumn("user_id") {
			q, v, err := squirrel.Update(crud.model.GetName()).
					Set("user_id", userId).
					Set("permission", auth.DEFAULT_PERMISSION).
					ToSql()
			if err != nil {
				log.Errorf("Failed to create query to update: %v == %v", crud.model.GetName(), err)
				continue
			}

			_, err = dbResource.db.Exec(q, v...)
			if err != nil {
				log.Errorf("	Failed to execute become admin update query: %v", err)
				continue
			}

		}

	}

	_, err := dbResource.db.Exec("update world set permission = ?, default_permission = ? where table_name not like '%_audit'",
		auth.DEFAULT_PERMISSION, auth.DEFAULT_PERMISSION)
	if err != nil {
		log.Errorf("Failed to update world permissions: %v", err)
	}

	_, err = dbResource.db.Exec("update world set permission = ?, default_permission = ? where table_name like '%_audit'",
		auth.NewPermission(auth.Create, auth.Create, auth.Create).IntValue(),
		auth.NewPermission(auth.Read, auth.Read, auth.Read).IntValue())
	if err != nil {
		log.Errorf("Failed to world update audit permissions: %v", err)
	}

	_, err = dbResource.db.Exec("update action set permission = ?", auth.NewPermission(auth.None, auth.Read|auth.Execute, auth.Create|auth.Execute).IntValue())
	_, err = dbResource.db.Exec("update action set permission = ? where action_name in ('signin')", auth.NewPermission(auth.Peek|auth.Execute, auth.Read|auth.Execute, auth.Create|auth.Execute).IntValue())

	if err != nil {
		log.Errorf("Failed to update audit permissions: %v", err)
	}

	return true
}

func (dr *DbResource) GetRowPermission(row map[string]interface{}) PermissionInstance {

	refId, ok := row["reference_id"]
	if !ok {
		refId = row["id"]
	}
	rowType := row["__type"].(string)

	var perm PermissionInstance

	if rowType != "usergroup" {
		if row["user_id"] != nil {
			uid, _ := row["user_id"].(string)
			perm.UserId = uid
		} else {
			row, _ = dr.GetReferenceIdToObject(rowType, refId.(string))
			u := row["user_id"]
			if u != nil {
				uid, _ := u.(string)
				perm.UserId = uid
			}
		}

	}

	loc := strings.Index(dr.cruds[rowType].model.GetName(), "_has_")
	//log.Infof("Location [%v]: %v", dr.model.GetName(), loc)
	if loc == -1 && dr.cruds[rowType].model.HasMany("usergroup") {

		perm.UserGroupId = dr.GetObjectUserGroupsByWhere(rowType, "reference_id", refId.(string))

	} else if rowType == "usergroup" {
		perm.UserGroupId = []auth.GroupPermission{
			{
				ReferenceId: refId.(string),
				Permission:  auth.ParsePermission(dr.cruds["usergroup"].model.GetDefaultPermission()),
			},
		}
	}
	if row["permission"] != nil {

		var err error
		i64, ok := row["permission"].(int64)
		if !ok {
			f64, ok := row["permission"].(float64)
			if !ok {
				i64, err = strconv.ParseInt(row["permission"].(string), 10, 64)
				//p, err := int64(row["permission"].(int))
				if err != nil {
					log.Errorf("Invalid cast :%v", err)
				}
			} else {
				i64 = int64(f64)
			}
		}

		perm.Permission = auth.ParsePermission(i64)
	}
	//log.Infof("Row permission: %v  ---------------- %v", perm, row)
	return perm
}

func (dr *DbResource) GetRowsByWhereClause(typeName string, where ...squirrel.Eq) ([]map[string]interface{}, [][]map[string]interface{}, error) {

	stmt := squirrel.Select("*").From(typeName)

	for _, w := range where {
		stmt = stmt.Where(w)
	}

	s, q, err := stmt.ToSql()

	//log.Infof("Select query: %v == [%v]", s, q)
	rows, err := dr.db.Queryx(s, q...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	m1, include, err := dr.ResultToArrayOfMap(rows, dr.cruds[typeName].model.GetColumnMap(), map[string]bool{"*": true})

	return m1, include, err

}

func (dr *DbResource) GetUserGroupIdByUserId(userId int64) uint64 {

	s, q, err := squirrel.Select("usergroup_id").From("user_user_id_has_usergroup_usergroup_id").Where(squirrel.NotEq{"usergroup_id": 1}).Where(squirrel.Eq{"user_id": userId}).OrderBy("created_at").Limit(1).ToSql()
	if err != nil {
		log.Errorf("Failed to create sql query: %v", err)
		return 0
	}

	var refId uint64

	err = dr.db.QueryRowx(s, q...).Scan(&refId)
	if err != nil {
		log.Errorf("Failed to scan user group id from the result: %v", err)
	}

	return refId

}

func (dr *DbResource) GetSingleRowByReferenceId(typeName string, referenceId string) (map[string]interface{}, []map[string]interface{}, error) {

	s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).ToSql()
	if err != nil {
		log.Errorf("Failed to create select query by ref id: %v", referenceId)
		return nil, nil, err
	}

	rows, err := dr.db.Queryx(s, q...)
	defer rows.Close()
	resultRows, includeRows, err := dr.ResultToArrayOfMap(rows, dr.cruds[typeName].model.GetColumnMap(), map[string]bool{"*": true})
	if err != nil {
		return nil, nil, err
	}

	if len(resultRows) < 1 {
		return nil, nil, errors.New("No such entity")
	}

	m := resultRows[0]
	n := includeRows[0]

	return m, n, err

}

func (dr *DbResource) GetIdToObject(typeName string, id int64) (map[string]interface{}, error) {
	s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	m, _, err := dr.ResultToArrayOfMap(row, dr.cruds[typeName].model.GetColumnMap(), nil)

	if len(m) == 0 {
		log.Infof("No result found for [%v][%v]", typeName, id)
		return nil, err
	}

	return m[0], err
}

func (dr *DbResource) TruncateTable(typeName string) error {

	s, q, err := squirrel.Delete(typeName).ToSql()
	if err != nil {
		return err
	}

	_, err = dr.db.Exec(s, q...)
	return err

}

func (dr *DbResource) DirectInsert(typeName string, data map[string]interface{}) error {

	columnMap := dr.cruds[typeName].model.GetColumnMap()

	cols := make([]string, 0)
	vals := make([]interface{}, 0)
	for columnName := range columnMap {

		cols = append(cols, columnName)
		vals = append(vals, data[columnName])

	}

	sqlString, args, err := squirrel.Insert(typeName).Columns(cols...).Values(vals...).ToSql()

	if err != nil {
		return err
	}

	_, err = dr.db.Exec(sqlString, args...)
	return err
}

func (dr *DbResource) GetAllObjects(typeName string) ([]map[string]interface{}, error) {
	s, q, err := squirrel.Select("*").From(typeName).ToSql()
	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	m, _, err := dr.ResultToArrayOfMap(row, dr.cruds[typeName].model.GetColumnMap(), nil)

	return m, err
}
func (dr *DbResource) GetAllRawObjects(typeName string) ([]map[string]interface{}, error) {
	s, q, err := squirrel.Select("*").From(typeName).ToSql()
	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	m, err := RowsToMap(row, typeName)

	return m, err
}

func (dr *DbResource) GetReferenceIdToObject(typeName string, referenceId string) (map[string]interface{}, error) {
	//log.Infof("Get Object by reference id [%v][%v]", typeName, referenceId)
	s, q, err := squirrel.Select("*").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).ToSql()
	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	//cols, err := row.Columns()
	//if err != nil {
	//  return nil, err
	//}

	results, _, err := dr.ResultToArrayOfMap(row, dr.cruds[typeName].model.GetColumnMap(), nil)
	if err != nil {
		return nil, err
	}

	//log.Infof("Have to return first of %d results", len(results))
	if len(results) == 0 {
		return nil, fmt.Errorf("No such object [%v][%v]", typeName, referenceId)
	}

	return results[0], err
}

func (dr *DbResource) GetReferenceIdByWhereClause(typeName string, queries ...squirrel.Eq) ([]string, error) {
	builder := squirrel.Select("reference_id").From(typeName)

	for _, qu := range queries {
		builder = builder.Where(qu)
	}

	s, q, err := builder.ToSql()
	log.Debugf("reference id by where query: %v", s)

	if err != nil {
		return nil, err
	}

	res, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer res.Close()

	ret := make([]string, 0)
	for res.Next() {
		var s string
		res.Scan(&s)
		ret = append(ret, s)
	}

	return ret, err

}

func (dr *DbResource) GetIdByWhereClause(typeName string, queries ...squirrel.Eq) ([]int64, error) {
	builder := squirrel.Select("id").From(typeName)

	for _, qu := range queries {
		builder = builder.Where(qu)
	}

	s, q, err := builder.ToSql()
	log.Debugf("reference id by where query: %v", s)

	if err != nil {
		return nil, err
	}

	res, err := dr.db.Queryx(s, q...)

	if err != nil {
		return nil, err
	}
	defer res.Close()

	ret := make([]int64, 0)
	for res.Next() {
		var s int64
		res.Scan(&s)
		ret = append(ret, s)
	}

	return ret, err

}

func (dr *DbResource) GetIdToReferenceId(typeName string, id int64) (string, error) {

	s, q, err := squirrel.Select("reference_id").From(typeName).Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return "", err
	}

	var str string
	err = dr.db.QueryRowx(s, q...).Scan(&str)
	return str, err

}

func (dr *DbResource) GetReferenceIdToId(typeName string, referenceId string) (uint64, error) {

	var id uint64
	s, q, err := squirrel.Select("id").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).ToSql()
	if err != nil {
		return 0, err
	}

	err = dr.db.QueryRowx(s, q...).Scan(&id)
	return id, err

}

func (dr *DbResource) GetSingleColumnValueByReferenceId(typeName string, selectColumn, matchColumn string, values []string) ([]interface{}, error) {

	s, q, err := squirrel.Select(selectColumn).From(typeName).Where(squirrel.Eq{matchColumn: values}).ToSql()
	if err != nil {
		return nil, err
	}

	rows := dr.db.QueryRowx(s, q...)
	return rows.SliceScan()
}

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

func (dr *DbResource) ResultToArrayOfMap(rows *sqlx.Rows, columnMap map[string]api2go.ColumnInfo, includedRelationMap map[string]bool) ([]map[string]interface{}, [][]map[string]interface{}, error) {

	//finalArray := make([]map[string]interface{}, 0)

	responseArray, err := RowsToMap(rows, dr.model.GetName())
	if err != nil {
		return responseArray, nil, err
	}

	includes := make([][]map[string]interface{}, 0)

	for _, row := range responseArray {
		localInclude := make([]map[string]interface{}, 0)

		for key, val := range row {
			//log.Infof("Key: [%v] == %v", key, val)

			columnInfo, ok := columnMap[key]
			if !ok {
				//log.Infof("Column not found: %v", key)
				continue
			}

			if !columnInfo.IsForeignKey {
				continue
			}

			if val == "" || val == nil {
				continue
			}

			typeName := columnInfo.ForeignKeyData.TableName
			i, ok := val.(int64)
			if !ok {

				si, ok := val.(string)
				if ok {
					i, err = strconv.ParseInt(si, 10, 64)
					if err != nil {
						log.Errorf("Failed to convert [%v] to int", si)
						continue
					}
				} else {
					log.Errorf("Id should have been integer [%v]: %v", val, reflect.TypeOf(val))
					continue
				}
			}

			refId, err := dr.GetIdToReferenceId(typeName, i)

			row[key] = refId
			if err != nil {
				log.Errorf("Failed to get ref id for [%v][%v]: %v", typeName, val, err)
				continue
			}

			if includedRelationMap != nil && (includedRelationMap[typeName] || includedRelationMap["*"]) {
				obj, err := dr.GetIdToObject(typeName, i)
				obj["__type"] = typeName

				if err != nil {
					log.Errorf("Failed to get ref object for [%v][%v]: %v", typeName, val, err)
				} else {
					localInclude = append(localInclude, obj)
				}
			}

			//delete(responseArray[i], "id")

		}

		includes = append(includes, localInclude)
		//finalArray = append(finalArray, row)

	}

	return responseArray, includes, nil
}

func (dr *DbResource) ResultToArrayOfMapRaw(rows *sqlx.Rows, columnMap map[string]api2go.ColumnInfo) ([]map[string]interface{}, error) {

	//finalArray := make([]map[string]interface{}, 0)

	responseArray, err := RowsToMap(rows, dr.model.GetName())
	if err != nil {
		return responseArray, err
	}

	return responseArray, nil
}
