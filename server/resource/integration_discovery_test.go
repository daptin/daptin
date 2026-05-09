package resource

import "testing"

func TestIntegrationEnableFlagAcceptsBooleanValues(t *testing.T) {
	enabled, err := integrationEnableFlag(true)
	if err != nil {
		t.Fatalf("boolean true should not fail: %v", err)
	}
	if !enabled {
		t.Fatalf("boolean true should mark integration enabled")
	}

	enabled, err = integrationEnableFlag(false)
	if err != nil {
		t.Fatalf("boolean false should not fail: %v", err)
	}
	if enabled {
		t.Fatalf("boolean false should mark integration disabled")
	}
}

func TestIntegrationEnableFlagAcceptsNumericAndStringValues(t *testing.T) {
	cases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{name: "int enabled", value: int(1), expected: true},
		{name: "int disabled", value: int(0), expected: false},
		{name: "int64 enabled", value: int64(1), expected: true},
		{name: "numeric string enabled", value: "1", expected: true},
		{name: "string true enabled", value: "true", expected: true},
		{name: "string false disabled", value: "false", expected: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled, err := integrationEnableFlag(tc.value)
			if err != nil {
				t.Fatalf("enable value should parse: %v", err)
			}
			if enabled != tc.expected {
				t.Fatalf("enabled = %v, want %v", enabled, tc.expected)
			}
		})
	}
}
