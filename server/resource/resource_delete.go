package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	//sq "gopkg.in/Masterminds/squirrel.v1"
	//"reflect"
	//"github.com/satori/go.uuid"
	"gopkg.in/Masterminds/squirrel.v1"
	"github.com/pkg/errors"
)

// Delete an object
// Possible Responder status codes are:
// - 200 OK: Deletion was a success, returns meta information, currently not implemented! Do not use this
// - 202 Accepted: Processing is delayed, return nothing
// - 204 No Content: Deletion was successful, return nothing

func (dr *DbResource) Delete(id string, req api2go.Request) (api2go.Responder, error) {

	for _, bf := range dr.ms.BeforeDelete {
		//log.Infof("[Before][%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		r, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{
			{
				"reference_id": id,
				"__type":       dr.model.GetName(),
			},
		})
		if err != nil {
			log.Errorf("Error from BeforeDelete middleware: %v", err)
			return nil, err
		}
		if r == nil || len(r) == 0 {
			return nil, errors.New("Cannot delete this object")
		}
	}

	itemBeingDeleted, err := dr.FindOne(id, req)
	if err != nil {
		return nil, err
	}

	data := itemBeingDeleted.Result().(*api2go.Api2GoModel)

	m := dr.model
	//log.Infof("Get all resource type: %v\n", m)

	auditModel := data.GetAuditModel()
	log.Infof("Object [%v]%v has been changed, trying to audit in %v", data.GetTableName(), data.GetID(), auditModel.GetTableName())
	if auditModel.GetTableName() != "" {
		//auditModel.Data["deleted_at"] = time.Now()
		creator, ok := dr.cruds[auditModel.GetTableName()]
		if !ok {
			log.Errorf("No creator for audit type: %v", auditModel.GetTableName())
		} else {
			_, err := creator.Create(auditModel, req)
			if err != nil {
				log.Errorf("Failed to create audit entry: %v", err)
			} else {
				log.Infof("[%v][%v] Created audit record", auditModel.GetTableName(), data.GetID())
				//log.Infof("ReferenceId for change: %v", resp.Result())
			}
		}
	}

	//queryBuilder := squirrel.Update(m.GetTableName()).Set("deleted_at", time.Now()).Where(squirrel.Eq{"reference_id": id})
	queryBuilder := squirrel.Delete(m.GetTableName()).Where(squirrel.Eq{"reference_id": id})

	sql1, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Infof("Error: %v", err)
		return nil, err
	}

	log.Infof("Sql: %v\n", sql1)

	_, err = dr.db.Exec(sql1, args...)
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
