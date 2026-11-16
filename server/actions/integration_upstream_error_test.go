package actions

import (
	"context"
	"errors"
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
