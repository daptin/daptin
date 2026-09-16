package resource

import (
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestExchangeMiddlewareHasOneFailureBoundary(t *testing.T) {
	methods := []interface{}{"patch"}
	rows := []map[string]interface{}{{"__type": "order"}}
	em := &exchangeMiddleware{exchangeMap: map[string][]ExchangeContract{
		"order": {{
			Name: "invalid-target", TargetType: "invalid",
			Attributes: map[string]interface{}{"methods": methods, "hook": "before"},
		}},
	}}
	req := &api2go.Request{PlainRequest: &http.Request{Method: http.MethodPatch}}

	if _, err := em.InterceptBefore(nil, req, rows, nil); err != nil {
		t.Fatalf("before exchange changed the established continue-on-target-failure behavior: %v", err)
	}

	em.exchangeMap["order"][0].Attributes["hook"] = "after"
	if _, err := em.InterceptAfter(nil, req, rows, nil); err == nil {
		t.Fatal("after exchange must require the source mutation transaction")
	}
}

func TestExchangeEnqueueRowUsesCommittedUpdateVersion(t *testing.T) {
	updatedRow := map[string]interface{}{"reference_id": "source", "version": float64(4)}
	enqueueRow, err := exchangeEnqueueRow(updatedRow, "patch")
	if err != nil {
		t.Fatal(err)
	}
	if enqueueRow["version"] != int64(5) {
		t.Fatalf("enqueued version = %#v, want 5", enqueueRow["version"])
	}
	if updatedRow["version"] != float64(4) {
		t.Fatalf("middleware changed the resource response row: %#v", updatedRow["version"])
	}
}
