package main

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

func TestSiteResponseHeadersRealE2E(t *testing.T) {
	requireRealE2E(t)
	used := map[int]bool{}
	databasePath := filepath.Join(t.TempDir(), "site-headers.db")
	root := t.TempDir()
	content := strings.Repeat("customer frontend ", 120)
	for name, body := range map[string]string{"index.html": "<html>" + content + "</html>", "app.js": content} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.js")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "blocked.js")); err != nil {
		t.Fatal(err)
	}
	start := func() (string, int, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, used)
		https := freeTransportE2EPort(t, used)
		olric := freeTransportE2EPortPair(t, used)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		return base, https, startTransportE2EDaptin(t, port, https, base, transportE2EDaptinOptions{connectionString: databasePath, olricPort: olric})
	}
	base, _, process := start()
	client := &http.Client{Timeout: 30 * time.Second}
	admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
	reader := accessGroupsE2ESignupSigninUser(t, client, base, admin, "site-header-reader")
	store := accessGroupsE2ECreateRecord(t, client, base, admin, "cloud_store", map[string]interface{}{
		"name": "site-headers-store", "store_type": "local", "store_provider": "local", "root_path": root, "store_parameters": "{}",
	})
	policy := map[string]interface{}{
		"Content-Security-Policy": "default-src 'self'; object-src 'none'; base-uri 'self'",
		"X-Content-Type-Options":  "nosniff", "Referrer-Policy": "strict-origin-when-cross-origin", "Strict-Transport-Security": "max-age=31536000",
	}
	makeSite := func(name string, headers interface{}) string {
		id := accessGroupsE2ECreateRecord(t, client, base, admin, "site", map[string]interface{}{"name": name, "hostname": name + ".test", "path": "", "enable": true, "site_type": "static", "headers": headers})
		accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/site/"+id, admin, map[string]interface{}{"data": map[string]interface{}{
			"type": "site", "id": id, "attributes": map[string]interface{}{"permission": int64(auth.UserCRUD | auth.UserExecute | auth.GuestExecute)},
			"relationships": map[string]interface{}{"cloud_store_id": map[string]interface{}{"data": map[string]interface{}{"type": "cloud_store", "id": store}}},
		}}, http.StatusOK)
		return id
	}
	siteID := makeSite("headers-one", policy)
	makeSite("headers-two", map[string]interface{}{"Content-Security-Policy": "default-src 'none'"})
	makeSite("headers-empty", nil)
	for _, bad := range []interface{}{[]interface{}{}, map[string]interface{}{"X-Test": "bad\r\nInjected: yes"}, map[string]interface{}{"content-length": "0"}, map[string]interface{}{"X-Test": 3}} {
		accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/site/"+siteID, admin,
			accessGroupsE2ERecordPayload("site", siteID, map[string]interface{}{"headers": bad}), http.StatusBadRequest)
	}
	accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/api/site", admin,
		accessGroupsE2ERecordPayload("site", "", map[string]interface{}{"name": "bad-policy", "hostname": "bad.test", "headers": map[string]interface{}{"Content-Encoding": "gzip"}}), http.StatusBadRequest)
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/site/"+siteID, reader,
		accessGroupsE2ERecordPayload("site", siteID, map[string]interface{}{"headers": map[string]interface{}{"X-Test": "untrusted"}}), http.StatusForbidden)
	stored := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/site/"+siteID, admin, nil, http.StatusOK).(map[string]interface{})
	if stored["data"].(map[string]interface{})["attributes"].(map[string]interface{})["headers"].(map[string]interface{})["Content-Security-Policy"] != policy["Content-Security-Policy"] {
		t.Fatal("invalid or denied update changed persisted policy")
	}
	ftpE2ESetConfig(t, client, base, admin, "enable_https", "true")
	ftpE2ESetConfig(t, client, base, admin, "hostname", "localhost")
	process.stopProcess()
	base, httpsPort, process := start()
	defer process.stopProcess()
	// Only the disposable loopback server uses its generated self-signed certificate.
	tlsClient := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{ServerName: "headers-one.test", InsecureSkipVerify: true}}}
	defer tlsClient.CloseIdleConnections()
	etags := map[string]string{}
	request := func(client *http.Client, target, host, method, path, encoding, etag string, status int, wantCSP string) {
		t.Helper()
		req, err := http.NewRequest(method, target+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		req.Header.Set("Accept-Encoding", encoding)
		req.Header.Set("If-None-Match", etag)
		req.Header.Set("X-Forwarded-Proto", "https")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s %s %s: status=%d, want %d", method, host, path, response.StatusCode, status)
		}
		if response.Header.Get("Content-Security-Policy") != wantCSP {
			t.Fatalf("%s%s CSP=%q, want %q", host, path, response.Header.Get("Content-Security-Policy"), wantCSP)
		}
		if (host == "headers-one.test" || strings.HasPrefix(path, "/headers-one.test/")) && wantCSP != "" {
			if response.Header.Get("X-Content-Type-Options") != "nosniff" || response.Header.Get("Referrer-Policy") != policy["Referrer-Policy"] {
				t.Fatalf("lost policy: %v", response.Header)
			}
			if (response.Header.Get("Strict-Transport-Security") != "") != strings.HasPrefix(target, "https:") {
				t.Fatalf("wrong HSTS transport: %v", response.Header)
			}
		} else if response.Header.Get("X-Content-Type-Options") != "" {
			t.Fatal("policy leaked between Sites or to API")
		}
		if status == 200 && method == http.MethodGet {
			var body io.Reader = response.Body
			if encoding == "gzip" {
				if response.Header.Get("Content-Encoding") != "gzip" {
					t.Fatal("gzip response missing")
				}
				gz, err := gzip.NewReader(body)
				if err != nil {
					t.Fatal(err)
				}
				defer gz.Close()
				body = gz
			}
			data, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			if wantCSP != "" && !strings.Contains(string(data), "customer frontend") {
				t.Fatal("policy request lost site content")
			}
		}
		etags[path+encoding] = response.Header.Get("ETag")
	}
	csp := policy["Content-Security-Policy"].(string)
	for _, tc := range []struct {
		path, encoding, key string
		status              int
	}{
		{"/", "", "", 200}, {"/", "", "", 200}, {"/", "gzip", "", 200}, {"/", "gzip", "/gzip", 304},
		{"/app.js", "", "", 200}, {"/app.js", "gzip", "", 200}, {"/app.js", "gzip", "/app.jsgzip", 304},
		{"/client/route", "", "", 200}, {"/client/route", "gzip", "", 200}, {"/blocked.js", "", "", 502},
	} {
		request(client, base, "headers-one.test", http.MethodGet, tc.path, tc.encoding, etags[tc.key], tc.status, csp)
	}
	request(client, base, "other-host.test", http.MethodGet, "/headers-one.test/app.js", "gzip", "", 200, csp)
	rangeRequest, _ := http.NewRequest(http.MethodGet, base+"/app.js", nil)
	rangeRequest.Host = "headers-one.test"
	rangeRequest.Header.Set("Range", "bytes=0-9")
	rangeRequest.Header.Set("Accept-Encoding", "gzip")
	rangeResponse, err := client.Do(rangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	rangeBody, err := io.ReadAll(rangeResponse.Body)
	rangeResponse.Body.Close()
	if err != nil || rangeResponse.StatusCode != http.StatusPartialContent || len(rangeBody) != 10 ||
		rangeResponse.Header.Get("Content-Security-Policy") != csp || rangeResponse.Header.Get("Content-Encoding") != "" {
		t.Fatalf("range response lost representation or policy: %d %v %v", rangeResponse.StatusCode, rangeResponse.Header, err)
	}
	request(client, base, "headers-one.test", http.MethodHead, "/", "", "", 200, csp)
	request(client, base, "headers-two.test", http.MethodGet, "/", "", "", 200, "default-src 'none'")
	request(client, base, "headers-empty.test", http.MethodGet, "/", "", "", 200, "")
	request(client, base, "headers-one.test", http.MethodGet, "/api/site/"+siteID, "", "", http.StatusForbidden, "")
	request(tlsClient, fmt.Sprintf("https://127.0.0.1:%d", httpsPort), "headers-one.test", http.MethodGet, "/", "gzip", "", 200, csp)
	// Clearing the same resource policy takes effect through the same restart path.
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/site/"+siteID, admin,
		accessGroupsE2ERecordPayload("site", siteID, map[string]interface{}{"headers": map[string]interface{}{}}), http.StatusOK)
	process.stopProcess()
	base, _, process = start()
	defer process.stopProcess()
	req, _ := http.NewRequest(http.MethodGet, base+"/", nil)
	req.Host = "headers-one.test"
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Security-Policy") != "" || response.Header.Get("X-Content-Type-Options") != "" {
		t.Fatalf("cleared policy remained: %d %v", response.StatusCode, response.Header)
	}
}
