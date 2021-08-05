package resource

import (
	"encoding/base64"
	"github.com/jmoiron/sqlx"
	"strings"

	"github.com/artpar/api2go"
	uuid "github.com/artpar/go.uuid"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	log "github.com/sirupsen/logrus"

	//"reflect"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// Update an object
// Possible Responder status codes are:
// - 200 OK: Update successful, however some field(s) were changed, returns updates source
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Update was successful, no fields were changed by the server, return nothing
func (dbResource *DbResource) UpdateWithoutFilters(obj interface{}, req api2go.Request, updateTransaction *sqlx.Tx) (map[string]interface{}, error) {

	data, ok := obj.(*api2go.Api2GoModel)

	if !ok {
		log.Errorf("Request data is not api2go model: %v", data)
		return nil, errors.New("invalid request")
	}

	id := data.GetID()
	idInt, err := GetReferenceIdToIdWithTransaction(dbResource.model.GetName(), id, updateTransaction)
	if err != nil {
		return nil, err
	}

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}
	isAdmin := IsAdminWithTransaction(sessionUser.UserReferenceId, updateTransaction)

	attrs := data.GetAllAsAttributes()

	if !data.HasVersion() {
		originalData, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.model.GetTableName(), id, updateTransaction)
		if err != nil {
			return nil, err
		}
		data = api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(), nil, 0, nil, originalData)
		data.SetAttributes(attrs)
	}

	allChanges := data.GetChanges()
	allColumns := dbResource.model.GetColumns()
	//log.Printf("Update object request with changes: %v", allChanges)

	//dataToInsert := make(map[string]interface{})

	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	var colsList []string
	var valsList []interface{}
	if len(allChanges) > 0 {
		for _, col := range allColumns {

			//log.Printf("Add column: %v", col.ColumnName)
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

			if col.ColumnName == "version" {
				continue
			}

			change, ok := allChanges[col.ColumnName]
			if !ok {
				continue
			}

			//log.Printf("Check column: [%v]  (%v) => (%v) ", col.ColumnName, change.OldValue, change.NewValue)

			var val interface{}
			val = change.NewValue
			if col.IsForeignKey {

				//log.Printf("Convert ref id to id %v[%v]", col.ForeignKeyData.Namespace, val)

				switch col.ForeignKeyData.DataSource {
				case "self":
					if val != nil && val != "" {

						valString := val.(string)

						foreignObject, err := dbResource.GetReferenceIdToObjectWithTransaction(col.ForeignKeyData.Namespace, valString, updateTransaction)
						if err != nil {
							return nil, err
						}

						foreignObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(col.ForeignKeyData.Namespace, valString, updateTransaction)

						if isAdmin || foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
							val = foreignObject["id"]
						} else {
							return nil, errors.New(fmt.Sprintf("no refer permission on object [%v][%v]", col.ForeignKeyData.Namespace, valString))
						}
					} else {
						ok = true
					}

				case "cloud_store":

					if val == nil {
						ok = false
						continue
					}

					uploadActionPerformer, err := NewFileUploadActionPerformer(dbResource.Cruds)
					CheckErr(err, "Failed to create upload action performer")
					log.Printf("created upload action performer")
					if err != nil {
						continue
					}

					files, ok := val.([]interface{})
					uploadPath := ""

					for i := range files {
						file := files[i].(map[string]interface{})

						i2, ok := file["file"]
						fileContentsBase64 := ""
						ok1 := false
						if ok {

							fileContentsBase64, ok1 = i2.(string)
						}
						if !ok || !ok1 {
							fileContentsBase64, ok = file["contents"].(string)
							if !ok {
								continue
							}
						}
						splitParts := strings.Split(fileContentsBase64, ",")
						encodedPart := splitParts[0]
						if len(splitParts) > 1 {
							encodedPart = splitParts[1]
						}
						fileBytes, _ := base64.StdEncoding.DecodeString(encodedPart)
						filemd5 := GetMD5Hash(fileBytes)
						file["md5"] = filemd5
						file["size"] = len(fileBytes)
						path, ok := file["path"]
						if ok {
							uploadPath = path.(string)
						} else {
							file["path"] = ""
						}
						files[i] = file
					}

					actionRequestParameters := make(map[string]interface{})
					actionRequestParameters["file"] = val
					actionRequestParameters["path"] = uploadPath

					log.Printf("Get cloud store details: %v", col.ForeignKeyData.Namespace)
					cloudStore, err := dbResource.GetCloudStoreByNameWithTransaction(col.ForeignKeyData.Namespace, updateTransaction)
					CheckErr(err, "Failed to get cloud storage details")
					if err != nil {
						continue
					}

					log.Printf("Cloud storage: %v", cloudStore)

					actionRequestParameters["oauth_token_id"] = cloudStore.OAutoTokenId
					actionRequestParameters["store_provider"] = cloudStore.StoreProvider
					actionRequestParameters["root_path"] = cloudStore.RootPath + "/" + col.ForeignKeyData.KeyName

					log.Printf("Initiate file upload action")
					_, _, errs := uploadActionPerformer.DoAction(Outcome{}, actionRequestParameters, updateTransaction)
					if errs != nil && len(errs) > 0 {
						log.Errorf("Failed to upload attachments: %v", errs)
					}

					columnAssetCache, ok := dbResource.AssetFolderCache[dbResource.tableInfo.TableName][col.ColumnName]
					if ok {
						err = columnAssetCache.UploadFiles(val.([]interface{}))
						CheckErr(err, "Failed to store uploaded file in column [%v]", col.ColumnName)
						if err != nil {
							return nil, err
						}
					}

					files, ok = val.([]interface{})
					if ok {

						for i := range files {
							file := files[i].(map[string]interface{})
							delete(file, "file")
							delete(file, "contents")
							files[i] = file
						}
						val, err = json.Marshal(files)
						CheckErr(err, "Failed to marshal file data to column")
					}

				default:
					CheckErr(errors.New("undefined foreign key"), "Data source: %v", col.ForeignKeyData.DataSource)
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
					valString, ok := val.(string)
					if ok {

						//val, err = time.Parse("2006-01-02T15:04:05.999Z", valString)
						val, _, err = fieldtypes.GetDateTime(valString)
						CheckErr(err, fmt.Sprintf("Failed to parse string as date time in update [%v]", val))
						if err != nil {
							ok = false
						}
					} else {
						floatVal, ok := val.(float64)
						if ok {
							val = time.Unix(int64(floatVal), 0)
							err = nil
						}
					}
				} else {
					val = parsedTime
				}
				// 2017-07-13T18:30:00.000Z

			} else if col.ColumnType == "enum" {
				valString, ok := val.(string)
				if !ok {
					valString = fmt.Sprintf("%v", val)
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
				val = valString

			} else if col.ColumnType == "encrypted" {

				secret, err := dbResource.configStore.GetConfigValueForWithTransaction("encryption.secret", "backend", updateTransaction)
				if err != nil {
					log.Errorf("Failed to get secret from config: %v", err)
					return nil, errors.New("unable to store a secret at this time")
				} else {
					if val == nil {
						val = ""
					}
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
					valString, ok := val.(string)
					if ok {

						val1, err := time.Parse("2006-01-02T15:04:05.999Z", valString)

						InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", val))
						if err != nil {
							val, err = time.Parse("2006-01-02", val.(string))
							InfoErr(err, fmt.Sprintf("Failed to parse string as date [%v]", val))
						} else {
							val = val1
						}
					} else {
						floatVal, ok := val.(float64)
						if ok {
							val = time.Unix(int64(floatVal), 0)
							err = nil
						}
					}
				} else {
					val = parsedTime
				}

			} else if col.ColumnType == "time" {
				parsedTime, ok := val.(time.Time)
				if !ok {
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
				} else {
					val = parsedTime
				}
				// 2017-07-13T18:30:00.000Z

			} else if col.ColumnType == "truefalse" {
				valBoolean, ok := val.(bool)
				if ok {
					val = valBoolean
				} else {
					valString, ok := val.(string)
					if ok {
						str := strings.ToLower(strings.TrimSpace(valString))
						if str == "true" || str == "1" {
							val = true
						} else {
							val = false
						}
					} else {
						valInt, ok := val.(int)
						if ok {
							if ok && valInt != 0 {
								val = true
							} else if ok {
								val = false
							}
						}

					}
				}
			}

			if ok {
				//dataToInsert[col.ColumnName] = val
				colsList = append(colsList, col.ColumnName)
				valsList = append(valsList, val)
			}

		}

		colsList = append(colsList, "updated_at")
		valsList = append(valsList, time.Now())

		colsList = append(colsList, "version")
		valsList = append(valsList, data.GetNextVersion())

		if len(languagePreferences) == 0 {

			builder := statementbuilder.Squirrel.Update(dbResource.model.GetName())

			setVals := make(map[string]interface{})
			for i := range colsList {
				setVals[colsList[i]] = valsList[i]
			}
			builder = builder.Set(goqu.Record(setVals))

			query, vals, err := builder.Where(goqu.Ex{"reference_id": id}).Where(goqu.Ex{"version": data.GetCurrentVersion()}).ToSQL()
			//log.Printf("Update query: %v", query)
			if err != nil {
				log.Errorf("Failed to create update query: %v", err)
				return nil, err
			}

			log.Printf("Update query: %v", query)
			_, err = updateTransaction.Exec(query, vals...)
			if err != nil {
				log.Errorf("Failed to execute update query [%s] [%v] 411: %v", query, vals, err)
				return nil, err
			}

		} else if len(languagePreferences) > 0 {

			for _, lang := range languagePreferences {

				langTableCols := make([]interface{}, 0)
				langTableVals := make([]interface{}, 0)

				for _, col := range colsList {
					langTableCols = append(langTableCols, col)
				}

				for _, val := range valsList {
					langTableVals = append(langTableVals, val)
				}

				builder := statementbuilder.Squirrel.Update(dbResource.model.GetName() + "_i18n")

				updateMap := make(map[string]interface{})
				for i := range langTableCols {
					updateMap[langTableCols[i].(string)] = langTableVals[i]
				}
				builder = builder.Set(updateMap)

				query, vals, err := builder.Where(goqu.Ex{"translation_reference_id": idInt}).Where(goqu.Ex{"language_id": lang}).ToSQL()
				log.Printf("Update query: %v", query)
				if err != nil {
					log.Errorf("Failed to create update query: %v", err)
				}

				//log.Printf("Update query: %v == %v", query, vals)
				res, err := updateTransaction.Exec(query, vals...)
				rowsAffected, err := res.RowsAffected()
				if err != nil || rowsAffected == 0 {
					log.Errorf("Failed to execute update query: %v", err)

					u, _ := uuid.NewV4()
					nuuid := u.String()

					langTableCols = append(langTableCols, "language_id", "translation_reference_id", "reference_id")
					langTableVals = append(langTableVals, lang, idInt, nuuid)

					insert := statementbuilder.Squirrel.Insert(dbResource.model.GetName() + "_i18n")
					insert = insert.Cols(langTableCols...)
					insert = insert.Vals(langTableVals)
					query, vals, err := insert.ToSQL()

					_, err = updateTransaction.Exec(query, vals...)

					return nil, err
				}
			}
		}

	}

	if data.IsDirty() && dbResource.tableInfo.IsAuditEnabled {

		auditModel := data.GetAuditModel()
		log.Printf("Object [%v][%v] has been changed, trying to audit in %v", data.GetTableName(), data.GetID(), auditModel.GetTableName())
		if auditModel.GetTableName() != "" {
			creator, ok := dbResource.Cruds[auditModel.GetTableName()]
			if !ok {
				log.Errorf("No creator for audit type: %v", auditModel.GetTableName())
			} else {
				pr := &http.Request{
					Method: "POST",
				}
				pr = pr.WithContext(req.PlainRequest.Context())
				auditCreateRequest := api2go.Request{
					PlainRequest: pr,
				}
				_, err := creator.CreateWithTransaction(auditModel, auditCreateRequest, updateTransaction)
				if err != nil {
					log.Errorf("Failed to create audit entry: %v\n%v", err, auditModel)
					return nil, err
				} else {
					log.Printf("[%v][%v] Created audit record", auditModel.GetTableName(), data.GetID())
					//log.Printf("ReferenceId for change: %v", resp.Result())
				}
			}
		}

	} else {
		//log.Printf("[%v][%v] Not creating an audit row", data.GetTableName(), data.GetID())
	}

	updatedResource, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.model.GetName(), id, updateTransaction)
	if err != nil {
		log.Errorf("[511] Failed to select the newly created entry: %v", err)
		return nil, err
	}

	for _, rel := range dbResource.model.GetRelations() {
		relationName := rel.GetRelation()

		//log.Printf("Check relation in Update: %v", rel.String())
		if rel.GetSubject() == dbResource.model.GetName() {

			if relationName == "belongs_to" || relationName == "has_one" {
				continue
			}

			val11, ok := attrs[rel.GetObjectName()]
			if !ok {
				continue
			}
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
				continue
			}

			//log.Printf("Update object for relation on [%v] : [%v]", rel.GetObjectName(), val11)

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
					item[rel.GetObjectName()] = item["id"]
					item[rel.GetSubjectName()] = updatedResource["reference_id"]
					delete(item, "id")
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

					subjectId, err := GetReferenceIdToIdWithTransaction(rel.GetSubject(), item[rel.GetSubjectName()].(string), updateTransaction)
					objectId, err := GetReferenceIdToIdWithTransaction(rel.GetObject(), item[rel.GetObjectName()].(string), updateTransaction)

					joinReferenceId, err := GetReferenceIdByWhereClauseWithTransaction(rel.GetJoinTableName(), updateTransaction, goqu.Ex{
						rel.GetObjectName():  objectId,
						rel.GetSubjectName(): subjectId,
					})
					CheckErr(err, "join row not found")

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, int64(auth.DEFAULT_PERMISSION), nil, item)

					pr := &http.Request{
						Method: "POST",
					}
					pr = pr.WithContext(req.PlainRequest.Context())

					if len(joinReferenceId) > 0 {

						if hasColumns {
							log.Infof("Updating existing join table row properties: %v", joinReferenceId[0])
							modl.Data["reference_id"] = joinReferenceId[0]
							pr.Method = "PATCH"

							_, err = dbResource.Cruds[rel.GetJoinTableName()].UpdateWithTransaction(modl, api2go.Request{
								PlainRequest: pr,
							}, updateTransaction)
							if err != nil {
								log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
								return nil, err
							}
						} else {
							log.Infof("Relation alredy present [%s]: %v, no columns to update", rel.GetJoinTableName(), joinReferenceId[0])
						}

					} else {

						log.Infof("Creating new join table row properties: %v", rel.GetJoinTableName())
						_, err := dbResource.Cruds[rel.GetJoinTableName()].CreateWithTransaction(modl, api2go.Request{
							PlainRequest: pr,
						}, updateTransaction)
						CheckErr(err, "Failed to update and insert join table row")
						if err != nil {
							rollbackErr := updateTransaction.Rollback()
							CheckErr(rollbackErr, "failed to rollback")
							return nil, err
						}

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
			log.Printf("Update %v on: %v", rel.String(), val)

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
						log.Warnf("invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, valMapInterface := range valMapList {
					valMap := valMapInterface.(map[string]interface{})

					updateForeignRow := make(map[string]interface{})

					updateForeignRow, err = dbResource.Cruds[rel.GetSubject()].GetReferenceIdToObjectWithTransaction(rel.GetSubject(), valMap[rel.GetSubjectName()].(string), updateTransaction)
					if err != nil {
						log.Printf("Failed to get object by reference id: %v", err)
						continue
					}
					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, int64(auth.DEFAULT_PERMISSION), nil, updateForeignRow)

					model.SetAttributes(map[string]interface{}{
						rel.GetObjectName(): updatedResource["reference_id"].(string),
					})

					_, err := dbResource.Cruds[rel.GetSubject()].UpdateWithTransaction(model, req, updateTransaction)
					if err != nil {
						rollbackErr := updateTransaction.Rollback()
						CheckErr(rollbackErr, "failed to rollback")
						log.Errorf("Failed to update [%v][%v]: %v", rel.GetObject(), updatedResource["reference_id"], err)
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
						log.Warnf("invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, valMapInterface := range valMapList {
					valMap := valMapInterface.(map[string]interface{})
					updateForeignRow := make(map[string]interface{})
					updateForeignRow, err = dbResource.GetReferenceIdToObjectWithTransaction(rel.GetSubject(), valMap[rel.GetSubjectName()].(string), updateTransaction)
					if err != nil {
						log.Errorf("Failed to fetch related row to update [%v] == %v", rel.GetSubject(), valMap)
						continue
					}
					updateForeignRow[rel.GetSubjectName()] = updatedResource["reference_id"].(string)

					model := api2go.NewApi2GoModelWithData(rel.GetSubject(), nil, int64(auth.DEFAULT_PERMISSION), nil, updateForeignRow)

					_, err := dbResource.Cruds[rel.GetSubject()].UpdateWithTransaction(model, req, updateTransaction)
					if err != nil {
						rollbackErr := updateTransaction.Rollback()
						CheckErr(rollbackErr, "failed to rollback")
						log.Errorf("Failed to update [%v][%v]: %v", rel.GetObject(), updatedResource["reference_id"], err)
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
						log.Warnf("invalid value type for column [%v] = %v", rel.GetSubjectName(), val)
					}
				}

				for _, itemInterface := range values {
					item := itemInterface.(map[string]interface{})
					//obj := make(map[string]interface{})
					item[rel.GetSubjectName()] = item["id"]
					item[rel.GetObjectName()] = updatedResource["reference_id"]
					delete(item, "id")
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

					subjectId, err := GetReferenceIdToIdWithTransaction(rel.GetSubject(), item[rel.GetSubjectName()].(string), updateTransaction)
					if err != nil {
						return nil, err
					}
					objectId, err := GetReferenceIdToIdWithTransaction(rel.GetObject(), item[rel.GetObjectName()].(string), updateTransaction)
					if err != nil {
						return nil, err
					}

					joinReferenceId, err := GetReferenceIdByWhereClauseWithTransaction(rel.GetJoinTableName(), updateTransaction, goqu.Ex{
						rel.GetObjectName():  objectId,
						rel.GetSubjectName(): subjectId,
					})

					modl := api2go.NewApi2GoModelWithData(rel.GetJoinTableName(), nil, int64(auth.DEFAULT_PERMISSION), nil, item)

					pr := &http.Request{
						Method: "POST",
					}
					pr = pr.WithContext(req.PlainRequest.Context())

					if len(joinReferenceId) > 0 {

						if hasColumns {
							log.Infof("Updating existing join table row properties: %v", joinReferenceId[0])
							modl.Data["reference_id"] = joinReferenceId[0]
							pr.Method = "PATCH"

							_, err = dbResource.Cruds[rel.GetJoinTableName()].UpdateWithTransaction(modl, api2go.Request{
								PlainRequest: pr,
							}, updateTransaction)
							if err != nil {
								log.Errorf("Failed to insert join table data [%v] : %v", rel.GetJoinTableName(), err)
								return nil, err
							}
						} else {
							log.Infof("Relation already present [%s]: %v, no columns to update", rel.GetJoinTableName(), joinReferenceId[0])
						}

					} else {

						log.Infof("Creating new join table row properties: %v", rel.GetJoinTableName())
						_, err := dbResource.Cruds[rel.GetJoinTableName()].CreateWithTransaction(modl, api2go.Request{
							PlainRequest: pr,
						}, updateTransaction)
						CheckErr(err, "Failed to update and insert join table row")
						if err != nil {
							rollbackErr := updateTransaction.Rollback()
							CheckErr(rollbackErr, "failed to rollback")
							return nil, err
						}

					}

				}
				break

			default:
				log.Errorf("Unknown relation: %v", relationName)
			}

			//_, err = dbResource.db.Exec(relUpdateQuery, vars...)
			//if err != nil {
			//  log.Errorf("Failed to execute update query for relation: %v", err)
			//}

		}
	}
	//

	for relationName, deleteRelations := range data.DeleteIncludes {
		referencedRelation := api2go.TableRelation{}
		referencedTypeName := ""
		//hostRelationTypeName := ""
		hostRelationName := ""
		for _, relation := range dbResource.model.GetRelations() {

			if relation.GetSubject() == dbResource.model.GetTableName() && relation.GetObjectName() == relationName {
				referencedRelation = relation
				referencedTypeName = relation.GetObject()
				//hostRelationTypeName = relation.GetSubject()
				hostRelationName = relation.GetSubjectName()
				break
			} else if relation.GetObject() == dbResource.model.GetTableName() && relation.GetSubjectName() == relationName {
				referencedRelation = relation
				//hostRelationTypeName = relation.GetObject()
				hostRelationName = relation.GetObjectName()
				referencedTypeName = relation.GetSubject()
				break
			}
		}
		if referencedRelation.GetRelation() == "" {
			continue
		}

		log.Printf("Delete [%v] relation: [%v][%v]", referencedRelation.GetRelation(), relationName, deleteRelations)

		for _, deleteId := range deleteRelations {

			otherObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(referencedTypeName, deleteId, updateTransaction)

			if isAdmin || otherObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {

				otherObjectId, err := GetReferenceIdToIdWithTransaction(referencedTypeName, deleteId, updateTransaction)

				if err != nil {
					log.Errorf("Referenced object not found: %v", err)
					continue
				}

				if referencedRelation.Relation == "has_many" || referencedRelation.Relation == "has_many_and_belongs_to_many" {

					joinReference, _, err := dbResource.Cruds[referencedRelation.GetJoinTableName()].GetRowsByWhereClauseWithTransaction(referencedRelation.GetJoinTableName(),
						nil, updateTransaction, goqu.Ex{
							relationName:     otherObjectId,
							hostRelationName: idInt,
						},
					)
					if err != nil {
						log.Errorf("Referenced relation not found: %v", err)
						return nil, err
					}
					if len(joinReference) < 1 {
						log.Warnf("nothing to delete for the relation row to delete does not exist - %v[%v] - %v[%v]", relationName, otherObjectId, hostRelationName, idInt)
						continue
					}

					joinReferenceObject := joinReference[0]
					err = dbResource.Cruds[referencedRelation.GetJoinTableName()].DeleteWithoutFilters(joinReferenceObject["reference_id"].(string), req, updateTransaction)
					if err != nil {
						log.Errorf("Failed to delete relation [%v][%v]: %v", referencedRelation.GetSubject(), referencedRelation.GetObjectName(), err)
						return nil, err
					}
				} else {
					// has_one or belongs_to
					// todo: write code for belongs_to and has_one relation reference deletes
					// check for relation side and update the appropriate column

					selfTypeName := referencedRelation.GetSubject()
					selfSubjectName := referencedRelation.GetSubjectName()
					targetTypeName := referencedRelation.GetObject()
					//targetSubjectName := referencedRelation.GetObjectName()

					if selfTypeName != dbResource.model.GetName() {
						selfTypeName = referencedRelation.GetObject()
						selfSubjectName = referencedRelation.GetObjectName()
						targetTypeName = referencedRelation.GetSubject()
						//targetSubjectName = referencedRelation.GetSubjectName()
					} else {

					}

					foreignObject, err := dbResource.GetIdToObjectWithTransaction(targetTypeName, otherObjectId, updateTransaction)
					if err != nil {
						log.Errorf("Failed to get foreign object by reference deleteId: %v", err)
						continue
					}
					modelToUpdate := api2go.NewApi2GoModelWithData(referencedTypeName, nil, 0, nil, foreignObject)

					updatedAttributes := map[string]interface{}{
						selfSubjectName: nil,
					}

					modelToUpdate.SetAttributes(updatedAttributes)
					_, err = dbResource.Cruds[referencedTypeName].UpdateWithTransaction(modelToUpdate, req, updateTransaction)
					CheckErr(err, "Failed to update object to remove reference")

				}

			} else {
				log.Errorf("Not allowed to delete relation [%v][%v]: %v", referencedRelation.GetSubject(), referencedRelation.GetObjectName(), err)
			}

		}
	}

	return updatedResource, nil

}

func (dbResource *DbResource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	data, _ := obj.(*api2go.Api2GoModel)
	//log.Printf("Update object request: [%v][%v]", dbResource.model.GetTableName(), data.GetID())

	updateRequest := &http.Request{
		Method: "PATCH",
	}
	updateRequest = updateRequest.WithContext(req.PlainRequest.Context())

	transaction, err := dbResource.Connection.Beginx()
	if err != nil {
		return nil, err
	}
	data.Data["__type"] = dbResource.model.GetName()
	for _, bf := range dbResource.ms.BeforeUpdate {
		//log.Printf("Invoke BeforeUpdate [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		finalData, err := bf.InterceptBefore(dbResource, &api2go.Request{
			PlainRequest: updateRequest,
			QueryParams:  req.QueryParams,
			Header:       req.Header,
			Pagination:   req.Pagination,
		}, []map[string]interface{}{
			data.GetAllAsAttributes(),
		}, transaction)
		if err != nil {
			transaction.Rollback()
			log.Errorf("Error From BeforeUpdate middleware: %v", err)
			return nil, err
		}
		if len(finalData) == 0 {
			return nil, fmt.Errorf("failed to updated this object because of [%v]", bf.String())
		}
		res := finalData[0]
		data.Data = res
	}

	updatedResource, err := dbResource.UpdateWithoutFilters(obj, req, transaction)
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "Failed to rollback")
		return NewResponse(nil, nil, 500, nil), err
	}

	for _, bf := range dbResource.ms.AfterUpdate {
		//log.Printf("Invoke AfterUpdate [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		results, err := bf.InterceptAfter(dbResource, &api2go.Request{
			PlainRequest: updateRequest,
			QueryParams:  req.QueryParams,
			Header:       req.Header,
			Pagination:   req.Pagination,
		}, []map[string]interface{}{updatedResource}, transaction)
		if len(results) != 0 {
			updatedResource = results[0]

		} else {
			updatedResource = nil
		}

		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "failed to rollback")
			return nil, err
			log.Errorf("Error from AfterUpdate middleware: %v", err)
		}
	}
	commitErr := transaction.Commit()
	CheckErr(commitErr, "failed to commit")
	if commitErr != nil {
		return nil, commitErr
	}
	delete(updatedResource, "id")

	return NewResponse(nil, api2go.NewApi2GoModelWithData(dbResource.model.GetName(), dbResource.model.GetColumns(), dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), updatedResource), 200, nil), nil

}

func (dbResource *DbResource) UpdateWithTransaction(obj interface{}, req api2go.Request, transaction *sqlx.Tx) (api2go.Responder, error) {
	data, _ := obj.(*api2go.Api2GoModel)
	//log.Printf("Update object request: [%v][%v]", dbResource.model.GetTableName(), data.GetID())

	updateRequest := &http.Request{
		Method: "PATCH",
	}
	updateRequest = updateRequest.WithContext(req.PlainRequest.Context())

	data.Data["__type"] = dbResource.model.GetName()
	for _, bf := range dbResource.ms.BeforeUpdate {
		//log.Printf("Invoke BeforeUpdate [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		finalData, err := bf.InterceptBefore(dbResource, &api2go.Request{
			PlainRequest: updateRequest,
			QueryParams:  req.QueryParams,
			Header:       req.Header,
			Pagination:   req.Pagination,
		}, []map[string]interface{}{
			data.GetAllAsAttributes(),
		}, transaction)
		if err != nil {
			log.Errorf("Error From BeforeUpdate middleware: %v", err)
			return nil, err
		}
		if len(finalData) == 0 {
			return nil, fmt.Errorf("failed to updated this object because of [%v]", bf.String())
		}
		res := finalData[0]
		data.Data = res
	}

	updatedResource, err := dbResource.UpdateWithoutFilters(obj, req, transaction)
	if err != nil {
		return NewResponse(nil, nil, 500, nil), err
	}

	for _, bf := range dbResource.ms.AfterUpdate {
		//log.Printf("Invoke AfterUpdate [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		results, err := bf.InterceptAfter(dbResource, &api2go.Request{
			PlainRequest: updateRequest,
			QueryParams:  req.QueryParams,
			Header:       req.Header,
			Pagination:   req.Pagination,
		}, []map[string]interface{}{updatedResource}, transaction)
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

	return NewResponse(nil, api2go.NewApi2GoModelWithData(dbResource.model.GetName(), dbResource.model.GetColumns(), dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), updatedResource), 200, nil), nil

}
