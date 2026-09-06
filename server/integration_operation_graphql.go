package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/graphql-go/graphql"
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
)

func addIntegrationOperationGraphQLMutations(mutationFields graphql.Fields, actionResponseType *graphql.Object, resources map[string]*resource.DbResource) {
	worldCrud := resources["world"]
	if worldCrud == nil {
		log.Warnf("Skipping integration GraphQL mutations: world resource is not available")
		return
	}
	transaction, err := worldCrud.Connection().Beginx()
	if err != nil {
		log.Warnf("Failed to load integrations for GraphQL generation: %v", err)
		return
	}
	defer transaction.Rollback()

	integrations, err := worldCrud.GetActiveIntegrations(transaction)
	if err != nil {
		log.Warnf("Failed to list integrations for GraphQL generation: %v", err)
		return
	}

	for _, integration := range integrations {
		if !integration.Enable {
			log.Debugf("Skipping disabled integration in GraphQL schema provider=[%s]", integration.Name)
			continue
		}
		router, err := loadGraphQLIntegrationOpenAPIRouter(integration)
		if err != nil {
			log.Errorf("Failed to load OpenAPI spec for GraphQL integration [%s]: %v", integration.Name, err)
			continue
		}
		seen := make(map[string]bool)
		registered := 0
		for _, pathItem := range router.Paths {
			if pathItem == nil {
				log.Warnf("Skipping nil path item in GraphQL integration schema provider=[%s]", integration.Name)
				continue
			}
			for _, operation := range pathItem.Operations() {
				if operation == nil {
					log.Warnf("Skipping nil operation in GraphQL integration schema provider=[%s]", integration.Name)
					continue
				}
				operationID := operation.OperationID
				if operationID == "" || seen[operationID] {
					if operationID == "" {
						log.Debugf("Skipping integration GraphQL operation without operationId provider=[%s]", integration.Name)
					} else {
						log.Warnf("Skipping duplicate integration GraphQL operationId provider=[%s] operation=[%s]", integration.Name, operationID)
					}
					continue
				}
				seen[operationID] = true
				actionName, err := resource.IntegrationOperationActionName(integration.Name, operationID)
				if err != nil {
					log.Errorf("Invalid integration action identity provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
					continue
				}
				action, err := worldCrud.GetActionByName("integration", actionName, transaction)
				if err != nil {
					log.Warnf("Skipping GraphQL integration operation without installed action provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
					continue
				}
				fieldName := "execute" + strcase.ToCamel(operationID) + "On" + strcase.ToCamel(integration.Name)
				args, inputArgs := actionGraphQLArguments(action)
				mutationFields[fieldName] = &graphql.Field{
					Type:        graphql.NewList(actionResponseType),
					Description: firstGraphQLNonEmpty(operation.Description, operation.Summary, fmt.Sprintf("Execute %s on %s", operationID, integration.Name)),
					Args:        args,
					Resolve: func(integration resource.Integration, operationID string, inputArgs []graphqlActionArg) func(params graphql.ResolveParams) (interface{}, error) {
						return func(params graphql.ResolveParams) (interface{}, error) {
							input := actionGraphQLInput(params.Args, inputArgs)

							log.Tracef("GraphQL integration operation execution started provider=[%s] operation=[%s]", integration.Name, operationID)
							requestURL, err := url.Parse("/integration/" + url.PathEscape(integration.Name) + "/" + url.PathEscape(operationID))
							if err != nil {
								return nil, err
							}
							request := (&http.Request{Method: "EXECUTE", URL: requestURL}).WithContext(params.Context)
							responses, err := executeIntegrationOperationAction(resources, integration.Name, operationID, input, request)
							if err != nil {
								log.Warnf("GraphQL integration operation execution failed provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
								return nil, err
							}
							log.Infof("GraphQL integration operation completed provider=[%s] operation=[%s] responses=%d", integration.Name, operationID, len(responses))
							return responses, nil
						}
					}(integration, operationID, inputArgs),
				}
				registered++
			}
		}
		log.Infof("Registered GraphQL integration operation mutations provider=[%s] count=%d", integration.Name, registered)
	}
}

func loadGraphQLIntegrationOpenAPIRouter(integration resource.Integration) (router *openapi3.T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Errorf("Recovered panic while loading GraphQL integration OpenAPI spec provider=[%s]: %v", integration.Name, recovered)
			router = nil
			err = fmt.Errorf("failed to load integration OpenAPI spec: %v", recovered)
		}
	}()
	specBytes := []byte(integration.Specification)
	if integration.SpecificationFormat == "yaml" {
		specBytes, err = ghodssyaml.YAMLToJSON(specBytes)
		if err != nil {
			return nil, err
		}
	}
	if integration.SpecificationLanguage == "openapiv2" {
		openapiv2Spec := openapi2.T{}
		if err := json.Unmarshal(specBytes, &openapiv2Spec); err != nil {
			return nil, err
		}
		return openapi2conv.ToV3(&openapiv2Spec)
	}
	router, err = openapi3.NewLoader().LoadFromData(specBytes)
	if err != nil {
		return nil, err
	}
	err = openapi3.NewLoader().ResolveRefsIn(router, nil)
	return router, err
}

func safeGraphQLName(name string) string {
	builder := strings.Builder{}
	for _, char := range name {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('_')
		}
	}
	name = builder.String()
	if name == "" {
		return "input"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "_" + name
	}
	return name
}

func firstGraphQLNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
