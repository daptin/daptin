package resource

import (
	"github.com/daptin/daptin/server/auth"
)

type PermissionInstance struct {
	UserId      string
	UserGroupId []auth.GroupPermission
	Permission  auth.AuthPermission
}

func (p PermissionInstance) CanExecute(userId string, usergroupId []auth.GroupPermission) bool {

	if p.UserId == userId && (p.Permission&auth.UserExecute == auth.UserExecute) {
		return true
	}

	if p.Permission&auth.GuestExecute == auth.GuestExecute {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupExecute == auth.GroupExecute {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanCreate(userId string, usergroupId []auth.GroupPermission) bool {
	if p.UserId == userId && (p.Permission&auth.UserCreate == auth.UserCreate) {
		return true
	}

	if p.Permission&auth.GuestCreate == auth.GuestCreate {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupCreate == auth.GroupCreate {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanUpdate(userId string, usergroupId []auth.GroupPermission) bool {
	if p.UserId == userId && (p.Permission&auth.UserUpdate == auth.UserUpdate) {
		return true
	}

	if p.Permission&auth.GuestUpdate == auth.GuestUpdate {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupUpdate == auth.GroupUpdate {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanDelete(userId string, usergroupId []auth.GroupPermission) bool {
	if p.UserId == userId && (p.Permission&auth.UserDelete == auth.UserDelete) {
		return true
	}

	if p.Permission&auth.GuestDelete == auth.GuestDelete {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupDelete == auth.GroupDelete {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanRefer(userId string, usergroupId []auth.GroupPermission) bool {
	if p.UserId == userId && (p.Permission&auth.UserRefer == auth.UserRefer) {
		return true
	}

	if p.Permission&auth.GuestRefer == auth.GuestRefer {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupRefer == auth.GroupRefer {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanRead(userId string, usergroupId []auth.GroupPermission) bool {
	if p.UserId == userId && (p.Permission&auth.UserRead == auth.UserRead) {
		return true
	}

	if p.Permission&auth.GuestRead == auth.GuestRead {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupRead == auth.GroupRead {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanPeek(userId string, usergroupId []auth.GroupPermission) bool {
	if (p.UserId != "" && p.UserId == userId) && (p.Permission&auth.UserPeek == auth.UserPeek) {
		return true
	}

	if p.Permission&auth.GuestPeek == auth.GuestPeek {
		return true
	}

	for _, uGroup := range usergroupId {
		for _, oGroup := range p.UserGroupId {
			if (oGroup.GroupReferenceId != "" && uGroup.GroupReferenceId == oGroup.GroupReferenceId) && oGroup.Permission&auth.GroupPeek == auth.GroupPeek {
				return true
			}
		}
	}

	return false
}
