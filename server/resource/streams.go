package resource

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
)

// StreamProcess handles the Read operations, and applies transformations on the data the create a new view
type StreamProcessor struct {
	cruds    map[string]*DbResource
	contract StreamContract
}

// Stream contract defines column mappings and transformations. Also includes the query params which are to be used in the first place
type StreamContract struct {
	StreamName      string
	RootEntityName  string
	Columns         []api2go.ColumnInfo
	Relations       []api2go.TableRelation
	Transformations []Transformation
	QueryParams     map[string][]string
}

// A Transformation is the representation of column data changing its values according to the attribute map
type Transformation struct {
	Operation  string
	Attributes map[string]interface{}
}

// Get the contract
func (dr *StreamProcessor) GetContract() StreamContract {
	return dr.contract
}

// Get the contract
func (dr *StreamProcessor) GetName() string {
	return dr.contract.StreamName
}

// FindOne implementation in accordance with JSONAPI
// FindOne is not implemented for streams
func (dr *StreamProcessor) FindOne(ID string, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")

}

// Create implementation in accordance with JSONAPI
// Create is not implemented for streams
func (dr *StreamProcessor) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")

}

// Delete implementation in accordance with JSONAPI
// Delete is not implemented for streams
func (dr *StreamProcessor) Delete(id string, req api2go.Request) (api2go.Responder, error) {

	return nil, fmt.Errorf("not implemented")
}

// Update implementation in accordance with JSONAPI
// Update is not implemented for streams
func (dr *StreamProcessor) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	return nil, fmt.Errorf("not implemented")
}

// FindAll implementation in accordance with JSONAPI
// FindAll does the initial query to the database and applites the transformation contract on the result rows
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
			var indexes interface{}
			indexes, ok := transformation.Attributes["Columns"].([]string)
			if !ok {
				indexes = makeIndexArray(transformation.Attributes["Columns"].([]interface{}))
			}
			df = df.Select(indexes)
		case "rename":
			oldName := transformation.Attributes["OldName"].(string)
			newName := transformation.Attributes["NewName"].(string)
			df = df.Rename(newName, oldName)

		case "duplicate":
			oldName := transformation.Attributes["ColumnName"].(string)
			newName := transformation.Attributes["NewColumnName"].(string)

			newVals := make([]interface{}, 0)

			for _, row := range items {
				row[newName] = row[oldName]
				newVals = append(newVals, row[oldName])
			}

			df = df.Mutate(
				series.New(newVals, series.String, newName),
			)
		case "drop":
			var indexes interface{}
			indexes, ok := transformation.Attributes["Columns"].([]string)
			if !ok {
				indexes = makeIndexArray(transformation.Attributes["Columns"].([]interface{}))
			}
			df = df.Drop(indexes)

		case "filter":

			colName, ok := transformation.Attributes["ColumnName"]

			if !ok {
				continue
			}

			colnNameString, ok := colName.(string)

			if !ok || colnNameString == "" {
				continue
			}

			comparator, ok := transformation.Attributes["Comparator"]

			if !ok {
				continue
			}
			comparatorString, ok := comparator.(string)
			if !ok {
				continue
			}
			comparatorStringVal := series.Comparator(comparatorString)

			value := transformation.Attributes["Value"]

			filter := dataframe.F{
				Colname:    colnNameString,
				Comparator: comparatorStringVal,
				Comparando: value,
			}

			df = df.Filter(filter)

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

func makeIndexArray(indexes []interface{}) interface{} {

	if len(indexes) == 0 {
		return []string{}
	}

	switch indexes[0].(type) {
	case []int:
		retArr := make([][]int, 0)
		for _, v := range indexes {
			retArr = append(retArr, v.([]int))
		}
		return retArr
	case int:
		retArr := make([]int, 0)
		for _, v := range indexes {
			retArr = append(retArr, v.(int))
		}
		return retArr
	case []bool:
		retArr := make([][]bool, 0)
		for _, v := range indexes {
			retArr = append(retArr, v.([]bool))
		}
		return retArr
	case string:
		retArr := make([]string, 0)
		for _, v := range indexes {
			retArr = append(retArr, v.(string))
		}
		return retArr
	case []string:
		retArr := make([][]string, 0)
		for _, v := range indexes {
			retArr = append(retArr, v.([]string))
		}
		return retArr
	}
	return []string{}

}

// Creates a new stream processor which will apply the given contract
func NewStreamProcessor(stream StreamContract, cruds map[string]*DbResource) *StreamProcessor {
	return &StreamProcessor{
		cruds:    cruds,
		contract: stream,
	}
}
