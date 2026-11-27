//go:build live

package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"
)

// This test transport owns the experiment's request/response evidence. It records
// all received body bytes; parsed views are derived from that single record.
// It never records credential headers or changes bytes forwarded to the gateway.
type agentLiveRecorder struct {
	transport   http.RoundTripper
	secret      string
	journal     io.Writer
	mu          sync.Mutex
	exchanges   []agentLiveExchange
	writeErrors []error
}
type agentLiveExchange struct {
	Method          string         `json:"method"`
	Path            string         `json:"path"`
	Request         agentLiveBody  `json:"request"`
	Status          int            `json:"status"`
	ContentType     string         `json:"content_type"`
	ContentEncoding string         `json:"content_encoding"`
	Response        *agentLiveBody `json:"response"`
	TransportError  string         `json:"transport_error,omitempty"`
}
type agentLiveBody struct {
	Bytes         []byte      `json:"body_base64"`
	ReceivedBytes int         `json:"received_bytes"`
	SHA256        string      `json:"saved_body_sha256"`
	Complete      bool        `json:"complete"`
	Redacted      bool        `json:"redacted"`
	Text          *string     `json:"body_text,omitempty"`
	JSON          interface{} `json:"json,omitempty"`
	ReadError     string      `json:"read_error,omitempty"`
	DecodeError   string      `json:"decode_error,omitempty"`
}

func (r *agentLiveRecorder) redact(body []byte) []byte {
	if r.secret == "" {
		return append([]byte{}, body...)
	}
	return bytes.ReplaceAll(body, []byte(r.secret), []byte("[redacted]"))
}
func (r *agentLiveRecorder) capture(raw []byte, encoding string, readErr error) agentLiveBody {
	saved := r.redact(raw)
	decoded := raw
	var decodeErr error
	if strings.EqualFold(encoding, "gzip") {
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		decodeErr = err
		if err == nil {
			decoded, decodeErr = io.ReadAll(reader)
			reader.Close()
		}
	}
	safeDecoded := r.redact(decoded)
	redacted := !bytes.Equal(raw, saved) || !bytes.Equal(decoded, safeDecoded)
	// Preserve encoded bytes too. When a compressed body echoes a credential,
	// re-encode the redacted content for the private report, never for forwarding.
	if strings.EqualFold(encoding, "gzip") && decodeErr == nil && !bytes.Equal(decoded, safeDecoded) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write(safeDecoded)
		_ = gz.Close()
		saved = buf.Bytes()
	}
	result := agentLiveBody{Bytes: append([]byte{}, saved...), ReceivedBytes: len(raw), SHA256: fmt.Sprintf("%x", sha256.Sum256(saved)), Complete: readErr == nil, Redacted: redacted}
	if readErr != nil {
		result.ReadError = string(r.redact([]byte(readErr.Error())))
	}
	if decodeErr != nil {
		result.DecodeError = string(r.redact([]byte(decodeErr.Error())))
	}
	if decodeErr == nil && (encoding == "" || strings.EqualFold(encoding, "identity") || strings.EqualFold(encoding, "gzip")) && utf8.Valid(safeDecoded) {
		text := string(safeDecoded)
		result.Text = &text
		_ = json.Unmarshal(safeDecoded, &result.JSON)
	}
	return result
}

type agentLiveReadError struct{ err error }

func (r agentLiveReadError) Read([]byte) (int, error) { return 0, r.err }
func replayAgentBody(raw []byte, err error) io.ReadCloser {
	if err != nil {
		return io.NopCloser(io.MultiReader(bytes.NewReader(raw), agentLiveReadError{err}))
	}
	return io.NopCloser(bytes.NewReader(raw))
}
func (r *agentLiveRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var requestBytes []byte
	var requestErr error
	if req.Body != nil {
		requestBytes, requestErr = io.ReadAll(req.Body)
		req.Body.Close()
		req.Body = replayAgentBody(requestBytes, requestErr)
	}
	record := agentLiveExchange{Method: req.Method, Path: req.URL.EscapedPath(), Request: r.capture(requestBytes, req.Header.Get("Content-Encoding"), requestErr)}
	response, err := r.transport.RoundTrip(req)
	if response != nil {
		record.Status = response.StatusCode
		record.ContentType = response.Header.Get("Content-Type")
		record.ContentEncoding = response.Header.Get("Content-Encoding")
		var body []byte
		var readErr error
		if response.Body != nil {
			body, readErr = io.ReadAll(response.Body)
			response.Body.Close()
		}
		response.Body = replayAgentBody(body, readErr)
		captured := r.capture(body, record.ContentEncoding, readErr)
		record.Response = &captured
	}
	if err != nil {
		record.TransportError = string(r.redact([]byte(err.Error())))
	}
	r.mu.Lock()
	r.exchanges = append(r.exchanges, record)
	if r.journal != nil {
		data, writeErr := json.Marshal(record)
		if writeErr == nil {
			data = append(data, '\n')
			var n int
			n, writeErr = r.journal.Write(data)
			if writeErr == nil && n != len(data) {
				writeErr = io.ErrShortWrite
			}
		}
		if writeErr != nil {
			r.writeErrors = append(r.writeErrors, writeErr)
		}
	}
	r.mu.Unlock()
	return response, err
}
func (r *agentLiveRecorder) drain() []agentLiveExchange {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := append([]agentLiveExchange{}, r.exchanges...)
	r.exchanges = nil
	return records
}
func (r *agentLiveRecorder) report(entry map[string]interface{}) error {
	exchanges := r.drain()
	entry["provider_exchanges"] = exchanges
	requests, replies := []interface{}{}, []interface{}{}
	rejections := []string{}
	for _, exchange := range exchanges {
		if exchange.Method != "POST" || !strings.HasSuffix(exchange.Path, "/chat/completions") {
			continue
		}
		requests = append(requests, exchange.Request.JSON)
		if exchange.Response == nil {
			continue
		}
		if exchange.Status < 200 || exchange.Status >= 300 {
			text := "base64:" + base64.StdEncoding.EncodeToString(exchange.Response.Bytes)
			if exchange.Response.Text != nil {
				text = *exchange.Response.Text
			}
			rejections = append(rejections, text)
		} else if exchange.Response.JSON != nil {
			replies = append(replies, exchange.Response.JSON)
		}
	}
	entry["provider_requests"] = requests
	entry["provider_replies"] = replies
	entry["provider_rejections"] = rejections
	r.mu.Lock()
	defer r.mu.Unlock()
	err := errors.Join(r.writeErrors...)
	r.writeErrors = nil
	return err
}
