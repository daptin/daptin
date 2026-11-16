package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jmoiron/sqlx"
)

func TestIntegrationHTTPUpstreamErrorSanitizesCauses(t *testing.T) {
	for _, test := range []struct {
		name   string
		cause  error
		status int
	}{
		{"connection", &url.Error{Op: "Get", URL: "https://secret:password@private.test/?token=secret", Err: errors.New("connection refused")}, 502},
		{"deadline", &url.Error{Op: "Post", URL: "https://private.test/?secret=token", Err: context.DeadlineExceeded}, 504},
		{"network timeout", &net.DNSError{Err: "private resolver", IsTimeout: true}, 504},
		{"body read", io.ErrUnexpectedEOF, 502},
		{"cancel", context.Canceled, 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := integrationHTTPUpstreamError(test.cause, "integration upstream unavailable")
			var typed api2go.HTTPError
			if !errors.As(err, &typed) || typed.Status() != test.status {
				t.Fatalf("error=%v; want HTTP %d", err, test.status)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
				t.Fatalf("sensitive cause leaked: %v", err)
			}
		})
	}
}

func TestIntegrationHTTPTimeoutAndCancellationReachProvider(t *testing.T) {
	for _, test := range []struct {
		name    string
		timeout int
		cancel  bool
	}{{"configured timeout", 250, false}, {"parent cancellation", 10000, true}} {
		t.Run(test.name, func(t *testing.T) {
			reached := make(chan struct{})
			stopped := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(reached); <-r.Context().Done(); close(stopped) }))
			defer upstream.Close()
			op := &openapi3.Operation{OperationID: "invoke"}
			op.Extensions = map[string]interface{}{"x-daptin-timeout-ms": test.timeout}
			performer := &integrationActionPerformer{integration: resource.Integration{Name: "failure.example"},
				router:     &openapi3.T{Servers: openapi3.Servers{{URL: upstream.URL}}, Paths: openapi3.Paths{"/invoke": {Get: op}}},
				commandMap: map[string]*openapi3.Operation{"invoke": op}, pathMap: map[string]string{"invoke": "/invoke"}, methodMap: map[string]string{"invoke": "get"},
				runtimeState: func(*sqlx.Tx) (string, bool, error) { return "failure.example", true, nil },
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancel {
				go func() {
					select {
					case <-reached:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			_, _, failures := performer.DoAction(actionresponse.Outcome{Method: "invoke"}, map[string]interface{}{"httpRequest": httptest.NewRequest(http.MethodPost, "/integration/failure.example/invoke", nil).WithContext(ctx)}, nil)
			want := 504
			if test.cancel {
				want = 502
			}
			var typed api2go.HTTPError
			if len(failures) != 1 || !errors.As(failures[0], &typed) || typed.Status() != want {
				t.Fatalf("failures=%v, want HTTP %d", failures, want)
			}
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("provider request was not cancelled")
			}
		})
	}
}

func TestIntegrationHTTPResponsesShareOutcomeContract(t *testing.T) {
	for _, status := range []int{200, 201, 202, 400, 422, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"detail":"fixture-provider-detail","units":2}`)
			}))
			defer upstream.Close()
			op := &openapi3.Operation{OperationID: "invoke"}
			performer := &integrationActionPerformer{integration: resource.Integration{Name: "failure.example"},
				router:     &openapi3.T{Servers: openapi3.Servers{{URL: upstream.URL}}, Paths: openapi3.Paths{"/invoke": {Get: op}}},
				commandMap: map[string]*openapi3.Operation{"invoke": op}, pathMap: map[string]string{"invoke": "/invoke"}, methodMap: map[string]string{"invoke": "get"},
				runtimeState: func(*sqlx.Tx) (string, bool, error) { return "failure.example", true, nil },
			}
			responder, responses, failures := performer.DoAction(actionresponse.Outcome{Method: "invoke"}, nil, nil)
			if status < 400 {
				if len(failures) != 0 || responder == nil || responder.StatusCode() != status || len(responses) != 2 {
					t.Fatalf("HTTP %d result: responder=%v errors=%v", status, responder, failures)
				}
				return
			}
			var typed api2go.HTTPError
			if responder != nil || len(responses) != 0 || len(failures) != 1 || !errors.As(failures[0], &typed) || typed.Status() != status {
				t.Fatalf("HTTP %d rejection: responder=%v errors=%v", status, responder, failures)
			}
			if strings.Contains(typed.Error(), "fixture-provider-detail") || strings.Contains(typed.Error(), upstream.URL) {
				t.Fatalf("raw provider details leaked into public error: %v", typed)
			}
			body, ok := IntegrationHTTPRejectionResponse(fmt.Errorf("wrapped: %w", failures[0]), "failure.example", "invoke")
			if !ok || body["detail"] != "fixture-provider-detail" || body["units"] != float64(2) || len(body) != 2 {
				t.Fatalf("provider response not preserved: %#v", body)
			}
			if _, ok = IntegrationHTTPRejectionResponse(failures[0], "other.example", "invoke"); ok {
				t.Fatal("rejection attached to another integration")
			}
			if _, ok = IntegrationHTTPRejectionResponse(failures[0], "failure.example", "other"); ok {
				t.Fatal("rejection attached to another operation")
			}
		})
	}
	if _, ok := IntegrationHTTPRejectionResponse(api2go.NewHTTPError(nil, "forbidden", 403), "failure.example", "invoke"); ok {
		t.Fatal("authorization error treated as provider JSON")
	}
}
