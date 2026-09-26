package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
)

type checkoutOutcome struct {
	Type       string
	Method     string
	Condition  string
	Attributes map[string]interface{}
}

type checkoutAction struct {
	Name      string
	OutFields []checkoutOutcome
}

type checkoutSchema struct {
	Actions []checkoutAction
}

func TestPaymentCheckoutExampleContract(t *testing.T) {
	root := filepath.Join("examples", "payment-checkout")
	schemaPath := filepath.Join(root, "schemas", "schema_payment_checkout.yaml")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema checkoutSchema
	if err := yaml.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("parse payment schema: %v", err)
	}
	schemaText := string(schemaBytes)
	for _, required := range []string{
		"prepare_checkout", "begin_checkout", "refresh_checkout", "SWITCH_USER",
		"__PAYMENTS_SERVICE_USER_REFERENCE_ID__", "provision_stripe_checkout_credential",
		"checkout_entitlement", "api_member", "archived_at", "current_plan",
	} {
		if !strings.Contains(schemaText, required) {
			t.Fatalf("payment schema is missing %q", required)
		}
	}
	assertCheckoutArchiveBehavior(t, schema)
	if strings.Contains(schemaText, "__PAYMENTS_SERVICE_CREDENTIAL_REFERENCE_ID__") {
		t.Fatal("payment schema must resolve the service-owned credential through Daptin resources")
	}
	for _, forbidden := range []string{"api.sandbox.paypal.com", "ipnpb.sandbox.paypal.com", "$network.request"} {
		if strings.Contains(schemaText, forbidden) {
			t.Fatalf("payment schema contains obsolete or bypass path %q", forbidden)
		}
	}

	openAPIPath := filepath.Join(root, "openapi", "stripe-checkout.json")
	document, err := openapi3.NewLoader().LoadFromFile(openAPIPath)
	if err != nil {
		t.Fatalf("load payment OpenAPI description: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate payment OpenAPI description: %v", err)
	}

	for _, relative := range []string{"README.md", "scripts/setup.mjs", "scripts/verify.mjs"} {
		contents, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(contents))
		for _, forbidden := range []string{"select *", "insert into", "update _config", "delete from"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s documents a direct SQL bypass: %q", relative, forbidden)
			}
		}
	}

	if _, err := os.Stat(filepath.Join("schema", "pay_by_stripe.yaml")); !os.IsNotExist(err) {
		t.Fatalf("obsolete schema/pay_by_stripe.yaml must not remain")
	}
}

func assertCheckoutArchiveBehavior(t *testing.T, schema checkoutSchema) {
	t.Helper()
	prepare := checkoutActionByName(t, schema, "prepare_checkout")
	assertOutcomeEnabled(t, enabledCheckoutOutcomes(t, prepare, map[string]interface{}{
		"subject": map[string]interface{}{"archived_at": nil},
	}), "checkout_attempt", "POST")
	for _, subject := range []map[string]interface{}{
		{"archived_at": "2026-09-09T00:00:00Z"},
		{},
	} {
		outcomes := enabledCheckoutOutcomes(t, prepare, map[string]interface{}{"subject": subject})
		assertUnavailableOutcomeEnabled(t, outcomes)
		assertNoCheckoutSideEffects(t, outcomes)
	}

	begin := checkoutActionByName(t, schema, "begin_checkout")
	activeBegin := enabledCheckoutOutcomes(t, begin, map[string]interface{}{
		"current_plan": map[string]interface{}{"archived_at": nil},
	})
	assertOutcomeEnabled(t, activeBegin, "api_plan", "GET_BY_ID")
	assertOutcomeEnabled(t, activeBegin, "stripe_checkout", "createCheckoutSession")
	for _, outcome := range activeBegin {
		if outcome.Type == "api_plan" && outcome.Method == "GET_BY_ID" && outcome.Attributes["reference_id"] != "$.api_plan_id" {
			t.Fatalf("begin_checkout must reload the plan related to its persisted subject: %#v", outcome.Attributes)
		}
	}
	for _, plan := range []map[string]interface{}{
		{"archived_at": "2026-09-09T00:00:00Z"},
		{},
	} {
		outcomes := enabledCheckoutOutcomes(t, begin, map[string]interface{}{"current_plan": plan})
		assertUnavailableOutcomeEnabled(t, outcomes)
		assertNoCheckoutSideEffects(t, outcomes)
	}

	refresh := checkoutActionByName(t, schema, "refresh_checkout")
	assertOutcomeEnabled(t, refresh.OutFields, "stripe_checkout", "retrieveCheckoutSession")
	assertOutcomeEnabled(t, refresh.OutFields, "api_member", "POST")
	for _, outcome := range refresh.OutFields {
		if strings.Contains(outcome.Condition, "archived_at") {
			t.Fatal("refresh_checkout must reconcile initiated payments after plan archival")
		}
	}
}

func checkoutActionByName(t *testing.T, schema checkoutSchema, name string) checkoutAction {
	t.Helper()
	for _, action := range schema.Actions {
		if action.Name == name {
			return action
		}
	}
	t.Fatalf("payment schema is missing action %q", name)
	return checkoutAction{}
}

func enabledCheckoutOutcomes(t *testing.T, action checkoutAction, context map[string]interface{}) []checkoutOutcome {
	t.Helper()
	enabled := make([]checkoutOutcome, 0, len(action.OutFields))
	for _, outcome := range action.OutFields {
		if outcome.Condition == "" {
			enabled = append(enabled, outcome)
			continue
		}
		value, err := resource.EvaluateString(outcome.Condition, context)
		if err != nil {
			t.Fatalf("evaluate %s condition %q: %v", action.Name, outcome.Condition, err)
		}
		if active, ok := value.(bool); ok && active {
			enabled = append(enabled, outcome)
		}
	}
	return enabled
}

func assertOutcomeEnabled(t *testing.T, outcomes []checkoutOutcome, outcomeType, method string) {
	t.Helper()
	for _, outcome := range outcomes {
		if outcome.Type == outcomeType && outcome.Method == method {
			return
		}
	}
	t.Fatalf("outcome %s/%s is not enabled: %#v", outcomeType, method, outcomes)
}

func assertNoCheckoutSideEffects(t *testing.T, outcomes []checkoutOutcome) {
	t.Helper()
	for _, outcome := range outcomes {
		if outcome.Method != "GET_BY_ID" && outcome.Method != "ACTIONRESPONSE" {
			t.Fatalf("archived plan enabled checkout side effect %s/%s", outcome.Type, outcome.Method)
		}
	}
}

func assertUnavailableOutcomeEnabled(t *testing.T, outcomes []checkoutOutcome) {
	t.Helper()
	for _, outcome := range outcomes {
		if outcome.Type == "client.notify" && outcome.Method == "ACTIONRESPONSE" && outcome.Attributes["type"] == "error" {
			return
		}
	}
	t.Fatalf("archived plan did not return an error notification: %#v", outcomes)
}
