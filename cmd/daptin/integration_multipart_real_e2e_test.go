package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Exercise installation, discovery, generated action evaluation and the actual
// provider request together, including Box's attributes-before-file requirement.
func TestIntegrationMultipartRealE2E(t *testing.T) {
	requireRealE2E(t)
	payload := []byte{0, 255, 'u', 'p', 'l', 'o', 'a', 'd', '\n'}
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		reject := func(err interface{}) { t.Error(err); http.Error(w, fmt.Sprint(err), http.StatusBadRequest) }
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer owner-token" {
			reject("incorrect provider method or credential")
			return
		}
		switch r.URL.Path {
		case "/operation/files/content/operation", "/path/files/content/path", "/root/files/content/root":
			reader, err := r.MultipartReader()
			if err != nil {
				reject(err)
				return
			}
			part, err := reader.NextPart()
			if err != nil {
				reject(err)
				return
			}
			if part.FormName() != "attributes" || part.Header.Get("Content-Type") != "application/json" {
				reject("attributes must be the first JSON part")
				return
			}
			var attributes struct {
				Name   string `json:"name"`
				Parent struct {
					ID string `json:"id"`
				} `json:"parent"`
			}
			if err := json.NewDecoder(part).Decode(&attributes); err != nil || attributes.Name != "probe.bin" || attributes.Parent.ID != "0" {
				reject("incorrect upload attributes")
				return
			}
			part, err = reader.NextPart()
			if err != nil {
				reject(err)
				return
			}
			content, err := io.ReadAll(part)
			if err != nil || part.FormName() != "file" || part.FileName() != "probe.bin" || part.Header.Get("Content-Type") != "application/octet-stream" || !bytes.Equal(content, payload) {
				reject("incorrect binary file part")
				return
			}
			if _, err := reader.NextPart(); err != io.EOF {
				reject("unexpected extra multipart part")
				return
			}
		case "/root/json":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["message"] != "hello" {
				reject("JSON body changed")
				return
			}
		case "/root/form":
			if err := r.ParseForm(); err != nil || r.PostForm.Get("message") != "hello" {
				reject(fmt.Sprintf("URL-encoded body changed: content-type=%q form=%v", r.Header.Get("Content-Type"), r.PostForm))
				return
			}
		default:
			reject("wrong OpenAPI server selected: " + r.URL.Path)
			return
		}
		transportE2EWriteJSON(w, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	admin := fixture.SignupAdmin(t)
	client, base := fixture.Client, fixture.URL
	credential := transportE2ECreateCredential(t, client, base, admin)
	paths := map[string]interface{}{}
	for _, level := range []string{"operation", "path", "root"} {
		operation := map[string]interface{}{
			"operationId": "upload" + level,
			"responses":   transportE2EJSONResponses(),
			"requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{
				"multipart/form-data": map[string]interface{}{"schema": map[string]interface{}{
					"type": "object", "required": []string{"attributes", "file"},
					"properties": map[string]interface{}{
						"attributes": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}, "parent": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"id": map[string]interface{}{"type": "string"}}}}},
						"file":       map[string]interface{}{"type": "string", "format": "binary"},
					},
				}},
			}},
		}
		item := map[string]interface{}{"post": operation}
		// Different prefixes make selection observable even on one loopback host.
		if level != "root" {
			item["servers"] = []map[string]interface{}{{"url": upstream.URL + "/path"}}
		}
		if level == "operation" {
			operation["servers"] = []map[string]interface{}{{"url": upstream.URL + "/operation"}}
		}
		paths["/files/content/"+level] = item
	}
	for _, format := range []string{"json", "form"} {
		media := "application/json"
		if format == "form" {
			media = "application/x-www-form-urlencoded"
		}
		paths["/"+format] = map[string]interface{}{"post": map[string]interface{}{
			"operationId": format, "responses": transportE2EJSONResponses(),
			"requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{media: map[string]interface{}{"schema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"message": map[string]interface{}{"type": "string"}}}}}},
		}}
	}
	integration := transportE2ECreateIntegration(t, client, base, admin, "multipart.example", transportE2EBaseSpec("Multipart acceptance", upstream.URL+"/root", paths))
	transportE2EInstallIntegration(t, client, base, admin, integration)
	input := map[string]interface{}{
		"credential_id": credential,
		"attributes":    map[string]interface{}{"name": "probe.bin", "parent": map[string]interface{}{"id": "0"}},
		"file":          map[string]interface{}{"name": "probe.bin", "file": base64.StdEncoding.EncodeToString(payload), "type": "application/octet-stream"},
	}
	for _, operation := range []string{"uploadoperation", "uploadpath", "uploadroot", "json", "form"} {
		t.Run(operation, func(t *testing.T) {
			values := input
			if operation == "json" || operation == "form" {
				values = map[string]interface{}{"credential_id": credential, "message": "hello"}
			} else {
				details := transportE2EGetJSON(t, client, base+"/integration/multipart.example/operations/"+operation, admin)
				assertTransportE2EString(t, details, "input_schema.properties.attributes.type", "object")
				assertTransportE2EString(t, details, "input_schema.properties.file.type", "object")
				assertTransportE2EString(t, details, "input_schema.properties.file.properties.file.type", "string")
			}
			for _, route := range []string{"direct", "action"} {
				url := base + "/integration/multipart.example/" + operation
				body := map[string]interface{}{"credential_id": credential, "input": values}
				if route == "action" {
					url = base + "/action/integration/multipart.example/" + operation
					body = map[string]interface{}{"attributes": values}
				}
				before := calls.Load()
				status, response := postLLMActionForStatus(t, client, url, admin, body)
				if status != http.StatusOK || calls.Load() != before+1 {
					t.Fatalf("%s status=%d provider calls=%d: %s", route, status, calls.Load()-before, response)
				}
			}
		})
	}
	for _, invalid := range []struct {
		name string
		file interface{}
	}{
		{"filename-only", "probe.bin"},
		{"malformed-base64", map[string]interface{}{"name": "probe.bin", "file": "@@@"}},
		{"missing-bytes", map[string]interface{}{"name": "probe.bin"}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			values := map[string]interface{}{"credential_id": credential, "attributes": input["attributes"], "file": invalid.file}
			for _, route := range []string{"direct", "action"} {
				url := base + "/integration/multipart.example/uploadoperation"
				body := map[string]interface{}{"credential_id": credential, "input": values}
				if route == "action" {
					url = base + "/action/integration/multipart.example/uploadoperation"
					body = map[string]interface{}{"attributes": values}
				}
				before := calls.Load()
				status, response := postLLMActionForStatus(t, client, url, admin, body)
				if status != http.StatusBadRequest || calls.Load() != before {
					t.Fatalf("%s invalid file status=%d provider calls=%d: %s", route, status, calls.Load()-before, response)
				}
			}
		})
	}

}
