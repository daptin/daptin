package resource

import (
	"github.com/daptin/daptin/server/auth"
)

type PermissionInstance struct {
	UserId      string
	UserGroupId []auth.GroupPermission
	Permission  auth.ObjectPermission
}

func (p PermissionInstance) CanExecute(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.ExecuteStrict)
}

func (p PermissionInstance) CanCreate(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.CreateStrict)
}

func (p PermissionInstance) CanUpdate(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.UpdateStrict)
}

func (p PermissionInstance) CanDelete(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.DeleteStrict)
}

func (p PermissionInstance) CanRefer(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.ReferStrict)
}

func (p PermissionInstance) CanRead(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.ReadStrict)
}

func (p PermissionInstance) CanPeek(userId string, usergroupId []auth.GroupPermission) bool {
	return p.CheckBit(userId, usergroupId, auth.Peek)
}

func (p1 PermissionInstance) CheckBit(userId string, usergroupId []auth.GroupPermission, bit auth.AuthPermission) bool {
	if userId == p1.UserId && len(p1.UserId) > 0 {
		return p1.Permission.OwnerCan(bit)
	}

	for _, uid := range usergroupId {

		for _, gid := range p1.UserGroupId {
			if uid.GroupReferenceId == gid.GroupReferenceId && len(gid.GroupReferenceId) > 0 {
				if gid.Permission.GroupCan(bit) {
					return true
				}
			}
		}
	}
	return p1.Permission.GuestCan(bit)
}
