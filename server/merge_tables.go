package server

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
	"github.com/sirupsen/logrus"
)

func MergeTables(existingTables []table_info.TableInfo, initConfigTables []table_info.TableInfo) []table_info.TableInfo {
	allTables := make([]table_info.TableInfo, 0)
	existingTablesMap := make(map[string]bool)
	mergedInitConfigTables := make([]table_info.TableInfo, 0, len(initConfigTables))
	initConfigTablesMap := make(map[string]int)
	for _, table := range initConfigTables {
		existingIndex, exists := initConfigTablesMap[table.TableName]
		if exists {
			mergedInitConfigTables[existingIndex] = mergeTableConfigIntoExisting(mergedInitConfigTables[existingIndex], table, true)
			continue
		}
		initConfigTablesMap[table.TableName] = len(mergedInitConfigTables)
		mergedInitConfigTables = append(mergedInitConfigTables, table)
	}
	initConfigTables = mergedInitConfigTables

	for j, existableTable := range existingTables {
		existingTablesMap[existableTable.TableName] = true
		var isBeingModified = false

		for _, tableBeingModified := range initConfigTables {
			if tableBeingModified.TableName != existableTable.TableName {
				continue
			}

			logrus.Infof("Table from initial configuration:          %-20s", existableTable.TableName)
			existableTable = mergeTableConfigIntoExisting(existableTable, tableBeingModified, isBeingModified)
			isBeingModified = true
		}

		if isBeingModified {
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

func mergeTableConfigIntoExisting(existing table_info.TableInfo, override table_info.TableInfo, partialOverride bool) table_info.TableInfo {
	if len(override.Columns) > 0 {
		for _, overrideColumn := range override.Columns {
			columnAlreadyExists := false
			colIndex := -1
			for i, existingColumn := range existing.Columns {
				if existingColumn.ColumnName == overrideColumn.ColumnName {
					columnAlreadyExists = true
					colIndex = i
					break
				}
			}
			if columnAlreadyExists {
				mergeConfigColumn(&existing.Columns[colIndex], overrideColumn, partialOverride)
			} else {
				existing.Columns = append(existing.Columns, overrideColumn)
			}
		}
	}

	if len(override.Relations) > 0 {
		relMap := make(map[string]bool)
		for _, rel := range existing.Relations {
			relMap[rel.Hash()] = true
		}
		for _, rel := range override.Relations {
			if !relMap[rel.Hash()] {
				existing.AddRelation(rel)
			}
		}
	}

	if override.DefaultGroups != nil {
		existing.DefaultGroups = override.DefaultGroups
	}
	if override.AccessGroups != nil {
		existing.AccessGroups = override.AccessGroups
	}
	if override.DefaultRelations != nil {
		existing.DefaultRelations = override.DefaultRelations
	}
	if override.DefaultPermission != 0 {
		existing.DefaultPermission = override.DefaultPermission
	}
	if override.Permission != 0 {
		existing.Permission = override.Permission
	}
	if override.StateMachines != nil {
		existing.StateMachines = override.StateMachines
	}
	if !partialOverride || override.IsStateTrackingEnabled || override.ExplicitFields["IsStateTrackingEnabled"] || override.ExplicitFields["is_state_tracking_enabled"] {
		existing.IsStateTrackingEnabled = override.IsStateTrackingEnabled
	}
	if !partialOverride || override.TranslationsEnabled || override.ExplicitFields["TranslationsEnabled"] || override.ExplicitFields["translations_enabled"] {
		existing.TranslationsEnabled = override.TranslationsEnabled
	}
	if !partialOverride || override.DefaultOrder != "" || override.ExplicitFields["DefaultOrder"] || override.ExplicitFields["default_order"] {
		existing.DefaultOrder = override.DefaultOrder
	}
	if !partialOverride || override.IsAuditEnabled || override.ExplicitFields["IsAuditEnabled"] || override.ExplicitFields["is_audit_enabled"] {
		existing.IsAuditEnabled = override.IsAuditEnabled
	}
	if !partialOverride || override.IsHidden || override.ExplicitFields["IsHidden"] || override.ExplicitFields["is_hidden"] {
		existing.IsHidden = override.IsHidden
	}
	if !partialOverride || override.IsTopLevel || override.ExplicitFields["IsTopLevel"] || override.ExplicitFields["is_top_level"] {
		existing.IsTopLevel = override.IsTopLevel
	}
	if !partialOverride || override.IsJoinTable || override.ExplicitFields["IsJoinTable"] || override.ExplicitFields["is_join_table"] {
		existing.IsJoinTable = override.IsJoinTable
	}
	if override.Conformations != nil {
		existing.Conformations = override.Conformations
	}
	if override.Validations != nil {
		existing.Validations = override.Validations
	}
	if override.CompositeKeys != nil {
		existing.CompositeKeys = override.CompositeKeys
	}
	if !partialOverride || override.Icon != "" || override.ExplicitFields["Icon"] || override.ExplicitFields["icon"] {
		existing.Icon = override.Icon
	}
	if !partialOverride || override.TableDescription != "" || override.ExplicitFields["TableDescription"] || override.ExplicitFields["table_description"] {
		existing.TableDescription = override.TableDescription
	}
	if override.Metering != nil {
		existing.Metering = override.Metering
	}

	return existing
}

func mergeConfigColumn(base *api2go.ColumnInfo, override api2go.ColumnInfo, partialOverride bool) {
	if partialOverride {
		mergePartialConfigColumn(base, override)
		return
	}

	base.DefaultValue = override.DefaultValue
	base.ExcludeFromApi = override.ExcludeFromApi
	base.IsIndexed = override.IsIndexed
	base.IsNullable = override.IsNullable
	base.IsUnique = override.IsUnique
	base.ColumnType = override.ColumnType
	base.Options = override.Options
	base.DataType = override.DataType
	base.ColumnDescription = override.ColumnDescription

	preserveCloudStoreColumn := base.IsForeignKey &&
		base.ForeignKeyData.DataSource == "cloud_store" &&
		!override.IsForeignKey &&
		override.ForeignKeyData.KeyName == ""
	if override.ForeignKeyData.KeyName != "" {
		base.ForeignKeyData = override.ForeignKeyData
	}
	if !preserveCloudStoreColumn {
		base.IsForeignKey = override.IsForeignKey
	}
	base.IsPrimaryKey = override.IsPrimaryKey
}

func mergePartialConfigColumn(base *api2go.ColumnInfo, override api2go.ColumnInfo) {
	if override.DefaultValue != "" {
		base.DefaultValue = override.DefaultValue
	}
	if override.ExcludeFromApi {
		base.ExcludeFromApi = override.ExcludeFromApi
	}
	if override.IsIndexed {
		base.IsIndexed = override.IsIndexed
	}
	if override.IsNullable {
		base.IsNullable = override.IsNullable
	}
	if override.IsUnique {
		base.IsUnique = override.IsUnique
	}
	if override.ColumnType != "" {
		base.ColumnType = override.ColumnType
	}
	if override.Options != nil {
		base.Options = override.Options
	}
	if override.DataType != "" {
		base.DataType = override.DataType
	}
	if override.ColumnDescription != "" {
		base.ColumnDescription = override.ColumnDescription
	}
	if override.ForeignKeyData.DataSource != "" {
		base.ForeignKeyData.DataSource = override.ForeignKeyData.DataSource
	}
	if override.ForeignKeyData.Namespace != "" {
		base.ForeignKeyData.Namespace = override.ForeignKeyData.Namespace
	}
	if override.ForeignKeyData.KeyName != "" {
		base.ForeignKeyData.KeyName = override.ForeignKeyData.KeyName
	}
	if override.IsForeignKey || hasForeignKeyData(override.ForeignKeyData) {
		base.IsForeignKey = true
	}
	if override.IsPrimaryKey {
		base.IsPrimaryKey = override.IsPrimaryKey
	}
}

func hasForeignKeyData(data api2go.ForeignKeyData) bool {
	return data.DataSource != "" || data.Namespace != "" || data.KeyName != ""
}
