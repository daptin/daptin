package resource

import (
	"fmt"
	"github.com/alexeyco/simpletable"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func InfoErr(err error, message string) {
	if err != nil {
		log.Infof("%v: %v", message, err)
	}

}
func CheckErr(err error, message ...interface{}) {
	if err != nil {
		args := message[1:]
		args = append(args, err)
		log.Errorf(message[0].(string), args)
	}
}

func CheckRelations(config *CmsConfig, db *sqlx.DB) {
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

		userRelation := api2go.NewTableRelation(table.TableName+"_state", "belongs_to", "user")
		userGroupRelation := api2go.NewTableRelation(table.TableName+"_state", "has_many", "usergroup")

		if len(existingRelations) > 0 {
			log.Infof("Found existing %d relations from db for [%v]", len(existingRelations), config.Tables[i].TableName)
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

					newTables = append(newTables, stateTable)

					stateTableHasOneDescription := api2go.NewTableRelation(stateTable.TableName, "has_one", "smd")
					stateTableHasOneDescription.SubjectName = table.TableName + "_status"
					stateTableHasOneDescription.ObjectName = table.TableName + "_smd"
					finalRelations = append(finalRelations, stateTableHasOneDescription)
					relationsDone[stateTableHasOneDescription.Hash()] = true
					relationsDone[stateRelation.Hash()] = true
					finalRelations = append(finalRelations, stateRelation)

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
			}

			if table.TableName == "usergroup" {
				continue
			}

			relation := api2go.NewTableRelation(table.TableName, "belongs_to", "user")
			finalRelations = append(finalRelations, relation)
			relationsDone[relation.Hash()] = true

			if table.TableName == "world_column" {
				continue
			}

			relationGroup := api2go.NewTableRelation(table.TableName, "has_many", "usergroup")
			relationsDone[relationGroup.Hash()] = true

			finalRelations = append(finalRelations, relationGroup)

		}

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

	// first fist column names for each column, if they were initially left blank.
	for i, c := range tableInfo.Columns {
		if c.ColumnName == "" {
			c.ColumnName = c.Name
			tableInfo.Columns[i].Name = c.Name
		}
		columnsWeWant[c.ColumnName] = false
		colInfoMap[c.ColumnName] = c
	}

	// append all the standard columns to this table
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

	finalColumns := make(map[string]api2go.ColumnInfo, 0)
	finalColumnsList := make([]api2go.ColumnInfo, 0)

	for i, c := range tableInfo.Columns {
		if c.ColumnName == "" {
			c.ColumnName = c.Name
			tableInfo.Columns[i].ColumnName = c.Name
		}
	}

	for _, col := range tableInfo.Columns {
		finalColumns[col.ColumnName] = col
	}

	for _, c := range finalColumns {
		finalColumnsList = append(finalColumnsList, c)
	}
	tableInfo.Columns = finalColumnsList

	columnsWeWant, colInfoMap := CreateAMapOfColumnsWeWantInTheFinalTable(tableInfo)
	log.Infof("Columns we want in [%v]", tableInfo.TableName)

	if tableInfo.TableName == "todo" {
		log.Infof("special break")
	}

	PrintTableInfo(tableInfo)
	//for col := range columnsWeWant {
	//	log.Infof("Column: [%v]%v @ %v - %v", tableInfo.TableName, col, colInfoMap[col].ColumnType, colInfoMap[col].DataType)
	//}

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
func PrintTableInfo(info *TableInfo) {

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
	table.Println()

}
