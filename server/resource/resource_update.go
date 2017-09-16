package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	//"reflect"
	"errors"
	"github.com/artpar/goms/server/auth"
	"gopkg.in/Masterminds/squirrel.v1"
	"net/http"
	"time"
	"fmt"
)

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dr *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	data, ok := obj.(*api2go.Api2GoModel)
	log.Infof("Update object request: [%v]", dr.model.GetTableName(), data.GetID())

	for _, bf := range dr.ms.BeforeUpdate {
		//log.Infof("Invoke BeforeUpdate [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

		finalData, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{
			data.Data,
		})
		if err != nil {
			log.Errorf("Erroset attributes from BeforeUpdate middleware: %v", err)
			return nil, err
		}
		if len(finalData) == 0 {
			return nil, errors.New("Failed to updated this object")
		}
		res := finalData[0]
		data.Data = res
	}

	if !ok {
		log.Errorf("Request data is not api2go model: %v", data)
		return nil, errors.New("Invalid request")
	}
	id := data.GetID()

	v := req.PlainRequest.Context().Value("user_id")

	currentUserReferenceId := ""
	if v != nil {
		currentUserReferenceId = v.(string)
	}

	currentUsergroups := make([]auth.GroupPermission, 0)

	t := req.PlainRequest.Context().Value("usergroup_id")
	if t != nil {
		currentUsergroups = t.([]auth.GroupPermission)
	}
	attrs := data.GetAllAsAttributes()

	if !data.HasVersion() {
		originalData, err := dr.GetReferenceIdToObject(dr.model.GetTableName(), id)
		if err != nil {
			return nil, err
		}
		data = api2go.NewApi2GoModelWithData(dr.model.GetTableName(), nil, 0, nil, originalData)
		data.SetAttributes(attrs)
	}

	allChanges := data.GetChanges()
	allColumns := dr.model.GetColumns()
	log.Infof("Update object request with changes: %v", allChanges)

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

		if col.ColumnName == "deleted_at" {
			continue
		}

		if col.ColumnName == "reference_id" {
			continue
		}

		if col.ColumnName == "updated_at" {
			continue
		}

		if col.ColumnName == "version" {
			continue
		}

		change, ok := allChanges[col.ColumnName]
		if !ok {
			continue
		}

		//log.Infof("Check column: [%v]  (%v) => (%v) ", col.ColumnName, change.OldValue, change.NewValue)

		var val interface{}
		val = change.NewValue
		if col.IsForeignKey {

			if val == "" {
				continue
			}

			log.Infof("Convert ref id to id %v[%v]", col.ForeignKeyData.TableName, val)

			valString := val.(string)

			foreignObject, err := dr.GetReferenceIdToObject(col.ForeignKeyData.TableName, valString)
			if err != nil {
				return nil, err
			}

			foreignObjectPermission := dr.GetObjectPermission(col.ForeignKeyData.TableName, valString)

			if foreignObjectPermission.CanWrite(currentUserReferenceId, currentUsergroups) {
				val = foreignObject["id"]
			} else {
				return nil, errors.New(fmt.Sprintf("No write permisssion on object [%v][%v]", col.ForeignKeyData.TableName, valString))
			}

		}
		var err error

		if col.ColumnType == "password" {
			val, err = BcryptHashString(val.(string))
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				continue
			}
		} else if col.ColumnType == "datetime" {
			parsedTime, ok := val.(time.Time)
			if !ok {
				val, err = time.Parse("2006-01-02T15:04:05.999Z", val.(string))
				CheckErr(err, fmt.Sprintf("Failed to parse string as date time [%v]", val))
			} else {
				val = parsedTime
			}
			// 2017-07-13T18:30:00.000Z

		} else if col.ColumnType == "encrypted" {

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
		} else if col.ColumnType == "date" {

			// 2017-07-13T18:30:00.000Z

			parsedTime, ok := val.(time.Time)
			if !ok {
				val1, err := time.Parse("2006-01-02T15:04:05.999Z", val.(string))

				InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", val))
				if err != nil {
					val, err = time.Parse("2006-01-02", val.(string))
					InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", val))
				} else {
					val = val1
				}
			} else {
				val = parsedTime
			}

		} else if col.ColumnType == "time" {
			parsedTime, ok := val.(time.Time)
			if !ok {
				val, err = time.Parse("15:04:05", val.(string))
				CheckErr(err, fmt.Sprintf("Failed to parse string as time [%v]", val))
			} else {
				val = parsedTime
			}
			// 2017-07-13T18:30:00.000Z

		}

		if ok {
			dataToInsert[col.ColumnName] = val
			colsList = append(colsList, col.ColumnName)
			valsList = append(valsList, val)
		}

	}

	colsList = append(colsList, "updated_at")
	valsList = append(valsList, time.Now())

	colsList = append(colsList, "version")
	valsList = append(valsList, data.GetNextVersion())

	builder := squirrel.Update(dr.model.GetName())

	for i := range colsList {
		//log.Infof("cols to set: %v == %v", colsList[i], valsList[i])
		builder = builder.Set(colsList[i], valsList[i])
	}

	query, vals, err := builder.Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
	if err != nil {
		log.Errorf("Failed to create update query: %v", err)
		return NewResponse(nil, nil, 500, nil), err
	}

	//log.Infof("Update query: %v == %v", query, vals)
	_, err = dr.db.Exec(query, vals...)
	if err != nil {
		log.Errorf("Failed to execute update query: %v", err)
		return NewResponse(nil, nil, 500, nil), err
	}

	if data.IsDirty() {

		auditModel := data.GetAuditModel()
		log.Infof("Object [%v]%v has been changed, trying to audit in %v", data.GetTableName(), data.GetID(), auditModel.GetTableName())
		if auditModel.GetTableName() != "" {
			creator, ok := dr.cruds[auditModel.GetTableName()]
			if !ok {
				log.Errorf("No creator for audit type: %v", auditModel.GetTableName())
			} else {
				_, err := creator.Create(auditModel, req)
				if err != nil {
					log.Errorf("Failed to create audit entry: %v", err)
				} else {
					log.Infof("[%v][%v] Created audit record", auditModel.GetTableName(), data.GetID())
					//log.Infof("ReferenceId for change: %v", resp.Result())
				}
			}
		}

	} else {
		log.Infof("[%v][%v] Model was not dirty, not creating an audit row", data.GetTableName(), data.GetID())
	}

	//query, vals, err = squirrel.Select("*").From(dr.model.GetName()).Where(squirrel.Eq{"reference_id": id}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
	//if err != nil {
	//	log.Errorf("Failed to create select query: %v", err)
	//	return nil, err
	//}

	updatedResource, err := dr.GetReferenceIdToObject(dr.model.GetName(), id)
	if err != nil {
		log.Errorf("Failed to select the newly created entry: %v", err)
		return nil, err
	}

	for _, rel := range dr.model.GetRelations() {
		relationName := rel.GetRelation()
		//log.Infof("Check relation in Update: %v", rel.String())
		if rel.GetSubject() == dr.model.GetName() {

			if relationName == "belongs_to" || relationName == "has_one" {
				continue
			}

			val11, ok := attrs[rel.GetObjectName()]
			if !ok || len(val11.([]map[string]interface{})) < 1 {
				continue
			}
			log.Infof("Update object for relation on [%v] : [%v]", rel.GetObjectName(), val11)

			valueList := val11.([]map[string]interface{})
			switch relationName {
			case "has_one":
			case "belongs_to":
				break

			case "has_many_and_belongs_to_many":
			case "has_many":

				for _, item := range valueList {
					obj := make(map[string]interface{})
					obj[rel.GetObjectName()] = item[rel.GetObjectName()]
					obj[rel.GetSubjectName()] = updatedResource["reference_id"]

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, auth.DEFAULT_PERMISSION, nil, obj)

					_, err := dr.cruds[rel.GetJoinTableName()].Create(modl, req)
					if err != nil {
						log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
						continue
					}

				}

				break

			default:
				log.Errorf("Unknown relation: %v", relationName)
			}

		} else {

			val, ok := attrs[rel.GetSubjectName()]
			if !ok {
				continue
			}
			log.Infof("Update %v on: %v", rel.String(), val)

			//var relUpdateQuery string
			//var vars []interface{}
			switch relationName {
			case "has_one":
				intId := updatedResource["id"].(int64)
				log.Infof("Converted ids for [%v]: %v", rel.GetObject(), intId)

				valMapList := val.([]map[string]interface{})

				for _, valMap := range valMapList {

					updateForeignRow := make(map[string]interface{})

					updateForeignRow, err = dr.cruds[rel.GetSubject()].GetReferenceIdToObject(rel.GetSubject(), valMap[rel.GetSubjectName()].(string))
					if err != nil {
						log.Infof("Failed to get object by reference id: %v", err)
						continue
					}
					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, auth.DEFAULT_PERMISSION, nil, updateForeignRow)

					model.SetAttributes(map[string]interface{}{
						rel.GetObjectName(): updatedResource["reference_id"].(string),
					})

					_, err := dr.cruds[rel.GetSubject()].Update(model, req)
					if err != nil {
						log.Errorf("Failed to update [%v][%v]: %V", rel.GetObject(), )
					}
				}

				//relUpdateQuery, vars, err = squirrel.Update(rel.GetSubject()).
				//    Set(rel.GetObjectName(), intId).Where(squirrel.Eq{"reference_id": val}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()

				//if err != nil {
				//  log.Errorf("Failed to make update query: %v", err)
				//  continue
				//}

				//log.Infof("Relation update query params: %v", vars)

				break
			case "belongs_to":
				intId := updatedResource["id"].(int64)
				log.Infof("Converted ids for [%v]: %v", rel.GetObject(), intId)

				valMapList := val.([]map[string]interface{})

				for _, valMap := range valMapList {
					updateForeignRow := make(map[string]interface{})
					updateForeignRow, err = dr.GetReferenceIdToObject(rel.GetSubject(), valMap[rel.GetSubjectName()].(string))
					updateForeignRow[rel.GetSubjectName()] = updatedResource["reference_id"].(string)

					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, auth.DEFAULT_PERMISSION, nil, updateForeignRow)

					_, err := dr.cruds[rel.GetSubject()].Update(model, req)
					if err != nil {
						log.Errorf("Failed to update [%v][%v]: %V", rel.GetObject(), )
					}
				}

				break

			case "has_many":
				values := val.([]map[string]interface{})

				for _, obj := range values {

					updateObject := make(map[string]interface{})
					updateObject[rel.GetSubjectName()] = obj[rel.GetSubjectName()]
					updateObject[rel.GetObjectName()] = updatedResource["reference_id"].(string)

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, auth.DEFAULT_PERMISSION, nil, updateObject)

					pre := &http.Request{
						Method: "POST",
					}
					pre = pre.WithContext(req.PlainRequest.Context())
					req1 := api2go.Request{
						PlainRequest: pre,
					}

					_, err := dr.cruds[rel.GetJoinTableName()].Create(modl, req1)

					if err != nil {
						log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
						continue
					}
				}
				break;

			case "has_many_and_belongs_to_many":
				values := val.([]map[string]interface{})

				for _, obj := range values {
					obj[rel.GetSubjectName()] = val
					obj[rel.GetObjectName()] = updatedResource["id"]

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, auth.DEFAULT_PERMISSION, nil, obj)
					pre := &http.Request{
						Method: "POST",
					}
					pre = pre.WithContext(req.PlainRequest.Context())
					req1 := api2go.Request{
						PlainRequest: pre,
					}
					_, err := dr.cruds[rel.GetJoinTableName()].Create(modl, req1)

					if err != nil {
						log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
						continue
					}
				}
				break

			default:
				log.Errorf("Unknown relation: %v", relationName)
			}

			//_, err = dr.db.Exec(relUpdateQuery, vars...)
			//if err != nil {
			//  log.Errorf("Failed to execute update query for relation: %v", err)
			//}

		}
	}
	//

	for _, bf := range dr.ms.AfterUpdate {
		//log.Infof("Invoke AfterUpdate [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

		results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{updatedResource})
		if len(results) != 0 {
			updatedResource = results[0]
		} else {
			updatedResource = nil
		}

		if err != nil {
			log.Errorf("Error from AfterUpdate middleware: %v", err)
		}
	}
	delete(updatedResource, "id")

	//for k, v := range updatedResource {
	//  k1 := reflect.TypeOf(v)
	//  //log.Infof("K: %v", k1)
	//  if v != nil && k1.Kind() == reflect.Slice {
	//    updatedResource[k] = string(v.([]uint8))
	//  }
	//}

	//log.Infof("Create response: %v", m)

	return NewResponse(nil, api2go.NewApi2GoModelWithData(dr.model.GetName(), dr.model.GetColumns(), dr.model.GetDefaultPermission(), dr.model.GetRelations(), updatedResource), 200, nil), nil

}
