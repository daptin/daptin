package server

import (
  "github.com/jmoiron/sqlx"
  "strings"
  "fmt"
  log "github.com/Sirupsen/logrus"
  "github.com/artpar/api2go"
  "github.com/satori/go.uuid"
  "encoding/json"
  "github.com/artpar/gocms/datastore"
  "gopkg.in/Masterminds/squirrel.v1"
)

func UpdateWorldColumnTable(initConfig *CmsConfig, db *sqlx.DB) {

  for _, table := range initConfig.Tables {

    var worldid int

    db.QueryRowx("select id from world where table_name = ? and deleted_at is null", table.TableName).Scan(&worldid);

    for _, col := range table.Columns {

      /**
      Name            string
  ColumnName      string
  ColumnType      string
  IsPrimaryKey    bool
  IsAutoIncrement bool
  IsIndexed       bool
  IsUnique        bool
  IsNullable      bool
  IsForeignKey    bool
  ForeignKeyData  ForeignKeyData
  DataType        string
  DefaultValue    string
       */
      //var colInfo api2go.ColumnInfo
      var count int
      err := db.QueryRowx("select count(*) from world_column where world_id = ? and column_name = ? and deleted_at is null", worldid, col.ColumnName).Scan(&count)
      if err != nil || count < 1 {
        log.Infof("No existing row for TableColumn[%v][%v]: %v", table.TableName, col.ColumnName, err)

        mapData := make(map[string]interface{})

        mapData["world_id"] = worldid;
        mapData["reference_id"] = uuid.NewV4().String();
        mapData["permission"] = 0;
        mapData["name"] = col.Name;

        mapData["column_name"] = col.ColumnName;

        mapData["column_type"] = col.ColumnType;
        mapData["is_primary_key"] = col.IsPrimaryKey;
        mapData["is_auto_increment"] = col.IsAutoIncrement;
        mapData["is_indexed"] = col.IsIndexed;
        mapData["is_unique"] = col.IsUnique;
        mapData["is_nullable"] = col.IsNullable;
        mapData["is_foreign_key"] = col.IsForeignKey;
        mapData["include_in_api"] = col.IncludeInApi;
        mapData["foreign_key_data"] = col.ForeignKeyData.String();
        mapData["data_type"] = col.DataType;
        mapData["default_value"] = col.DefaultValue;

        query, args, err := squirrel.Insert("world_column").SetMap(mapData).ToSql()
        if err != nil {
          log.Errorf("Failed to create insert query: %v", err)
        }

        log.Infof("Query for insert: %v", query)

        _, err = db.Exec(query, args...)
        if err != nil {
          log.Errorf("Failed to insert new row in world_column: %v", err)
        }

      }

    }
  }

}

func UpdateWorldTable(initConfig *CmsConfig, db *sqlx.DB) {

  tx := db
  var err error

  //tx.Queryx("SET FOREIGN_KEY_CHECKS=0;")

  var userId int
  var userGroupId int
  var c int
  err = tx.QueryRowx("select count(*) from user where deleted_at is null").Scan(&c)
  CheckErr(err, "Failed to get user count")
  //log.Infof("Current user grou")
  if c < 1 {
    u1 := uuid.NewV4().String()
    _, err = tx.Exec("insert into usergroup (name, user_id, usergroup_id, reference_id, permission) value ('guest group', null, null, ?, 755);", u1)
    CheckErr(err, "Failed to insert usergroup")
    u2 := uuid.NewV4().String()
    _, err = tx.Exec("insert into user (name, email, reference_id, permission) value ('guest', 'guest@cms.go', ?, 755)", u2)
    CheckErr(err, "Failed to insert user")

    err = tx.QueryRowx("select id from user where reference_id = ?", u2).Scan(&userId)
    CheckErr(err, "Failed to select user")
    err = tx.QueryRowx("select id from usergroup where reference_id = ?", u1).Scan(&userGroupId)
    CheckErr(err, "Failed to user group")

    tx.Exec("update user set user_id = ?, usergroup_id = ?", userId, userGroupId)
    tx.Exec("update usergroup set user_id = ?, usergroup_id = ?", userId, userGroupId)
  } else {

    err = tx.QueryRowx("select id from user where deleted_at is null limit 1").Scan(&userId)
    CheckErr(err, "Failed to select user")
    err = tx.QueryRowx("select id from usergroup where  deleted_at is null limit 1").Scan(&userGroupId)
    CheckErr(err, "Failed to user group")
  }

  for i, table := range initConfig.Tables {
    refId := uuid.NewV4().String()
    schema, err := json.Marshal(table)

    var cou int
    tx.QueryRowx("select count(*) from world where table_name = ?", table.TableName).Scan(&cou)
    if cou > 0 {

      var defaultPermission int

      err = tx.QueryRowx("select default_permission from world where table_name = ?  and deleted_at is null", table.TableName).Scan(&defaultPermission)
      if err != nil {
        log.Errorf("Failed to scan default permission for table [%v]: %v", table.TableName, err)
      } else {
        log.Infof("Default permission for [%v]: %v", table.TableName, defaultPermission)
      }

      table.DefaultPermission = defaultPermission
      initConfig.Tables[i] = table


      continue
    }

    _, err = tx.Exec("insert into world (table_name, schema_json, permission, reference_id, user_id, usergroup_id, default_permission) value (?,?,755, ?, ?, ?, 755)", table.TableName, string(schema), refId, userId, userGroupId)
    CheckErr(err, "Failed to insert into world table about " + table.TableName)

  }

  //log.Infof("Completed update world table: %v", initConfig)
  CheckErr(err, "Failed to commit")

}

func CheckErr(err error, message string) {
  if err != nil {
    log.Infof("%v: %v", message, err)
  }
}

func CreateUniqueConstraints(initConfig *CmsConfig, db *sqlx.DB) {
  for _, table := range initConfig.Tables {
    for _, column := range table.Columns {

      if column.IsUnique {
        indexName := "index_" + table.TableName + "_" + column.ColumnName + "_unique"
        alterTable := "alter table " + table.TableName + " add unique index " + indexName + "(" + column.ColumnName + ")"
        log.Infof("Create unique index sql: %v", alterTable)
        _, err := db.Exec(alterTable)
        if err != nil {
          log.Infof("Table[%v] Column[%v]: Failed to create unique index: %v", table.TableName, column.ColumnName, err)
        }
      }
    }
  }
}

func CreateIndexes(initConfig *CmsConfig, db *sqlx.DB) {
  for _, table := range initConfig.Tables {
    for _, column := range table.Columns {

      if column.IsUnique {
        indexName := "index_" + table.TableName + "_" + column.ColumnName + "_index"
        alterTable := "create index " + indexName + " on " + table.TableName + " (" + column.ColumnName + ")"
        log.Infof("Create index sql: %v", alterTable)
        _, err := db.Exec(alterTable)
        if err != nil {
          log.Errorf("Failed to create index on Table[%v] Column[%v]: %v", table.TableName, column.ColumnName, err)
        }
      }
    }
  }
}

func CreateRelations(initConfig *CmsConfig, db *sqlx.DB) {
  for _, table := range initConfig.Tables {
    for _, column := range table.Columns {
      if column.IsForeignKey {
        keyName := table.TableName + "_" + column.ForeignKeyData.TableName + "_" + column.ForeignKeyData.ColumnName + "_fk"
        alterSql := "alter table " + table.TableName + " add constraint " + keyName + " foreign key (" + column.ColumnName + ") references " + column.ForeignKeyData.String()

        _, err := db.Exec(alterSql)
        if err != nil {
          log.Infof("Failed to create foreign key [%v], probably it exists: %v", err, keyName)
        } else {
          log.Infof("Key created [%v][%v]", table.TableName, keyName)
        }
      }
    }
  }
}

func CheckRelations(config *CmsConfig, db *sqlx.DB) {
  relations := config.Relations

  for _, relation := range relations {
    log.Infof("[%v] [%v] [%v]", relation.Subject, relation.Relation, relation.Object)
    switch relation.Relation {
    case "belongs_to":
      fromTable := relation.Subject
      targetTable := relation.Object
      col := api2go.ColumnInfo{
        Name: targetTable + "_id",
        ColumnName: targetTable + "_id",
        IsForeignKey: true,
        ColumnType: "alias",
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }
      for i, t := range config.Tables {
        if t.TableName == fromTable {
          c := t.Columns
          c = append(c, col)
          config.Tables[i].Columns = c
        }
      }
      break

    case "has_many":

      fromTable := relation.Subject
      targetTable := relation.Object

      newTable := datastore.TableInfo{
        TableName: fromTable + "_has_" + targetTable,
        Columns: make([]api2go.ColumnInfo, 0),
      }

      col1 := api2go.ColumnInfo{
        Name: fromTable + "_id",
        ColumnName: fromTable + "_id",
        ColumnType: "alias",
        IsForeignKey: true,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: fromTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col1)

      col2 := api2go.ColumnInfo{
        Name: targetTable + "_id",
        ColumnName: targetTable + "_id",
        ColumnType: "alias",
        IsForeignKey: true,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col2)

      config.Tables = append(config.Tables, newTable)

      break



    case "has_many_and_belongs_to_many":
      fromTable := relation.Subject
      targetTable := relation.Object

      newTable := datastore.TableInfo{
        TableName: fromTable + "_" + targetTable,
        Columns: make([]api2go.ColumnInfo, 0),
      }

      col1 := api2go.ColumnInfo{
        Name: fromTable + "_id",
        ColumnName: fromTable + "_id",
        IsForeignKey: true,
        ColumnType: "alias",
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: fromTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col1)

      col2 := api2go.ColumnInfo{
        Name: targetTable + "_id",
        ColumnName: targetTable + "_id",
        ColumnType: "alias",
        IsForeignKey: true,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col2)

      config.Tables = append(config.Tables, newTable)

      break



    default:
      log.Errorf("Failed to identify relation type: %v", relation)

    }
  }

}

func CheckAllTableStatus(initConfig *CmsConfig, db *sqlx.DB) []datastore.TableInfo {
  tables := []datastore.TableInfo{}
  for _, table := range initConfig.Tables {
    CheckTable(&table, db, initConfig)
    tables = append(tables, table)
  }
  return tables
}

func CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo *datastore.TableInfo) (map[string]bool, map[string]api2go.ColumnInfo) {
  columnsWeWant := map[string]bool{}
  colInfoMap := map[string]api2go.ColumnInfo{}
  for _, c := range tableInfo.Columns {
    columnsWeWant[c.ColumnName] = false
    colInfoMap[c.ColumnName] = c
  }

  for _, sCol := range datastore.StandardColumns {
    _, ok := colInfoMap[sCol.ColumnName]
    if ok {
      log.Infof("Column [%v] already present in config for table [%v]", sCol.ColumnName, tableInfo.TableName)
    } else {
      colInfoMap[sCol.Name] = sCol
      columnsWeWant[sCol.Name] = true
      tableInfo.Columns = append(tableInfo.Columns, sCol)
    }
  }
  return columnsWeWant, colInfoMap
}

func CheckTable(tableInfo *datastore.TableInfo, db *sqlx.DB, initConfig *CmsConfig) {

  columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)

  initConfig.Relations = append(initConfig.Relations, datastore.TableRelation{
    Subject: tableInfo.TableName,
    Relation: "belongs_to",
    Object: "user",
  })

  initConfig.Relations = append(initConfig.Relations, datastore.TableRelation{
    Subject: tableInfo.TableName,
    Relation: "belongs_to",
    Object: "usergroup",
  })

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

      query := alterTableAddColumn(tableInfo.TableName, &info)
      log.Infof("Alter query: %v", query)
      _, err := db.Exec(query)
      if err != nil {
        log.Errorf("Failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
      }
    }
  }
}

func alterTableAddColumn(tableName string, colInfo *api2go.ColumnInfo) string {
  return fmt.Sprintf("alter table %v add column %v", tableName, getColumnLine(colInfo))
}

func CreateTable(tableInfo *datastore.TableInfo, db *sqlx.DB) {

  createTableQuery := makeCreateTableQuery(tableInfo)

  log.Infof("Create table query\n%v", createTableQuery)
  _, err := db.Exec(createTableQuery)
  if err != nil {
    log.Errorf("Failed to create table: %v", err)
  }
}

func makeCreateTableQuery(tableInfo *datastore.TableInfo) string {
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

    columnLine := getColumnLine(&c)

    colsDone[c.ColumnName] = true
    columnStrings = append(columnStrings, columnLine)
  }
  columnString := strings.Join(columnStrings, ",\n  ")
  createTableQuery += columnString + ")";
  return createTableQuery
}

func getColumnLine(c *api2go.ColumnInfo) string {
  columnParams := []string{c.ColumnName, c.DataType}

  if !c.IsNullable {
    columnParams = append(columnParams, "not null")
  } else {
    columnParams = append(columnParams, "null")
  }

  if c.IsAutoIncrement {
    columnParams = append(columnParams, "AUTO_INCREMENT PRIMARY KEY")
  } else if c.IsPrimaryKey {
    columnParams = append(columnParams, "PRIMARY KEY")
  }

  if c.DefaultValue != "" {
    columnParams = append(columnParams, "default " + c.DefaultValue)
  }

  columnLine := strings.Join(columnParams, " ")
  return columnLine
}

