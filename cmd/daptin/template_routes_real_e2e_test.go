package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
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

func TestTemplateRouteStartupRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []struct{ name, driver, dsn string }{{"sqlite", "sqlite3", ""}, {"postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN")}} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.driver == "postgres" && backend.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
			}
			root := t.TempDir()
			assets := filepath.Join(root, "assets")
			if err := os.MkdirAll(assets, 0700); err != nil {
				t.Fatal(err)
			}
			staticBody := "<!doctype html><body>static Site shell</body>"
			if err := os.WriteFile(filepath.Join(assets, "index.html"), []byte(staticBody), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(assets, "app.js"), []byte("static asset"), 0600); err != nil {
				t.Fatal(err)
			}
			used := map[int]bool{}
			port, httpsPort := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
			base := fmt.Sprintf("http://127.0.0.1:%d", port)
			dsn := backend.dsn
			if dsn == "" {
				dsn = filepath.Join(root, "templates.db")
			}
			options := transportE2EDaptinOptions{databaseType: backend.driver, connectionString: dsn, olricPort: freeTransportE2EPortPair(t, used)}
			process := startTransportE2EDaptin(t, port, httpsPort, base, options)
			defer func() { process.stopProcess() }()
			client := &http.Client{Timeout: 20 * time.Second}
			admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
			create := func(kind string, attrs map[string]interface{}) string {
				t.Helper()
				return accessGroupsE2ECreateRecord(t, client, base, admin, kind, attrs)
			}
			patch := func(kind, id string, attrs map[string]interface{}) {
				t.Helper()
				accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/"+kind+"/"+id, admin, accessGroupsE2ERecordPayload(kind, id, attrs), http.StatusOK)
			}
			restart := func() {
				t.Helper()
				process.stopProcess()
				process = startTransportE2EDaptin(t, port, httpsPort, base, options)
			}
			page := func(host, method, path, encoding, etag, token string, want int) ([]byte, http.Header) {
				t.Helper()
				var payload io.Reader
				if method == http.MethodPost {
					payload = strings.NewReader(`{"attributes":{}}`)
				}
				request, err := http.NewRequest(method, base+path, payload)
				if err != nil {
					t.Fatal(err)
				}
				request.Host = host
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept-Encoding", encoding)
				if etag != "" {
					request.Header.Set("If-None-Match", etag)
				}
				if token != "" {
					request.Header.Set("Authorization", "Bearer "+token)
				}
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				data, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != want {
					t.Fatalf("%s %s on %s: HTTP %d, want %d: %s", method, path, host, response.StatusCode, want, data)
				}
				if response.Header.Get("Content-Encoding") == "gzip" && method != http.MethodHead && want != http.StatusNotModified {
					reader, err := gzip.NewReader(bytes.NewReader(data))
					if err != nil {
						t.Fatal(err)
					}
					data, err = io.ReadAll(reader)
					reader.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				return data, response.Header.Clone()
			}
			store := create("cloud_store", map[string]interface{}{"name": "template-route-store", "store_type": "local", "store_provider": "local", "root_path": assets, "store_parameters": "{}"})
			for _, site := range []struct {
				host       string
				permission int64
			}{{"template-root.test", int64(auth.UserCRUD | auth.UserExecute | auth.GuestExecute)}, {"template-private.test", 0}} {
				id := create("site", map[string]interface{}{"name": site.host, "hostname": site.host, "path": "", "enable": true, "site_type": "static"})
				accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/site/"+id, admin, map[string]interface{}{"data": map[string]interface{}{"type": "site", "id": id, "attributes": map[string]interface{}{"permission": site.permission}, "relationships": map[string]interface{}{"cloud_store_id": map[string]interface{}{"data": map[string]interface{}{"type": "cloud_store", "id": store}}}}}, http.StatusOK)
			}
			template := func(name, body, patterns string) string {
				t.Helper()
				return create("template", map[string]interface{}{"name": name, "content": body, "mime_type": "text/html", "url_pattern": patterns, "headers": `{"X-Template-Route":"yes","X-Content-Type-Options":"nosniff"}`, "action_config": "{}", "cache_config": `{"enable":true,"etag_strategy":"strong"}`, "permission": 34})
			}
			body := "<!doctype html><body>routed root template" + strings.Repeat(" template bytes", 600) + "</body>"
			rootTemplate := template("root-template", body, `[]`)
			healthy := template("healthy-template", "healthy routed template", `[]`)
			invalid := template("invalid-template", "conflicting template", `[]`)
			// Exercise the renderer independently while HTTP route compilation is disabled.
			world := accessGroupsE2EFindResourceID(t, client, base, admin, "world", "table_name", "world")
			definition := map[string]interface{}{"Name": "preview_root_template", "OnType": "world", "InstanceOptional": true, "InFields": []interface{}{}, "OutFields": []interface{}{
				map[string]interface{}{"Type": "template.render", "Method": "EXECUTE", "Reference": "rendered", "SkipInResponse": true, "Attributes": map[string]interface{}{"template": "root-template"}},
				map[string]interface{}{"Type": "response.create", "Method": "EXECUTE", "Attributes": map[string]interface{}{"response_type": "render", "content": "$rendered.content", "mime_type": "$rendered.mime_type", "headers": "$rendered.headers"}},
			}}
			encoded, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			create("action", map[string]interface{}{"action_name": "preview_root_template", "label": "Preview root template", "world_id": world, "instance_optional": true, "action_schema": string(encoded)})
			restart()
			data, _ := page("template-root.test", "GET", "/", "identity", "", "", 200)
			if string(data) != staticBody {
				t.Fatalf("static root=%q", data)
			}
			dashboard, _ := page("unmatched.test", "GET", "/", "identity", "", "", 200)
			patch("template", rootTemplate, map[string]interface{}{"url_pattern": `["/"]`})
			patch("template", healthy, map[string]interface{}{"url_pattern": `["/healthy-template"]`})
			restart()
			assertMounted := func() {
				t.Helper()
				for _, host := range []string{"template-root.test", "unmatched.test"} {
					data, headers := page(host, "GET", "/", "identity", "", "", 200)
					if string(data) != body || headers.Get("X-Template-Route") != "yes" || headers.Get("X-Content-Type-Options") != "nosniff" {
						t.Fatalf("root template on %s not mounted with headers", host)
					}
				}
			}
			assertMounted()
			_, headers := page("template-root.test", "GET", "/", "gzip", "", "", 200)
			if headers.Get("Content-Encoding") != "gzip" || headers.Get("ETag") == "" {
				t.Fatal("template gzip/ETag contract lost")
			}
			page("template-root.test", "GET", "/", "gzip", headers.Get("ETag"), "", 304)
			data, headers = page("template-root.test", "HEAD", "/", "identity", "", "", 200)
			if len(data) != 0 || headers.Get("X-Template-Route") != "yes" {
				t.Fatal("template HEAD contract lost")
			}
			page("template-private.test", "GET", "/", "identity", "", "", 401)
			data, _ = page("template-private.test", "GET", "/", "identity", "", admin, 200)
			if string(data) != body {
				t.Fatal("authorized Site template unavailable")
			}
			data, _ = page("unmatched.test", "GET", "/template-root.test/", "identity", "", "", 200)
			if string(data) != body {
				t.Fatal("path-based Site routing lost root template")
			}
			page("template-root.test", "GET", "/meta", "identity", "", "", 200)
			t.Log("Root template works globally with headers, gzip, validators, HEAD and existing Site permissions")
			for _, test := range []struct{ name, patterns string }{{"built-in endpoint", `["/meta"]`}, {"root catch-all", `["/*path"]`}, {"duplicate root", `["/"]`}, {"wrong JSON shape", `{}`}, {"malformed wildcard", `["/bad/*path/more"]`}, {"duplicate within row", `["/same","/same"]`}} {
				// These writes intentionally succeed: startup must handle persisted data
				// regardless of which writer produced it. There is no write validator.
				patch("template", invalid, map[string]interface{}{"url_pattern": test.patterns})
				assertMounted() // route changes are activated at restart, not per request
				restart()
				if !strings.Contains(process.logs.String(), "Template HTTP routes not mounted") {
					t.Fatalf("%s: missing configuration diagnostic", test.name)
				}
				if !strings.Contains(process.logs.String(), invalid) && !strings.Contains(process.logs.String(), rootTemplate) {
					t.Fatalf("%s: diagnostic lacks template reference", test.name)
				}
				data, headers = page("template-root.test", "GET", "/", "identity", "", "", 200)
				if string(data) != staticBody || headers.Get("X-Template-Route") != "" {
					t.Fatalf("%s: partial template routes mounted", test.name)
				}
				data, _ = page("unmatched.test", "GET", "/", "identity", "", "", 200)
				if !bytes.Equal(data, dashboard) {
					t.Fatalf("%s: dashboard root fallback changed", test.name)
				}
				data, headers = page("template-root.test", "GET", "/healthy-template", "identity", "", "", 200)
				if string(data) != staticBody || headers.Get("X-Template-Route") != "" {
					t.Fatalf("%s: valid template leaked from rejected batch", test.name)
				}
				page("template-root.test", "GET", "/app.js", "identity", "", "", 200)
				page("unmatched.test", "GET", "/meta", "identity", "", "", 200)
				accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/template/"+invalid, admin, nil, http.StatusOK)
				data, _ = page("unmatched.test", "POST", "/action/world/preview_root_template", "identity", "", admin, 200)
				if string(data) != body {
					t.Fatalf("%s: routed configuration disabled action renderer", test.name)
				}
				patch("template", invalid, map[string]interface{}{"url_pattern": `[]`})
				restart()
				assertMounted()
				t.Logf("%s: ready with full template set unmounted, APIs and fallbacks available; API repair plus restart restored routes", test.name)
			}
		})
	}
}
