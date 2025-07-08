package permission

import (
	"encoding/binary"
	"fmt"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
)

type PermissionInstance struct {
	UserId      daptinid.DaptinReferenceId
	UserGroupId auth.GroupPermissionList
	Permission  auth.AuthPermission
}

// MarshalBinary implements encoding.BinaryMarshaler interface
func (p PermissionInstance) MarshalBinary() (data []byte, err error) {
	userIdBytes := p.UserId
	permissionsBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(permissionsBytes, uint64(p.Permission))

	userGroupIdBytes := make([]byte, len(p.UserGroupId)*auth.AuthGroupBinaryRepresentationSize)
	for i, groupPermission := range p.UserGroupId {
		groupPermissionBytes, err := groupPermission.MarshalBinary()
		if err != nil {
			return nil, err
		}
		copy(userGroupIdBytes[i*auth.AuthGroupBinaryRepresentationSize:], groupPermissionBytes)
	}

	result := append(userIdBytes[:], permissionsBytes...)
	result = append(result, userGroupIdBytes...)
	return result, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface
func (p *PermissionInstance) UnmarshalBinary(data []byte) error {
	p.UserId = daptinid.DaptinReferenceId(data[:16])
	p.Permission = auth.AuthPermission(binary.LittleEndian.Uint64(data[16:24]))

	userGroupIdBytes := data[24:]
	if len(userGroupIdBytes)%auth.AuthGroupBinaryRepresentationSize != 0 {
		return fmt.Errorf("invalid user group data length")
	}

	userGroupCount := len(userGroupIdBytes) / auth.AuthGroupBinaryRepresentationSize
	userGroupId := make(auth.GroupPermissionList, userGroupCount)
	for i := 0; i < userGroupCount; i++ {
		start := i * auth.AuthGroupBinaryRepresentationSize
		end := (i + 1) * auth.AuthGroupBinaryRepresentationSize
		groupPermission := auth.GroupPermission{}
		err := groupPermission.UnmarshalBinary(userGroupIdBytes[start:end])
		if err != nil {
			return err
		}

		userGroupId[i] = groupPermission
	}

	p.UserGroupId = userGroupId
	return nil
}

func (p PermissionInstance) CanExecute(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {

	if p.UserId == userId && (p.Permission&auth.UserExecute == auth.UserExecute) {
		return true
	}

	if p.Permission&auth.GuestExecute == auth.GuestExecute {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupExecute == auth.GroupExecute {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanCreate(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {
	if p.UserId == userId && (p.Permission&auth.UserCreate == auth.UserCreate) {
		return true
	}

	if p.Permission&auth.GuestCreate == auth.GuestCreate {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}

		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupCreate == auth.GroupCreate {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanUpdate(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {
	if p.UserId == userId && (p.Permission&auth.UserUpdate == auth.UserUpdate) {
		return true
	}

	if p.Permission&auth.GuestUpdate == auth.GuestUpdate {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}

		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupUpdate == auth.GroupUpdate {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanDelete(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {

	if p.UserId == userId && (p.Permission&auth.UserDelete == auth.UserDelete) {
		return true
	}

	if p.Permission&auth.GuestDelete == auth.GuestDelete {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}

		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupDelete == auth.GroupDelete {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanRefer(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {

	if p.UserId == userId && (p.Permission&auth.UserRefer == auth.UserRefer) {
		return true
	}

	if p.Permission&auth.GuestRefer == auth.GuestRefer {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}
		for _, oGroup := range p.UserGroupId {
			if uGroup.GroupReferenceId == oGroup.GroupReferenceId && oGroup.Permission&auth.GroupRefer == auth.GroupRefer {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanRead(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {
	if p.UserId == userId && (p.Permission&auth.UserRead == auth.UserRead) {
		return true
	}

	if p.Permission&auth.GuestRead == auth.GuestRead {
		return true
	}

	for _, uGroup := range usergroupId {
		// user belongs to administrator group
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}
		for _, oGroup := range p.UserGroupId {
			if (uGroup.GroupReferenceId == oGroup.GroupReferenceId || uGroup.RelationReferenceId == oGroup.GroupReferenceId) && oGroup.Permission&auth.GroupRead == auth.GroupRead {
				return true
			}
		}
	}

	return false
}

func (p PermissionInstance) CanPeek(userId daptinid.DaptinReferenceId, usergroupId auth.GroupPermissionList,
	adminGroupId daptinid.DaptinReferenceId) bool {

	if p.UserId == userId && (p.Permission&auth.UserPeek == auth.UserPeek) {
		return true
	}

	if p.Permission&auth.GuestPeek == auth.GuestPeek {
		return true
	}

	for _, uGroup := range usergroupId {
		if uGroup.GroupReferenceId == adminGroupId {
			return true
		}

		for _, oGroup := range p.UserGroupId {
			if (uGroup.GroupReferenceId == oGroup.GroupReferenceId) && oGroup.Permission&auth.GroupPeek == auth.GroupPeek {
				return true
			}
		}
	}

	return false
}
