package actions

import (
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"io"
	"os/exec"

	"github.com/artpar/api2go/v2"
	"github.com/jmoiron/sqlx"
)

/*
*

	Become administrator of daptin action implementation
*/
type commandExecuteActionPerformer struct {
	cruds map[string]*resource.DbResource
}

// Name of the action
func (d *commandExecuteActionPerformer) Name() string {
	return "command.execute"
}

// Executes a command using os.Exec
// returns the response (possible structured) or error
func (d *commandExecuteActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{},
	transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	var err error

	command := inFieldMap["command"].(string)
	args := inFieldMap["arguments"].([]string)

	execution := exec.Command(command, args...)

	outBuffer, err := execution.StdoutPipe()
	errorBuffer, err := execution.StderrPipe()

	err = execution.Run()

	errOutput, err := io.ReadAll(errorBuffer)
	output, err := io.ReadAll(outBuffer)

	if err != nil {
		return nil, []actionresponse.ActionResponse{
			resource.NewActionResponse("client.notify",
				resource.NewClientNotification("error", err.Error(), "Failed")),
			resource.NewActionResponse("output", output),
			resource.NewActionResponse("error", errOutput),
			resource.NewActionResponse("errorMessage", err.Error()),
		}, nil
	} else {
		return nil, []actionresponse.ActionResponse{
			resource.NewActionResponse("client.notify",
				resource.NewClientNotification("success", "command executed", "Command Executed")),
			resource.NewActionResponse("output", output),
			resource.NewActionResponse("error", errOutput),
		}, nil
	}
}

// Create a new action performer for command execute using exec.Command
func NewCommandExecuteActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := commandExecuteActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
