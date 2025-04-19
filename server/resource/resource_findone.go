package resource

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"strings"
	"time"

	//"strings"
	log "github.com/sirupsen/logrus"
)

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dbResource *DbResource) FindOne(referenceIdString string, req api2go.Request) (api2go.Responder, error) {

	var referenceId daptinid.DaptinReferenceId

	if string(referenceIdString) == "mine" && dbResource.tableInfo.TableName == "user_account" {
		//log.Debugf("Request for mine")
		sessionUser := req.PlainRequest.Context().Value("user")
		if sessionUser != nil {
			authUser := sessionUser.(*auth.SessionUser)
			//log.Debugf("Overrider reference id mine with %v", authUser.UserReferenceId)
			referenceId = authUser.UserReferenceId
		}
	} else {
		if referenceIdString != "" {
			parse, err := uuid.Parse(referenceIdString)
			if err != nil {
				return nil, err
			}
			referenceId = daptinid.DaptinReferenceId(parse)
		}
	}

	transaction, err := dbResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [34]")
		return nil, err
	}

	for _, bf := range dbResource.ms.BeforeFindOne {
		//log.Debugf("Invoke BeforeFindOne [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		start := time.Now()
		r, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": referenceId,
				"__type":       dbResource.model.GetName(),
			},
		}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindOne BeforeFilter[%v]: %v", bf.String(), duration)

		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			log.Errorf("Error from BeforeFindOne[%s][%s] middleware: %v", bf.String(), dbResource.model.GetName(), err)
			return nil, err
		}
		if r == nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			return nil, errors.New("Cannot find this object")
		}
	}

	modelName := dbResource.model.GetName()
	//log.Debugf("Find [%s] by id [%s]", modelName, referenceId)

	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	includedRelations := make(map[string]bool, 0)
	if len(req.QueryParams["included_relations"]) > 0 {
		//included := req.QueryParams["included_relations"][0]
		//includedRelationsList := strings.Split(included, ",")
		for _, incl := range req.QueryParams["included_relations"] {
			includedRelations[incl] = true
		}

	} else {
		includedRelations = nil
	}

	start := time.Now()
	data, include, err := dbResource.GetSingleRowByReferenceIdWithTransaction(modelName, referenceId, includedRelations, transaction)
	log.Tracef("Completed FindOne GetSingleRowByReferenceIdWithTransaction")

	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "Failed to rollback")
		return nil, err
	}
	duration := time.Since(start)
	log.Tracef("[TIMING] FindOne: %v", duration)

	if OlricCache != nil {
		cacheKey := fmt.Sprintf("riti-%v-%v", modelName, referenceId)
		_ = OlricCache.Put(context.Background(), cacheKey, data["id"], olric.EX(5*time.Minute), olric.NX())
		cacheKey2 := fmt.Sprintf("itr-%v-%v", modelName, data["id"])
		_ = OlricCache.Put(context.Background(), cacheKey2, data["reference_id"], olric.EX(5*time.Minute), olric.NX())
	}

	if len(languagePreferences) > 0 {
		for _, lang := range languagePreferences {
			data_i18n_id, err := dbResource.GetIdByWhereClause(modelName+"_i18n", transaction, goqu.Ex{
				"translation_reference_id": data["id"],
				"language_id":              lang,
			})
			if err == nil && len(data_i18n_id) > 0 {
				for _, data_i18n := range data_i18n_id {
					translatedObj, err := dbResource.GetIdToObjectWithTransaction(modelName+"_i18n", data_i18n, transaction)
					CheckErr(err, "Failed to fetch translated object for [%v][%v][%v]", modelName, lang, data["id"])
					if err != nil {
						rollbackErr := transaction.Rollback()
						CheckErr(rollbackErr, "Failed to rollback")
						return nil, err
					}
					if translatedObj != nil {
						for colName, valName := range translatedObj {
							if IsStandardColumn(colName) {
								continue
							}
							if valName == nil {
								continue
							}
							data[colName] = valName
						}
					}
				}
				break
			} else {
				CheckErr(err, "No translated rows for [%v][%v][%v]", modelName, referenceId, lang)
			}
		}
	}

	//log.Tracef("Single row result: %v", data)
	for _, bf := range dbResource.ms.AfterFindOne {
		log.Tracef("Invoke AfterFindOne [%v][%v] on FindAll Request", bf.String(), modelName)

		start := time.Now()
		results, err := bf.InterceptAfter(dbResource, &req, []map[string]interface{}{data}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindOne AfterFilter [145] [%v]: %v", bf.String(), duration)

		if len(results) != 0 {
			data = results[0]
		} else {
			//log.Debugf("No results after executing: [%v]", bf.String())
			data = nil
		}
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			log.Errorf("Error from AfterFindOne middleware: %v", err)
			return nil, err
		}
		include, err = bf.InterceptAfter(dbResource, &req, include, transaction)
		log.Tracef("Completed all AfterFindOne includes")

		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			log.Errorf("Error from AfterFindOne middleware: %v", err)
			return nil, err
		}
	}
	log.Tracef("Completed all AfterFindOne middlewares")

	commitErr := transaction.Commit()
	CheckErr(commitErr, "failed to commit")

	infos := dbResource.model.GetColumns()
	var a = api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(), infos,
		dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), data)

	for _, inc := range include {
		incType := inc["__type"].(string)

		if strings.Index(incType, ".") > -1 {
			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, nil, 0, nil, inc))
		} else {
			p, ok := inc["permission"].(int64)
			if !ok {
				log.Warnf("Failed to convert [%v] to permission: %v", inc["permission"], inc["__type"])
				p = 0
			}

			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, dbResource.Cruds[incType].model.GetColumns(), int64(p), dbResource.Cruds[incType].model.GetRelations(), inc))
		}

	}
	log.Tracef("Completed FindOne [194]")
	return NewResponse(nil, a, 200, nil), commitErr
}

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dbResource *DbResource) FindOneWithTransaction(referenceId daptinid.DaptinReferenceId, req api2go.Request, transaction *sqlx.Tx) (api2go.Responder, error) {

	if string(referenceId[0:4]) == "mine" && dbResource.tableInfo.TableName == "user_account" {
		//log.Debugf("Request for mine")
		sessionUser := req.PlainRequest.Context().Value("user")
		if sessionUser != nil {
			authUser := sessionUser.(*auth.SessionUser)
			//log.Debugf("Overrider reference id mine with %v", authUser.UserReferenceId)
			referenceId = authUser.UserReferenceId
		}
	}

	for _, bf := range dbResource.ms.BeforeFindOne {
		//log.Debugf("Invoke BeforeFindOne [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		start := time.Now()
		r, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": referenceId,
				"__type":       dbResource.model.GetName(),
			},
		}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindOne BeforeFilter[%v]: %v", bf.String(), duration)

		if err != nil {
			log.Errorf("Error from BeforeFindOne[%s][%s] middleware: %v", bf.String(), dbResource.model.GetName(), err)
			return nil, err
		}
		if r == nil {
			return nil, errors.New("Cannot find this object")
		}
	}

	modelName := dbResource.model.GetName()
	//log.Debugf("Find [%s] by id [%s]", modelName, referenceId)

	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	includedRelations := make(map[string]bool, 0)
	if len(req.QueryParams["included_relations"]) > 0 {
		//included := req.QueryParams["included_relations"][0]
		//includedRelationsList := strings.Split(included, ",")
		for _, incl := range req.QueryParams["included_relations"] {
			includedRelations[incl] = true
		}

	} else {
		includedRelations = nil
	}

	start := time.Now()
	log.Tracef("FindOneWithTransaction GetSingleRowByReferenceIdWithTransaction")
	data, include, err := dbResource.GetSingleRowByReferenceIdWithTransaction(modelName, referenceId, includedRelations, transaction)
	log.Tracef("Completed FindOneWithTransaction GetSingleRowByReferenceIdWithTransaction")
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)
	log.Tracef("[TIMING] FindOne: %v", duration)
	if OlricCache != nil {
		cacheKey := fmt.Sprintf("riti-%v-%v", modelName, referenceId)
		_ = OlricCache.Put(context.Background(), cacheKey, data["id"], olric.EX(5*time.Minute), olric.NX())
		cacheKey2 := fmt.Sprintf("itr-%v-%v", modelName, data["id"])
		_ = OlricCache.Put(context.Background(), cacheKey2, data["reference_id"], olric.EX(5*time.Minute), olric.NX())
	}

	if len(languagePreferences) > 0 {
		for _, lang := range languagePreferences {
			data_i18n_id, err := dbResource.GetIdByWhereClause(modelName+"_i18n", transaction, goqu.Ex{
				"translation_reference_id": data["id"],
				"language_id":              lang,
			})
			if err == nil && len(data_i18n_id) > 0 {
				for _, data_i18n := range data_i18n_id {
					translatedObj, err := dbResource.GetIdToObjectWithTransaction(modelName+"_i18n", data_i18n, transaction)
					CheckErr(err, "Failed to fetch translated object for [%v][%v][%v]", modelName, lang, data["id"])
					if err != nil {
						return nil, err
					}
					if translatedObj != nil {
						for colName, valName := range translatedObj {
							if IsStandardColumn(colName) {
								continue
							}
							if valName == nil {
								continue
							}
							data[colName] = valName
						}
					}
				}
				break
			} else {
				CheckErr(err, "No translated rows for [%v][%v][%v]", modelName, referenceId, lang)
			}
		}
	}

	//log.Tracef("Single row result: %v", data)
	for _, bf := range dbResource.ms.AfterFindOne {
		log.Tracef("Invoke AfterFindOne [%v][%v] on FindAll Request", bf.String(), modelName)

		start := time.Now()
		results, err := bf.InterceptAfter(dbResource, &req, []map[string]interface{}{data}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindOne AfterFilter [309] [%v]: %v", bf.String(), duration)

		if len(results) != 0 {
			data = results[0]
		} else {
			//log.Debugf("No results after executing: [%v]", bf.String())
			data = nil
		}
		if err != nil {
			log.Errorf("Error from AfterFindOne middleware: %v", err)
			return nil, err
		}
		include, err = bf.InterceptAfter(dbResource, &req, include, transaction)

		if err != nil {
			log.Errorf("Error from AfterFindOne middleware: %v", err)
		}
	}

	delete(data, "id")

	infos := dbResource.model.GetColumns()
	var a = api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(),
		infos, dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), data)

	for _, inc := range include {
		incType := inc["__type"].(string)

		if strings.Index(incType, ".") > -1 {
			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, nil, 0, nil, inc))
		} else {
			p, ok := inc["permission"].(int64)
			if !ok {
				log.Warnf("Failed to convert [%v] to permission: %v", inc["permission"], inc["__type"])
				p = 0
			}

			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, dbResource.Cruds[incType].model.GetColumns(), int64(p), dbResource.Cruds[incType].model.GetRelations(), inc))
		}

	}

	return NewResponse(nil, a, 200, nil), nil
}
