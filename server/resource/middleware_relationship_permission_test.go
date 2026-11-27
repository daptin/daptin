package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/table_info"
)

func TestObjectMutationPermissionsIgnoreQuery(t *testing.T) {
	dm, cleanup := testOlric(t, "relationship-query-permissions")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	user, rowID, adminGroup := makeRefId(), makeRefId(), makeRefId()
	oldUserCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = &DbResource{AdministratorGroupId: adminGroup}
	t.Cleanup(func() {
		if oldUserCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
		} else {
			CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserCrud
		}
	})
	if err := dm.Put(context.Background(), "admin."+string(user[:]), false); err != nil {
		t.Fatal(err)
	}
	crud := &DbResource{tableInfo: &table_info.TableInfo{TableName: "permission_probe"}, AdministratorGroupId: adminGroup}
	checker := &ObjectAccessPermissionChecker{}
	rows := []map[string]interface{}{{"__type": "permission_probe", "reference_id": rowID}}
	for _, grants := range []struct {
		name string
		bits auth.AuthPermission
	}{
		{"peek_only", auth.UserPeek},
		{"can_update", auth.UserPeek | auth.UserUpdate},
		{"can_delete", auth.UserPeek | auth.UserDelete},
		{"revoked", auth.None},
	} {
		if err := dm.Put(context.Background(), fmt.Sprintf("object-permission-permission_probe-%s", rowID), permission.PermissionInstance{UserId: user, Permission: grants.bits}); err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{http.MethodPatch, http.MethodPut, http.MethodDelete} {
			for _, target := range []string{
				"/api/permission_probe/" + rowID.String(),
				"/api/permission_probe/" + rowID.String() + "?x=/relationships/",
				"/api/permission_probe/" + rowID.String() + "?/relationships/",
				"/api/permission_probe/" + rowID.String() + "/relationships/usergroup_id",
				"nil_url",
			} {
				t.Run(grants.name+"/"+method+"/"+target, func(t *testing.T) {
					var requestURL *url.URL
					if target != "nil_url" {
						var err error
						requestURL, err = url.Parse(target)
						if err != nil {
							t.Fatal(err)
						}
					}
					req := (&http.Request{Method: method, URL: requestURL}).WithContext(context.WithValue(context.Background(), "user", &auth.SessionUser{UserReferenceId: user}))
					allowed := grants.bits&auth.UserUpdate != 0
					if method == http.MethodDelete {
						allowed = grants.bits&auth.UserDelete != 0
					}
					if requestURL != nil && requestURL.Path == "/api/permission_probe/"+rowID.String()+"/relationships/usergroup_id" {
						allowed = grants.bits&auth.UserPeek != 0
					}
					result, err := checker.InterceptBefore(crud, &api2go.Request{PlainRequest: req}, rows, nil)
					if allowed && (err != nil || len(result) != 1) {
						t.Fatalf("authorized operation denied: result=%v error=%v", result, err)
					}
					if !allowed && (err == nil || len(result) != 0) {
						t.Fatalf("unauthorized operation admitted: result=%v error=%v", result, err)
					}
				})
			}
		}
	}
}
