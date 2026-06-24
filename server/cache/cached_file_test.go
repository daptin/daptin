package cache

import (
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/google/uuid"
)

func TestCachedFileMarshalRoundTripIncludesPermissionSnapshot(t *testing.T) {
	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	groupRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())

	original := &CachedFile{
		Data:         []byte("private-bytes"),
		ETag:         `"etag"`,
		Modtime:      time.Unix(100, 0),
		MimeType:     "image/svg+xml",
		Path:         "asset/ref/file.svg",
		Size:         len("private-bytes"),
		IsDownload:   false,
		ExpiresAt:    time.Unix(200, 0),
		AuthzVersion: 1,
		TablePermission: permission.PermissionInstance{
			UserId:     ownerRef,
			Permission: auth.UserPeek,
		},
		RowPermission: permission.PermissionInstance{
			UserId:     ownerRef,
			Permission: auth.UserRead,
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupRead,
			}},
		},
		AdminGroupId: adminGroupRef,
	}

	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary returned error: %v", err)
	}

	var decoded CachedFile
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary returned error: %v", err)
	}

	if decoded.AuthzVersion != original.AuthzVersion {
		t.Fatalf("AuthzVersion = %d, want %d", decoded.AuthzVersion, original.AuthzVersion)
	}
	if decoded.TablePermission.UserId != ownerRef || decoded.TablePermission.Permission != auth.UserPeek {
		t.Fatalf("table permission did not round-trip: %#v", decoded.TablePermission)
	}
	if decoded.RowPermission.UserId != ownerRef || decoded.RowPermission.Permission != auth.UserRead {
		t.Fatalf("row permission did not round-trip: %#v", decoded.RowPermission)
	}
	if len(decoded.RowPermission.UserGroupId) != 1 || decoded.RowPermission.UserGroupId[0].GroupReferenceId != groupRef {
		t.Fatalf("row permission groups did not round-trip: %#v", decoded.RowPermission.UserGroupId)
	}
	if decoded.AdminGroupId != adminGroupRef {
		t.Fatalf("AdminGroupId = %v, want %v", decoded.AdminGroupId, adminGroupRef)
	}
}
