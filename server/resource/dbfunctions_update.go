package resource

import (
	"gopkg.in/Masterminds/squirrel.v1"
	"github.com/jmoiron/sqlx"
	"time"
	log "github.com/sirupsen/logrus"
	"github.com/satori/go.uuid"
	"github.com/artpar/api2go"
	"fmt"
	"encoding/json"
	"github.com/artpar/goms/server/auth"
)

func (resource *DbResource) UpdateAccessTokenByTokenId(id *int64, accessToken string, expiresIn int64) (error) {

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return err
	}

	accessToken, err = Encrypt([]byte(encryptionSecret), accessToken)
	if err != nil {
		return err
	}

	s, v, err := squirrel.Update("oauth_token").
			Set("access_token", accessToken).
			Set("expires_in", expiresIn).
			Where(squirrel.Eq{"id": id}).ToSql()

	if err != nil {
		return err
	}

	_, err = resource.db.Exec(s, v...)
	return err

}


func (resource *DbResource) UpdateAccessTokenByTokenReferenceId(referenceId string, accessToken string, expiresIn int64) (error) {

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return err
	}

	accessToken, err = Encrypt([]byte(encryptionSecret), accessToken)
	if err != nil {
		return err
	}

	s, v, err := squirrel.Update("oauth_token").
			Set("access_token", accessToken).
			Set("expires_in", expiresIn).
			Where(squirrel.Eq{"reference_id": referenceId}).ToSql()

	if err != nil {
		return err
	}

	_, err = resource.db.Exec(s, v...)
	return err

}


func UpdateExchanges(initConfig *CmsConfig, db *sqlx.DB) {

	log.Infof("We have %d data exchange updates", len(initConfig.ExchangeContracts))

	adminId, _ := GetAdminUserIdAndUserGroupId(db)

	for _, exchange := range initConfig.ExchangeContracts {

		s, v, err := squirrel.Select("reference_id").From("data_exchange").Where(squirrel.Eq{"name": exchange.Name}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()

		if err != nil {
			log.Errorf("Failed to query existing data exchange: %v", err)
			continue
		}

		var referenceId string
		err = db.QueryRowx(s, v...).Scan(&referenceId)

		if err != nil {
			log.Infof("No existing data exchange for  [%v]", exchange.Name)
		}

		if err == nil {

			attrsJson, err := json.Marshal(exchange.Attributes)
			CheckErr(err, "Failed to marshal attributes to json: %v")

			optionsJson, err := json.Marshal(exchange.Options)

			CheckErr(err, "Failed to marshal options to json: %v")
			sourceAttrsJson, err := json.Marshal(exchange.SourceAttributes)
			CheckErr(err, "Failed to marshal source attrs to json")
			targetAttrsJson, err := json.Marshal(exchange.TargetAttributes)
			CheckErr(err, "Failed to marshal target attrs to json")

			s, v, err = squirrel.
			Update("data_exchange").
					Set("source_attributes", sourceAttrsJson).
					Set("source_type", exchange.SourceType).
					Set("target_attributes", targetAttrsJson).
					Set("target_type", exchange.TargetType).
					Set("attributes", attrsJson).
					Set("options", optionsJson).
					Set("updated_at", time.Now()).
					Set("user_id", adminId).
					Where(squirrel.Eq{"reference_id": referenceId}).
					ToSql()

			_, err = db.Exec(s, v...)

			CheckErr(err, "Failed to update exchange row")

		} else {
			attrsJson, err := json.Marshal(exchange.Attributes)
			CheckErr(err, "Failed to marshal attributes to json")

			optionsJson, err := json.Marshal(exchange.Options)
			CheckErr(err, "Failed to marshal options to json")
			sourceAttrsJson, err := json.Marshal(exchange.SourceAttributes)
			CheckErr(err, "Failed to marshal source attributes to json")
			targetAttrsJson, err := json.Marshal(exchange.TargetAttributes)
			CheckErr(err, "Failed to marshal target attributes to json")

			s, v, err = squirrel.
			Insert("data_exchange").
					Columns("permission", "name", "source_attributes", "source_type", "target_attributes", "target_type",
				"attributes", "options", "created_at", "user_id", "reference_id").
					Values(auth.DEFAULT_PERMISSION, exchange.Name, sourceAttrsJson, exchange.SourceType, targetAttrsJson, exchange.TargetType,
				attrsJson, optionsJson, time.Now(), adminId, uuid.NewV4().String()).
					ToSql()

			_, err = db.Exec(s, v...)

			CheckErr(err, "Failed to insert exchange row")

		}

	}

	allExchnages := make([]ExchangeContract, 0)

	s, v, err := squirrel.Select("name", "source_attributes", "source_type", "target_attributes",
		"target_type", "attributes", "options", "oauth_token_id").
			From("data_exchange").Where(squirrel.Eq{"deleted_at": nil}).ToSql()

	rows, err := db.Queryx(s, v...)
	CheckErr(err, "Failed to query existing exchanges")

	if err == nil {
		for ; rows.Next(); {

			var name, source_type, target_type string;
			var attributes, source_attributes, target_attributes, options []byte
			var oauth_token_id *int64

			var ec ExchangeContract
			err = rows.Scan(&name, &source_attributes, &source_type, &target_attributes, &target_type, &attributes, &options, &oauth_token_id)
			CheckErr(err, "Failed to Scan existing exchanges")

			m := make(map[string]interface{})
			err = json.Unmarshal(source_attributes, &m)
			ec.SourceAttributes = m
			CheckErr(err, "Failed to unmarshal source attributes")

			m = make(map[string]interface{})
			err = json.Unmarshal(target_attributes, &m)
			ec.TargetAttributes = m
			CheckErr(err, "Failed to unmarshal target attributes")

			ec.Name = name
			ec.SourceType = source_type
			ec.TargetType = target_type

			var columnMapping []ColumnMap
			err = json.Unmarshal(attributes, &columnMapping)
			CheckErr(err, "Failed to unmarshal column mapping")

			ec.Attributes = columnMapping
			err = json.Unmarshal(options, &ec.Options)
			CheckErr(err, "Failed to unmarshal exchange options")

			if oauth_token_id == nil {
			}

			ec.OauthTokenId = oauth_token_id

			allExchnages = append(allExchnages, ec)
		}
	}

	initConfig.ExchangeContracts = allExchnages

}

func UpdateStateMachineDescriptions(initConfig *CmsConfig, db *sqlx.DB) {

	log.Infof("We have %d state machine descriptions", len(initConfig.StateMachineDescriptions))

	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)

	for i := range initConfig.Tables {
		ar := make([]LoopbookFsmDescription, 0)
		initConfig.Tables[i].StateMachines = ar
	}

	for _, smd := range initConfig.StateMachineDescriptions {

		s, v, err := squirrel.Select("reference_id").From("smd").Where(squirrel.Eq{"name": smd.Name}).ToSql()
		if err != nil {
			log.Errorf("Failed to create select smd query: %v", err)
			continue
		}

		var refId string
		err = db.QueryRowx(s, v...).Scan(&refId)
		if err != nil {

			// no existing row

			eventsDescription, err := json.Marshal(smd.Events)
			if err != nil {
				log.Errorf("Failed to convert to json: %v", err)
				continue
			}

			insertMap := map[string]interface{}{}
			insertMap["name"] = smd.Name
			insertMap["label"] = smd.Label
			insertMap["initial_state"] = smd.InitialState
			insertMap["events"] = eventsDescription
			insertMap["reference_id"] = uuid.NewV4().String()
			insertMap["permission"] = auth.DEFAULT_PERMISSION
			insertMap["user_id"] = adminUserId
			s, v, err := squirrel.Insert("smd").SetMap(insertMap).ToSql()

			if err != nil {
				log.Errorf("Failed to create update smd query: %v", err)
				continue
			}

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to execute update smd query [%v]: %v", s, err)
			}

		} else {
			// no existing row

			eventsDescription, err := json.Marshal(smd.Events)
			if err != nil {
				log.Errorf("Failed to convert to json: %v", err)
				continue
			}

			updateMap := map[string]interface{}{}
			updateMap["name"] = smd.Name
			updateMap["label"] = smd.Label
			updateMap["initial_state"] = smd.InitialState
			updateMap["events"] = eventsDescription
			updateMap["user_id"] = adminUserId
			s, v, err := squirrel.Update("smd").SetMap(updateMap).Where(squirrel.Eq{"reference_id": refId}).ToSql()

			if err != nil {
				log.Errorf("Failed to create update smd query: %v", err)
				continue
			}

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to execute update smd query [%v]: %v", s, err)
			}

		}

	}
}

func UpdateWorldColumnTable(initConfig *CmsConfig, db *sqlx.DB) {

	for i, table := range initConfig.Tables {

		var worldid int

		db.QueryRowx("select id from world where table_name = ? and deleted_at is null", table.TableName).Scan(&worldid)
		for j, col := range table.Columns {

			var colInfo api2go.ColumnInfo
			err := db.QueryRowx("select name, is_unique, data_type, is_indexed, permission, column_type, column_name, column_description, is_nullable, default_value, is_primary_key, is_foreign_key, include_in_api, foreign_key_data, is_auto_increment from world_column where world_id = ? and column_name = ? and deleted_at is null", worldid, col.ColumnName).StructScan(&colInfo)
			if err != nil {
				log.Infof("Failed to scan world column: ", err)
				log.Infof("No existing row for TableColumn[%v][%v]: %v", table.TableName, col.ColumnName, err)

				mapData := make(map[string]interface{})

				mapData["name"] = col.Name
				mapData["world_id"] = worldid
				mapData["is_unique"] = col.IsUnique
				mapData["data_type"] = col.DataType
				mapData["is_indexed"] = col.IsIndexed
				mapData["permission"] = auth.DEFAULT_PERMISSION
				mapData["column_type"] = col.ColumnType
				mapData["column_name"] = col.ColumnName
				mapData["column_description"] = col.ColumnDescription
				mapData["is_nullable"] = col.IsNullable
				mapData["reference_id"] = uuid.NewV4().String()
				mapData["default_value"] = col.DefaultValue
				mapData["is_primary_key"] = col.IsPrimaryKey
				mapData["is_foreign_key"] = col.IsForeignKey
				mapData["include_in_api"] = col.ExcludeFromApi
				mapData["foreign_key_data"] = col.ForeignKeyData.String()
				mapData["is_auto_increment"] = col.IsAutoIncrement
				query, args, err := squirrel.Insert("world_column").SetMap(mapData).ToSql()
				if err != nil {
					log.Errorf("Failed to create insert query: %v", err)
				}

				log.Infof("Query for insert: %v", query)

				_, err = db.Exec(query, args...)
				if err != nil {
					log.Errorf("Failed to insert new row in world_column: %v", err)
				}

			} else {
				//log.Infof("Picked for from db [%v][%v] :  [%v]", table.TableName, colInfo.ColumnName, colInfo.DefaultValue)
				initConfig.Tables[i].Columns[j] = colInfo
			}
		}
	}
}

func UpdateActionTable(initConfig *CmsConfig, db *sqlx.DB) error {

	var err error

	currentActions, err := GetActionMapByTypeName(db)
	if err != nil {
		return err
	}

	worldTableMap, err := GetWorldTableMapBy("table_name", db)
	if err != nil {
		return err
	}

	for _, action := range initConfig.Actions {

		world, ok := worldTableMap[action.OnType]
		if !ok {
			log.Errorf("Action [%v] defined on unknown type [%v]", action.Name, action.OnType)
			continue
		}

		var worldIdString string
		worldId := world["id"]
		worldIdUint8, ok := worldId.([]uint8)
		if !ok {
			worldIdString = fmt.Sprintf("%v", worldId)
		} else {
			worldIdString = string(worldIdUint8)
		}
		_, ok = currentActions[worldIdString][action.Name]
		if ok {
			log.Infof("Action [%v] on [%v] already present in database", action.Name, action.OnType)

			actionJson, err := json.Marshal(action)
			CheckErr(err, "Failed to marshal infields")
			s, v, err := squirrel.Update("action").
					Set("label", action.Label).
					Set("world_id", worldId).
					Set("action_schema", actionJson).
					Set("instance_optional", action.InstanceOptional).Where(squirrel.Eq{"action_name": action.Name}).ToSql()

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to insert action [%v]: %v", action.Name, err)
			}
		} else {

			actionSchema, _ := json.Marshal(action)

			s, v, err := squirrel.Insert("action").Columns(
				"action_name",
				"label",
				"world_id",
				"action_schema",
				"instance_optional",
				"reference_id",
				"permission").Values(
				action.Name,
				action.Label,
				worldId,
				actionSchema,
				action.InstanceOptional,
				uuid.NewV4().String(),
				777).ToSql()

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to insert action [%v]: %v", action.Name, err)
			}

		}

	}

	return nil
}

func UpdateWorldTable(initConfig *CmsConfig, db *sqlx.DB) {

	tx := db
	var err error

	//tx.Queryx("SET FOREIGN_KEY_CHECKS=0;")

	var userId int
	var userGroupId int
	var systemHasNoAdmin = false
	var userCount int
	s, v, err := squirrel.Select("count(*)").From("user").Where(squirrel.Eq{"deleted_at": nil}).ToSql()
	err = tx.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")
	//log.Infof("Current user grou")
	if userCount < 1 {
		systemHasNoAdmin = true
		u2 := uuid.NewV4().String()

		s, v, err := squirrel.Insert("user").Columns("name", "email", "reference_id", "permission").Values("guest", "guest@cms.go", u2, auth.DEFAULT_PERMISSION).ToSql()
		CheckErr(err, "Failed to create insert sql")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user")

		s, v, err = squirrel.Select("id").From("user").Where(squirrel.Eq{"reference_id": u2}).ToSql()
		CheckErr(err, "Failed to create select user sql ")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select user for world update")

		u1 := uuid.NewV4().String()
		s, v, err = squirrel.Insert("usergroup").Columns("name", "reference_id", "permission").Values("guest group", u1, auth.DEFAULT_PERMISSION).ToSql()
		CheckErr(err, "Failed to create insert usergroup sql")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert usergroup")

		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"reference_id": u1}).ToSql()
		CheckErr(err, "Failed to create select usergroup sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")

		refIf := uuid.NewV4().String()
		s, v, err = squirrel.Insert("user_user_id_has_usergroup_usergroup_id").Columns("user_id", "usergroup_id", "permission", "reference_id").Values(userId, userGroupId, auth.DEFAULT_PERMISSION, refIf).ToSql()
		CheckErr(err, "Failed to create insert user has usergroup sql ")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user has usergroup")

		//tx.Exec("update user set user_id = ?, usergroup_id = ?", userId, userGroupId)
		//tx.Exec("update usergroup set user_id = ?, usergroup_id = ?", userId, userGroupId)
	} else if userCount < 2 {
		systemHasNoAdmin = true
		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {

		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}

	defaultWorldPermission := int64(750)

	if systemHasNoAdmin {
		defaultWorldPermission = 777
	}

	for i, table := range initConfig.Tables {
		refId := uuid.NewV4().String()
		schema, err := json.Marshal(table)

		var cou int
		s, v, err := squirrel.Select("count(*)").From("world").Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
		tx.QueryRowx(s, v...).Scan(&cou)

		if cou > 0 {

			//s, v, err = squirrel.Select("default_permission").From("world").Where(squirrel.Eq{"table_name": table.TableName}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
			//CheckErr(err, "Failed to create select default permission sql")
			log.Infof("Update table data [%v] == IsTopLevel[%v], IsHidden[%v]", table.TableName, table.IsTopLevel, table.IsHidden)

			s, v, err = squirrel.Update("world").
					Set("schema_json", string(schema)).
					Set("is_top_level", table.IsTopLevel).
					Set("is_hidden", table.IsHidden).
					Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
			CheckErr(err, "Failed to create update default permission sql")

			_, err := tx.Exec(s, v...)
			CheckErr(err, fmt.Sprintf("Failed to update json schema for table [%v]: %v", table.TableName, err))

			continue
		}

		log.Infof("Insert table data [%v] == IsTopLevel[%v], IsHidden[%v]", table.TableName, table.IsTopLevel, table.IsHidden)

		s, v, err = squirrel.Insert("world").
				Columns("table_name", "schema_json", "permission", "reference_id", "default_permission", "user_id", "is_top_level", "is_hidden").
				Values(table.TableName, string(schema), defaultWorldPermission, refId, defaultWorldPermission, userId, table.IsTopLevel, table.IsHidden).ToSql()
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert into world table about "+table.TableName)
		initConfig.Tables[i].DefaultPermission = defaultWorldPermission

	}

	s, v, err = squirrel.Select("schema_json", "permission", "default_permission", "is_top_level", "is_hidden").
			From("world").
			Where(squirrel.Eq{"deleted_at": nil}).ToSql()

	CheckErr(err, "Failed to scan world table")

	res, err := tx.Queryx(s, v...)

	tables := make([]TableInfo, 0)
	for res.Next() {
		var tabInfo TableInfo
		var tableSchema []byte
		var permission, defaultPermission int64
		var isTopLevel, isHidden bool
		err = res.Scan(&tableSchema, &permission, &defaultPermission, &isTopLevel, &isHidden)
		CheckErr(err, "Failed to scan table info")
		err = json.Unmarshal(tableSchema, &tabInfo)
		CheckErr(err, "Failed to convert json to table schema")
		tabInfo.Permission = permission
		tabInfo.DefaultPermission = defaultPermission
		tabInfo.IsTopLevel = isTopLevel
		tabInfo.IsHidden = isHidden
		tables = append(tables, tabInfo)
	}
	initConfig.Tables = tables

}
