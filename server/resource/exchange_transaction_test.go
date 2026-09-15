package resource

import (
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestExchangeMiddlewareFailureBoundary(t *testing.T) {
	methods := []interface{}{"patch"}
	rows := []map[string]interface{}{{"__type": "order"}}
	em := &exchangeMiddleware{
		exchangeMap: map[string][]ExchangeContract{
			"order": {
				{
					Name:       "invalid-target",
					TargetType: "invalid",
					Attributes: map[string]interface{}{
						"methods": methods,
					},
				},
			},
		},
	}
	req := &api2go.Request{PlainRequest: &http.Request{Method: http.MethodPatch}}

	for _, hook := range []string{"before", "after"} {
		t.Run(hook, func(t *testing.T) {
			em.exchangeMap["order"][0].Attributes["hook"] = hook
			em.exchangeMap["order"][0].Options = nil
			if _, err := interceptExchange(em, hook, req, rows); err != nil {
				t.Fatalf("default failure policy changed the established mutation behavior: %v", err)
			}

			em.exchangeMap["order"][0].Options = map[string]interface{}{"on_error": exchangeOnErrorError}
			if _, err := interceptExchange(em, hook, req, rows); err == nil {
				t.Fatal("explicit error policy must return the exchange failure")
			}
		})
	}
}

func interceptExchange(em *exchangeMiddleware, hook string, request *api2go.Request,
	rows []map[string]interface{}) ([]map[string]interface{}, error) {
	if hook == "before" {
		return em.InterceptBefore(nil, request, rows, nil)
	}
	return em.InterceptAfter(nil, request, rows, nil)
}

func TestExchangeErrorPolicy(t *testing.T) {
	for _, test := range []struct {
		name    string
		options map[string]interface{}
		want    string
		wantErr bool
	}{
		{name: "default", want: exchangeOnErrorContinue},
		{name: "continue", options: map[string]interface{}{"on_error": "continue"}, want: exchangeOnErrorContinue},
		{name: "retry", options: map[string]interface{}{"on_error": "retry"}, want: exchangeOnErrorRetry},
		{name: "error", options: map[string]interface{}{"on_error": "error"}, want: exchangeOnErrorError},
		{name: "invalid", options: map[string]interface{}{"on_error": "ignore"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := exchangeErrorPolicy(ExchangeContract{Name: "orders", Options: test.options})
			if (err != nil) != test.wantErr {
				t.Fatalf("exchangeErrorPolicy() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("exchangeErrorPolicy() = %q, want %q", got, test.want)
			}
		})
	}
}
