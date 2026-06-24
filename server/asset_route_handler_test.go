package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/cache"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestCachedAssetAllowedUsesPermissionSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	otherRef := daptinid.DaptinReferenceId(uuid.New())
	groupRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())

	cachedFile := &cache.CachedFile{
		AuthzVersion: cachedAssetAuthzVersion,
		TablePermission: permission.PermissionInstance{
			Permission: auth.UserPeek,
			UserId:     ownerRef,
		},
		RowPermission: permission.PermissionInstance{
			Permission: auth.UserRead,
			UserId:     ownerRef,
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupRead,
			}},
		},
		AdminGroupId: adminGroupRef,
	}

	tests := []struct {
		name    string
		user    *auth.SessionUser
		allowed bool
	}{
		{
			name:    "guest denied",
			user:    &auth.SessionUser{},
			allowed: false,
		},
		{
			name:    "owner allowed",
			user:    &auth.SessionUser{UserReferenceId: ownerRef},
			allowed: true,
		},
		{
			name:    "other user denied",
			user:    &auth.SessionUser{UserReferenceId: otherRef},
			allowed: false,
		},
		{
			name: "group member denied without table peek",
			user: &auth.SessionUser{
				UserReferenceId: otherRef,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: groupRef,
				}},
			},
			allowed: false,
		},
		{
			name: "admin group allowed",
			user: &auth.SessionUser{
				UserReferenceId: otherRef,
				Groups: auth.GroupPermissionList{{
					GroupReferenceId: adminGroupRef,
				}},
			},
			allowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
			request = request.WithContext(context.WithValue(request.Context(), "user", tt.user))
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = request

			if got := cachedAssetAllowed(cachedFile, ctx); got != tt.allowed {
				t.Fatalf("cachedAssetAllowed = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestCachedAssetAllowedForGroupSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupRef := daptinid.DaptinReferenceId(uuid.New())
	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())

	cachedFile := &cache.CachedFile{
		AuthzVersion: cachedAssetAuthzVersion,
		TablePermission: permission.PermissionInstance{
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupPeek,
			}},
		},
		RowPermission: permission.PermissionInstance{
			UserGroupId: auth.GroupPermissionList{{
				GroupReferenceId: groupRef,
				Permission:       auth.GroupRead,
			}},
		},
		AdminGroupId: adminGroupRef,
	}

	request := httptest.NewRequest(http.MethodGet, "/asset/asset/ref/file", nil)
	request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
		Groups: auth.GroupPermissionList{{
			GroupReferenceId: groupRef,
		}},
	}))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	if !cachedAssetAllowed(cachedFile, ctx) {
		t.Fatal("expected group member with table peek and row read to be allowed")
	}
}

func TestCachedAssetRequiresAuthzSnapshot(t *testing.T) {
	if cachedAssetHasAuthz(&cache.CachedFile{}) {
		t.Fatal("expected cached file without authz snapshot to be treated as unsafe")
	}
	if !cachedAssetHasAuthz(&cache.CachedFile{AuthzVersion: cachedAssetAuthzVersion}) {
		t.Fatal("expected current authz snapshot version to be accepted")
	}
}
