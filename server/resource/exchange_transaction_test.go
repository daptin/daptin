package resource

import (
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestExchangeMiddlewareReturnsTargetFailure(t *testing.T) {
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
			var err error
			if hook == "before" {
				_, err = em.InterceptBefore(nil, req, rows, nil)
			} else {
				_, err = em.InterceptAfter(nil, req, rows, nil)
			}
			if err == nil {
				t.Fatal("expected exchange target failure")
			}
		})
	}
}
