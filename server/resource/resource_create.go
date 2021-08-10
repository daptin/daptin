package resource

import (
	"crypto/md5"
	"encoding/base64"
	"strconv"

	"github.com/artpar/api2go"
	uuid "github.com/artpar/go.uuid"
	"github.com/doug-martin/goqu/v9"
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
	//log.Printf("Create object of type [%v]", dbResource.model.GetName())
	data := obj.(*api2go.Api2GoModel)
	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	isAdmin := IsAdminWithTransaction(sessionUser.UserReferenceId, createTransaction)

	attrs := data.GetAllAsAttributes()

	allColumns := dbResource.model.GetColumns()

	dataToInsert := make(map[string]interface{})
	u, _ := uuid.NewV4()
	newUuid := u.String()

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

		val, ok := attrs[col.ColumnName]

		if !ok || val == nil {
			if col.DefaultValue != "" {
				//var err error
				if len(col.DefaultValue) > 2 && col.DefaultValue[0] == col.DefaultValue[len(col.DefaultValue)-1] {
					val = col.DefaultValue[1 : len(col.DefaultValue)-1]
				} else {
					val = col.DefaultValue
				}
			} else {
				continue
			}
		}

		if col.ColumnName == "reference_id" {
			s := val.(string)
			if len(s) > 0 {
				newUuid = s
			} else {
				continue
			}
		}

		if col.IsForeignKey {

			switch col.ForeignKeyData.DataSource {
			case "self":

				//log.Printf("Convert reference_id to id %v[%v]", col.ForeignKeyData.Namespace, val)
				valString, ok := val.(string)
				if !ok {
					log.Errorf("Expected string in foreign key column[%v], found %v", col.ColumnName, val)
					return nil, errors.New("unexpected value in foreign key column")
				}
				var uId interface{}
				var err error
				if valString == "" {
					uId = nil
				} else {
					foreignObjectReferenceId, err := GetReferenceIdToIdWithTransaction(col.ForeignKeyData.Namespace, valString, createTransaction)
					if err != nil {
						return nil, err
					}

					foreignObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(col.ForeignKeyData.Namespace, valString, createTransaction)

					if isAdmin || foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
						uId = foreignObjectReferenceId
					} else {
						log.Printf("User cannot refer this object [%v][%v]", col.ForeignKeyData.Namespace, valString)
						ok = false
					}

				}
				if err != nil {
					return nil, err
				}
				val = uId

			case "cloud_store":

				files, ok := val.([]interface{})
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
							encodedPart = splitParts[1]
						}
						fileBytes, _ := base64.StdEncoding.DecodeString(encodedPart)
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

					uploadActionPerformer, err := NewFileUploadActionPerformer(dbResource.Cruds)
					CheckErr(err, "Failed to create upload action performer")
					log.Printf("created upload action performer")
					if err != nil {
						continue
					}

					actionRequestParameters := make(map[string]interface{})
					actionRequestParameters["file"] = val
					actionRequestParameters["path"] = uploadPath

					log.Printf("Get cloud store details: %v", col.ForeignKeyData.Namespace)
					cloudStore, err := dbResource.GetCloudStoreByNameWithTransaction(col.ForeignKeyData.Namespace, createTransaction)
					CheckErr(err, "Failed to get cloud storage details")
					if err != nil {
						continue
					}

					log.Printf("Cloud storage: %v", cloudStore)

					actionRequestParameters["oauth_token_id"] = cloudStore.OAutoTokenId
					actionRequestParameters["store_provider"] = cloudStore.StoreProvider
					actionRequestParameters["root_path"] = cloudStore.RootPath + "/" + col.ForeignKeyData.KeyName

					log.Printf("Initiate file upload action")
					_, _, errs := uploadActionPerformer.DoAction(Outcome{}, actionRequestParameters, createTransaction)
					if errs != nil && len(errs) > 0 {
						log.Errorf("Failed to upload attachments: %v", errs)
					}
					for i := range files {
						file := files[i].(map[string]interface{})
						delete(file, "file")
						delete(file, "contents")
						files[i] = file
					}
					val, err = json.Marshal(files)
					CheckErr(err, "Failed to marshal file data to column")
				} else {
					val = nil
				}

			default:
				CheckErr(errors.New("undefined foreign key"), "Data source: %v", col.ForeignKeyData.DataSource)

			}

		}
		var err error

		if col.ColumnType == "password" || col.ColumnType == "bcrypt" {
			val, err = BcryptHashString(val.(string))
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				val = ""
			}
		}

		if col.ColumnType == "md5-bcrypt" {
			digest := md5.New()
			digest.Write([]byte(val.(string)))
			hash := fmt.Sprintf("%x", digest.Sum(nil))
			val, err = BcryptHashString(hash)
			if err != nil {
				log.Errorf("Failed to convert string to bcrypt hash, not storing the value: %v", err)
				val = ""
			}
		}

		if col.ColumnType == "md5" {
			digest := md5.New()
			digest.Write([]byte(val.(string)))
			val = fmt.Sprintf("%x", digest.Sum(nil))
		}

		if col.ColumnType == "datetime" {

			// 2017-07-13T18:30:00.000Z
			valString, ok := val.(string)
			if ok {
				val, err = dateparse.ParseLocal(valString)
				CheckErr(err, fmt.Sprintf("Failed to parse string as date time in create [%v] = [%v]", col.ColumnName, val))
			} else {
				floatVal, ok := val.(float64)
				if ok {
					val = time.Unix(int64(floatVal), 0)
					err = nil
				} else {
					int64Val, ok := val.(int64)
					if ok {
						val = time.Unix(int64Val, 0)
						err = nil
					}
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

		} else if col.ColumnType == "measurement" {
			valString, ok := val.(string)
			if ok {

				if val == "" || val == "-" || strings.ToLower(valString) == "na" {
					val = 0
				}
				if BeginsWith(strings.ToLower(col.DataType), "int") {
					floatVal, _ := strconv.ParseFloat(valString, 64)
					intVal := int(floatVal)
					val = fmt.Sprintf("%d", intVal)
				}
			}
		} else if col.ColumnType == "encrypted" {

			secret, err := dbResource.configStore.GetConfigValueForWithTransaction("encryption.secret", "backend", createTransaction)
			if err != nil {
				log.Errorf("Failed to get secret from config: %v", err)
				val = ""
			} else {
				val, err = Encrypt([]byte(secret), val.(string))
				if err != nil {
					log.Errorf("Failed to convert string to encrypted value, not storing the value: %v", err)
					val = ""
				}
			}
		} else if col.ColumnType == "truefalse" {
			valBoolean, ok := val.(bool)
			if ok {
				if valBoolean {
					val = true
				} else {
					val = false
				}
			} else {
				valString, ok := val.(string)
				if ok {
					valueClean := strings.ToLower(strings.TrimSpace(valString))
					if valueClean == "true" || valueClean == "1" {
						val = true
					} else {
						val = false
					}
				}
			}
		}

		dataToInsert[col.ColumnName] = val
		colsList = append(colsList, col.ColumnName)
		valsList = append(valsList, val)
	}

	if !InArray(colsList, "reference_id") {
		colsList = append(colsList, "reference_id")
		valsList = append(valsList, newUuid)
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

	query, vals, err := statementbuilder.Squirrel.Insert(dbResource.model.GetName()).Cols(colsList...).Vals(valsList).ToSQL()

	if err != nil {
		log.Errorf("438 Failed to create insert query: %v", err)
		return nil, err
	}

	_, err = createTransaction.Exec(query, vals...)
	if err != nil {
		log.Errorf("Insert query 437: %v", query)
		//log.Printf("Insert values: %v", vals)
		log.Errorf("Failed to execute insert query 439: %v", err)
		//log.Errorf("%v", vals)
		return nil, err
	}
	createdResource, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.model.GetName(), newUuid, createTransaction)

	if err != nil {
		log.Errorf("[453] Failed to select the newly created entry: %v", err)
		return nil, err
	}

	if len(languagePreferences) > 0 {

		for _, languagePreference := range languagePreferences {

			colsList = append(colsList, "language_id")
			valsList = append(valsList, languagePreference)

			colsList = append(colsList, "translation_reference_id")
			valsList = append(valsList, createdResource["id"])

			query, vals, err := statementbuilder.Squirrel.Insert(dbResource.model.GetName() + "_i18n").Cols(colsList...).Vals(valsList).ToSQL()
			if err != nil {
				log.Errorf("469 Failed to create insert query: %v", err)
				return nil, err
			}

			_, err = createTransaction.Exec(query, vals...)
			if err != nil {
				log.Printf("Insert query 468: %v", query)
				log.Errorf("Failed to execute insert query 469: %v", err)
				log.Errorf("%v", vals)
				return nil, err

			}
		}
	}

	//log.Printf("Created entry: %v", createdResource)

	groupsToAdd := dbResource.defaultGroups
	for _, groupId := range groupsToAdd {
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, _ := statementbuilder.Squirrel.
			Insert(dbResource.model.GetName()+"_"+dbResource.model.GetName()+"_id"+"_has_usergroup_usergroup_id").
			Cols(dbResource.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
			Vals([]interface{}{createdResource["id"], groupId, nuuid, auth.DEFAULT_PERMISSION}).ToSQL()

		log.Infof("Add new object [%v][%v] to usergroup [%v]", dbResource.tableInfo.TableName, createdResource["reference_id"], groupId)
		_, err = createTransaction.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user group relation for [%v]: %v", dbResource.model.GetName(), err)
			return nil, err

		}
	}

	if dbResource.model.GetName() == "usergroup" && sessionUser.UserId != 0 {

		//log.Printf("Associate new usergroup with user: %v", sessionUser.UserId)
		//u, _ := uuid.NewV4()
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

		adminUserId, _ := GetAdminUserIdAndUserGroupId(dbResource.db)
		log.Printf("Associate new user with user: %v", adminUserId)

		belongsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Update(USER_ACCOUNT_TABLE_NAME).
			Set(goqu.Record{USER_ACCOUNT_ID_COLUMN: adminUserId}).
			Where(goqu.Ex{"id": createdResource["id"]}).ToSQL()

		//log.Printf("Query: %v", belogsToUserGroupSql)
		_, err = createTransaction.Exec(belongsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dbResource.model.GetName(), err)
			return nil, err

		}

	}

	for _, rel := range dbResource.tableInfo.Relations {
		if rel.Relation == "has_one" && rel.Object == dbResource.tableInfo.TableName {
			log.Printf("Updating foreign key column [%s] in table %s => %v", rel.ObjectName, rel.SubjectName, createdResource["id"])

			foreignObjectId, ok := attrs[rel.SubjectName]
			if !ok || foreignObjectId == nil {
				continue
			}

			updateRelatedTable, args, err := statementbuilder.Squirrel.Update(rel.Subject).Set(goqu.Record{
				rel.ObjectName: createdResource["id"],
			}).Where(
				goqu.Ex{
					"reference_id": foreignObjectId,
				}).ToSQL()

			if err != nil {
				log.Printf("Failed to create update foreign key sql: %s", err)
				return nil, err
			}

			_, err = createTransaction.Exec(updateRelatedTable, args...)

			if err != nil {
				log.Printf("Zero rows were affected: %v", err)
			}

		}
	}

	delete(createdResource, "id")
	createdResource["__type"] = dbResource.model.GetName()

	return createdResource, nil

}

func (dbResource *DbResource) CreateWithTransaction(obj interface{}, req api2go.Request, transaction *sqlx.Tx) (api2go.Responder, error) {
	data := obj.(*api2go.Api2GoModel)
	//log.Printf("Create object request: [%v] %v", dbResource.model.GetTableName(), data.Data)

	for _, bf := range dbResource.ms.BeforeCreate {
		//log.Printf("Invoke BeforeCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		data.Data["__type"] = dbResource.model.GetName()
		responseData, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{data.Data}, transaction)
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
		//log.Printf("Invoke AfterCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
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
	data := obj.(*api2go.Api2GoModel)
	//log.Printf("Create object request: [%v] %v", dbResource.model.GetTableName(), data.Data)

	transaction, err := dbResource.Connection.Beginx()
	if err != nil {
		return nil, err
	}

	for _, bf := range dbResource.ms.BeforeCreate {
		//log.Printf("Invoke BeforeCreate [%v][%v] on Create Request", bf.String(), dbResource.model.GetName())
		data.Data["__type"] = dbResource.model.GetName()
		responseData, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{data.Data}, transaction)
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
