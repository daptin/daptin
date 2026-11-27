package resource

import (
	"github.com/daptin/daptin/server/auth"
	"testing"
)

func TestAgentExecutionResourcesAreShortAndNotCallerWritable(t *testing.T) {
	for _, name := range []string{"agent", "agentrun", "agentstp", "agenttry"} {
		if len(name) > 8 {
			t.Fatal(name)
		}
		table := tableFromConfig(t, &CmsConfig{Tables: StandardTables}, name)
		if table.DefaultPermission&(auth.UserCreate|auth.UserUpdate|auth.UserDelete|auth.GuestCRUD|auth.GroupCRUD) != 0 {
			t.Fatalf("%s grants caller writes", name)
		}
	}
}
