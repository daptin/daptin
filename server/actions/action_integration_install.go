package actions

import (
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

/*
*

	Become administrator of daptin action implementation
*/
type integrationInstallationPerformer struct {
	cruds            map[string]*resource.DbResource
	integration      resource.Integration
	router           *openapi3.T
	commandMap       map[string]*openapi3.Operation
	pathMap          map[string]string
	methodMap        map[string]string
	encryptionSecret []byte
}

// Name of the action
func (d *integrationInstallationPerformer) Name() string {
	return "integration.install"
}

// Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *integrationInstallationPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	referenceId := daptinid.InterfaceToDIR(inFieldMap["reference_id"])
	integration, _, err := d.cruds["integration"].GetSingleRowByReferenceIdWithTransaction("integration", referenceId, nil, transaction)

	spec, ok := integration["specification"]
	if !ok || spec == "" {
		return nil, nil, []error{errors.New("no specification present")}
	}

	specBytes := []byte(spec.(string))

	authSpec, ok := integration["authentication_specification"].(string)

	decryptedSpec, err := resource.Decrypt(d.encryptionSecret, authSpec)

	authDataMap := make(map[string]interface{})

	err = json.Unmarshal([]byte(decryptedSpec), &authDataMap)
	if err != nil {
		return nil, nil, []error{errors.New(fmt.Sprintf("failed to parse auth specification: %v", err))}
	}

	if integration["specification_format"] == "yaml" {

		specBytes, err = yaml.YAMLToJSON(specBytes)

		if err != nil {
			log.Errorf("Failed to convert yaml to json for integration: %v", err)
			return nil, nil, []error{err}
		}

	}

	var router *openapi3.T

	if integration["specification_language"] == "openapiv2" {

		openapiv2Spec := openapi2.T{}

		err := json.Unmarshal(specBytes, &openapiv2Spec)

		if err != nil {
			log.Errorf("Failed to unmarshal as openapiv2: %v", err)
			return nil, nil, []error{err}
		}

		router, err = openapi2conv.ToV3(&openapiv2Spec)

		if err != nil {
			log.Errorf("Failed to convert to openapi v3 spec: %v", err)
			return nil, nil, []error{err}
		}

	}

	if err != nil {
		return nil, nil, []error{err}
	}

	if router == nil {

		router, err = openapi3.NewLoader().LoadFromData(specBytes)
	}

	commandMap := make(map[string]*openapi3.Operation)
	pathMap := make(map[string]string)
	methodMap := make(map[string]string)
	for path, pathItem := range router.Paths {
		for method, command := range pathItem.Operations() {
			log.Printf("Register action [%v] at [%v]", command.OperationID, integration["name"])
			commandMap[command.OperationID] = command
			pathMap[command.OperationID] = path
			methodMap[command.OperationID] = method
		}
	}

	actions := make([]actionresponse.Action, 0)

	host := router.Servers[0].URL

	globalAttrs := make(map[string]string)

	for name, securityRef := range router.Components.SecuritySchemes {

		if authDataMap[name] != nil {
			continue
		}

		switch securityRef.Value.In {
		case "header":
			globalAttrs[name] = "~" + name
		case "query":
			globalAttrs[name] = "~" + name
		case "path":
			globalAttrs[name] = "~" + name
		}
	}

	for commandId, command := range commandMap {

		path := pathMap[commandId]

		params, err := GetParametersNames(host + path)
		if err != nil {
			log.Errorf("Failed to get parameter names from [%v] == %v", host+path, err)
			return nil, nil, []error{err}
		}
		cols := make([]api2go.ColumnInfo, 0)

		attrs := map[string]interface{}{}

		for key, val := range globalAttrs {
			attrs[key] = val
		}

		for _, param := range params {
			if authDataMap[param] != nil {
				continue
			}

			cols = append(cols, api2go.ColumnInfo{
				Name:       param,
				ColumnName: param,
				ColumnType: "label",
				DataType:   "varchar(100)",
			})
			attrs[param] = "~" + param
		}

		for _, param := range command.Parameters {
			if authDataMap[param.Value.Name] != nil {
				continue
			}
			cols = append(cols, api2go.ColumnInfo{
				Name:       param.Value.Name,
				ColumnName: param.Value.Name,
				ColumnType: "label",
				DataType:   "varchar(100)",
			})
			attrs[param.Value.Name] = "~" + param.Value.Name

		}

		if command.RequestBody != nil && command.RequestBody.Value != nil {

			contents := command.RequestBody.Value.Content

			jsonMedia := contents.Get("application/json")

			if jsonMedia != nil {
				bodyParameterNames, err := GetBodyParameterNames(ModeRequest, "", jsonMedia.Schema.Value)

				if err != nil {
					log.Errorf("Failed to get parameter names from body [%v] == %v", host+path, err)
					return nil, nil, []error{err}
				}

				for _, param := range bodyParameterNames {
					if authDataMap[param] != nil {
						continue
					}
					cols = append(cols, api2go.ColumnInfo{
						Name:       param,
						ColumnName: param,
						ColumnType: "label",
						DataType:   "varchar(100)",
					})
					attrs[param] = "~" + param
				}
			}
		}

		integrationName := integration["name"].(string)
		action := actionresponse.Action{}
		action.Name = commandId
		action.Label = flect.Humanize(commandId)
		action.OnType = "integration"
		action.InFields = cols
		action.InstanceOptional = true

		action.OutFields = []actionresponse.Outcome{
			{
				Type:       integrationName,
				Method:     commandId,
				Attributes: attrs,
			},
		}

		actions = append(actions, action)

	}

	err = resource.UpdateActionTable(&resource.CmsConfig{
		Actions: actions,
	}, transaction)

	return nil, []actionresponse.ActionResponse{}, []error{err}
}

// Create a new action performer for becoming administrator action
func NewIntegrationInstallationPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	encryptionSecret, err := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		log.Errorf("Failed to get encryption secret from config store: %v", err)
	}
	handler := integrationInstallationPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
	}

	return &handler, nil

}

func GetBodyParameterNames(mode Mode, name string, schema *openapi3.Schema) ([]string, error) {

	switch {
	case schema.Type == "boolean":
		return []string{}, nil
	case schema.Type == "number", schema.Type == "integer":
		return []string{}, nil
	case schema.Type == "string":
		return []string{}, nil
	case schema.Type == "array", schema.Items != nil:
		var names []string

		if schema.Items != nil && schema.Items.Value != nil {

			name, err := GetBodyParameterNames(mode, name, schema.Items.Value)

			if err != nil {
				return nil, err
			}
			names = append(names, name...)

		}

		return names, nil
	case schema.Type == "object", len(schema.Properties) > 0:
		var names []string

		for k, v := range schema.Properties {
			if excludeFromMode(mode, v.Value) {
				continue
			}

			names = append(names, k)

			name, err := GetBodyParameterNames(mode, k, v.Value)

			if err != nil {

				log.Errorf("can't get example for '%s' == %v", k, err)
			} else {
				names = append(names, name...)
			}
		}

		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Value != nil {
			addl := schema.AdditionalProperties.Value

			if !excludeFromMode(mode, addl) {
				name, err := GetBodyParameterNames(mode, "", addl)
				if err != nil {
					return nil, fmt.Errorf("can't get example for additional properties")
				} else {
					names = append(names, name...)
				}
			}
		}

		return names, nil
	}

	return nil, errors.New("not a valid schema")
}
