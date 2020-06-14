package resource

import (
	"fmt"
	"github.com/alexeyco/simpletable"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/database"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func InfoErr(err error, message ...interface{}) bool {
	if err != nil {
		fmtString := message[0].(string)
		args := make([]interface{}, 0)
		if len(message) > 1 {
			args = message[1:]
		}
		args = append(args, err)
		log.Infof(fmtString+": %v", args...)
		return true
	}
	return false

}

func CheckErr(err error, message ...interface{}) bool {

	if err != nil {
		fmtString := message[0].(string)
		args := make([]interface{}, 0)
		if len(message) > 1 {
			args = message[1:]
		}
		args = append(args, err)
		log.Errorf(fmtString+": %v", args...)
		return true
	}
	return false
}

func CheckInfo(err error, message ...interface{}) bool {
	if err != nil {
		fmtString := message[0].(string)
		args := make([]interface{}, 0)
		if len(message) > 1 {
			args = message[1:]
		}
		args = append(args, err)
		log.Infof(fmtString+": %v", args...)
		return true
	}
	return false
}

func CheckRelations(config *CmsConfig) {
	relations := config.Relations
	config.Relations = make([]api2go.TableRelation, 0)
	finalRelations := make([]api2go.TableRelation, 0)
	relationsDone := make(map[string]bool)

	for _, relation := range relations {

		_, ok := relationsDone[relation.Hash()]
		if ok {
			continue
		} else {
			relationsDone[relation.Hash()] = true
			finalRelations = append(finalRelations, relation)
		}
	}

	newTables := make([]TableInfo, 0)

	for i, table := range config.Tables {

		config.Tables[i].IsTopLevel = true
		existingRelations := config.Tables[i].Relations

		if table.TableName != "usergroup" &&
			!table.IsJoinTable &&
			!EndsWithCheck(table.TableName, "_audit") {
			relation := api2go.NewTableRelation(table.TableName, "belongs_to", USER_ACCOUNT_TABLE_NAME)
			relationGroup := api2go.NewTableRelation(table.TableName, "has_many", "usergroup")

			if !relationsDone[relation.Hash()] {
				relationsDone[relation.Hash()] = true
				config.Tables[i].Relations = append(config.Tables[i].Relations, relation)
				finalRelations = append(finalRelations, relation)
			}

			if !relationsDone[relationGroup.Hash()] {
				relationsDone[relationGroup.Hash()] = true
				config.Tables[i].Relations = append(config.Tables[i].Relations, relationGroup)
				finalRelations = append(finalRelations, relationGroup)
			}

		}

		userRelation := api2go.NewTableRelation(table.TableName+"_state", "belongs_to", USER_ACCOUNT_TABLE_NAME)
		userGroupRelation := api2go.NewTableRelation(table.TableName+"_state", "has_many", "usergroup")

		if len(existingRelations) > 0 {
			//log.Infof("Found existing %d relations from db for [%v]", len(existingRelations), config.Tables[i].TableName)
			for _, rel := range existingRelations {

				relhash := rel.Hash()
				_, ok := relationsDone[relhash]
				if ok {
					continue
				} else {
					finalRelations = append(finalRelations, rel)

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

				if !relationsDone[userRelation.Hash()] {
					relationsDone[userRelation.Hash()] = true
					finalRelations = append(finalRelations, userRelation)
				}

				if !relationsDone[userGroupRelation.Hash()] {
					relationsDone[userGroupRelation.Hash()] = true
					finalRelations = append(finalRelations, userGroupRelation)
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

					stateTableHasOneDescription := api2go.NewTableRelation(stateTable.TableName, "has_one", "smd")
					stateTableHasOneDescription.SubjectName = table.TableName + "_status"
					stateTableHasOneDescription.ObjectName = table.TableName + "_smd"
					finalRelations = append(finalRelations, stateTableHasOneDescription)
					relationsDone[stateTableHasOneDescription.Hash()] = true
					relationsDone[stateRelation.Hash()] = true
					finalRelations = append(finalRelations, stateRelation)

					stateTable.Relations = []api2go.TableRelation{stateRelation, stateTableHasOneDescription, userRelation, userGroupRelation}
					newTables = append(newTables, stateTable)

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

				stateTableHasOneDescription := api2go.NewTableRelation(stateTable.TableName, "has_one", "smd")
				stateTableHasOneDescription.SubjectName = table.TableName + "_status"
				stateTableHasOneDescription.ObjectName = table.TableName + "_smd"
				finalRelations = append(finalRelations, stateTableHasOneDescription)
				relationsDone[stateTableHasOneDescription.Hash()] = true

				stateRelation := api2go.TableRelation{
					Subject:     stateTable.TableName,
					SubjectName: table.TableName + "_has_state",
					Object:      table.TableName,
					ObjectName:  "is_state_of_" + table.TableName,
					Relation:    "belongs_to",
				}
				relationsDone[stateRelation.Hash()] = true
				relationsDone[userRelation.Hash()] = true
				relationsDone[userGroupRelation.Hash()] = true
				finalRelations = append(finalRelations, stateRelation)
				finalRelations = append(finalRelations, userRelation)
				finalRelations = append(finalRelations, userGroupRelation)

				stateTable.Relations = []api2go.TableRelation{stateRelation, userRelation, userGroupRelation, stateTableHasOneDescription}
				newTables = append(newTables, stateTable)
			}

		}
		config.Tables[i] = table
	}

	log.Infof("%d state tables on base entities", len(newTables))
	config.Tables = append(config.Tables, newTables...)

	//newRelations := make([]api2go.TableRelation, 0)
	convertRelationsToColumns(finalRelations, config)
	convertRelationsToColumns(StandardRelations, config)

	//config.Tables[stateMachineDescriptionTableIndex] = stateMachineDescriptionTable

	//for _, relation := range finalRelations {
	//	log.Infof("All relations: %v", relation.String())
	//}
	PrintRelations(finalRelations)
}

func PrintRelations(relations []api2go.TableRelation) {
	table := simpletable.New()

	header := simpletable.Header{
		Cells: []*simpletable.Cell{
			{
				Text: "Subject",
			},
			{
				Text: "Relation",
			},
			{
				Text: "Object",
			},
		},
	}
	table.Header = &header

	body := simpletable.Body{
		Cells: make([][]*simpletable.Cell, 0),
	}

	for _, relation := range relations {
		row := make([]*simpletable.Cell, 0)

		row = append(row, &simpletable.Cell{
			Text: relation.Subject,
		}, &simpletable.Cell{
			Text: relation.Relation,
		}, &simpletable.Cell{
			Text: relation.Object,
		})

		body.Cells = append(body.Cells, row)
	}

	table.Body = &body
	table.Println()

}

func CheckAllTableStatus(initConfig *CmsConfig, db database.DatabaseConnection) {

	var tables []TableInfo
	tableCreatedMap := map[string]bool{}

	for _, table := range initConfig.Tables {
		if len(table.TableName) < 2 {
			continue
		}

		if !tableCreatedMap[table.TableName] {
			//if strings.Index(table.TableName, "_has_") == -1 {
			log.Infof("Check table %v", table.TableName)
			//continue
			//}
			tx, err := db.Beginx()
			if err != nil {
				CheckErr(err, "Failed to start txn for create table", table.TableName)
				continue
			}
			err = CheckTable(&table, db, tx)
			if err != nil {
				err = tx.Rollback()
				CheckErr(err, "Failed to rollback create table txn after failure")
				tx, err = db.Beginx()
				CheckErr(err, "Failed to create new transaction create table txn after failure")
			} else {
				tables = append(tables, table)
				err = tx.Commit()
				CheckErr(err, "Failed to commit create table txn after failure")
				tableCreatedMap[table.TableName] = true
			}
		}
	}
	initConfig.Tables = tables
	return
}

func CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo *TableInfo) (map[string]bool, map[string]api2go.ColumnInfo) {
	columnsWeWant := map[string]bool{}
	colInfoMap := map[string]api2go.ColumnInfo{}
	finalColumnList := make([]api2go.ColumnInfo, 0)

	// append all the standard columns to this table
	for _, sCol := range StandardColumns {
		_, ok := colInfoMap[sCol.ColumnName]
		if ok {
			//log.Infof("Column [%v] already present in config for table [%v]", sCol.ColumnName, tableInfo.TableName)
		} else {
			colInfoMap[sCol.Name] = sCol
			columnsWeWant[sCol.Name] = false
			finalColumnList = append(finalColumnList, sCol)
		}
	}

	// first fist column names for each column, if they were initially left blank.
	for _, c := range tableInfo.Columns {
		_, ok := colInfoMap[c.ColumnName]
		if ok {

		} else {
			columnsWeWant[c.ColumnName] = false
			colInfoMap[c.ColumnName] = c
			finalColumnList = append(finalColumnList, c)
		}
	}

	tableInfo.Columns = finalColumnList

	return columnsWeWant, colInfoMap
}

func CheckTable(tableInfo *TableInfo, db database.DatabaseConnection, tx *sqlx.Tx) error {

	for i, c := range tableInfo.Columns {
		if c.ColumnName == "" && c.Name != "" {
			tableInfo.Columns[i].ColumnName = SmallSnakeCaseText(c.Name)
		} else if c.ColumnName != "" && c.Name == "" {
			tableInfo.Columns[i].Name = c.ColumnName
		}
	}

	columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)

	s := fmt.Sprintf("select * from %s limit 1", tableInfo.TableName)
	//log.Infof("Sql: %v", s)
	rowx := db.QueryRowx(s)
	columns, err := rowx.Columns()
	if err != nil {
		log.Infof("Failed to select * from %v: %v", tableInfo.TableName, err)
		err = CreateTable(tableInfo, tx)
		return err
	} else {
		dest := make(map[string]interface{})
		err = rowx.MapScan(dest)
		CheckErr(err, "Failed to scan query result to map")
	}

	for _, col := range columns {
		_, ok := columnsWeWant[col]
		if !ok {
			log.Infof("extra column [%v] found in table [%v]", col, tableInfo.TableName)
		} else {
			log.Infof("Column [%v] already present in table [%v]", col, tableInfo.TableName)
			columnsWeWant[col] = true
		}
	}

	for col, present := range columnsWeWant {

		if !present {
			log.Infof("Column [%v] is not present in table [%v]", col, tableInfo.TableName)
			info := colInfoMap[col]

			if info.DataType == "" {
				log.Infof("No column type known for column: %v", info)
				info.DataType = "varchar(50)"
				//continue
			}

			query := alterTableAddColumn(tableInfo.TableName, &info, tx.DriverName())
			log.Infof("Alter query: %v", query)
			_, err := tx.Exec(query)
			if err != nil {
				log.Errorf("Failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
				return fmt.Errorf("failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
			}
		}
	}
	return nil
}

func PrintTableInfo(info *TableInfo, title string) {

	table := simpletable.New()

	table.Header = &simpletable.Header{
		Cells: []*simpletable.Cell{
			{
				Text: "Column name",
			},
			{
				Text: "Column type",
			},
			{
				Text: "Data type",
			},
		},
	}
	tableBody := simpletable.Body{
		Cells: make([][]*simpletable.Cell, 0),
	}

	for _, col := range info.Columns {
		tableRow := make([]*simpletable.Cell, 0)

		tableRow = append(tableRow, &simpletable.Cell{
			Text: col.ColumnName,
		}, &simpletable.Cell{
			Text: col.ColumnType,
		}, &simpletable.Cell{
			Text: col.DataType,
		})
		tableBody.Cells = append(tableBody.Cells, tableRow)
	}

	table.Body = &tableBody
	log.Println(title)
	log.Println(table.String())

}
