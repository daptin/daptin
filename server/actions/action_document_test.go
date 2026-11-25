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
	"os"
	"reflect"
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

func TestDocumentPresentationComposition(t *testing.T) {
	raw, err := os.ReadFile("testdata/document_presentation.pptx")
	if err != nil {
		t.Fatal(err)
	}
	performers := NewDocumentActionPerformers()
	extracted := documentTestResult(t, performers[0], documentTestInput("pptx", raw))
	for _, want := range []string{"Document presentation", "Second slide Ελληνικά", "Private speaker notes"} {
		if !strings.Contains(extracted["text"].(string), want) {
			t.Fatalf("extraction omitted %q", want)
		}
	}
	doc, err := manifold.FromStructured(extracted["document"].(*manifold.StructuredDocument))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Presentation == nil || len(doc.Presentation.Slides) != 2 || len(doc.Presentation.Slides[0].Notes) == 0 || len(doc.Presentation.Slides[0].Objects) != 2 {
		t.Fatalf("incomplete presentation extraction: %+v", doc.Presentation)
	}
	first := doc.Presentation.Slides[0].Objects[0]
	if first.X != 40 || first.Y != 40 || first.Width != 500 || first.Height != 70 {
		t.Fatalf("geometry changed: %+v", first)
	}
	convert := func(source, target string, content []byte, allow bool) []byte {
		input := documentTestInput(source, content)
		input["target_format"], input["allow_lossy"] = target, allow
		return documentTestOutput(t, documentTestResult(t, performers[1], input), target)
	}
	sf := convert("pptx", "sf", raw, false)
	reopened := documentTestResult(t, performers[0], documentTestInput("sf", sf))
	if !reflect.DeepEqual(extracted["document"], reopened["document"]) {
		t.Fatal("SF changed presentation content or native resources")
	}
	roundtrip := convert("sf", "pptx", sf, false)
	if !reflect.DeepEqual(documentTestArchive(t, raw), documentTestArchive(t, roundtrip)) {
		t.Fatal("unchanged native PPTX parts were rewritten or dropped")
	}
	// Typed SF edits must reach the original deck rather than a separate export.
	doc.Presentation.Slides[0].Objects[0].X = 88
	doc.Presentation.Slides[0].Objects[0].Content[0].Runs[0].Text = "Edited through Daptin"
	structured, err := manifold.ToStructured(doc)
	if err != nil {
		t.Fatal(err)
	}
	edited := documentTestResult(t, performers[1], map[string]interface{}{"source_format": "sf", "document": structured, "target_format": "pptx"})
	back := documentTestResult(t, performers[0], documentTestInput("pptx", documentTestOutput(t, edited, "pptx")))
	if !strings.Contains(back["text"].(string), "Edited through Daptin") {
		t.Fatal("SF text edit was ignored")
	}
	changed, err := manifold.FromStructured(back["document"].(*manifold.StructuredDocument))
	if err != nil || changed.Presentation.Slides[0].Objects[0].X != 88 {
		t.Fatalf("SF geometry edit was ignored: %v", err)
	}
	pdf := convert("pptx", "pdf", raw, true)
	pdfDoc := documentTestResult(t, performers[0], documentTestInput("pdf", pdf))
	if strings.Contains(pdfDoc["text"].(string), "Private speaker notes") || !strings.Contains(pdfDoc["text"].(string), "Second slide Ελληνικά") {
		t.Fatal("PDF did not distinguish visible content from notes")
	}
	convert("pdf", "pptx", pdf, true)
	text := convert("pptx", "txt", raw, false)
	if !bytes.Contains(text, []byte("Private speaker notes")) {
		t.Fatal("text extraction lost notes")
	}
}

func TestDocumentPresentationDenialsReturnNoOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/document_presentation.pptx")
	if err != nil {
		t.Fatal(err)
	}
	p := NewDocumentActionPerformers()[1]
	for _, test := range []struct {
		name, source, target string
		raw                  []byte
		allow                bool
		status               int
	}{
		{"notes require acknowledgement", "pptx", "pdf", raw, false, 422},
		{"slides cannot silently become Word flow", "pptx", "docx", raw, true, 422},
		{"flow needs acknowledgement", "txt", "pptx", []byte("New slide"), false, 422},
		{"legacy PPT", "ppt", "pptx", raw, true, 400},
		{"malformed PPTX", "pptx", "sf", []byte("bad"), false, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			fields := documentTestInput(test.source, test.raw)
			fields["target_format"], fields["allow_lossy"] = test.target, test.allow
			result, responses, errs := p.DoAction(actionresponse.Outcome{}, fields, nil)
			var httpErr api2go.HTTPError
			if result != nil || len(responses) != 0 || len(errs) != 1 || !errors.As(errs[0], &httpErr) || httpErr.Status() != test.status {
				t.Fatalf("denial returned output or wrong error: %v", errs)
			}
		})
	}
}

func documentTestOutput(t *testing.T, result map[string]interface{}, target string) []byte {
	t.Helper()
	file := result["file"].([]interface{})[0].(map[string]interface{})
	if file["name"] != "document."+target {
		t.Fatalf("incorrect output filename: %#v", file["name"])
	}
	if target == "pptx" && file["type"] != "application/vnd.openxmlformats-officedocument.presentationml.presentation" {
		t.Fatalf("incorrect PPTX MIME type: %#v", file["type"])
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(file["contents"].(string))
	if err != nil || file["size"] != len(raw) {
		t.Fatalf("invalid output attachment: %v", err)
	}
	return raw
}

func documentTestArchive(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	parts := make(map[string][]byte, len(archive.File))
	for _, part := range archive.File {
		reader, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[part.Name] = content
	}
	return parts
}
