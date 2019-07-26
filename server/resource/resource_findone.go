package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
	//"strings"
	log "github.com/sirupsen/logrus"
)

// FindOne returns an object by its ID
// Possible Responder success status code 200
func (dr *DbResource) FindOne(referenceId string, req api2go.Request) (api2go.Responder, error) {

	for _, bf := range dr.ms.BeforeFindOne {
		//log.Printf("Invoke BeforeFindOne [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		r, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{
			{
				"reference_id": referenceId,
				"__type":       dr.model.GetName(),
			},
		})
		if err != nil {
			log.Printf("Error from BeforeFindOne[%s][%s] middleware: %v", bf.String(), dr.model.GetName(), err)
			return nil, err
		}
		if r == nil {
			return nil, errors.New("Cannot find this object")
		}
	}

	modelName := dr.model.GetName()
	log.Printf("Find [%s] by id [%s]", modelName, referenceId)
	//
	//if strings.Index(modelName, "_has_") > 0 {
	//	parts := strings.Split(modelName, "_has_")
	//}

	data, include, err := dr.GetSingleRowByReferenceId(modelName, referenceId)

	//log.Printf("Single row result: %v", data)
	for _, bf := range dr.ms.AfterFindOne {
		//log.Printf("Invoke AfterFindOne [%v][%v] on FindAll Request", bf.String(), modelName)

		results, err := bf.InterceptAfter(dr, &req, []map[string]interface{}{data})
		if len(results) != 0 {
			data = results[0]
		} else {
			log.Printf("No results after executing: [%v]", bf.String())
			data = nil
		}
		if err != nil {
			log.Printf("Error from AfterFindOne middleware: %v", err)
		}
		include, err = bf.InterceptAfter(dr, &req, include)

		if err != nil {
			log.Printf("Error from AfterFindOne middleware: %v", err)
		}
	}

	delete(data, "id")
	//delete(data, "deleted_at")

	infos := dr.model.GetColumns()
	var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
	a.Data = data

	for _, inc := range include {
		p, ok := inc["permission"].(int64)
		if !ok {
			log.Printf("Failed to convert [%v] to permission: %v", inc["permission"], inc["__type"])
			p = 0
		}
		incType := inc["__type"].(string)
		if BeginsWith(incType, "image.") {
			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, nil, 0, nil, inc))
		} else {
			a.Includes = append(a.Includes, api2go.NewApi2GoModelWithData(incType, dr.Cruds[incType].model.GetColumns(), int64(p), dr.Cruds[incType].model.GetRelations(), inc))
		}
	}

	return NewResponse(nil, a, 200, nil), err
}
