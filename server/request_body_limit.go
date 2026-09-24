package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultBufferedRequestBodyLimit = 10 * 1024 * 1024
	maxBufferedRequestBodyLimit     = 16 * 1024 * 1024
)

func parseBufferedRequestBodyLimit(value string) (int, error) {
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > maxBufferedRequestBodyLimit {
		return 0, fmt.Errorf("http.max_buffered_request_bytes must be an integer from 1 to %d", maxBufferedRequestBodyLimit)
	}
	return limit, nil
}

// readBoundedRequestBody restores the body for the handler after checking its size.
// A known oversized length is rejected without reading from the connection.
func readBoundedRequestBody(request *http.Request, limit int) ([]byte, int) {
	if request.Body == nil || request.Body == http.NoBody {
		return nil, 0
	}
	if request.ContentLength > int64(limit) {
		return nil, http.StatusRequestEntityTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, int64(limit)+1))
	request.Body.Close()
	if err != nil {
		return nil, http.StatusBadRequest
	}
	if len(body) > limit {
		return nil, http.StatusRequestEntityTooLarge
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, 0
}

func bufferedRequestPath(method, path string) bool {
	switch {
	case strings.HasPrefix(path, "/action/"):
		return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete || method == http.MethodGet
	case strings.HasPrefix(path, "/api/"):
		return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
	case strings.HasPrefix(path, "/integration/"):
		return method == http.MethodPost
	case strings.HasPrefix(path, "/track/start/"):
		return method == http.MethodPost
	}
	return false
}

func bufferedRequestBodyMiddleware(limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bufferedRequestPath(c.Request.Method, c.Request.URL.Path) {
			_, status := readBoundedRequestBody(c.Request, limit)
			if status != 0 {
				c.AbortWithStatus(status)
				return
			}
		}
		c.Next()
	}
}
