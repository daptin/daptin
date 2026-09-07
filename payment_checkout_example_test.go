package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
)

func TestPaymentCheckoutExampleContract(t *testing.T) {
	root := filepath.Join("examples", "payment-checkout")
	schemaPath := filepath.Join(root, "schemas", "schema_payment_checkout.yaml")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]interface{}
	if err := yaml.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("parse payment schema: %v", err)
	}
	schemaText := string(schemaBytes)
	for _, required := range []string{
		"prepare_checkout", "begin_checkout", "refresh_checkout", "SWITCH_USER",
		"__PAYMENTS_SERVICE_USER_REFERENCE_ID__", "provision_stripe_checkout_credential",
		"checkout_entitlement", "api_member",
	} {
		if !strings.Contains(schemaText, required) {
			t.Fatalf("payment schema is missing %q", required)
		}
	}
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
