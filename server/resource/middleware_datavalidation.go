package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	"strings"
	//"github.com/go-playground/validator"
	"fmt"
	"github.com/artpar/conform"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/universal-translator"
	"gopkg.in/go-playground/validator.v9"
)

type DataValidationMiddleware struct {
	config       *CmsConfig
	tableInfoMap map[string]TableInfo
	translator   ut.Translator
}

func (dvm DataValidationMiddleware) String() string {
	return "DataValidationMiddleware"
}

func (dvm *DataValidationMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	return results, nil

}

func (dvm *DataValidationMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}) ([]map[string]interface{}, error) {

	var err error

	switch strings.ToLower(req.PlainRequest.Method) {
	case "get":
		fallthrough
	case "delete":
		break
	case "post":
		fallthrough
	case "update":
		fallthrough
	case "patch":
		validations := dvm.tableInfoMap[dr.model.GetName()].Validations
		conformations := dvm.tableInfoMap[dr.model.GetName()].Conformations

		//log.Infof("We have %d objects to validate", len(objects))

		for i, obj := range objects {

			for _, validate := range validations {

				colValue, ok := obj[validate.ColumnName]
				if !ok {
					continue
				}
				errs := ValidatorInstance.VarWithValue(colValue, obj, validate.Tags)

				if errs != nil {
					validationErrors, ok := errs.(validator.ValidationErrors)
					if !ok {
						return nil, api2go.NewHTTPError(errs, "failed to validate incoming data", 400)
					}
					httpErr := api2go.NewHTTPError(errs, strings.Replace(validationErrors[0].Translate(dvm.translator), "for ''", fmt.Sprintf("'%v'", validate.ColumnName), 1), 400)
					return nil, httpErr
				}

			}

			for _, conformation := range conformations {
				colValue, ok := obj[conformation.ColumnName]
				if !ok {
					continue
				}
				colValueString, ok := colValue.(string)

				if !ok {
					continue
				}
				transformedValue := conform.TransformString(colValueString, conformation.Tags)
				objects[i][conformation.ColumnName] = transformedValue
			}

		}

		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return objects, err

}

func NewDataValidationMiddleware(cmsConfig *CmsConfig, cruds *map[string]*DbResource) DatabaseRequestInterceptor {

	tableInfoMap := make(map[string]TableInfo)

	for _, tabInfo := range cmsConfig.Tables {
		tableInfoMap[tabInfo.TableName] = tabInfo
	}

	e := en.New()
	uni := ut.New(e, e)
	en1, _ := uni.GetTranslator("en") // or fallback if fails to find 'en'

	return &DataValidationMiddleware{
		config:       cmsConfig,
		tableInfoMap: tableInfoMap,
		translator:   en1,
	}
}
