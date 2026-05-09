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
	"strconv"
	"strings"
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
	configStore      *resource.ConfigStore
}

// Name of the action
func (d *integrationInstallationPerformer) Name() string {
	return "integration.install"
}

// Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *integrationInstallationPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	referenceId := daptinid.InterfaceToDIR(inFieldMap["reference_id"])
	integrationCrud := d.cruds["integration"]
	if integrationCrud == nil {
		log.Errorf("install_integration failed: integration resource is not available")
		return nil, nil, []error{errors.New("integration resource is not available")}
	}
	integration, _, err := integrationCrud.GetSingleRowByReferenceIdWithTransaction("integration", referenceId, nil, transaction)
	if err != nil {
		log.Warnf("install_integration failed loading integration reference_id=[%s]: %v", referenceId.String(), err)
		return nil, nil, []error{err}
	}

	spec, ok := integration["specification"]
	if !ok || spec == "" {
		log.Warnf("install_integration failed: no specification present reference_id=[%s]", referenceId.String())
		return nil, nil, []error{errors.New("no specification present")}
	}

	specString, ok := spec.(string)
	if !ok {
		log.Warnf("install_integration failed: specification has invalid type [%T] reference_id=[%s]", spec, referenceId.String())
		return nil, nil, []error{fmt.Errorf("specification must be a string, got %T", spec)}
	}
	specBytes := []byte(specString)

	authSpec, ok := integration["authentication_specification"].(string)
	if !ok {
		log.Warnf("install_integration failed: authentication_specification has invalid type [%T] reference_id=[%s]", integration["authentication_specification"], referenceId.String())
		return nil, nil, []error{fmt.Errorf("authentication_specification must be a string, got %T", integration["authentication_specification"])}
	}

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
		if pathItem == nil {
			log.Warnf("install_integration skipping nil path item provider=[%v] path=[%s]", integration["name"], path)
			continue
		}
		for method, command := range pathItem.Operations() {
			if command == nil {
				log.Warnf("install_integration skipping nil operation provider=[%v] method=[%s] path=[%s]", integration["name"], method, path)
				continue
			}
			operationID := command.OperationID
			if len(operationID) == 0 {
				operationID = method + " " + path
			}
			if _, exists := commandMap[operationID]; exists {
				log.Warnf("install_integration rejected duplicate operationId provider=[%v] operation=[%s]", integration["name"], operationID)
				return nil, nil, []error{fmt.Errorf("duplicate operationId [%s] in integration [%s]", operationID, integration["name"])}
			}
			log.Debugf("install_integration discovered operation provider=[%v] operation=[%s] method=[%s] path=[%s]", integration["name"], operationID, method, path)
			commandMap[operationID] = command
			pathMap[operationID] = path
			methodMap[operationID] = method
		}
	}

	actions := make([]actionresponse.Action, 0)

	host := router.Servers[0].URL

	globalAttrs := make(map[string]string)
	authType := strings.ToLower(fmt.Sprintf("%v", integration["authentication_type"]))
	authInputNames := make(map[string]bool)

	for name, securityRef := range router.Components.SecuritySchemes {
		if integrationAuthUsesSecurityScheme(authType, securityRef.Value) {
			authInputNames[name] = true
			if securityRef.Value.Name != "" {
				authInputNames[securityRef.Value.Name] = true
			}
			continue
		}

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

		switch authType {
		case "oauth2":
			cols = append(cols, api2go.ColumnInfo{
				Name:              "oauth_token_id",
				ColumnName:        "oauth_token_id",
				ColumnType:        "hidden",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "OAuth token reference id to use for this integration execution.",
			})
			attrs["oauth_token_id"] = "~oauth_token_id"
		case "custom_credentials":
			cols = append(cols, api2go.ColumnInfo{
				Name:              "credential_id",
				ColumnName:        "credential_id",
				ColumnType:        "hidden",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "Credential reference id to use for this integration execution.",
			})
			attrs["credential_id"] = "~credential_id"
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
			if authInputNames[param.Value.Name] && (param.Value.In == "header" || param.Value.In == "query" || param.Value.In == "cookie") {
				continue
			}
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

		integrationName, ok := stringField(integration, "name")
		if !ok {
			log.Warnf("install_integration failed: name is missing or invalid reference_id=[%s]", referenceId.String())
			return nil, nil, []error{errors.New("integration name must be a string")}
		}
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
	if err != nil {
		log.Errorf("install_integration failed updating action table provider=[%v]: %v", integration["name"], err)
		return nil, []actionresponse.ActionResponse{}, []error{err}
	}

	err = d.refreshInstalledIntegrationPerformer(integration, transaction)
	if err != nil {
		log.Errorf("install_integration failed refreshing runtime mappings provider=[%v]: %v", integration["name"], err)
	} else {
		log.Infof("install_integration completed provider=[%v] operations=%d", integration["name"], len(actions))
	}
	return nil, []actionresponse.ActionResponse{}, []error{err}
}

func (d *integrationInstallationPerformer) refreshInstalledIntegrationPerformer(integration map[string]interface{}, transaction *sqlx.Tx) error {
	enableValue := int64(1)
	if rawEnable, ok := integration["enable"]; ok {
		switch val := rawEnable.(type) {
		case int64:
			enableValue = val
		case int:
			enableValue = int64(val)
		case string:
			parsed, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return err
			}
			enableValue = parsed
		}
	}

	integrationRuntime, err := integrationFromRow(integration, enableValue == 1)
	if err != nil {
		log.Warnf("Failed to refresh integration operation mappings from invalid row: %v", err)
		return err
	}
	if !integrationRuntime.Enable {
		log.Infof("Removing disabled integration operation mappings provider=[%s]", integrationRuntime.Name)
		delete(resource.ActionHandlerMap, integrationRuntime.Name)
		for _, crud := range d.cruds {
			if crud.ActionHandlerMap != nil {
				delete(crud.ActionHandlerMap, integrationRuntime.Name)
			}
		}
		return nil
	}

	performer, err := NewIntegrationActionPerformer(integrationRuntime, nil, d.cruds, d.configStore, transaction)
	if err != nil {
		log.Errorf("Failed to create refreshed integration action performer provider=[%s]: %v", integrationRuntime.Name, err)
		return err
	}
	resource.ActionHandlerMap[performer.Name()] = performer
	for _, crud := range d.cruds {
		if crud.ActionHandlerMap == nil {
			crud.ActionHandlerMap = make(map[string]actionresponse.ActionPerformerInterface)
		}
		crud.ActionHandlerMap[performer.Name()] = performer
	}
	log.Printf("Refreshed integration operation mappings for [%s] without restart", integrationRuntime.Name)
	log.Infof("Refreshed integration operation mappings provider=[%s] crud_maps=%d", integrationRuntime.Name, len(d.cruds))
	return nil
}

func integrationFromRow(row map[string]interface{}, enable bool) (resource.Integration, error) {
	name, ok := stringField(row, "name")
	if !ok {
		return resource.Integration{}, errors.New("integration name must be a string")
	}
	specLanguage, ok := stringField(row, "specification_language")
	if !ok {
		return resource.Integration{}, errors.New("integration specification_language must be a string")
	}
	specFormat, ok := stringField(row, "specification_format")
	if !ok {
		return resource.Integration{}, errors.New("integration specification_format must be a string")
	}
	spec, ok := stringField(row, "specification")
	if !ok {
		return resource.Integration{}, errors.New("integration specification must be a string")
	}
	authType, ok := stringField(row, "authentication_type")
	if !ok {
		return resource.Integration{}, errors.New("integration authentication_type must be a string")
	}
	authSpec, ok := stringField(row, "authentication_specification")
	if !ok {
		return resource.Integration{}, errors.New("integration authentication_specification must be a string")
	}
	return resource.Integration{
		Name:                        name,
		SpecificationLanguage:       specLanguage,
		SpecificationFormat:         specFormat,
		Specification:               spec,
		AuthenticationType:          authType,
		AuthenticationSpecification: authSpec,
		Enable:                      enable,
	}, nil
}

func stringField(row map[string]interface{}, key string) (string, bool) {
	value, ok := row[key]
	if !ok || value == nil {
		return "", false
	}
	strValue, ok := value.(string)
	return strValue, ok
}

func integrationAuthUsesSecurityScheme(authType string, securityScheme *openapi3.SecurityScheme) bool {
	if securityScheme == nil {
		return false
	}
	switch authType {
	case "oauth2":
		return securityScheme.Type == "oauth2"
	case "custom_credentials":
		return securityScheme.Type == "http" || securityScheme.Type == "apiKey"
	default:
		return false
	}
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
		configStore:      configStore,
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
