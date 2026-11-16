package server

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
)

func TestIntegrationOperationActionResultUsesExactResponseContract(t *testing.T) {
	result, status, err := integrationOperationActionResult("example", "create", []actionresponse.ActionResponse{
		{ResponseType: "example.create.response", Attributes: map[string]interface{}{"id": "example_123"}},
		{ResponseType: "example.create.statusCode", Attributes: 201},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"id": "example_123"}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("integration operation result = %#v, want %#v", result, want)
	}
	if status != 201 {
		t.Fatalf("integration operation status = %d, want 201", status)
	}
}

func TestIntegrationOperationActionResultRejectsIncompleteResponse(t *testing.T) {
	_, _, err := integrationOperationActionResult("example", "create", []actionresponse.ActionResponse{
		{ResponseType: "example.create.response", Attributes: map[string]interface{}{"id": "example_123"}},
	})
	if err == nil {
		t.Fatal("expected missing provider status to fail")
	}
}

func TestIntegrationOperationErrorStatus(t *testing.T) {
	for _, test := range []struct {
		err  error
		want int
	}{
		{api2go.NewHTTPError(nil, "invalid input", 400), 400},
		{api2go.NewHTTPError(nil, "forbidden", 403), 403},
		{api2go.NewHTTPError(nil, "quota", 402), 402},
		{fmt.Errorf("wrapped: %w", api2go.NewHTTPError(nil, "upstream unavailable", 502)), 502},
		{api2go.NewHTTPError(nil, "upstream timed out", 504), 504},
		{errors.New("internal failure"), http.StatusInternalServerError},
	} {
		if got := integrationOperationErrorStatus(test.err); got != test.want {
			t.Fatalf("status=%d, want %d for %v", got, test.want, test.err)
		}
	}
}
