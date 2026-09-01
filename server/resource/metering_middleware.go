package resource

import (
	"context"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/jmoiron/sqlx"
)

type meteringDecisionContextKey struct{}

type MeteringMiddleware struct {
	service *MeteringService
}

func NewMeteringMiddleware(cruds *map[string]*DbResource) DatabaseRequestInterceptor {
	return &MeteringMiddleware{service: NewMeteringService(cruds)}
}

func (m *MeteringMiddleware) String() string {
	return "metering"
}

func (m *MeteringMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, rows []map[string]interface{}, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	if !shouldMeterResource(dr, req) {
		return rows, nil
	}
	user := sessionUserFromAPIRequest(req)
	if user == nil || user.UserId == 0 {
		return rows, nil
	}
	decision, err := m.service.Admit(MeteringContext{
		Request:     req.PlainRequest,
		User:        user,
		Endpoint:    endpointFromRequest(req),
		Method:      req.PlainRequest.Method,
		EntityType:  dr.TableInfo().TableName,
		RequestType: "crud",
		Metering:    dr.TableInfo().Metering,
		Metadata: map[string]interface{}{
			"phase": "preflight",
			"table": dr.TableInfo().TableName,
		},
	}, tx)
	if err != nil {
		return nil, err
	}
	req.PlainRequest = req.PlainRequest.WithContext(context.WithValue(req.PlainRequest.Context(), meteringDecisionContextKey{}, decision))
	return rows, nil
}

func (m *MeteringMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, rows []map[string]interface{}, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	if !shouldMeterResource(dr, req) {
		return rows, nil
	}
	user := sessionUserFromAPIRequest(req)
	if user == nil || user.UserId == 0 {
		return rows, nil
	}
	start := time.Now()
	response := map[string]interface{}{
		"rows": rows,
	}
	decision, _ := req.PlainRequest.Context().Value(meteringDecisionContextKey{}).(*MeteringDecision)
	err := m.service.Complete(MeteringContext{
		Request:       req.PlainRequest,
		User:          user,
		Endpoint:      endpointFromRequest(req),
		Method:        req.PlainRequest.Method,
		EntityType:    dr.TableInfo().TableName,
		RequestType:   "crud",
		StatusCode:    statusCodeForMethod(req.PlainRequest.Method),
		LatencyMS:     int(time.Since(start).Milliseconds()),
		RequestBytes:  requestContentLength(req.PlainRequest.ContentLength),
		ResponseBytes: len(ToJson(response)),
		Metering:      dr.TableInfo().Metering,
		Metadata: map[string]interface{}{
			"table":     dr.TableInfo().TableName,
			"row_count": len(rows),
		},
		Response: response,
	}, decision, tx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func requestContentLength(contentLength int64) int {
	if contentLength < 0 {
		return 0
	}
	return int(contentLength)
}

func shouldMeterResource(dr *DbResource, req *api2go.Request) bool {
	if dr == nil || dr.TableInfo() == nil || req == nil || req.PlainRequest == nil {
		return false
	}
	if IsMeteringInternalRequest(req.PlainRequest) {
		return false
	}
	if IsMeteringSystemTable(dr.TableInfo().TableName) {
		return false
	}
	cfg := dr.TableInfo().Metering
	return cfg != nil && cfg.Enabled
}

func sessionUserFromAPIRequest(req *api2go.Request) *auth.SessionUser {
	if req == nil || req.PlainRequest == nil {
		return nil
	}
	user := req.PlainRequest.Context().Value("user")
	if user == nil {
		return nil
	}
	sessionUser, _ := user.(*auth.SessionUser)
	return sessionUser
}

func endpointFromRequest(req *api2go.Request) string {
	if req == nil || req.PlainRequest == nil || req.PlainRequest.URL == nil {
		return ""
	}
	return req.PlainRequest.URL.Path
}

func statusCodeForMethod(method string) int {
	switch method {
	case "POST":
		return 201
	case "DELETE":
		return 204
	default:
		return 200
	}
}
