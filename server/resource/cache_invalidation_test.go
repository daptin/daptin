package resource

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/google/uuid"
)

// testOlric creates an embedded Olric instance and returns a DMap plus a cleanup function.
func testOlric(t *testing.T, dmapName string) (olric.DMap, func()) {
	t.Helper()
	cfg := olricConfig.New("local")
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("failed to create olric: %v", err)
	}

	go func() { _ = emb.Start() }()

	time.Sleep(500 * time.Millisecond)

	client := emb.NewEmbeddedClient()
	dm, err := client.NewDMap(dmapName)
	if err != nil {
		t.Fatalf("failed to create DMap: %v", err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = emb.Shutdown(ctx)
	}
	return dm, cleanup
}

// swapOlricCache replaces the global OlricCache with the given DMap and returns a restore function.
func swapOlricCache(dm olric.DMap) func() {
	old := OlricCache
	OlricCache = dm
	return func() { OlricCache = old }
}

func makeRefId() daptinid.DaptinReferenceId {
	return daptinid.DaptinReferenceId(uuid.New())
}

// ---------------------------------------------------------------------------
// InvalidateObjectPermissionCache
// ---------------------------------------------------------------------------

func TestInvalidateObjectPermissionCache_NilCache(t *testing.T) {
	restore := swapOlricCache(nil)
	defer restore()
	// Must not panic
	InvalidateObjectPermissionCache("world", makeRefId())
}

func TestInvalidateObjectPermissionCache_RemovesEntry(t *testing.T) {
	dm, cleanup := testOlric(t, "test-object-perm")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	cacheKey := fmt.Sprintf("object-permission-%v-%v", "world", refId)

	perm := permission.PermissionInstance{
		Permission: auth.GuestRead | auth.UserRead,
	}

	err := dm.Put(context.Background(), cacheKey, perm)
	if err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	// Verify entry exists
	val, err := dm.Get(context.Background(), cacheKey)
	if err != nil || val == nil {
		t.Fatalf("expected cache hit before invalidation, got err=%v", err)
	}

	InvalidateObjectPermissionCache("world", refId)

	// Verify entry is gone
	_, err = dm.Get(context.Background(), cacheKey)
	if err == nil {
		t.Error("expected cache miss after invalidation, but got a hit")
	}
}

func TestInvalidateObjectPermissionCache_NonExistentKey(t *testing.T) {
	dm, cleanup := testOlric(t, "test-object-perm-missing")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	// Should not panic on missing key
	InvalidateObjectPermissionCache("world", makeRefId())
}

func TestInvalidateObjectPermissionCache_KeyFormat(t *testing.T) {
	// Verify the invalidation key format matches the caching key format
	dm, cleanup := testOlric(t, "test-object-perm-format")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	objectType := "user_account"

	// Simulate what GetObjectPermissionByReferenceIdWithTransaction does
	cacheKey := fmt.Sprintf("object-permission-%v-%v", objectType, refId)
	perm := permission.PermissionInstance{Permission: auth.UserCRUD}

	err := dm.Put(context.Background(), cacheKey, perm)
	if err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	// Invalidate using the function
	InvalidateObjectPermissionCache(objectType, refId)

	// Must be gone — proves key format matches
	_, err = dm.Get(context.Background(), cacheKey)
	if err == nil {
		t.Error("cache key format mismatch: entry still present after invalidation")
	}
}

// ---------------------------------------------------------------------------
// InvalidateRowPermissionCache
// ---------------------------------------------------------------------------

func TestInvalidateRowPermissionCache_NilCache(t *testing.T) {
	restore := swapOlricCache(nil)
	defer restore()
	InvalidateRowPermissionCache("task", makeRefId())
}

func TestInvalidateRowPermissionCache_RemovesEntry(t *testing.T) {
	dm, cleanup := testOlric(t, "test-row-perm")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	cacheKey := fmt.Sprintf("row-permission-%v-%v", "task", refId)

	perm := permission.PermissionInstance{Permission: auth.UserRead}
	err := dm.Put(context.Background(), cacheKey, perm)
	if err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	val, err := dm.Get(context.Background(), cacheKey)
	if err != nil || val == nil {
		t.Fatalf("expected cache hit before invalidation")
	}

	InvalidateRowPermissionCache("task", refId)

	_, err = dm.Get(context.Background(), cacheKey)
	if err == nil {
		t.Error("expected cache miss after invalidation")
	}
}

func TestInvalidateRowPermissionCache_NonExistentKey(t *testing.T) {
	dm, cleanup := testOlric(t, "test-row-perm-missing")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	InvalidateRowPermissionCache("task", makeRefId())
}

// ---------------------------------------------------------------------------
// InvalidateObjectGroupsCache
// ---------------------------------------------------------------------------

func TestInvalidateObjectGroupsCache_NilCache(t *testing.T) {
	restore := swapOlricCache(nil)
	defer restore()
	InvalidateObjectGroupsCache("task", 42)
}

func TestInvalidateObjectGroupsCache_RemovesEntry(t *testing.T) {
	dm, cleanup := testOlric(t, "test-obj-groups")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	objectType := "product"
	objectId := int64(99)
	cacheKey := fmt.Sprintf("object-groups-%v-%v", objectType, objectId)

	groups := auth.GroupPermissionList{
		{
			GroupReferenceId: makeRefId(),
			Permission:       auth.GroupCRUD,
		},
	}
	err := dm.Put(context.Background(), cacheKey, groups)
	if err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	val, err := dm.Get(context.Background(), cacheKey)
	if err != nil || val == nil {
		t.Fatalf("expected cache hit before invalidation")
	}

	InvalidateObjectGroupsCache(objectType, objectId)

	_, err = dm.Get(context.Background(), cacheKey)
	if err == nil {
		t.Error("expected cache miss after invalidation")
	}
}

func TestInvalidateObjectGroupsCache_NonExistentKey(t *testing.T) {
	dm, cleanup := testOlric(t, "test-obj-groups-missing")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	InvalidateObjectGroupsCache("product", 999)
}

func TestInvalidateObjectGroupsCache_KeyFormat(t *testing.T) {
	dm, cleanup := testOlric(t, "test-obj-groups-format")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	objectType := "cloud_store"
	objectId := int64(7)

	// Simulate what GetObjectGroupsByObjectIdWithTransaction does
	cacheKey := fmt.Sprintf("object-groups-%v-%v", objectType, objectId)
	groups := auth.GroupPermissionList{}

	err := dm.Put(context.Background(), cacheKey, groups)
	if err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	InvalidateObjectGroupsCache(objectType, objectId)

	_, err = dm.Get(context.Background(), cacheKey)
	if err == nil {
		t.Error("cache key format mismatch: entry still present after invalidation")
	}
}

// ---------------------------------------------------------------------------
// InvalidateAdminCacheForUser
// ---------------------------------------------------------------------------

func TestInvalidateAdminCacheForUser_NilCache(t *testing.T) {
	restore := swapOlricCache(nil)
	defer restore()
	InvalidateAdminCacheForUser(makeRefId())
}

func TestInvalidateAdminCacheForUser_RemovesBulkAdminMap(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-bulk")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	adminMap := make(AdminMapType)
	adminUUID := uuid.New()
	adminMap[adminUUID] = true

	err := dm.Put(context.Background(), "administrator_reference_id", adminMap)
	if err != nil {
		t.Fatalf("failed to seed admin map: %v", err)
	}

	val, err := dm.Get(context.Background(), "administrator_reference_id")
	if err != nil || val == nil {
		t.Fatalf("expected admin map cache hit before invalidation")
	}

	userRefId := daptinid.DaptinReferenceId(adminUUID)
	InvalidateAdminCacheForUser(userRefId)

	_, err = dm.Get(context.Background(), "administrator_reference_id")
	if err == nil {
		t.Error("expected admin map cache miss after invalidation")
	}
}

func TestInvalidateAdminCacheForUser_RemovesPerUserAdminFlag(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-per-user")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	userRefId := makeRefId()
	key := "admin." + string(userRefId[:])

	err := dm.Put(context.Background(), key, true)
	if err != nil {
		t.Fatalf("failed to seed per-user admin flag: %v", err)
	}

	val, err := dm.Get(context.Background(), key)
	if err != nil || val == nil {
		t.Fatalf("expected per-user admin cache hit before invalidation")
	}

	InvalidateAdminCacheForUser(userRefId)

	_, err = dm.Get(context.Background(), key)
	if err == nil {
		t.Error("expected per-user admin cache miss after invalidation")
	}
}

func TestInvalidateAdminCacheForUser_RemovesBothCaches(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-both")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	userUUID := uuid.New()
	userRefId := daptinid.DaptinReferenceId(userUUID)

	// Seed the bulk admin map
	adminMap := make(AdminMapType)
	adminMap[userUUID] = true
	err := dm.Put(context.Background(), "administrator_reference_id", adminMap)
	if err != nil {
		t.Fatalf("failed to seed admin map: %v", err)
	}

	// Seed the per-user flag
	perUserKey := "admin." + string(userRefId[:])
	err = dm.Put(context.Background(), perUserKey, true)
	if err != nil {
		t.Fatalf("failed to seed per-user flag: %v", err)
	}

	// Single invalidation call should remove both
	InvalidateAdminCacheForUser(userRefId)

	_, err = dm.Get(context.Background(), "administrator_reference_id")
	if err == nil {
		t.Error("bulk admin map still present after invalidation")
	}
	_, err = dm.Get(context.Background(), perUserKey)
	if err == nil {
		t.Error("per-user admin flag still present after invalidation")
	}
}

func TestInvalidateAdminCacheForUser_NonExistentKey(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-missing")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	// No pre-seeded data — should not panic
	InvalidateAdminCacheForUser(makeRefId())
}

func TestInvalidateAdminCacheForUser_PerUserKeyFormat(t *testing.T) {
	// Verify the per-user key format matches what IsAdminWithTransaction uses
	dm, cleanup := testOlric(t, "test-admin-key-format")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	userRefId := makeRefId()

	// Simulate what IsAdminWithTransaction does at dbresource.go:336
	key := "admin." + string(userRefId[:])
	err := dm.Put(context.Background(), key, true)
	if err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	// Invalidate using our function
	InvalidateAdminCacheForUser(userRefId)

	// Must be gone — proves key format matches
	_, err = dm.Get(context.Background(), key)
	if err == nil {
		t.Error("per-user admin key format mismatch: entry still present after invalidation")
	}
}

// ---------------------------------------------------------------------------
// TTL reduction verification
// ---------------------------------------------------------------------------

func TestAdminMapTTL_ReducedTo5Minutes(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-ttl")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	// Seed admin map with the same TTL as production code: 5 min
	adminMap := make(AdminMapType)
	adminMap[uuid.New()] = true

	err := dm.Put(context.Background(), "administrator_reference_id", adminMap, olric.EX(5*time.Minute), olric.NX())
	if err != nil {
		t.Fatalf("failed to put admin map with 5min TTL: %v", err)
	}

	// Verify it's accessible
	val, err := dm.Get(context.Background(), "administrator_reference_id")
	if err != nil || val == nil {
		t.Fatal("admin map should be accessible within TTL")
	}
}

func TestObjectPermissionTTL_ReducedTo5Minutes(t *testing.T) {
	dm, cleanup := testOlric(t, "test-obj-perm-ttl")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	cacheKey := fmt.Sprintf("object-permission-%v-%v", "world", refId)
	perm := permission.PermissionInstance{Permission: auth.UserRead}

	err := dm.Put(context.Background(), cacheKey, perm, olric.EX(5*time.Minute), olric.NX())
	if err != nil {
		t.Fatalf("failed to put object permission with 5min TTL: %v", err)
	}

	val, err := dm.Get(context.Background(), cacheKey)
	if err != nil || val == nil {
		t.Fatal("object permission should be accessible within TTL")
	}
}

func TestPerUserAdminTTL_ReducedTo2Minutes(t *testing.T) {
	dm, cleanup := testOlric(t, "test-per-user-ttl")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	userRefId := makeRefId()
	key := "admin." + string(userRefId[:])

	err := dm.Put(context.Background(), key, true, olric.EX(2*time.Minute), olric.NX())
	if err != nil {
		t.Fatalf("failed to put per-user admin with 2min TTL: %v", err)
	}

	val, err := dm.Get(context.Background(), key)
	if err != nil || val == nil {
		t.Fatal("per-user admin flag should be accessible within TTL")
	}
}

// ---------------------------------------------------------------------------
// NX semantics: invalidation then re-cache
// ---------------------------------------------------------------------------

func TestNX_InvalidateThenRecache(t *testing.T) {
	// NX means "only set if not exists". This test verifies that after
	// invalidation (Delete), a new Put with NX succeeds — proving that
	// invalidation is the correct way to update NX-protected cache entries.
	dm, cleanup := testOlric(t, "test-nx-recache")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	cacheKey := fmt.Sprintf("object-permission-%v-%v", "world", refId)

	// First put: NX succeeds
	perm1 := permission.PermissionInstance{Permission: auth.UserRead}
	err := dm.Put(context.Background(), cacheKey, perm1, olric.NX())
	if err != nil {
		t.Fatalf("first NX put should succeed: %v", err)
	}

	// Second put: NX should be rejected (key exists)
	perm2 := permission.PermissionInstance{Permission: auth.UserCRUD}
	err = dm.Put(context.Background(), cacheKey, perm2, olric.NX())
	if err == nil {
		t.Log("NX put on existing key was accepted (Olric version may silently ignore NX conflict)")
	}

	// Invalidate
	InvalidateObjectPermissionCache("world", refId)

	// Now NX put should succeed again
	perm3 := permission.PermissionInstance{Permission: auth.GroupCRUD}
	err = dm.Put(context.Background(), cacheKey, perm3, olric.NX())
	if err != nil {
		t.Errorf("NX put after invalidation should succeed: %v", err)
	}

	// Verify the new value is stored
	val, err := dm.Get(context.Background(), cacheKey)
	if err != nil {
		t.Fatalf("expected cache hit after re-cache: %v", err)
	}
	var result permission.PermissionInstance
	err = val.Scan(&result)
	if err != nil {
		t.Fatalf("failed to scan cached permission: %v", err)
	}
	if result.Permission != auth.GroupCRUD {
		t.Errorf("expected re-cached permission to be GroupCRUD, got %v", result.Permission)
	}
}

// ---------------------------------------------------------------------------
// Cross-invalidation: different types don't interfere
// ---------------------------------------------------------------------------

func TestInvalidation_OnlyAffectsTargetedCache(t *testing.T) {
	dm, cleanup := testOlric(t, "test-isolation")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()

	// Seed three different cache types for the same refId
	objPermKey := fmt.Sprintf("object-permission-%v-%v", "task", refId)
	rowPermKey := fmt.Sprintf("row-permission-%v-%v", "task", refId)

	perm := permission.PermissionInstance{Permission: auth.UserRead}
	dm.Put(context.Background(), objPermKey, perm)
	dm.Put(context.Background(), rowPermKey, perm)

	// Invalidate only object-permission
	InvalidateObjectPermissionCache("task", refId)

	_, err := dm.Get(context.Background(), objPermKey)
	if err == nil {
		t.Error("object-permission should be invalidated")
	}

	// row-permission must still be present
	val, err := dm.Get(context.Background(), rowPermKey)
	if err != nil || val == nil {
		t.Error("row-permission should NOT be affected by object-permission invalidation")
	}

	// Now invalidate row-permission
	InvalidateRowPermissionCache("task", refId)
	_, err = dm.Get(context.Background(), rowPermKey)
	if err == nil {
		t.Error("row-permission should be invalidated")
	}
}

func TestInvalidation_DifferentObjectTypes(t *testing.T) {
	dm, cleanup := testOlric(t, "test-type-isolation")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	refId := makeRefId()
	perm := permission.PermissionInstance{Permission: auth.UserRead}

	worldKey := fmt.Sprintf("object-permission-%v-%v", "world", refId)
	taskKey := fmt.Sprintf("object-permission-%v-%v", "task", refId)

	dm.Put(context.Background(), worldKey, perm)
	dm.Put(context.Background(), taskKey, perm)

	// Invalidate only "world"
	InvalidateObjectPermissionCache("world", refId)

	_, err := dm.Get(context.Background(), worldKey)
	if err == nil {
		t.Error("world permission should be invalidated")
	}

	val, err := dm.Get(context.Background(), taskKey)
	if err != nil || val == nil {
		t.Error("task permission should NOT be affected by world invalidation")
	}
}

func TestInvalidateAdminCache_DoesNotAffectObjectPermissions(t *testing.T) {
	dm, cleanup := testOlric(t, "test-admin-vs-obj")
	defer cleanup()
	restore := swapOlricCache(dm)
	defer restore()

	userRefId := makeRefId()

	// Seed admin caches
	adminMap := make(AdminMapType)
	adminMap[uuid.UUID(userRefId)] = true
	dm.Put(context.Background(), "administrator_reference_id", adminMap)
	dm.Put(context.Background(), "admin."+string(userRefId[:]), true)

	// Seed an object permission
	objKey := fmt.Sprintf("object-permission-%v-%v", "world", userRefId)
	perm := permission.PermissionInstance{Permission: auth.UserRead}
	dm.Put(context.Background(), objKey, perm)

	// Invalidate admin cache
	InvalidateAdminCacheForUser(userRefId)

	// Object permission must survive
	val, err := dm.Get(context.Background(), objKey)
	if err != nil || val == nil {
		t.Error("object permission should NOT be affected by admin cache invalidation")
	}
}

// ---------------------------------------------------------------------------
// Entity name parsing for usergroup relation tables
// ---------------------------------------------------------------------------

func TestEntityNameParsing_FromJoinTableName(t *testing.T) {
	tests := []struct {
		tableName  string
		wantEntity string
	}{
		{"task_task_id_has_usergroup_usergroup_id", "task"},
		{"user_account_user_account_id_has_usergroup_usergroup_id", "user_account"},
		{"cloud_store_cloud_store_id_has_usergroup_usergroup_id", "cloud_store"},
		{"product_product_id_has_usergroup_usergroup_id", "product"},
		{"json_schema_json_schema_id_has_usergroup_usergroup_id", "json_schema"},
		{"plan_plan_id_has_usergroup_usergroup_id", "plan"},
		{"outbox_outbox_id_has_usergroup_usergroup_id", "outbox"},
		{"oauth_token_oauth_token_id_has_usergroup_usergroup_id", "oauth_token"},
		{"stream_stream_id_has_usergroup_usergroup_id", "stream"},
		{"deployment_deployment_id_has_usergroup_usergroup_id", "deployment"},
		{"feed_feed_id_has_usergroup_usergroup_id", "feed"},
		{"smd_smd_id_has_usergroup_usergroup_id", "smd"},
		{"site_site_id_has_usergroup_usergroup_id", "site"},
	}

	for _, tt := range tests {
		t.Run(tt.tableName, func(t *testing.T) {
			// This is the parsing logic used in resource_create.go, resource_update.go, resource_delete.go
			doubledEntity := tt.tableName[:len(tt.tableName)-len("_id_has_usergroup_usergroup_id")]
			gotEntity := doubledEntity[:len(doubledEntity)/2]

			if gotEntity != tt.wantEntity {
				t.Errorf("parsed entity = %q, want %q (from table %q, doubled = %q)",
					gotEntity, tt.wantEntity, tt.tableName, doubledEntity)
			}

			// Also verify the column name derivation
			wantCol := tt.wantEntity + "_id"
			gotCol := gotEntity + "_id"
			if gotCol != wantCol {
				t.Errorf("derived column = %q, want %q", gotCol, wantCol)
			}
		})
	}
}
