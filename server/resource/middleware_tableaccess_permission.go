package resource

import (
	"github.com/artpar/api2go"
	//log "github.com/sirupsen/logrus"
	//"gopkg.in/Masterminds/squirrel.v1"
	"errors"

	"github.com/artpar/daptin/server/auth"
)

type TableAccessPermissionChecker struct {
}

func (pc *TableAccessPermissionChecker) String() string {
	return "TableAccessPermissionChecker"
}

func (pc *TableAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if results == nil || len(results) < 1 {
		return results, nil
	}

	//returnMap := make([]map[string]interface{}, 0)

	user := req.PlainRequest.Context().Value("user")
	sessionUser := auth.SessionUser{}

	if user != nil {
		sessionUser = user.(auth.SessionUser)

	}

	tableOwnership := dr.GetObjectPermissionByWhereClause("world", "table_name", dr.model.GetName())

	//notIncludedMapCache := make(map[string]bool)
	//includedMapCache := make(map[string]bool)

	//log.Infof("Result: %v", result)

	//referenceId := result["reference_id"].(string)
	//_, ok := notIncludedMapCache[referenceId]
	//if ok {
	//	continue
	//}
	//_, ok = includedMapCache[referenceId]
	//if ok {
	//	returnMap = append(returnMap, result)
	//	continue
	//}

	//permission := dr.GetRowPermission(result)
	//log.Infof("Row Permission for [%v] for [%v]", permission, result)
	if req.PlainRequest.Method == "GET" {
		if tableOwnership.CanRead(sessionUser.UserReferenceId, sessionUser.Groups) {
			//returnMap = append(returnMap, result)
			//includedMapCache[referenceId] = true
			return results, nil
		} else {
			//notIncludedMapCache[referenceId] = true
			return nil, ERR_UNAUTHORIZED
		}
	} else if tableOwnership.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
		//log.Infof("[TableAccessPermissionChecker] Result not to be included: %v", result["reference_id"])
		//returnMap = append(returnMap, result)
		//includedMapCache[referenceId] = true
		return results, nil
	}

	return nil, ERR_UNAUTHORIZED
}

var (
	ERR_UNAUTHORIZED = errors.New("Unauthorized")
)

func (pc *TableAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	//var err error
	//log.Infof("context: %v", context.GetAll(req.PlainRequest))

	user := req.PlainRequest.Context().Value("user")
	sessionUser := auth.SessionUser{}

	if user != nil {
		sessionUser = user.(auth.SessionUser)

	}

	tableOwnership := dr.GetObjectPermissionByWhereClause("world", "table_name", dr.model.GetName())

	//log.Infof("[TableAccessPermissionChecker] PermissionInstance check for type: [%v] on [%v] @%v", req.PlainRequest.Method, dr.model.GetName(), tableOwnership.PermissionInstance)
	if req.PlainRequest.Method == "GET" {
		if !tableOwnership.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ERR_UNAUTHORIZED
		}
	} else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" {
		if !tableOwnership.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ERR_UNAUTHORIZED

		}
	} else if req.PlainRequest.Method == "POST" {
		if !tableOwnership.CanCreate(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ERR_UNAUTHORIZED

		}
	} else if req.PlainRequest.Method == "DELETE" {
		if !tableOwnership.CanDelete(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ERR_UNAUTHORIZED

		}
	} else {
		return nil, ERR_UNAUTHORIZED
	}

	return results, nil

}
