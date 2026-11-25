package actions

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/manifold"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/jmoiron/sqlx"
)

const (
	documentMaxInput    = 8 << 20
	documentMaxOutput   = 16 << 20
	documentMaxExpanded = 32 << 20
	documentMaxParts    = 2048
	documentTimeout     = 30 * time.Second
)

// Both operations share admission. There are no background workers to drain.
type documentActionPerformer struct {
	name  string
	slots chan struct{}
}

func NewDocumentActionPerformers() []actionresponse.ActionPerformerInterface {
	slots := make(chan struct{}, 2)
	return []actionresponse.ActionPerformerInterface{
		&documentActionPerformer{name: "document.extract", slots: slots},
		&documentActionPerformer{name: "document.convert", slots: slots},
	}
}

func (p *documentActionPerformer) Name() string { return p.name }

func (p *documentActionPerformer) DoAction(_ actionresponse.Outcome, input map[string]interface{}, _ *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	ctx := context.Background()
	if req, ok := input["httpRequest"].(*http.Request); ok && req != nil {
		ctx = req.Context()
	}
	ctx, cancel := context.WithTimeout(ctx, documentTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, nil, []error{documentHTTPError(err)}
	}
	// Reject overload instead of waiting with the action's transaction open.
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	default:
		return nil, nil, []error{api2go.NewHTTPError(nil, "document processor is busy", http.StatusServiceUnavailable)}
	}
	attrs, err := p.process(ctx, input)
	if err != nil {
		return nil, nil, []error{documentHTTPError(err)}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, []error{documentHTTPError(err)}
	}
	// Bound the complete response, including text, assets and diagnostics.
	if err := stdjson.NewEncoder(&documentBuffer{limit: documentMaxOutput}).Encode(attrs); err != nil {
		return nil, nil, []error{documentHTTPError(err)}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, []error{documentHTTPError(err)}
	}
	return api2go.Response{Res: api2go.NewApi2GoModelWithData(p.name, nil, 0, nil, attrs)}, nil, nil
}

func (p *documentActionPerformer) process(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	var target manifold.Format
	var err error
	allowLossy := false
	if p.name == "document.convert" {
		target, err = documentFormat(input["target_format"])
		if err != nil {
			return nil, err
		}
		if value, exists := input["allow_lossy"]; exists {
			var ok bool
			allowLossy, ok = value.(bool)
			if !ok {
				return nil, errors.New("allow_lossy must be a boolean")
			}
		}
	}
	doc, err := readActionDocument(ctx, input)
	if err != nil {
		return nil, err
	}
	diagnostics := doc.Diagnostics
	if diagnostics == nil {
		diagnostics = []manifold.Diagnostic{}
	}
	if p.name == "document.extract" {
		structured, err := manifold.ToStructured(doc)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"document": structured, "text": doc.Text(), "metadata": doc.Metadata, "diagnostics": diagnostics}, nil
	}
	output := &documentBuffer{limit: documentMaxOutput}
	if err := manifold.Write(ctx, output, doc, target, manifold.WriteOptions{AllowLossy: allowLossy}); err != nil {
		if errors.Is(err, manifold.ErrLossy) {
			httpErr := api2go.NewHTTPError(nil, "conversion requires explicit allow_lossy acknowledgement", http.StatusUnprocessableEntity)
			httpErr.Errors = []api2go.Error{{Status: "422", Code: "document_lossy", Title: "Conversion would lose document features", Detail: err.Error(), Meta: map[string]interface{}{"diagnostics": diagnostics}}}
			return nil, httpErr
		}
		return nil, err
	}
	name := "document." + string(target)
	mediaTypes := map[manifold.Format]string{
		manifold.DOCX: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		manifold.PPTX: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		manifold.PDF:  "application/pdf", manifold.SF: "application/json", manifold.Text: "text/plain; charset=utf-8",
	}
	file := map[string]interface{}{"name": name, "type": mediaTypes[target], "size": output.Len(), "contents": base64.StdEncoding.EncodeToString(output.Bytes())}
	return map[string]interface{}{"file": []interface{}{file}, "diagnostics": diagnostics}, nil
}

func documentFormat(value interface{}) (manifold.Format, error) {
	switch value {
	case "docx":
		return manifold.DOCX, nil
	case "pptx":
		return manifold.PPTX, nil
	case "pdf":
		return manifold.PDF, nil
	case "sf":
		return manifold.SF, nil
	case "txt":
		return manifold.Text, nil
	default:
		return "", errors.New("format must be one of docx, pptx, pdf, sf, txt")
	}
}

func readActionDocument(ctx context.Context, input map[string]interface{}) (*manifold.Document, error) {
	format, err := documentFormat(input["source_format"])
	if err != nil {
		return nil, err
	}
	password := ""
	if value, exists := input["password"]; exists {
		var ok bool
		password, ok = value.(string)
		if !ok || format != manifold.PDF {
			return nil, errors.New("password is only supported as a string for PDF input")
		}
	}
	var raw []byte
	if structured, exists := input["document"]; exists {
		if _, hasFile := input["file"]; hasFile || format != manifold.SF {
			return nil, errors.New("document requires source_format sf and cannot be combined with file")
		}
		buffer := &documentBuffer{limit: documentMaxInput}
		if err := stdjson.NewEncoder(buffer).Encode(structured); err != nil {
			return nil, err
		}
		raw = buffer.Bytes()
	} else {
		// Daptin's GraphQL json action argument is a JSON-encoded string.
		// Normalize that representation here so every action transport uses
		// the same validation and conversion path.
		fileValue := input["file"]
		if encoded, ok := fileValue.(string); ok {
			if len(encoded) > documentMaxOutput {
				return nil, errDocumentSize
			}
			if err := stdjson.Unmarshal([]byte(encoded), &fileValue); err != nil {
				return nil, errors.New("file must be a JSON attachment array")
			}
		}
		files, ok := fileValue.([]interface{})
		if !ok || len(files) != 1 {
			return nil, errors.New("file must contain exactly one inline attachment")
		}
		file, ok := files[0].(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid document attachment")
		}
		encoded, ok := file["contents"].(string)
		if !ok {
			return nil, errors.New("document attachment requires base64 contents; paths and URLs are not resolved")
		}
		if len(encoded) > base64.StdEncoding.EncodedLen(documentMaxInput) {
			return nil, errDocumentSize
		}
		// One encoding is accepted, with no data URL or whitespace guessing.
		if strings.ContainsAny(encoded, "\r\n") {
			return nil, errors.New("contents must be canonical base64")
		}
		raw, err = base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			return nil, errors.New("invalid document base64 contents")
		}
	}
	if len(raw) > documentMaxInput {
		return nil, errDocumentSize
	}
	if format == manifold.DOCX || format == manifold.PPTX {
		if err := validateDocumentArchive(ctx, raw); err != nil {
			return nil, err
		}
	}
	return manifold.Read(ctx, bytes.NewReader(raw), format, manifold.ReadOptions{Password: password})
}

// Count actual expanded bytes as well as declared sizes before handing the
// archive to Manifold. No archive member is extracted to the filesystem.
func validateDocumentArchive(ctx context.Context, raw []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("invalid document archive: %w", err)
	}
	if len(archive.File) > documentMaxParts {
		return errDocumentSize
	}
	remaining := int64(documentMaxExpanded)
	for _, part := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if part.UncompressedSize64 > uint64(remaining) {
			return errDocumentSize
		}
		r, err := part.Open()
		if err != nil {
			return err
		}
		n, readErr := io.Copy(io.Discard, io.LimitReader(&documentContextReader{ctx: ctx, reader: r}, remaining+1))
		closeErr := r.Close()
		if n > remaining {
			return errDocumentSize
		}
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		remaining -= n
	}
	return nil
}

var errDocumentSize = errors.New("document exceeds processing size limits")

type documentContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *documentContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type documentBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *documentBuffer) Len() int      { return b.buffer.Len() }
func (b *documentBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (b *documentBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errDocumentSize
	}
	return b.buffer.Write(p)
}

func documentHTTPError(err error) error {
	var httpErr api2go.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, errDocumentSize):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status = http.StatusRequestTimeout
	case errors.Is(err, manifold.ErrUnsupported):
		status = http.StatusUnprocessableEntity
	}
	return api2go.NewHTTPError(nil, err.Error(), status)
}
