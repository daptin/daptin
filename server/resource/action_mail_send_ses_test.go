package resource

import (
	"testing"
	"time"
)

func TestEmailViaAes(t *testing.T) {
	a, _ := NewAwsMailSendActionPerformer(nil, nil, nil, nil)
	var outcome Outcome = Outcome{}
	inFields := make(map[string]interface{})
	inFields["to"] = []string{"founderartpar@unlogged.io"}
	inFields["subject"] = "test mail from daptin - " + time.Now().String()
	inFields["from"] = "no-reply@100x.bot"
	inFields["body"] = "hello email ses - " + time.Now().String()
	a.DoAction(outcome, inFields, nil)
}
