package subsite

import (
	"encoding/json"
	"github.com/daptin/daptin/server/actionresponse"
)

func GetActionConfig(actionRequestInt interface{}) (actionresponse.ActionRequest, error) {
	var actionRequest actionresponse.ActionRequest
	if actionRequestInt == nil {
		actionRequestInt = "{}"
	}
	actionReqStr := actionRequestInt.(string)
	if len(actionReqStr) == 0 {
		actionReqStr = "{}"
	}
	err := json.Unmarshal([]byte(actionReqStr), &actionRequest)

	if err != nil {
		return actionresponse.ActionRequest{}, err
	}
	return actionRequest, nil
}
