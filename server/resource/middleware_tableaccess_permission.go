package resource

import (
	"github.com/artpar/api2go"
	//log "github.com/sirupsen/logrus"
	//"gopkg.in/Masterminds/squirrel.v1"
	"errors"

	"github.com/artpar/daptin/server/auth"
)

// The TableAccessPermissionChecker middleware is resposible for entity level authorization check, before and after the changes
type TableAccessPermissionChecker struct {
}

func (pc *TableAccessPermissionChecker) String() string {
	return "TableAccessPermissionChecker"
}

// Intercept after check implements if the data should be returned after the data change is complete
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

	//log.Infof("Row Permission for [%v] for [%v]", permission, result)
	if req.PlainRequest.Method == "GET" {
		if tableOwnership.CanRead(sessionUser.UserReferenceId, sessionUser.Groups) {
			//returnMap = append(returnMap, result)
			//includedMapCache[referenceId] = true
			return results, nil
		} else {
			//notIncludedMapCache[referenceId] = true
			return nil, ErrUnauthorized
		}
	} else if tableOwnership.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
		//log.Infof("[TableAccessPermissionChecker] Result not to be included: %v", result["reference_id"])
		//returnMap = append(returnMap, result)
		//includedMapCache[referenceId] = true
		return results, nil
	}

	return nil, ErrUnauthorized
}

var (
	// Error Unauthorized
	ErrUnauthorized = errors.New("Unauthorized")
)

// Intercept before implemetation for entity level authentication check
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
			return nil, ErrUnauthorized
		}
	} else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" {
		if !tableOwnership.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ErrUnauthorized

		}
	} else if req.PlainRequest.Method == "POST" {
		if !tableOwnership.CanCreate(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ErrUnauthorized

		}
	} else if req.PlainRequest.Method == "DELETE" {
		if !tableOwnership.CanDelete(sessionUser.UserReferenceId, sessionUser.Groups) {
			return nil, ErrUnauthorized

		}
	} else {
		return nil, ErrUnauthorized
	}

	return results, nil

}
