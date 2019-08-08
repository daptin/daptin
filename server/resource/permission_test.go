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
				Permission:          auth.NewPermission(auth.Read, auth.None, auth.CRUD|auth.Execute),
			},
		},
		Permission: auth.NewPermission(auth.None, auth.None, auth.Create),
	}

	pi.CanCreate("user2", []auth.GroupPermission{
		{
			GroupReferenceId:    "group1",
			ObjectReferenceId:   "",
			RelationReferenceId: "",
			Permission:          auth.NewPermission(auth.Read, auth.None, auth.CRUD|auth.Execute),
		},
	})

}
