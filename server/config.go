package server

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/flect"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

//import "github.com/daptin/daptin/datastore"

func CreateConfigHandler(configStore *resource.ConfigStore) func(context *gin.Context) {

	return func(c *gin.Context) {
		webConfig := configStore.GetWebConfig()
		c.JSON(200, webConfig)
	}
}

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
		Marketplaces:             make([]resource.Marketplace, 0),
	}

	globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
	globalInitConfig.Tasks = append(globalInitConfig.Tasks, resource.StandardTasks...)
	globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
	globalInitConfig.Streams = append(globalInitConfig.Streams, resource.StandardStreams...)
	globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, resource.StandardMarketplaces...)
	globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
	globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)

	schemaPath, specifiedSchemaPath := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")

	var files1 []string
	if specifiedSchemaPath {

		if len(schemaPath) == 0 {
			schemaPath = "."
		}

		if schemaPath[len(schemaPath)-1] != '/' {
			schemaPath = schemaPath + "/"
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

		viper.SetConfigFile(fileName)

		err = viper.ReadInConfig()
		if err != nil {
			errs = append(errs, err)
		}

		initConfig := resource.CmsConfig{}
		err = viper.Unmarshal(&initConfig)

		if err != nil {
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
		globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, initConfig.Marketplaces...)
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

		for _, marketplace := range initConfig.Marketplaces {
			log.Infof("Marketplace [%v][%v]", fileName, marketplace.Endpoint)
		}

		for _, smd := range initConfig.StateMachineDescriptions {
			log.Infof("Marketplace [%v][%v][%v]", fileName, smd.Name, smd.InitialState)
		}

		if initConfig.EnableGraphQL {
			globalInitConfig.EnableGraphQL = true
		}

		//log.Infof("File added to config, deleting %v", fileName)

	}

	return globalInitConfig, errs

}
