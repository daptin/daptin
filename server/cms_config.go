package server

import (
  "github.com/artpar/api2go"
  "github.com/artpar/goms/datastore"
  "github.com/artpar/goms/server/resource"
  "github.com/jmoiron/sqlx"
  log "github.com/sirupsen/logrus"
  "gopkg.in/Masterminds/squirrel.v1"
  "time"
)

type CmsConfig struct {
  Tables    []datastore.TableInfo
  Relations []api2go.TableRelation
  Actions   []resource.Action `json:"actions"`
}

type User struct {
  Name       string
  Email      string
  Password   string
  Id         uint64
  CreatedAt  time.Time
  UpdatedAt  time.Time
  Permission int
  Status     string
  DeletedAt  *time.Time `sql:"index"`
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
  db         *sqlx.DB
}

var settingsTableName = "_config"

var ConfigTableStructure = datastore.TableInfo{
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
      DataType:   "varchar(100)",
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
  var val string

  s, v, err := squirrel.Select("value").
      From(settingsTableName).
      Where(squirrel.Eq{"name": key}).
      Where(squirrel.Eq{"configstate": "enabled"}).
      Where(squirrel.Eq{"configenv": c.defaultEnv}).
      Where(squirrel.Eq{"configtype": configtype}).ToSql()

  CheckErr(err, "Failed to create config select query")

  err = c.db.QueryRowx(s, v...).Scan(&val)
  if err != nil {
    log.Infof("Failed to scan config value: ", err)
  }
  return val, err
}

func (c *ConfigStore) GetWebConfig() map[string]string {

  s, v, err := squirrel.Select("name", "value").
      From(settingsTableName).
      Where(squirrel.Eq{"configtype": "web"}).
      Where(squirrel.Eq{"configstate": "enabled"}).
      Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

  CheckErr(err, "Failed to create config select query")

  retMap := make(map[string]string)
  res, err := c.db.Queryx(s, v...)

  for res.Next() {
    var name, val string
    res.Scan(&name, &val)
    retMap[name] = val
  }

  return retMap

}

func (c *ConfigStore) SetConfigValueFor(key string, val string, configtype string) error {
  var previousValue string

  s, v, err := squirrel.Select("value").
      From(settingsTableName).
      Where(squirrel.Eq{"name": key}).
      Where(squirrel.Eq{"configstate": "enabled"}).
      Where(squirrel.Eq{"configtype": configtype}).
      Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

  CheckErr(err, "Failed to create config select query")

  err = c.db.QueryRowx(s, v...).Scan(&val)

  if err != nil {

    // row doesnt exist
    s, v, err := squirrel.Insert(settingsTableName).
        Columns("name", "configstate", "configtype", "configenv", "value").
        Values(key, "enabled", configtype, c.defaultEnv, val).ToSql()

    CheckErr(err, "Failed to create config insert query")

    _, err = c.db.Exec(s, v...)
    CheckErr(err, "Failed to execute config insert query")
    return err
  } else {

    // row already exists

    s, v, err := squirrel.Update(settingsTableName).
        Set("value", val).
        Set("previous_value", previousValue).
        Where(squirrel.Eq{"name": key}).
        Where(squirrel.Eq{"configstate": "enabled"}).
        Where(squirrel.Eq{"configtype": configtype}).
        Where(squirrel.Eq{"configenv": c.defaultEnv}).ToSql()

    CheckErr(err, "Failed to create config insert query")

    _, err = c.db.Exec(s, v...)
    CheckErr(err, "Failed to execute config update query")
    return err
  }

}

func NewConfigStore(db *sqlx.DB) (*ConfigStore, error) {
  var cs ConfigStore
  s, v, err := squirrel.Select("count(*)").From(settingsTableName).ToSql()
  CheckErr(err, "Failed to create sql for config check table")
  if err != nil {
    return &cs, err
  }

  var cou int
  err = db.QueryRowx(s, v...).Scan(&cou)
  if err != nil {
    log.Infof("Count query failed. Creating table: %v", err)

    createTableQuery := MakeCreateTableQuery(&ConfigTableStructure, db.DriverName())

    _, err = db.Exec(createTableQuery)
    CheckErr(err, "Failed to create config table")

  }

  return &ConfigStore{
    db:         db,
    defaultEnv: "release",
  }, nil

}
