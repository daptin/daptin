package resource

import (
	"crypto/md5"
	"encoding/base64"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"net/http"
	"net/url"
	"strconv"

	"github.com/artpar/api2go/v2"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	//"reflect"

	//"strconv"
	"fmt"

	"github.com/araddon/dateparse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/pkg/errors"

	//"strconv"
	"strings"
	"time"
)

// Create a new object. Newly created object/struct must be in Responder.
// Possible Responder status codes are:
// - 201 Created: Resource was created and needs to be returned
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Resource created with a client generated ID, and no fields were modified by
//   the server

func (dbResource *DbResource) CreateWithoutFilter(obj interface{}, req api2go.Request, createTransaction *sqlx.Tx) (map[string]interface{}, error) {
	log.Tracef("Create object of type [%v]", dbResource.model.GetName())
	data := obj.(api2go.Api2GoModel)
	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	isAdmin := IsAdminWithTransaction(sessionUser, createTransaction)

	attrs := data.GetAllAsAttributes()

	allColumns := dbResource.model.GetColumns()

	dataToInsert := make(map[string]interface{})
	u, _ := uuid.NewV7()
	newObjectReferenceId := daptinid.DaptinReferenceId(u)

	var colsList []interface{}
	var valsList []interface{}
	for _, col := range allColumns {

		//log.Printf("Add column: %v", col.ColumnName)
		if col.IsAutoIncrement {
			continue
		}

		if col.ColumnName == "created_at" {
			continue
		}

		if col.ColumnName == "updated_at" {
			continue
		}

		if col.ColumnName == "permission" {
			continue
		}

		if col.ColumnName == USER_ACCOUNT_ID_COLUMN && dbResource.model.GetName() != "user_account_user_account_id_has_usergroup_usergroup_id" {
			continue
		}

		//log.Printf("Check column: %v", col.ColumnName)

		columnValue, columnValueOk := attrs[col.ColumnName]

		if !columnValueOk || columnValue == nil {
			if col.DefaultValue != "" {
				//var err error
				if len(col.DefaultValue) > 2 && col.DefaultValue[0] == col.DefaultValue[len(col.DefaultValue)-1] {
					columnValue = col.DefaultValue[1 : len(col.DefaultValue)-1]
				} else {
					columnValue = col.DefaultValue
				}
			} else {
				continue
			}
		}

		if col.ColumnName == "reference_id" {
			s := columnValue.(string)
			if len(s) > 0 {
				newObjectReferenceId = daptinid.DaptinReferenceId(uuid.MustParse(s))
				columnValue = newObjectReferenceId[:]
			} else {
				continue
			}
		}

		if col.IsForeignKey {

			switch col.ForeignKeyData.DataSource {
			case "self":

				//log.Printf("Convert reference_id to id %v[%v]", col.ForeignKeyData.Namespace, columnValue)
				var dir = daptinid.InterfaceToDIR(columnValue)
				if dir == daptinid.NullReferenceId {
					if col.IsNullable {
						columnValue = nil
					} else {
						log.Errorf("Expected string in foreign key column[%v], found %v", col.ColumnName, columnValue)
						return nil, errors.New("unexpected value in foreign key column")
					}
				} else {
					var uId interface{}
					foreignObjectReferenceId, err := GetReferenceIdToIdWithTransaction(col.ForeignKeyData.Namespace, dir, createTransaction)
					if err != nil {
						return nil, fmt.Errorf("[129] foreign object not found [%v][%v]", col.ForeignKeyData.Namespace, dir)
					}

					foreignObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(col.ForeignKeyData.Namespace, dir, createTransaction)

					if isAdmin || foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
						uId = foreignObjectReferenceId
					} else {
						log.Errorf("[137] User cannot refer this object [%v][%v]", col.ForeignKeyData.Namespace, columnValue)
						return nil, fmt.Errorf("refer object not allowed [%v][%v]", col.ForeignKeyData.Namespace, columnValue)
					}

					columnValue = uId
				}

			case "cloud_store":

				files, ok := columnValue.([]interface{})
				uploadPath := ""
				if ok {
					var err error

					columnAssetCache, ok := dbResource.AssetFolderCache[dbResource.tableInfo.TableName][col.ColumnName]
					if ok {
						err = columnAssetCache.UploadFiles(files)
					}

					for i := range files {
						file := files[i].(map[string]interface{})

						fileContentsBase64, ok := file["file"].(string)
						if !ok {
							fileContentsBase64, ok = file["contents"].(string)
							if !ok {
								continue
							}
						}
						splitParts := strings.Split(fileContentsBase64, ",")
						encodedPart := splitParts[0]
						if len(splitParts) > 1 {
							encodedPart = splitParts[len(splitParts)-1]
						}
						fileBytes, _ := base64.StdEncoding.DecodeString(encodedPart)
						//partial := fileBytes[0:500]
						//log.Infof("Partial file: [%v]", partial)
						filemd5 := GetMD5Hash(fileBytes)
						file["md5"] = filemd5
						file["size"] = len(fileBytes)
						path, ok := file["path"]
						if ok && path != nil {
							uploadPath = path.(string)
						} else {
							file["path"] = ""
						}
						files[i] = file
					}

					uploadActionPerformer := ActionHandlerMap["cloudstore.file.upload"]

					actionRequestParameters := make(map[string]interface{})
					actionRequestParameters["file"] = columnValue
					actionRequestParameters["path"] = uploadPath

					log.Printf("Get cloud store details: %v", col.ForeignKeyData.Namespace)
					cloudStore, err := dbResource.GetCloudStoreByNameWithTransaction(col.ForeignKeyData.Namespace, createTransaction)
					CheckErr(err, "Failed to get cloud storage details")
					if err != nil {
						continue
					}

					log.Printf("[195] Cloud storage: %v", cloudStore)

					actionRequestParameters["credential_name"] = cloudStore.CredentialName
					actionRequestParameters["store_provider"] = cloudStore.StoreProvider
					actionRequestParameters["store_type"] = cloudStore.StoreType
					actionRequestParameters["name"] = cloudStore.Name
					actionRequestParameters["root_path"] = cloudStore.RootPath + "/" + col.ForeignKeyData.KeyName

					log.Printf("Initiate file upload action from resource create")
					_, _, errs := uploadActionPerformer.DoAction(actionresponse.Outcome{}, actionRequestParameters, createTransaction)
					if errs != nil && len(errs) > 0 {
						log.Errorf("Failed to upload attachments: %v", errs)
					}
					for i := range files {
						file := files[i].(map[string]interface{})
						delete(file, "file")
						delete(file, "contents")
						files[i] = file
					}
					columnValue, err = json.Marshal(files)
					CheckErr(err, "Failed to marshal file data to column")
				} else {
					columnValue = nil
				}

			default:
				CheckErr(errors.New("undefined foreign key"), "Data source: %v", col.ForeignKeyData.DataSource)

			}

		}
		var err error

		if col.ColumnType == "password" || col.ColumnType == "bcrypt" {
			columnValue, err = BcryptHashString(columnValue.(string))
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				columnValue = ""
			}
		}

		if col.ColumnType == "md5-bcrypt" {
			digest := md5.New()
			digest.Write([]byte(columnValue.(string)))
			hash := fmt.Sprintf("%x", digest.Sum(nil))
			columnValue, err = BcryptHashString(hash)
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				columnValue = ""
			}
		}

		if col.ColumnType == "md5" {
			digest := md5.New()
			digest.Write([]byte(columnValue.(string)))
			columnValue = fmt.Sprintf("%x", digest.Sum(nil))
		}

		if col.ColumnType == "datetime" {

			// 2017-07-13T18:30:00.000Z
			valString, ok := columnValue.(string)
			if ok {
				columnValue, err = dateparse.ParseLocal(valString)
				CheckErr(err, fmt.Sprintf("Failed to parse string as date time in create [%v] = [%v]", col.ColumnName, columnValue))
			} else {
				floatVal, ok := columnValue.(float64)
				if ok {
					columnValue = time.Unix(int64(floatVal), 0)
					err = nil
				} else {
					int64Val, ok := columnValue.(int64)
					if ok {
						columnValue = time.Unix(int64Val, 0)
						err = nil
					}
				}
			}

		} else if col.ColumnType == "date" {

			parsedTime, ok := columnValue.(time.Time)
			if !ok {

				valString, ok := columnValue.(string)
				if ok {
					columnValue, err = dateparse.ParseLocal(valString)
					InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", columnValue))
				} else {
					floatVal, ok := columnValue.(float64)
					if ok {
						columnValue = time.Unix(int64(floatVal), 0)
					}
				}

			} else {
				columnValue = parsedTime
			}

		} else if col.ColumnType == "enum" {
			valString, ok := columnValue.(string)
			if !ok {
				valString = fmt.Sprintf("%v", columnValue)
			}

			isEnumOption := false
			valString = strings.ToLower(valString)
			for _, enumVal := range col.Options {

				if valString == enumVal.Value {
					isEnumOption = true
					break
				}
			}

			if !isEnumOption {
				log.Printf("Provided value is not a valid enum option, reject request [%v] [%v]", valString, col.Options)
				return nil, errors.New(fmt.Sprintf("invalid value for %s", col.Name))
			}
			columnValue = valString

		} else if col.ColumnType == "time" {

			// 2017-07-13T18:30:00.000Z
			valString, ok := columnValue.(string)
			if ok {
				columnValue, err = time.Parse("15:04:05", valString)

				CheckErr(err, fmt.Sprintf("Failed to parse string as time [%v]", columnValue))
			} else {

				floatVal, ok := columnValue.(float64)
				if ok {
					columnValue = time.Unix(int64(floatVal), 0)
					err = nil
				}
			}

		} else if col.ColumnType == "measurement" {
			valString, ok := columnValue.(string)
			if ok {

				if columnValue == "" || columnValue == "-" || strings.ToLower(valString) == "na" {
					columnValue = 0
				}
				if BeginsWith(strings.ToLower(col.DataType), "int") {
					floatVal, _ := strconv.ParseFloat(valString, 64)
					intVal := int(floatVal)
					columnValue = fmt.Sprintf("%d", intVal)
				}
			}
		} else if col.ColumnType == "encrypted" {

			secret, err := dbResource.ConfigStore.GetConfigValueForWithTransaction("encryption.secret", "backend", createTransaction)
			if err != nil {
				log.Errorf("Failed to get secret from config: %v", err)
				columnValue = ""
			} else {
				columnValue, err = Encrypt([]byte(secret), columnValue.(string))
				if err != nil {
					log.Errorf("Failed to convert string to encrypted value, not storing the value: %v", err)
					columnValue = ""
				}
			}
		} else if col.ColumnType == "truefalse" {
			valBoolean, ok := columnValue.(bool)
			if ok {
				if valBoolean {
					columnValue = true
				} else {
					columnValue = false
				}
			} else {
				valString, ok := columnValue.(string)
				if ok {
					valueClean := strings.ToLower(strings.TrimSpace(valString))
					if valueClean == "true" || valueClean == "1" {
						columnValue = true
					} else {
						columnValue = false
					}
				}
			}
			//log.Warnf("driver name in truefalse: %s", createTransaction.DriverName())
			if createTransaction.DriverName() == "sqlite3" {
				_, isInt := columnValue.(int64)
				if !isInt {
					columnValueBool, isBool := columnValue.(bool)
					if isBool {
						if columnValueBool {
							columnValue = 1
						} else {
							columnValue = 0
						}
					} else {
						columnValue = 0
					}
				}
			}

		}

		_, isInterfaceArray := columnValue.([]interface{})
		if isInterfaceArray {
			columnValue, err = json.MarshalToString(columnValue)
		}
		_, isMapInterfaceArray := columnValue.([]map[string]interface{})
		if isMapInterfaceArray {
			columnValue, err = json.MarshalToString(columnValue)
		}

		dataToInsert[col.ColumnName] = columnValue
		colsList = append(colsList, col.ColumnName)
		valsList = append(valsList, columnValue)
	}

	if !InArray(colsList, "reference_id") {
		colsList = append(colsList, "reference_id")
		valsList = append(valsList, newObjectReferenceId[:])
	}
	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	colsList = append(colsList, "permission")
	valsList = append(valsList, dbResource.model.GetDefaultPermission())

	colsList = append(colsList, "created_at")
	valsList = append(valsList, time.Now())

	colsList = append(colsList, "updated_at")
	valsList = append(valsList, time.Now())

	if sessionUser.UserId != 0 && dbResource.model.HasColumn(USER_ACCOUNT_ID_COLUMN) && dbResource.model.GetName() != "user_account_user_account_id_has_usergroup_usergroup_id" {

		colsList = append(colsList, USER_ACCOUNT_ID_COLUMN)
		valsList = append(valsList, sessionUser.UserId)
	}

	query, vals, err := statementbuilder.Squirrel.
		Insert(dbResource.model.GetName()).Cols(colsList...).Prepared(true).Vals(valsList).ToSQL()

	if err != nil {
		log.Errorf("438 Failed to create insert query: %v", err)
		return nil, err
	}

	_, err = createTransaction.Exec(query, vals...)
	if err != nil {
		log.Errorf("[431] Failed to execute insert query: %v, vals [%v]", query, vals)
		return nil, err
	}
	createdResource, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.model.GetName(), newObjectReferenceId, createTransaction)

	if err != nil {
		log.Errorf("[453] Failed to select the newly created entry: [%v][%v] %v",
			dbResource.model.GetName(), newObjectReferenceId, err)
		return nil, err
	}

	if len(languagePreferences) > 0 {

		for _, languagePreference := range languagePreferences {

			colsList = append(colsList, "language_id")
			valsList = append(valsList, languagePreference)

			colsList = append(colsList, "translation_reference_id")
			valsList = append(valsList, createdResource["id"])

			query, vals, err := statementbuilder.Squirrel.Insert(dbResource.model.GetName() + "_i18n").
				Cols(colsList...).Prepared(true).Vals(valsList).ToSQL()
			if err != nil {
				log.Errorf("469 Failed to create insert query: %v", err)
				return nil, err
			}

			_, err = createTransaction.Exec(query, vals...)
			if err != nil {
				log.Infof("Insert query 468: %v", query)
				log.Errorf("Failed to execute insert query 469: %v", err)
				log.Errorf("%v", vals)
				return nil, err

			}
		}
	}

	//log.Printf("Created entry: %v", createdResource)

	for relationName, values := range dbResource.defaultRelations {

		if len(values) == 0 {
			continue
		}

		relation, found := dbResource.tableInfo.GetRelationByName(relationName)
		if !found {
			log.Warnf("Relations [%v] not found on table [%v]", relationName, dbResource.tableInfo)
			continue
		}

		typeName := relation.Subject
		columnName := relation.SubjectName

		if dbResource.tableInfo.TableName == relation.Subject {
			typeName = relation.Object
			columnName = relation.ObjectName
		}

		insertSql := statementbuilder.Squirrel.
			Insert(relation.GetJoinTableName()).Prepared(true).
			Cols(dbResource.model.GetName()+"_id", columnName, "reference_id", "permission")

		for _, valueToAdd := range values {
			nuuid, _ := uuid.NewV7()

			belogsToUserGroupSql, q, _ := insertSql.Vals([]interface{}{createdResource["id"], valueToAdd, nuuid[:], auth.DEFAULT_PERMISSION}).ToSQL()

			log.Tracef("Add new object [%v][%v] to [%v] [%v]", dbResource.tableInfo.TableName, createdResource["reference_id"], typeName, valueToAdd)
			_, err = createTransaction.Exec(belogsToUserGroupSql, q...)

			if err != nil {
				log.Errorf("Failed to insert add [%v] [%v] relation for [%v]: %v", relationName, valueToAdd, dbResource.model.GetName(), err)
				return nil, err
			}
		}
	}

	groupsToAdd := dbResource.defaultGroups
	for _, groupId := range groupsToAdd {
		nuuid, _ := uuid.NewV7()

		relationTableName := dbResource.model.GetName() + "_" + dbResource.model.GetName() + "_id" + "_has_usergroup_usergroup_id"
		relationTableModel, ok := dbResource.Cruds[relationTableName]
		if !ok {
			log.Errorf("Relation table model not found [%s]", relationTableName)
		} else {
			belogsToUserGroupSql, q, _ := statementbuilder.Squirrel.
				Insert(relationTableName).
				Cols(dbResource.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").Prepared(true).
				Vals([]interface{}{createdResource["id"], groupId, nuuid[:], relationTableModel.model.GetDefaultPermission()}).ToSQL()

			log.Tracef("Add new object [%v][%v] to usergroup [%v]", dbResource.tableInfo.TableName, createdResource["reference_id"], groupId)
			_, err = createTransaction.Exec(belogsToUserGroupSql, q...)
			log.Tracef("Added new object [%v][%v] to usergroup [%v]", dbResource.tableInfo.TableName, createdResource["reference_id"], groupId)

			if err != nil {
				log.Errorf("Failed to insert add user group relation for [%v]: %v", dbResource.model.GetName(), err)
				return nil, err

			}
		}
	}

	if dbResource.model.GetName() == "usergroup" && sessionUser.UserId != 0 {

		log.Tracef("Associate new usergroup with user: %v", sessionUser.UserId)
		//u, _ := uuid.NewV7()
		//nuuid := u.String()
		//
		//belogsToUserGroupSql, q, err := statementbuilder.Squirrel.
		//	Insert("user_account_user_account_id_has_usergroup_usergroup_id").
		//	Cols(USER_ACCOUNT_ID_COLUMN, "usergroup_id", "reference_id", "permission").
		//	Vals([]interface{}{sessionUser.UserId, createdResource["id"], nuuid, auth.DEFAULT_PERMISSION}).ToSQL()
		////log.Printf("Query: %v", belogsToUserGroupSql)
		//_, err = dbResource.db.Exec(belogsToUserGroupSql, q...)
		//
		//if err != nil {
		//	log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dbResource.model.GetName(), err)
		//}

	} else if dbResource.model.GetName() == USER_ACCOUNT_TABLE_NAME {

		//adminUserId, _ := GetAdminUserIdAndUserGroupId(createTransaction)
		log.Tracef("Associate new user as owner of their own user_account: %v", createdResource["id"])

		belongsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Update(USER_ACCOUNT_TABLE_NAME).Prepared(true).
			Set(goqu.Record{USER_ACCOUNT_ID_COLUMN: createdResource["id"]}).
			Where(goqu.Ex{"id": createdResource["id"]}).ToSQL()

		_, err = createTransaction.Exec(belongsToUserGroupSql, q...)
		log.Tracef("_, err = createTransaction.Exec(belongsToUserGroupSql, q...)\n")

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dbResource.model.GetName(), err)
			return nil, err

		}

	}

	for _, rel := range dbResource.model.GetRelations() {
		relationName := rel.GetRelation()

		log.Tracef("Check relation in Update: %v", rel.String())
		if rel.GetSubject() == dbResource.model.GetName() {

			if relationName == "belongs_to" || relationName == "has_one" {
				continue
			}

			val11, ok := attrs[rel.GetObjectName()]
			if !ok {
				continue
			}
			returnList := make([]string, 0)
			var valueList []interface{}
			valueListMap, ok := val11.([]map[string]interface{})
			if ok {
				valueList = MapArrayToInterfaceArray(valueListMap)
			} else {
				valueList, ok = val11.([]interface{})
				if !ok {
					log.Warnf("invalue value for column [%v]", rel.GetObjectName())
					continue
				}
			}

			if len(valueList) < 1 {
				attrs[rel.GetObjectName()] = returnList
				continue
			}

			log.Tracef("Update object for relation on [%v] : [%v]", rel.GetObjectName(), val11)

			switch relationName {
			case "has_one":
			case "belongs_to":
				break

			case "has_many_and_belongs_to_many":
				fallthrough
			case "has_many":

				for _, itemInterface := range valueList {
					item := itemInterface.(map[string]interface{})
					//obj := make(map[string]interface{})
					item[rel.GetObjectName()] = item["reference_id"]
					returnList = append(returnList, item["reference_id"].(string))
					item[rel.GetSubjectName()] = newObjectReferenceId
					delete(item, "reference_id")
					delete(item, "meta")
					delete(item, "type")
					delete(item, "reference_id")

					attributes, ok := item["attributes"]
					hasColumns := false
					if ok {
						attributesMap, mapOk := attributes.(map[string]interface{})
						if mapOk {
							for key, val := range attributesMap {
								isJoinTableColumn := false
								for _, col := range rel.Columns {
									if col.Name == key {
										isJoinTableColumn = true
										break
									}
								}
								if !isJoinTableColumn {
									log.Infof("Attribute [%v] is not a join table column in [%v]", key, rel.GetJoinTableName())
									continue
								}

								if val == nil || key == "reference_id" {
									continue
								}
								item[key] = val
								hasColumns = true
							}
						}
						delete(item, "attributes")
					}

					subjectId := data.GetColumnOriginalValue("id")
					objectId, err := GetReferenceIdToIdWithTransaction(rel.GetObject(),
						daptinid.InterfaceToDIR(item[rel.GetObjectName()]), createTransaction)
					if err != nil {
						return nil, fmt.Errorf("object not found [%v][%v]", rel.GetObject(), item[rel.GetObjectName()])
					}

					joinReferenceId, err := GetReferenceIdByWhereClauseWithTransaction(rel.GetJoinTableName(), createTransaction, goqu.Ex{
						rel.GetObjectName():  objectId,
						rel.GetSubjectName(): subjectId,
					})
					CheckErr(err, "join row not found")

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, int64(auth.DEFAULT_PERMISSION), nil, item)

					ur, _ := url.Parse("/" + rel.GetSubject() + "/relationships/" + rel.GetObjectName())

					pr := &http.Request{
						Method: "POST",
						URL:    ur,
					}
					pr = pr.WithContext(req.PlainRequest.Context())

					if len(joinReferenceId) > 0 {

						if hasColumns {
							log.Infof("[670] Updating existing join table row properties: %v", joinReferenceId[0])
							modl.SetID(string(joinReferenceId[0][:]))
							pr.Method = "PATCH"

							_, err = dbResource.Cruds[rel.GetJoinTableName()].UpdateWithTransaction(modl, api2go.Request{
								PlainRequest: pr,
							}, createTransaction)
							if err != nil {
								log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
								return nil, err
							}
						} else {
							log.Infof("Relation already present [%s]: %v, no columns to update", rel.GetJoinTableName(), joinReferenceId[0])
						}

					} else {

						log.Infof("[620] Creating new join table row properties: %v - %v", rel.GetJoinTableName(), modl.GetAttributes())
						_, err := dbResource.Cruds[rel.GetJoinTableName()].CreateWithTransaction(modl, api2go.Request{
							PlainRequest: pr,
						}, createTransaction)
						CheckErr(err, "[624] Failed to update and insert join table row")
						if err != nil {
							return nil, err
						}

					}

				}
				attrs[rel.GetObjectName()] = returnList

				break

			default:
				log.Errorf("Unknown relation: %v", relationName)
			}

		} else {

			val, ok := attrs[rel.GetSubjectName()]
			if !ok {
				continue
			}
			log.Printf("Update %v [%v] on: %v -> %v", rel.String(), newObjectReferenceId, rel.GetSubjectName(), val)

			returnList := make([]daptinid.DaptinReferenceId, 0)
			//var relUpdateQuery string
			//var vars []interface{}
			switch relationName {
			case "has_one":
				//intId := updatedResource["id"].(int64)
				//log.Printf("Converted ids for [%v]: %v", rel.GetObject(), intId)

				valMapList, ok := val.([]interface{})

				if !ok {
					valMap, ok := val.([]map[string]interface{})
					if ok {
						valMapList = MapArrayToInterfaceArray(valMap)
					} else {
						log.Warnf("[718] invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, valMapInterface := range valMapList {
					valMap := valMapInterface.(map[string]interface{})

					foreignObjectReferenceId := daptinid.InterfaceToDIR(valMap[rel.GetSubjectName()])
					returnList = append(returnList, foreignObjectReferenceId)

					oldRow := map[string]interface{}{
						rel.GetObjectName(): "",
						"reference_id":      foreignObjectReferenceId,
					}

					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, int64(auth.DEFAULT_PERMISSION), nil, oldRow)

					model.SetAttributes(map[string]interface{}{
						rel.GetObjectName(): newObjectReferenceId,
					})

					_, err := dbResource.Cruds[rel.GetSubject()].UpdateWithTransaction(model, req, createTransaction)
					if err != nil {
						log.Errorf("Failed to update [%v][%v]: %v", rel.GetObject(), newObjectReferenceId, err)
						return nil, err
					}
				}

				//relUpdateQuery, vars, err = statementbuilder.Squirrel.Update(rel.GetSubject()).
				//    Set(rel.GetObjectName(), intId).Where(goqu.Ex{"reference_id": val}).ToSQL()

				//if err != nil {
				//  log.Errorf("Failed to make update query: %v", err)
				//  continue
				//}

				//log.Printf("Relation update query params: %v", vars)

				break
			case "belongs_to":
				//intId := updatedResource["id"].(int64)
				//log.Printf("Converted ids for [%v]: %v", rel.GetObject(), intId)

				valMapList, ok := val.([]interface{})

				if !ok {
					valMap, ok := val.([]map[string]interface{})
					if ok {
						valMapList = MapArrayToInterfaceArray(valMap)
					} else {
						log.Warnf("[768] invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, valMapInterface := range valMapList {
					valMap := valMapInterface.(map[string]interface{})
					updateForeignRow := make(map[string]interface{})
					foreignObjectReferenceId := daptinid.InterfaceToDIR(valMap[rel.GetSubjectName()])
					returnList = append(returnList, foreignObjectReferenceId)

					updateForeignRow, err = dbResource.GetReferenceIdToObjectWithTransaction(rel.GetSubject(), foreignObjectReferenceId, createTransaction)
					if err != nil {
						log.Errorf("Failed to fetch related row to update [%v] == %v", rel.GetSubject(), valMap)
						continue
					}
					updateForeignRow[rel.GetSubjectName()] = newObjectReferenceId

					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, int64(auth.DEFAULT_PERMISSION), nil, updateForeignRow)

					_, err := dbResource.Cruds[rel.GetSubject()].UpdateWithTransaction(model, req, createTransaction)
					if err != nil {
						log.Errorf("Failed to update [%v][%v]: %v", rel.GetObject(), newObjectReferenceId, err)
						return nil, err
					}
				}

				break

			case "has_many_and_belongs_to_many":
				fallthrough
			case "has_many":
				values, ok := val.([]interface{})
				if !ok {
					valMap, ok := val.([]map[string]interface{})
					if ok {
						values = MapArrayToInterfaceArray(valMap)
					} else {
						log.Warnf("[805] invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, itemInterface := range values {
					item := itemInterface.(map[string]interface{})
					//obj := make(map[string]interface{})
					targetEntityDir := daptinid.InterfaceToDIR(item["reference_id"])
					item[rel.GetSubjectName()] = targetEntityDir
					returnList = append(returnList, targetEntityDir)
					item[rel.GetObjectName()] = newObjectReferenceId
					delete(item, "reference_id")
					delete(item, "meta")
					delete(item, "type")
					delete(item, "reference_id")

					attributes, ok := item["attributes"]
					hasColumns := false
					if ok {
						attributesMap, mapOk := attributes.(map[string]interface{})
						if mapOk {
							for key, val := range attributesMap {
								if val == nil || key == "reference_id" {
									continue
								}
								item[key] = val
								hasColumns = true
							}
						}
						delete(item, "attributes")
					}

					subjectId, err := GetReferenceIdToIdWithTransaction(rel.GetSubject(),
						item[rel.GetSubjectName()].(daptinid.DaptinReferenceId), createTransaction)
					if err != nil {
						return nil, fmt.Errorf("subject not found [%v][%v]", rel.GetSubject(), item[rel.GetSubjectName()])
					}

					objectId := createdResource["id"]

					joinRow, err := GetObjectByWhereClauseWithTransaction(rel.GetJoinTableName(), createTransaction, goqu.Ex{
						rel.GetObjectName():  objectId,
						rel.GetSubjectName(): subjectId,
					})

					var modl api2go.Api2GoModel
					if err != nil || len(joinRow) < 1 {
						modl = api2go.NewApi2GoModel(rel.GetJoinTableName(), nil, int64(auth.DEFAULT_PERMISSION), nil)
					} else {
						modl = api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, int64(auth.DEFAULT_PERMISSION), nil, joinRow[0])
					}

					modl.SetAttributes(item)
					ur, _ := url.Parse("/" + rel.GetSubject() + "/relationships/" + rel.GetObjectName())
					pr := &http.Request{
						Method: "POST",
						URL:    ur,
					}
					pr = pr.WithContext(req.PlainRequest.Context())

					if len(joinRow) > 0 {

						if hasColumns {
							log.Infof("[804] Updating existing join table row properties: %v", joinRow[0]["reference_id"])
							pr.Method = "PATCH"

							_, err = dbResource.Cruds[rel.GetJoinTableName()].UpdateWithTransaction(modl, api2go.Request{
								PlainRequest: pr,
							}, createTransaction)
							if err != nil {
								log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
								return nil, err
							}
						} else {
							log.Infof("Relation already present [%s]: %v, no columns to update", rel.GetJoinTableName(), joinRow[0]["reference_id"])
						}

					} else {

						log.Infof("[902] Creating new join table row properties: %v - %v", rel.GetJoinTableName(), modl.GetAttributes())
						_, err := dbResource.Cruds[rel.GetJoinTableName()].CreateWithTransaction(modl, api2go.Request{
							PlainRequest: pr,
						}, createTransaction)
						CheckErr(err, "[895] Failed to update and insert join table row")
						if err != nil {
							return nil, err
						}

					}

				}
				break

			default:
				log.Errorf("Unknown relation: %v", relationName)
			}

			createdResource[rel.GetSubjectName()] = returnList

			//_, err = dbResource.db.Exec(relUpdateQuery, vars...)
			//if err != nil {
			//  log.Errorf("Failed to execute update query for relation: %v", err)
			//}

		}
	}

	delete(createdResource, "id")
	createdResource["__type"] = dbResource.model.GetName()
	log.Tracef("[END] Create object of type [%v]", dbResource.model.GetName())

	return createdResource, nil

}

func (dbResource *DbResource) CreateWithTransaction(obj interface{}, req api2go.Request, transaction *sqlx.Tx) (api2go.Responder, error) {
	data := obj.(api2go.Api2GoModel)
	//log.Printf("Create object request: [%v] %v", dbResource.model.GetTableName(), data.Data)

	for _, bf := range dbResource.ms.BeforeCreate {
		//log.Printf("Invoke BeforeCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		data.SetType(dbResource.model.GetName())
		responseData, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{data.GetAttributes()}, transaction)
		if err != nil {
			log.Warnf("Error from BeforeCreate[%v]: %v", bf.String(), err)
			return nil, err
		}
		if responseData == nil {
			return nil, errors.New(fmt.Sprintf("No object to act upon after %v", bf.String()))
		}
	}

	createdResource, err := dbResource.CreateWithoutFilter(obj, req, transaction)
	if err != nil {
		return NewResponse(nil, nil, 500, nil), err
	}

	for _, bf := range dbResource.ms.AfterCreate {
		log.Tracef("Invoke AfterCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		results, err := bf.InterceptAfter(dbResource, &req, []map[string]interface{}{createdResource}, transaction)
		if err != nil {
			log.Errorf("Error from AfterCreate[%v] middleware: %v", bf.String(), err)
		}
		if len(results) < 1 {
			createdResource = nil
		} else {
			createdResource = results[0]
		}
	}

	n1 := dbResource.model.GetName()
	c1 := dbResource.model.GetColumns()
	p1 := dbResource.model.GetDefaultPermission()
	r1 := dbResource.model.GetRelations()
	return NewResponse(nil,
		api2go.NewApi2GoModelWithData(n1, c1, p1, r1, createdResource),
		201, nil,
	), nil

}

func (dbResource *DbResource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	data := obj.(api2go.Api2GoModel)
	//log.Printf("Create object request: [%v] %v", dbResource.model.GetTableName(), data.Data)

	transaction, err := dbResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [980]")
		return nil, err
	}

	for _, bf := range dbResource.ms.BeforeCreate {
		//log.Printf("Invoke BeforeCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		data.SetType(dbResource.model.GetName())
		responseData, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{data.GetAttributes()}, transaction)
		if err != nil {
			log.Warnf("Error from BeforeCreate[%v]: %v", bf.String(), err)
			transaction.Rollback()
			return nil, err
		}
		if responseData == nil {
			transaction.Rollback()
			return nil, errors.New(fmt.Sprintf("No object to act upon after %v", bf.String()))
		}
	}

	createdResource, err := dbResource.CreateWithoutFilter(obj, req, transaction)
	log.Tracef("CreateWithoutFilter [%v]", dbResource.model.GetName())
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "failed to rollback")
		return NewResponse(nil, nil, 500, nil), err
	}

	for _, bf := range dbResource.ms.AfterCreate {
		//log.Printf("Invoke AfterCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		results, err := bf.InterceptAfter(dbResource, &req, []map[string]interface{}{createdResource}, transaction)
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "failed to rollback")
			log.Errorf("Error from AfterCreate[%v] middleware: %v", bf.String(), err)
			return nil, err
		}
		if len(results) < 1 {
			createdResource = nil
		} else {
			createdResource = results[0]
		}
	}
	commitErr := transaction.Commit()
	if commitErr != nil {
		return nil, commitErr
	}

	n1 := dbResource.model.GetName()
	c1 := dbResource.model.GetColumns()
	p1 := dbResource.model.GetDefaultPermission()
	r1 := dbResource.model.GetRelations()
	return NewResponse(nil,
		api2go.NewApi2GoModelWithData(n1, c1, p1, r1, createdResource),
		201, nil,
	), nil

}
