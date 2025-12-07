package server

import (
	"github.com/daptin/daptin/server/table_info"
	"github.com/sirupsen/logrus"
)

func MergeTables(existingTables []table_info.TableInfo, initConfigTables []table_info.TableInfo) []table_info.TableInfo {
	allTables := make([]table_info.TableInfo, 0)
	existingTablesMap := make(map[string]bool)

	newTableMap := make(map[string]table_info.TableInfo)
	for _, newTable := range initConfigTables {
		newTableMap[newTable.TableName] = newTable
	}

	for j, existableTable := range existingTables {
		existingTablesMap[existableTable.TableName] = true
		var isBeingModified = false
		var indexBeingModified = -1

		for i, newTable := range initConfigTables {
			if newTable.TableName == existableTable.TableName {
				isBeingModified = true
				indexBeingModified = i
				break
			}
		}

		if isBeingModified {
			logrus.Infof("Table from initial configuration:          %-20s", existableTable.TableName)
			tableBeingModified := initConfigTables[indexBeingModified]

			if len(tableBeingModified.Columns) > 0 {

				for _, newColumnDef := range tableBeingModified.Columns {
					columnAlreadyExist := false
					colIndex := -1
					for i, existingColumn := range existableTable.Columns {
						//log.Printf("Table column old/new [%v][%v] == [%v][%v] @ %v", tableBeingModified.TableName, newColumnDef.Name, existableTable.TableName, existingColumn.Name, i)
						if existingColumn.ColumnName == newColumnDef.ColumnName {
							columnAlreadyExist = true
							colIndex = i
							break
						}
					}
					//log.Printf("Decide for table column [%v][%v] @ index: %v [%v]", tableBeingModified.TableName, newColumnDef.Name, colIndex, columnAlreadyExist)
					if columnAlreadyExist {
						//log.Printf("Modifying existing columns[%v][%v] is not supported at present. not sure what would break. and alter query isnt being run currently.", existableTable.Columns[colIndex], newColumnDef);

						existableTable.Columns[colIndex].DefaultValue = newColumnDef.DefaultValue
						existableTable.Columns[colIndex].ExcludeFromApi = newColumnDef.ExcludeFromApi
						existableTable.Columns[colIndex].IsIndexed = newColumnDef.IsIndexed
						existableTable.Columns[colIndex].IsNullable = newColumnDef.IsNullable
						existableTable.Columns[colIndex].IsUnique = newColumnDef.IsUnique
						existableTable.Columns[colIndex].ColumnType = newColumnDef.ColumnType
						existableTable.Columns[colIndex].Options = newColumnDef.Options
						existableTable.Columns[colIndex].DataType = newColumnDef.DataType
						existableTable.Columns[colIndex].ColumnDescription = newColumnDef.ColumnDescription
						if newColumnDef.ForeignKeyData.KeyName != "" {
							existableTable.Columns[colIndex].ForeignKeyData = newColumnDef.ForeignKeyData
						}
						existableTable.Columns[colIndex].IsForeignKey = newColumnDef.IsForeignKey
						existableTable.Columns[colIndex].IsPrimaryKey = newColumnDef.IsPrimaryKey

					} else {
						existableTable.Columns = append(existableTable.Columns, newColumnDef)
					}
				}

			}
			if len(tableBeingModified.Relations) > 0 {

				existingRelations := existableTable.Relations
				relMap := make(map[string]bool)
				for _, rel := range existingRelations {
					relMap[rel.Hash()] = true
				}

				for _, newRel := range tableBeingModified.Relations {

					_, ok := relMap[newRel.Hash()]
					if !ok {
						existableTable.AddRelation(newRel)
					}
				}
			}
			existableTable.DefaultGroups = tableBeingModified.DefaultGroups
			existableTable.DefaultRelations = tableBeingModified.DefaultRelations
			existableTable.StateMachines = tableBeingModified.StateMachines
			existableTable.IsStateTrackingEnabled = tableBeingModified.IsStateTrackingEnabled
			existableTable.TranslationsEnabled = tableBeingModified.TranslationsEnabled
			existableTable.DefaultOrder = tableBeingModified.DefaultOrder
			existableTable.IsAuditEnabled = tableBeingModified.IsAuditEnabled
			existableTable.Conformations = tableBeingModified.Conformations
			existableTable.Validations = tableBeingModified.Validations
			existableTable.CompositeKeys = tableBeingModified.CompositeKeys
			existableTable.Icon = tableBeingModified.Icon
			existingTables[j] = existableTable
		} else {
			logrus.Tracef("Table %s is not being modified", existableTable.TableName)
		}
		allTables = append(allTables, existableTable)
	}

	for _, newTable := range initConfigTables {
		if existingTablesMap[newTable.TableName] {
			continue
		}
		allTables = append(allTables, newTable)
	}

	return allTables

}
