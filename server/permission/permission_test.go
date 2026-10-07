package permission

import (
	"fmt"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

func TestPermissionValues(t *testing.T) {
	fmt.Printf("Permissoin [%v] == %d\n", "None", auth.None)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestPeek", auth.GuestPeek)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestRead", auth.GuestRead)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestCreate", auth.GuestCreate)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestUpdate", auth.GuestUpdate)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestDelete", auth.GuestDelete)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestExecute", auth.GuestExecute)
	fmt.Printf("Permissoin [%v] == %d\n", "GuestRefer", auth.GuestRefer)
	fmt.Printf("Permissoin [%v] == %d\n", "UserPeek", auth.UserPeek)
	fmt.Printf("Permissoin [%v] == %d\n", "UserRead", auth.UserRead)
	fmt.Printf("Permissoin [%v] == %d\n", "UserCreate", auth.UserCreate)
	fmt.Printf("Permissoin [%v] == %d\n", "UserUpdate", auth.UserUpdate)
	fmt.Printf("Permissoin [%v] == %d\n", "UserDelete", auth.UserDelete)
	fmt.Printf("Permissoin [%v] == %d\n", "UserExecute", auth.UserExecute)
	fmt.Printf("Permissoin [%v] == %d\n", "UserRefer", auth.UserRefer)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupPeek", auth.GroupPeek)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupRead", auth.GroupRead)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupCreate", auth.GroupCreate)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupUpdate", auth.GroupUpdate)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupDelete", auth.GroupDelete)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupExecute", auth.GroupExecute)
	fmt.Printf("Permissoin [%v] == %d\n", "GroupRefer", auth.GroupRefer)

}

func TestPermission(t *testing.T) {

	pi := PermissionInstance{
		UserId: daptinid.DaptinReferenceId(uuid.New()),
		UserGroupId: auth.GroupPermissionList{
			{
				GroupReferenceId:    daptinid.DaptinReferenceId(uuid.New()),
				ObjectReferenceId:   daptinid.NullReferenceId,
				RelationReferenceId: daptinid.NullReferenceId,
				Permission:          auth.UserRead | auth.GroupCRUD | auth.GroupExecute,
			},
		},
		Permission: auth.GroupCreate,
	}

	pi.CanCreate(daptinid.DaptinReferenceId(uuid.New()), auth.GroupPermissionList{
		{
			GroupReferenceId:    daptinid.DaptinReferenceId(uuid.New()),
			ObjectReferenceId:   daptinid.NullReferenceId,
			RelationReferenceId: daptinid.NullReferenceId,
			Permission:          auth.GuestRead | auth.GroupCRUD | auth.GroupExecute,
		},
	}, daptinid.NullReferenceId)

}

func TestAuthenticatedExecuteExcludesGuestsAndAllowsAuthenticatedUsers(t *testing.T) {
	permission := PermissionInstance{Permission: auth.AuthenticatedExecute}
	if permission.CanExecute(daptinid.NullReferenceId, nil, daptinid.NullReferenceId) {
		t.Fatal("guest must not satisfy AuthenticatedExecute")
	}
	userID := daptinid.DaptinReferenceId(uuid.New())
	if !permission.CanExecute(userID, nil, daptinid.NullReferenceId) {
		t.Fatal("authenticated user should satisfy AuthenticatedExecute")
	}
}

func TestCalendarSharingUsesExistingPermissionOperations(t *testing.T) {
	owner := daptinid.DaptinReferenceId(uuid.New())
	delegate := daptinid.DaptinReferenceId(uuid.New())
	readerGroup := daptinid.DaptinReferenceId(uuid.New())
	editorGroup := daptinid.DaptinReferenceId(uuid.New())
	managerGroup := daptinid.DaptinReferenceId(uuid.New())
	freebusyGroup := daptinid.DaptinReferenceId(uuid.New())
	adminGroup := daptinid.DaptinReferenceId(uuid.New())
	unrelatedGroup := daptinid.DaptinReferenceId(uuid.New())

	grant := func(group daptinid.DaptinReferenceId, rights auth.AuthPermission) auth.GroupPermission {
		return auth.GroupPermission{GroupReferenceId: group, RelationReferenceId: daptinid.DaptinReferenceId(uuid.New()), Permission: rights}
	}
	membership := func(group daptinid.DaptinReferenceId) auth.GroupPermissionList {
		return auth.GroupPermissionList{grant(group, 0)}
	}

	collection := PermissionInstance{
		UserId:     owner,
		Permission: auth.UserCRUD | auth.UserExecute,
		UserGroupId: auth.GroupPermissionList{
			grant(readerGroup, auth.GroupPeek|auth.GroupRead),
			grant(editorGroup, auth.GroupPeek|auth.GroupRead|auth.GroupCreate|auth.GroupUpdate|auth.GroupDelete|auth.GroupRefer),
			grant(managerGroup, auth.GroupPeek|auth.GroupRead|auth.GroupExecute),
			grant(freebusyGroup, auth.GroupPeek),
		},
	}
	event := PermissionInstance{
		UserId:     owner,
		Permission: auth.UserCRUD,
		UserGroupId: auth.GroupPermissionList{
			grant(readerGroup, auth.GroupPeek|auth.GroupRead),
			grant(editorGroup, auth.GroupPeek|auth.GroupRead|auth.GroupUpdate|auth.GroupDelete|auth.GroupRefer),
			grant(managerGroup, auth.GroupPeek|auth.GroupRead),
			grant(freebusyGroup, auth.GroupPeek),
		},
	}
	checks := map[string]func(daptinid.DaptinReferenceId, auth.GroupPermissionList) bool{
		"collection.peek": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return collection.CanPeek(user, groups, adminGroup)
		},
		"collection.read": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return collection.CanRead(user, groups, adminGroup)
		},
		"collection.create": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return collection.CanCreate(user, groups, adminGroup)
		},
		"collection.update": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return collection.CanUpdate(user, groups, adminGroup)
		},
		"collection.refer": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return collection.CanRefer(user, groups, adminGroup)
		},
		"event.peek": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return event.CanPeek(user, groups, adminGroup)
		},
		"event.read": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return event.CanRead(user, groups, adminGroup)
		},
		"event.update": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return event.CanUpdate(user, groups, adminGroup)
		},
		"event.delete": func(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList) bool {
			return event.CanDelete(user, groups, adminGroup)
		},
	}

	for _, tc := range []struct {
		name   string
		user   daptinid.DaptinReferenceId
		groups auth.GroupPermissionList
		allow  map[string]bool
	}{
		{name: "owner", user: owner, allow: map[string]bool{
			"collection.peek": true, "collection.read": true, "collection.create": true, "collection.update": true, "collection.refer": true,
			"event.peek": true, "event.read": true, "event.update": true, "event.delete": true,
		}},
		{name: "reader", user: delegate, groups: membership(readerGroup), allow: map[string]bool{
			"collection.peek": true, "collection.read": true, "event.peek": true, "event.read": true,
		}},
		{name: "editor", user: delegate, groups: membership(editorGroup), allow: map[string]bool{
			"collection.peek": true, "collection.read": true, "collection.create": true, "collection.update": true, "collection.refer": true,
			"event.peek": true, "event.read": true, "event.update": true, "event.delete": true,
		}},
		{name: "manager", user: delegate, groups: membership(managerGroup), allow: map[string]bool{
			"collection.peek": true, "collection.read": true, "event.peek": true, "event.read": true,
		}},
		{name: "freebusy only", user: delegate, groups: membership(freebusyGroup), allow: map[string]bool{
			"collection.peek": true, "event.peek": true,
		}},
		{name: "unrelated", user: delegate, groups: membership(unrelatedGroup)},
		{name: "revoked", user: delegate},
		{name: "administrator", user: delegate, groups: membership(adminGroup), allow: map[string]bool{
			"collection.peek": true, "collection.read": true, "collection.create": true, "collection.update": true, "collection.refer": true,
			"event.peek": true, "event.read": true, "event.update": true, "event.delete": true,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, check := range checks {
				if got, want := check(tc.user, tc.groups), tc.allow[name]; got != want {
					t.Errorf("%s = %t, want %t", name, got, want)
				}
			}
		})
	}
}
