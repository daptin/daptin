package server

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	defaultGraphQLRequestBodyLimit = 10 * 1024 * 1024
	maxGraphQLRequestBodyLimit     = 64 * 1024 * 1024
)

func parseGraphQLRequestBodyLimit(value string) (int, error) {
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > maxGraphQLRequestBodyLimit {
		return 0, fmt.Errorf("graphql.max_request_bytes must be an integer from 1 to %d", maxGraphQLRequestBodyLimit)
	}
	return limit, nil
}

type meteringPayloadWriter struct {
	gin.ResponseWriter
	capture *resource.MeteringPayloadCapture
	body    bytes.Buffer
	bytes   int
}

func (writer *meteringPayloadWriter) Write(value []byte) (int, error) {
	n, err := writer.ResponseWriter.Write(value)
	if writer.capture.HasReservation() {
		writer.bytes += n
		if remaining := writer.capture.BodyLimit() - writer.body.Len(); remaining > 0 {
			writer.body.Write(value[:min(n, remaining)])
		}
	}
	return n, err
}

func (writer *meteringPayloadWriter) WriteString(value string) (int, error) {
	n, err := writer.ResponseWriter.WriteString(value)
	if writer.capture.HasReservation() {
		writer.bytes += n
		if remaining := writer.capture.BodyLimit() - writer.body.Len(); remaining > 0 {
			writer.body.WriteString(value[:min(n, remaining)])
		}
	}
	return n, err
}

func meteringPayloadMiddleware(cruds *map[string]*resource.DbResource, graphqlRequestBodyLimit int) gin.HandlerFunc {
	service := resource.NewMeteringService(cruds)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !meteringPayloadPathEnabled(path, *cruds) {
			c.Next()
			return
		}
		var graphqlBody []byte
		if path == "/graphql" && c.Request.Method == http.MethodPost && c.Request.Body != nil {
			body, status := readBoundedRequestBody(c.Request, graphqlRequestBodyLimit)
			if status != 0 {
				c.AbortWithStatus(status)
				return
			}
			graphqlBody = body
		}
		if path == "/graphql" && !graphQLOperationWithinSelectionLimit(c.Request, graphqlBody) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "GraphQL operation exceeds 256 selections"})
			return
		}
		request, capture := resource.NewMeteringPayloadCapture(c.Request, resource.MeteringCaptureLimit(meteringPayloadPathConfig(path, *cruds)))
		c.Request = request
		if path == "/graphql" {
			c.Next()
			return
		}
		writer := &meteringPayloadWriter{ResponseWriter: c.Writer, capture: capture}
		c.Writer = writer
		c.Next()
		reservations := capture.Reservations()
		if len(reservations) != 1 {
			return
		}
		transaction, err := (*cruds)["api_usage"].Connection().Beginx()
		if err != nil {
			log.Errorf("begin metering payload recording: %v", err)
			return
		}
		defer transaction.Rollback()
		reservation := reservations[0]
		if err := service.RecordResponseBody(reservation.Owner, reservation.Token, writer.body.Bytes(), writer.bytes, transaction); err != nil {
			log.Errorf("record metering response payload: %v", err)
			return
		}
		if err := transaction.Commit(); err != nil {
			log.Errorf("commit metering response payload: %v", err)
		}
	}
}

func meteringPayloadPathEnabled(path string, cruds map[string]*resource.DbResource) bool {
	return resource.MeteringConfigActive(meteringPayloadPathConfig(path, cruds))
}

func meteringPayloadPathConfig(path string, cruds map[string]*resource.DbResource) *table_info.MeteringConfig {
	if path == "/graphql" {
		return &table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true, MaxBytes: 16 << 20}}
	}
	if strings.HasPrefix(path, "/api/") {
		tableName := strings.SplitN(strings.TrimPrefix(path, "/api/"), "/", 2)[0]
		if crud := cruds[tableName]; crud != nil {
			return crud.TableInfo().Metering
		}
		return nil
	}
	if strings.HasPrefix(path, "/action/") {
		parts := strings.SplitN(strings.TrimPrefix(path, "/action/"), "/", 2)
		if len(parts) != 2 {
			return nil
		}
		if crud := cruds[parts[0]]; crud != nil {
			return resource.MeteringConfigForAction(crud.TableInfo().Metering, parts[1])
		}
		return nil
	}
	if strings.HasPrefix(path, "/integration/") {
		if crud := cruds["integration"]; crud != nil {
			return resource.MeteringConfigForAction(crud.TableInfo().Metering, strings.TrimPrefix(path, "/integration/"))
		}
	}
	return nil
}
