package resource

import (
	"github.com/artpar/api2go"
	//log "github.com/sirupsen/logrus"
	//"gopkg.in/Masterminds/squirrel.v1"

	"github.com/daptin/daptin/server/auth"
	"strings"
)

type ObjectAccessPermissionChecker struct {
}

func (pc *ObjectAccessPermissionChecker) String() string {
	return "ObjectAccessPermissionChecker"
}

func (pc *ObjectAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if results == nil || len(results) < 1 {
		return results, nil
	}

	returnMap := make([]map[string]interface{}, 0)

	user := req.PlainRequest.Context().Value("user")
	sessionUser := auth.SessionUser{}

	if user != nil {
		sessionUser = user.(auth.SessionUser)

	}

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)

		referenceId := result["reference_id"].(string)
		_, ok := notIncludedMapCache[referenceId]
		if ok {
			continue
		}
		_, ok = includedMapCache[referenceId]
		if ok {
			returnMap = append(returnMap, result)
			continue
		}

		permission := dr.GetRowPermission(result)
		//log.Infof("Row Permission for [%v] for [%v]", permission, result)

		if req.PlainRequest.Method == "GET" {
			if permission.CanRead(sessionUser.UserReferenceId, sessionUser.Groups) {
				returnMap = append(returnMap, result)
				includedMapCache[referenceId] = true
			} else {
				notIncludedMapCache[referenceId] = true
			}
		} else if permission.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
			returnMap = append(returnMap, result)
			includedMapCache[referenceId] = true
		} else {
			//log.Infof("[ObjectAccessPermissionChecker] Result not to be included: %v", result["reference_id"])
			notIncludedMapCache[referenceId] = true
		}
	}

	return returnMap, nil

}

func (pc *ObjectAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if req.PlainRequest.Method == "POST" {
		return results, nil
	}

	//var err error
	//log.Infof("context: %v", context.GetAll(req.PlainRequest))

	user := req.PlainRequest.Context().Value("user")
	sessionUser := auth.SessionUser{}

	if user != nil {
		sessionUser = user.(auth.SessionUser)

	}

	returnMap := make([]map[string]interface{}, 0)

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)
		refIdInterface := result["reference_id"]
		referenceId := refIdInterface.(string)

		if strings.Index(result["__type"].(string), "_has_") > -1 {
			returnMap = append(returnMap, result)
			includedMapCache[referenceId] = true
			continue
		}

		if refIdInterface == nil {
			returnMap = append(returnMap, result)
			continue
		}
		_, ok := notIncludedMapCache[referenceId]
		if ok {
			continue
		}
		_, ok = includedMapCache[referenceId]
		if ok {
			returnMap = append(returnMap, result)
			continue
		}

		permission := dr.GetRowPermission(result)
		//log.Infof("[ObjectAccessPermissionChecker] PermissionInstance check for type: [%v] on [%v] @%v", req.PlainRequest.Method, dr.model.GetName(), permission.PermissionInstance)
		//log.Infof("Row Permission for [%v] for [%v]", permission, result)

		if req.PlainRequest.Method == "GET" {
			if permission.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
				returnMap = append(returnMap, result)
				includedMapCache[referenceId] = true
			} else {
				//log.Infof("[ObjectAccessPermissionChecker] Result not to be included: %v", refIdInterface)
				notIncludedMapCache[referenceId] = true

			}
		} else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" {
			if permission.CanUpdate(sessionUser.UserReferenceId, sessionUser.Groups) {
				returnMap = append(returnMap, result)
				includedMapCache[referenceId] = true
			} else {
				//log.Infof("[ObjectAccessPermissionChecker] Result not to be included: %v", refIdInterface)
				notIncludedMapCache[referenceId] = true
			}
		} else if req.PlainRequest.Method == "DELETE" {
			if permission.CanDelete(sessionUser.UserReferenceId, sessionUser.Groups) {
				returnMap = append(returnMap, result)
				includedMapCache[referenceId] = true
			} else {
				//log.Infof("[ObjectAccessPermissionChecker] Result not to be included: %v", refIdInterface)
				notIncludedMapCache[referenceId] = true
			}
		} else {
			continue
		}
	}

	return returnMap, nil

}
