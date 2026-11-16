package subsite

import (
	"crypto/tls"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseHeadersValidation(t *testing.T) {
	for _, value := range []interface{}{nil, "null", "{}", map[string]interface{}{}, `{"content-security-policy":"default-src 'self'","X-Content-Type-Options":"nosniff"}`} {
		if _, err := ParseResponseHeaders(value); err != nil {
			t.Errorf("valid policy %#v: %v", value, err)
		}
	}
	for _, value := range []interface{}{
		`[]`, `true`, `"policy"`, `{"X-Test":3}`, `{"X-Test":null}`, `{"X-Test":["a"]}`,
		`{"X-Test":"one","x-test":"two"}`, `{"X-Test":"one","X-Test":"two"}`,
		`{"bad header":"value"}`, `{"X-Test":"one\r\nInjected: yes"}`, `{"X-Test":"\u0000"}`,
		`{"Content-Length":"0"}`, `{"content-encoding":"gzip"}`, `{"Vary":"Origin"}`, `{"Set-Cookie":"token=one"}`,
		`{"ETag":"bad"}`, `{"Content-Type":"text/html"}`, `{} {}`, `{"X-Test":"value"`, "",
		map[string]interface{}{"X-Test": strings.Repeat("x", 16384)},
		`{"X-Test":"` + strings.Repeat("<", 3000) + `"}`,
	} {
		if _, err := ParseResponseHeaders(value); err == nil {
			t.Errorf("invalid policy accepted: %#v", value)
		}
	}
	many := map[string]string{}
	for i := 0; i < 33; i++ {
		many[fmt.Sprintf("X-Test-%d", i)] = "value"
	}
	if _, err := ParseResponseHeaders(many); err == nil {
		t.Fatal("unbounded header count accepted")
	}
}

func TestResponseHeadersOnlyUseNativeTLSForHSTS(t *testing.T) {
	policy, err := ParseResponseHeaders(`{"strict-transport-security":"max-age=31536000","x-content-type-options":"nosniff"}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, secure := range []bool{false, true} {
		request := httptest.NewRequest("GET", "http://site.test/", nil)
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("Forwarded", "proto=https")
		if secure {
			request.TLS = &tls.ConnectionState{}
		}
		response := httptest.NewRecorder()
		policy.Apply(response, request)
		if (response.Header().Get("Strict-Transport-Security") != "") != secure {
			t.Fatalf("TLS=%t: HSTS=%q", secure, response.Header().Get("Strict-Transport-Security"))
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("policy lost on HTTP")
		}
	}
}
