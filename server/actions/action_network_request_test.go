package actions

import (
	"context"
	"github.com/daptin/daptin/server/actionresponse"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNetworkActionCancellationAndResponseLimit(t *testing.T) {
	t.Run("request cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		request, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		start := time.Now()
		_, _, failures := (&networkRequestActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{"Url": server.URL, "httpRequest": request}, nil)
		if len(failures) != 1 || time.Since(start) > time.Second {
			t.Fatalf("request did not honor cancellation: %v", failures)
		}
	})
	t.Run("oversized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 2<<20))) }))
		defer server.Close()
		_, _, failures := (&networkRequestActionPerformer{}).DoAction(actionresponse.Outcome{}, map[string]interface{}{"Url": server.URL}, nil)
		if len(failures) != 1 || !strings.Contains(failures[0].Error(), "exceeds") {
			t.Fatalf("large response accepted: %v", failures)
		}
	})
}
func TestNetworkActionRejectsMalformedInputsBeforeSending(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	for _, attrs := range []map[string]interface{}{{"Headers": map[string]interface{}{"X-Test": 7}}, {"Headers": "invalid"}, {"Query": map[string]interface{}{"q": 7}}, {"FormData": "invalid"}, {"Method": 7}, {"Url": 7}} {
		if _, ok := attrs["Url"]; !ok {
			attrs["Url"] = server.URL
		}
		_, _, failures := (&networkRequestActionPerformer{}).DoAction(actionresponse.Outcome{}, attrs, nil)
		if len(failures) != 1 {
			t.Fatalf("malformed input accepted: %v", attrs)
		}
	}
	if calls != 0 {
		t.Fatalf("malformed inputs reached provider %d times", calls)
	}
}
