package resource

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v9"
	"time"
)

type CmsConfig struct {
	Tables                   []TableInfo
	EnableGraphQL            bool
	Imports                  []DataFileImport
	StateMachineDescriptions []LoopbookFsmDescription
	Relations                []api2go.TableRelation
	Actions                  []Action
	ExchangeContracts        []ExchangeContract
	Hostname                 string
	SubSites                 map[string]SubSiteInformation
	Tasks                    []Task
	Streams                  []StreamContract
	ActionPerformers         []ActionPerformerInterface
}

var ValidatorInstance = validator.New()

func (ti *CmsConfig) AddRelations(relations ...api2go.TableRelation) {
	if ti.Relations == nil {
		ti.Relations = make([]api2go.TableRelation, 0)
	}

	for _, relation := range relations {
		exists := false
		hash := relation.Hash()

		for _, existingRelation := range ti.Relations {
			if existingRelation.Hash() == hash {
				exists = true
				log.Printf("Relation already exists: %v", relation)
				break
			}
		}

		if !exists {
			ti.Relations = append(ti.Relations, relation)
		}
	}

}

type SubSiteInformation struct {
	SubSite    SubSite
	CloudStore CloudStore
	SourceRoot string
}

type Config struct {
	Name          string
	ConfigType    string // web/backend/mobile
	ConfigState   string // enabled/disabled
	ConfigEnv     string // debug/test/release
	Value         string
	ValueType     string // number/string/byteslice
	PreviousValue string
	UpdatedAt     time.Time
}

type ConfigStore struct {
	defaultEnv string
}

var settingsTableName = "_config"

var ConfigTableStructure = TableInfo{
	TableName: settingsTableName,
	Columns: []api2go.ColumnInfo{
		{
			Name:            "id",
			ColumnName:      "id",
			ColumnType:      "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:       "name",
			ColumnName: "name",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: false,
			IsIndexed:  true,
		},
		{
			Name:       "ConfigType",
			ColumnName: "configtype",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: false,
			IsIndexed:  true,
		},
		{
			Name:       "ConfigState",
			ColumnName: "configstate",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: false,
			IsIndexed:  true,
		},
		{
			Name:       "ConfigEnv",
			ColumnName: "configenv",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: false,
			IsIndexed:  true,
		},
		{
			Name:       "Value",
			ColumnName: "value",
			ColumnType: "string",
			DataType:   "varchar(5000)",
			IsNullable: true,
			IsIndexed:  true,
		},
		{
			Name:       "ValueType",
			ColumnName: "valuetype",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: true,
			IsIndexed:  true,
		},
		{
			Name:       "PreviousValue",
			ColumnName: "previousvalue",
			ColumnType: "string",
			DataType:   "varchar(100)",
			IsNullable: true,
			IsIndexed:  true,
		},
		{
			Name:         "CreatedAt",
			ColumnName:   "created_at",
			ColumnType:   "datetime",
			DataType:     "timestamp",
			DefaultValue: "current_timestamp",
			IsNullable:   false,
			IsIndexed:    true,
		},
		{
			Name:       "UpdatedAt",
			ColumnName: "updated_at",
			ColumnType: "datetime",
			DataType:   "timestamp",
			IsNullable: true,
			IsIndexed:  true,
		},
	},
}

func (configStore *ConfigStore) SetDefaultEnv(env string) {
	configStore.defaultEnv = env
}

func (configStore *ConfigStore) GetConfigValueFor(key string, configtype string, transaction *sqlx.Tx) (string, error) {
	var val interface{}

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).
		Where(goqu.Ex{"configtype": configtype}).ToSQL()

	CheckErr(err, "[180] failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[185] failed to prepare statment: %v", err)
		return "", err
	}
	defer stmt1.Close()
	err = stmt1.QueryRowx(v...).Scan(&val)
	if err != nil {
		log.Printf("[198] No config value set for [%v]: %v", key, err)
		return "", err
	}
	return fmt.Sprintf("%s", val), err
}

func (configStore *ConfigStore) GetConfigValueForWithTransaction(key string, configtype string, transaction *sqlx.Tx) (string, error) {
	var val interface{}

	cacheKey := fmt.Sprintf("config-%v-%v", configtype, key)

	if OlricCache != nil {
		cachedValue, err := OlricCache.Get(cacheKey)
		if err == nil {
			return cachedValue.(string), nil
		}
	}

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).
		Where(goqu.Ex{"configtype": configtype}).ToSQL()

	CheckErr(err, "[180] failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[185] failed to prepare statment: %v", err)
		return "", err
	}
	defer stmt1.Close()
	err = stmt1.QueryRowx(v...).Scan(&val)
	if err != nil {
		log.Printf("[239] No config value set for [%v]: %v", key, err)
		return "", err
	}
	value := fmt.Sprintf("%s", val)

	if OlricCache != nil {
		cachePutErr := OlricCache.PutIfEx(cacheKey, value, 5*time.Minute, olric.IfNotFound)
		CheckErr(cachePutErr, "failed to store config value in cache [%v]", cacheKey)
	}

	return value, err
}

func (configStore *ConfigStore) GetConfigIntValueFor(key string, configtype string, transaction *sqlx.Tx) (int, error) {
	var val int

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).
		Where(goqu.Ex{"configtype": configtype}).ToSQL()

	CheckErr(err, "Failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[209] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&val)
	if err != nil {
		log.Printf("[278] No config value set for [%v]: %v", key, err)
	}
	return val, err
}

func (configStore *ConfigStore) GetAllConfig(transaction *sqlx.Tx) map[string]string {

	s, v, err := statementbuilder.Squirrel.Select("name", "value").
		From(settingsTableName).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

	CheckErr(err, "Failed to create config select query")

	retMap := make(map[string]string)

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[233] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	res, err := stmt1.Queryx(v...)
	if err != nil {
		log.Errorf("Failed to get web config map: %v", err)
	}
	defer func(res *sqlx.Rows) {
		err := res.Close()
		if err != nil {
			log.Errorf("failed to close rows after value scan: %v", err)
		}
	}(res)

	for res.Next() {
		var name, val string
		res.Scan(&name, &val)
		retMap[name] = val
	}

	return retMap

}

func (configStore *ConfigStore) DeleteConfigValueFor(key string, configtype string, transaction *sqlx.Tx) error {

	s, v, err := statementbuilder.Squirrel.Delete(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configtype": configtype}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

	CheckErr(err, "Failed to create config insert query")

	_, err = transaction.Exec(s, v...)
	CheckErr(err, "Failed to execute config insert query")
	return err
}

func (configStore *ConfigStore) SetConfigValueFor(key string, val interface{}, configtype string, transaction *sqlx.Tx) error {
	var previousValue string

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configtype": configtype}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

	CheckErr(err, "Failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[280] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&previousValue)

	if err != nil {

		// row doesnt exist
		s, v, err := statementbuilder.Squirrel.
			Insert(settingsTableName).Cols("name", "configstate", "configtype", "configenv", "value").
			Vals([]interface{}{key, "enabled", configtype, configStore.defaultEnv, val}).ToSQL()

		CheckErr(err, "failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config insert query")
		return err
	} else {

		// row already exists

		s, v, err := statementbuilder.Squirrel.Update(settingsTableName).
			Set(goqu.Record{
				"value":         val,
				"updated_at":    time.Now(),
				"previousvalue": previousValue,
			}).
			Where(goqu.Ex{"name": key}).
			Where(goqu.Ex{"configstate": "enabled"}).
			Where(goqu.Ex{"configtype": configtype}).
			Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

		CheckErr(err, "Failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config update query")
		return err
	}

}

func (configStore *ConfigStore) SetConfigValueForWithTransaction(key string, val interface{}, configtype string, transaction *sqlx.Tx) error {
	var previousValue string

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configtype": configtype}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

	CheckErr(err, "Failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[280] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&previousValue)

	if err != nil {

		// row doesnt exist
		s, v, err := statementbuilder.Squirrel.
			Insert(settingsTableName).Cols("name", "configstate", "configtype", "configenv", "value").
			Vals([]interface{}{key, "enabled", configtype, configStore.defaultEnv, val}).ToSQL()

		CheckErr(err, "failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config insert query")
		return err
	} else {

		// row already exists

		s, v, err := statementbuilder.Squirrel.Update(settingsTableName).
			Set(goqu.Record{
				"value":         val,
				"updated_at":    time.Now(),
				"previousvalue": previousValue,
			}).
			Where(goqu.Ex{"name": key}).
			Where(goqu.Ex{"configstate": "enabled"}).
			Where(goqu.Ex{"configtype": configtype}).
			Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

		CheckErr(err, "Failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config update query")
		return err
	}

}

func (configStore *ConfigStore) SetConfigIntValueFor(key string, val int, configtype string, transaction *sqlx.Tx) error {
	var previousValue string

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(goqu.Ex{"name": key}).
		Where(goqu.Ex{"configstate": "enabled"}).
		Where(goqu.Ex{"configtype": configtype}).
		Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

	CheckErr(err, "Failed to create config select query")

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[336] failed to prepare statment: %v", err)
		return nil
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&previousValue)

	if err != nil {

		// row doesnt exist
		s, v, err := statementbuilder.Squirrel.Insert(settingsTableName).
			Cols("name", "configstate", "configtype", "configenv", "value").
			Vals([]interface{}{key, "enabled", configtype, configStore.defaultEnv, val}).ToSQL()

		CheckErr(err, "Failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config insert query")
		return err
	} else {

		// row already exists

		s, v, err := statementbuilder.Squirrel.Update(settingsTableName).
			Set(goqu.Record{
				"value":         val,
				"previousvalue": previousValue,
				"updated_at":    time.Now(),
			}).
			Where(goqu.Ex{"name": key}).
			Where(goqu.Ex{"configstate": "enabled"}).
			Where(goqu.Ex{"configtype": configtype}).
			Where(goqu.Ex{"configenv": configStore.defaultEnv}).ToSQL()

		CheckErr(err, "Failed to create config insert query")

		_, err = transaction.Exec(s, v...)
		CheckErr(err, "Failed to execute config update query")
		return err
	}

}

func NewConfigStore(db database.DatabaseConnection) (*ConfigStore, error) {
	var cs ConfigStore
	s, _, err := statementbuilder.Squirrel.Select(goqu.COUNT("*")).From(settingsTableName).ToSQL()
	CheckErr(err, "Failed to create sql for config check table")
	if err != nil {
		return &cs, err
	}

	stmt1, err := db.Preparex(s)

	if err != nil {
		//log.Printf("Count query failed. Creating table: %v", err)
		createTableQuery := MakeCreateTableQuery(&ConfigTableStructure, db.DriverName())

		_, err := db.Exec(createTableQuery)
		CheckErr(err, "Failed to create config table")
		if err != nil {
			log.Debugf("create config table query: %v", createTableQuery)
		}

	} else {
		stmt1.Close()
		log.Debugf("Config table already exists")
	}

	return &ConfigStore{
		defaultEnv: "release",
	}, nil

}
