package resource

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/alexeyco/simpletable"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/tealeg/xlsx"
	"gopkg.in/Masterminds/squirrel.v1"
	"io/ioutil"
	"net/http"
	"os"
	"time"
	"github.com/daptin/daptin/server/statementbuilder"
)

func (resource *DbResource) UpdateAccessTokenByTokenId(id int64, accessToken string, expiresIn int64) error {

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return err
	}

	accessToken, err = Encrypt([]byte(encryptionSecret), accessToken)
	if err != nil {
		return err
	}

	s, v, err := statementbuilder.Squirrel.Update("oauth_token").
		Set("access_token", accessToken).
		Set("expires_in", expiresIn).
		Where(squirrel.Eq{"id": id}).ToSql()

	if err != nil {
		return err
	}

	_, err = resource.db.Exec(s, v...)
	return err

}

func (resource *DbResource) UpdateAccessTokenByTokenReferenceId(referenceId string, accessToken string, expiresIn int64) error {

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return err
	}

	accessToken, err = Encrypt([]byte(encryptionSecret), accessToken)
	if err != nil {
		return err
	}

	s, v, err := statementbuilder.Squirrel.Update("oauth_token").
		Set("access_token", accessToken).
		Set("expires_in", expiresIn).
		Where(squirrel.Eq{"reference_id": referenceId}).ToSql()

	if err != nil {
		return err
	}

	_, err = resource.db.Exec(s, v...)
	return err

}

func UpdateStandardData(initConfig *CmsConfig, db database.DatabaseConnection) {

	//for _, row := range StandardData {
	//
	//
	//	typeName := row.Data["__type"]
	//
	//}

}

func UpdateTasksData(initConfig *CmsConfig, db database.DatabaseConnection) error {

	tasks, err := GetTasks(db)
	if err != nil {
		return err
	}
	taskMap := make(map[string]Task)
	for _, job := range tasks {
		taskMap[job.Name] = job
	}

	newTasks := initConfig.Tasks

	for _, newTask := range newTasks {

		_, ok := taskMap[newTask.Name]
		taskMap[newTask.Name] = newTask
		var s string
		var v []interface{}

		if ok {
			log.Printf("Updating existing cron job: %v", newTask.Name)

			s, v, err = statementbuilder.Squirrel.Update("task").
				Set("active", newTask.Active).
				Set("schedule", newTask.Schedule).
				Set("attributes", toJson(newTask.Attributes)).
				Set("action_name", newTask.ActionName).
				Set("entity_name", newTask.EntityName).
				ToSql()

		} else {

			uuidRef, err := uuid.NewV4()
			if err != nil {
				return err
			}
			refId := uuidRef.String()
			s, v, err = statementbuilder.Squirrel.Insert("task").
				Columns("name", "schedule", "active", "action_name", "entity_name", "reference_id", "attributes", "created_at").
				Values(newTask.Name, newTask.Schedule, newTask.Active, newTask.ActionName, newTask.EntityName, refId, toJson(newTask.Attributes), time.Now()).
				ToSql()

		}

		if err != nil {
			return err
		}

		_, err = db.Exec(s, v...)
		if err != nil {
			return err
		}

	}

	finalJobs := make([]Task, 0)

	for _, job := range taskMap {
		finalJobs = append(finalJobs, job)
	}

	initConfig.Tasks = finalJobs

	return nil

}

func GetTasks(connection database.DatabaseConnection) ([]Task, error) {

	s, v, err := statementbuilder.Squirrel.Select("name", "job_type as jobtype", "schedule", "active", "attributes as attributesjson", "as_user_id as AsUserEmail").From("task").Where(squirrel.Eq{"active": true}).ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := connection.Queryx(s, v...)
	if err != nil {
		return nil, err
	}

	jobs := make([]Task, 0)

	for rows.Next() {
		var job Task

		err = rows.StructScan(&job)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(job.AttributesJson), &job.Attributes)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil

}

func UpdateMarketplaces(initConfig *CmsConfig, db database.DatabaseConnection) {

	s, v, err := statementbuilder.Squirrel.Select("endpoint", "root_path").From("marketplace").ToSql()

	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)

	CheckErr(err, "Failed to create query for marketplace select")

	res, err := db.Queryx(s, v...)
	CheckErr(err, "Failed to scan market places")
	defer res.Close()
	existingMarketPlaces := make(map[string]Marketplace)
	for res.Next() {
		m := make(map[string]interface{})
		res.MapScan(m)
		streamNameString, ok := m["endpoint"].(string)
		if !ok {
			streamName := string(m["endpoint"].([]uint8))
			streamNameString = streamName
		}
		rootPath := m["root_path"]
		rootPathString := ""
		if rootPath != nil {
			rps, ok := rootPath.(string)
			if !ok {
				rootPathString = string(rootPath.([]uint8))
			} else {
				rootPathString = rps
			}
		}

		endPointString, ok := m["endpoint"].(string)
		if !ok {
			endPointString = string(m["endpoint"].([]uint8))
		}
		existingMarketPlaces[streamNameString] = Marketplace{
			Endpoint: endPointString,
			RootPath: rootPathString,
		}

	}

	log.Infof("We have %d existing market places", len(existingMarketPlaces))

	for _, marketplace := range initConfig.Marketplaces {

		log.Infof("Process marketplace [%v]", marketplace.Endpoint)

		schema, err := json.Marshal(marketplace)
		CheckErr(err, "Failed to marshal marketplace contract")

		_, ok := existingMarketPlaces[marketplace.Endpoint]

		if ok {

			log.Infof("Marketplace [%v] already present in db, updating db values", marketplace.Endpoint)

			s, v, err := statementbuilder.Squirrel.Update("marketplace").
				Set("root_path", marketplace.RootPath).
				Where(squirrel.Eq{"endpoint": marketplace.Endpoint}).
				ToSql()

			_, err = db.Exec(s, v...)
			CheckErr(err, "Failed to update table for marketplace contract")

		} else {
			log.Infof("We have a new marketplace contract: %v", marketplace.Endpoint)

			existingMarketPlaces[marketplace.Endpoint] = marketplace
			u, _ := uuid.NewV4()

			s, v, err := statementbuilder.Squirrel.Insert("marketplace").Columns("endpoint", "root_path", "reference_id", "permission", "user_account_id").
				Values(marketplace.Endpoint, schema, u.String(), auth.DEFAULT_PERMISSION, adminUserId).ToSql()

			_, err = db.Exec(s, v...)
			CheckErr(err, "Failed to insert into db about marketplace [%v]: %v", marketplace.Endpoint, err)

		}

	}

	allMarketPlaces := make([]Marketplace, 0)

	for _, marketplace := range existingMarketPlaces {

		allMarketPlaces = append(allMarketPlaces, marketplace)

	}

	initConfig.Marketplaces = allMarketPlaces

}

func UpdateStreams(initConfig *CmsConfig, db database.DatabaseConnection) {

	s, v, err := statementbuilder.Squirrel.Select("stream_name", "stream_contract").From("stream").ToSql()

	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)

	CheckErr(err, "Failed to create query for stream select")

	res, err := db.Queryx(s, v...)
	CheckErr(err, "Failed to query streams")
	if err != nil {
		return
	}
	existingStreams := make(map[string]StreamContract)
	defer res.Close()
	for res.Next() {
		m := make(map[string]interface{})
		res.MapScan(m)
		streamName, ok := m["stream_name"].(string)
		if !ok {
			streamName = string(m["stream_name"].([]uint8))
		}
		var contract StreamContract

		streamContractString, ok := m["stream_contract"].(string)
		if !ok {
			streamContractString = string(m["stream_contract"].([]uint8))
		}
		err := json.Unmarshal([]byte(streamContractString), &contract)
		CheckErr(err, "Failed to unmarshal stream contract for [%v]: %v", streamName)
		existingStreams[streamName] = contract

	}

	for i, stream := range initConfig.Streams {
		for j, col := range stream.Columns {
			if col.ColumnName == "" {
				col.ColumnName = col.Name
				stream.Columns[j] = col
			}
		}
		initConfig.Streams[i] = stream
	}

	for i, stream := range existingStreams {
		for j, col := range stream.Columns {
			if col.ColumnName == "" {
				col.ColumnName = col.Name
				stream.Columns[j] = col
			}
		}
		existingStreams[i] = stream
	}

	log.Infof("We have %d existing streams", len(existingStreams))

	for _, stream := range initConfig.Streams {

		log.Infof("Process stream [%v]", stream.StreamName)

		schema, err := json.Marshal(stream)
		CheckErr(err, "Failed to marshal stream contract")

		_, ok := existingStreams[stream.StreamName]

		if ok {

			log.Infof("Stream [%v] already present in db, updating db values", stream.StreamName)

			s, v, err := statementbuilder.Squirrel.Update("stream").
				Set("stream_contract", schema).
				Where(squirrel.Eq{"stream_name": stream.StreamName}).
				ToSql()

			_, err = db.Exec(s, v...)
			CheckErr(err, "Failed to update table for stream contract")

		} else {
			log.Infof("We have a new stream contract: %v", stream.StreamName)

			existingStreams[stream.StreamName] = stream

			u, _ := uuid.NewV4()
			s, v, err := statementbuilder.Squirrel.Insert("stream").Columns("stream_name", "stream_contract", "reference_id", "permission", "user_account_id").
				Values(stream.StreamName, schema, u.String(), auth.DEFAULT_PERMISSION, adminUserId).ToSql()

			_, err = db.Exec(s, v...)
			CheckErr(err, "Failed to insert into db about stream [%v]: %v", stream.StreamName, err)

		}

	}

	allStreams := make([]StreamContract, 0)

	for _, stream := range existingStreams {

		allStreams = append(allStreams, stream)

	}

	initConfig.Streams = allStreams

}

func UpdateExchanges(initConfig *CmsConfig, db database.DatabaseConnection) {

	log.Infof("We have %d data exchange updates", len(initConfig.ExchangeContracts))

	adminId, _ := GetAdminUserIdAndUserGroupId(db)

	for _, exchange := range initConfig.ExchangeContracts {

		s, v, err := statementbuilder.Squirrel.Select("reference_id").From("data_exchange").Where(squirrel.Eq{"name": exchange.Name}).ToSql()

		if err != nil {
			log.Errorf("Failed to query existing data exchange: %v", err)
			continue
		}

		var referenceId string
		err = db.QueryRowx(s, v...).Scan(&referenceId)

		if err != nil {
			log.Infof("No existing data exchange for  [%v]", exchange.Name)
		}

		if err == nil {

			attrsJson, err := json.Marshal(exchange.Attributes)
			CheckErr(err, "Failed to marshal attributes to json: %v")

			optionsJson, err := json.Marshal(exchange.Options)

			CheckErr(err, "Failed to marshal options to json: %v")
			sourceAttrsJson, err := json.Marshal(exchange.SourceAttributes)
			CheckErr(err, "Failed to marshal source attrs to json")
			targetAttrsJson, err := json.Marshal(exchange.TargetAttributes)
			CheckErr(err, "Failed to marshal target attrs to json")

			s, v, err = statementbuilder.Squirrel.
				Update("data_exchange").
				Set("source_attributes", sourceAttrsJson).
				Set("source_type", exchange.SourceType).
				Set("target_attributes", targetAttrsJson).
				Set("target_type", exchange.TargetType).
				Set("attributes", attrsJson).
				Set("options", optionsJson).
				Set("updated_at", time.Now()).
				Set("user_account_id", adminId).
				Where(squirrel.Eq{"reference_id": referenceId}).
				ToSql()

			_, err = db.Exec(s, v...)

			CheckErr(err, "Failed to update exchange row")

		} else {
			attrsJson, err := json.Marshal(exchange.Attributes)
			CheckErr(err, "Failed to marshal attributes to json")

			optionsJson, err := json.Marshal(exchange.Options)
			CheckErr(err, "Failed to marshal options to json")
			sourceAttrsJson, err := json.Marshal(exchange.SourceAttributes)
			CheckErr(err, "Failed to marshal source attributes to json")
			targetAttrsJson, err := json.Marshal(exchange.TargetAttributes)
			CheckErr(err, "Failed to marshal target attributes to json")
			u, _ := uuid.NewV4()

			s, v, err = statementbuilder.Squirrel.
				Insert("data_exchange").
				Columns("permission", "name", "source_attributes", "source_type", "target_attributes", "target_type",
				"attributes", "options", "created_at", "user_account_id", "reference_id").
				Values(auth.DEFAULT_PERMISSION, exchange.Name, sourceAttrsJson, exchange.SourceType, targetAttrsJson, exchange.TargetType,
				attrsJson, optionsJson, time.Now(), adminId, u.String()).
				ToSql()

			_, err = db.Exec(s, v...)

			CheckErr(err, "Failed to insert exchange row")

		}

	}

	allExchnages := make([]ExchangeContract, 0)

	s, v, err := statementbuilder.Squirrel.Select("name", "source_attributes", "source_type", "target_attributes",
		"target_type", "attributes", "options", "oauth_token_id").
		From("data_exchange").ToSql()

	rows, err := db.Queryx(s, v...)
	CheckErr(err, "Failed to query existing exchanges")

	if err == nil {
		defer rows.Close()
		for rows.Next() {

			var name, source_type, target_type string
			var attributes, source_attributes, target_attributes, options []byte
			var oauth_token_id *int64

			var ec ExchangeContract
			err = rows.Scan(&name, &source_attributes, &source_type, &target_attributes, &target_type, &attributes, &options, &oauth_token_id)
			CheckErr(err, "Failed to Scan existing exchanges")

			m := make(map[string]interface{})
			err = json.Unmarshal(source_attributes, &m)
			ec.SourceAttributes = m
			CheckErr(err, "Failed to unmarshal source attributes")

			m = make(map[string]interface{})
			err = json.Unmarshal(target_attributes, &m)
			ec.TargetAttributes = m
			CheckErr(err, "Failed to unmarshal target attributes")

			ec.Name = name
			ec.SourceType = source_type
			ec.TargetType = target_type

			var columnMapping []ColumnMap
			err = json.Unmarshal(attributes, &columnMapping)
			CheckErr(err, "Failed to unmarshal column mapping")

			ec.Attributes = columnMapping
			err = json.Unmarshal(options, &ec.Options)
			CheckErr(err, "Failed to unmarshal exchange options")

			if oauth_token_id == nil {
			}

			ec.OauthTokenId = oauth_token_id

			allExchnages = append(allExchnages, ec)
		}
	}

	initConfig.ExchangeContracts = allExchnages

}

func UpdateStateMachineDescriptions(initConfig *CmsConfig, db database.DatabaseConnection) {

	log.Infof("We have %d state machine descriptions", len(initConfig.StateMachineDescriptions))

	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)

	for i := range initConfig.Tables {
		ar := make([]LoopbookFsmDescription, 0)
		initConfig.Tables[i].StateMachines = ar
	}

	for _, smd := range initConfig.StateMachineDescriptions {

		s, v, err := statementbuilder.Squirrel.Select("reference_id").From("smd").Where(squirrel.Eq{"name": smd.Name}).ToSql()
		if err != nil {
			log.Errorf("Failed to create select smd query: %v", err)
			continue
		}

		var refId string
		err = db.QueryRowx(s, v...).Scan(&refId)
		if err != nil {

			// no existing row

			eventsDescription, err := json.Marshal(smd.Events)
			if err != nil {
				log.Errorf("Failed to convert to json: %v", err)
				continue
			}
			u, _ := uuid.NewV4()

			insertMap := map[string]interface{}{}
			insertMap["name"] = smd.Name
			insertMap["label"] = smd.Label
			insertMap["initial_state"] = smd.InitialState
			insertMap["events"] = eventsDescription
			insertMap["reference_id"] = u.String()
			insertMap["permission"] = auth.DEFAULT_PERMISSION
			insertMap["user_account_id"] = adminUserId
			s, v, err := statementbuilder.Squirrel.Insert("smd").SetMap(insertMap).ToSql()

			if err != nil {
				log.Errorf("Failed to create update smd query: %v", err)
				continue
			}

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to execute update smd query [%v]: %v", s, err)
			}

		} else {
			// no existing row

			eventsDescription, err := json.Marshal(smd.Events)
			if err != nil {
				log.Errorf("Failed to convert to json: %v", err)
				continue
			}

			updateMap := map[string]interface{}{}
			updateMap["name"] = smd.Name
			updateMap["label"] = smd.Label
			updateMap["initial_state"] = smd.InitialState
			updateMap["events"] = eventsDescription
			updateMap["user_account_id"] = adminUserId
			s, v, err := statementbuilder.Squirrel.Update("smd").SetMap(updateMap).Where(squirrel.Eq{"reference_id": refId}).ToSql()

			if err != nil {
				log.Errorf("Failed to create update smd query: %v", err)
				continue
			}

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to execute update smd query [%v]: %v", s, err)
			}

		}

	}
}

func UpdateActionTable(initConfig *CmsConfig, db database.DatabaseConnection) error {

	var err error

	currentActions, err := GetActionMapByTypeName(db)
	if err != nil {
		return err
	}

	worldTableMap, err := GetWorldTableMapBy("table_name", db)
	if err != nil {
		return err
	}
	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)

	for _, action := range initConfig.Actions {

		log.Infof("Check action: %v@%v", action.Name, action.OnType)
		world, ok := worldTableMap[action.OnType]
		if !ok {
			log.Errorf("Action [%v] defined on unknown type [%v]", action.Name, action.OnType)
			continue
		}

		var worldIdString string
		worldId := world["id"]
		worldIdUint8, ok := worldId.([]uint8)
		if !ok {
			worldIdString = fmt.Sprintf("%v", worldId)
		} else {
			worldIdString = string(worldIdUint8)
		}
		_, ok = currentActions[worldIdString][action.Name]
		if ok {
			log.Infof("Action [%v] on [%v] already present in database", action.Name, action.OnType)

			actionJson, err := json.Marshal(action)
			CheckErr(err, "Failed to marshal action infields")
			s, v, err := statementbuilder.Squirrel.Update("action").
				Set("label", action.Label).
				Set("world_id", worldId).
				Set("action_schema", actionJson).
				Set("instance_optional", action.InstanceOptional).Where(squirrel.Eq{"action_name": action.Name}).ToSql()

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to insert action [%v]: %v", action.Name, err)
			}
		} else {
			log.Infof("Action [%v] is new, adding action: %v @ %v", action.Name, action.OnType)

			actionSchema, _ := json.Marshal(action)

			u, _ := uuid.NewV4()
			s, v, err := statementbuilder.Squirrel.Insert("action").Columns(
				"action_name",
				"label",
				"world_id",
				"action_schema",
				"instance_optional",
				"user_account_id",
				"reference_id",
				"permission").Values(
				action.Name,
				action.Label,
				worldId,
				actionSchema,
				action.InstanceOptional,
				adminUserId,
				u.String(),
				auth.ALLOW_ALL_PERMISSIONS.IntValue()).ToSql()

			_, err = db.Exec(s, v...)
			if err != nil {
				log.Errorf("Failed to insert action [%v]: %v", action.Name, err)
			}
		}
	}

	return nil
}

func ImportDataFiles(initConfig *CmsConfig, db database.DatabaseConnection, cruds map[string]*DbResource) {
	importCount := len(initConfig.Imports)

	if importCount == 0 {
		return
	}

	log.Printf("Importing [%v] data files", importCount)
	ctx := context.TODO()
	pr1 := http.Request{
		Method: "POST",
	}
	pr := pr1.WithContext(ctx)
	adminUserId, _ := GetAdminUserIdAndUserGroupId(db)
	adminUser, err := cruds["world"].GetIdToObject("user_account", adminUserId)
	if err != nil {
		log.Errorf("No admin user present")
	} else {
		adminUserRefId := adminUser["reference_id"].(string)

		sessionUser := &auth.SessionUser{
			UserId:          adminUserId,
			UserReferenceId: adminUserRefId,
			Groups:          []auth.GroupPermission{},
		}

		pr = pr.WithContext(context.WithValue(pr.Context(), "user", sessionUser))

	}

	req := api2go.Request{
		PlainRequest: pr,
	}

	for _, importFile := range initConfig.Imports {

		log.Infof("Process import file %v", importFile.String())
		fileBytes, err := ioutil.ReadFile(importFile.FilePath)
		if err != nil {
			log.Errorf("Failed to read file [%v]: %v", importFile.FilePath, err)
			continue
		}

		importSuccess := false

		switch importFile.FileType {

		case "json":

			jsonData := make(map[string][]map[string]interface{}, 0)
			err := json.Unmarshal(fileBytes, &jsonData)
			if err != nil {
				log.Errorf("Failed to read content as json to import: %v", err)
				continue
			}

			for typeName, data := range jsonData {
				errs := ImportDataMapArray(data, cruds[typeName], req)
				if len(errs) > 0 {
					for _, err := range errs {
						log.Errorf("Error while importing json data: %v", err)
					}
				}
			}

		case "xlsx":
			xlsxFile, err := xlsx.OpenBinary(fileBytes)
			if err != nil {
				log.Errorf("Failed to read file [%v] as xlsx file: %v", importFile.FilePath, err)
			}

			data, _, err := GetDataArray(xlsxFile.Sheets[0])
			if err != nil {
				log.Errorf("Failed to sheet 0 data to import: %v", err)
				continue
			}

			importSuccess = true
			ImportDataMapArray(data, cruds[importFile.Entity], req)

		case "csv":

			csvReader := csv.NewReader(bytes.NewReader(fileBytes))
			data, err := csvReader.ReadAll()
			CheckErr(err, "Failed to read csv file [%v]", importFile.FilePath)
			if err != nil {
				continue
			}

			header := data[0]
			importSuccess = true
			for i, h := range header {
				header[i] = SmallSnakeCaseText(h)
			}
			ImportDataStringArray(data, header, importFile.Entity, cruds[importFile.Entity], req)

		default:
			CheckErr(errors.New("unknown file type"), "Failed to import [%v]: [%v]", importFile.FileType, importFile.FilePath)
		}

		if importSuccess {
			err := os.Remove(importFile.FilePath)
			CheckErr(err, "Failed to remove import file after import [%v]", importFile.FilePath)
		}

	}

}

func ImportDataMapArray(data []map[string]interface{}, crud *DbResource, req api2go.Request) []error {
	errs := make([]error, 0)
	for _, row := range data {
		model := api2go.NewApi2GoModelWithData(crud.tableInfo.TableName, nil, auth.DEFAULT_PERMISSION.IntValue(), nil, row)
		_, err := crud.Create(model, req)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func ImportDataStringArray(data [][]string, headers []string, entityName string, crud *DbResource, req api2go.Request) []error {
	errs := make([]error, 0)
	for _, rowArray := range data {

		rowMap := make(map[string]interface{})

		for i, header := range headers {
			rowMap[header] = rowArray[i]
		}
		model := api2go.NewApi2GoModelWithData(entityName, nil, auth.DEFAULT_PERMISSION.IntValue(), nil, rowMap)
		_, err := crud.Create(model, req)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func UpdateWorldTable(initConfig *CmsConfig, db *sqlx.Tx) {

	tx := db
	var err error
	log.Infof("Start table check")

	//tx.Queryx("SET FOREIGN_KEY_CHECKS=0;")

	var userId int
	var userGroupId int
	var systemHasNoAdmin = false
	var userCount int
	s, v, err := statementbuilder.Squirrel.Select("count(*)").From("user_account").ToSql()
	err = tx.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")
	//log.Infof("Current user grou")
	if userCount < 1 {
		systemHasNoAdmin = true
		u, _ := uuid.NewV4()
		u2 := u.String()

		s, v, err := statementbuilder.Squirrel.Insert("user_account").
			Columns("name", "email", "reference_id", "permission").
			Values("guest", "guest@cms.go", u2, auth.DEFAULT_PERMISSION).ToSql()

		CheckErr(err, "Failed to create insert sql")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user: %v", s)

		s, v, err = statementbuilder.Squirrel.Select("id").From("user_account").Where(squirrel.Eq{"reference_id": u2}).ToSql()
		CheckErr(err, "Failed to create select user sql ")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select user for world update: %v", s)

		u, _ = uuid.NewV4()
		u1 := u.String()
		s, v, err = statementbuilder.Squirrel.Insert("usergroup").
			Columns("name", "reference_id", "permission").
			Values("guests", u1, auth.DEFAULT_PERMISSION.IntValue()).ToSql()

		CheckErr(err, "Failed to create insert user-group sql for guests")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user-group for guests: %v", s)

		u, _ = uuid.NewV4()
		u1 = u.String()
		s, v, err = statementbuilder.Squirrel.Insert("usergroup").
			Columns("name", "reference_id", "permission").
			Values("administrators", u1, auth.DEFAULT_PERMISSION.IntValue()).ToSql()
		CheckErr(err, "Failed to create insert user-group sql for administrators")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user-group sql for administrators")

		u, _ = uuid.NewV4()
		u1 = u.String()
		s, v, err = statementbuilder.Squirrel.Insert("usergroup").
			Columns("name", "reference_id", "permission").
			Values("users", u1, auth.DEFAULT_PERMISSION).ToSql()
		CheckErr(err, "Failed to create insert user-group sql for administrators")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user-group sql for administrators")

		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"reference_id": u1}).ToSql()
		CheckErr(err, "Failed to create select usergroup sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
		u, _ = uuid.NewV4()
		refIf := u.String()
		s, v, err = statementbuilder.Squirrel.Insert("user_account_user_account_id_has_usergroup_usergroup_id").
			Columns("user_account_id", "usergroup_id", "permission", "reference_id").
			Values(userId, userGroupId, auth.DEFAULT_PERMISSION, refIf).ToSql()
		CheckErr(err, "Failed to create insert user has usergroup sql ")
		_, err = tx.Exec(s, v...)
		CheckErr(err, "Failed to insert user has usergroup")

		//tx.Exec("update user set user_id = ?, usergroup_id = ?", userId, userGroupId)
		//tx.Exec("update usergroup set user_id = ?, usergroup_id = ?", userId, userGroupId)
	} else if userCount < 2 {
		systemHasNoAdmin = true
		s, v, err := statementbuilder.Squirrel.Select("id").From("user_account").OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {

		s, v, err := statementbuilder.Squirrel.Select("id").From("user_account").Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = tx.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = tx.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}

	defaultWorldPermission := auth.DEFAULT_PERMISSION

	if systemHasNoAdmin {
		defaultWorldPermission = auth.NewPermission(auth.CRUD|auth.Execute, auth.CRUD|auth.Execute, auth.CRUD|auth.Execute)
	}

	st := simpletable.New()
	st.Header = &simpletable.Header{
		[]*simpletable.Cell{
			{
				Text: "TableName",
			},
			{
				Text: "Is Top Level",
			},
			{
				Text: "Is Hidden",
			},
			{
				Text: "Table Exists",
			},
		},
	}

	stBody := &simpletable.Body{
		Cells: make([][]*simpletable.Cell, 0),
	}
	for _, table := range initConfig.Tables {
		u, _ := uuid.NewV4()

		refId := u.String()
		schema, err := json.Marshal(table)

		var cou int
		s, v, err := statementbuilder.Squirrel.Select("count(*)").From("world").Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
		tx.QueryRowx(s, v...).Scan(&cou)

		stBody.Cells = append(stBody.Cells, []*simpletable.Cell{
			{
				Text: table.TableName,
			},
			{
				Text: fmt.Sprintf("%v", table.IsTopLevel),
			},
			{
				Text: fmt.Sprintf("%v", table.IsHidden),
			},
			{
				Text: fmt.Sprintf("%v", cou),
			},
		})

		if cou > 0 {

			//s, v, err = statementbuilder.Squirrel.Select("default_permission").From("world").Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
			//CheckErr(err, "Failed to create select default permission sql")
			//log.Infof("Update table data [%v] == IsTopLevel[%v], IsHidden[%v]", table.TableName, table.IsTopLevel, table.IsHidden)

			s, v, err = statementbuilder.Squirrel.Update("world").
				Set("world_schema_json", string(schema)).
				Set("is_top_level", table.IsTopLevel).
				Set("is_hidden", table.IsHidden).
				Set("default_order", table.DefaultOrder).
				Where(squirrel.Eq{"table_name": table.TableName}).ToSql()
			CheckErr(err, "Failed to create update default permission sql")

			_, err := tx.Exec(s, v...)
			CheckErr(err, fmt.Sprintf("Failed to update json schema for table [%v]: %v", table.TableName, err))

		} else {

			if table.Permission == 0 {
				table.Permission = defaultWorldPermission.IntValue()
			}
			if table.DefaultPermission == 0 {
				table.DefaultPermission = defaultWorldPermission.IntValue()
			}

			log.Infof("Insert table data (IsTopLevel[%v], IsHidden[%v]) [%v]", table.IsTopLevel, table.IsHidden, table.TableName)

			s, v, err = statementbuilder.Squirrel.Insert("world").
				Columns("table_name", "world_schema_json", "permission", "reference_id", "default_permission", "user_account_id", "is_top_level", "is_hidden", "default_order").
				Values(table.TableName, string(schema), table.Permission, refId, table.DefaultPermission, userId, table.IsTopLevel, table.IsHidden, table.DefaultOrder).ToSql()
			_, err = tx.Exec(s, v...)
			CheckErr(err, "Failed to insert into world table about "+table.TableName)
			//initConfig.Tables[i].DefaultPermission = defaultWorldPermission

		}

	}
	st.Body = stBody
	st.Print()

	s, v, err = statementbuilder.Squirrel.Select("world_schema_json", "permission", "default_permission", "is_top_level", "is_hidden").
		From("world").
		ToSql()

	CheckErr(err, "Failed to create query for scan world table")

	res, err := tx.Queryx(s, v...)
	CheckErr(err, "Failed to scan world tables")
	if err != nil {
		return
	}

	defer res.Close()

	tables := make([]TableInfo, 0)
	for res.Next() {
		var tabInfo TableInfo
		var tableSchema []byte
		var permission, defaultPermission int64
		var isTopLevel, isHidden bool
		err = res.Scan(&tableSchema, &permission, &defaultPermission, &isTopLevel, &isHidden)
		CheckErr(err, "Failed to scan table info")
		err = json.Unmarshal(tableSchema, &tabInfo)
		CheckErr(err, "Failed to convert json to table schema")
		tabInfo.Permission = permission
		tabInfo.DefaultPermission = defaultPermission
		tabInfo.IsTopLevel = isTopLevel
		tabInfo.IsHidden = isHidden
		tables = append(tables, tabInfo)
	}
	initConfig.Tables = tables

}
