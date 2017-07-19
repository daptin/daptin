package server

import (
  "io/ioutil"
  "path/filepath"
  "github.com/artpar/goms/server/resource"
  "github.com/artpar/api2go"
  log "github.com/sirupsen/logrus"
  "encoding/json"
  "gopkg.in/gin-gonic/gin.v1"
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
    Actions:                  make([]resource.Action, 0),
    StateMachineDescriptions: make([]resource.LoopbookFsmDescription, 0),
  }

  globalInitConfig.Tables = append(globalInitConfig.Tables, resource.StandardTables...)
  globalInitConfig.Relations = append(globalInitConfig.Relations, resource.StandardRelations...)
  globalInitConfig.Actions = append(globalInitConfig.Actions, resource.SystemActions...)
  globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, resource.SystemSmds...)
  globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, resource.SystemExchanges...)

  files, err := filepath.Glob("schema_*_gocms.json")
  log.Infof("Found files to load: %v", files)

  if err != nil {
    errs = append(errs, err)
    return globalInitConfig, errs
  }

  for _, fileName := range files {
    log.Infof("Process file: %v", fileName)

    fileContents, err := ioutil.ReadFile(fileName)
    if err != nil {
      errs = append(errs, err)
      continue
    }
    var initConfig resource.CmsConfig
    err = json.Unmarshal(fileContents, &initConfig)
    if err != nil {
      errs = append(errs, err)
      continue
    }

    globalInitConfig.Tables = append(globalInitConfig.Tables, initConfig.Tables...)
    globalInitConfig.Relations = append(globalInitConfig.Relations, initConfig.Relations...)
    globalInitConfig.Actions = append(globalInitConfig.Actions, initConfig.Actions...)
    globalInitConfig.StateMachineDescriptions = append(globalInitConfig.StateMachineDescriptions, initConfig.StateMachineDescriptions...)
    globalInitConfig.ExchangeContracts = append(globalInitConfig.ExchangeContracts, initConfig.ExchangeContracts...)

    //for _, table := range initConfig.Tables {
    //log.Infof("Table: %v: %v", table.TableName, table.Relations)
    //}

    log.Infof("File added to config, deleting %v", fileName)

  }

  return globalInitConfig, errs

}
