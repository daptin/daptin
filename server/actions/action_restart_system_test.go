package actions

import (
	"errors"
	"github.com/jmoiron/sqlx"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
)

func TestRestartSystemPerformerRequestsLifecycleRestart(t *testing.T) {
	requests := 0
	performer, err := NewRestartSystemPerformer(func(*sqlx.Tx) error { requests++; return nil })
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

func TestRestartSystemUsesOwningTransactionAndReturnsPersistenceFailure(t *testing.T) {
	tx := &sqlx.Tx{}
	failure := errors.New("restart request not persisted")
	performer, err := NewRestartSystemPerformer(func(active *sqlx.Tx) error {
		if active != tx {
			t.Fatal("transaction replaced")
		}
		return failure
	})
	if err != nil {
		t.Fatal(err)
	}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, nil, tx)
	if len(responses) != 0 || len(errs) != 1 || errs[0] != failure {
		t.Fatalf("unpersisted restart acknowledged: %v %v", responses, errs)
	}
}
