package resource

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"

	"fmt"
	"github.com/daptin/daptin/server/statementbuilder"
	"net/http"
)

// Delete an object
// Possible Responder status codes are:
// - 200 OK: Deletion was a success, returns meta information, currently not implemented! Do not use this
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Deletion was successful, return nothing

func (dbResource *DbResource) DeleteWithoutFilters(id daptinid.DaptinReferenceId, req api2go.Request, transaction *sqlx.Tx) error {

	data, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.model.GetTableName(), id, transaction)
	if err != nil {
		return err
	}
	apiModel := api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(), nil, 0, nil, data)

	//user := req.PlainRequest.Context().Value("user")
	//sessionUser := &auth.SessionUser{}

	//if user != nil {
	//	sessionUser = user.(*auth.SessionUser)
	//
	//}
	//isAdmin := IsAdminWithTransaction(sessionUser, transaction)

	m := dbResource.model
	//log.Printf("Get all resource type: %v\n", m)

	if !EndsWithCheck(apiModel.GetTableName(), "_audit") && dbResource.tableInfo.IsAuditEnabled {
		auditModel := apiModel.GetAuditModel()
		log.Printf("Object [%v][%v] has been changed, trying to audit in %v", apiModel.GetTableName(), apiModel.GetID(), auditModel.GetTableName())
		if auditModel.GetTableName() != "" {
			//auditModel.Data["deleted_at"] = time.Now()
			creator, ok := dbResource.Cruds[auditModel.GetTableName()]
			if !ok {
				log.Errorf("No creator for audit type: %v", auditModel.GetTableName())
			} else {
				ur, _ := url.Parse("/" + auditModel.GetTableName())

				pr := &http.Request{
					Method: "POST",
					URL:    ur,
				}
				pr = pr.WithContext(req.PlainRequest.Context())
				createRequest := api2go.Request{
					PlainRequest: pr,
				}
				_, err := creator.CreateWithTransaction(auditModel, createRequest, transaction)
				if err != nil {
					log.Errorf("[66] Failed to create audit entry: %v", err)
				} else {
					log.Printf("[%v][%v] Created audit record", auditModel.GetTableName(), apiModel.GetID())
					//log.Printf("ReferenceId for change: %v", resp.Result())
				}
			}
		}
	}

	parentId := data["id"].(int64)
	//parentReferenceId := daptinid.InterfaceToDIR(data["reference_id"])

	for _, column := range dbResource.model.GetColumns() {
		if column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {

			cloudStoreData, err := dbResource.GetCloudStoreByNameWithTransaction(column.ForeignKeyData.Namespace, transaction)
			if err != nil {
				log.Errorf("Failed to load cloud store information %v: %v", column.ForeignKeyData.Namespace, err)
				continue
			}

			deleteFileActionPerformer := ActionHandlerMap["cloudstore.file.delete"]

			fileListJson, ok := data[column.ColumnName].([]map[string]interface{})
			if !ok {
				log.Warnf("[92] Unknown content in cloud store column [%s][%s] => %v", dbResource.model.GetName(), column.ColumnName, data[column.ColumnName])
				continue
			}
			log.Infof("[95] Delete attached file on column [%s] from disk: %v", column.Name, fileListJson)
			for _, fileItem := range fileListJson {

				outcome := actionresponse.Outcome{}
				actionParameters := map[string]interface{}{
					"credential_name": cloudStoreData.CredentialName,
					"store_provider":  cloudStoreData.StoreProvider,
					"store_type":      cloudStoreData.StoreType,
					"name":            cloudStoreData.Name,
					"path":            fileItem["path"].(string) + "/" + fileItem["name"].(string),
					"root_path":       cloudStoreData.RootPath + "/" + column.ForeignKeyData.KeyName,
				}
				_, _, errList := deleteFileActionPerformer.DoAction(outcome, actionParameters, transaction)
				if len(errList) > 0 {
					log.Errorf("[108] Failed to delete file: %v", errList)
				}

				columnAssetCache, ok := dbResource.AssetFolderCache[dbResource.tableInfo.TableName][column.ColumnName]
				if ok {
					err = columnAssetCache.DeleteFileByName(fileItem["path"].(string) + string(os.PathSeparator) + fileItem["name"].(string))
					CheckErr(err, "[114] Failed to delete file from local asset cache: %v", column.ColumnName)
				}

			}
		}
	}

	//for _, rel := range dbResource.model.GetRelations() {
	//
	//	if EndsWithCheck(rel.GetSubject(), "_audit") || EndsWithCheck(rel.GetObject(), "_audit") {
	//		continue
	//	}
	//
	//	if rel.GetSubject() == dbResource.model.GetTableName() {
	//
	//		switch rel.Relation {
	//		case "has_one":
	//			break
	//		case "belongs_to":
	//			break
	//		case "has_many":
	//			joinTableName := rel.GetJoinTableName()
	//			//columnName := rel.GetSubjectName()
	//
	//			joinIdQuery, vals, err := statementbuilder.Squirrel.
	//				Select("reference_id", rel.GetObjectName()).Prepared(true).
	//				From(joinTableName).Where(goqu.Ex{rel.GetSubjectName(): parentId}).ToSQL()
	//			CheckErr(err, "Failed to create query for getting join ids")
	//
	//			if err == nil {
	//
	//				stmt1, err := transaction.Preparex(joinIdQuery)
	//				if err != nil {
	//					log.Errorf("[139] failed to prepare statment: %v", err)
	//				}
	//				defer func(stmt1 *sqlx.Stmt) {
	//					err := stmt1.Close()
	//					if err != nil {
	//						log.Errorf("failed to close prepared statement: %v", err)
	//					}
	//				}(stmt1)
	//
	//				res, err := stmt1.Queryx(vals...)
	//				CheckErr(err, "Failed to query for join ids")
	//				if err == nil {
	//
	//					ids := map[daptinid.DaptinReferenceId]int64{}
	//					for res.Next() {
	//						var relationReferenceId daptinid.DaptinReferenceId
	//						var objectReferenceId int64
	//						err = res.Scan(&relationReferenceId, &objectReferenceId)
	//						CheckErr(err, "Failed to scan relation reference id")
	//						ids[relationReferenceId] = objectReferenceId
	//					}
	//					err = res.Close()
	//					err = stmt1.Close()
	//
	//					canDeleteAllIds := true
	//
	//					for _, objectId := range ids {
	//
	//						otherObjectPermission := dbResource.GetObjectPermissionByIdWithTransaction(rel.GetObject(), objectId, transaction)
	//
	//						if !isAdmin && !otherObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
	//							log.Tracef("CanRefer Denied by otherObjectPermission: %v on user %v", otherObjectPermission, sessionUser)
	//							canDeleteAllIds = false
	//							break
	//						}
	//
	//					}
	//
	//					if canDeleteAllIds {
	//						for relationId := range ids {
	//							//log.Printf("Delete relation with [%v][%v]", joinTableName, relationId)
	//							err = dbResource.Cruds[joinTableName].DeleteWithoutFilters(relationId, req, transaction)
	//							CheckErr(err, "Failed to delete join 1")
	//						}
	//					} else {
	//						return fmt.Errorf("the request object could not be detached from all relations "+
	//							"since the user is unauthorized to remove one or more relations [%s][%s]", joinTableName, rel.GetObject())
	//					}
	//
	//				}
	//
	//			}
	//
	//			break
	//		case "has_many_and_belongs_to_many":
	//			joinTableName := rel.GetJoinTableName()
	//			//columnName := rel.GetSubjectName()
	//
	//			joinIdQuery, vals, err := statementbuilder.Squirrel.
	//				Select("reference_id").Prepared(true).
	//				From(joinTableName).Where(goqu.Ex{rel.GetSubjectName(): parentId}).ToSQL()
	//			CheckErr(err, "Failed to create query for getting join ids")
	//
	//			if err == nil {
	//
	//				stmt1, err := transaction.Preparex(joinIdQuery)
	//				if err != nil {
	//					log.Errorf("[201] failed to prepare statment: %v", err)
	//				}
	//				defer func(stmt1 *sqlx.Stmt) {
	//					err := stmt1.Close()
	//					if err != nil {
	//						log.Errorf("failed to close prepared statement: %v", err)
	//					}
	//				}(stmt1)
	//
	//				res, err := stmt1.Queryx(vals...)
	//				CheckErr(err, "Failed to query for join ids")
	//				defer func(r *sqlx.Rows) {
	//					err := r.Close()
	//					if err != nil {
	//						log.Errorf("[229] failed to close rows after value scan in defer")
	//						return
	//					}
	//				}(res)
	//				if err == nil {
	//
	//					var ids []daptinid.DaptinReferenceId
	//					for res.Next() {
	//						var s daptinid.DaptinReferenceId
	//						err = res.Scan(&s)
	//						CheckErr(err, "[239] Failed to scan value in delete")
	//						ids = append(ids, s)
	//					}
	//
	//					res.Close()
	//					stmt1.Close()
	//					canDeleteAllIds := true
	//
	//					for _, id := range ids {
	//
	//						otherObjectPermission := GetObjectPermissionByReferenceIdWithTransaction(rel.GetObject(), id, transaction)
	//
	//						if !isAdmin && !otherObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups, dbResource.AdministratorGroupId) {
	//							canDeleteAllIds = false
	//							break
	//						}
	//
	//					}
	//
	//					if canDeleteAllIds {
	//						for _, id := range ids {
	//							//log.Printf("Delete relation with [%v][%v]", joinTableName, id)
	//							err = dbResource.Cruds[joinTableName].DeleteWithoutFilters(id, req, transaction)
	//							CheckErr(err, "Failed to delete join 2")
	//						}
	//					} else {
	//						return fmt.Errorf("[265] the request object could not be detached "+
	//							"from all relations[%s] since the user is unauthorized", joinTableName)
	//					}
	//
	//				}
	//
	//			}
	//
	//		}
	//
	//	} else {
	//
	//		// i am the object
	//		// delete subject
	//
	//		switch rel.Relation {
	//		case "has_one":
	//
	//			ur, _ := url.Parse("/" + rel.GetSubject() + "/relationships/" + rel.GetObjectName())
	//			pr := &http.Request{
	//				Method: "GET",
	//				URL:    ur,
	//			}
	//
	//			pr = pr.WithContext(req.PlainRequest.Context())
	//
	//			subRequest := api2go.Request{
	//				PlainRequest: pr,
	//				QueryParams: map[string][]string{
	//					rel.GetObject() + "_id":  {parentReferenceId.String()},
	//					rel.GetObject() + "Name": {rel.GetSubjectName()},
	//				},
	//			}
	//
	//			_, allRelatedObjects, err := dbResource.Cruds[rel.GetSubject()].PaginatedFindAllWithTransaction(subRequest, transaction)
	//			CheckErr(err, "Failed to get related objects of: %v", rel.GetSubject())
	//			if err != nil {
	//				return err
	//			}
	//
	//			results := allRelatedObjects.Result().([]api2go.Api2GoModel)
	//			for _, result := range results {
	//				_, err := dbResource.Cruds[rel.GetSubject()].DeleteWithTransaction(
	//					daptinid.DaptinReferenceId(uuid.MustParse(result.GetID())), req, transaction)
	//				CheckErr(err, "Failed to delete related object before deleting parent")
	//			}
	//
	//			break
	//		case "belongs_to":
	//
	//			ur, _ := url.Parse("/" + rel.GetSubject() + "/relationships/" + rel.GetObjectName())
	//
	//			pr := &http.Request{
	//				URL:    ur,
	//				Method: "GET",
	//			}
	//
	//			pr = pr.WithContext(req.PlainRequest.Context())
	//
	//			subRequest := api2go.Request{
	//				PlainRequest: pr,
	//				QueryParams: map[string][]string{
	//					rel.GetObject() + "_id":  {parentReferenceId.String()},
	//					rel.GetObject() + "Name": {rel.GetSubjectName()},
	//				},
	//			}
	//
	//			_, allRelatedObjects, err := dbResource.Cruds[rel.GetSubject()].PaginatedFindAllWithTransaction(subRequest, transaction)
	//			CheckErr(err, "Failed to get related objects of: %v", rel.GetSubject())
	//
	//			results := allRelatedObjects.Result().([]api2go.Api2GoModel)
	//			for _, result := range results {
	//				_, err := dbResource.Cruds[rel.GetSubject()].DeleteWithTransaction(daptinid.DaptinReferenceId(uuid.MustParse(result.GetID())), req, transaction)
	//				CheckErr(err, "Failed to delete related object before deleting parent")
	//			}
	//
	//			break
	//		case "has_many":
	//			joinTableName := rel.GetJoinTableName()
	//
	//			//columnName := rel.GetSubjectName()
	//
	//			joinIdQuery, vals, err := statementbuilder.Squirrel.
	//				Select("reference_id").Prepared(true).
	//				From(joinTableName).Where(goqu.Ex{rel.GetObjectName(): parentId}).ToSQL()
	//			CheckErr(err, "Failed to create query for getting join ids")
	//
	//			if err == nil {
	//
	//				stmt1, err := transaction.Preparex(joinIdQuery)
	//				if err != nil {
	//					log.Errorf("[322] failed to prepare statment: %v", err)
	//				}
	//				defer func(stmt1 *sqlx.Stmt) {
	//					err := stmt1.Close()
	//					if err != nil {
	//						log.Errorf("failed to close prepared statement: %v", err)
	//					}
	//				}(stmt1)
	//
	//				res, err := stmt1.Queryx(vals...)
	//				CheckErr(err, "Failed to query for join ids")
	//				defer func(res *sqlx.Rows) {
	//					err := res.Close()
	//					if err != nil {
	//						log.Errorf("failed to close result after value scan in defer")
	//					}
	//				}(res)
	//				if err == nil {
	//
	//					var ids []daptinid.DaptinReferenceId
	//					for res.Next() {
	//						var s daptinid.DaptinReferenceId
	//						res.Scan(&s)
	//						ids = append(ids, s)
	//					}
	//					res.Close()
	//					stmt1.Close()
	//
	//					for _, id := range ids {
	//						_, err = dbResource.Cruds[joinTableName].DeleteWithTransaction(id, req, transaction)
	//						CheckErr(err, "Failed to delete join 3")
	//					}
	//
	//				}
	//
	//			}
	//
	//			break
	//		case "has_many_and_belongs_to_many":
	//			joinTableName := rel.GetJoinTableName()
	//			//columnName := rel.GetSubjectName()
	//
	//			ur, _ := url.Parse("/" + rel.GetSubject() + "/relationships/" + rel.GetObjectName())
	//			pr := &http.Request{
	//				URL:    ur,
	//				Method: "GET",
	//			}
	//
	//			pr = pr.WithContext(req.PlainRequest.Context())
	//
	//			iii, _ := uuid.FromBytes(id[:])
	//			subRequest := api2go.Request{
	//				PlainRequest: pr,
	//				QueryParams: map[string][]string{
	//					rel.GetObject() + "_id":  {iii.String()},
	//					rel.GetObject() + "Name": {rel.GetSubjectName()},
	//				},
	//			}
	//
	//			_, allRelatedObjects, err := dbResource.Cruds[joinTableName].PaginatedFindAllWithTransaction(subRequest, transaction)
	//			CheckErr(err, "Failed to get related objects of: %v", joinTableName)
	//
	//			results := allRelatedObjects.Result().([]api2go.Api2GoModel)
	//			for _, result := range results {
	//				_, err := dbResource.Cruds[joinTableName].DeleteWithTransaction(
	//					daptinid.DaptinReferenceId(uuid.MustParse(result.GetID())), req, transaction)
	//				CheckErr(err, "Failed to delete related object before deleting parent")
	//			}
	//
	//		}
	//
	//	}
	//
	//}

	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	if len(languagePreferences) > 0 {

		for _, lang := range languagePreferences {

			queryBuilder := statementbuilder.Squirrel.Delete(m.GetTableName() + "_i18n").
				Where(goqu.Ex{"translation_reference_id": parentId}).Prepared(true).
				Where(goqu.Ex{"language_id": lang})

			sql1, args, err := queryBuilder.ToSQL()
			if err != nil {
				log.Printf("Error: %v", err)
				return err
			}

			log.Printf("Delete Sql: %v\n", sql1)

			_, err = transaction.Exec(sql1, args...)

		}
	} else {

		queryBuilder := statementbuilder.Squirrel.
			Delete(m.GetTableName()).Prepared(true).Where(goqu.Ex{"reference_id": id[:]})

		sql1, args, err := queryBuilder.ToSQL()
		if err != nil {
			log.Printf("Error: %v", err)
			return err
		}

		log.Printf("Delete Sql: %v\n", sql1)

		_, err = transaction.Exec(sql1, args...)
		return err
	}

	return err

}

func (dbResource *DbResource) Delete(idString string, req api2go.Request) (api2go.Responder, error) {
	id := daptinid.DaptinReferenceId(uuid.MustParse(idString))

	transaction, err := dbResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [451]")
		return nil, err
	}

	log.Infof("Delete [%v][%v]", dbResource.model.GetTableName(), id)
	for _, bf := range dbResource.ms.BeforeDelete {
		//log.Printf("[Before][%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		r, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": id,
				"__type":       dbResource.model.GetName(),
			},
		}, transaction)
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			log.Errorf("Error from BeforeDelete[%v] middleware: %v", bf.String(), err)
			return nil, err
		}
		if r == nil || len(r) == 0 {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			return nil, fmt.Errorf("Cannot delete this object [%v][%v]", bf.String(), id)
		}
	}

	err = dbResource.DeleteWithoutFilters(daptinid.DaptinReferenceId(id), req, transaction)
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "Failed to rollback")
		return nil, err
	}

	for _, bf := range dbResource.ms.AfterDelete {
		//log.Printf("Invoke AfterDelete [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		_, err = bf.InterceptAfter(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": id,
			},
		}, transaction)
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			log.Errorf("Error from AfterDelete middleware: %v", err)
			return nil, err
		}
	}

	commitErr := transaction.Commit()
	CheckErr(commitErr, "Failed to commit")

	return NewResponse(nil, nil, 200, nil), commitErr
}

func (dbResource *DbResource) DeleteWithTransaction(id daptinid.DaptinReferenceId, req api2go.Request, transaction *sqlx.Tx) (api2go.Responder, error) {

	log.Printf("Delete [%v][%v]", dbResource.model.GetTableName(), id)
	for _, bf := range dbResource.ms.BeforeDelete {
		//log.Printf("[Before][%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		r, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": id,
				"__type":       dbResource.model.GetName(),
			},
		}, transaction)
		if err != nil {
			log.Errorf("Error from BeforeDelete[%v] middleware: %v", bf.String(), err)
			return nil, err
		}
		if r == nil || len(r) == 0 {
			return nil, fmt.Errorf("Cannot delete this object [%v][%v]", bf.String(), id)
		}
	}

	err := dbResource.DeleteWithoutFilters(id, req, transaction)
	if err != nil {
		return nil, err
	}

	for _, bf := range dbResource.ms.AfterDelete {
		//log.Printf("Invoke AfterDelete [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		_, err = bf.InterceptAfter(dbResource, &req, []map[string]interface{}{
			{
				"reference_id": id,
			},
		}, transaction)
		if err != nil {
			log.Errorf("Error from AfterDelete middleware: %v", err)
			return nil, err
		}
	}

	return NewResponse(nil, nil, 200, nil), nil
}
