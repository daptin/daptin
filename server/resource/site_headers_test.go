package resource

import (
	"errors"
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func TestSiteHeadersUseCanonicalResourceValidation(t *testing.T) {
	var table table_info.TableInfo
	for _, candidate := range StandardTables {
		if candidate.TableName == "site" {
			table = candidate
		}
	}
	if len(table.Validations) == 0 {
		t.Fatal("Site validation missing from system schema")
	}
	middleware := NewDataValidationMiddleware(&CmsConfig{Tables: []table_info.TableInfo{table}}, nil)
	crud := &DbResource{model: api2go.NewApi2GoModel("site", nil, 0, nil)}
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		for _, value := range []interface{}{nil, map[string]interface{}{}, map[string]interface{}{"Content-Security-Policy": "default-src 'self'"}, `{"Referrer-Policy":"no-referrer"}`} {
			if _, err := middleware.InterceptBefore(crud, &api2go.Request{PlainRequest: &http.Request{Method: method}}, []map[string]interface{}{{"headers": value}}, nil); err != nil {
				t.Fatalf("%s valid policy: %v", method, err)
			}
		}
		for _, value := range []interface{}{[]interface{}{}, map[string]interface{}{"Bad Name": "value"}, map[string]interface{}{"Content-Length": "0"}, map[string]interface{}{"X-Test": "one\nInjected: yes"}} {
			_, err := middleware.InterceptBefore(crud, &api2go.Request{PlainRequest: &http.Request{Method: method}}, []map[string]interface{}{{"headers": value}}, nil)
			var httpErr api2go.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status() != 400 {
				t.Fatalf("%s invalid policy error=%v, want HTTP 400", method, err)
			}
		}
		if _, err := middleware.InterceptBefore(crud, &api2go.Request{PlainRequest: &http.Request{Method: method}}, []map[string]interface{}{{"name": "site"}}, nil); err != nil {
			t.Fatalf("%s missing optional policy: %v", method, err)
		}
	}
}
