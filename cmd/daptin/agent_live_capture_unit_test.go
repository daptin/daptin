//go:build live

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type agentCaptureTransportFunc func(*http.Request) (*http.Response, error)

func (f agentCaptureTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentRecorderRetainsEveryResponseBodyWithoutChangingForwardedBytes(t *testing.T) {
	var gzipBody bytes.Buffer
	gz := gzip.NewWriter(&gzipBody)
	_, _ = gz.Write([]byte(`{"error":"gone"}`))
	_ = gz.Close()
	for _, test := range []struct {
		name           string
		status         int
		kind, encoding string
		body           []byte
	}{
		{"json array error", 410, "application/json", "", []byte(`[{"error":{"message":"endpoint gone"}}]`)},
		{"large HTML error", 502, "text/html", "", []byte("<html>" + strings.Repeat("error details\n", 20000) + "</html>")},
		{"empty response", 204, "", "", []byte{}},
		{"binary error", 500, "application/octet-stream", "", []byte{0, 255, 254, 1, 2}},
		{"gzip error", 410, "application/json", "gzip", gzipBody.Bytes()},
		{"invalid gzip", 500, "application/json", "gzip", []byte{31, 139}},
		{"redirect body", 302, "text/plain", "", []byte("moved")},
		{"malformed JSON", 200, "application/json", "", []byte(`{"broken":`)},
		{"event stream", 200, "text/event-stream", "", []byte("data: {\"answer\":1}\n\ndata: [DONE]\n\n")},
		{"successful body", 200, "application/json", "", []byte(`{"id":"full-response","choices":[],"usage":{},"provider_extension":"retain me"}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fake-secret" {
					t.Error("credential header was changed")
				}
				w.Header().Set("Content-Type", test.kind)
				if test.encoding != "" {
					w.Header().Set("Content-Encoding", test.encoding)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write(test.body)
			}))
			defer server.Close()
			var journal bytes.Buffer
			recorder := &agentLiveRecorder{transport: http.DefaultTransport, secret: "fake-secret", journal: &journal}
			request, _ := http.NewRequest("POST", server.URL+"/chat/completions", strings.NewReader(`{"model":"test"}`))
			request.Header.Set("Authorization", "Bearer fake-secret")
			request.Header.Set("Accept-Encoding", "identity")
			response, err := recorder.RoundTrip(request)
			if err != nil {
				t.Fatal(err)
			}
			var journaled agentLiveExchange
			if err := json.Unmarshal(bytes.TrimSpace(journal.Bytes()), &journaled); err != nil || journaled.Response == nil || !bytes.Equal(journaled.Response.Bytes, test.body) {
				t.Fatal("response body was not journaled before forwarding")
			}
			forwarded, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(forwarded, test.body) {
				t.Fatal("recorder altered or truncated forwarded body")
			}
			entry := map[string]interface{}{}
			if err := recorder.report(entry); err != nil {
				t.Fatal(err)
			}
			records := entry["provider_exchanges"].([]agentLiveExchange)
			if len(records) != 1 {
				t.Fatalf("got %d exchanges", len(records))
			}
			got := records[0]
			if got.Status != test.status || got.Response == nil || !got.Response.Complete || got.Response.ReceivedBytes != len(test.body) || !bytes.Equal(got.Response.Bytes, test.body) {
				t.Fatalf("incomplete capture: %#v", got)
			}
			serialized, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(serialized, []byte("fake-secret")) {
				t.Fatal("credential header leaked into capture")
			}
			var saved struct {
				Response struct {
					Body string `json:"body_base64"`
				} `json:"response"`
			}
			if err = json.Unmarshal(serialized, &saved); err != nil {
				t.Fatal(err)
			}
			restored, err := base64.StdEncoding.DecodeString(saved.Response.Body)
			if err != nil || !bytes.Equal(restored, test.body) {
				t.Fatal("body bytes lost during JSON persistence")
			}
			if test.name == "gzip error" && (got.Response.Text == nil || *got.Response.Text != `{"error":"gone"}`) {
				t.Fatal("compressed body not available as decoded text")
			}
			if test.name == "successful body" {
				replies := entry["provider_replies"].([]interface{})
				if replies[0].(map[string]interface{})["provider_extension"] != "retain me" {
					t.Fatal("complete successful envelope was not retained")
				}
			}
		})
	}
}

func TestAgentRecorderReportsRetentionWriteFailures(t *testing.T) {
	reader, writer := io.Pipe()
	reader.CloseWithError(io.ErrClosedPipe)
	defer writer.Close()
	recorder := &agentLiveRecorder{journal: writer, transport: agentCaptureTransportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 410, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("gone"))}, nil
	})}
	request, _ := http.NewRequest("GET", "http://example.test/error", nil)
	response, err := recorder.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	entry := map[string]interface{}{}
	if err := recorder.report(entry); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("retention failure was lost: %v", err)
	}
	if string(entry["provider_exchanges"].([]agentLiveExchange)[0].Response.Bytes) != "gone" {
		t.Fatal("in-memory body was lost after journal failure")
	}
}

func TestAgentRecorderRetainsPartialBodyAndTransportFailure(t *testing.T) {
	recorder := &agentLiveRecorder{transport: agentCaptureTransportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 502, Header: make(http.Header), Body: io.NopCloser(io.MultiReader(strings.NewReader("partial body"), agentLiveReadError{io.ErrUnexpectedEOF}))}, nil
	})}
	request, _ := http.NewRequest("GET", "http://example.test/error", nil)
	response, err := recorder.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "partial body" || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("forwarded read failure was changed")
	}
	record := recorder.drain()[0]
	if record.Response == nil || record.Response.Complete || string(record.Response.Bytes) != "partial body" || record.Response.ReadError == "" {
		t.Fatal("partial error body was discarded")
	}
	recorder.transport = agentCaptureTransportFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	_, _ = recorder.RoundTrip(request)
	failed := recorder.drain()[0]
	if failed.Response != nil || failed.TransportError == "" {
		t.Fatal("transport failure must distinguish no HTTP response from an empty body")
	}
}

func TestAgentRecorderRedactsCredentialsIncludingCompressedErrors(t *testing.T) {
	for _, encoding := range []string{"", "gzip"} {
		raw := []byte(`{"error":"fake-secret","other":"retain this"}`)
		if encoding == "gzip" {
			var b bytes.Buffer
			z := gzip.NewWriter(&b)
			_, _ = z.Write(raw)
			_ = z.Close()
			raw = b.Bytes()
		}
		recorder := &agentLiveRecorder{secret: "fake-secret"}
		captured := recorder.capture(raw, encoding, nil)
		if !captured.Redacted || captured.Text == nil || strings.Contains(*captured.Text, "fake-secret") || !strings.Contains(*captured.Text, "retain this") {
			t.Fatal("error credential was not safely redacted")
		}
		saved := captured.Bytes
		if encoding == "gzip" {
			z, err := gzip.NewReader(bytes.NewReader(saved))
			if err != nil {
				t.Fatal(err)
			}
			saved, err = io.ReadAll(z)
			z.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if bytes.Contains(saved, []byte("fake-secret")) {
			t.Fatal("stored body retained credential")
		}
	}
}
