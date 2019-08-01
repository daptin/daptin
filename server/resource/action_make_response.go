package resource

import (
	"github.com/artpar/api2go"
)

/**
  Become administrator of daptin action implementation
*/
type MakeResponsePerformer struct {
}

// Name of the action
func (d *MakeResponsePerformer) Name() string {
	return "response.create"
}

// Perform action and try to make the current user the admin of the system
// Checks CanBecomeAdmin and then invokes BecomeAdmin if true
func (d *MakeResponsePerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {
	responseType, ok := inFieldMap["response_type"]
	if !ok {
		responseType = request.Type
	}
	return nil, []ActionResponse{
		NewActionResponse(responseType.(string), inFieldMap),
	}, nil
}

// Create a new action performer for becoming administrator action
func NewMakeResponsePerformer() (ActionPerformerInterface, error) {

	handler := MakeResponsePerformer{}

	return &handler, nil

}
