package server

import (
	json1 "encoding/json"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/fsm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/table_info"
	yaml2 "github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
)

//import "github.com/daptin/daptin/datastore"

// Load config files which have the naming of the form schema_*_daptin.json/yaml
func LoadConfigFiles() (resource.CmsConfig, []error) {

	var err error

	errs := make([]error, 0)
	var globalInitConfig resource.CmsConfig
	globalInitConfig = resource.CmsConfig{
		Tables:                   make([]table_info.TableInfo, 0),
		Relations:                make([]api2go.TableRelation, 0),
		Imports:                  make([]rootpojo.DataFileImport, 0),
		EnableGraphQL:            false,
		Actions:                  make([]actionresponse.Action, 0),
		StateMachineDescriptions: make([]fsm.LoopbookFsmDescription, 0),
		Streams:                  make([]resource.StreamContract, 0),
		//Marketplaces:             make([]resource.Marketplace, 0),
	}

	globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
	globalInitConfig.Tasks = append(globalInitConfig.Tasks, resource.StandardTasks...)
	globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
	globalInitConfig.Streams = append(globalInitConfig.Streams, resource.StandardStreams...)
	//globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, resource.StandardMarketplaces...)
	globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
	globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)

	schemaPath, specifiedSchemaPath := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")

	var files1 []string
	if specifiedSchemaPath {

		if len(schemaPath) == 0 {
			schemaPath = "."
		}

		if schemaPath[len(schemaPath)-1] != os.PathSeparator {
			schemaPath = schemaPath + string(os.PathSeparator)
		}
		files1, _ = filepath.Glob(schemaPath + "schema_*.*")
	}

	files, err := filepath.Glob("schema_*.*")
	files = append(files, files1...)
	log.Printf("Found files to load: %v", files)

	if err != nil {
		errs = append(errs, err)
		return globalInitConfig, errs
	}

	for _, fileName := range files {
		log.Printf("Process file: %v", fileName)

		fileBytes, err := os.ReadFile(fileName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		initConfig := resource.CmsConfig{}
		initConfigRaw := map[string]interface{}{}
		//fmt.Printf("Loaded config: \n%v", string(fileBytes))

		switch {
		case EndsWithCheck(fileName, "yml"):
			fallthrough
		case EndsWithCheck(fileName, "yaml"):
			jsonBytes, err := yaml2.YAMLToJSON(fileBytes)
			log.Debugf("YAML: %v: %v", string(jsonBytes), err)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			err = json1.Unmarshal(jsonBytes, &initConfig)
			if err == nil {
				_ = json1.Unmarshal(jsonBytes, &initConfigRaw)
			}
			log.Debugf("JSON: %v: %v", string(jsonBytes), err)
			//err = yaml.UnmarshalStrict(fileBytes, &initConfig)
		case EndsWithCheck(fileName, "json"):
			err = json1.Unmarshal(fileBytes, &initConfig)
			if err == nil {
				_ = json1.Unmarshal(fileBytes, &initConfigRaw)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
		default:
			log.Infof("Skipping unsupported schema file: %v", fileName)
			continue
		}

		//js, _ := json.Marshal(initConfig)
		//log.Printf("Loaded config: %v", string(js))

		if err != nil {
			log.Errorf("Failed to load config file: %v", err)
			errs = append(errs, err)
			continue
		}

		tables := make([]table_info.TableInfo, 0)
		for i, table := range initConfig.Tables {
			table.TableName = flect.Underscore(table.TableName)
			if len(table.TableName) < 1 {
				errs = append(errs, fmt.Errorf("schema file %s contains a table without a name", fileName))
				continue
			}

			if err := normalizeSchemaTableColumns(&table); err != nil {
				errs = append(errs, fmt.Errorf("schema file %s: %w", fileName, err))
				continue
			}
			if err := validateSchemaEventHandlers(table); err != nil {
				errs = append(errs, eventHandlerSchemaError{fmt.Errorf("schema file %s: %w", fileName, err)})
				continue
			}
			rawTablesValue := initConfigRaw["Tables"]
			if rawTablesValue == nil {
				rawTablesValue = initConfigRaw["tables"]
			}
			if rawTables, ok := rawTablesValue.([]interface{}); ok && len(rawTables) > i {
				if rawTable, ok := rawTables[i].(map[string]interface{}); ok {
					table.ExplicitFields = make(map[string]bool)
					for key := range rawTable {
						table.ExplicitFields[key] = true
						table.ExplicitFields[flect.Underscore(key)] = true
					}
				}
			}
			tables = append(tables, table)
		}
		initConfig.Tables = tables

		globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)

		//globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
		globalInitConfig.AddRelations(initConfig.Relations...)

		for i, importPath := range initConfig.Imports {
			if importPath.FilePath[0] != '/' {
				importPath.FilePath = schemaPath + importPath.FilePath
				initConfig.Imports[i] = importPath
			}
		}

		globalInitConfig.Imports = append(globalInitConfig.Imports, initConfig.Imports...)
		globalInitConfig.Streams = append(globalInitConfig.Streams, initConfig.Streams...)
		//globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, initConfig.Marketplaces...)
		globalInitConfig.Tasks = append(globalInitConfig.Tasks, initConfig.Tasks...)
		globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)
		globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, initConfig.StateMachineDescriptions...)
		globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, initConfig.ExchangeContracts...)

		for _, action := range initConfig.Actions {
			log.Infof("Action [%v][%v]", fileName, action.Name)
		}

		//for _, marketplace := range initConfig.Marketplaces {
		//	log.Printf("Marketplace [%v][%v]", fileName, marketplace.Endpoint)
		//}

		for _, smd := range initConfig.StateMachineDescriptions {
			log.Infof("StateMachineDescriptions  [%v][%v][%v]", fileName, smd.Name, smd.InitialState)
		}

		if initConfig.EnableGraphQL {
			log.Infof("EnableGraphQL = true")
			globalInitConfig.EnableGraphQL = true
		}

		log.Tracef("File added to config %v", fileName)

	}

	return globalInitConfig, errs

}

type eventHandlerSchemaError struct{ error }

func fatalSchemaEventError(errs []error) error {
	for _, err := range errs {
		var handlerErr eventHandlerSchemaError
		if errors.As(err, &handlerErr) {
			return err
		}
	}
	return nil
}

// normalizeSchemaTableColumns establishes the database/runtime identity of
// every schema column before the table is copied into the global config or
// merged with a built-in table. YAML schemas historically allowed either Name
// or ColumnName, so both forms remain supported.
func normalizeSchemaTableColumns(table *table_info.TableInfo) error {
	seen := make(map[string]struct{}, len(table.Columns))
	for i := range table.Columns {
		column := &table.Columns[i]
		column.Name = strings.TrimSpace(column.Name)
		column.ColumnName = strings.TrimSpace(column.ColumnName)
		if column.Name == "" && column.ColumnName == "" {
			return fmt.Errorf("table %s contains column %d without Name or ColumnName", table.TableName, i)
		}
		if column.ColumnName == "" {
			column.ColumnName = column.Name
		}
		column.ColumnName = flect.Underscore(column.ColumnName)
		if column.ColumnName == "" {
			return fmt.Errorf("table %s contains column %d with an invalid name", table.TableName, i)
		}
		if column.Name == "" {
			column.Name = column.ColumnName
		}
		if _, exists := seen[column.ColumnName]; exists {
			return fmt.Errorf("table %s contains duplicate column %s after normalization", table.TableName, column.ColumnName)
		}
		seen[column.ColumnName] = struct{}{}
	}
	return nil
}

func validateSchemaEventHandlers(table table_info.TableInfo) error {
	for index, handler := range table.EventHandlers {
		stage, operation, ok := strings.Cut(handler.Event, ":")
		if !ok || (stage != "before" && stage != "after") ||
			(operation != "create" && operation != "update" && operation != "delete") {
			return fmt.Errorf("table %s EventHandlers[%d] has invalid event %q", table.TableName, index, handler.Event)
		}
		if handler.Attributes == nil {
			return fmt.Errorf("table %s EventHandlers[%d] has no Attributes", table.TableName, index)
		}
		switch handler.Handler {
		case "validation":
			condition, ok := handler.Attributes["Condition"].(string)
			if stage != "before" || !ok || strings.TrimSpace(condition) == "" {
				return fmt.Errorf("table %s EventHandlers[%d] requires a before event and validation Condition", table.TableName, index)
			}
		case "conformation":
			if stage != "before" || len(handler.Attributes) == 0 {
				return fmt.Errorf("table %s EventHandlers[%d] requires a before event and conformation fields", table.TableName, index)
			}
		case "js":
			script, ok := handler.Attributes["Script"].(string)
			if stage != "before" || !ok || strings.TrimSpace(script) == "" {
				return fmt.Errorf("table %s EventHandlers[%d] requires a before event and Script", table.TableName, index)
			}
		case "action.execute":
			action, actionOK := handler.Attributes["ActionName"].(string)
			entity, entityOK := handler.Attributes["EntityName"].(string)
			if stage != "after" || !actionOK || !entityOK || strings.TrimSpace(action) == "" || strings.TrimSpace(entity) == "" {
				return fmt.Errorf("table %s EventHandlers[%d] requires an after event, ActionName and EntityName", table.TableName, index)
			}
		case "http.post", "async.http.post":
			url, ok := handler.Attributes["Url"].(string)
			if stage != "after" || !ok || strings.TrimSpace(url) == "" {
				return fmt.Errorf("table %s EventHandlers[%d] requires an after event and Url", table.TableName, index)
			}
			if headers := handler.Attributes["Headers"]; headers != nil {
				if _, ok := headers.(map[string]interface{}); !ok {
					return fmt.Errorf("table %s EventHandlers[%d] Headers must be an object", table.TableName, index)
				}
			}
			if body := handler.Attributes["Body"]; body != nil {
				if _, ok := body.(map[string]interface{}); !ok {
					return fmt.Errorf("table %s EventHandlers[%d] Body must be an object", table.TableName, index)
				}
			}
		default:
			return fmt.Errorf("table %s EventHandlers[%d] has unsupported Handler %q", table.TableName, index, handler.Handler)
		}
	}
	return nil
}

func schemaEventExchanges(table table_info.TableInfo) []resource.ExchangeContract {
	exchanges := make([]resource.ExchangeContract, 0)
	for index, handler := range table.EventHandlers {
		stage, operation, _ := strings.Cut(handler.Event, ":")
		if stage != "after" {
			continue
		}
		name := "__daptin_event_" + resource.GetMD5HashString(fmt.Sprintf("%s/%d", table.TableName, index))
		exchange := resource.ExchangeContract{
			Name:             name,
			SourceType:       "self",
			SourceAttributes: map[string]interface{}{"name": table.TableName},
			Attributes: map[string]interface{}{
				"name": table.TableName, "hook": "after", "methods": []interface{}{map[string]string{"create": "post", "update": "patch", "delete": "delete"}[operation]},
				"condition": handler.Condition, "event_handler": true, "event_order": index,
				"wait_for_delivery": handler.Handler == "http.post",
			},
		}
		switch handler.Handler {
		case "action.execute":
			exchange.TargetType = "action"
			exchange.TargetAttributes = map[string]interface{}{
				"type": handler.Attributes["EntityName"], "action": handler.Attributes["ActionName"], "attributes": map[string]interface{}{},
			}
		default:
			exchange.TargetType = "rest"
			exchange.TargetAttributes = map[string]interface{}{"url": handler.Attributes["Url"], "method": "POST"}
			if headers := handler.Attributes["Headers"]; headers != nil {
				exchange.TargetAttributes["headers"] = headers
			}
			if body := handler.Attributes["Body"]; body != nil {
				exchange.TargetAttributes["body"] = body
			} else {
				exchange.TargetAttributes["body"] = map[string]interface{}{
					"event": handler.Event, "table": table.TableName, "record": "{{.}}",
					"user": "{{.user}}", "timestamp": "{{now}}",
				}
			}
		}
		exchanges = append(exchanges, exchange)
	}
	return exchanges
}
