package resource

import (
	"fmt"
	"github.com/alexeyco/simpletable"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/table_info"
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
		log.Printf(fmtString+": %v", args...)
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
		log.Printf(fmtString+": %v", args...)
		return true
	}
	return false
}

func relationHash(rel api2go.TableRelation) string {
	relation := rel.GetRelation()
	if relation == "has_one" {
		relation = "belongs_to"
	} else if relation == "has_many_and_belongs_to_many" {
		relation = "has_many"
	}
	return fmt.Sprintf("%s-%s-%s", rel.GetObjectName(), relation, rel.GetSubjectName())
}

func CheckRelations(config *CmsConfig) {
	newRelationsFromConfig := config.Relations
	config.Relations = make([]api2go.TableRelation, 0)
	finalRelations := make([]api2go.TableRelation, 0)
	relationsDone := make(map[string]bool)

	for _, newRelationFromConfig := range newRelationsFromConfig {

		_, ok := relationsDone[relationHash(newRelationFromConfig)]
		if ok {
			continue
		} else {
			relationsDone[relationHash(newRelationFromConfig)] = true
			finalRelations = append(finalRelations, newRelationFromConfig)
		}
	}

	newTables := make([]table_info.TableInfo, 0)

	for i := range config.Tables {

		config.Tables[i].IsTopLevel = true
		existingRelations := config.Tables[i].Relations

		if config.Tables[i].TableName != "usergroup" &&
			!config.Tables[i].IsJoinTable &&
			!EndsWithCheck(config.Tables[i].TableName, "_audit") {
			relation := api2go.NewTableRelation(config.Tables[i].TableName, "belongs_to", USER_ACCOUNT_TABLE_NAME)
			relationGroup := api2go.NewTableRelation(config.Tables[i].TableName, "has_many", "usergroup")

			if !relationsDone[relationHash(relation)] {
				relationsDone[relationHash(relation)] = true
				config.Tables[i].AddRelation(relation)
				finalRelations = append(finalRelations, relation)
			}

			if !relationsDone[relationHash(relationGroup)] {
				relationsDone[relationHash(relationGroup)] = true
				config.Tables[i].AddRelation(relationGroup)
				finalRelations = append(finalRelations, relationGroup)
			}

		}

		userRelation := api2go.NewTableRelation(config.Tables[i].TableName+"_state", "belongs_to", USER_ACCOUNT_TABLE_NAME)
		userGroupRelation := api2go.NewTableRelation(config.Tables[i].TableName+"_state", "has_many", "usergroup")

		if len(existingRelations) > 0 {
			//log.Printf("Found existing %d relations from db for [%v]", len(existingRelations), config.Tables[i].TableName)
			for _, rel := range existingRelations {

				relhash := relationHash(rel)
				_, ok := relationsDone[relhash]
				if ok {
					continue
				} else {
					finalRelations = append(finalRelations, rel)

					relationsDone[relhash] = true
				}
			}

			if config.Tables[i].IsStateTrackingEnabled {

				stateRelation := api2go.TableRelation{
					Subject:     config.Tables[i].TableName + "_state",
					SubjectName: config.Tables[i].TableName + "_has_state",
					Object:      config.Tables[i].TableName,
					ObjectName:  "is_state_of_" + config.Tables[i].TableName,
					Relation:    "belongs_to",
				}

				if !relationsDone[relationHash(userRelation)] {
					relationsDone[relationHash(userRelation)] = true
					finalRelations = append(finalRelations, userRelation)
				}

				if !relationsDone[relationHash(userGroupRelation)] {
					relationsDone[relationHash(userGroupRelation)] = true
					finalRelations = append(finalRelations, userGroupRelation)
				}

				if !relationsDone[relationHash(stateRelation)] {

					stateTable := table_info.TableInfo{
						TableName: config.Tables[i].TableName + "_state",
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
					stateTableHasOneDescription.SubjectName = config.Tables[i].TableName + "_status"
					stateTableHasOneDescription.ObjectName = config.Tables[i].TableName + "_smd"
					finalRelations = append(finalRelations, stateTableHasOneDescription)
					relationsDone[relationHash(stateTableHasOneDescription)] = true
					relationsDone[relationHash(stateRelation)] = true
					finalRelations = append(finalRelations, stateRelation)

					stateTable.Relations = []api2go.TableRelation{stateRelation, stateTableHasOneDescription, userRelation, userGroupRelation}
					newTables = append(newTables, stateTable)

				}
			}

		} else {

			if config.Tables[i].IsStateTrackingEnabled {
				stateTable := table_info.TableInfo{
					TableName: config.Tables[i].TableName + "_state",
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
				stateTableHasOneDescription.SubjectName = config.Tables[i].TableName + "_status"
				stateTableHasOneDescription.ObjectName = config.Tables[i].TableName + "_smd"
				finalRelations = append(finalRelations, stateTableHasOneDescription)
				relationsDone[relationHash(stateTableHasOneDescription)] = true

				stateRelation := api2go.TableRelation{
					Subject:     stateTable.TableName,
					SubjectName: config.Tables[i].TableName + "_has_state",
					Object:      config.Tables[i].TableName,
					ObjectName:  "is_state_of_" + config.Tables[i].TableName,
					Relation:    "belongs_to",
				}
				relationsDone[relationHash(stateRelation)] = true
				relationsDone[relationHash(userRelation)] = true
				relationsDone[relationHash(userGroupRelation)] = true
				finalRelations = append(finalRelations, stateRelation)
				finalRelations = append(finalRelations, userRelation)
				finalRelations = append(finalRelations, userGroupRelation)

				stateTable.Relations = []api2go.TableRelation{stateRelation, userRelation, userGroupRelation, stateTableHasOneDescription}
				newTables = append(newTables, stateTable)
			}

		}
	}

	for i, tab := range config.Tables {
		for _, rel := range finalRelations {
			if rel.GetSubject() == tab.TableName || rel.GetObject() == tab.TableName {
				config.Tables[i].AddRelation(rel)
			}
		}
	}

	log.Printf("%d state tables on base entities", len(newTables))
	config.Tables = append(config.Tables, newTables...)

	//newRelations := make([]api2go.TableRelation, 0)
	convertRelationsToColumns(finalRelations, config)
	convertRelationsToColumns(StandardRelations, config)

	//updatedTables := make([]TableInfo, 0)
	//for _, table := range config.Tables {
	//	table.Relations = filterRelations(table.TableName, finalRelations, StandardRelations)
	//	updatedTables = append(updatedTables, table)
	//	log.Debugf("Table [%s] has relations %v", table.TableName, table.Relations)
	//}
	//config.Tables = updatedTables

	//config.Tables[stateMachineDescriptionTableIndex] = stateMachineDescriptionTable

	//for _, relation := range finalRelations {
	//	log.Printf("All relations: %v", relation.String())
	//}
	if log.GetLevel() == log.DebugLevel {
		PrintRelations(finalRelations)
	}
}

func filterRelations(name string, relations []api2go.TableRelation, relations2 []api2go.TableRelation) []api2go.TableRelation {

	relationList := make([]api2go.TableRelation, 0)

	for _, relation := range relations {
		if relation.Subject == name || relation.ObjectName == name {
			relationList = append(relationList, relation)
		}
	}

	for _, relation := range relations2 {
		if relation.Subject == name || relation.ObjectName == name {
			relationList = append(relationList, relation)
		}
	}

	return relationList

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
			{
				Text: "Subject Name",
			},
			{
				Text: "Object Name",
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
		}, &simpletable.Cell{
			Text: relation.SubjectName,
		}, &simpletable.Cell{
			Text: relation.ObjectName,
		},
		)

		body.Cells = append(body.Cells, row)
	}

	table.Body = &body
	table.Println()

}

func CheckAllTableStatus(initConfig *CmsConfig, db database.DatabaseConnection) {

	var tables []table_info.TableInfo
	tableCreatedMap := map[string]bool{}

	for _, table := range initConfig.Tables {
		if len(table.TableName) < 2 {
			continue
		}

		if !tableCreatedMap[table.TableName] {
			log.Tracef("Check table %v", table.TableName)
			err := CheckTable(&table, db)
			if err != nil {
				CheckErr(err, "Failed to check and create table: [%v]", table.TableName)
			} else {
				tables = append(tables, table)
				CheckErr(err, "Failed to commit create table txn after failure")
			}
		}
	}
	initConfig.Tables = tables
	return
}

func CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo *table_info.TableInfo) (map[string]bool, map[string]api2go.ColumnInfo) {
	columnsWeWant := map[string]bool{}
	colInfoMap := map[string]api2go.ColumnInfo{}
	finalColumnList := make([]api2go.ColumnInfo, 0)

	// append all the standard columns to this table
	for _, sCol := range StandardColumns {
		_, ok := colInfoMap[sCol.ColumnName]
		if ok {
			//log.Printf("Column [%v] already present in config for table [%v]", sCol.ColumnName, tableInfo.TableName)
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

func CheckTable(tableInfo *table_info.TableInfo, db database.DatabaseConnection) error {

	for i, c := range tableInfo.Columns {
		if c.ColumnType == "truefalse" {
			c.DataType = "bool"
		}
		if c.ColumnName == "" && c.Name != "" {
			tableInfo.Columns[i].ColumnName = SmallSnakeCaseText(c.Name)
		} else if c.ColumnName != "" && c.Name == "" {
			tableInfo.Columns[i].Name = c.ColumnName
		}
	}

	columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)

	s := fmt.Sprintf("select * from %s limit 1", tableInfo.TableName)
	log.Debugf("Sql: %v", s)
	stmt1, err := db.Preparex(s)
	log.Debugf("Prepared Sql: %v", s)
	var columns []string
	if err != nil {
		// expected error, no need to log
		log.Tracef("Failed to select * from %v: %v", tableInfo.TableName, err)
		err = CreateTable(tableInfo, db)
		return err
	} else {
		defer stmt1.Close()
		rowx := stmt1.QueryRowx()
		columns, err = rowx.Columns()

		// this is required
		// dont remove this
		// else p
		dest := make(map[string]interface{})
		err = rowx.MapScan(dest)
		//CheckErr(err, "Failed to scan query result to map")
	}

	for _, col := range columns {
		_, ok := columnsWeWant[col]
		if !ok {
			log.Printf("extra column [%v] found in table [%v]", col, tableInfo.TableName)
		} else {
			//log.Printf("Column [%v] already present in table [%v]", col, tableInfo.TableName)
			columnsWeWant[col] = true
		}
	}

	for col, present := range columnsWeWant {

		if !present {
			log.Printf("Column [%v] is not present in table [%v]", col, tableInfo.TableName)
			info := colInfoMap[col]

			if info.DataType == "" {
				log.Printf("No column type known for column: %v", info)
				info.DataType = "varchar(50)"
				//continue
			}

			query := alterTableAddColumn(tableInfo.TableName, &info, db.DriverName())
			log.Printf("Alter query: %v", query)
			_, err := db.Exec(query)
			if err != nil {
				log.Errorf("Failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
				return fmt.Errorf("failed to add column [%s] to table [%v]: %v", col, tableInfo.TableName, err)
			}
		}
	}
	return nil
}

func PrintTableInfo(info *table_info.TableInfo, title string) {

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
