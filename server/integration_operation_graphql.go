package server

import (
	"fmt"
	"strings"

	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/graphql-go/graphql"
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
)

type graphqlIntegrationArg struct {
	OriginalName string
	GraphQLName  string
}

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
				fieldName := "execute" + strcase.ToCamel(operationID) + "On" + strcase.ToCamel(integration.Name)
				args, inputArgs := integrationOperationGraphQLArgs(router, operation)
				mutationFields[fieldName] = &graphql.Field{
					Type:        graphql.NewList(actionResponseType),
					Description: firstGraphQLNonEmpty(operation.Description, operation.Summary, fmt.Sprintf("Execute %s on %s", operationID, integration.Name)),
					Args:        args,
					Resolve: func(integration resource.Integration, operationID string, inputArgs []graphqlIntegrationArg) func(params graphql.ResolveParams) (interface{}, error) {
						return func(params graphql.ResolveParams) (interface{}, error) {
							input := make(map[string]interface{})
							for _, arg := range inputArgs {
								if value, ok := params.Args[arg.GraphQLName]; ok {
									input[arg.OriginalName] = value
								}
							}
							if value, ok := params.Args["oauth_token_id"]; ok {
								input["oauth_token_id"] = value
							}
							if value, ok := params.Args["credential_id"]; ok {
								input["credential_id"] = value
							}

							user := params.Context.Value("user")
							sessionUser := graphQLSessionUserFromContextValue(user)
							requestSessionUser := *sessionUser
							input["sessionUser"] = sessionUser
							input["requestSessionUser"] = &requestSessionUser

							log.Tracef("GraphQL integration operation execution started provider=[%s] operation=[%s]", integration.Name, operationID)
							transaction, err := resources["world"].Connection().Beginx()
							if err != nil {
								log.Errorf("GraphQL integration operation transaction begin failed provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
								return nil, err
							}
							performer, ok := resource.GetActionHandler(resources["world"], integration.Name)
							if !ok || performer == nil {
								_ = transaction.Rollback()
								log.Warnf("GraphQL integration provider not found provider=[%s] operation=[%s]", integration.Name, operationID)
								return nil, fmt.Errorf("integration provider [%s] is not installed or enabled", integration.Name)
							}
							_, responses, errs := performer.DoAction(actionresponse.Outcome{
								Type:   integration.Name,
								Method: operationID,
							}, input, transaction)
							if len(errs) > 0 {
								_ = transaction.Rollback()
								log.Warnf("GraphQL integration operation execution failed provider=[%s] operation=[%s]: %v", integration.Name, operationID, errs[0])
								return nil, errs[0]
							}
							if err = transaction.Commit(); err != nil {
								log.Errorf("GraphQL integration operation transaction commit failed provider=[%s] operation=[%s]: %v", integration.Name, operationID, err)
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

func integrationOperationGraphQLArgs(router *openapi3.T, operation *openapi3.Operation) (graphql.FieldConfigArgument, []graphqlIntegrationArg) {
	args := graphql.FieldConfigArgument{
		"oauth_token_id": &graphql.ArgumentConfig{Type: graphql.String, Description: "OAuth token reference id to use for OAuth2-backed integrations."},
		"credential_id":  &graphql.ArgumentConfig{Type: graphql.String, Description: "Credential reference id to use for custom credential-backed integrations."},
	}
	inputArgs := make([]graphqlIntegrationArg, 0)
	addArg := func(originalName string, typ graphql.Type, required bool, description string) {
		graphQLName := safeGraphQLName(originalName)
		if _, exists := args[graphQLName]; exists {
			graphQLName = graphQLName + "_" + fmt.Sprintf("%d", len(args))
		}
		if required {
			typ = graphql.NewNonNull(typ)
		}
		args[graphQLName] = &graphql.ArgumentConfig{Type: typ, Description: description}
		inputArgs = append(inputArgs, graphqlIntegrationArg{OriginalName: originalName, GraphQLName: graphQLName})
	}

	for _, parameterRef := range operation.Parameters {
		if parameterRef == nil || parameterRef.Value == nil || isGraphQLIntegrationAuthParameter(router, parameterRef.Value) {
			continue
		}
		parameter := parameterRef.Value
		addArg(parameter.Name, graphQLTypeForOpenAPI3Schema(parameter.Schema), parameter.Required, parameter.Description)
	}

	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		bodySchema := graphQLRequestBodySchema(operation.RequestBody.Value)
		if bodySchema != nil && bodySchema.Value != nil && len(bodySchema.Value.Properties) > 0 {
			for name, property := range bodySchema.Value.Properties {
				description := ""
				if property != nil && property.Value != nil {
					description = property.Value.Description
				}
				addArg(name, graphQLTypeForOpenAPI3Schema(property), stringInSlice(name, bodySchema.Value.Required), description)
			}
		} else {
			addArg("input", graphql.String, operation.RequestBody.Value.Required, "JSON string for operation input. This fallback is used because the provider OpenAPI spec does not declare concrete input fields.")
		}
	}

	if len(inputArgs) == 0 {
		addArg("input", graphql.String, false, "JSON string for operation input. This fallback is used because the provider OpenAPI spec does not declare concrete input fields.")
	}
	return args, inputArgs
}

func graphQLRequestBodySchema(requestBody *openapi3.RequestBody) *openapi3.SchemaRef {
	for _, mediaType := range []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"} {
		if content := requestBody.Content.Get(mediaType); content != nil && content.Schema != nil {
			return content.Schema
		}
	}
	for _, content := range requestBody.Content {
		if content != nil && content.Schema != nil {
			return content.Schema
		}
	}
	return nil
}

func graphQLTypeForOpenAPI3Schema(schemaRef *openapi3.SchemaRef) graphql.Type {
	if schemaRef == nil || schemaRef.Value == nil {
		return graphql.String
	}
	switch schemaRef.Value.Type {
	case "boolean":
		return graphql.Boolean
	case "integer":
		return graphql.Int
	case "number":
		return graphql.Float
	default:
		return graphql.String
	}
}

func isGraphQLIntegrationAuthParameter(router *openapi3.T, parameter *openapi3.Parameter) bool {
	if parameter == nil {
		return false
	}
	if strings.EqualFold(parameter.In, "header") && strings.EqualFold(parameter.Name, "authorization") {
		return true
	}
	if router == nil || router.Components.SecuritySchemes == nil {
		return false
	}
	for _, securityRef := range router.Components.SecuritySchemes {
		if securityRef == nil || securityRef.Value == nil {
			continue
		}
		scheme := securityRef.Value
		if scheme.Type == "apiKey" && strings.EqualFold(scheme.In, parameter.In) && strings.EqualFold(scheme.Name, parameter.Name) {
			return true
		}
	}
	return false
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

func graphQLSessionUserFromContextValue(user interface{}) *auth.SessionUser {
	if sessionUser, ok := user.(*auth.SessionUser); ok && sessionUser != nil {
		return sessionUser
	}
	if user != nil {
		log.Warnf("Ignoring unexpected user context type [%T] for GraphQL integration operation", user)
	}
	return &auth.SessionUser{}
}

func firstGraphQLNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringInSlice(value string, values []string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
