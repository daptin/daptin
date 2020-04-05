package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	"strings"

	//"github.com/Masterminds/squirrel"

	"github.com/daptin/daptin/server/auth"
	//"strings"
	"fmt"
)

type ObjectAccessPermissionChecker struct {
}

func (pc *ObjectAccessPermissionChecker) String() string {
	return "ObjectAccessPermissionChecker"
}

func (pc *ObjectAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	originalCount := len(results)
	if results == nil || originalCount < 1 {
		return results, nil
	}

	returnMap := make([]map[string]interface{}, 0)

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	adminId := dr.GetAdminReferenceId()
	isAdmin := adminId != "" && adminId == sessionUser.UserReferenceId

	if isAdmin {
		return results, nil
	}

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)

		if result == nil {
			continue
		}

		if strings.Index(result["__type"].(string), ".") > -1 {
			log.Infof("Included object is an image")
			returnMap = append(returnMap, result)
			continue
		}

		//log.Infof("Check permission for : %v", result)

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
func BeginsWith(longerString string, smallerString string) bool {
	if len(smallerString) > len(longerString) {
		return false
	}
	return strings.ToLower(longerString)[0:len(smallerString)] == strings.ToLower(smallerString)
}

func (pc *ObjectAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if req.PlainRequest.Method == "POST" {
		return results, nil
	}

	//var err error
	//log.Infof("context: %v", context.GetAll(req.PlainRequest))

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	adminId := dr.GetAdminReferenceId()
	isAdmin := adminId != "" && adminId == sessionUser.UserReferenceId

	if isAdmin {
		return results, nil
	}

	returnMap := make([]map[string]interface{}, 0)

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)
		refIdInterface := result["reference_id"]

		//if strings.Index(result["__type"].(string), "_has_") > -1 {
		//	returnMap = append(returnMap, result)
		//	includedMapCache[referenceId] = true
		//	continue
		//}

		if refIdInterface == nil {
			returnMap = append(returnMap, result)
			continue
		}
		referenceId := refIdInterface.(string)
		_, ok := notIncludedMapCache[referenceId]
		if ok {
			continue
		}
		_, ok = includedMapCache[referenceId]
		if ok {
			returnMap = append(returnMap, result)
			continue
		}

		originalRowReference := map[string]interface{}{
			"__type":                result["__type"],
			"reference_id":          result["reference_id"],
			"relation_reference_id": result["relation_reference_id"],
		}
		permission := dr.GetRowPermission(originalRowReference)
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

	if len(results) != 0 && len(returnMap) == 0 {
		return returnMap, api2go.NewHTTPError(fmt.Errorf("%v", "forbidden"), pc.String(), 403)
	}

	return returnMap, nil

}
