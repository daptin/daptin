package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoutedTemplateCacheHeadersRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	usedPorts := map[int]bool{}
	databasePath := filepath.Join(t.TempDir(), "template-cache.db")
	firstPort := freeTransportE2EPort(t, usedPorts)
	firstHTTPSPort := freeTransportE2EPort(t, usedPorts)
	firstOlricPort := freeTransportE2EPortPair(t, usedPorts)
	firstURL := fmt.Sprintf("http://127.0.0.1:%d", firstPort)
	first := startTransportE2EDaptin(t, firstPort, firstHTTPSPort, firstURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, olricPort: firstOlricPort,
	})
	defer first.stopProcess()
	client := &http.Client{Timeout: 20 * time.Second}
	token := accessGroupsE2ESignupSigninAdmin(t, client, firstURL)
	accessGroupsE2ECreateRecord(t, client, firstURL, token, "template", map[string]interface{}{
		"name": "audit_cache_route", "content": "<p>{{.q}}</p>",
		"mime_type": "text/html", "url_pattern": `["/audit-cache-route"]`,
		"headers":       `{"X-Audit-Template":"yes","Content-Security-Policy":"default-src 'self'"}`,
		"action_config": "{}",
		"cache_config":  `{"enable":true,"max_age":60,"revalidate":true,"etag_strategy":"strong","enable_in_memory_cache":true,"vary_by_query_params":["q"],"custom_headers":{"X-Frame-Options":"DENY"}}`,
	})
	accessGroupsE2ECreateRecord(t, client, firstURL, token, "template", map[string]interface{}{
		"name": "audit_no_store_route", "content": "no-store response",
		"mime_type": "text/plain", "url_pattern": `["/audit-no-store-route"]`,
		"headers": `{"Cache-Control":"no-store"}`, "action_config": "{}",
		"cache_config": `{"enable":true,"max_age":60,"enable_in_memory_cache":true}`,
	})
	for _, route := range []struct {
		name, path, content, mime, headers, config string
	}{
		{"audit_header_vary", "/audit-header-vary", "header variant", "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true,"vary_by_headers":["X-Audit-Variant"]}`},
		{"audit_gzip", "/audit-gzip", strings.Repeat("cache-body-", 600), "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true,"etag_strategy":"strong"}`},
		{"audit_host_gzip", "/audit-host-gzip", strings.Repeat("host-cache-body-", 600), "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true,"etag_strategy":"strong"}`},
		{"audit_expiry", "/audit-expiry", "expiring response", "text/plain", `{}`, `{"enable":true,"max_age":1,"in_memory_cache_ttl":1,"enable_in_memory_cache":true}`},
		{"audit_no_cache", "/audit-no-cache", "revalidated response", "text/plain", `{}`, `{"enable":true,"no_cache":true,"enable_in_memory_cache":true}`},
		{"audit_private", "/audit-private", "private response", "text/plain", `{}`, `{"enable":true,"private":true,"max_age":60,"enable_in_memory_cache":true}`},
		{"audit_weak", "/audit-weak", "weak response", "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true,"etag_strategy":"weak"}`},
		{"audit_no_etag", "/audit-no-etag", "no validator", "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true,"etag_strategy":"none"}`},
		{"audit_disposition", "/audit-disposition", "download response", "text/plain", `{"Content-Disposition":"attachment; filename=\"report.txt\""}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true}`},
		{"audit_post", "/audit-post", "method response", "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true}`},
		{"audit_head", "/audit-head", "head response", "text/plain", `{}`, `{"enable":true,"max_age":60,"enable_in_memory_cache":true}`},
		{"audit_disabled", "/audit-disabled", "uncached response", "text/plain", `{}`, `{"enable":false,"enable_in_memory_cache":true}`},
	} {
		accessGroupsE2ECreateRecord(t, client, firstURL, token, "template", map[string]interface{}{
			"name": route.name, "content": route.content, "mime_type": route.mime,
			"url_pattern": fmt.Sprintf("[%q]", route.path), "headers": route.headers,
			"action_config": "{}", "cache_config": route.config,
		})
	}
	accessGroupsE2ECreateRecord(t, client, firstURL, token, "template", map[string]interface{}{
		"name": "audit_cookie_route", "content": "cookie response",
		"mime_type": "text/plain", "url_pattern": `["/audit-cookie-route"]`,
		"headers": `{"Set-Cookie":"audit=1"}`, "action_config": "{}",
		"cache_config": `{"enable":true,"max_age":60,"enable_in_memory_cache":true}`,
	})
	storeID := accessGroupsE2ECreateRecord(t, client, firstURL, token, "cloud_store", map[string]interface{}{
		"name": "template-cache-site-store", "store_type": "local", "store_provider": "local",
		"root_path": t.TempDir(), "store_parameters": "{}",
	})
	siteID := accessGroupsE2ECreateRecord(t, client, firstURL, token, "site", map[string]interface{}{
		"name": "template-cache-site", "hostname": "cache.example.test", "path": "cache-site",
		"enable": true, "site_type": "static",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, firstURL+"/api/site/"+siteID, token, map[string]interface{}{
		"data": map[string]interface{}{
			"type": "site", "id": siteID,
			"relationships": map[string]interface{}{"cloud_store_id": map[string]interface{}{
				"data": map[string]interface{}{"type": "cloud_store", "id": storeID},
			}},
		},
	}, http.StatusOK)
	first.stopProcess()

	secondPort := freeTransportE2EPort(t, usedPorts)
	secondHTTPSPort := freeTransportE2EPort(t, usedPorts)
	secondOlricPort := freeTransportE2EPortPair(t, usedPorts)
	secondURL := fmt.Sprintf("http://127.0.0.1:%d", secondPort)
	second := startTransportE2EDaptin(t, secondPort, secondHTTPSPort, secondURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, olricPort: secondOlricPort,
	})
	defer second.stopProcess()

	type routeResult struct {
		status int
		header http.Header
		body   []byte
	}
	fetchAt := func(baseURL, method, path string, headers map[string]string) routeResult {
		t.Helper()
		req, err := http.NewRequest(method, baseURL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", "identity")
		for key, value := range headers {
			if strings.EqualFold(key, "Host") {
				req.Host = value
			} else {
				req.Header.Set(key, value)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return routeResult{resp.StatusCode, resp.Header, body}
	}
	fetch := func(method, path string, headers map[string]string) routeResult {
		return fetchAt(secondURL, method, path, headers)
	}

	request := func(validator string) (int, http.Header, string) {
		headers := map[string]string{}
		if validator != "" {
			headers["If-None-Match"] = validator
		}
		result := fetch(http.MethodGet, "/audit-cache-route?q=one", headers)
		return result.status, result.header, string(result.body)
	}

	firstStatus, firstHeaders, firstBody := request("")
	if firstStatus != http.StatusOK || firstHeaders.Get("X-Cache") != "MISS" {
		t.Fatalf("first response: status=%d headers=%#v body=%q", firstStatus, firstHeaders, firstBody)
	}
	etag := firstHeaders.Get("ETag")
	if etag == "" {
		t.Fatal("first response has no ETag")
	}
	t.Run("expires matches max-age", func(t *testing.T) {
		responseDate, dateErr := time.Parse(http.TimeFormat, firstHeaders.Get("Date"))
		expires, expiresErr := time.Parse(http.TimeFormat, firstHeaders.Get("Expires"))
		if dateErr != nil || expiresErr != nil {
			t.Fatalf("invalid Date/Expires headers: date=%q (%v) expires=%q (%v)", firstHeaders.Get("Date"), dateErr, firstHeaders.Get("Expires"), expiresErr)
		}
		if delta := expires.Sub(responseDate); delta < 55*time.Second || delta > 65*time.Second {
			t.Errorf("Expires is %s after Date, want about 60s", delta)
		}
	})
	for _, tc := range []struct {
		name, validator, cache, body string
		status                       int
	}{
		{name: "hit", cache: "HIT", body: firstBody, status: http.StatusOK},
		{name: "not modified", validator: etag, cache: "HIT", status: http.StatusNotModified},
		{name: "nonmatching", validator: `"other"`, cache: "HIT", body: firstBody, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, headers, body := request(tc.validator)
			if status != tc.status || headers.Get("X-Cache") != tc.cache || body != tc.body {
				t.Fatalf("response: status=%d headers=%#v body=%q", status, headers, body)
			}
			for _, key := range []string{"Cache-Control", "Expires", "ETag", "X-Audit-Template", "Content-Security-Policy", "X-Frame-Options", "X-Vary-By-Query-Params"} {
				if got, want := headers.Get(key), firstHeaders.Get(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if headers.Get("Content-Disposition") != "" || !strings.Contains(headers.Get("Cache-Control"), "max-age=60") {
				t.Errorf("unexpected cache headers: %#v", headers)
			}
		})
	}
	t.Run("query variants", func(t *testing.T) {
		other := fetch(http.MethodGet, "/audit-cache-route?q=two", nil)
		if other.status != http.StatusOK || other.header.Get("X-Cache") != "MISS" || string(other.body) == firstBody {
			t.Fatalf("different query reused response: %#v body=%q", other.header, other.body)
		}
		otherHit := fetch(http.MethodGet, "/audit-cache-route?q=two", nil)
		if otherHit.header.Get("X-Cache") != "HIT" || string(otherHit.body) != string(other.body) {
			t.Fatalf("second query hit: %#v body=%q", otherHit.header, otherHit.body)
		}
	})
	t.Run("header variants", func(t *testing.T) {
		for index, variant := range []string{"A", "A", "B", "B"} {
			result := fetch(http.MethodGet, "/audit-header-vary", map[string]string{"X-Audit-Variant": variant})
			want := "MISS"
			if index%2 == 1 {
				want = "HIT"
			}
			if result.status != http.StatusOK || result.header.Get("X-Cache") != want || !strings.Contains(result.header.Get("Vary"), "X-Audit-Variant") {
				t.Fatalf("header variant %q: status=%d headers=%#v", variant, result.status, result.header)
			}
		}
	})
	t.Run("last modified validator", func(t *testing.T) {
		result := fetch(http.MethodGet, "/audit-cache-route?q=one", map[string]string{"If-Modified-Since": firstHeaders.Get("Last-Modified")})
		if result.status != http.StatusNotModified || result.header.Get("ETag") != etag || result.header.Get("X-Audit-Template") != "yes" {
			t.Fatalf("If-Modified-Since response: status=%d headers=%#v", result.status, result.header)
		}
	})
	t.Run("weak and absent validators", func(t *testing.T) {
		weak := fetch(http.MethodGet, "/audit-weak", nil)
		if !strings.HasPrefix(weak.header.Get("ETag"), "W/") {
			t.Fatalf("weak ETag missing: %#v", weak.header)
		}
		strongForm := strings.TrimPrefix(weak.header.Get("ETag"), "W/")
		matched := fetch(http.MethodGet, "/audit-weak", map[string]string{"If-None-Match": strongForm})
		if matched.status != http.StatusNotModified || matched.header.Get("ETag") != weak.header.Get("ETag") {
			t.Fatalf("weak comparison: status=%d headers=%#v", matched.status, matched.header)
		}
		noTag := fetch(http.MethodGet, "/audit-no-etag", nil)
		bogus := fetch(http.MethodGet, "/audit-no-etag", map[string]string{"If-None-Match": `"made-up"`})
		if noTag.header.Get("ETag") != "" || bogus.status != http.StatusOK || bogus.header.Get("ETag") != "" {
			t.Fatalf("ETag disabled: first=%#v conditional=%#v", noTag.header, bogus.header)
		}
	})
	t.Run("declared disposition", func(t *testing.T) {
		initial := fetch(http.MethodGet, "/audit-disposition", nil)
		hit := fetch(http.MethodGet, "/audit-disposition", nil)
		conditional := fetch(http.MethodGet, "/audit-disposition", map[string]string{"If-None-Match": initial.header.Get("ETag")})
		for _, result := range []routeResult{initial, hit, conditional} {
			if result.header.Get("Content-Disposition") != `attachment; filename="report.txt"` {
				t.Fatalf("declared disposition lost: %#v", result.header)
			}
		}
		if initial.header.Get("X-Cache") != "MISS" || hit.header.Get("X-Cache") != "HIT" || conditional.status != http.StatusNotModified {
			t.Fatalf("disposition path: initial=%#v hit=%#v 304=%#v", initial.header, hit.header, conditional.header)
		}
	})
	t.Run("gzip and identity representations", func(t *testing.T) {
		identity := fetch(http.MethodGet, "/audit-gzip", nil)
		compressed := fetch(http.MethodGet, "/audit-gzip", map[string]string{"Accept-Encoding": "gzip"})
		if identity.status != http.StatusOK || compressed.status != http.StatusOK || compressed.header.Get("Content-Encoding") != "gzip" || !strings.Contains(compressed.header.Get("Vary"), "Accept-Encoding") || compressed.header.Get("ETag") == identity.header.Get("ETag") {
			t.Fatalf("gzip representation headers: identity=%#v gzip=%#v", identity.header, compressed.header)
		}
		reader, err := gzip.NewReader(strings.NewReader(string(compressed.body)))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil || string(decoded) != string(identity.body) {
			t.Errorf("gzip body mismatch: err=%v identityLen=%d decodedLen=%d identityPrefix=%q decodedPrefix=%q", err, len(identity.body), len(decoded), identity.body[:min(len(identity.body), 80)], decoded[:min(len(decoded), 80)])
			if secondReader, secondErr := gzip.NewReader(bytes.NewReader(decoded)); secondErr == nil {
				secondDecoded, readErr := io.ReadAll(secondReader)
				_ = secondReader.Close()
				t.Logf("second decompression: err=%v matches identity=%t", readErr, bytes.Equal(secondDecoded, identity.body))
			}
		}
		validated := fetch(http.MethodGet, "/audit-gzip", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": compressed.header.Get("ETag")})
		if validated.status != http.StatusNotModified || validated.header.Get("ETag") != compressed.header.Get("ETag") {
			t.Fatalf("gzip validator: status=%d headers=%#v", validated.status, validated.header)
		}
		refused := fetch(http.MethodGet, "/audit-gzip", map[string]string{"Accept-Encoding": "gzip;q=0"})
		if refused.header.Get("Content-Encoding") != "" || string(refused.body) != string(identity.body) {
			t.Errorf("gzip;q=0 served compressed response: headers=%#v body=%q", refused.header, refused.body[:min(len(refused.body), 48)])
		}
	})
	t.Run("host router compresses once", func(t *testing.T) {
		host := map[string]string{"Host": "cache.example.test", "Authorization": "Bearer " + token}
		identity := fetch(http.MethodGet, "/audit-host-gzip", host)
		compressedHeaders := map[string]string{"Host": host["Host"], "Authorization": host["Authorization"], "Accept-Encoding": "gzip"}
		compressed := fetch(http.MethodGet, "/audit-host-gzip", compressedHeaders)
		if identity.status != http.StatusOK || identity.header.Get("X-Cache") != "MISS" || compressed.status != http.StatusOK || compressed.header.Get("X-Cache") != "HIT" || compressed.header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("host route: identity=%d %#v gzip=%d %#v", identity.status, identity.header, compressed.status, compressed.header)
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed.body))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil || !bytes.Equal(decoded, identity.body) {
			t.Fatalf("host route gzip body differs after one decompression: %v", err)
		}
		validated := fetch(http.MethodGet, "/audit-host-gzip", map[string]string{"Host": host["Host"], "Authorization": host["Authorization"], "Accept-Encoding": "gzip", "If-None-Match": compressed.header.Get("ETag")})
		if validated.status != http.StatusNotModified || validated.header.Get("ETag") != compressed.header.Get("ETag") {
			t.Fatalf("host route validator: status=%d headers=%#v", validated.status, validated.header)
		}
		refused := fetch(http.MethodGet, "/audit-host-gzip", map[string]string{"Host": host["Host"], "Authorization": host["Authorization"], "Accept-Encoding": "gzip;q=0"})
		if refused.header.Get("Content-Encoding") != "" || !bytes.Equal(refused.body, identity.body) {
			t.Fatalf("host route ignored gzip;q=0: %#v", refused.header)
		}
		defaultRouter := fetch(http.MethodGet, "/audit-host-gzip", map[string]string{"Accept-Encoding": "gzip"})
		if defaultRouter.header.Get("X-Cache") != "HIT" || defaultRouter.header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("shared entry on default router: %#v", defaultRouter.header)
		}
		defaultReader, err := gzip.NewReader(bytes.NewReader(defaultRouter.body))
		if err != nil {
			t.Fatal(err)
		}
		defaultDecoded, err := io.ReadAll(defaultReader)
		_ = defaultReader.Close()
		if err != nil || !bytes.Equal(defaultDecoded, identity.body) {
			t.Fatalf("shared entry double-compressed on default router: %v", err)
		}
	})
	t.Run("expiry", func(t *testing.T) {
		initial := fetch(http.MethodGet, "/audit-expiry", nil)
		hit := fetch(http.MethodGet, "/audit-expiry", nil)
		time.Sleep(1500 * time.Millisecond)
		expired := fetch(http.MethodGet, "/audit-expiry", nil)
		if initial.header.Get("X-Cache") != "MISS" || hit.header.Get("X-Cache") != "HIT" || expired.header.Get("X-Cache") != "MISS" {
			t.Fatalf("expiry path: first=%#v hit=%#v expired=%#v", initial.header, hit.header, expired.header)
		}
	})
	t.Run("no-cache and private", func(t *testing.T) {
		for _, path := range []string{"/audit-no-cache", "/audit-private", "/audit-disabled"} {
			for attempt := 0; attempt < 2; attempt++ {
				result := fetch(http.MethodGet, path, nil)
				if result.status != http.StatusOK || result.header.Get("X-Cache") != "MISS" {
					t.Fatalf("%s attempt %d: status=%d headers=%#v", path, attempt, result.status, result.header)
				}
			}
		}
		first := fetch(http.MethodGet, "/audit-no-cache", nil)
		conditional := fetch(http.MethodGet, "/audit-no-cache", map[string]string{"If-None-Match": first.header.Get("ETag")})
		if conditional.status != http.StatusNotModified || conditional.header.Get("X-Cache") != "MISS" {
			t.Fatalf("no-cache did not re-render before validation: status=%d headers=%#v", conditional.status, conditional.header)
		}
	})
	t.Run("HEAD shares the rendered representation", func(t *testing.T) {
		head := fetch(http.MethodHead, "/audit-head", nil)
		get := fetch(http.MethodGet, "/audit-head", nil)
		if head.status != http.StatusOK || len(head.body) != 0 || head.header.Get("X-Cache") != "MISS" || get.header.Get("X-Cache") != "HIT" || string(get.body) != "head response" || head.header.Get("ETag") != get.header.Get("ETag") {
			t.Fatalf("HEAD/GET mismatch: head=%#v get=%#v body=%q", head.header, get.header, get.body)
		}
		conditional := fetch(http.MethodHead, "/audit-head", map[string]string{"If-None-Match": head.header.Get("ETag")})
		if conditional.status != http.StatusNotModified || len(conditional.body) != 0 {
			t.Fatalf("conditional HEAD: status=%d headers=%#v body=%q", conditional.status, conditional.header, conditional.body)
		}
	})
	t.Run("unsafe methods do not hit cache", func(t *testing.T) {
		for attempt := 0; attempt < 2; attempt++ {
			result := fetch(http.MethodPost, "/audit-post", nil)
			if result.status != http.StatusOK || result.header.Get("X-Cache") != "MISS" {
				t.Fatalf("POST attempt %d: status=%d headers=%#v", attempt, result.status, result.header)
			}
		}
		result := fetch(http.MethodGet, "/audit-post", nil)
		if result.header.Get("X-Cache") != "MISS" {
			t.Fatalf("POST populated GET cache: %#v", result.header)
		}
	})
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := client.Get(secondURL + "/audit-no-store-route")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Cache") != "MISS" || resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("no-store response %d: status=%d headers=%#v", attempt, resp.StatusCode, resp.Header)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := client.Get(secondURL + "/audit-cookie-route")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Cache") != "MISS" || resp.Header.Get("Set-Cookie") != "audit=1" {
			t.Fatalf("cookie response %d: status=%d headers=%#v", attempt, resp.StatusCode, resp.Header)
		}
	}
	configRequest, err := http.NewRequest(http.MethodPost, secondURL+"/_config/backend/gzip.enable", strings.NewReader("false"))
	if err != nil {
		t.Fatal(err)
	}
	configRequest.Header.Set("Authorization", "Bearer "+token)
	configResponse, err := client.Do(configRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, configResponse.Body)
	_ = configResponse.Body.Close()
	if configResponse.StatusCode != http.StatusOK {
		t.Fatalf("disable gzip returned %d", configResponse.StatusCode)
	}
	second.stopProcess()
	thirdPort := freeTransportE2EPort(t, usedPorts)
	thirdHTTPSPort := freeTransportE2EPort(t, usedPorts)
	thirdOlricPort := freeTransportE2EPortPair(t, usedPorts)
	thirdURL := fmt.Sprintf("http://127.0.0.1:%d", thirdPort)
	third := startTransportE2EDaptin(t, thirdPort, thirdHTTPSPort, thirdURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, olricPort: thirdOlricPort,
	})
	defer third.stopProcess()
	t.Run("gzip disabled on both routers", func(t *testing.T) {
		for _, tc := range []struct {
			path    string
			headers map[string]string
		}{
			{path: "/audit-gzip", headers: map[string]string{"Accept-Encoding": "gzip"}},
			{path: "/audit-host-gzip", headers: map[string]string{"Host": "cache.example.test", "Authorization": "Bearer " + token, "Accept-Encoding": "gzip"}},
		} {
			first := fetchAt(thirdURL, http.MethodGet, tc.path, tc.headers)
			hit := fetchAt(thirdURL, http.MethodGet, tc.path, tc.headers)
			if first.status != http.StatusOK || hit.status != http.StatusOK || first.header.Get("X-Cache") != "MISS" || hit.header.Get("X-Cache") != "HIT" || first.header.Get("Content-Encoding") != "" || hit.header.Get("Content-Encoding") != "" || !bytes.Equal(first.body, hit.body) {
				t.Fatalf("gzip disabled %s: first=%d %#v hit=%d %#v", tc.path, first.status, first.header, hit.status, hit.header)
			}
		}
	})
}
