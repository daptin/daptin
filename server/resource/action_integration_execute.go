package resource

import (
	"encoding/json"
	"errors"
	"github.com/artpar/api2go"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"github.com/imroc/req"
	"log"
	"regexp"
	"strings"
)

/**
  Become administrator of daptin action implementation
*/
type IntegrationActionPerformer struct {
	cruds       map[string]*DbResource
	integration Integration
	router      *openapi3.Swagger
	commandMap  map[string]*openapi3.Operation
	pathMap     map[string]string
	methodMap   map[string]string
}

// Name of the action
func (d *IntegrationActionPerformer) Name() string {
	return d.integration.Name
}

// Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *IntegrationActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	operation, ok := d.commandMap[request.Method]
	method := d.methodMap[request.Method]
	path, ok := d.pathMap[request.Method]
	pathItem := d.router.Paths.Find(path)

	if !ok || pathItem == nil {
		return nil, nil, []error{errors.New("no such method")}
	}

	r := req.New()

	authKeys := make(map[string]interface{})
	json.Unmarshal([]byte(d.integration.AuthenticationSpecification), &authKeys)

	for key, val := range authKeys {
		inFieldMap[key] = val
	}

	url := d.router.Servers[0].URL + path

	templateVar, err := regexp.Compile(`\{([^}]+)\}`)
	if err != nil {
		return nil, nil, []error{err}
	}

	matches := templateVar.FindAllStringSubmatch(url, -1)

	for _, matc := range matches {
		value := inFieldMap[matc[1]]
		url = strings.Replace(url, matc[0], value.(string), -1)
	}

	evaluateString(url, inFieldMap)
	var resp *req.Resp

	switch strings.ToLower(method) {
	case "post":
		resp, err = r.Post(url, inFieldMap)

	case "get":
		resp, err = r.Get(url)
	case "delete":
		resp, err = r.Delete(url)
	case "patch":
		resp, err = r.Patch(url)
	case "put":
		resp, err = r.Put(url)
	case "options":
		resp, err = r.Options(url)

	}

	var res map[string]interface{}
	resp.ToJSON(&res)
	responder := NewResponse(nil, res, resp.Response().StatusCode, nil)
	return responder, []ActionResponse{}, nil
}

// Create a new action performer for becoming administrator action
func NewIntegrationActionPerformer(integration Integration, initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	var err error
	jsonBytes := []byte(integration.Specification)

	if integration.SpecificationFormat == "yaml" {

		jsonBytes, err = yaml.YAMLToJSON(jsonBytes)

		if err != nil {
			return nil, err
		}

	}

	router, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(jsonBytes)

	if err != nil {
		return nil, err
	}

	commandMap := make(map[string]*openapi3.Operation)
	pathMap := make(map[string]string)
	methodMap := make(map[string]string)
	for path, pathItem := range router.Paths {
		for method, command := range pathItem.Operations() {
			log.Printf("Register action [%v] at [%v]", command.OperationID, integration.Name)
			commandMap[command.OperationID] = command
			pathMap[command.OperationID] = path
			methodMap[command.OperationID] = method
		}
	}

	handler := IntegrationActionPerformer{
		cruds:       cruds,
		integration: integration,
		router:      router,
		commandMap:  commandMap,
		pathMap:     pathMap,
		methodMap:   methodMap,
	}

	return &handler, nil

}
