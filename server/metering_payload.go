package server

import (
	"bytes"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type meteringPayloadWriter struct {
	gin.ResponseWriter
	capture *resource.MeteringPayloadCapture
	body    bytes.Buffer
}

func (writer *meteringPayloadWriter) Write(value []byte) (int, error) {
	n, err := writer.ResponseWriter.Write(value)
	if writer.capture.HasReservation() {
		writer.body.Write(value[:n])
	}
	return n, err
}

func (writer *meteringPayloadWriter) WriteString(value string) (int, error) {
	n, err := writer.ResponseWriter.WriteString(value)
	if writer.capture.HasReservation() {
		writer.body.WriteString(value[:n])
	}
	return n, err
}

func meteringPayloadMiddleware(cruds *map[string]*resource.DbResource) gin.HandlerFunc {
	service := resource.NewMeteringService(cruds)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !meteringPayloadPathEnabled(path, *cruds) {
			c.Next()
			return
		}
		request, capture := resource.NewMeteringPayloadCapture(c.Request)
		c.Request = request
		writer := &meteringPayloadWriter{ResponseWriter: c.Writer, capture: capture}
		c.Writer = writer
		c.Next()
		reservations := capture.Reservations()
		if len(reservations) == 0 {
			return
		}
		transaction, err := (*cruds)["api_usage"].Connection().Beginx()
		if err != nil {
			log.Errorf("begin metering payload recording: %v", err)
			return
		}
		defer transaction.Rollback()
		for _, reservation := range reservations {
			if err := service.RecordResponseBody(reservation.Owner, reservation.Token, writer.body.Bytes(), transaction); err != nil {
				log.Errorf("record metering response payload: %v", err)
				return
			}
		}
		if err := transaction.Commit(); err != nil {
			log.Errorf("commit metering response payload: %v", err)
		}
	}
}

func meteringPayloadPathEnabled(path string, cruds map[string]*resource.DbResource) bool {
	if path == "/graphql" {
		return true
	}
	if strings.HasPrefix(path, "/api/") {
		tableName := strings.SplitN(strings.TrimPrefix(path, "/api/"), "/", 2)[0]
		crud := cruds[tableName]
		return crud != nil && crud.TableInfo().Metering != nil && crud.TableInfo().Metering.Enabled
	}
	if strings.HasPrefix(path, "/action/") {
		parts := strings.SplitN(strings.TrimPrefix(path, "/action/"), "/", 2)
		if len(parts) != 2 {
			return false
		}
		crud := cruds[parts[0]]
		if crud == nil {
			return false
		}
		config := resource.MeteringConfigForAction(crud.TableInfo().Metering, parts[1])
		return config != nil && config.Enabled
	}
	if strings.HasPrefix(path, "/integration/") {
		crud := cruds["integration"]
		if crud == nil {
			return false
		}
		config := resource.MeteringConfigForAction(crud.TableInfo().Metering, strings.TrimPrefix(path, "/integration/"))
		return config != nil && config.Enabled
	}
	return false
}
