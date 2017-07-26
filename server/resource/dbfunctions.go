package resource

import (
	"encoding/json"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"strings"
	//"errors"
	"github.com/artpar/goms/server/auth"
	"time"
)

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

func GetObjectByWhereClause(objType string, db *sqlx.DB, queries ...squirrel.Eq) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)

	builder := squirrel.Select("*").From(objType).Where(squirrel.Eq{"deleted_at": nil})

	for _, q := range queries {
		builder = builder.Where(q)
	}
	q, v, err := builder.ToSql()

	if err != nil {
		return result, err
	}

	rows, err := db.Queryx(q, v...)

	if err != nil {
		return result, err
	}

	return RowsToMap(rows, objType)
}

func GetActionMapByTypeName(db *sqlx.DB) (map[string]map[string]interface{}, error) {

	allActions, err := GetObjectByWhereClause("action", db)
	if err != nil {
		return nil, err
	}

	typeActionMap := make(map[string]map[string]interface{})

	for _, action := range allActions {
		actioName := action["action_name"].(string)
		worldIdString := fmt.Sprintf("%v", action["world_id"])

		_, ok := typeActionMap[worldIdString]
		if !ok {
			typeActionMap[worldIdString] = make(map[string]interface{})
		}

		_, ok = typeActionMap[worldIdString][actioName]
		if ok {
			log.Infof("Action [%v][%v] already exisys", worldIdString, actioName)
		}
		typeActionMap[worldIdString][actioName] = action
	}

	return typeActionMap, err

}

func GetWorldTableMapBy(col string, db *sqlx.DB) (map[string]map[string]interface{}, error) {

	allWorlds, err := GetObjectByWhereClause("world", db)
	if err != nil {
		return nil, err
	}

	resMap := make(map[string]map[string]interface{})

	for _, world := range allWorlds {
		resMap[world[col].(string)] = world
	}
	return resMap, err

}

//func GetWorldTablesList(col string, db *sqlx.DB) ([]TableInfo, error) {
//
//  allWorlds, err := db.Query("select table_name")
//  if err != nil {
//    return nil, err
//  }
//
//  resMap := make(map[string]map[string]interface{})
//
//  for _, world := range allWorlds {
//    resMap[world[col].(string)] = world
//  }
//  return resMap, err
//
//}

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

func GetAdminUserIdAndUserGroupId(db *sqlx.DB) (int64, int64) {
	var userCount int
	s, v, err := squirrel.Select("count(*)").From("user").Where(squirrel.Eq{"deleted_at": nil}).ToSql()
	err = db.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")

	var userId int64
	var userGroupId int64

	if userCount < 2 {
		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {

		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}
	return userId, userGroupId

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

	defaultWorldPermission := int64(777)

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

			s, v, err = squirrel.Update("world").Set("schema_json", string(schema)).Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
			CheckErr(err, "Failed to create update default permission sql")

			_, err := tx.Exec(s, v...)
			CheckErr(err, fmt.Sprintf("Failed to update json schema for table [%v]: %v", table.TableName, err))

			continue
		}

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

func CheckErr(err error, message string) {
	if err != nil {
		log.Errorf("%v: %v", message, err)
	}
}

func CreateUniqueConstraints(initConfig *CmsConfig, db *sqlx.DB) {
	for _, table := range initConfig.Tables {

		for _, column := range table.Columns {

			if column.IsUnique {
				indexName := "index_" + table.TableName + "_" + column.ColumnName + "_unique"
				alterTable := "create unique index " + indexName + " on " + table.TableName + "(" + column.ColumnName + ")"
				log.Infof("Create unique index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					log.Infof("Table[%v] Column[%v]: Failed to create unique index: %v", table.TableName, column.ColumnName, err)
				}
			}
		}

		if strings.Index(table.TableName, "_has_") > -1 {

			cols := []string{}

			for _, col := range table.Columns {
				if col.IsForeignKey {
					cols = append(cols, col.ColumnName)
				}
			}

			indexName := GetMD5Hash("index_join_" + table.TableName + "_" + "_unique")
			alterTable := "create unique index " + indexName + " on " + table.TableName + "(" + strings.Join(cols, ", ") + ")"
			log.Infof("Create unique index sql: %v", alterTable)
			_, err := db.Exec(alterTable)
			if err != nil {
				log.Infof("Table[%v] Column[%v]: Failed to create unique join index: %v", table.TableName, err)
			}

		}

	}
}

func CreateIndexes(initConfig *CmsConfig, db *sqlx.DB) {
	for _, table := range initConfig.Tables {
		for _, column := range table.Columns {

			if column.IsUnique {
				indexName := "u" + GetMD5Hash("index_"+table.TableName+"_"+column.ColumnName+"_index")
				alterTable := "create unique index " + indexName + " on " + table.TableName + " (" + column.ColumnName + ")"
				//log.Infof("Create index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					//log.Infof("Failed to create index on Table[%v] Column[%v]: %v", table.TableName, column.ColumnName, err)
				}
			} else if column.IsIndexed {
				indexName := "i" + GetMD5Hash("index_"+table.TableName+"_"+column.ColumnName+"_index")
				alterTable := "create index " + indexName + " on " + table.TableName + " (" + column.ColumnName + ")"
				//log.Infof("Create index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					//log.Infof("Failed to create index on Table[%v] Column[%v]: %v", table.TableName, column.ColumnName, err)
				}
			}
		}
	}
}

func CreateRelations(initConfig *CmsConfig, db *sqlx.DB) {

	for i, table := range initConfig.Tables {
		for _, column := range table.Columns {
			if column.IsForeignKey {
				keyName := "fk" + GetMD5Hash(table.TableName+"_"+column.ColumnName+"_"+column.ForeignKeyData.TableName+"_"+column.ForeignKeyData.ColumnName+"_fk")

				if db.DriverName() == "sqlite3" {
					continue
				}

				alterSql := "alter table " + table.TableName + " add constraint " + keyName + " foreign key (" + column.ColumnName + ") references " + column.ForeignKeyData.String()
				log.Infof("Alter table add constraint sql: %v", alterSql)
				_, err := db.Exec(alterSql)
				if err != nil {
					log.Infof("Failed to create foreign key [%v], probably it exists: %v", err, keyName)
				} else {
					log.Infof("Key created [%v][%v]", table.TableName, keyName)
				}
			}
		}

		relations := make([]api2go.TableRelation, 0)

		for _, rel := range initConfig.Relations {
			if rel.GetSubject() == table.TableName || rel.GetObject() == table.TableName {
				relations = append(relations, rel)
			}
		}

		initConfig.Tables[i].Relations = relations
	}
}

func CheckRelations(config *CmsConfig, db *sqlx.DB) {
	relations := config.Relations

	relationsDone := make(map[string]bool)

	for _, relation := range relations {
		relationsDone[relation.Hash()] = true
	}

	newTables := make([]TableInfo, 0)

	for i, table := range config.Tables {
		config.Tables[i].IsTopLevel = true
		existingRelations := config.Tables[i].Relations
		config.Tables[i].Relations = make([]api2go.TableRelation, 0)

		if len(existingRelations) > 0 {
			log.Infof("Found existing %d relations from db for [%v]", len(existingRelations), config.Tables[i].TableName)
			for _, rel := range existingRelations {

				relhash := rel.Hash()
				_, ok := relationsDone[relhash]
				if ok {
					continue
				} else {
					relations = append(relations, rel)

					relationsDone[relhash] = true
				}
			}

			if table.IsStateTrackingEnabled {

				stateRelation := api2go.TableRelation{
					Subject:     table.TableName + "_state",
					SubjectName: table.TableName + "_has_state",
					Object:      table.TableName,
					ObjectName:  "is_state_of_" + table.TableName,
					Relation:    "belongs_to",
				}

				if !relationsDone[stateRelation.Hash()] {

					stateTable := TableInfo{
						TableName: table.TableName + "_state",
						Columns: []api2go.ColumnInfo{
							{
								Name:       "current_state",
								ColumnName: "current_state",
								ColumnType: "label",
								DataType:   "varchar(100)",
								IsNullable: false,
							},
						},
					}

					newTables = append(newTables, stateTable)

					stateTableHasOneDescription := api2go.NewTableRelation(stateTable.TableName, "has_one", "smd")
					stateTableHasOneDescription.SubjectName = table.TableName + "_status"
					stateTableHasOneDescription.ObjectName = table.TableName + "_smd"
					relations = append(relations, stateTableHasOneDescription)
					relationsDone[stateTableHasOneDescription.Hash()] = true
					relationsDone[stateRelation.Hash()] = true
					relations = append(relations, stateRelation)

				}
			}

		} else {

			if table.IsStateTrackingEnabled {
				stateTable := TableInfo{
					TableName: table.TableName + "_state",
					Columns: []api2go.ColumnInfo{
						{
							Name:       "current_state",
							ColumnName: "current_state",
							ColumnType: "label",
							DataType:   "varchar(100)",
							IsNullable: false,
						},
					},
				}

				newTables = append(newTables, stateTable)

				stateTableHasOneDescription := api2go.NewTableRelation(stateTable.TableName, "has_one", "smd")
				stateTableHasOneDescription.SubjectName = table.TableName + "_status"
				stateTableHasOneDescription.ObjectName = table.TableName + "_smd"
				relations = append(relations, stateTableHasOneDescription)
				relationsDone[stateTableHasOneDescription.Hash()] = true

				stateRelation := api2go.TableRelation{
					Subject:     stateTable.TableName,
					SubjectName: table.TableName + "_has_state",
					Object:      table.TableName,
					ObjectName:  "is_state_of_" + table.TableName,
					Relation:    "belongs_to",
				}
				relationsDone[stateRelation.Hash()] = true
				relations = append(relations, stateRelation)
			}

			if table.TableName == "usergroup" {
				continue
			}

			relation := api2go.NewTableRelation(table.TableName, "belongs_to", "user")
			relations = append(relations, relation)
			relationsDone[relation.Hash()] = true

			if table.TableName == "world_column" {
				continue
			}

			relationGroup := api2go.NewTableRelation(table.TableName, "has_many", "usergroup")
			relationsDone[relationGroup.Hash()] = true

			relations = append(relations, relationGroup)

		}

	}

	log.Infof("%d state tables on base entities", len(newTables))
	config.Tables = append(config.Tables, newTables...)

	//newRelations := make([]api2go.TableRelation, 0)
	config.Relations = relations

	for _, relation := range relations {
		relation2 := relation.GetRelation()
		log.Infof("Relation to table [%v]", relation.String())
		if relation2 == "belongs_to" || relation2 == "has_one" {
			fromTable := relation.GetSubject()
			targetTable := relation.GetObject()

			isNullable := false
			if targetTable == "user" || targetTable == "usergroup" || relation2 == "has_one" {
				isNullable = true
			}

			col := api2go.ColumnInfo{
				Name:         relation.GetObject(),
				ColumnName:   relation.GetObjectName(),
				IsForeignKey: true,
				ColumnType:   "alias",
				IsNullable:   isNullable,
				ForeignKeyData: api2go.ForeignKeyData{
					TableName:  targetTable,
					ColumnName: "id",
					DataSource: "self",
				},
				DataType: "int(11)",
			}

			noMatch := true
			for i, t := range config.Tables {
				if t.TableName == fromTable {
					noMatch = false
					c := t.Columns

					exists := false
					for _, c1 := range c {
						if c1.ColumnName == col.ColumnName {
							exists = true
							break
						}
					}

					if !exists {
						c = append(c, col)
						config.Tables[i].Columns = c
					}

					log.Infof("Add column [%v] to table [%v]", col.ColumnName, t.TableName)
					if targetTable != "user" && relation.GetRelation() == "belongs_to" {
						config.Tables[i].IsTopLevel = false
						log.Infof("Table [%v] is not top level == %v", t.TableName, targetTable)
					}
				}

			}
			if noMatch {
				log.Infof("No matching table found: %v", relation)
			}
		} else if relation2 == "has_many" {

			fromTable := relation.GetSubject()
			targetTable := relation.GetObject()

			newTable := TableInfo{
				TableName: relation.GetJoinTableName(),
				Columns:   make([]api2go.ColumnInfo, 0),
			}

			col1 := api2go.ColumnInfo{
				Name:         fromTable + "_id",
				ColumnName:   relation.GetSubjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					DataSource: "self",
					TableName:  fromTable,
					ColumnName: "id",
				},
				DataType: "int(11)",
			}

			newTable.Columns = append(newTable.Columns, col1)

			col2 := api2go.ColumnInfo{
				Name:         targetTable + "_id",
				ColumnName:   relation.GetObjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					TableName:  targetTable,
					DataSource: "self",
					ColumnName: "id",
				},
				DataType: "int(11)",
			}

			newTable.Columns = append(newTable.Columns, col2)
			newTable.Relations = append(newTable.Relations, relation)
			log.Infof("Add column [%v] to table [%v]", col1.ColumnName, newTable.TableName)
			log.Infof("Add column [%v] to table [%v]", col2.ColumnName, newTable.TableName)

			config.Tables = append(config.Tables, newTable)

			if targetTable != "usergroup" {
				stateTable := TableInfo{
					TableName: newTable.TableName + "_state",
					Columns: []api2go.ColumnInfo{
						{
							ColumnName: "state",
							Name:       "state",
							ColumnType: "label",
							DataType:   "varchar(100)",
							IsNullable: false,
						},
						{
							ColumnName:   "smd_id",
							Name:         "smd_id",
							ColumnType:   "alias",
							DataType:     "int(11)",
							IsForeignKey: true,
							IsNullable:   false,
							ForeignKeyData: api2go.ForeignKeyData{
								DataSource: "self",
								TableName:  "smd",
								ColumnName: "id",
							},
						},
						{
							ColumnName:   newTable.TableName + "_id",
							Name:         newTable.TableName + "_id",
							ColumnType:   "alias",
							DataType:     "int(11)",
							IsForeignKey: true,
							IsNullable:   false,
							ForeignKeyData: api2go.ForeignKeyData{
								DataSource: "self",
								TableName:  newTable.TableName,
								ColumnName: "id",
							},
						},
					},
				}
				config.Tables = append(config.Tables, stateTable)
			}

		} else if relation2 == "has_many_and_belongs_to_many" {

			fromTable := relation.GetSubject()
			targetTable := relation.GetObject()

			newTable := TableInfo{
				TableName: relation.GetSubjectName() + "_" + relation.GetObjectName(),
				Columns:   make([]api2go.ColumnInfo, 0),
			}

			col1 := api2go.ColumnInfo{
				Name:         relation.GetSubjectName(),
				ColumnName:   relation.GetSubjectName(),
				IsForeignKey: true,
				ColumnType:   "alias",
				ForeignKeyData: api2go.ForeignKeyData{
					TableName:  fromTable,
					DataSource: "self",
					ColumnName: "id",
				},
				DataType: "int(11)",
			}

			newTable.Columns = append(newTable.Columns, col1)

			col2 := api2go.ColumnInfo{
				Name:         relation.GetObject(),
				ColumnName:   relation.GetObjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					TableName:  targetTable,
					ColumnName: "id",
					DataSource: "self",
				},
				DataType: "int(11)",
			}

			newTable.Columns = append(newTable.Columns, col2)
			newTable.Relations = append(newTable.Relations, relation)
			log.Infof("Add column [%v] to table [%v]", col1.ColumnName, newTable.TableName)
			log.Infof("Add column [%v] to table [%v]", col2.ColumnName, newTable.TableName)

			config.Tables = append(config.Tables, newTable)

			if targetTable != "usergroup" {

				stateTable := TableInfo{
					TableName: newTable.TableName + "_state",
					Columns: []api2go.ColumnInfo{
						{
							ColumnName: "state",
							Name:       "state",
							ColumnType: "label",
							DataType:   "varchar(100)",
							IsNullable: false,
						},
						{
							ColumnName:   "smd_id",
							Name:         "smd_id",
							ColumnType:   "alias",
							IsForeignKey: true,
							DataType:     "int(11)",
							IsNullable:   false,
							ForeignKeyData: api2go.ForeignKeyData{
								TableName:  "smd",
								ColumnName: "id",
								DataSource: "self",
							},
						},
						{
							ColumnName:   newTable.TableName + "_id",
							Name:         newTable.TableName + "_id",
							ColumnType:   "alias",
							DataType:     "int(11)",
							IsForeignKey: true,
							IsNullable:   false,
							ForeignKeyData: api2go.ForeignKeyData{
								TableName:  newTable.TableName,
								ColumnName: "id",
								DataSource: "self",
							},
						},
					},
				}
				config.Tables = append(config.Tables, stateTable)
			}
		} else {
			log.Errorf("Failed to identify relation type: %v", relation)
		}

	}

	//config.Tables[stateMachineDescriptionTableIndex] = stateMachineDescriptionTable

	for _, rela := range relations {
		log.Infof("All relations: %v", rela.String())
	}
}

func CheckAllTableStatus(initConfig *CmsConfig, db *sqlx.DB) {

	tables := []TableInfo{}

	for _, table := range initConfig.Tables {
		CheckTable(&table, db)
		tables = append(tables, table)
	}
	initConfig.Tables = tables
	return
}

func CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo *TableInfo) (map[string]bool, map[string]api2go.ColumnInfo) {
	columnsWeWant := map[string]bool{}
	colInfoMap := map[string]api2go.ColumnInfo{}
	for i, c := range tableInfo.Columns {
		if c.ColumnName == "" {
			c.ColumnName = c.Name
			tableInfo.Columns[i].Name = c.Name
		}
		columnsWeWant[c.ColumnName] = false
		colInfoMap[c.ColumnName] = c
	}

	for _, sCol := range StandardColumns {
		_, ok := colInfoMap[sCol.ColumnName]
		if ok {
			//log.Infof("Column [%v] already present in config for table [%v]", sCol.ColumnName, tableInfo.TableName)
		} else {
			colInfoMap[sCol.Name] = sCol
			columnsWeWant[sCol.Name] = false
			tableInfo.Columns = append(tableInfo.Columns, sCol)
		}
	}
	return columnsWeWant, colInfoMap
}

func CheckTable(tableInfo *TableInfo, db *sqlx.DB) {

	for i, c := range tableInfo.Columns {
		if c.ColumnName == "" {
			c.ColumnName = c.Name
			tableInfo.Columns[i].ColumnName = c.Name
		}
	}
	columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)
	log.Infof("Columns we want in [%v]: %v", tableInfo.TableName, columnsWeWant)

	s := fmt.Sprintf("select * from %s limit 1", tableInfo.TableName)
	//log.Infof("Sql: %v", s)
	columns, err := db.QueryRowx(s).Columns()
	if err != nil {
		log.Infof("Failed to select * from %v: %v", tableInfo.TableName, err)
		CreateTable(tableInfo, db)
		return
	}

	for _, col := range columns {
		present, ok := columnsWeWant[col]
		if !ok {
			log.Infof("extra column [%v] found in table [%v]", col, tableInfo.TableName)
		} else {
			if present {
				log.Infof("Column [%v] already present in table [%v]", col, tableInfo.TableName)
			}
			columnsWeWant[col] = true
		}
	}

	for col, present := range columnsWeWant {

		if !present {
			log.Infof("Column [%v] is not present in table [%v]", col, tableInfo.TableName)
			info := colInfoMap[col]

			if info.DataType == "" {
				log.Infof("No column type known for column: %v", info)
				continue
			}

			query := alterTableAddColumn(tableInfo.TableName, &info, db.DriverName())
			log.Infof("Alter query: %v", query)
			_, err := db.Exec(query)
			if err != nil {
				log.Errorf("Failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
			}
		}
	}
}

func alterTableAddColumn(tableName string, colInfo *api2go.ColumnInfo, sqlDriverName string) string {
	sq := fmt.Sprintf("alter table %v add column %v", tableName, getColumnLine(colInfo, sqlDriverName))

	return sq
}

func CreateTable(tableInfo *TableInfo, db *sqlx.DB) {

	createTableQuery := MakeCreateTableQuery(tableInfo, db.DriverName())

	log.Infof("Create table query\n%v", createTableQuery)
	_, err := db.Exec(createTableQuery)
	if err != nil {
		log.Errorf("Failed to create table: %v", err)
	}
}

func MakeCreateTableQuery(tableInfo *TableInfo, sqlDriverName string) string {
	createTableQuery := fmt.Sprintf("create table %s (\n", tableInfo.TableName)

	columnStrings := []string{}
	colsDone := map[string]bool{}
	for _, c := range tableInfo.Columns {

		if c.ColumnName == "" && c.Name == "" {
			log.Errorf("Column name is null: %v", c)
		}

		if c.ColumnName == "" {
			c.ColumnName = c.Name
		}

		if colsDone[c.ColumnName] {
			continue
		}

		columnLine := getColumnLine(&c, sqlDriverName)

		colsDone[c.ColumnName] = true
		columnStrings = append(columnStrings, columnLine)
	}
	columnString := strings.Join(columnStrings, ",\n  ")
	createTableQuery += columnString + ")"
	return createTableQuery
}

func getColumnLine(c *api2go.ColumnInfo, sqlDriverName string) string {
	columnParams := []string{c.ColumnName, c.DataType}

	if !c.IsNullable {
		columnParams = append(columnParams, "not null")
	} else {
		columnParams = append(columnParams, "null")
	}

	if c.IsAutoIncrement {
		if sqlDriverName == "sqlite3" {
			columnParams = append(columnParams, "PRIMARY KEY")
		} else {
			columnParams = append(columnParams, "AUTO_INCREMENT PRIMARY KEY")
		}
	} else if c.IsPrimaryKey {
		columnParams = append(columnParams, "PRIMARY KEY")
	}

	if c.DefaultValue != "" {
		columnParams = append(columnParams, "default "+c.DefaultValue)
	}

	columnLine := strings.Join(columnParams, " ")
	return columnLine
}
