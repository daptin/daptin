package server

import (
	stdjson "encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultCorsConfigJSON = `{"version":"1","allowed_origins":[],"allowed_methods":[],"allowed_headers":[],"exposed_headers":[],"allow_credentials":false,"max_age":0}`
	maxCorsMaxAge         = 86400
)

var defaultCorsConfig = CorsConfig{
	Version:        "1",
	AllowedOrigins: []string{},
	AllowedMethods: []string{},
	AllowedHeaders: []string{},
	ExposedHeaders: []string{},
}

var supportedCorsMethods = map[string]struct{}{
	http.MethodGet:    {},
	http.MethodHead:   {},
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

// CorsConfig is the immutable CORS policy loaded when a Daptin member starts.
// Every cluster member reads the same _config value and applies the policy
// locally, avoiding database or distributed-cache work on the request path.
type CorsConfig struct {
	Version          string   `json:"version"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

type CorsMiddleware struct {
	allowedOrigins    map[string]struct{}
	allowedMethods    map[string]struct{}
	allowedHeaders    map[string]struct{}
	allowedMethodsCSV string
	allowedHeadersCSV string
	exposedHeadersCSV string
	allowCredentials  bool
	maxAge            int
}

func ParseCorsConfig(value string) (CorsConfig, error) {
	decoder := stdjson.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	var config CorsConfig
	if err := decoder.Decode(&config); err != nil {
		return CorsConfig{}, fmt.Errorf("parse JSON: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return CorsConfig{}, fmt.Errorf("parse JSON: multiple values")
		}
		return CorsConfig{}, fmt.Errorf("parse JSON: %w", err)
	}
	if config.Version != "1" {
		return CorsConfig{}, fmt.Errorf("unsupported version %q", config.Version)
	}
	if config.MaxAge < 0 || config.MaxAge > maxCorsMaxAge {
		return CorsConfig{}, fmt.Errorf("max_age must be between 0 and %d seconds", maxCorsMaxAge)
	}

	var err error
	config.AllowedOrigins, err = validateCorsOrigins(config.AllowedOrigins)
	if err != nil {
		return CorsConfig{}, err
	}
	config.AllowedMethods, err = validateCorsMethods(config.AllowedMethods)
	if err != nil {
		return CorsConfig{}, err
	}
	config.AllowedHeaders, err = validateCorsHeaders("allowed_headers", config.AllowedHeaders)
	if err != nil {
		return CorsConfig{}, err
	}
	config.ExposedHeaders, err = validateCorsHeaders("exposed_headers", config.ExposedHeaders)
	if err != nil {
		return CorsConfig{}, err
	}
	if config.AllowCredentials && len(config.AllowedOrigins) == 0 {
		return CorsConfig{}, fmt.Errorf("allow_credentials requires at least one allowed origin")
	}
	return config, nil
}

func validateCorsOrigins(origins []string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if origin == "" || origin != strings.TrimSpace(origin) || origin == "*" || strings.EqualFold(origin, "null") {
			return nil, fmt.Errorf("invalid allowed origin %q", origin)
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return nil, fmt.Errorf("allowed origin %q must be an absolute HTTP(S) origin without path, query, fragment, or credentials", origin)
		}
		if parsed.Path == "/" || strings.HasSuffix(origin, "/") {
			return nil, fmt.Errorf("allowed origin %q must not have a trailing slash", origin)
		}
		if _, err := strconv.Atoi(parsed.Port()); parsed.Port() != "" && err != nil {
			return nil, fmt.Errorf("allowed origin %q has an invalid port", origin)
		}
		canonical := canonicalParsedOrigin(parsed)
		if _, exists := seen[canonical]; exists {
			return nil, fmt.Errorf("duplicate allowed origin %q", origin)
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateCorsMethods(methods []string) ([]string, error) {
	normalized := make([]string, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if !validHTTPToken(method) {
			return nil, fmt.Errorf("invalid or unsupported CORS method %q", method)
		}
		if _, supported := supportedCorsMethods[method]; !supported {
			return nil, fmt.Errorf("invalid or unsupported CORS method %q", method)
		}
		if _, exists := seen[method]; exists {
			return nil, fmt.Errorf("duplicate allowed method %q", method)
		}
		seen[method] = struct{}{}
		normalized = append(normalized, method)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateCorsHeaders(field string, headers []string) ([]string, error) {
	normalized := make([]string, 0, len(headers))
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if !validHTTPToken(header) {
			return nil, fmt.Errorf("invalid %s value %q", field, header)
		}
		header = http.CanonicalHeaderKey(header)
		key := strings.ToLower(header)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate %s value %q", field, header)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, header)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))) {
			return false
		}
	}
	return true
}

func NewCorsMiddleware(config CorsConfig) *CorsMiddleware {
	middleware := &CorsMiddleware{
		allowedOrigins:    make(map[string]struct{}, len(config.AllowedOrigins)),
		allowedMethods:    make(map[string]struct{}, len(config.AllowedMethods)),
		allowedHeaders:    make(map[string]struct{}, len(config.AllowedHeaders)),
		allowedMethodsCSV: strings.Join(config.AllowedMethods, ", "),
		allowedHeadersCSV: strings.Join(config.AllowedHeaders, ", "),
		exposedHeadersCSV: strings.Join(config.ExposedHeaders, ", "),
		allowCredentials:  config.AllowCredentials,
		maxAge:            config.MaxAge,
	}
	for _, origin := range config.AllowedOrigins {
		middleware.allowedOrigins[origin] = struct{}{}
	}
	for _, method := range config.AllowedMethods {
		middleware.allowedMethods[method] = struct{}{}
	}
	for _, header := range config.AllowedHeaders {
		middleware.allowedHeaders[strings.ToLower(header)] = struct{}{}
	}
	return middleware
}

func (cm *CorsMiddleware) CorsMiddlewareFunc(c *gin.Context) {
	// The representation can change when an Origin header is added, so Vary
	// must also be present on responses to requests which did not send Origin.
	addVary(c.Writer.Header(), "Origin")
	origin := c.GetHeader("Origin")
	if origin == "" {
		c.Next()
		return
	}

	isPreflight := c.Request.Method == http.MethodOptions
	if isPreflight {
		addVary(c.Writer.Header(), "Access-Control-Request-Method")
		addVary(c.Writer.Header(), "Access-Control-Request-Headers")
	}
	canonicalOrigin, validOrigin := canonicalRequestOrigin(origin)
	_, allowedOrigin := cm.allowedOrigins[canonicalOrigin]
	if !validOrigin || !allowedOrigin {
		if isPreflight {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
		return
	}

	if isPreflight {
		method := strings.ToUpper(strings.TrimSpace(c.GetHeader("Access-Control-Request-Method")))
		if _, allowed := cm.allowedMethods[method]; !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if !cm.requestHeadersAllowed(c.GetHeader("Access-Control-Request-Headers")) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		cm.setCommonHeaders(c, origin)
		if cm.allowedMethodsCSV != "" {
			c.Header("Access-Control-Allow-Methods", cm.allowedMethodsCSV)
		}
		if cm.allowedHeadersCSV != "" {
			c.Header("Access-Control-Allow-Headers", cm.allowedHeadersCSV)
		}
		if cm.maxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(cm.maxAge))
		}
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	cm.setCommonHeaders(c, origin)
	c.Next()
}

func canonicalRequestOrigin(origin string) (string, bool) {
	if origin == "" || origin != strings.TrimSpace(origin) || origin == "*" || strings.EqualFold(origin, "null") {
		return "", false
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	return canonicalParsedOrigin(parsed), true
}

func canonicalParsedOrigin(parsed *url.URL) string {
	scheme := strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	return scheme + "://" + host
}

func (cm *CorsMiddleware) requestHeadersAllowed(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, requested := range strings.Split(value, ",") {
		requested = strings.TrimSpace(requested)
		if !validHTTPToken(requested) {
			return false
		}
		if _, allowed := cm.allowedHeaders[strings.ToLower(requested)]; !allowed {
			return false
		}
	}
	return true
}

func (cm *CorsMiddleware) setCommonHeaders(c *gin.Context, origin string) {
	c.Header("Access-Control-Allow-Origin", origin)
	if cm.allowCredentials {
		c.Header("Access-Control-Allow-Credentials", "true")
	}
	if cm.exposedHeadersCSV != "" {
		c.Header("Access-Control-Expose-Headers", cm.exposedHeadersCSV)
	}
}

func addVary(header http.Header, value string) {
	for _, existingLine := range header.Values("Vary") {
		for _, existing := range strings.Split(existingLine, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}
