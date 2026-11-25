package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// Run with DAPTIN_REAL_E2E=1. DAPTIN_DOCUMENT_CORPUS_DIR optionally enables
// the version-pinned upstream corpus; every fixture is verified by its manifest.
func TestDocumentActionsQualificationRealE2E(t *testing.T) {
	requireRealE2E(t)
	schema, err := os.ReadFile("examples/document-processing/schema_document_processing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	options := transportE2EDaptinOptions{schema: string(schema), connectionString: filepath.Join(t.TempDir(), "qualification.db")}
	fixture := startDaptinE2E(t, options)
	t.Logf("fresh SQLite server: %s", fixture.URL)
	admin := fixture.SignupAdmin(t)
	member := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "document-qualification")
	world := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", "conversion_document")
	for _, operation := range []string{"extract", "convert"} {
		createDocumentQualificationAction(t, fixture, admin, world, operation)
	}
	t.Run("default_http_wire_limit", func(t *testing.T) {
		fields := documentQualificationInput("txt", bytes.Repeat([]byte("x"), 8<<20))
		fields["target_format"] = "txt"
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/conversion_document/qualify_convert", member,
			map[string]interface{}{"attributes": map[string]interface{}{"input": fields}}, 413)
	})
	// Raise the existing HTTP wire budget through backend configuration, then
	// restart to test the independent 8 MiB decoded performer boundary.
	request, err := http.NewRequest(http.MethodPost, fixture.URL+"/_config/backend/http.max_buffered_request_bytes", strings.NewReader("16777216"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+admin)
	response, err := fixture.Client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("configure HTTP wire limit: %d", response.StatusCode)
	}
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	t.Logf("restarted with 16 MiB HTTP budget: %s", fixture.URL)
	call := func(t *testing.T, operation string, input map[string]interface{}, status int) map[string]interface{} {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/conversion_document/qualify_"+operation,
			member, map[string]interface{}{"attributes": map[string]interface{}{"input": input}}, status)
		if status != http.StatusOK {
			if _, found := accessGroupsE2EFindString(response, "contents"); found {
				t.Fatal("failed action returned partial document bytes")
			}
			return nil
		}
		result, found := transportE2EPath(response, "0.Attributes.result")
		attrs, valid := result.(map[string]interface{})
		if !found || !valid {
			t.Fatalf("invalid performer result: %#v", response)
		}
		return attrs
	}
	input := func(format string, raw []byte) map[string]interface{} { return documentQualificationInput(format, raw) }
	convert := func(t *testing.T, source, target string, raw []byte, allow bool) []byte {
		t.Helper()
		fields := input(source, raw)
		fields["target_format"], fields["allow_lossy"] = target, allow
		result := call(t, "convert", fields, http.StatusOK)
		files := result["file"].([]interface{})
		if len(files) != 1 {
			t.Fatalf("output attachment count: %d", len(files))
		}
		file := files[0].(map[string]interface{})
		decoded, err := base64.StdEncoding.Strict().DecodeString(file["contents"].(string))
		if err != nil || float64(len(decoded)) != file["size"] || file["name"] != "document."+target {
			t.Fatalf("incorrect attachment metadata: %v", err)
		}
		return decoded
	}
	text := []byte("Document qualification\nHello world, κόσμος, Привет.\nTable content and footnotes.")
	sources := map[string][]byte{"txt": text}
	for _, target := range []string{"sf", "docx", "pdf"} {
		sources[target] = convert(t, "txt", target, text, false)
	}
	t.Run("anonymous_denial", func(t *testing.T) {
		for _, operation := range []string{"extract", "convert"} {
			fields := input("txt", text)
			fields["target_format"] = "pdf"
			accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/conversion_document/qualify_"+operation, "",
				map[string]interface{}{"attributes": map[string]interface{}{"input": fields}}, 403)
		}
	})
	t.Run("all_format_pairs", func(t *testing.T) {
		for _, source := range []string{"txt", "sf", "docx", "pdf"} {
			for _, target := range []string{"txt", "sf", "docx", "pdf"} {
				t.Run(source+"_to_"+target, func(t *testing.T) {
					raw := convert(t, source, target, sources[source], true)
					extracted := call(t, "extract", input(target, raw), http.StatusOK)
					for _, want := range []string{"Document qualification", "Hello world", "κόσμος", "Привет", "Table content"} {
						if !strings.Contains(extracted["text"].(string), want) {
							t.Fatalf("%s→%s lost %q: %q", source, target, want, extracted["text"])
						}
					}
					if target == "pdf" {
						qualifyDocumentPDFWithPoppler(t, raw)
					}
					if target == "docx" {
						archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
						if err != nil {
							t.Fatal(err)
						}
						if _, err := archive.Open("word/document.xml"); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	})
	t.Run("structured_object_and_loss_policy", func(t *testing.T) {
		fields := input("pdf", sources["pdf"])
		fields["target_format"] = "docx"
		call(t, "convert", fields, 422)
		fields["allow_lossy"] = true
		call(t, "convert", fields, 200)
		extracted := call(t, "extract", input("docx", sources["docx"]), 200)
		call(t, "convert", map[string]interface{}{"source_format": "sf", "document": extracted["document"], "target_format": "docx"}, 200)
		call(t, "extract", map[string]interface{}{"source_format": "sf", "document": extracted["document"]}, 200)
		fields = input("txt", []byte("Missing glyph \U0010ffff"))
		fields["target_format"], fields["allow_lossy"] = "pdf", true
		call(t, "convert", fields, 422)
	})
	t.Run("empty_documents", func(t *testing.T) {
		call(t, "extract", input("txt", nil), 200)
		for _, target := range []string{"txt", "sf", "docx", "pdf"} {
			convert(t, "txt", target, nil, false)
		}
	})
	t.Run("action_permission_revocation", func(t *testing.T) {
		id := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "action", "action_name", "qualify_extract")
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/action/"+id, admin,
			accessGroupsE2ERecordPayload("action", id, map[string]interface{}{"permission": 0}), 200)
		call(t, "extract", input("txt", text), 403)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/action/"+id, admin,
			accessGroupsE2ERecordPayload("action", id, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), 200)
		call(t, "extract", input("txt", text), 200)
	})
	t.Run("invalid_inputs", func(t *testing.T) {
		tests := []struct {
			name   string
			fields map[string]interface{}
			status int
		}{
			{"missing_format", map[string]interface{}{}, 400},
			{"unknown_format", input("doc", text), 400},
			{"case_sensitive_format", input("PDF", sources["pdf"]), 400},
			{"missing_file", map[string]interface{}{"source_format": "txt"}, 400},
			{"zero_files", map[string]interface{}{"source_format": "txt", "file": []interface{}{}}, 400},
			{"two_files", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{}, map[string]interface{}{}}}, 400},
			{"invalid_file", map[string]interface{}{"source_format": "txt", "file": []interface{}{42}}, 400},
			{"missing_contents", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"name": "source.txt"}}}, 400},
			{"base64_wrong_type", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": 42}}}, 400},
			{"bad_base64", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "@@@"}}}, 400},
			{"base64_linebreak", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "SGVsbG8=\n"}}}, 400},
			{"base64_data_url", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "data:text/plain;base64,SGVsbG8="}}}, 400},
			{"noncanonical_base64", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "Zh=="}}}, 400},
			{"malformed_file_json", map[string]interface{}{"source_format": "txt", "file": "["}, 400},
			{"wrong_file_json", map[string]interface{}{"source_format": "txt", "file": "{}"}, 400},
			{"invalid_pdf", input("pdf", []byte("%PDF-1.7\ntruncated")), 400},
			{"invalid_docx", input("docx", []byte("PKtruncated")), 400},
			{"invalid_sf", input("sf", []byte("{")), 400},
			{"unknown_sf", input("sf", []byte(`{"type":"doc","content":[]}`)), 422},
			{"multiple_sf_values", input("sf", append(bytes.Clone(sources["sf"]), sources["sf"]...)), 400},
			{"ambiguous_source", map[string]interface{}{"source_format": "sf", "document": map[string]interface{}{}, "file": []interface{}{}}, 400},
			{"wrong_document_format", map[string]interface{}{"source_format": "txt", "document": map[string]interface{}{}}, 400},
			{"password_on_txt", map[string]interface{}{"source_format": "txt", "file": input("txt", text)["file"], "password": "secret"}, 400},
			{"invalid_password_type", map[string]interface{}{"source_format": "pdf", "file": input("pdf", sources["pdf"])["file"], "password": true}, 400},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				call(t, "extract", test.fields, test.status)
				if _, ok := test.fields["target_format"]; !ok {
					test.fields["target_format"] = "txt"
				}
				call(t, "convert", test.fields, test.status)
			})
		}
		for _, bad := range []interface{}{"true", 1, []interface{}{true}} {
			fields := input("txt", text)
			fields["target_format"], fields["allow_lossy"] = "pdf", bad
			call(t, "convert", fields, 400)
		}
		for _, target := range []string{"html", "vdocs", "json", "sf.gz", "PDF", ""} {
			fields := input("txt", text)
			fields["target_format"] = target
			call(t, "convert", fields, 400)
		}
	})
	t.Run("size_boundaries", func(t *testing.T) {
		for _, operation := range []string{"extract", "convert"} {
			fields := input("txt", bytes.Repeat([]byte("x"), (8<<20)+1))
			fields["target_format"] = "txt"
			call(t, operation, fields, 413)
			fields = input("docx", documentQualificationZip(t, 1, (32<<20)+1))
			fields["target_format"] = "txt"
			call(t, operation, fields, 413)
			fields = input("docx", documentQualificationZip(t, 2049, 0))
			fields["target_format"] = "txt"
			call(t, operation, fields, 413)
		}
		atLimit := bytes.Repeat([]byte("x"), 8<<20)
		convert(t, "txt", "txt", atLimit, false)
		call(t, "extract", input("txt", atLimit), 413)
		fields := input("txt", bytes.Repeat([]byte{'"'}, 8<<20))
		fields["target_format"] = "sf"
		call(t, "convert", fields, 413)
	})
	t.Run("no_path_or_network_resolution", func(t *testing.T) {
		var calls atomic.Int64
		trap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.Write([]byte("private")) }))
		defer trap.Close()
		fields := map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"path": "/etc/passwd", "url": trap.URL}}}
		call(t, "extract", fields, 400)
		fields["target_format"] = "pdf"
		call(t, "convert", fields, 400)
		if calls.Load() != 0 {
			t.Fatal("document operation fetched caller-supplied URL")
		}
	})
	t.Run("deterministic_malformed_samples", func(t *testing.T) {
		random := rand.New(rand.NewSource(20261125))
		for _, format := range []string{"docx", "pdf", "sf"} {
			for i := 0; i < 20; i++ {
				raw := make([]byte, 16+random.Intn(2048))
				random.Read(raw)
				status, body := postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/conversion_document/qualify_extract", member,
					map[string]interface{}{"attributes": map[string]interface{}{"input": input(format, raw)}})
				if status != 400 && status != 422 {
					t.Fatalf("%s malformed sample %d: HTTP %d %s", format, i, status, body)
				}
			}
		}
		call(t, "extract", input("txt", text), 200)
	})
	t.Run("pinned_upstream_corpus", func(t *testing.T) {
		dir := os.Getenv("DAPTIN_DOCUMENT_CORPUS_DIR")
		if dir == "" {
			t.Skip("set DAPTIN_DOCUMENT_CORPUS_DIR to Manifold v0.1.0 testdata/corpus")
		}
		raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			Name, Kind, Expect, SHA256 string
			Size                       int
		}
		if err := json.Unmarshal(raw, &cases); err != nil {
			t.Fatal(err)
		}
		for _, fixtureCase := range cases {
			t.Run(fixtureCase.Name, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join(dir, fixtureCase.Name))
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) != fixtureCase.Size || fmt.Sprintf("%x", sha256.Sum256(raw)) != fixtureCase.SHA256 {
					t.Fatal("fixture differs from pinned manifest")
				}
				fields := input(fixtureCase.Kind, raw)
				if fixtureCase.Expect == "password" {
					for _, password := range []string{"", "wrong"} {
						fields["password"] = password
						call(t, "extract", fields, 400)
						fields["target_format"] = "sf"
						call(t, "convert", fields, 400)
					}
					fields["password"] = "secret"
				}
				if fixtureCase.Expect == "reject" {
					status, body := postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/conversion_document/qualify_extract", member, map[string]interface{}{"attributes": map[string]interface{}{"input": fields}})
					if status != 400 && status != 422 {
						t.Fatalf("reject corpus: HTTP %d %s", status, body)
					}
					fields["target_format"] = "sf"
					status, body = postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/conversion_document/qualify_convert", member, map[string]interface{}{"attributes": map[string]interface{}{"input": fields}})
					if status != 400 && status != 422 {
						t.Fatalf("reject corpus conversion: HTTP %d %s", status, body)
					}
					return
				}
				result := call(t, "extract", fields, 200)
				if fixtureCase.Name == "pdf_scan.pdf" && result["text"] != "" {
					t.Fatal("scan invented OCR text")
				}
				fields["target_format"] = "sf"
				converted := call(t, "convert", fields, 200)
				reopened := call(t, "extract", map[string]interface{}{"source_format": "sf", "file": converted["file"]}, 200)
				if !reflect.DeepEqual(result["document"], reopened["document"]) {
					t.Fatal("SF conversion changed the extracted structure, assets, notes or provenance")
				}
			})
		}
	})
}

func TestDocumentActionsQuotaRealE2E(t *testing.T) {
	requireRealE2E(t)
	schema, err := os.ReadFile("examples/document-processing/schema_document_processing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: string(schema)})
	admin := fixture.SignupAdmin(t)
	member := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "document-quota")
	owner := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "document-quota@test.local")
	world := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", "conversion_document")
	plan := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "api_plan", map[string]interface{}{
		"name": "One successful conversion", "limits": `[{"metric":"requests","window":"month","maximum":1,"mode":"hard"}]`,
	})
	// Membership provisioning uses a fixed backend SWITCH_USER outcome. The
	// caller cannot supply ownership or an internal permission mode.
	definition, err := json.Marshal(map[string]interface{}{
		"Name": "assign_document_plan", "OnType": "conversion_document", "InstanceOptional": true,
		"OutFields": []map[string]interface{}{
			{"Type": "__as_user", "Method": "SWITCH_USER", "Attributes": map[string]interface{}{"user_reference_id": owner}},
			{"Type": "api_member", "Method": "POST", "Attributes": map[string]interface{}{
				"api_plan_id": plan, "status": "active", "period_start": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "action", map[string]interface{}{
		"action_name": "assign_document_plan", "label": "Assign plan", "world_id": world, "instance_optional": true, "action_schema": string(definition),
	})
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/conversion_document/assign_document_plan", admin,
		map[string]interface{}{"attributes": map[string]interface{}{}}, 200)
	doc := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, member, "conversion_document", map[string]interface{}{
		"title": "Quota source", "source_format": "docx", "source_base64": base64.StdEncoding.EncodeToString([]byte("not an archive")),
	})
	payload := map[string]interface{}{"attributes": map[string]interface{}{"conversion_document_id": doc}}
	action := fixture.URL + "/action/conversion_document/save_pdf"
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, action, member, payload, 400)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/conversion_document/"+doc, admin,
		accessGroupsE2ERecordPayload("conversion_document", doc, map[string]interface{}{"source_format": "txt"}), 200)
	success := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, action, member, payload, 200)
	assertDocumentE2EPDF(t, success)
	row := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/conversion_document/"+doc, member, nil, 200)
	before, _ := accessGroupsE2EFindString(row, "output_base64")
	// With quota exhausted, even malformed input must fail admission (402)
	// before the document parser can return its own invalid-archive error.
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/conversion_document/"+doc, admin,
		accessGroupsE2ERecordPayload("conversion_document", doc, map[string]interface{}{"source_format": "docx"}), 200)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, action, member, payload, 402)
	row = accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/conversion_document/"+doc, member, nil, 200)
	after, _ := accessGroupsE2EFindString(row, "output_base64")
	if before == "" || after != before {
		t.Fatal("quota denial changed persisted output")
	}
	quotas := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/api_quota?page[size]=100", admin, nil, 200))
	matched := false
	for _, row := range quotas {
		attrs := row.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["user_account_id"] == owner && attrs["metric"] == "requests" {
			matched = true
			if attrs["reserved"] != float64(0) || attrs["consumed"] != float64(1) {
				t.Fatalf("failed/denied work consumed quota: %#v", attrs)
			}
		}
	}
	if !matched {
		t.Fatal("conversion did not use the durable account quota")
	}
}

func TestDocumentActionsConcurrencyPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") == "" {
		t.Skip("set DAPTIN_E2E_MAIL_POSTGRES_BIN for private PostgreSQL concurrency qualification")
	}
	options := mailProtocolDatabase(t)
	schema, err := os.ReadFile("examples/document-processing/schema_document_processing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	options.schema = string(schema)
	fixture := startDaptinE2E(t, options)
	t.Logf("fresh PostgreSQL server: %s", fixture.URL)
	admin := fixture.SignupAdmin(t)
	world := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "world", "table_name", "conversion_document")
	for _, operation := range []string{"extract", "convert"} {
		createDocumentQualificationAction(t, fixture, admin, world, operation)
	}
	call := func(t *testing.T, operation string, fields map[string]interface{}, status int) map[string]interface{} {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/conversion_document/qualify_"+operation, admin,
			map[string]interface{}{"attributes": map[string]interface{}{"input": fields}}, status)
		if status != 200 {
			return nil
		}
		result, _ := transportE2EPath(response, "0.Attributes.result")
		return result.(map[string]interface{})
	}
	t.Run("concurrent_admission_and_disconnect_recovery", func(t *testing.T) { qualifyDocumentConcurrency(t, fixture, admin, call) })
	t.Run("render_bounded_by_deadline_or_output", func(t *testing.T) {
		fields := documentQualificationInput("docx", documentQualificationLongDOCX(t))
		fields["target_format"] = "pdf"
		client := &http.Client{Timeout: 50 * time.Second}
		started := time.Now()
		status, body := postLLMActionForStatus(t, client, fixture.URL+"/action/conversion_document/qualify_convert", admin,
			map[string]interface{}{"attributes": map[string]interface{}{"input": fields}})
		elapsed := time.Since(started)
		t.Logf("large PDF result: HTTP %d after %s", status, elapsed)
		// A fast renderer may reach the output cap first. Both guards are
		// legitimate terminal outcomes; execution must never exceed its bound.
		if (status != 408 && status != 413) || (status == 408 && elapsed < 29*time.Second) || elapsed > 45*time.Second {
			t.Fatalf("render limits not enforced: HTTP %d after %s: %s", status, elapsed, body)
		}
		call(t, "extract", documentQualificationInput("txt", []byte("after deadline")), 200)
	})
}

func documentQualificationLongDOCX(t *testing.T) []byte {
	t.Helper()
	// A compressed, valid package below the 32 MiB expanded-input limit. Its
	// many paragraphs require substantially more layout work than a text blob.
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
			strings.Repeat(`<w:p><w:r><w:t>`+strings.Repeat("word ", 80)+`</w:t></w:r></w:p>`, 60000) + `</w:body></w:document>`,
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, contents := range parts {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(writer, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func createDocumentQualificationAction(t *testing.T, fixture *daptinE2EFixture, admin, world, operation string) {
	t.Helper()
	attrs := map[string]interface{}{}
	for _, key := range []string{"source_format", "target_format", "file", "document", "password", "allow_lossy"} {
		attrs[key] = "!input." + key
	}
	name := "qualify_" + operation
	schema, err := json.Marshal(map[string]interface{}{
		"Name": name, "OnType": "conversion_document", "InstanceOptional": true,
		"InFields": []map[string]interface{}{{"Name": "input", "ColumnName": "input", "ColumnType": "json", "IsNullable": false}},
		"OutFields": []map[string]interface{}{
			{"Type": "document." + operation, "Method": "EXECUTE", "Reference": "processed", "Attributes": attrs},
			{"Type": "document.result", "Method": "ACTIONRESPONSE", "Attributes": map[string]interface{}{"result": "~processed"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "action", map[string]interface{}{
		"action_name": name, "label": name, "world_id": world, "instance_optional": true, "action_schema": string(schema),
	})
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/action/"+id, admin,
		accessGroupsE2ERecordPayload("action", id, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), 200)
}

func documentQualificationInput(format string, raw []byte) map[string]interface{} {
	return map[string]interface{}{"source_format": format, "file": []interface{}{map[string]interface{}{"name": "source." + format, "contents": base64.StdEncoding.EncodeToString(raw)}}}
}

func documentQualificationZip(t *testing.T, count, expanded int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for i := 0; i < count; i++ {
		member, err := archive.Create(fmt.Sprintf("part-%04d", i))
		if err != nil {
			t.Fatal(err)
		}
		if expanded > 0 {
			if _, err := io.CopyN(member, strings.NewReader(strings.Repeat("x", expanded)), int64(expanded)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func qualifyDocumentPDFWithPoppler(t *testing.T, raw []byte) {
	t.Helper()
	binary, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("install Poppler for independent PDF text validation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-", "-")
	cmd.Stdin = bytes.NewReader(raw)
	text, err := cmd.Output()
	if err != nil || !bytes.Contains(text, []byte("Document qualification")) {
		t.Fatalf("independent PDF reader: %v %q", err, text)
	}
}

func qualifyDocumentConcurrency(t *testing.T, fixture *daptinE2EFixture, token string, call func(*testing.T, string, map[string]interface{}, int) map[string]interface{}) {
	t.Helper()
	// Large but admissible text keeps both renderers occupied long enough to
	// observe admission, then cancellation must release both slots.
	fields := documentQualificationInput("txt", []byte(strings.Repeat(strings.Repeat("word ", 80)+"\n", 14000)))
	fields["target_format"] = "pdf"
	body, err := json.Marshal(map[string]interface{}{"attributes": map[string]interface{}{"input": fields}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fixture.URL+"/action/conversion_document/qualify_convert", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			if response, err := fixture.Client.Do(req); err == nil {
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
		}()
	}
	saturated := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/conversion_document/qualify_extract", token,
			map[string]interface{}{"attributes": map[string]interface{}{"input": documentQualificationInput("txt", []byte("probe"))}})
		if status == 503 {
			saturated = true
			break
		}
		if status != 200 {
			t.Fatalf("admission probe: HTTP %d", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	workers.Wait()
	if !saturated {
		t.Fatal("did not observe shared conversion/extraction admission limit")
	}
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := postLLMActionForStatus(t, fixture.Client, fixture.URL+"/action/conversion_document/qualify_extract", token,
			map[string]interface{}{"attributes": map[string]interface{}{"input": documentQualificationInput("txt", []byte("recovered"))}})
		if status == 200 {
			call(t, "convert", map[string]interface{}{"source_format": "txt", "target_format": "pdf", "file": documentQualificationInput("txt", []byte("recovered"))["file"]}, 200)
			return
		}
		if status != 503 {
			t.Fatalf("recovery probe: HTTP %d", status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("disconnect left document processor saturated")
}
