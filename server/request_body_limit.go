package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/gin-gonic/gin"
)

const (
	defaultBufferedRequestBodyLimit = 10 * 1024 * 1024
	maxBufferedRequestBodyLimit     = 16 * 1024 * 1024
	// A draft sends Base64 attachments plus JSON-escaped text/HTML. Its wire
	// budget is separate from the decoded and serialized MIME limits.
	maxMailDraftRequestBodyLimit = 48 * 1024 * 1024
)

func parseBufferedRequestBodyLimit(value string) (int, error) {
	return parseRequestBodyLimit(value, "http.max_buffered_request_bytes", maxBufferedRequestBodyLimit)
}

func parseRequestBodyLimit(value, setting string, maximum int) (int, error) {
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > maximum {
		return 0, fmt.Errorf("%s must be an integer from 1 to %d", setting, maximum)
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

func bufferedRequestBodyMiddleware(limit, mailDraftLimit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bufferedRequestPath(c.Request.Method, c.Request.URL.Path) {
			requestLimit := limit
			if c.Request.Method == http.MethodPost && c.Request.URL.Path == "/action/mail_account/draft_save" {
				user, _ := c.Request.Context().Value("user").(*auth.SessionUser)
				if user != nil && user.UserId > 0 && user.UserReferenceId != daptinid.NullReferenceId {
					requestLimit = mailDraftLimit
				}
			}
			_, status := readBoundedRequestBody(c.Request, requestLimit)
			if status != 0 {
				c.AbortWithStatus(status)
				return
			}
		}
		c.Next()
	}
}
