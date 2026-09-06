package resource

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// IntegrationOperationAuthUsage describes whether Daptin can and must attach
// provider authentication to an operation using the integration's configured
// credential resolver.
type IntegrationOperationAuthUsage struct {
	UsesAuthentication     bool
	RequiresAuthentication bool
}

// EffectiveIntegrationSecurityRequirements applies OpenAPI operation override
// semantics. A non-nil operation security value, including an empty array,
// replaces the document-level declaration.
func EffectiveIntegrationSecurityRequirements(router *openapi3.T, operation *openapi3.Operation) openapi3.SecurityRequirements {
	if operation != nil && operation.Security != nil {
		return *operation.Security
	}
	if router == nil {
		return nil
	}
	return router.Security
}

// IntegrationSecurityRequirementSupported reports whether one complete
// Security Requirement Object can be satisfied by the configured Daptin
// credential resolver. Every scheme in the object must be supported.
func IntegrationSecurityRequirementSupported(authType string, requirement openapi3.SecurityRequirement, schemes openapi3.SecuritySchemes) (bool, error) {
	if len(requirement) == 0 {
		return true, nil
	}
	for name := range requirement {
		ref, ok := schemes[name]
		if !ok {
			return false, fmt.Errorf("OpenAPI security requirement references missing scheme [%s]", name)
		}
		if ref == nil || ref.Value == nil {
			return false, fmt.Errorf("OpenAPI security scheme [%s] is unresolved", name)
		}
		if !integrationAuthTypeSupportsScheme(authType, ref.Value) {
			return false, nil
		}
	}
	return true, nil
}

// IntegrationOperationAuthenticationUsage classifies the effective OpenAPI
// requirements for action inputs and operation discovery.
func IntegrationOperationAuthenticationUsage(router *openapi3.T, operation *openapi3.Operation, authType string) (IntegrationOperationAuthUsage, error) {
	requirements := EffectiveIntegrationSecurityRequirements(router, operation)
	if len(requirements) == 0 {
		return IntegrationOperationAuthUsage{}, nil
	}

	hasAnonymous := false
	hasSupportedAuthentication := false
	var schemes openapi3.SecuritySchemes
	if router != nil {
		schemes = router.Components.SecuritySchemes
	}
	for _, requirement := range requirements {
		if len(requirement) == 0 {
			hasAnonymous = true
			continue
		}
		supported, err := IntegrationSecurityRequirementSupported(authType, requirement, schemes)
		if err != nil {
			return IntegrationOperationAuthUsage{}, err
		}
		if supported {
			hasSupportedAuthentication = true
		}
	}

	if !hasAnonymous && !hasSupportedAuthentication {
		return IntegrationOperationAuthUsage{}, fmt.Errorf("no OpenAPI security requirement can be satisfied by integration authentication_type [%s]", authType)
	}
	return IntegrationOperationAuthUsage{
		UsesAuthentication:     hasSupportedAuthentication,
		RequiresAuthentication: hasSupportedAuthentication && !hasAnonymous,
	}, nil
}

func integrationAuthTypeSupportsScheme(authType string, scheme *openapi3.SecurityScheme) bool {
	if scheme == nil {
		return false
	}
	switch strings.ToLower(authType) {
	case "oauth2":
		return scheme.Type == "oauth2"
	case "custom_credentials":
		switch scheme.Type {
		case "http":
			return strings.EqualFold(scheme.Scheme, "basic") || strings.EqualFold(scheme.Scheme, "bearer")
		case "apiKey":
			return strings.EqualFold(scheme.In, "header") || strings.EqualFold(scheme.In, "query") || strings.EqualFold(scheme.In, "cookie")
		default:
			return false
		}
	default:
		return false
	}
}
