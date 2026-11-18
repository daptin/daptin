package main

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestRateLimiterRejectsForgedForwardingHeadersRealE2E(t *testing.T) {
	f := startDaptinE2E(t, transportE2EDaptinOptions{})
	admin := f.SignupAdmin(t)
	ftpE2ESetConfig(t, f.Client, f.URL, admin, "limit.rate", `{"version":"1","limits":{"/statistics":2}}`)
	accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/action/world/restart_daptin", admin,
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	waitForAdminRestart(t, f.Client, f.URL)

	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		t.Run(header, func(t *testing.T) {
			// Start with a fresh window so each header is tested independently.
			time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
			// Allow for a second boundary during the burst. Each request claims a new IP.
			for i := 0; i < 12; i++ {
				request, err := http.NewRequest(http.MethodGet, f.URL+"/statistics", nil)
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set(header, fmt.Sprintf("198.51.100.%d", i+1))
				response, err := f.Client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
				if response.Header.Get("X-RateLimit-Limit") != "2" {
					t.Fatalf("configured limit=%q, want 2", response.Header.Get("X-RateLimit-Limit"))
				}
				if response.StatusCode == http.StatusTooManyRequests {
					return
				}
				if response.StatusCode != http.StatusForbidden {
					t.Fatalf("anonymous statistics status=%d, want 403 or 429", response.StatusCode)
				}
			}
			t.Fatal("rotating forwarding headers bypassed the configured rate limit")
		})
	}
}
