package actions

import (
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"testing"
)

func TestMailServiceStatusRejectsMissingIdentity(t *testing.T) {
	performer := &mailServiceStatusActionPerformer{}
	// A nil snapshot/resource would panic if denial touched runtime metadata.
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{"sessionUser": map[string]interface{}{"Groups": []string{"administrators"}}}, nil)
	if len(errs) == 0 || len(responses) != 0 {
		t.Fatalf("missing identity admitted: %v %v", responses, errs)
	}
}

func TestMailServiceStatusRejectsUnavailableMembershipContext(t *testing.T) {
	adminID := daptinid.DaptinReferenceId{1}
	performer := &mailServiceStatusActionPerformer{world: &resource.DbResource{AdministratorGroupId: adminID}}
	active := &auth.SessionUser{UserReferenceId: daptinid.DaptinReferenceId{2}, Groups: auth.GroupPermissionList{{GroupReferenceId: adminID}}}
	// Without a transaction persisted membership cannot be resolved. Even
	// apparently privileged action context must not bypass that failure.
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{"sessionUser": active}, nil)
	if len(errs) == 0 || len(responses) != 0 {
		t.Fatalf("unavailable membership admitted: %v %v", responses, errs)
	}
}
