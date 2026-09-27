package resource

import (
	"errors"
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/table_info"
)

func TestDataValidationMiddlewareConformsBeforeValidation(t *testing.T) {
	config := &CmsConfig{Tables: []table_info.TableInfo{{
		TableName: "conformation_probe",
		Validations: []columns.ColumnTag{
			{ColumnName: "code", Tags: "required,alphanum"},
		},
		Conformations: []columns.ColumnTag{
			{ColumnName: "code", Tags: "trim,upper"},
			{ColumnName: "count", Tags: "trim"},
		},
	}}}
	middleware := NewDataValidationMiddleware(config, nil)
	crud := &DbResource{model: api2go.NewApi2GoModel("conformation_probe", nil, 0, nil)}

	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			objects := []map[string]interface{}{{"code": "  ab12  ", "count": int64(7)}}
			result, err := middleware.InterceptBefore(crud, &api2go.Request{
				PlainRequest: &http.Request{Method: method},
			}, objects, nil)
			if err != nil {
				t.Fatalf("conform then validate: %v", err)
			}
			if got := result[0]["code"]; got != "AB12" {
				t.Fatalf("code = %#v, want AB12", got)
			}
			if got := result[0]["count"]; got != int64(7) {
				t.Fatalf("non-string count changed: %#v", got)
			}
		})
	}
}

func TestDataValidationMiddlewareValidatesConformedValue(t *testing.T) {
	config := &CmsConfig{Tables: []table_info.TableInfo{{
		TableName: "conformation_probe",
		Validations: []columns.ColumnTag{
			{ColumnName: "code", Tags: "required,alphanum"},
		},
		Conformations: []columns.ColumnTag{
			{ColumnName: "code", Tags: "trim,upper"},
		},
	}}}
	middleware := NewDataValidationMiddleware(config, nil)
	crud := &DbResource{model: api2go.NewApi2GoModel("conformation_probe", nil, 0, nil)}
	objects := []map[string]interface{}{{"code": "  ab-12  "}}

	_, err := middleware.InterceptBefore(crud, &api2go.Request{
		PlainRequest: &http.Request{Method: http.MethodPost},
	}, objects, nil)
	if err == nil {
		t.Fatal("expected the conformed non-alphanumeric value to fail validation")
	}
	if got := objects[0]["code"]; got != "AB-12" {
		t.Fatalf("code = %#v, want conformed value AB-12", got)
	}
}

func TestDataValidationMiddlewareRejectsMissingRequiredFieldOnlyOnCreate(t *testing.T) {
	config := &CmsConfig{Tables: []table_info.TableInfo{{
		TableName: "validation_probe",
		Validations: []columns.ColumnTag{
			{ColumnName: "name", Tags: "required"},
			{ColumnName: "count", Tags: "gte=0"},
		},
	}}}
	middleware := NewDataValidationMiddleware(config, nil)
	crud := &DbResource{model: api2go.NewApi2GoModel("validation_probe", nil, 0, nil)}

	_, err := middleware.InterceptBefore(crud, &api2go.Request{
		PlainRequest: &http.Request{Method: http.MethodPost},
	}, []map[string]interface{}{{}}, nil)
	if err == nil {
		t.Fatal("missing required field was accepted on create")
	}
	var httpErr api2go.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status() != http.StatusBadRequest {
		t.Fatalf("missing required field error = %T %v, want HTTP 400", err, err)
	}
	_, err = middleware.InterceptBefore(crud, &api2go.Request{
		PlainRequest: &http.Request{Method: http.MethodPost},
	}, []map[string]interface{}{{"name": "present"}}, nil)
	if err != nil {
		t.Fatalf("create rejected omitted optional validated field: %v", err)
	}

	_, err = middleware.InterceptBefore(crud, &api2go.Request{
		PlainRequest: &http.Request{Method: http.MethodPatch},
	}, []map[string]interface{}{{}}, nil)
	if err != nil {
		t.Fatalf("partial update rejected omitted required field: %v", err)
	}
}

func TestDataValidationMiddlewareAssetTypes(t *testing.T) {
	config := &CmsConfig{Tables: []table_info.TableInfo{{
		TableName: "asset_probe",
		Columns: []api2go.ColumnInfo{
			{ColumnName: "photo", ColumnType: "image"},
			{ColumnName: "document", ColumnType: "file.pdf"},
			{ColumnName: "collaborative", ColumnType: "file.md|txt"},
		},
	}}}
	middleware := NewDataValidationMiddleware(config, nil)
	crud := &DbResource{model: api2go.NewApi2GoModel("asset_probe", nil, 0, nil)}
	request := &api2go.Request{PlainRequest: &http.Request{Method: http.MethodPost}}

	for _, test := range []struct {
		name  string
		value interface{}
		bad   bool
	}{
		{"image", []interface{}{map[string]interface{}{"name": "photo.png", "type": "image/png", "file": "data:image/png;base64,AA=="}}, false},
		{"wrong extension", []interface{}{map[string]interface{}{"name": "photo.txt", "type": "text/plain"}}, true},
		{"wrong MIME", []interface{}{map[string]interface{}{"name": "photo.png", "type": "text/plain"}}, true},
		{"conflicting data URL", []interface{}{map[string]interface{}{"name": "photo.png", "type": "image/png", "file": "data:text/plain;base64,AA=="}}, true},
		{"inline binary", "AA==", false},
		{"missing metadata", []interface{}{map[string]interface{}{"name": "photo.png", "file": "AA=="}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := middleware.InterceptBefore(crud, request, []map[string]interface{}{{"photo": test.value}}, nil)
			if (err != nil) != test.bad {
				t.Fatalf("error = %v, want rejection %v", err, test.bad)
			}
		})
	}
	_, err := middleware.InterceptBefore(crud, request, []map[string]interface{}{{"document": `[{"name":"brief.pdf","type":"application/pdf"}]`}}, nil)
	if err != nil {
		t.Fatalf("PDF rejected: %v", err)
	}
	_, err = middleware.InterceptBefore(crud, request, []map[string]interface{}{{
		"collaborative": []interface{}{map[string]interface{}{
			"name": "collaborative.yjs", "type": YjsStateMediaType, "contents": "AA==",
		}},
	}}, nil)
	if err != nil {
		t.Fatalf("internal YJS state rejected: %v", err)
	}
}
