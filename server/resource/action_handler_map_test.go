package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/jmoiron/sqlx"
)

type testActionPerformer struct {
	name string
}

func (p testActionPerformer) Name() string {
	return p.name
}

func (p testActionPerformer) DoAction(actionresponse.Outcome, map[string]interface{}, *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	return nil, nil, nil
}

func TestRegisterActionHandlerOnAllUpdatesGlobalAndCrudMaps(t *testing.T) {
	cruds := map[string]*DbResource{
		"world":       {ActionHandlerMap: make(map[string]actionresponse.ActionPerformerInterface)},
		"integration": {},
	}

	performer := testActionPerformer{name: "slack.com"}
	RegisterActionHandlerOnAll(cruds, performer.Name(), performer)
	defer DeleteActionHandlerOnAll(cruds, performer.Name())

	if got, ok := GetGlobalActionHandler(performer.Name()); !ok || got.Name() != performer.Name() {
		t.Fatalf("global handler was not registered")
	}

	for name, crud := range cruds {
		got, ok := GetActionHandler(crud, performer.Name())
		if !ok || got.Name() != performer.Name() {
			t.Fatalf("handler was not registered on crud %s", name)
		}
	}
}

func TestDeleteActionHandlerOnAllRemovesGlobalAndCrudMaps(t *testing.T) {
	cruds := map[string]*DbResource{
		"world":       {ActionHandlerMap: make(map[string]actionresponse.ActionPerformerInterface)},
		"integration": {ActionHandlerMap: make(map[string]actionresponse.ActionPerformerInterface)},
	}

	performer := testActionPerformer{name: "disabled.com"}
	RegisterActionHandlerOnAll(cruds, performer.Name(), performer)
	DeleteActionHandlerOnAll(cruds, performer.Name())

	if _, ok := GetGlobalActionHandler(performer.Name()); ok {
		t.Fatalf("global handler was not deleted")
	}

	for name, crud := range cruds {
		if _, ok := GetActionHandler(crud, performer.Name()); ok {
			t.Fatalf("handler was not deleted from crud %s", name)
		}
	}
}
