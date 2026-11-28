package server

import (
	jsonstd "encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
)

// schemaInstaller persists definitions through the existing schema synchronizers.
// Physical schema changes and runtime construction remain startup responsibilities.
func schemaInstaller(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore) func(string, []byte, *sqlx.Tx) error {
	return func(name string, body []byte, tx *sqlx.Tx) error {
		config, err := parseSchemaDocument(name, body)
		if err != nil {
			return err
		}
		// Imports execute once on the receiving node. New entities become importable
		// after restart, through the normal data import action.
		for _, item := range config.Imports {
			if cruds[item.Entity] == nil {
				return fmt.Errorf("import entity %s is unavailable; restart after installing its schema, then use the data import action", item.Entity)
			}
		}
		worlds, err := resource.GetWorldTableMapBy("table_name", tx)
		if err != nil {
			return err
		}
		affected := make(map[string]bool)
		for _, table := range config.Tables {
			affected[table.TableName] = true
		}
		for _, relation := range config.Relations {
			affected[relation.Subject] = true
			affected[relation.Object] = true
		}
		names := make([]string, 0, len(affected))
		for name := range affected {
			names = append(names, name)
		}
		sort.Strings(names)
		existing := make([]table_info.TableInfo, 0, len(names))
		for _, name := range names {
			row := worlds[name]
			if row == nil {
				continue
			}
			var table table_info.TableInfo
			if err := jsonstd.Unmarshal([]byte(resource.StringOrEmpty(row["world_schema_json"])), &table); err != nil {
				return err
			}
			permission, err := resource.ResourceRowInt64(row["permission"])
			if err != nil {
				return err
			}
			table.Permission = auth.AuthPermission(permission)
			table.TableName = resource.StringOrEmpty(row["table_name"])
			table.Icon = resource.StringOrEmpty(row["icon"])
			table.DefaultOrder = resource.StringOrEmpty(row["default_order"])
			for column, target := range map[string]*bool{"is_top_level": &table.IsTopLevel, "is_hidden": &table.IsHidden, "is_join_table": &table.IsJoinTable, "is_state_tracking_enabled": &table.IsStateTrackingEnabled} {
				value, err := strconv.ParseBool(fmt.Sprint(row[column]))
				if err != nil {
					return fmt.Errorf("read world.%s for %s: %w", column, name, err)
				}
				*target = value
			}
			existing = append(existing, table)
		}
		for _, base := range existing {
			matched := false
			for i := range config.Tables {
				if config.Tables[i].TableName == base.TableName {
					config.Tables[i] = mergeTableConfigIntoExisting(base, config.Tables[i], true)
					matched = true
					break
				}
			}
			if !matched {
				config.Tables = append(config.Tables, base)
			}
		}
		for i := range config.Tables {
			for _, relation := range config.Relations {
				if relation.Subject == config.Tables[i].TableName || relation.Object == config.Tables[i].TableName {
					config.Tables[i].AddRelation(relation)
				}
			}
		}
		if len(config.Tables) > 0 {
			if err := resource.UpdateWorldTable(&config, tx); err != nil {
				return err
			}
		}
		if err := resource.UpdateActionTable(&config, tx); err != nil {
			return err
		}
		resource.UpdateExchanges(&config, tx)
		resource.UpdateStateMachineDescriptions(&config, tx)
		resource.UpdateStreams(&config, tx)
		if err := resource.UpdateTasksData(&config, tx); err != nil {
			return err
		}
		if config.EnableGraphQL {
			if err := configStore.SetConfigValueFor("graphql.enable", "true", "backend", tx); err != nil {
				return err
			}
		}
		resource.ImportDataFiles(config.Imports, tx, cruds)
		return nil
	}
}
