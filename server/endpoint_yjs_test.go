package server

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/google/uuid"
)

func yjsTestReferenceId() daptinid.DaptinReferenceId {
	return daptinid.DaptinReferenceId(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
}

func TestCanonicalYjsRoomParts(t *testing.T) {
	referenceId := "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name      string
		room      string
		canonical bool
	}{
		{name: "database room", room: "document." + referenceId + ".content", canonical: true},
		{name: "standalone name", room: "team-notes", canonical: false},
		{name: "invalid uuid remains standalone", room: "team.shared.notes", canonical: false},
		{name: "extra segment remains standalone", room: "document." + referenceId + ".content.extra", canonical: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parts, canonical := canonicalYjsRoomParts(test.room)
			if canonical != test.canonical {
				t.Fatalf("canonical=%v, want %v", canonical, test.canonical)
			}
			if canonical && (len(parts) != 3 || parts[0] != "document" || parts[2] != "content") {
				t.Fatalf("unexpected canonical parts: %v", parts)
			}
		})
	}
}

func TestYjsPermissionAccess(t *testing.T) {
	userReferenceId := yjsTestReferenceId()
	groupReferenceId := daptinid.DaptinReferenceId(uuid.MustParse("22222222-2222-2222-2222-222222222222"))
	otherReferenceId := daptinid.DaptinReferenceId(uuid.MustParse("33333333-3333-3333-3333-333333333333"))
	user := &auth.SessionUser{UserReferenceId: userReferenceId}

	tests := []struct {
		name       string
		permission permission.PermissionInstance
		groups     auth.GroupPermissionList
		allowed    bool
		readOnly   bool
	}{
		{name: "owner update", permission: permission.PermissionInstance{UserId: userReferenceId, Permission: auth.UserUpdate}, allowed: true},
		{name: "owner read", permission: permission.PermissionInstance{UserId: userReferenceId, Permission: auth.UserRead}, allowed: true, readOnly: true},
		{name: "group update", permission: permission.PermissionInstance{UserGroupId: auth.GroupPermissionList{{GroupReferenceId: groupReferenceId, Permission: auth.GroupUpdate}}}, groups: auth.GroupPermissionList{{GroupReferenceId: groupReferenceId}}, allowed: true},
		{name: "group read", permission: permission.PermissionInstance{UserGroupId: auth.GroupPermissionList{{GroupReferenceId: groupReferenceId, Permission: auth.GroupRead}}}, groups: auth.GroupPermissionList{{GroupReferenceId: groupReferenceId}}, allowed: true, readOnly: true},
		{name: "unrelated user", permission: permission.PermissionInstance{UserId: otherReferenceId, Permission: auth.UserCRUD}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user.Groups = test.groups
			allowed, readOnly := yjsPermissionAccess(test.permission, user, daptinid.NullReferenceId)
			if allowed != test.allowed || readOnly != test.readOnly {
				t.Fatalf("allowed=%v readOnly=%v, want allowed=%v readOnly=%v", allowed, readOnly, test.allowed, test.readOnly)
			}
		})
	}
}
