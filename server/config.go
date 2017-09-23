package server

import (
	"path/filepath"
	"github.com/artpar/goms/server/resource"
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/go-playground/validator.v9"
)

//import "github.com/artpar/goms/datastore"

func CreateConfigHandler(configStore *resource.ConfigStore) func(context *gin.Context) {

	return func(c *gin.Context) {
		webConfig := configStore.GetWebConfig()
		c.JSON(200, webConfig)
	}
}

func loadConfigFiles() (resource.CmsConfig, []error) {

	var err error

	errs := make([]error, 0)
	var globalInitConfig resource.CmsConfig
	globalInitConfig = resource.CmsConfig{
		Tables:                   make([]resource.TableInfo, 0),
		Relations:                make([]api2go.TableRelation, 0),
		Imports:                  make([]resource.DataFileImport, 0),
		Actions:                  make([]resource.Action, 0),
		StateMachineDescriptions: make([]resource.LoopbookFsmDescription, 0),
		Streams:                  make([]resource.StreamContract, 0),
		Marketplaces:             make([]resource.Marketplace, 0),
	}

	globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
	globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
	globalInitConfig.Streams = append(globalInitConfig.Streams, resource.StandardStreams...)
	globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, resource.StandardMarketplaces...)
	globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
	globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)

	files, err := filepath.Glob("schema_*_goms.*")
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
		//err = viper.UnmarshalKey("tables", &initConfig.Relations)
		//err = viper.UnmarshalKey("tables", &initConfig.Streams)
		//err = viper.UnmarshalKey("tables", &initConfig.ExchangeContracts)
		//err = viper.UnmarshalKey("tables", &initConfig.StateMachineDescriptions)
		//err = viper.UnmarshalKey("tables", &initConfig.Actions)
		//err = viper.UnmarshalKey("tables", &initConfig.Imports)
		all := viper.AllSettings()
		log.Infof("All settings", all)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)
		globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
		globalInitConfig.Imports = append(globalInitConfig.Imports, initConfig.Imports...)
		globalInitConfig.Streams = append(globalInitConfig.Streams, initConfig.Streams...)
		globalInitConfig.Marketplaces = append(globalInitConfig.Marketplaces, initConfig.Marketplaces...)
		globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)
		globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, initConfig.StateMachineDescriptions...)
		globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, initConfig.ExchangeContracts...)

		for _, table := range initConfig.Tables {
			log.Infof("Table: %v: %v", table.TableName, table.Columns)
		}

		for _, action := range initConfig.Actions {
			log.Infof("Action [%v][%v]", fileName, action.Name)
		}

		for _, marketplace := range initConfig.Marketplaces {
			log.Infof("Marketplace [%v][%v]", fileName, marketplace.Endpoint)
		}

		for _, smd := range initConfig.StateMachineDescriptions {
			log.Infof("Marketplace [%v][%v]", fileName, smd.Name, smd.InitialState)
		}

		//log.Infof("File added to config, deleting %v", fileName)

	}

	globalInitConfig.Validator = validator.New()
	return globalInitConfig, errs

}
