package auth

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	daptinid "github.com/daptin/daptin/server/id"
	jwtmiddleware "github.com/daptin/daptin/server/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func startTestOlric(t *testing.T) (*olric.Olric, *olric.EmbeddedClient) {
	t.Helper()
	port, err := freeTCPPort(t)
	if err != nil {
		t.Fatalf("failed to allocate olric port: %v", err)
	}
	started := make(chan struct{})
	cfg := olricConfig.New("local")
	cfg.BindAddr = "127.0.0.1"
	cfg.BindPort = port
	cfg.MemberlistConfig.BindAddr = "127.0.0.1"
	cfg.MemberlistConfig.BindPort = 0
	cfg.MemberlistConfig.Name = net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.BindPort))
	cfg.LogOutput = nil
	cfg.Started = func() {
		close(started)
	}
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("failed to create olric: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- emb.Start() }()

	select {
	case <-started:
	case err := <-errCh:
		t.Fatalf("failed to start olric: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for olric to start")
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = emb.Shutdown(ctx)
		select {
		case err := <-errCh:
			if err != nil {
				t.Logf("olric stopped with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Log("timed out waiting for olric shutdown")
		}
	})

	return emb, emb.NewEmbeddedClient()
}

func freeTCPPort(t *testing.T) (int, error) {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

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

func TestAuthVersionFromValue(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int64
		ok    bool
	}{
		{name: "int", value: int(3), want: 3, ok: true},
		{name: "int64", value: int64(4), want: 4, ok: true},
		{name: "float64", value: float64(5), want: 5, ok: true},
		{name: "string", value: "6", want: 6, ok: true},
		{name: "missing", value: nil, ok: false},
		{name: "empty string", value: "", ok: false},
		{name: "invalid string", value: "not-a-version", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AuthVersionFromValue(tc.value)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestValidateJWTAuthVersion(t *testing.T) {
	t.Run("matching version accepts", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email":          "user@example.com",
			AuthVersionClaim: float64(2),
		})
		if err := ValidateJWTAuthVersion(token, &SessionUser{AuthVersion: 2}); err != nil {
			t.Fatalf("expected token to pass auth version validation: %v", err)
		}
	})

	t.Run("mismatched version rejects", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email":          "user@example.com",
			AuthVersionClaim: float64(1),
		})
		if err := ValidateJWTAuthVersion(token, &SessionUser{AuthVersion: 2}); err == nil {
			t.Fatal("expected stale token to fail auth version validation")
		}
	})

	t.Run("missing version rejects legacy token", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email": "user@example.com",
		})
		if err := ValidateJWTAuthVersion(token, &SessionUser{AuthVersion: 2}); err == nil {
			t.Fatal("expected legacy token without auth_version to fail")
		}
	})

	t.Run("missing session rejects token", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			AuthVersionClaim: float64(2),
		})
		if err := ValidateJWTAuthVersion(token, nil); err == nil {
			t.Fatal("expected token without session user to fail")
		}
	})
}

func TestAuthCheckMiddlewareJWTAuthVersionLifecycle(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	userRef := uuid.New()
	groupRef := uuid.New()
	relationRef := uuid.New()
	setupStatements := []string{
		`create table user_account (id integer primary key, email text, name text, reference_id blob, auth_version integer not null default 1)`,
		`create table usergroup (id integer primary key, name text, reference_id blob)`,
		`create table user_account_user_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			user_account_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp
		)`,
	}
	for _, statement := range setupStatements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}
	if _, err := db.Exec(`insert into user_account (id, email, name, reference_id, auth_version) values (?, ?, ?, ?, ?)`, 1, "user@example.com", "Test User", userRef[:], 2); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`insert into usergroup (id, name, reference_id) values (?, ?, ?)`, 1, "users", groupRef[:]); err != nil {
		t.Fatalf("insert usergroup: %v", err)
	}
	if _, err := db.Exec(
		`insert into user_account_user_account_id_has_usergroup_usergroup_id (id, user_account_id, usergroup_id, reference_id, permission, created_at) values (?, ?, ?, ?, ?, ?)`,
		1, 1, 1, relationRef[:], int64(UserRead), time.Now(),
	); err != nil {
		t.Fatalf("insert usergroup relation: %v", err)
	}

	oldJWTMiddleware := jwtMiddleware
	oldAuthCache := olricCache
	oldTokenCache := jwtmiddleware.TokenCache
	defer func() {
		jwtMiddleware = oldJWTMiddleware
		olricCache = oldAuthCache
		jwtmiddleware.TokenCache = oldTokenCache
	}()
	olricCache = nil
	jwtmiddleware.TokenCache = nil

	secret := []byte("jwt-secret")
	issuer := "issuer"
	_, olricClient := startTestOlric(t)
	InitJwtMiddleware(secret, issuer, olricClient)
	authMiddleware := &AuthMiddleware{db: db, olricDb: olricClient}

	newRequest := func(authVersion interface{}) *http.Request {
		claims := jwt.MapClaims{
			"email": "user@example.com",
			"name":  "Test User",
			"sub":   userRef.String(),
			"nbf":   time.Now().Add(-time.Minute).Unix(),
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iss":   issuer,
			"iat":   time.Now().Unix(),
			"jti":   uuid.New().String(),
		}
		if authVersion != nil {
			claims[AuthVersionClaim] = authVersion
		}
		tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/user_account", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		return req
	}

	t.Run("matching token continues and attaches session user", func(t *testing.T) {
		ok, abort, req := authMiddleware.AuthCheckMiddlewareWithHttp(newRequest(float64(2)), httptest.NewRecorder(), false)
		if !ok || abort {
			t.Fatalf("expected matching token to continue, got ok=%v abort=%v", ok, abort)
		}
		sessionUser, ok := req.Context().Value("user").(*SessionUser)
		if !ok || sessionUser == nil {
			t.Fatal("expected session user in request context")
		}
		if sessionUser.UserId != 1 {
			t.Fatalf("expected session user id 1, got %d", sessionUser.UserId)
		}
		if sessionUser.AuthVersion != 2 {
			t.Fatalf("expected session auth version 2, got %d", sessionUser.AuthVersion)
		}
	})

	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite before cache-hit checks: %v", err)
	}

	t.Run("matching cached token continues without database lookup", func(t *testing.T) {
		ok, abort, req := authMiddleware.AuthCheckMiddlewareWithHttp(newRequest(float64(2)), httptest.NewRecorder(), false)
		if !ok || abort {
			t.Fatalf("expected cached matching token to continue, got ok=%v abort=%v", ok, abort)
		}
		sessionUser, ok := req.Context().Value("user").(*SessionUser)
		if !ok || sessionUser == nil {
			t.Fatal("expected cached session user in request context")
		}
		if sessionUser.AuthVersion != 2 {
			t.Fatalf("expected cached session auth version 2, got %d", sessionUser.AuthVersion)
		}
	})

	t.Run("stale token is unauthorized", func(t *testing.T) {
		ok, abort, _ := authMiddleware.AuthCheckMiddlewareWithHttp(newRequest(float64(1)), httptest.NewRecorder(), false)
		if ok || abort {
			t.Fatalf("expected stale token to stop without abort, got ok=%v abort=%v", ok, abort)
		}
	})

	t.Run("legacy token without auth_version is unauthorized", func(t *testing.T) {
		ok, abort, _ := authMiddleware.AuthCheckMiddlewareWithHttp(newRequest(nil), httptest.NewRecorder(), false)
		if ok || abort {
			t.Fatalf("expected legacy token to stop without abort, got ok=%v abort=%v", ok, abort)
		}
	})
}

func TestSessionUserBinaryRoundTripIncludesAuthVersion(t *testing.T) {
	referenceID := daptinid.DaptinReferenceId(uuid.New())
	sessionUser := SessionUser{
		UserId:          42,
		UserReferenceId: referenceID,
		AuthVersion:     7,
	}

	data, err := sessionUser.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal session user: %v", err)
	}

	var decoded SessionUser
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatalf("unmarshal session user: %v", err)
	}

	if decoded.UserId != sessionUser.UserId {
		t.Fatalf("expected user id %d, got %d", sessionUser.UserId, decoded.UserId)
	}
	if decoded.UserReferenceId != sessionUser.UserReferenceId {
		t.Fatalf("expected reference id %s, got %s", sessionUser.UserReferenceId, decoded.UserReferenceId)
	}
	if decoded.AuthVersion != sessionUser.AuthVersion {
		t.Fatalf("expected auth version %d, got %d", sessionUser.AuthVersion, decoded.AuthVersion)
	}
}

func TestSessionUserBinaryLegacyCacheDefaultsAuthVersion(t *testing.T) {
	referenceID := daptinid.DaptinReferenceId(uuid.New())

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, 42)
	refData, err := referenceID.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal reference id: %v", err)
	}
	data = append(data, refData...)

	var decoded SessionUser
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatalf("unmarshal legacy session user: %v", err)
	}

	if decoded.AuthVersion != 1 {
		t.Fatalf("expected legacy cache auth version default 1, got %d", decoded.AuthVersion)
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
	_, client := startTestOlric(t)
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
	_, client := startTestOlric(t)
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
