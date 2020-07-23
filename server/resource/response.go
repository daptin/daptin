package resource

import "github.com/artpar/api2go"

//type Response struct {
//  metadata   map[string]interface{}
//  result     interface{}
//  statusCode int
//}

func NewResponse(metadata map[string]interface{}, result interface{}, statusCode int, pagination *api2go.Pagination) api2go.Responder {
	if pagination != nil {

		return api2go.Response{
			Meta:       metadata,
			Res:        result,
			Pagination: *pagination,
			Code:       statusCode,
		}
	} else {
		return api2go.Response{
			Meta: metadata,
			Res:  result,
			Code: statusCode,
		}
	}
}

//func (r Response) Metadata() map[string]interface{} {
//  return r.metadata
//}
//func (r Response) Result() interface{} {
//  return r.result
//}
//func (r Response) StatusCode() int {
//  return r.statusCode
//}

//type ErrorResponse struct {
//  Message string
//}
