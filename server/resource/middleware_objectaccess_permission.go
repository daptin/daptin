package resource

import (
	"github.com/artpar/api2go"
	//log "github.com/sirupsen/logrus"
	//"gopkg.in/Masterminds/squirrel.v1"

	"github.com/artpar/goms/server/auth"
)

type ObjectAccessPermissionChecker struct {
}

func (pc *ObjectAccessPermissionChecker) String() string {
	return "ObjectAccessPermissionChecker"
}

func (pc *ObjectAccessPermissionChecker) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if results == nil || len(results) < 1 {
		return results, nil
	} else {
		return results, nil
	}

	returnMap := make([]map[string]interface{}, 0)

	userIdString := req.PlainRequest.Context().Value("user_id")
	userGroupId := req.PlainRequest.Context().Value("usergroup_id")

	currentUserId := ""
	if userIdString != nil {
		currentUserId = userIdString.(string)

	}

	currentUserGroupId := []auth.GroupPermission{}
	if userGroupId != nil {
		currentUserGroupId = userGroupId.([]auth.GroupPermission)
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
		if permission.CanRead(currentUserId, currentUserGroupId) {
			returnMap = append(returnMap, result)
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
	userIdString := req.PlainRequest.Context().Value("user_id")
	userGroupId := req.PlainRequest.Context().Value("usergroup_id")

	currentUserId := ""
	if userIdString != nil {
		currentUserId = userIdString.(string)

	}

	currentUserGroupId := []auth.GroupPermission{}
	if userGroupId != nil {
		currentUserGroupId = userGroupId.([]auth.GroupPermission)
	}

	returnMap := make([]map[string]interface{}, 0)

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)

		refIdInterface := result["reference_id"]
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

		permission := dr.GetRowPermission(result)
		//log.Infof("[ObjectAccessPermissionChecker] Permission check for type: [%v] on [%v] @%v", req.PlainRequest.Method, dr.model.GetName(), permission.Permission)
		//log.Infof("Row Permission for [%v] for [%v]", permission, result)

		if req.PlainRequest.Method == "GET" {
			if permission.CanRead(currentUserId, currentUserGroupId) {
				returnMap = append(returnMap, result)
				includedMapCache[referenceId] = true
			} else {
				//log.Infof("[ObjectAccessPermissionChecker] Result not to be included: %v", refIdInterface)
				notIncludedMapCache[referenceId] = true

			}
		} else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" || req.PlainRequest.Method == "POST" || req.PlainRequest.Method == "DELETE" {
			if permission.CanWrite(currentUserId, currentUserGroupId) {
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
