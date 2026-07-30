package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestExportActionsAreAdministratorOnly(t *testing.T) {
	for _, actionName := range []string{"export_data", "export_csv_data"} {
		t.Run(actionName, func(t *testing.T) {
			var found bool
			for _, action := range SystemActions {
				if action.Name != actionName || action.OnType != "world" {
					continue
				}
				found = true
				if action.Permission == nil || *action.Permission != auth.None {
					t.Fatalf("%s base permission = %v, want auth.None", actionName, action.Permission)
				}
				if len(action.AccessGroups) != 1 || action.AccessGroups[0].Name != "administrators" {
					t.Fatalf("%s access groups = %#v, want administrators", actionName, action.AccessGroups)
				}
				permission := action.AccessGroups[0].Permission
				if permission == nil || *permission != auth.GroupExecute {
					t.Fatalf("%s administrator permission = %v, want GroupExecute", actionName, permission)
				}
			}
			if !found {
				t.Fatalf("system action %s not found", actionName)
			}
		})
	}
}
