package resource

import (
	"crypto/md5"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	//"reflect"
	"github.com/artpar/go.uuid"
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

const DEFAULT_LANGUAGE = "en"

func NewFromDbResourceWithTransaction(resources *DbResource, tx *sqlx.Tx) *DbResource {

	return &DbResource{
		Cruds:            resources.Cruds,
		configStore:      resources.configStore,
		model:            resources.model,
		db:               tx,
		connection:       resources.connection,
		ActionHandlerMap: resources.ActionHandlerMap,
		contextCache:     resources.contextCache,
		defaultGroups:    resources.defaultGroups,
		ms:               resources.ms,
		tableInfo:        resources.tableInfo,
	}

}

// Create a new object. Newly created object/struct must be in Responder.
// Possible Responder status codes are:
// - 201 Created: Resource was created and needs to be returned
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Resource created with a client generated ID, and no fields were modified by
//   the server

func (dr *DbResource) CreateWithoutFilter(obj interface{}, req api2go.Request) (map[string]interface{}, error) {
	//log.Infof("Create object of type [%v]", dr.model.GetName())
	data := obj.(*api2go.Api2GoModel)
	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}
	adminId := dr.GetAdminReferenceId()
	isAdmin := adminId != "" && adminId == sessionUser.UserReferenceId

	attrs := data.GetAllAsAttributes()

	allColumns := dr.model.GetColumns()

	dataToInsert := make(map[string]interface{})
	u, _ := uuid.NewV4()
	newUuid := u.String()

	var colsList []string
	var valsList []interface{}
	for _, col := range allColumns {

		//log.Infof("Add column: %v", col.ColumnName)
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

		if col.ColumnName == USER_ACCOUNT_ID_COLUMN && dr.model.GetName() != "user_account_user_account_id_has_usergroup_usergroup_id" {
			continue
		}

		//log.Infof("Check column: %v", col.ColumnName)

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

				//log.Infof("Convert reference_id to id %v[%v]", col.ForeignKeyData.Namespace, val)
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
					foreignObject, err := dr.GetReferenceIdToObject(col.ForeignKeyData.Namespace, valString)
					if err != nil {
						return nil, err
					}

					foreignObjectPermission := dr.GetObjectPermissionByReferenceId(col.ForeignKeyData.Namespace, valString)

					if isAdmin || foreignObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
						uId = foreignObject["id"]
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

				uploadActionPerformer, err := NewFileUploadActionPerformer(dr.Cruds)
				CheckErr(err, "Failed to create upload action performer")
				log.Infof("created upload action performer")
				if err != nil {
					continue
				}

				actionRequestParameters := make(map[string]interface{})
				actionRequestParameters["file"] = val

				log.Infof("Get cloud store details: %v", col.ForeignKeyData.Namespace)
				cloudStore, err := dr.GetCloudStoreByName(col.ForeignKeyData.Namespace)
				CheckErr(err, "Failed to get cloud storage details")
				if err != nil {
					continue
				}

				log.Infof("Cloud storage: %v", cloudStore)

				actionRequestParameters["oauth_token_id"] = cloudStore.OAutoTokenId
				actionRequestParameters["store_provider"] = cloudStore.StoreProvider
				actionRequestParameters["root_path"] = cloudStore.RootPath + "/" + col.ForeignKeyData.KeyName

				log.Infof("Initiate file upload action")
				_, _, errs := uploadActionPerformer.DoAction(Outcome{}, actionRequestParameters)
				if errs != nil && len(errs) > 0 {
					log.Errorf("Failed to upload attachments: %v", errs)
				}

				columnAssetCache, ok := dr.AssetFolderCache[dr.tableInfo.TableName][col.ColumnName]
				if ok {
					err = columnAssetCache.UploadFiles(val.([]interface{}))
				}

				files, ok := val.([]interface{})
				if ok {
					for i := range files {
						file := files[i].(map[string]interface{})
						delete(file, "file")
						files[i] = file
					}
					val, err = json.Marshal(files)
				} else {
					val = nil
				}
				CheckErr(err, "Failed to marshal file data to column")

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

				CheckErr(err, fmt.Sprintf("Failed to parse string as date time in create [%v]", val))
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
			}
		} else if col.ColumnType == "encrypted" {

			secret, err := dr.configStore.GetConfigValueFor("encryption.secret", "backend")
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
					val = 1
				} else {
					val = 0
				}
			} else {
				valString, ok := val.(string)
				if ok {
					if strings.ToLower(strings.TrimSpace(valString)) == "true" {
						val = 1
					} else {
						val = 0
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
	if dr.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	colsList = append(colsList, "permission")
	valsList = append(valsList, dr.model.GetDefaultPermission())

	colsList = append(colsList, "created_at")
	valsList = append(valsList, time.Now())

	if sessionUser.UserId != 0 && dr.model.HasColumn(USER_ACCOUNT_ID_COLUMN) && dr.model.GetName() != "user_account_user_account_id_has_usergroup_usergroup_id" {

		colsList = append(colsList, USER_ACCOUNT_ID_COLUMN)
		valsList = append(valsList, sessionUser.UserId)
	}

	query, vals, err := statementbuilder.Squirrel.Insert(dr.model.GetName()).Columns(colsList...).Values(valsList...).ToSql()

	if err != nil {
		log.Errorf("Failed to create insert query: %v", err)
		return nil, err
	}

	_, err = dr.db.Exec(query, vals...)
	if err != nil {
		log.Infof("Insert query: %v", query)
		//log.Infof("Insert values: %v", vals)
		log.Errorf("Failed to execute insert query: %v", err)
		log.Errorf("%v", vals)
		return nil, err
	}
	createdResource, err := dr.GetReferenceIdToObject(dr.model.GetName(), newUuid)

	if err != nil {
		log.Errorf("Failed to select the newly created entry: %v", err)
		return nil, err
	}

	if len(languagePreferences) > 0 {

		for _, languagePreference := range languagePreferences {

			colsList = append(colsList, "language_id")
			valsList = append(valsList, languagePreference)

			colsList = append(colsList, "translation_reference_id")
			valsList = append(valsList, createdResource["id"])

			query, vals, err := statementbuilder.Squirrel.Insert(dr.model.GetName() + "_i18n").Columns(colsList...).Values(valsList...).ToSql()
			if err != nil {
				log.Errorf("Failed to create insert query: %v", err)
				return nil, err
			}

			_, err = dr.db.Exec(query, vals...)
			if err != nil {
				log.Infof("Insert query: %v", query)
				log.Errorf("Failed to execute insert query: %v", err)
				log.Errorf("%v", vals)
				return nil, err
			}
		}
	}

	//log.Infof("Created entry: %v", createdResource)

	userGroupId := dr.GetUserGroupIdByUserId(sessionUser.UserId)

	groupsToAdd := dr.defaultGroups
	for _, groupId := range groupsToAdd {
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Insert(dr.model.GetName() + "_" + dr.model.GetName() + "_id" + "_has_usergroup_usergroup_id").
			Columns(dr.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
			Values(createdResource["id"], groupId, nuuid, auth.DEFAULT_PERMISSION).ToSql()

		//log.Infof("Query for default group belonging: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user group relation for [%v]: %v", dr.model.GetName(), err)
		}
	}

	if userGroupId != 0 && dr.model.HasMany("usergroup") {

		//log.Infof("Associate new entity [%v][%v] with usergroup: %v", dr.model.GetTableName(), createdResource["reference_id"], userGroupId)
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Insert(dr.model.GetName() + "_" + dr.model.GetName() + "_id" + "_has_usergroup_usergroup_id").
			Columns(dr.model.GetName()+"_id", "usergroup_id", "reference_id", "permission").
			Values(createdResource["id"], userGroupId, nuuid, auth.DEFAULT_PERMISSION).ToSql()

		//log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user group relation for [%v]: %v", dr.model.GetName(), err)
		}

	} else if dr.model.GetName() == "usergroup" && sessionUser.UserId != 0 {

		log.Infof("Associate new usergroup with user: %v", sessionUser.UserId)
		u, _ := uuid.NewV4()
		nuuid := u.String()

		belogsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Insert("user_account_user_account_id_has_usergroup_usergroup_id").
			Columns(USER_ACCOUNT_ID_COLUMN, "usergroup_id", "reference_id", "permission").
			Values(sessionUser.UserId, createdResource["id"], nuuid, auth.DEFAULT_PERMISSION).ToSql()
		//log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dr.model.GetName(), err)
		}

	} else if dr.model.GetName() == USER_ACCOUNT_TABLE_NAME {

		adminUserId, _ := GetAdminUserIdAndUserGroupId(dr.db)
		log.Infof("Associate new user with user: %v", adminUserId)

		belogsToUserGroupSql, q, err := statementbuilder.Squirrel.
			Update(USER_ACCOUNT_TABLE_NAME).
			Set(USER_ACCOUNT_ID_COLUMN, adminUserId).
			Where(squirrel.Eq{"id": createdResource["id"]}).ToSql()

		//log.Infof("Query: %v", belogsToUserGroupSql)
		_, err = dr.db.Exec(belogsToUserGroupSql, q...)

		if err != nil {
			log.Errorf("Failed to insert add user relation for usergroup [%v]: %v", dr.model.GetName(), err)
		}

	}

	for _, rel := range dr.tableInfo.Relations {
		if rel.Relation == "has_one" && rel.Object == dr.tableInfo.TableName {
			log.Printf("Need to update foreign key column in table %s", rel.SubjectName)

			foreignObjectId, ok := attrs[rel.SubjectName]
			if !ok || foreignObjectId == nil {
				continue
			}

			updateRelatedTable, args, err := statementbuilder.Squirrel.Update(rel.Subject).Set(rel.ObjectName, createdResource["id"]).Where(
				squirrel.Eq{
					"reference_id": foreignObjectId,
				}).ToSql()

			if err != nil {
				log.Printf("Failed to create update foreign key sql: %s", err)
				continue
			}

			_, err = dr.db.Exec(updateRelatedTable, args...)

			if err != nil {
				log.Printf("Zero rows were affected: %v", err)
			}

		}
	}

	delete(createdResource, "id")
	createdResource["__type"] = dr.model.GetName()

	return createdResource, nil

}

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

	createdResource, err := dr.CreateWithoutFilter(obj, req)
	if err != nil {
		return NewResponse(nil, nil, 500, nil), err
	}

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

	n1 := dr.model.GetName()
	c1 := dr.model.GetColumns()
	p1 := dr.model.GetDefaultPermission()
	r1 := dr.model.GetRelations()
	return NewResponse(nil,
		api2go.NewApi2GoModelWithData(n1, c1, p1, r1, createdResource),
		201, nil,
	), nil

}
