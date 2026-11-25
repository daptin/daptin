package actions

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/manifold"
	"github.com/daptin/daptin/server/actionresponse"
)

func documentTestInput(format string, raw []byte) map[string]interface{} {
	return map[string]interface{}{"source_format": format, "file": []interface{}{map[string]interface{}{
		"name": "input." + format, "contents": base64.StdEncoding.EncodeToString(raw),
	}}}
}

func documentTestResult(t *testing.T, performer actionresponse.ActionPerformerInterface, input map[string]interface{}) map[string]interface{} {
	t.Helper()
	result, responses, errs := performer.DoAction(actionresponse.Outcome{}, input, nil)
	if len(errs) != 0 || len(responses) != 0 || result == nil {
		t.Fatalf("result=%v responses=%v errors=%v", result, responses, errs)
	}
	return result.Result().(api2go.Api2GoModel).GetAttributes()
}

func TestDocumentPerformersRichRoundTrip(t *testing.T) {
	performers := NewDocumentActionPerformers()
	if performers[0].Name() != "document.extract" || performers[1].Name() != "document.convert" {
		t.Fatal("incorrect performer names")
	}
	doc := &manifold.Document{Metadata: manifold.Metadata{Title: "Composition"}, Blocks: []manifold.Block{
		{Kind: manifold.Heading, Level: 1, Runs: []manifold.Run{{Text: "Document workflow"}}},
		{Kind: manifold.Paragraph, Runs: []manifold.Run{{Text: "Hello κόσμος", Bold: true}}},
		{Kind: manifold.Table, Rows: []manifold.Row{{Cells: []manifold.Cell{{Blocks: []manifold.Block{
			{Kind: manifold.Paragraph, Runs: []manifold.Run{{Text: "Table content"}}},
		}}}}}},
	}}
	structured, err := manifold.ToStructured(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"docx", "pdf", "sf", "txt"} {
		t.Run(format, func(t *testing.T) {
			converted := documentTestResult(t, performers[1], map[string]interface{}{
				"source_format": "sf", "target_format": format, "document": structured,
			})
			file := converted["file"].([]interface{})[0].(map[string]interface{})
			raw, err := base64.StdEncoding.DecodeString(file["contents"].(string))
			if err != nil || len(raw) != file["size"] {
				t.Fatalf("incorrect file metadata: %v", err)
			}
			extracted := documentTestResult(t, performers[0], map[string]interface{}{"source_format": format, "file": converted["file"]})
			text := extracted["text"].(string)
			for _, want := range []string{"Document workflow", "Hello κόσμος", "Table content"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s extraction missing %q: %q", format, want, text)
				}
			}
		})
	}
}

func TestDocumentLossRequiresAcknowledgement(t *testing.T) {
	performers := NewDocumentActionPerformers()
	doc := &manifold.Document{Blocks: []manifold.Block{{Kind: manifold.Paragraph, Runs: []manifold.Run{{Text: "Visible content"}}}},
		Diagnostics: []manifold.Diagnostic{{Code: "unsupported_source", Message: "Original feature omitted"}}}
	structured, err := manifold.ToStructured(doc)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]interface{}{"source_format": "sf", "target_format": "docx", "document": structured}
	result, responses, errs := performers[1].DoAction(actionresponse.Outcome{}, input, nil)
	if result != nil || len(responses) != 0 || len(errs) != 1 {
		t.Fatalf("lossy output published: %v %v %v", result, responses, errs)
	}
	var httpErr api2go.HTTPError
	if !errors.As(errs[0], &httpErr) || httpErr.Status() != 422 || len(httpErr.Errors) != 1 || httpErr.Errors[0].Meta == nil {
		t.Fatalf("loss diagnostics missing: %v", errs)
	}
	input["allow_lossy"] = true
	documentTestResult(t, performers[1], input)
	extracted := documentTestResult(t, performers[0], map[string]interface{}{"source_format": "sf", "document": structured})
	if len(extracted["diagnostics"].([]manifold.Diagnostic)) != 1 {
		t.Fatal("extraction discarded diagnostics")
	}
}

func TestDocumentValidationPublishesNoOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]interface{}
		status int
	}{
		{"missing file", map[string]interface{}{"source_format": "txt"}, 400},
		{"unsupported format", documentTestInput("doc", nil), 400},
		{"invalid archive", documentTestInput("docx", []byte("bad")), 400},
		{"invalid pdf", documentTestInput("pdf", []byte("bad")), 400},
		{"unknown SF nodes", documentTestInput("sf", []byte(`{"type":"doc","content":[{"type":"unknown"}]}`)), 422},
		{"external path", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"path": "/etc/passwd"}}}, 400},
		{"invalid base64", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "@@@"}}}, 400},
		{"base64 newline", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "aGk=\n"}}}, 400},
		{"data URL", map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"contents": "data:text/plain;base64,aGk="}}}, 400},
		{"ambiguous input", map[string]interface{}{"source_format": "sf", "document": map[string]interface{}{}, "file": []interface{}{}}, 400},
		{"oversized input", documentTestInput("txt", bytes.Repeat([]byte("x"), documentMaxInput+1)), 413},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			performer := NewDocumentActionPerformers()[0]
			result, responses, errs := performer.DoAction(actionresponse.Outcome{}, test.input, nil)
			var httpErr api2go.HTTPError
			if result != nil || len(responses) != 0 || len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != test.status {
				t.Fatalf("result=%v responses=%v errors=%v, want HTTP %d", result, responses, errs, test.status)
			}
		})
	}
	convert := NewDocumentActionPerformers()[1]
	for _, extra := range []map[string]interface{}{{"target_format": "html"}, {"target_format": "pdf", "allow_lossy": "true"}, {"target_format": "pdf", "password": "secret"}} {
		input := documentTestInput("txt", []byte("hello"))
		for key, value := range extra {
			input[key] = value
		}
		if result, _, errs := convert.DoAction(actionresponse.Outcome{}, input, nil); result != nil || len(errs) != 1 {
			t.Fatalf("invalid conversion accepted: %v", extra)
		}
	}
}

func TestDocumentArchiveExpansionAndPartLimits(t *testing.T) {
	for _, manyParts := range []bool{false, true} {
		var raw bytes.Buffer
		writer := zip.NewWriter(&raw)
		if manyParts {
			for i := 0; i <= documentMaxParts; i++ {
				if _, err := writer.Create(string(rune(i + 1000))); err != nil {
					t.Fatal(err)
				}
			}
		} else {
			part, err := writer.Create("word/document.xml")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.CopyN(part, strings.NewReader(strings.Repeat("x", documentMaxExpanded+1)), documentMaxExpanded+1); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := validateDocumentArchive(context.Background(), raw.Bytes()); !errors.Is(err, errDocumentSize) {
			t.Fatalf("archive exceeded limits: %v", err)
		}
	}
}

func TestDocumentCancellationOverloadAndOutputBounds(t *testing.T) {
	performer := NewDocumentActionPerformers()[0].(*documentActionPerformer)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := documentTestInput("txt", []byte("hello"))
	input["httpRequest"] = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	_, _, errs := performer.DoAction(actionresponse.Outcome{}, input, nil)
	var httpErr api2go.HTTPError
	if len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != 408 {
		t.Fatalf("cancelled operation: %v", errs)
	}
	delete(input, "httpRequest")
	performer.slots <- struct{}{}
	performer.slots <- struct{}{}
	_, _, errs = performer.DoAction(actionresponse.Outcome{}, input, nil)
	if len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != 503 {
		t.Fatalf("overloaded operation: %v", errs)
	}
	<-performer.slots
	<-performer.slots
	documentTestResult(t, performer, input)
	buffer := &documentBuffer{limit: 3}
	if _, err := io.Copy(buffer, strings.NewReader("four")); !errors.Is(err, errDocumentSize) || buffer.Len() != 0 {
		t.Fatalf("output limit bypassed: %v", err)
	}
	// Extraction duplicates text in SF and plain text. Bound the whole response.
	large := documentTestInput("txt", bytes.Repeat([]byte("x"), documentMaxInput))
	result, responses, errs := performer.DoAction(actionresponse.Outcome{}, large, nil)
	if result != nil || len(responses) != 0 || len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != 413 {
		t.Fatalf("oversized response published: %v", errs)
	}
}

func TestDocumentJSONActionArgumentUsesSameValidation(t *testing.T) {
	p := NewDocumentActionPerformers()[0]
	attrs := documentTestResult(t, p, map[string]interface{}{
		"source_format": "txt", "file": `[{"name":"hello.txt","contents":"SGVsbG8="}]`,
	})
	if attrs["text"] != "Hello" {
		t.Fatalf("JSON argument result: %#v", attrs)
	}
	for _, file := range []string{`{}`, `[]`, `[{},{}]`, `[{"contents":"@@@"}]`, `[`, strings.Repeat(" ", documentMaxOutput+1)} {
		result, responses, errs := p.DoAction(actionresponse.Outcome{}, map[string]interface{}{"source_format": "txt", "file": file}, nil)
		if result != nil || len(responses) != 0 || len(errs) != 1 {
			t.Fatalf("invalid JSON argument produced output: %v", errs)
		}
	}
}

func TestDocumentAcknowledgementDoesNotBypassUnsupportedOutput(t *testing.T) {
	p := NewDocumentActionPerformers()[1]
	// This glyph is outside the built-in PDF fonts. AllowLossy acknowledges
	// import diagnostics; it cannot authorize font substitution or lost text.
	input := documentTestInput("txt", []byte("Missing glyph \U0010ffff"))
	input["target_format"] = "pdf"
	input["allow_lossy"] = true
	result, responses, errs := p.DoAction(actionresponse.Outcome{}, input, nil)
	var httpErr api2go.HTTPError
	if result != nil || len(responses) != 0 || len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != 422 {
		t.Fatalf("unsupported output published: %v", errs)
	}
}
