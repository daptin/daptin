package server

import (
	json1 "encoding/json"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/fsm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/table_info"
	yaml2 "github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	"github.com/naoina/toml"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
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
			log.Debugf("JSON: %v: %v", string(jsonBytes), err)
			//err = yaml.UnmarshalStrict(fileBytes, &initConfig)
		case EndsWithCheck(fileName, "json"):
			err = json1.Unmarshal(fileBytes, &initConfig)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		case EndsWithCheck(fileName, "toml"):
			err = toml.Unmarshal(fileBytes, &initConfig)
			if err != nil {
				errs = append(errs, err)
				continue
			}

		}

		//js, _ := json.Marshal(initConfig)
		//log.Printf("Loaded config: %v", string(js))

		if err != nil {
			log.Errorf("Failed to load config file: %v", err)
			errs = append(errs, err)
			continue
		}

		tables := make([]table_info.TableInfo, 0)
		for _, table := range initConfig.Tables {
			table.TableName = flect.Underscore(table.TableName)
			if len(table.TableName) < 1 {
				continue
			}

			for j, col := range table.Columns {
				table.Columns[j].ColumnName = flect.Underscore(col.ColumnName)
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

		for _, table := range initConfig.Tables {
			for i, col := range table.Columns {
				if col.Name == "" && col.ColumnName != "" {
					col.Name = col.ColumnName
				} else if col.Name != "" && col.ColumnName == "" {
					col.ColumnName = col.Name
				} else if col.Name == "" && col.ColumnName == "" {
					log.Printf("Error, column without name: %v", table)
				}
				table.Columns[i] = col
			}
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
