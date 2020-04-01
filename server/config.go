package server

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	yaml2 "github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	"github.com/naoina/toml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
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
		Tables:                   make([]resource.TableInfo, 0),
		Relations:                make([]api2go.TableRelation, 0),
		Imports:                  make([]resource.DataFileImport, 0),
		EnableGraphQL:            false,
		Actions:                  make([]resource.Action, 0),
		StateMachineDescriptions: make([]resource.LoopbookFsmDescription, 0),
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
	log.Infof("Found files to load: %v", files)

	if err != nil {
		errs = append(errs, err)
		return globalInitConfig, errs
	}

	for _, fileName := range files {
		log.Infof("Process file: %v", fileName)

		fileBytes, err := ioutil.ReadFile(fileName)
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
			if err != nil {
				errs = append(errs, err)
				continue
			}
			err = json.Unmarshal(jsonBytes, &initConfig)
			//err = yaml.UnmarshalStrict(fileBytes, &initConfig)
		case EndsWithCheck(fileName, "json"):
			err = json.Unmarshal(fileBytes, &initConfig)
		case EndsWithCheck(fileName, "toml"):
			err = toml.Unmarshal(fileBytes, &initConfig)

		}

		//js, _ := json.Marshal(initConfig)
		//log.Printf("Loaded config: %v", string(js))

		if err != nil {
			log.Errorf("Failed to load config file: %v", err)
			errs = append(errs, err)
			continue
		}

		for i, table := range initConfig.Tables {
			table.TableName = flect.Underscore(table.TableName)

			for j, col := range table.Columns {
				table.Columns[j].ColumnName = flect.Underscore(col.ColumnName)
			}
			initConfig.Tables[i] = table
		}

		globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)

		//globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
		globalInitConfig.AddRelations(initConfig.Relations...)

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
		//	log.Infof("Marketplace [%v][%v]", fileName, marketplace.Endpoint)
		//}

		for _, smd := range initConfig.StateMachineDescriptions {
			log.Infof("SMD  [%v][%v][%v]", fileName, smd.Name, smd.InitialState)
		}

		if initConfig.EnableGraphQL {
			globalInitConfig.EnableGraphQL = true
		}

		//log.Infof("File added to config, deleting %v", fileName)

	}

	return globalInitConfig, errs

}
