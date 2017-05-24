package dbapi

import "github.com/artpar/api2go"

type Response struct {
  metadata   map[string]interface{}
  result     interface{}
  statusCode int
}

func NewResponse(metadata map[string]interface{}, result interface{}, statusCode int) api2go.Responder {
  return Response{
    metadata:metadata,
    result:result,
    statusCode:statusCode,
  }
}

func (r Response) Metadata() map[string]interface{} {
  return r.metadata
}
func (r Response) Result() interface{} {
  return r.result
}
func (r Response) StatusCode() int {
  return r.statusCode
}

type ErrorResponse struct {
  Message string
}