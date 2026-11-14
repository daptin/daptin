package actions

import (
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
)

func TestRestartSystemPerformerRequestsLifecycleRestart(t *testing.T) {
	requests := 0
	performer, err := NewRestartSystemPerformer(func() { requests++ })
	if err != nil {
		t.Fatal(err)
	}
	if performer.Name() != "__restart" {
		t.Fatalf("performer name=%q", performer.Name())
	}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, nil, nil)
	if len(errs) != 0 || requests != 1 || len(responses) != 1 || responses[0].ResponseType != "Restart" {
		t.Fatalf("requests=%d responses=%#v errors=%v", requests, responses, errs)
	}
}

func TestRestartSystemPerformerRequiresLifecycleCallback(t *testing.T) {
	if _, err := NewRestartSystemPerformer(nil); err == nil {
		t.Fatal("missing lifecycle callback was accepted")
	}
}
