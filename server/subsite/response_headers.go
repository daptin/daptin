package subsite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/http/httpguts"
)

const (
	maxSiteHeaderBytes = 16 << 10
	maxSiteHeaderCount = 32
)

// ResponseHeaders is a validated, immutable snapshot of a site's header policy.
type ResponseHeaders map[string]string

// ParseResponseHeaders accepts the JSON column's resource and stored representations.
// Null means no policy. Header names are canonicalized once at the owning boundary.
func ParseResponseHeaders(value interface{}) (ResponseHeaders, error) {
	if value == nil {
		return nil, nil
	}
	var encoded []byte
	var err error
	switch v := value.(type) {
	case string:
		encoded = []byte(v)
	case []byte:
		encoded = v
	default:
		encoded, err = json.Marshal(value)
	}
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxSiteHeaderBytes {
		return nil, fmt.Errorf("site headers exceed 16 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token == nil {
		if _, err = decoder.Token(); err != io.EOF {
			return nil, fmt.Errorf("unexpected data after site headers")
		}
		return nil, nil
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("site headers must be a JSON object of strings")
	}
	headers := make(ResponseHeaders)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, err
		}
		name := token.(string)
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, fmt.Errorf("invalid site header name %q", name)
		}
		name = http.CanonicalHeaderKey(name)
		if _, exists := headers[name]; exists {
			return nil, fmt.Errorf("duplicate site header %q", name)
		}
		switch name {
		case "Connection", "Content-Length", "Transfer-Encoding", "Content-Encoding", "Content-Type", "Content-Range", "Accept-Ranges", "Trailer", "Te", "Keep-Alive", "Upgrade", "Proxy-Authenticate", "Proxy-Authorization", "Etag", "Last-Modified", "Cache-Control", "Expires", "Vary", "Set-Cookie", "Www-Authenticate", "Location":
			return nil, fmt.Errorf("site header %q is managed by Daptin", name)
		}
		var value interface{}
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		text, ok := value.(string)
		if !ok || !httpguts.ValidHeaderFieldValue(text) {
			return nil, fmt.Errorf("site header %q requires a valid string value", name)
		}
		headers[name] = text
		if len(headers) > maxSiteHeaderCount {
			return nil, fmt.Errorf("site headers exceed 32 entries")
		}
	}
	if _, err = decoder.Token(); err != nil {
		return nil, err
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after site headers")
	}
	// The durable JSON representation escapes characters such as '<'. Bound
	// that representation too, so accepted input remains valid after storage.
	encoded, err = json.Marshal(headers)
	if err != nil || len(encoded) > maxSiteHeaderBytes {
		return nil, fmt.Errorf("site headers exceed 16 KiB")
	}
	return headers, nil
}

// Apply runs after site selection and before authentication or content handling.
// Forwarded request headers are not authority for the native TLS connection.
func (headers ResponseHeaders) Apply(w http.ResponseWriter, request *http.Request) {
	for name, value := range headers {
		if name == "Strict-Transport-Security" && request.TLS == nil {
			continue
		}
		w.Header().Set(name, value)
	}
}
