package resource

import (
  "github.com/artpar/api2go"
)

type MiddlewareSet struct {
  BeforeCreate  []DatabaseRequestInterceptor
  BeforeFindAll []DatabaseRequestInterceptor
  BeforeFindOne []DatabaseRequestInterceptor
  BeforeUpdate  []DatabaseRequestInterceptor
  BeforeDelete  []DatabaseRequestInterceptor

  AfterCreate   []DatabaseRequestInterceptor
  AfterFindAll  []DatabaseRequestInterceptor
  AfterFindOne  []DatabaseRequestInterceptor
  AfterUpdate   []DatabaseRequestInterceptor
  AfterDelete   []DatabaseRequestInterceptor
}

type DatabaseRequestInterceptor interface {
  InterceptBefore(*DbResource, *api2go.Request) (api2go.Responder, error)
  InterceptAfter(*DbResource, *api2go.Request, []map[string]interface{}) ([]map[string]interface{}, error)
}
