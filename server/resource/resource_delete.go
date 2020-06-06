package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"

	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"net/http"
)

// Delete an object
// Possible Responder status codes are:
// - 200 OK: Deletion was a success, returns meta information, currently not implemented! Do not use this
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Deletion was successful, return nothing

func (dr *DbResource) DeleteWithoutFilters(id string, req api2go.Request) error {

	data, err := dr.GetReferenceIdToObject(dr.model.GetTableName(), id)
	if err != nil {
		return err
	}
	apiModel := api2go.NewApi2GoModelWithData(dr.model.GetTableName(), nil, 0, nil, data)

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)

	}
	adminId := dr.GetAdminReferenceId()
	isAdmin := adminId != "" && adminId == sessionUser.UserReferenceId

	m := dr.model
	//log.Infof("Get all resource type: %v\n", m)

	if !EndsWithCheck(apiModel.GetTableName(), "_audit") && dr.tableInfo.IsAuditEnabled {
		auditModel := apiModel.GetAuditModel()
		log.Infof("Object [%v][%v] has been changed, trying to audit in %v", apiModel.GetTableName(), apiModel.GetID(), auditModel.GetTableName())
		if auditModel.GetTableName() != "" {
			//auditModel.Data["deleted_at"] = time.Now()
			creator, ok := dr.Cruds[auditModel.GetTableName()]
			if !ok {
				log.Errorf("No creator for audit type: %v", auditModel.GetTableName())
			} else {
				pr := &http.Request{
					Method: "POST",
				}
				pr = pr.WithContext(req.PlainRequest.Context())
				createRequest := api2go.Request{
					PlainRequest: pr,
				}
				_, err := creator.Create(auditModel, createRequest)
				if err != nil {
					log.Errorf("Failed to create audit entry: %v", err)
				} else {
					log.Infof("[%v][%v] Created audit record", auditModel.GetTableName(), apiModel.GetID())
					//log.Infof("ReferenceId for change: %v", resp.Result())
				}
			}
		}
	}

	parentId := data["id"].(int64)
	parentReferenceId := data["reference_id"].(string)
	for _, rel := range dr.model.GetRelations() {

		if EndsWithCheck(rel.GetSubject(), "_audit") || EndsWithCheck(rel.GetObject(), "_audit") {
			continue
		}

		if rel.GetSubject() == dr.model.GetTableName() {

			switch rel.Relation {
			case "has_one":
				break
			case "belongs_to":
				break
			case "has_many":
				joinTableName := rel.GetJoinTableName()
				//columnName := rel.GetSubjectName()

				joinIdQuery, vals, err := statementbuilder.Squirrel.Select("reference_id", rel.GetObjectName()).From(joinTableName).Where(squirrel.Eq{rel.GetSubjectName(): parentId}).ToSql()
				CheckErr(err, "Failed to create query for getting join ids")

				if err == nil {

					res, err := dr.db.Queryx(joinIdQuery, vals...)
					CheckErr(err, "Failed to query for join ids")
					defer res.Close()
					if err == nil {

						ids := map[string]int64{}
						for res.Next() {
							var relationReferenceId string
							var objectReferenceId int64
							res.Scan(&relationReferenceId, &objectReferenceId)
							ids[relationReferenceId] = objectReferenceId
						}

						canDeleteAllIds := true

						for _, objectId := range ids {

							otherObjectPermission := dr.GetObjectPermissionById(rel.GetObject(), objectId)

							if !isAdmin || !otherObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
								canDeleteAllIds = false
								break
							}

						}

						if canDeleteAllIds {
							for relationId := range ids {
								log.Infof("Delete relation with [%v][%v]", joinTableName, relationId)
								err = dr.Cruds[joinTableName].DeleteWithoutFilters(relationId, req)
								CheckErr(err, "Failed to delete join 1")
							}
						} else {
							return fmt.Errorf("the request object could not be detached from all relations since the user is unauthorized")
						}

					}

				}

				break
			case "has_many_and_belongs_to_many":
				joinTableName := rel.GetJoinTableName()
				//columnName := rel.GetSubjectName()

				joinIdQuery, vals, err := statementbuilder.Squirrel.Select("reference_id").From(joinTableName).Where(squirrel.Eq{rel.GetSubjectName(): parentId}).ToSql()
				CheckErr(err, "Failed to create query for getting join ids")

				if err == nil {

					res, err := dr.db.Queryx(joinIdQuery, vals...)
					CheckErr(err, "Failed to query for join ids")
					defer res.Close()
					if err == nil {

						var ids []string
						for res.Next() {
							var s string
							res.Scan(&s)
							ids = append(ids, s)
						}

						canDeleteAllIds := true

						for _, id := range ids {

							otherObjectPermission := dr.GetObjectPermissionByReferenceId(rel.GetObject(), id)

							if !otherObjectPermission.CanRefer(sessionUser.UserReferenceId, sessionUser.Groups) {
								canDeleteAllIds = false
								break
							}

						}

						if canDeleteAllIds {
							for _, id := range ids {
								log.Infof("Delete relation with [%v][%v]", joinTableName, id)
								err = dr.Cruds[joinTableName].DeleteWithoutFilters(id, req)
								CheckErr(err, "Failed to delete join 2")
							}
						} else {
							return fmt.Errorf("the request object could not be detached from all relations since the user is unauthorized")
						}

					}

				}

			}

		} else {

			// i am the object
			// delete subject

			switch rel.Relation {
			case "has_one":

				pr := &http.Request{
					Method: "GET",
				}

				pr = pr.WithContext(req.PlainRequest.Context())

				subRequest := api2go.Request{
					PlainRequest: pr,
					QueryParams: map[string][]string{
						rel.GetObject() + "_id":  {parentReferenceId},
						rel.GetObject() + "Name": {rel.GetSubjectName()},
					},
				}

				_, allRelatedObjects, err := dr.Cruds[rel.GetSubject()].PaginatedFindAll(subRequest)
				CheckErr(err, "Failed to get related objects of: %v", rel.GetSubject())

				results := allRelatedObjects.Result().([]*api2go.Api2GoModel)
				for _, result := range results {
					_, err := dr.Cruds[rel.GetSubject()].Delete(result.GetID(), req)
					CheckErr(err, "Failed to delete related object before deleting parent")
				}

				break
			case "belongs_to":

				pr := &http.Request{
					Method: "GET",
				}

				pr = pr.WithContext(req.PlainRequest.Context())

				subRequest := api2go.Request{
					PlainRequest: pr,
					QueryParams: map[string][]string{
						rel.GetObject() + "_id":  {parentReferenceId},
						rel.GetObject() + "Name": {rel.GetSubjectName()},
					},
				}

				_, allRelatedObjects, err := dr.Cruds[rel.GetSubject()].PaginatedFindAll(subRequest)
				CheckErr(err, "Failed to get related objects of: %v", rel.GetSubject())

				results := allRelatedObjects.Result().([]*api2go.Api2GoModel)
				for _, result := range results {
					_, err := dr.Cruds[rel.GetSubject()].Delete(result.GetID(), req)
					CheckErr(err, "Failed to delete related object before deleting parent")
				}

				break
			case "has_many":
				joinTableName := rel.GetJoinTableName()

				//columnName := rel.GetSubjectName()

				joinIdQuery, vals, err := statementbuilder.Squirrel.Select("reference_id").From(joinTableName).Where(squirrel.Eq{rel.GetObjectName(): parentId}).ToSql()
				CheckErr(err, "Failed to create query for getting join ids")

				if err == nil {

					res, err := dr.db.Queryx(joinIdQuery, vals...)
					CheckErr(err, "Failed to query for join ids")
					defer res.Close()
					if err == nil {

						var ids []string
						for res.Next() {
							var s string
							res.Scan(&s)
							ids = append(ids, s)
						}

						for _, id := range ids {
							_, err = dr.Cruds[joinTableName].Delete(id, req)
							CheckErr(err, "Failed to delete join 3")
						}

					}

				}

				break
			case "has_many_and_belongs_to_many":
				joinTableName := rel.GetJoinTableName()
				//columnName := rel.GetSubjectName()

				pr := &http.Request{
					Method: "GET",
				}

				pr = pr.WithContext(req.PlainRequest.Context())

				subRequest := api2go.Request{
					PlainRequest: pr,
					QueryParams: map[string][]string{
						rel.GetObject() + "_id":  {id},
						rel.GetObject() + "Name": {rel.GetSubjectName()},
					},
				}

				_, allRelatedObjects, err := dr.Cruds[joinTableName].PaginatedFindAll(subRequest)
				CheckErr(err, "Failed to get related objects of: %v", joinTableName)

				results := allRelatedObjects.Result().([]*api2go.Api2GoModel)
				for _, result := range results {
					_, err := dr.Cruds[joinTableName].Delete(result.GetID(), req)
					CheckErr(err, "Failed to delete related object before deleting parent")
				}

			}

		}

	}

	languagePreferences := make([]string, 0)
	if dr.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	if len(languagePreferences) > 0 {

		for _, lang := range languagePreferences {

			queryBuilder := statementbuilder.Squirrel.Delete(m.GetTableName() + "_i18n").
				Where(squirrel.Eq{"translation_reference_id": parentId}).
				Where(squirrel.Eq{"language_id": lang})

			sql1, args, err := queryBuilder.ToSql()
			if err != nil {
				log.Infof("Error: %v", err)
				return err
			}

			log.Infof("Delete Sql: %v\n", sql1)

			_, err = dr.db.Exec(sql1, args...)

		}
	} else {

		queryBuilder := statementbuilder.Squirrel.Delete(m.GetTableName()).Where(squirrel.Eq{"reference_id": id})

		sql1, args, err := queryBuilder.ToSql()
		if err != nil {
			log.Infof("Error: %v", err)
			return err
		}

		log.Infof("Delete Sql: %v\n", sql1)

		_, err = dr.db.Exec(sql1, args...)
		return err
	}

	return err

}

func (dr *DbResource) Delete(id string, req api2go.Request) (api2go.Responder, error) {

	log.Infof("Delete [%v][%v]", dr.model.GetTableName(), id)
	for _, bf := range dr.ms.BeforeDelete {
		//log.Infof("[Before][%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		r, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{
			{
				"reference_id": id,
				"__type":       dr.model.GetName(),
			},
		})
		if err != nil {
			log.Errorf("Error from BeforeDelete[%v] middleware: %v", bf.String(), err)
			return nil, err
		}
		if r == nil || len(r) == 0 {
			return nil, fmt.Errorf("Cannot delete this object [%v][%v]", bf.String(), id)
		}
	}

	err := dr.DeleteWithoutFilters(id, req)
	if err != nil {
		return nil, err
	}

	for _, bf := range dr.ms.AfterDelete {
		//log.Infof("Invoke AfterDelete [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		_, err = bf.InterceptAfter(dr, &req, nil)
		if err != nil {
			log.Errorf("Error from AfterDelete middleware: %v", err)
		}
	}

	if err != nil {
		log.Infof("Error: %v", err)
		return nil, err
	}

	return NewResponse(nil, nil, 200, nil), nil
}
