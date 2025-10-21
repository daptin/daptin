package resource

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jinzhu/copier"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strings"
)

func CreateUniqueConstraints(initConfig *CmsConfig, db *sqlx.Tx) {
	log.Printf("Create constraints and indexes")

	existingIndexes := GetExistingIndexes(db)

	for _, table := range initConfig.Tables {

		//for _, column := range table.Columns {
		//
		//	if column.IsUnique {
		//		indexName := "i" + GetMD5Hash(table.TableName+"_"+column.ColumnName+"_unique")
		//		alterTable := "create unique index " + indexName + " on " + table.TableName + "(" + column.ColumnName + ")"
		//		//log.Printf("Create unique index sql: %v", alterTable)
		//		_, err := db.Exec(alterTable)
		//		if err != nil {
		//			log.Printf("Table[%v] Column[%v]: Failed to create unique index: %v", table.TableName, column.ColumnName, err)
		//		}
		//	}
		//}

		if len(table.CompositeKeys) > 0 {
			for _, compositeKeyCols := range table.CompositeKeys {
				indexName := "i" + GetMD5HashString("index_cl_"+strings.Join(compositeKeyCols, ",")+"_unique")

				if existingIndexes[indexName] {
					continue
				}
				alterTable := "create unique index " + indexName + " on " + table.TableName + "(" + strings.Join(compositeKeyCols, ",") + ")"
				//log.Printf("Create unique index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					log.Errorf("Table[%v] Column[%v]: Failed to create unique composite key index: %v", table.TableName, compositeKeyCols, err)
					log.Errorf("Create unique index sql: %v", alterTable)
					db.Exec("COMMIT ")
				}
			}
		}

		if strings.Index(table.TableName, "_has_") > -1 {

			var cols []string

			for _, col := range table.Columns {
				if col.IsForeignKey {
					cols = append(cols, col.ColumnName)
				}
			}

			if len(cols) < 1 {
				log.Printf("No foreign keys in %v", table.TableName)
				continue
			}

			indexName := "i" + GetMD5HashString("index_join_"+table.TableName+"_"+"_unique")
			if existingIndexes[indexName] {
				continue
			}

			alterTable := "create unique index " + indexName + " on " + table.TableName + "(" + strings.Join(cols, ", ") + ")"
			//log.Printf("Create unique index sql: %v", alterTable)
			_, err := db.Exec(alterTable)
			if err != nil {
				log.Debugf("Table[%v] Column[%v]: unique join index already exists: %v", table.TableName, cols, err)
				db.Exec("COMMIT ")
			}
		}
	}
}

func CreateIndexes(initConfig *CmsConfig, db database.DatabaseConnection) {
	log.Infof("Create indexes")

	transaction, err := db.Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction for CreateIndexes [88]")
	}
	existingIndexes := GetExistingIndexes(transaction)
	err = transaction.Rollback()
	if err != nil {
		CheckErr(err, "TX rollback failed")
	}

	for _, table := range initConfig.Tables {
		for _, column := range table.Columns {

			if column.IsUnique {
				indexName := "u" + GetMD5HashString("index_"+table.TableName+"_"+column.ColumnName+"_unique")
				if existingIndexes[indexName] {
					continue
				}
				alterTable := "create unique index " + indexName + " on " + table.TableName + " (" + column.ColumnName + ")"
				//log.Infof("Create index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					log.Debugf("[108] New index not created on Table[%v][%v]: %v", table.TableName, column.ColumnName, err)
				}
			} else if column.IsIndexed {
				indexName := "i" + GetMD5HashString("index_"+table.TableName+"_"+column.ColumnName+"_index")
				if existingIndexes[indexName] {
					continue
				}

				alterTable := "create index " + indexName + " on " + table.TableName + " (" + column.ColumnName + ")"
				//log.Infof("Create index sql: %v", alterTable)
				_, err := db.Exec(alterTable)
				if err != nil {
					log.Debugf("[120] New index not created on Table[%v] Column[%v]: %v", table.TableName, column.ColumnName, err)
				}
			}
		}
	}
}

func GetExistingIndexes(db *sqlx.Tx) map[string]bool {

	existingIndexes := make(map[string]bool)

	indexQuery := ""
	if db.DriverName() == "mysql" {
		indexQuery = `SELECT DISTINCT INDEX_NAME FROM INFORMATION_SCHEMA.STATISTICS union SELECT   CONSTRAINT_NAME FROM   INFORMATION_SCHEMA.KEY_COLUMN_USAGE`
	} else if db.DriverName() == "postgres" {
		indexQuery = `SELECT
    indexname
FROM
    pg_indexes
WHERE
    schemaname = 'public' union SELECT conname  FROM pg_catalog.pg_constraint con`
	} else if db.DriverName() == "sqlite3" {
		return existingIndexes
	}

	stmt1, err := db.Preparex(indexQuery)
	log.Infof("\tstmt1, err := db.Preparex(indexQuery)\n")
	if err != nil {
		log.Errorf("[877] failed to prepare statment: %v", err)
		return nil
	}
	defer stmt1.Close()

	rows, err := stmt1.Queryx()
	CheckInfo(err, "Failed to check existing indexes using sql [%v][%v]", db.DriverName(), indexQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var indexName string
			err = rows.Scan(&indexName)
			if err == nil {
				existingIndexes[indexName] = true
			} else {
				CheckErr(err, "Failed to scan existing index name")
			}
		}
	}
	return existingIndexes

}

func CreateRelations(initConfig *CmsConfig, db database.DatabaseConnection) {
	log.Printf("Create relations")

	transaction, err := db.Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [176]")
	}

	existingIndexes := GetExistingIndexes(transaction)

	for i, table := range initConfig.Tables {
		if len(table.TableName) < 1 {
			continue
		}
		for _, column := range table.Columns {
			if column.IsForeignKey && column.ForeignKeyData.DataSource == "self" {
				keyName := "fk" + GetMD5HashString(table.TableName+"_"+column.ColumnName+"_"+column.ForeignKeyData.Namespace+"_"+column.ForeignKeyData.KeyName+"_fk")

				if existingIndexes[keyName] {
					continue
				}

				if db.DriverName() == "sqlite3" {
					continue
				}

				alterSql := "alter table " + table.TableName + " add constraint " + keyName + " foreign key (" + column.ColumnName + ") references " + column.ForeignKeyData.String()
				//log.Printf("Alter table add constraint sql: %v", alterSql)
				_, err := db.Exec(alterSql)
				if err != nil {
					log.Printf("Failed to create foreign key [%v],  %v on column [%v][%v]", err, keyName, table.TableName, column.ColumnName)
					transaction.Rollback()
					transaction, err = db.Beginx()
					CheckErr(err, "Failed to create a new transaction after rollback.")
				} else {
					log.Infof("Key created [%v][%v]", table.TableName, keyName)
				}

				fkIndexName := fmt.Sprintf("index_fk_%s_%s", table.TableName, column.ColumnName)
				createFkIndex := "create index " + fkIndexName + " on " + table.TableName + " (" + column.ColumnName + ") "
				//log.Printf("Alter table add constraint sql: %v", alterSql)
				_, err = db.Exec(createFkIndex)
				if err != nil {
					log.Printf("Failed to create foreign key index [%v],  %v on column [%v][%v]", err, fkIndexName, table.TableName, column.ColumnName)
					transaction.Rollback()
					transaction, err = db.Beginx()
					CheckErr(err, "Failed to create a new transaction after rollback.")
				} else {
					log.Infof("Index on FK created [%v][%v]", table.TableName, fkIndexName)
				}

			}
		}

		relations := make([]api2go.TableRelation, 0)

		for _, rel := range initConfig.Relations {
			if rel.GetSubject() == table.TableName || rel.GetObject() == table.TableName {
				relations = append(relations, rel)
			}
		}

		//initConfig.Tables[i].AddRelation(relations...)
		// reset relations
		initConfig.Tables[i].Relations = relations
	}
}

func CheckTranslationTables(config *CmsConfig) {

	newRelations := make([]api2go.TableRelation, 0)

	tableMap := make(map[string]*table_info.TableInfo)
	for i := range config.Tables {
		t := config.Tables[i]
		tableMap[t.TableName] = &t
	}

	createTranslationTableFor := make([]string, 0)
	updateTranslationTableFor := make([]string, 0)

	for _, table := range config.Tables {

		if api2go.EndsWithCheck(table.TableName, "_audit") {
			log.Printf("[%v] is an audit table", table.TableName)
			continue
		}

		if api2go.EndsWithCheck(table.TableName, "_i18n") {
			log.Printf("[%v] is an audit table", table.TableName)
			continue
		}

		translationTableName := table.TableName + "_i18n"
		existingTranslationTable, ok := tableMap[translationTableName]
		if !ok {
			if table.TranslationsEnabled {
				createTranslationTableFor = append(createTranslationTableFor, table.TableName)
			}
		} else {
			if len(table.Columns) > len(existingTranslationTable.Columns) {
				log.Printf("New columns added to the table, translation table need to be updated")
				updateTranslationTableFor = append(updateTranslationTableFor, table.TableName)
			}
		}

	}

	for _, tableName := range createTranslationTableFor {

		table := tableMap[tableName]
		columnsCopy := make([]api2go.ColumnInfo, 0)
		translationTableName := tableName + "_i18n"
		log.Printf("Create translation table [%s] for table [%v]", table.TableName, translationTableName)

		for _, col := range table.Columns {

			var c api2go.ColumnInfo
			err := copier.Copy(&c, &col)
			if err != nil {
				log.Errorf("Failed to copy columns for audit table: %v", err)
				continue
			}

			if c.ColumnName == "id" {
				continue
			}

			c.IsNullable = true

			if c.IsForeignKey {
				c.IsForeignKey = false
				c.ForeignKeyData = api2go.ForeignKeyData{}
			}

			c.IsUnique = false
			c.IsPrimaryKey = false
			c.IsAutoIncrement = false

			//log.Printf("Add column to table [%v] == [%v]", translationTableName, c)
			columnsCopy = append(columnsCopy, c)

		}

		columnsCopy = append(columnsCopy, api2go.ColumnInfo{
			Name:       "language_id",
			ColumnType: "label",
			DataType:   "varchar(10)",
			IsNullable: false,
		})

		newRelation := api2go.TableRelation{
			Subject:    translationTableName,
			Relation:   "belongs_to",
			Object:     tableName,
			ObjectName: "translation_reference_id",
		}

		newRelations = append(newRelations, newRelation)

		newTable := table_info.TableInfo{
			TableName:         translationTableName,
			Columns:           columnsCopy,
			IsHidden:          true,
			DefaultPermission: auth.GuestCreate | auth.GuestRead | auth.GroupRead,
			Permission:        auth.GuestCreate | auth.UserCreate | auth.GroupCreate,
		}

		config.Tables = append(config.Tables, newTable)
	}

	log.Printf("%d Translation tables are new", len(createTranslationTableFor))
	log.Printf("%d Translation tables are updated", len(updateTranslationTableFor))

	for _, tableName := range updateTranslationTableFor {

		table := tableMap[tableName]
		auditTable := tableMap[tableName+"_audit"]
		existingColumns := auditTable.Columns

		existingColumnMap := make(map[string]api2go.ColumnInfo)
		for _, col := range existingColumns {
			existingColumnMap[col.Name] = col
		}

		tableColumnMap := make(map[string]api2go.ColumnInfo)
		for _, col := range table.Columns {
			tableColumnMap[col.Name] = col
		}

		newColsToAdd := make([]api2go.ColumnInfo, 0)

		for _, newCols := range table.Columns {

			_, ok := existingColumnMap[newCols.Name]
			if !ok {
				var newAuditCol api2go.ColumnInfo
				err := copier.Copy(&newAuditCol, &newCols)
				CheckErr(err, "Error while copying value from new audit column")
				newColsToAdd = append(newColsToAdd, newAuditCol)
			}

		}

		if len(newColsToAdd) > 0 {

			for i := range config.Tables {

				if config.Tables[i].TableName == auditTable.TableName {
					config.Tables[i].Columns = append(config.Tables[i].Columns, newColsToAdd...)
				}
			}

		}

	}

	convertRelationsToColumns(newRelations, config)

}

func CheckAuditTables(config *CmsConfig) {

	newRelations := make([]api2go.TableRelation, 0)

	tableMap := make(map[string]*table_info.TableInfo)
	for i := range config.Tables {
		t := config.Tables[i]
		tableMap[t.TableName] = &t
	}

	createAuditTableFor := make([]string, 0)
	updateAuditTableFor := make([]string, 0)

	for _, table := range config.Tables {

		if api2go.EndsWithCheck(table.TableName, "_audit") {
			log.Printf("[%v] is an audit table", table.TableName)
			continue
		}

		auditTableName := table.TableName + "_audit"
		existingAuditTable, ok := tableMap[auditTableName]
		if !ok {
			if table.IsAuditEnabled {
				createAuditTableFor = append(createAuditTableFor, table.TableName)
			}
		} else {
			if len(table.Columns) > len(existingAuditTable.Columns) {
				log.Printf("New columns added to the table, audit table need to be updated")
				updateAuditTableFor = append(updateAuditTableFor, table.TableName)
			}
		}

	}

	for _, tableName := range createAuditTableFor {

		table := tableMap[tableName]
		columnsCopy := make([]api2go.ColumnInfo, 0)
		auditTableName := tableName + "_audit"
		log.Printf("Create audit table [%s] for table [%v]", table.TableName, auditTableName)

		for _, col := range table.Columns {

			var c api2go.ColumnInfo
			err := copier.Copy(&c, &col)
			if err != nil {
				log.Errorf("Failed to copy columns for audit table: %v", err)
				continue
			}

			if c.ColumnName == "id" {
				continue
			}

			if c.ColumnType == "datetime" {
				c.IsNullable = true
			}

			if c.IsForeignKey {
				c.IsForeignKey = false
				c.ForeignKeyData = api2go.ForeignKeyData{}
				c.DataType = "varchar"
			}

			c.IsUnique = false
			c.IsPrimaryKey = false
			c.IsAutoIncrement = false

			//log.Printf("Add column to table [%v] == [%v]", auditTableName, c)
			columnsCopy = append(columnsCopy, c)

		}

		columnsCopy = append(columnsCopy, api2go.ColumnInfo{
			Name:       "source_reference_id",
			ColumnName: "source_reference_id",
			ColumnType: "label",
			DataType:   "varchar(64)",
			IsNullable: false,
		})

		//newRelation := api2go.TableRelation{
		//	Subject:    auditTableName,
		//	Relation:   "belongs_to",
		//	Object:     tableName,
		//	ObjectName: "audit_object_id",
		//}

		//newRelations = append(newRelations, newRelation)

		newTable := table_info.TableInfo{
			TableName:         auditTableName,
			Columns:           columnsCopy,
			IsHidden:          true,
			DefaultPermission: auth.GuestCreate | auth.GuestRead | auth.GroupRead,
			Permission:        auth.GuestCreate | auth.UserCreate | auth.GroupCreate,
		}

		config.Tables = append(config.Tables, newTable)
	}

	log.Infof("%d Audit tables are new", len(createAuditTableFor))
	log.Infof("%d Audit tables are updated", len(updateAuditTableFor))

	for _, tableName := range updateAuditTableFor {

		table := tableMap[tableName]
		auditTable := tableMap[tableName+"_audit"]
		existingColumns := auditTable.Columns

		existingColumnMap := make(map[string]api2go.ColumnInfo)
		for _, col := range existingColumns {
			existingColumnMap[col.Name] = col
		}

		tableColumnMap := make(map[string]api2go.ColumnInfo)
		for _, col := range table.Columns {
			tableColumnMap[col.Name] = col
		}

		newColsToAdd := make([]api2go.ColumnInfo, 0)

		for _, newCols := range table.Columns {

			_, ok := existingColumnMap[newCols.Name]
			if !ok {
				var newAuditCol api2go.ColumnInfo
				copier.Copy(&newAuditCol, &newCols)
				newColsToAdd = append(newColsToAdd, newAuditCol)
			}

		}

		if len(newColsToAdd) > 0 {

			for i := range config.Tables {

				if config.Tables[i].TableName == auditTable.TableName {
					config.Tables[i].Columns = append(config.Tables[i].Columns, newColsToAdd...)
				}
			}

		}

	}

	convertRelationsToColumns(newRelations, config)

}

func convertRelationsToColumns(relations []api2go.TableRelation, config *CmsConfig) {
	existingRelationMap := make(map[string]bool)
	tableMap := make(map[string]*table_info.TableInfo)
	for _, table := range config.Tables {
		tableMap[table.TableName] = &table
	}

	for _, rel := range config.Relations {
		existingRelationMap[rel.Hash()] = true
	}

	for _, relation := range relations {

		if existingRelationMap[relation.Hash()] {
			//log.Printf("Relation [%v] is already registered", relation.String())
			continue
		}
		//log.Printf("Register relation [%v]", relation.String())
		//config.Relations = append(config.Relations, relation)
		config.AddRelations(relation)
		existingRelationMap[relation.Hash()] = true

		relation2 := relation.GetRelation()
		//log.Printf("Relation to table [%v]", relation.String())
		if relation2 == "belongs_to" || relation2 == "has_one" {
			fromTable := relation.Subject
			targetTable := relation.Object

			//log.Printf("From table [%v] to table [%v]", fromTable, targetTable)
			isNullable := false
			if targetTable == USER_ACCOUNT_TABLE_NAME || targetTable == "usergroup" || relation2 == "has_one" {
				isNullable = true
			}

			col := api2go.ColumnInfo{
				Name:         relation.GetObject(),
				ColumnName:   relation.GetObjectName(),
				IsForeignKey: true,
				ColumnType:   "alias",
				IsNullable:   isNullable,
				ForeignKeyData: api2go.ForeignKeyData{
					Namespace:  targetTable,
					KeyName:    "id",
					DataSource: "self",
				},
				DataType: "int(11)",
			}

			noMatch := true

			// there are going to be 2 tables sometimes which will be marked as "not top tables", so we cannot break after first match
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
						config.Tables[i].Columns = append(config.Tables[i].Columns, relation.Columns...)
					}

					//log.Printf("Add column [%v] to table [%v]", col.ColumnName, t.TableName)
					if targetTable != USER_ACCOUNT_TABLE_NAME && relation.GetRelation() == "belongs_to" {
						//config.Tables[i].IsTopLevel = false
						//log.Printf("Table [%v] is not top level == %v", t.TableName, targetTable)
					}
					config.Tables[i].AddRelation(relation)
				}

			}
			if noMatch {
				//newTable := TableInfo{
				//	TableName: fromTable,
				//	Columns:   []api2go.ColumnInfo{col},
				//}
				//config.Tables = append(config.Tables, fromTable)
				log.Errorf("No matching table found for relation: %v", relation)
				log.Errorf("Created new table: %v", fromTable)
			}
		} else if relation2 == "has_many" {

			fromTable := relation.GetSubject()
			targetTable := relation.GetObject()

			newJoinTable := table_info.TableInfo{
				TableName:   relation.GetJoinTableName(),
				Columns:     make([]api2go.ColumnInfo, 0),
				IsJoinTable: true,
				IsTopLevel:  false,
			}

			col1 := api2go.ColumnInfo{
				Name:         fromTable + "_id",
				ColumnName:   relation.GetSubjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					DataSource: "self",
					Namespace:  fromTable,
					KeyName:    "id",
				},
				DataType: "int(11)",
			}

			newJoinTable.Columns = append(newJoinTable.Columns, col1)

			col2 := api2go.ColumnInfo{
				Name:         targetTable + "_id",
				ColumnName:   relation.GetObjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					Namespace:  targetTable,
					DataSource: "self",
					KeyName:    "id",
				},
				DataType: "int(11)",
			}

			newJoinTable.Columns = append(newJoinTable.Columns, col2)
			newJoinTable.Columns = append(newJoinTable.Columns, relation.Columns...)
			tableMap[fromTable].AddRelation(relation)
			tableMap[targetTable].AddRelation(relation)
			//newJoinTable.Relations = append(newJoinTable.Relations, relation)
			//log.Printf("Add column [%v] to table [%v]", col1.ColumnName, newJoinTable.TableName)
			//log.Printf("Add column [%v] to table [%v]", col2.ColumnName, newJoinTable.TableName)

			config.Tables = append(config.Tables, newJoinTable)

		} else if relation2 == "has_many_and_belongs_to_many" {

			fromTable := relation.GetSubject()
			targetTable := relation.GetObject()

			newJoinTable := table_info.TableInfo{
				TableName: relation.GetJoinTableName(),
				Columns:   make([]api2go.ColumnInfo, 0),
			}

			col1 := api2go.ColumnInfo{
				Name:         relation.GetSubjectName(),
				ColumnName:   relation.GetSubjectName(),
				IsForeignKey: true,
				ColumnType:   "alias",
				ForeignKeyData: api2go.ForeignKeyData{
					Namespace:  fromTable,
					DataSource: "self",
					KeyName:    "id",
				},
				DataType: "int(11)",
			}

			newJoinTable.Columns = append(newJoinTable.Columns, col1)

			col2 := api2go.ColumnInfo{
				Name:         relation.GetObject(),
				ColumnName:   relation.GetObjectName(),
				ColumnType:   "alias",
				IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{
					Namespace:  targetTable,
					KeyName:    "id",
					DataSource: "self",
				},
				DataType: "int(11)",
			}

			newJoinTable.Columns = append(newJoinTable.Columns, col2)
			newJoinTable.Columns = append(newJoinTable.Columns, relation.Columns...)
			tableMap[fromTable].AddRelation(relation)
			tableMap[targetTable].AddRelation(relation)

			//newJoinTable.Relations = append(newJoinTable.Relations, relation)
			//log.Printf("Add column [%v] to table [%v]", col1.ColumnName, newJoinTable.TableName)
			//log.Printf("Add column [%v] to table [%v]", col2.ColumnName, newJoinTable.TableName)

			config.Tables = append(config.Tables, newJoinTable)

		} else {
			log.Errorf("Failed to identify relation type: %v", relation)
		}

	}

}

func alterTableAddColumn(tableName string, colInfo *api2go.ColumnInfo, sqlDriverName string) string {
	sq := fmt.Sprintf("alter table %v add column %v", tableName, getColumnLine(colInfo, sqlDriverName))

	return sq
}

func CreateTable(tableInfo *table_info.TableInfo, db database.DatabaseConnection) error {

	createTableQuery := MakeCreateTableQuery(tableInfo, db.DriverName())

	log.Debugf("Create table query: %v", tableInfo.TableName)
	if len(tableInfo.TableName) < 2 {
		log.Tracef("Table name less than two characters is unacceptable [%v]", tableInfo.TableName)
		return nil
	}
	log.Debugf("%v", createTableQuery)
	_, err := db.Exec(createTableQuery)
	//db.Exec("COMMIT ")
	if err != nil {
		log.Errorf("create table sql: %v", createTableQuery)
		log.Errorf("[718] Failed to create table [%v]: %v", tableInfo.TableName, err)
		return fmt.Errorf("failed to create table [%v]: %v", tableInfo.TableName, err)
	}
	return nil
}

func MakeCreateTableQuery(tableInfo *table_info.TableInfo, sqlDriverName string) string {
	createTableQuery := fmt.Sprintf("create table %s (\n", tableInfo.TableName)

	var columnStrings []string
	colsDone := map[string]bool{}
	for _, c := range tableInfo.Columns {

		if c.ColumnName == "" && c.Name == "" {
			log.Errorf("Column name is null: %v", c)
		}

		if c.ColumnName == "" {
			c.ColumnName = c.Name
		}

		if strings.TrimSpace(c.ColumnName) == "" {
			continue
		}

		if colsDone[c.ColumnName] {
			continue
		}

		columnLine := getColumnLine(&c, sqlDriverName)

		colsDone[c.ColumnName] = true
		columnStrings = append(columnStrings, columnLine)
	}

	columnString := strings.Join(columnStrings, ",\n  ")
	createTableQuery += columnString + ") "

	if sqlDriverName == "mysql" {
		createTableQuery += "CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	}

	return createTableQuery
}

func getColumnLine(c *api2go.ColumnInfo, sqlDriverName string) string {

	//log.Warnf("Get column line [%v] => [%v][%v]", c.ColumnName, c.ColumnType, c.DataType)

	datatype := c.DataType

	if datatype == "" {
		datatype = "varchar(100)"
	}

	// update column type if the db is postgres
	if sqlDriverName == "postgres" {
		if BeginsWith(datatype, "int(") {
			datatype = "INTEGER"
		} else if BeginsWith(datatype, "medium") {
			datatype = datatype[len("medium"):]
		} else if BeginsWith(datatype, "long") {
			datatype = datatype[len("long"):]
		} else if BeginsWith(datatype, "varbinary") {
			datatype = strings.Replace(datatype, "varbinary", "bit", 1)
		}
	}

	if BeginsWith(datatype, "blob") && sqlDriverName == "postgres" {
		datatype = "bytea"
	}

	columnParams := []string{c.ColumnName, datatype}

	if datatype == "timestamp" && c.DefaultValue == "" {
		c.IsNullable = true
	}

	if !c.IsNullable {
		columnParams = append(columnParams, "not null")
	} else {
		columnParams = append(columnParams, "null")
	}

	if c.IsAutoIncrement {
		if sqlDriverName == "sqlite3" {
			columnParams = append(columnParams, "PRIMARY KEY")
		} else if sqlDriverName == "mysql" {
			columnParams = append(columnParams, "AUTO_INCREMENT PRIMARY KEY")
		} else if sqlDriverName == "postgres" {
			columnParams = []string{c.ColumnName, "SERIAL", "PRIMARY KEY"}
		}
	} else if c.IsPrimaryKey {
		columnParams = append(columnParams, "PRIMARY KEY")
	}

	if c.DefaultValue != "" {
		columnParams = append(columnParams, "default "+c.DefaultValue)
	}

	//if sqlDriverName == "mysql" && (c.DataType == "text" || BeginsWith(c.DataType, "varchar(")) {
	//	columnParams = append(columnParams, "CHARACTER SET utf8mb4")
	//	columnParams = append(columnParams, "COLLATE utf8mb4_unicode_ci")
	//}

	columnLine := strings.Join(columnParams, " ")
	return columnLine
}
