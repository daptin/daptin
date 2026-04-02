package auth

import (
	"context"
	"fmt"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"testing"
	"time"
)

func TestAllPermission(t *testing.T) {

	perm1 := GuestRead | UserRead | UserCreate | UserUpdate | GroupCRUD
	perm2 := GuestCreate | GuestRead | GuestRefer | UserRead | UserUpdate | UserExecute | GroupCreate | GroupRead | GroupRefer | GroupUpdate
	perm3 := None | UserRead | UserExecute | GroupCRUD | GroupExecute
	perm4 := GuestRead | UserRead | UserExecute | GroupCRUD | GroupExecute
	perm5 := GuestPeek | GuestExecute | UserRead | UserExecute | GroupCRUD | GroupExecute

	tperm1 := perm1
	tperm2 := perm2
	//tperm2 := ParsePermission(perm3)

	if perm1 == perm2 {
		t.Errorf("Permission should not be equal")
	}

	if perm1 != tperm1 {
		t.Errorf("Parsing failed")
	}

	if perm2 != tperm2 {
		t.Errorf("Parsing failed")
	}
	fmt.Printf("Perm 1: %v == %v == %v\n", perm1, perm1, tperm1)
	fmt.Printf("Perm 2: %v == %v == %v\n", perm2, perm2, tperm2)
	fmt.Printf("Perm 3: %v == %v\n", perm3, perm3)
	fmt.Printf("Perm 4: %v == %v\n", perm4, perm4)
	fmt.Printf("Perm 5: %v == %v\n", perm5, perm5)

}

func TestAuthPermissions(t *testing.T) {

	t.Logf("Permission None [%v] %v", None, None)

	t.Logf("Permission GuestPeek [%v] %v", GuestPeek, int64(GuestPeek))
	t.Logf("Permission GuestRead [%v] %v", GuestRead, int64(GuestRead))
	t.Logf("Permission GuestRefer [%v] %v", GuestRefer, int64(GuestRefer))
	t.Logf("Permission GuestCreate [%v] %v", GuestCreate, int64(GuestCreate))
	t.Logf("Permission GuestUpdate [%v] %v", GuestUpdate, int64(GuestUpdate))
	t.Logf("Permission GuestDelete [%v] %v", GuestDelete, int64(GuestDelete))
	t.Logf("Permission GuestExecute [%v] %v", GuestExecute, int64(GuestExecute))
	t.Logf("Permission GuestCRUD [%v] %v", GuestCRUD, int64(GuestCRUD))

	AllPermissions := []AuthPermission{
		None,
		GuestPeek,
		GuestRead,
		GuestRefer,
		GuestCreate,
		GuestUpdate,
		GuestDelete,
		GuestExecute,
		GuestCRUD,
		UserPeek,
		UserRead,
		UserRefer,
		UserCreate,
		UserUpdate,
		UserDelete,
		UserExecute,
		UserCRUD,
		GroupPeek,
		GroupRead,
		GroupRefer,
		GroupCreate,
		GroupUpdate,
		GroupDelete,
		GroupExecute,
		GroupCRUD,
	}

	for i, p1 := range AllPermissions {
		for j, p2 := range AllPermissions {
			if i == j {
				continue
			}

			if p1 == p2 {
				t.Errorf("Permissions are equal [%v] == [%v]", p1, p2)
			}
		}
	}
}

func TestInvalidateAuthCacheForEmail_NilCache(t *testing.T) {
	// Ensure olricCache is nil — should not panic
	oldCache := olricCache
	olricCache = nil
	defer func() { olricCache = oldCache }()

	InvalidateAuthCacheForEmail("test@example.com")
}

func TestInvalidateAuthCacheForEmail_RemovesEntry(t *testing.T) {
	cfg := olricConfig.New("local")
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("failed to create olric: %v", err)
	}

	go func() {
		_ = emb.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = emb.Shutdown(ctx)
	}()

	// Give olric time to start
	time.Sleep(500 * time.Millisecond)

	client := emb.NewEmbeddedClient()
	dm, err := client.NewDMap("auth-cache-test")
	if err != nil {
		t.Fatalf("failed to create DMap: %v", err)
	}

	// Set the package-level cache to our test DMap
	oldCache := olricCache
	olricCache = dm
	defer func() { olricCache = oldCache }()

	// Put a session user into cache
	session := SessionUser{UserId: 42}
	err = olricCache.Put(context.Background(), "user@test.com", session)
	if err != nil {
		t.Fatalf("failed to put session in cache: %v", err)
	}

	// Verify it's cached
	val, err := olricCache.Get(context.Background(), "user@test.com")
	if err != nil {
		t.Fatalf("expected cached value, got error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil cached value")
	}

	// Invalidate
	InvalidateAuthCacheForEmail("user@test.com")

	// Verify it's gone
	_, err = olricCache.Get(context.Background(), "user@test.com")
	if err == nil {
		t.Error("expected cache miss after invalidation, but got a hit")
	}
}

func TestInvalidateAuthCacheForEmail_NonExistentKey(t *testing.T) {
	cfg := olricConfig.New("local")
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("failed to create olric: %v", err)
	}

	go func() {
		_ = emb.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = emb.Shutdown(ctx)
	}()

	time.Sleep(500 * time.Millisecond)

	client := emb.NewEmbeddedClient()
	dm, err := client.NewDMap("auth-cache-test-2")
	if err != nil {
		t.Fatalf("failed to create DMap: %v", err)
	}

	oldCache := olricCache
	olricCache = dm
	defer func() { olricCache = oldCache }()

	// Should not panic or error when key doesn't exist
	InvalidateAuthCacheForEmail("nonexistent@test.com")
}
