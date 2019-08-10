package resource

import (
	"github.com/daptin/daptin/server/auth"
	"testing"
)

func TestPermission(t *testing.T) {

	pi := PermissionInstance{
		UserId: "user1",
		UserGroupId: []auth.GroupPermission{
			{
				GroupReferenceId:    "group1",
				ObjectReferenceId:   "",
				RelationReferenceId: "",
				Permission:          auth.UserRead | auth.GroupCRUD | auth.GroupExecute,
			},
		},
		Permission: auth.GroupCreate,
	}

	pi.CanCreate("user2", []auth.GroupPermission{
		{
			GroupReferenceId:    "group1",
			ObjectReferenceId:   "",
			RelationReferenceId: "",
			Permission:          auth.GuestRead | auth.GroupCRUD | auth.GroupExecute,
		},
	})

}
