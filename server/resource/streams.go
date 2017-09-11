package resource

import (
	"github.com/artpar/api2go"
	"github.com/kniren/gota/dataframe"
	"fmt"
)

type StreamProcessor struct {
	cruds    map[string]*DbResource
	contract StreamContract
}

type StreamContract struct {
	StreamName      string
	RootEntityName  string
	Columns         []api2go.ColumnInfo
	Relations       []api2go.TableRelation
	Transformations []Transformation
	QueryParams     map[string][]string
}

type Transformation struct {
	Operation  string
	Attributes map[string]interface{}
}

func (dr *StreamProcessor) GetContract() StreamContract {
	return dr.contract
}

func (dr *StreamProcessor) FindOne(ID string, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")

}

func (dr *StreamProcessor) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")

}

func (dr *StreamProcessor) Delete(id string, req api2go.Request) (api2go.Responder, error) {

	return nil, fmt.Errorf("not implemented")
}

func (dr *StreamProcessor) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")
}

func (dr *StreamProcessor) PaginatedFindAll(req api2go.Request) (totalCount uint, response api2go.Responder, err error) {

	contract := dr.contract

	for key, val := range contract.QueryParams {
		req.QueryParams[key] = val
	}

	totalCount, responder1, err := dr.cruds[dr.contract.RootEntityName].PaginatedFindAll(req)
	if err != nil {
		return 0, nil, err
	}
	responder := responder1.(api2go.Response)
	if err != nil {
		return totalCount, responder, err
	}

	listOfResults := responder.Result().([]*api2go.Api2GoModel)

	items := make([]map[string]interface{}, 0)

	for _, item := range listOfResults {
		items = append(items, item.Data)
	}

	df := dataframe.LoadMaps(items)

	for _, transformation := range contract.Transformations {

		switch transformation.Operation {
		case "select":
			indexes := transformation.Attributes["columns"].([]string)
			df = df.Select(indexes)
		case "rename":
			oldName := transformation.Attributes["oldName"].(string)
			newName := transformation.Attributes["newName"].(string)
			df = df.Rename(newName, oldName)
		}

	}

	newList := make([]*api2go.Api2GoModel, 0)

	maps := df.Maps()

	for _, row := range maps {
		model := api2go.NewApi2GoModelWithData(contract.StreamName, contract.Columns, 0, nil, row)
		newList = append(newList, model)
	}

	newResponder := NewResponse(nil, newList, responder.StatusCode(), &responder.Pagination)
	return totalCount, newResponder, nil
}

func NewStreamProcessor(stream StreamContract, cruds map[string]*DbResource) *StreamProcessor {
	return &StreamProcessor{
		cruds:    cruds,
		contract: stream,
	}
}
