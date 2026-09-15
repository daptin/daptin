package resource

import (
	stdjson "encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenericRestExchangeUsesConfiguredTarget(t *testing.T) {
	var received map[string]interface{}
	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", request.Method)
		}
		if request.Header.Get("X-Exchange") != "probe" {
			t.Errorf("X-Exchange = %q, want probe", request.Header.Get("X-Exchange"))
		}
		if err := stdjson.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)

	execution := NewExchangeExecution(ExchangeContract{
		Name:       "webhook",
		TargetType: "rest",
		TargetAttributes: map[string]interface{}{
			"url": target.URL, "method": http.MethodPatch,
			"headers": map[string]interface{}{"X-Exchange": "probe"},
		},
	}, &map[string]*DbResource{})
	if _, err := execution.Execute([]map[string]interface{}{{"__type": "order", "value": "accepted"}}, nil); err != nil {
		t.Fatal(err)
	}
	if received["value"] != "accepted" {
		t.Fatalf("received value = %v, want accepted", received["value"])
	}
}

func TestRestExchangeReturnsHTTPAndTransportFailures(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(target.Close)
	closedTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedTargetURL := closedTarget.URL
	closedTarget.Close()

	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "http status", url: target.URL},
		{name: "no response", url: "://invalid"},
		{name: "connection failure", url: closedTargetURL},
	} {
		t.Run(test.name, func(t *testing.T) {
			execution := NewExchangeExecution(ExchangeContract{
				Name:       "webhook",
				TargetType: "rest",
				TargetAttributes: map[string]interface{}{
					"url": test.url, "method": http.MethodPost,
				},
			}, &map[string]*DbResource{})
			if _, err := execution.Execute([]map[string]interface{}{{"__type": "order"}}, nil); err == nil {
				t.Fatal("REST exchange failure returned no error")
			}
		})
	}
}

func TestNamedRestExchangeUsesExistingRegistry(t *testing.T) {
	if _, err := NewRestExchangeHandler(ExchangeContract{TargetType: "gsheet-append"}); err != nil {
		t.Fatal(err)
	}
}
