package resource

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
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

var ValidatorInstance *validator.Validate = validator.New()

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
				log.Infof("Relation already exists: %v", relation)
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
	db         database.DatabaseConnection
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

func (c *ConfigStore) SetDefaultEnv(env string) {
	c.defaultEnv = env
}

func (c *ConfigStore) GetConfigValueFor(key string, configtype string) (string, error) {
	var val interface{}

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(squirrel.Eq{"name": key}).
		Where(squirrel.Eq{"configstate": "enabled"}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).
		Where(squirrel.Eq{"configtype": configtype}).ToSql()

	CheckErr(err, "Failed to create config select query")

	err = c.db.QueryRowx(s, v...).Scan(&val)
	if err != nil {
		log.Infof("Failed to scan config value [%v]: %v", key, err)
	}
	return fmt.Sprintf("%s", val), err
}

func (c *ConfigStore) GetConfigIntValueFor(key string, configtype string) (int, error) {
	var val int

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(squirrel.Eq{"name": key}).
		Where(squirrel.Eq{"configstate": "enabled"}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).
		Where(squirrel.Eq{"configtype": configtype}).ToSql()

	CheckErr(err, "Failed to create config select query")

	err = c.db.QueryRowx(s, v...).Scan(&val)
	if err != nil {
		log.Infof("Failed to scan config value: %v", err)
	}
	return val, err
}

func (c *ConfigStore) GetAllConfig() map[string]string {

	s, v, err := statementbuilder.Squirrel.Select("name", "value").
		From(settingsTableName).
		Where(squirrel.Eq{"configstate": "enabled"}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

	CheckErr(err, "Failed to create config select query")

	retMap := make(map[string]string)
	res, err := c.db.Queryx(s, v...)
	if err != nil {
		log.Errorf("Failed to get web config map: %v", err)
	}
	defer res.Close()

	for res.Next() {
		var name, val string
		res.Scan(&name, &val)
		retMap[name] = val
	}

	return retMap

}

func (c *ConfigStore) DeleteConfigValueFor(key string, configtype string) error {

	s, v, err := statementbuilder.Squirrel.Delete(settingsTableName).
		Where(squirrel.Eq{"name": key}).
		Where(squirrel.Eq{"configtype": configtype}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

	CheckErr(err, "Failed to create config insert query")

	_, err = c.db.Exec(s, v...)
	CheckErr(err, "Failed to execute config insert query")
	return err
}

func (c *ConfigStore) SetConfigValueFor(key string, val interface{}, configtype string) error {
	var previousValue string

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(squirrel.Eq{"name": key}).
		Where(squirrel.Eq{"configstate": "enabled"}).
		Where(squirrel.Eq{"configtype": configtype}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

	CheckErr(err, "Failed to create config select query")

	err = c.db.QueryRowx(s, v...).Scan(&previousValue)

	if err != nil {

		// row doesnt exist
		s, v, err := statementbuilder.Squirrel.Insert(settingsTableName).
			Columns("name", "configstate", "configtype", "configenv", "value").
			Values(key, "enabled", configtype, c.defaultEnv, val).ToSql()

		CheckErr(err, "Failed to create config insert query")

		_, err = c.db.Exec(s, v...)
		CheckErr(err, "Failed to execute config insert query")
		return err
	} else {

		// row already exists

		s, v, err := statementbuilder.Squirrel.Update(settingsTableName).
			Set("value", val).
			Set("updated_at", time.Now()).
			Set("previousvalue", previousValue).
			Where(squirrel.Eq{"name": key}).
			Where(squirrel.Eq{"configstate": "enabled"}).
			Where(squirrel.Eq{"configtype": configtype}).
			Where(squirrel.NotEq{"value": val}).
			Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

		CheckErr(err, "Failed to create config insert query")

		_, err = c.db.Exec(s, v...)
		CheckErr(err, "Failed to execute config update query")
		return err
	}

}

func (c *ConfigStore) SetConfigIntValueFor(key string, val int, configtype string) error {
	var previousValue string

	s, v, err := statementbuilder.Squirrel.Select("value").
		From(settingsTableName).
		Where(squirrel.Eq{"name": key}).
		Where(squirrel.Eq{"configstate": "enabled"}).
		Where(squirrel.Eq{"configtype": configtype}).
		Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

	CheckErr(err, "Failed to create config select query")

	err = c.db.QueryRowx(s, v...).Scan(&previousValue)

	if err != nil {

		// row doesnt exist
		s, v, err := statementbuilder.Squirrel.Insert(settingsTableName).
			Columns("name", "configstate", "configtype", "configenv", "value").
			Values(key, "enabled", configtype, c.defaultEnv, val).ToSql()

		CheckErr(err, "Failed to create config insert query")

		_, err = c.db.Exec(s, v...)
		CheckErr(err, "Failed to execute config insert query")
		return err
	} else {

		// row already exists

		s, v, err := statementbuilder.Squirrel.Update(settingsTableName).
			Set("value", val).
			Set("previousvalue", previousValue).
			Set("updated_at", time.Now()).
			Where(squirrel.Eq{"name": key}).
			Where(squirrel.NotEq{"value": val}).
			Where(squirrel.Eq{"configstate": "enabled"}).
			Where(squirrel.Eq{"configtype": configtype}).
			Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

		CheckErr(err, "Failed to create config insert query")

		_, err = c.db.Exec(s, v...)
		CheckErr(err, "Failed to execute config update query")
		return err
	}

}

func NewConfigStore(db database.DatabaseConnection) (*ConfigStore, error) {
	var cs ConfigStore
	s, v, err := statementbuilder.Squirrel.Select("count(*)").From(settingsTableName).ToSql()
	CheckErr(err, "Failed to create sql for config check table")
	if err != nil {
		return &cs, err
	}

	var cou int
	err = db.QueryRowx(s, v...).Scan(&cou)
	if err != nil {
		//log.Infof("Count query failed. Creating table: %v", err)

		createTableQuery := MakeCreateTableQuery(&ConfigTableStructure, db.DriverName())

		_, err = db.Exec(createTableQuery)
		CheckErr(err, "Failed to create config table")
		if err != nil {
			log.Printf("create config table query: %v", createTableQuery)
		}

	}

	return &ConfigStore{
		db:         db,
		defaultEnv: "release",
	}, nil

}
