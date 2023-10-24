package resource

import (
	"io"
	"os/exec"

	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
)

/*
*

	Become administrator of daptin action implementation
*/
type commandExecuteActionPerformer struct {
	cruds map[string]*DbResource
}

// Name of the action
func (d *commandExecuteActionPerformer) Name() string {
	return "command.execute"
}

// Executes a command using os.Exec
// returns the response (possible structured) or error
func (d *commandExecuteActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

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
		return nil, []ActionResponse{
			NewActionResponse("client.notify", NewClientNotification("error", err.Error(), "Failed")),
			NewActionResponse("output", output),
			NewActionResponse("error", errOutput),
			NewActionResponse("errorMessage", err.Error()),
		}, nil
	} else {
		return nil, []ActionResponse{
			NewActionResponse("client.notify", NewClientNotification("success", "command executed", "Command Executed")),
			NewActionResponse("output", output),
			NewActionResponse("error", errOutput),
		}, nil
	}
}

// Create a new action performer for command execute using exec.Command
func NewCommandExecuteActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := commandExecuteActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
