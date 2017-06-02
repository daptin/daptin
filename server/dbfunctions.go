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
  //"errors"
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
        mapData["permission"] = 777;
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

  for ; rows.Next(); {
    res := make(map[string]interface{})
    rows.MapScan(res)
    result = append(result, res)
  }

  return result, err
}

func GetActionMapByTypeName(db *sqlx.DB) (map[string]map[string]interface{}, error) {

  allActions, err := GetObjectByWhereClause("action", db)
  if err != nil {
    return nil, err
  }

  typeActionMap := make(map[string]map[string]interface{})

  for _, action := range allActions {
    actioName := string(action["action_name"].([]uint8))
    typeName := string(action["world_id"].([]uint8))

    _, ok := typeActionMap[typeName]
    if !ok {
      typeActionMap[typeName] = make(map[string]interface{})
    }

    _, ok = typeActionMap[typeName][actioName]
    if ok {
      log.Infof("Action already exisys")
    }
    typeActionMap[typeName][actioName] = action
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
    resMap[string(world[col].([]uint8))] = world
  }
  return resMap, err

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

    _, ok = currentActions[string(world["id"].([]uint8))][action.Name]
    if ok {
      log.Infof("Action [%v] on [%v] already present in database", action.Name, action.OnType)
      continue
    } else {

      ifj, _ := json.Marshal(action.InFields)
      ofj, _ := json.Marshal(action.OutFields)

      _, err = db.Exec("insert into action (action_name, label, world_id, in_fields, out_fields, reference_id, permission) value (?,?,?,?,?,?,'755')",
        action.Name, action.Label, world["id"], ifj, ofj, uuid.NewV4().String())
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
  var userCount int
  err = tx.QueryRowx("select count(*) from user where deleted_at is null").Scan(&userCount)
  CheckErr(err, "Failed to get user count")
  //log.Infof("Current user grou")
  if userCount < 1 {

    u2 := uuid.NewV4().String()
    _, err = tx.Exec("insert into user (name, email, reference_id, permission) value ('guest', 'guest@cms.go', ?, 755)", u2)
    CheckErr(err, "Failed to insert user")

    err = tx.QueryRowx("select id from user where reference_id = ?", u2).Scan(&userId)
    CheckErr(err, "Failed to select user")

    u1 := uuid.NewV4().String()
    _, err = tx.Exec("insert into usergroup (name, reference_id, permission) value ('guest group', ?, 755);", u1)
    CheckErr(err, "Failed to insert usergroup")

    err = tx.QueryRowx("select id from usergroup where reference_id = ?", u1).Scan(&userGroupId)
    CheckErr(err, "Failed to user group")

    refIf := uuid.NewV4().String()
    _, err := tx.Exec("insert into user_has_usergroup (user_id, usergroup_id, permission, reference_id) value (?,?,755, ?)", userId, userGroupId, refIf)
    CheckErr(err, "Failed to insert user has usergroup")




    //tx.Exec("update user set user_id = ?, usergroup_id = ?", userId, userGroupId)
    //tx.Exec("update usergroup set user_id = ?, usergroup_id = ?", userId, userGroupId)
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

    _, err = tx.Exec("insert into world (table_name, schema_json, permission, reference_id, default_permission, user_id, is_top_level, is_hidden) value (?,?,777, ?, 644, ?, ?, ?)", table.TableName, string(schema), refId, userId, table.IsTopLevel, table.IsHidden)
    CheckErr(err, "Failed to insert into world table about " + table.TableName)
    initConfig.Tables[i].DefaultPermission = 644

  }

  //log.Infof("Completed update world table: %v", initConfig)
  CheckErr(err, "Failed to commit")

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
          log.Infof("Failed to create index on Table[%v] Column[%v]: %v", table.TableName, column.ColumnName, err)
        }
      }
    }
  }
}

func CreateRelations(initConfig *CmsConfig, db *sqlx.DB) {

  for i, table := range initConfig.Tables {
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
  log.Infof("All relations: %v", relations)

  for i, table := range config.Tables {
    config.Tables[i].IsTopLevel = true

    if table.TableName == "usergroup" {
      continue
    }

    relation := api2go.NewTableRelation(table.TableName, "belongs_to", "user")
    relations = append(relations, relation)

    if table.TableName == "world_column" {
      continue
    }

    relationGroup := api2go.NewTableRelation(table.TableName, "has_many", "usergroup")

    relations = append(relations, relationGroup)

  }
  config.Relations = relations

  for _, relation := range relations {
    relation2 := relation.GetRelation()
    log.Infof("Relation to table [%v]", relation)
    log.Infof("Relation to table [%v] [%v] [%v]", relation.GetSubject(), relation2, relation.GetObject())
    if relation2 == "belongs_to" || relation2 == "has_one" {
      fromTable := relation.GetSubject()
      targetTable := relation.GetObject()

      isNullable := false
      if targetTable == "user" || targetTable == "usergroup" || relation2 == "has_one" {
        isNullable = true
      }

      col := api2go.ColumnInfo{
        Name: relation.GetObject(),
        ColumnName: relation.GetObjectName(),
        IsForeignKey: true,
        ColumnType: "alias",
        IsNullable: isNullable,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }
      noMatch := true
      for i, t := range config.Tables {
        if t.TableName == fromTable {
          noMatch = false
          c := t.Columns
          c = append(c, col)
          log.Infof("Add column [%v] to table [%v]", col.ColumnName, t.TableName)
          config.Tables[i].Columns = c
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

      newTable := datastore.TableInfo{
        TableName: fromTable + "_has_" + targetTable,
        Columns: make([]api2go.ColumnInfo, 0),
      }

      col1 := api2go.ColumnInfo{
        Name: fromTable + "_id",
        ColumnName: relation.GetSubjectName(),
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
        ColumnName: relation.GetObjectName(),
        ColumnType: "alias",
        IsForeignKey: true,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col2)
      newTable.Relations = append(newTable.Relations, relation)
      log.Infof("Add column [%v] to table [%v]", col1.ColumnName, newTable.TableName)
      log.Infof("Add column [%v] to table [%v]", col2.ColumnName, newTable.TableName)

      config.Tables = append(config.Tables, newTable)

    } else if relation2 == "has_many_and_belongs_to_many" {

      fromTable := relation.GetSubject()
      targetTable := relation.GetObject()

      newTable := datastore.TableInfo{
        TableName: fromTable + "_" + targetTable,
        Columns: make([]api2go.ColumnInfo, 0),
      }

      col1 := api2go.ColumnInfo{
        Name: relation.GetSubjectName(),
        ColumnName: relation.GetSubjectName(),
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
        Name: relation.GetObject(),
        ColumnName: relation.GetObjectName(),
        ColumnType: "alias",
        IsForeignKey: true,
        ForeignKeyData: api2go.ForeignKeyData{
          TableName: targetTable,
          ColumnName: "id",
        },
        DataType: "int(11)",
      }

      newTable.Columns = append(newTable.Columns, col2)
      newTable.Relations = append(newTable.Relations, relation)
      log.Infof("Add column [%v] to table [%v]", col1.ColumnName, newTable.TableName)
      log.Infof("Add column [%v] to table [%v]", col2.ColumnName, newTable.TableName)

      config.Tables = append(config.Tables, newTable)

    } else {
      log.Errorf("Failed to identify relation type: %v", relation)
    }

  }
}

func CheckAllTableStatus(initConfig *CmsConfig, db *sqlx.DB) {

  tables := []datastore.TableInfo{}

  for _, table := range initConfig.Tables {
    CheckTable(&table, db, initConfig)
    tables = append(tables, table)
  }
  initConfig.Tables = tables
  return
}

func CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo *datastore.TableInfo) (map[string]bool, map[string]api2go.ColumnInfo) {
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

  for i, c := range tableInfo.Columns {
    if c.ColumnName == "" {
      c.ColumnName = c.Name
      tableInfo.Columns[i].ColumnName = c.Name
    }
  }
  columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)
  log.Infof("Columns we want: %v", columnsWeWant)

  //initConfig.Relations = append(initConfig.Relations, api2go.TableRelation{
  //  Subject: tableInfo.TableName,
  //  Relation: "belongs_to",
  //  Object: "user",
  //})
  //
  //initConfig.Relations = append(initConfig.Relations, api2go.TableRelation{
  //  Subject: tableInfo.TableName,
  //  Relation: "belongs_to",
  //  Object: "usergroup",
  //})

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

