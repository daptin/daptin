package resource

import (
	log "github.com/sirupsen/logrus"
	//"syscall"
	"time"
	//"os/exec"
	//"fmt"
	"github.com/artpar/api2go"
	"github.com/sadlil/go-trigger"
)

type RestartSystemActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *RestartSystemActionPerformer) Name() string {
	return "__restart"
}

func (d *RestartSystemActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Initiating system update."
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	// new response
	restartAttrs = make(map[string]interface{})
	restartAttrs["location"] = "/"
	restartAttrs["window"] = "self"
	restartAttrs["delay"] = 5000
	actionResponse = NewActionResponse("client.redirect", restartAttrs)
	responses = append(responses, actionResponse)

	go restart()

	return nil, responses, nil
}

func NewRestarSystemPerformer(initConfig *CmsConfig) (ActionPerformerInterface, error) {

	handler := RestartSystemActionPerformer{}

	return &handler, nil

}

func restart() {
	log.Infof("Sleeping for 3 seconds before restart")
	time.Sleep(10 * time.Millisecond)
	log.Infof("Kill")
	//log.Infof("Sending %v to %v", syscall.SIGUSR2, syscall.Getpid())

	//exec.Command("kill", "-12", fmt.Sprint(syscall.Getpid())).Output()
	_, err := trigger.Fire("restart")
	CheckErr(err, "Failed to trigger restart")
	//syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)

}
