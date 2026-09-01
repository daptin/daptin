package resource

import (
	"testing"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

func TestLLMProviderFromCanonicalResourceRow(t *testing.T) {
	providerReference := uuid.MustParse("018f9f6d-7474-7d75-8000-000000000001")
	credentialReference := uuid.MustParse("018f9f6d-7474-7d75-8000-000000000002")
	provider, err := llmProviderFromRow(map[string]interface{}{
		"id":                    int64(4),
		"name":                  "primary",
		"provider_type":         "openai-compatible",
		"base_url":              "https://provider.example/v1",
		"credential_id":         daptinid.DaptinReferenceId(credentialReference),
		"provider_parameters":   `{"header":"value"}`,
		"allow_insecure":        false,
		"allow_private_network": int64(1),
		"enable":                true,
		"reference_id":          providerReference[:],
		"version":               int64(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.ReferenceId != daptinid.DaptinReferenceId(providerReference) ||
		provider.CredentialId != daptinid.DaptinReferenceId(credentialReference) {
		t.Fatalf("provider references were not preserved: %#v", provider)
	}
	if !provider.Enable || provider.AllowInsecure || !provider.AllowPrivateNetwork {
		t.Fatalf("provider boolean fields were not normalized: %#v", provider)
	}
	if provider.ProviderParameters["header"] != "value" {
		t.Fatalf("provider parameters were not decoded: %#v", provider.ProviderParameters)
	}
}

func TestLLMProviderFromCanonicalResourceRowRejectsInvalidJSON(t *testing.T) {
	_, err := llmProviderFromRow(map[string]interface{}{
		"name":                "broken",
		"provider_type":       "openai-compatible",
		"provider_parameters": `{`,
	})
	if err == nil {
		t.Fatal("expected invalid provider parameters to fail catalog loading")
	}
}
