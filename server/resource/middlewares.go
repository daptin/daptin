package resource

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/jmoiron/sqlx"
)

type MiddlewareSet struct {
	BeforeCreate  []DatabaseRequestInterceptor
	BeforeFindAll []DatabaseRequestInterceptor
	BeforeFindOne []DatabaseRequestInterceptor
	BeforeUpdate  []DatabaseRequestInterceptor
	BeforeDelete  []DatabaseRequestInterceptor

	AfterCreate  []DatabaseRequestInterceptor
	AfterFindAll []DatabaseRequestInterceptor
	AfterFindOne []DatabaseRequestInterceptor
	AfterUpdate  []DatabaseRequestInterceptor
	AfterDelete  []DatabaseRequestInterceptor
}

type DatabaseRequestInterceptor interface {
	InterceptBefore(*DbResource, *api2go.Request, []map[string]interface{}, *sqlx.Tx) ([]map[string]interface{}, error)
	InterceptAfter(*DbResource, *api2go.Request, []map[string]interface{}, *sqlx.Tx) ([]map[string]interface{}, error)
	fmt.Stringer
}

func lifecycleInterceptors(interceptors []DatabaseRequestInterceptor) []DatabaseRequestInterceptor {
	result := make([]DatabaseRequestInterceptor, 0, len(interceptors))
	for _, interceptor := range interceptors {
		switch interceptor.(type) {
		case *TableAccessPermissionChecker, *ObjectAccessPermissionChecker:
			continue
		default:
			result = append(result, interceptor)
		}
	}
	return result
}
