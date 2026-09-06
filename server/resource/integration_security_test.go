package resource

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestEffectiveIntegrationSecurityRequirementsUsesOperationOverride(t *testing.T) {
	global := openapi3.SecurityRequirements{{"global": {}}}
	empty := openapi3.SecurityRequirements{}
	router := &openapi3.T{Security: global}

	if got := EffectiveIntegrationSecurityRequirements(router, &openapi3.Operation{}); len(got) != 1 || got[0]["global"] == nil {
		t.Fatalf("missing operation security did not inherit global requirements: %#v", got)
	}
	if got := EffectiveIntegrationSecurityRequirements(router, &openapi3.Operation{Security: &empty}); len(got) != 0 {
		t.Fatalf("empty operation security did not remove global requirements: %#v", got)
	}
}

func TestIntegrationOperationAuthenticationUsagePreservesAndOrSemantics(t *testing.T) {
	router := &openapi3.T{Components: openapi3.Components{SecuritySchemes: openapi3.SecuritySchemes{
		"first":  {Value: &openapi3.SecurityScheme{Type: "apiKey", In: "header", Name: "X-First"}},
		"second": {Value: &openapi3.SecurityScheme{Type: "apiKey", In: "query", Name: "second_key"}},
		"oauth":  {Value: &openapi3.SecurityScheme{Type: "oauth2"}},
	}}}

	tests := []struct {
		name       string
		require    openapi3.SecurityRequirements
		authType   string
		uses       bool
		required   bool
		shouldFail bool
	}{
		{name: "anonymous", require: openapi3.SecurityRequirements{{}}, authType: "custom_credentials"},
		{name: "optional", require: openapi3.SecurityRequirements{{"first": {}}, {}}, authType: "custom_credentials", uses: true},
		{name: "and", require: openapi3.SecurityRequirements{{"first": {}, "second": {}}}, authType: "custom_credentials", uses: true, required: true},
		{name: "or", require: openapi3.SecurityRequirements{{"oauth": {}}, {"first": {}}}, authType: "custom_credentials", uses: true, required: true},
		{name: "unsupported and", require: openapi3.SecurityRequirements{{"oauth": {}, "first": {}}}, authType: "custom_credentials", shouldFail: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router.Security = test.require
			usage, err := IntegrationOperationAuthenticationUsage(router, &openapi3.Operation{}, test.authType)
			if test.shouldFail {
				if err == nil {
					t.Fatal("expected unsupported requirements to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if usage.UsesAuthentication != test.uses || usage.RequiresAuthentication != test.required {
				t.Fatalf("usage = %+v, want uses=%t required=%t", usage, test.uses, test.required)
			}
		})
	}
}

func TestIntegrationOperationAuthenticationUsageRejectsMissingAndUnresolvedSchemes(t *testing.T) {
	for _, schemes := range []openapi3.SecuritySchemes{
		{},
		{"missing": nil},
		{"missing": {Ref: "#/components/securitySchemes/absent"}},
	} {
		router := &openapi3.T{
			Security:   openapi3.SecurityRequirements{{"missing": {}}},
			Components: openapi3.Components{SecuritySchemes: schemes},
		}
		if _, err := IntegrationOperationAuthenticationUsage(router, &openapi3.Operation{}, "custom_credentials"); err == nil {
			t.Fatalf("expected invalid security scheme to fail: %#v", schemes)
		}
	}
}

func TestOpenAPIV2ConversionPreservesOperationSecurityOverride(t *testing.T) {
	empty := openapi2.SecurityRequirements{}
	document, err := openapi2conv.ToV3(&openapi2.T{
		Security: openapi2.SecurityRequirements{{"apiKey": {}}},
		SecurityDefinitions: map[string]*openapi2.SecurityScheme{
			"apiKey": {Type: "apiKey", In: "header", Name: "X-API-Key"},
		},
		Paths: map[string]*openapi2.PathItem{
			"/public": {Get: &openapi2.Operation{Security: &empty}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	operation := document.Paths["/public"].Get
	if operation.Security == nil {
		t.Fatal("OpenAPI v2 empty operation security lost its explicit override")
	}
	if got := EffectiveIntegrationSecurityRequirements(document, operation); len(got) != 0 {
		t.Fatalf("converted empty operation security inherited global auth: %#v", got)
	}
}
