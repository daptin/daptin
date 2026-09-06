package resource

import (
	"strings"
	"testing"
)

func TestIntegrationOperationActionNameScopesOperationToProvider(t *testing.T) {
	first, err := IntegrationOperationActionName("asana.com", "getUser")
	if err != nil {
		t.Fatal(err)
	}
	second, err := IntegrationOperationActionName("github.com", "getUser")
	if err != nil {
		t.Fatal(err)
	}
	if first != "asana.com/getUser" || second != "github.com/getUser" || first == second {
		t.Fatalf("provider-scoped action names are not distinct: %q %q", first, second)
	}
}

func TestIntegrationOperationActionNameRejectsAmbiguousOrOversizedNames(t *testing.T) {
	for _, test := range []struct {
		provider  string
		operation string
	}{
		{provider: "", operation: "getUser"},
		{provider: "provider/child", operation: "getUser"},
		{provider: "provider", operation: strings.Repeat("x", 100)},
	} {
		if _, err := IntegrationOperationActionName(test.provider, test.operation); err == nil {
			t.Fatalf("expected invalid action identity to fail: %#v", test)
		}
	}
}
