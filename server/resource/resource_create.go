package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	//"reflect"
	"github.com/artpar/go.uuid"
	//"strconv"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/daptin/daptin/server/auth"
	"github.com/pkg/errors"
	"time"
)

// Create a new object. Newly created object/struct must be in Responder.
// Possible Responder status codes are:
// - 201 Created: Resource was created and needs to be returned
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Resource created with a client generated ID, and no fields were modified by
//   the server

func (dr *DbResource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	data := obj.(*api2go.Api2GoModel)
	//log.Infof("Create object request: [%v] %v", dr.model.GetTableName(), data.Data)

	for _, bf := range dr.ms.BeforeCreate {
		//log.Infof("Invoke BeforeCreate [%v][%v] on Create Request", bf.String(), dr.model.GetName())
		data.Data["__type"] = dr.model.GetName()
		responseData, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{data.Data})
		if err != nil {
			log.Warnf("Error from before create middleware [%v]: %v", bf.String(), err)
			return nil, err
		}
		if responseData == nil {
			return nil, errors.New("No object to act upon")
		}
	}

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)

	}

	attrs := data.GetAllAsAttributes()

	allColumns := dr.model.GetColumns()

	dataToInsert := make(map[string]interface{})

	colsList := []string{}
	valsList := []interface{}{}
	for _, col := range allColumns {

		//log.Infof("Add column: %v", col.ColumnName)
		if col.IsAutoIncrement {
			continue
		}

		if col.ColumnName == "created_at" {
			continue
		}

		if col.ColumnName == "reference_id" {
			continue
		}

		if col.ColumnName == "updated_at" {
			continue
		}

		if col.ColumnName == "permission" {
			continue
		}

		if col.ColumnName == "user_id" && dr.model.GetName() != "user_user_id_has_usergroup_usergroup_id" {
			continue
		}

		//log.Infof("Check column: %v", col.ColumnName)

		val, ok := attrs[col.ColumnName]

		if !ok || val == nil {
			if col.DefaultValue != "" {
				val = col.DefaultValue
			} else {
				continue
			}
		}

		if col.IsForeignKey {
			//log.Infof("Convert reference id to id %v[%v]", col.ForeignKeyData.TableName, val)
			valString := val.(string)
			var uId interface{}
			var err error
			if valString == "" {
				uId = nil
			} else {
				foreignObject, err := dr.GetReferenceIdToObject(col.ForeignKeyData.Namespace, valString)
				if err != nil {
					return nil, err
				}

				foreignObjectPermission := dr.GetObjectPermission(col.ForeignKeyData.Namespace, valString)

				if foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
					uId = foreignObject["id"]
				} else {
					ok = false
				}

			}
			if err != nil {
				return nil, err
			}
			val = uId
		}
		var err error

		if col.ColumnType == "password" {
			val, err = BcryptHashString(val.(string))
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				val = ""
			}
		}

		if col.ColumnType == "datetime" {

			// 2017-07-13T18:30:00.000Z
			valString, ok := val.(string)
			if ok {
				val, err = dateparse.ParseLocal(valString)

				CheckErr(err, fmt.Sprintf("Failed to parse string as date time [%v]", val))
			} else {
				floatVal, ok := val.(float64)
				if ok {
					val = time.Unix(int64(floatVal), 0)
					err = nil
				}
			}

		} else if col.ColumnType == "date" {

			parsedTime, ok := val.(time.Time)
			if !ok {

				valString, ok := val.(string)
				if ok {
					val, err = dateparse.ParseLocal(valString)
					InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", val))
				} else {
					floatVal, ok := val.(float64)
					if ok {
						val = time.Unix(int64(floatVal), 0)
					}
				}

			} else {
				val = parsedTime
			}

		} else if col.ColumnType == "time" {

			// 2017-07-13T18:30:00.000Z
			valString, ok := val.(string)
			if ok {
				val, err = time.Parse("15:04:05", valString)

				CheckErr(err, fmt.Sprintf("Failed to parse string as time [%v]", val))
			} else {

				floatVal, ok := val.(float64)
				if ok {
					val = time.Unix(int64(floatVal), 0)
					err = nil
				}
			}

		}

		if col.ColumnType == "measurement" {
			if val == "" {
				continue
			}
		}

		if col.ColumnType == "encrypted" {

			secret, err := dr.configStore.GetConfigValueFor("encryption.secret", "backend")
			if err != nil {
				log.Error("Failed to get secret from config: %v", err)
				val = ""
			} else {
				val, err = Encrypt([]byte(secret), val.(string))
				if err != nil {
					log.Errorf("Failed to convert string to encrypted value, not storing the value: %v", err)
					val = ""
				}
			}
		}

		dataToInsert[col.ColumnName] = val
		colsList = append(colsList, col.ColumnName)
		valsList = append(valsList, val)
	}

	//for _, rel := range dr.model.GetRelations() {
	//  if rel.Relation == "belongs_to" || rel.Relation == "has_one" {
	//
	//    log.Infof("Relations : %v == %v", rel.Object, attrs)
	//    val, ok := attrs[rel.Object + "_id"]
	//    if ok {
	//      colsList = append(colsList, rel.Object + "_id")
	//      valsList = append(valsList, val)
	//    }
	//
	//  }
	//}
	u, _ := uuid.NewV4()
	newUuid := u.String()

	colsList = append(colsList, "reference_id")
	valsList = append(valsList, newUuid)

	colsList = append(colsList, "permission")
	valsList = append(valsList, dr.model.GetDefaultPermission())

	colsList = append(colsList, "created_at")
	valsList = append(valsList, time.Now())

	if sessionUser.UserId != 0 && dr.model.HasColumn("user_id") && dr.model.GetName() != "user_user_id_has_usergroup_usergroup_id" {

		colsList = append(colsList, "user_id")
		valsList = append(valsList, sessionUser.UserId)
	}

	query, vals, err := squirrel.Insert(dr.model.GetName()).Columns(colsList...).Values(valsList...).ToSql()
	if err != nil {
		log.Errorf("Failed to create insert query: %v", err)
		return NewResponse(nil, nil, 500, nil), err
	}

	log.Infof("Insert query: %v", query)
	_, err = dr.db.Exec(query, vals...)
	if err != nil {
		log.Errorf("Failed to execute insert query: %v", err)
		return NewResponse(nil, nil, 500, nil), err
	}

	createdResource, err := dr.GetReferenceIdToObject(dr.model.GetName(), newUuid)
	if err != nil {
		log.Errorf("Failed to select the newly created entry: %v", err)
		return nil, err
	}
	//

	//log.Infof("Created entry: %v", createdResource)

	userGroupId := dr.GetUserGroupIdByUserId(sessionUser.UserId)

	groupsToAdd := dr.defaultGroups
	log.Infof("Default groups to add object to: %v", groupsToAdd)
	for _, groupId := range groupsToAdd {
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := squirrel.
			Insert(dr.model.GetName() + "_" + dr.model.GetName() + "_id" + "_has_usergroup_usergroup_id").
			Columns(dr.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
			Values(createdResource["id"], groupId, nuuid, auth.DEFAULT_PERMISSION).ToSql()

		log.Infof("Query for default group belonging: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user group relation for [%v]: %v", dr.model.GetName(), err)
		}
	}

	if userGroupId != 0 && dr.model.HasMany("usergroup") {

		log.Infof("Associate new entity [%v][%v] with usergroup: %v", dr.model.GetTableName(), createdResource["reference_id"], userGroupId)
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := squirrel.
			Insert(dr.model.GetName() + "_" + dr.model.GetName() + "_id" + "_has_usergroup_usergroup_id").
			Columns(dr.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
			Values(createdResource["id"], userGroupId, nuuid, auth.DEFAULT_PERMISSION).ToSql()

		log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user group relation for [%v]: %v", dr.model.GetName(), err)
		}

	} else if dr.model.GetName() == "usergroup" && sessionUser.UserId != 0 {

		log.Infof("Associate new usergroup with user: %v", sessionUser.UserId)
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := squirrel.
			Insert("user_user_id_has_usergroup_usergroup_id").
			Columns("user_id", "usergroup_id", "reference_id", "permission").
			Values(sessionUser.UserId, createdResource["id"], nuuid, auth.DEFAULT_PERMISSION).ToSql()
		log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dr.model.GetName(), err)
		}

	} else if dr.model.GetName() == "user" {

		adminUserId, _ := GetAdminUserIdAndUserGroupId(dr.db)
		log.Infof("Associate new user with user: %v", adminUserId)

		belogsToUserGroupSql, q, err := squirrel.
			Update("user").
			Set("user_id", adminUserId).
			Where(squirrel.Eq{"id": createdResource["id"]}).ToSql()

		log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dr.model.GetName(), err)
		}

	}

	delete(createdResource, "id")
	createdResource["__type"] = dr.model.GetName()

	for _, bf := range dr.ms.AfterCreate {
		//log.Infof("Invoke AfterCreate [%v][%v] on Create Request", bf.String(), dr.model.GetName())
		results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{createdResource})
		if err != nil {
			log.Errorf("Error from after create middleware: %v", err)
		}
		if len(results) < 1 {
			createdResource = nil
		} else {
			createdResource = results[0]
		}
	}

	//for k, v := range createdResource {
	//  k1 := reflect.TypeOf(v)
	//  //log.Infof("K: %v", k1)
	//  if v != nil && k1.Kind() == reflect.Slice {
	//    createdResource[k] = string(v.([]uint8))
	//  }
	//}
	n1 := dr.model.GetName()
	c1 := dr.model.GetColumns()
	p1 := dr.model.GetDefaultPermission()
	r1 := dr.model.GetRelations()
	return NewResponse(nil,
		api2go.NewApi2GoModelWithData(n1, c1, p1, r1, createdResource),
		201, nil,
	), nil

}
