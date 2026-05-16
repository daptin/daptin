package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
)

func TestOAuthAuthorizationOptionsIncludePKCEWhenEnabled(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	row := map[string]interface{}{
		"pkce_enabled":          true,
		"pkce_challenge_method": "S256",
	}
	opts, err := oauthAuthorizationOptions(row, verifier)
	if err != nil {
		t.Fatalf("build options: %v", err)
	}

	conf := &oauth2.Config{
		ClientID:    "client-id",
		RedirectURL: "https://example.com/oauth",
		Scopes:      []string{"data.records:read"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://airtable.com/oauth2/v1/authorize",
			TokenURL: "https://airtable.com/oauth2/v1/token",
		},
	}
	authURL, err := url.Parse(conf.AuthCodeURL("state-value", opts...))
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	params := authURL.Query()
	if got := params.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("expected S256 challenge method, got %q", got)
	}
	if got, want := params.Get("code_challenge"), resource.OAuthPKCES256(verifier); got != want {
		t.Fatalf("unexpected code_challenge: got %q want %q", got, want)
	}
}

func TestOAuthAuthorizationOptionsOmitPKCEWhenDisabled(t *testing.T) {
	row := map[string]interface{}{
		"pkce_enabled":        false,
		"access_type_offline": true,
	}
	opts, err := oauthAuthorizationOptions(row, "")
	if err != nil {
		t.Fatalf("build options: %v", err)
	}

	conf := &oauth2.Config{
		ClientID:    "client-id",
		RedirectURL: "https://example.com/oauth",
		Scopes:      []string{"profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.example.com/auth",
			TokenURL: "https://accounts.example.com/token",
		},
	}
	authURL, err := url.Parse(conf.AuthCodeURL("state-value", opts...))
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	params := authURL.Query()
	if got := params.Get("code_challenge"); got != "" {
		t.Fatalf("expected no code_challenge, got %q", got)
	}
	if got := params.Get("code_challenge_method"); got != "" {
		t.Fatalf("expected no code_challenge_method, got %q", got)
	}
	if got := params.Get("access_type"); got != "offline" {
		t.Fatalf("expected offline access type, got %q", got)
	}
}

func TestExchangeOAuthCodePassesVerifierOnlyForPKCE(t *testing.T) {
	var seenVerifier string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		seenVerifier = r.Form.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	conf := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/oauth",
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenServer.URL,
		},
	}
	row := map[string]interface{}{"pkce_enabled": true}
	state := &oauthStateRecord{CodeVerifier: "stored-verifier"}

	if _, err := exchangeOAuthCode(context.Background(), conf, "auth-code", row, state); err != nil {
		t.Fatalf("exchange pkce code: %v", err)
	}
	if seenVerifier != "stored-verifier" {
		t.Fatalf("expected verifier to be posted, got %q", seenVerifier)
	}

	seenVerifier = "not-reset"
	row = map[string]interface{}{"pkce_enabled": false}
	if _, err := exchangeOAuthCode(context.Background(), conf, "auth-code", row, nil); err != nil {
		t.Fatalf("exchange non-pkce code: %v", err)
	}
	if seenVerifier != "" {
		t.Fatalf("expected verifier to be omitted, got %q", seenVerifier)
	}
}

func TestOAuthStateStoreLoadAndReuseRejection(t *testing.T) {
	db, configStore, cruds, connectRef, sessionUser := setupOAuthStateTestDB(t)
	defer db.Close()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if err := storeOAuthState(cruds, connectRef, "state-value", "stored-verifier", now, sessionUser, tx); err != nil {
		t.Fatalf("store state: %v", err)
	}

	state, err := loadOAuthState(cruds, configStore, connectRef, "state-value", now, true, tx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.CodeVerifier != "stored-verifier" {
		t.Fatalf("unexpected verifier: %q", state.CodeVerifier)
	}
	if state.OwnerUserID != sessionUser.UserId {
		t.Fatalf("unexpected owner user id: got %d want %d", state.OwnerUserID, sessionUser.UserId)
	}
	if state.OwnerUserReferenceID != sessionUser.UserReferenceId {
		t.Fatalf("unexpected owner reference id: got %s want %s", state.OwnerUserReferenceID, sessionUser.UserReferenceId)
	}
	if _, err := loadOAuthState(cruds, configStore, daptinid.DaptinReferenceId(uuid.New()), "state-value", now, true, tx); err == nil {
		t.Fatalf("expected state for different oauth_connect to be rejected")
	}

	if err := markOAuthStateUsed(cruds, state, now, sessionUser, tx); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if _, err := loadOAuthState(cruds, configStore, connectRef, "state-value", now, true, tx); err == nil {
		t.Fatalf("expected used state to be rejected")
	}
}

func TestOAuthStateStoreLoadWithoutVerifierForNonPKCE(t *testing.T) {
	db, configStore, cruds, connectRef, sessionUser := setupOAuthStateTestDB(t)
	defer db.Close()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if err := storeOAuthState(cruds, connectRef, "state-value", "", now, sessionUser, tx); err != nil {
		t.Fatalf("store state: %v", err)
	}

	state, err := loadOAuthState(cruds, configStore, connectRef, "state-value", now, false, tx)
	if err != nil {
		t.Fatalf("load non-pkce state: %v", err)
	}
	if state.CodeVerifier != "" {
		t.Fatalf("expected empty verifier for non-pkce state, got %q", state.CodeVerifier)
	}
	if _, err := loadOAuthState(cruds, configStore, connectRef, "state-value", now, true, tx); err == nil {
		t.Fatalf("expected verifier-required load to reject non-pkce state")
	}
}

func TestOAuthStateOwnerSessionUsesStoredOwner(t *testing.T) {
	ownerRef := daptinid.DaptinReferenceId(uuid.New())
	fallbackRef := daptinid.DaptinReferenceId(uuid.New())
	fallback := &auth.SessionUser{
		UserId:          99,
		UserReferenceId: fallbackRef,
		Groups: auth.GroupPermissionList{{
			GroupReferenceId: daptinid.DaptinReferenceId(uuid.New()),
			Permission:       auth.GroupRead,
		}},
		AuthVersion: 3,
	}
	state := &oauthStateRecord{
		OwnerUserID:          1,
		OwnerUserReferenceID: ownerRef,
	}

	sessionUser := oauthStateOwnerSession(state, fallback)
	if sessionUser.UserId != 1 {
		t.Fatalf("expected state owner user id, got %d", sessionUser.UserId)
	}
	if sessionUser.UserReferenceId != ownerRef {
		t.Fatalf("expected state owner reference id, got %s", sessionUser.UserReferenceId)
	}
	if len(sessionUser.Groups) != len(fallback.Groups) {
		t.Fatalf("expected fallback groups to be preserved")
	}
	if sessionUser.AuthVersion != fallback.AuthVersion {
		t.Fatalf("expected fallback auth version to be preserved")
	}
}

func TestOAuthStateExpiry(t *testing.T) {
	db, configStore, cruds, connectRef, sessionUser := setupOAuthStateTestDB(t)
	defer db.Close()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if err := storeOAuthState(cruds, connectRef, "state-value", "stored-verifier", now, sessionUser, tx); err != nil {
		t.Fatalf("store state: %v", err)
	}
	if _, err := loadOAuthState(cruds, configStore, connectRef, "state-value", now.Add(11*time.Minute), true, tx); err == nil {
		t.Fatalf("expected expired state to be rejected")
	}
}

func TestOAuthLoginBeginStoresNonNumericStateForNonPKCE(t *testing.T) {
	db, configStore, cruds, connectRef, sessionUser := setupOAuthStateTestDB(t)
	defer db.Close()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	performer := &oauthLoginBeginActionPerformer{cruds: cruds}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"authenticator": "airtable",
		"sessionUser":   sessionUser,
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("oauth login begin failed: %v", errs)
	}

	var state string
	var location string
	for _, response := range responses {
		switch response.ResponseType {
		case "client.store.set":
			attrs := response.Attributes.(map[string]interface{})
			if attrs["key"] == "secret" {
				state, _ = attrs["value"].(string)
			}
		case "client.redirect":
			attrs := response.Attributes.(map[string]interface{})
			location, _ = attrs["location"].(string)
		}
	}
	if state == "" {
		t.Fatalf("expected state response")
	}
	if regexp.MustCompile(`^[0-9]+$`).MatchString(state) {
		t.Fatalf("state should not be numeric-only: %q", state)
	}
	authURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	if got := authURL.Query().Get("state"); got != state {
		t.Fatalf("auth url state mismatch: got %q want %q", got, state)
	}
	if got := authURL.Query().Get("access_type"); got != "offline" {
		t.Fatalf("expected offline access type, got %q", got)
	}
	storedState, err := loadOAuthState(cruds, configStore, connectRef, state, time.Now().UTC(), false, tx)
	if err != nil {
		t.Fatalf("expected generated state to be stored: %v", err)
	}
	if storedState.CodeVerifier != "" {
		t.Fatalf("expected no verifier for non-pkce state, got %q", storedState.CodeVerifier)
	}
}

func setupOAuthStateTestDB(t *testing.T) (*sqlx.DB, *resource.ConfigStore, map[string]*resource.DbResource, daptinid.DaptinReferenceId, *auth.SessionUser) {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	configStore, err := resource.NewConfigStore(db)
	if err != nil {
		t.Fatalf("create config store: %v", err)
	}
	secret := "0123456789abcdef0123456789abcdef"
	if _, err := db.Exec(`insert into _config (name, configtype, configstate, configenv, value) values (?, ?, ?, ?, ?)`, "encryption.secret", "backend", "enabled", "release", secret); err != nil {
		t.Fatalf("insert encryption secret: %v", err)
	}
	adminRef := daptinid.DaptinReferenceId(uuid.New())
	connectRef := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(`create table usergroup (id integer primary key, name text, reference_id blob)`); err != nil {
		t.Fatalf("create usergroup: %v", err)
	}
	userRef := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(`create table user_account (id integer primary key, reference_id blob not null unique)`); err != nil {
		t.Fatalf("create user_account: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id) values (?, ?)`, 1, userRef[:]); err != nil {
		t.Fatalf("insert user_account: %v", err)
	}
	if _, err := db.Exec(`insert into usergroup (id, name, reference_id) values (?, ?, ?)`, 2, "administrators", adminRef[:]); err != nil {
		t.Fatalf("insert admin group: %v", err)
	}
	oldUserCrud := resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME]
	resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME] = &resource.DbResource{AdministratorGroupId: adminRef}
	t.Cleanup(func() {
		if oldUserCrud == nil {
			delete(resource.CRUD_MAP, resource.USER_ACCOUNT_TABLE_NAME)
		} else {
			resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME] = oldUserCrud
		}
	})
	if _, err := db.Exec(`create table oauth_connect (
		id integer primary key,
		name text,
		client_id text,
		client_secret text,
		scope text,
		redirect_uri text,
		auth_url text,
		token_url text,
		access_type_offline bool,
		pkce_enabled bool,
		pkce_challenge_method text,
		version integer default 1,
		created_at timestamp,
		updated_at timestamp,
		reference_id blob not null unique,
		permission integer
	)`); err != nil {
		t.Fatalf("create oauth_connect: %v", err)
	}
	encryptedClientSecret, err := resource.Encrypt([]byte(secret), "client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	if _, err := db.Exec(`insert into oauth_connect (
		id, name, client_id, client_secret, scope, redirect_uri, auth_url, token_url,
		access_type_offline, pkce_enabled, pkce_challenge_method, version, reference_id, permission
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		7, "airtable", "client-id", encryptedClientSecret, "profile", "https://example.com/oauth",
		"https://accounts.example.com/auth", "https://accounts.example.com/token",
		true, false, "S256", 1, connectRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert oauth_connect: %v", err)
	}
	if _, err := db.Exec(`create table oauth_state (
		id integer primary key autoincrement,
		state_hash text not null unique,
		code_verifier text null,
		expires_at integer not null,
		used_at integer null,
		oauth_connect_id integer,
		user_account_id integer,
		version integer default 1,
		created_at timestamp,
		updated_at timestamp null,
		reference_id blob not null,
		permission integer
	)`); err != nil {
		t.Fatalf("create oauth_state: %v", err)
	}

	cfg := olricConfig.New("local")
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("create olric: %v", err)
	}
	client := emb.NewEmbeddedClient()

	connectColumns := append([]api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", DataType: "varchar(80)", ColumnType: "label"},
		{Name: "client_id", ColumnName: "client_id", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "client_secret", ColumnName: "client_secret", DataType: "varchar(500)", ColumnType: "encrypted"},
		{Name: "scope", ColumnName: "scope", DataType: "varchar(1000)", ColumnType: "content"},
		{Name: "redirect_uri", ColumnName: "redirect_uri", DataType: "varchar(200)", ColumnType: "url"},
		{Name: "auth_url", ColumnName: "auth_url", DataType: "varchar(200)", ColumnType: "url"},
		{Name: "token_url", ColumnName: "token_url", DataType: "varchar(200)", ColumnType: "url"},
		{Name: "access_type_offline", ColumnName: "access_type_offline", DataType: "bool", ColumnType: "truefalse"},
		{Name: "pkce_enabled", ColumnName: "pkce_enabled", DataType: "bool", ColumnType: "truefalse"},
		{Name: "pkce_challenge_method", ColumnName: "pkce_challenge_method", DataType: "varchar(20)", ColumnType: "label"},
	}, resource.StandardColumns...)
	connectInfo := table_info.TableInfo{TableName: "oauth_connect", Columns: connectColumns}
	connectModel := api2go.NewApi2GoModel("oauth_connect", connectColumns, int64(auth.DEFAULT_PERMISSION), nil)

	stateColumns := append([]api2go.ColumnInfo{
		{Name: "state_hash", ColumnName: "state_hash", ColumnType: "label", DataType: "varchar(128)", IsUnique: true, IsIndexed: true},
		{Name: "code_verifier", ColumnName: "code_verifier", ColumnType: "encrypted", DataType: "varchar(500)", IsNullable: true},
		{Name: "expires_at", ColumnName: "expires_at", ColumnType: "measurement", DataType: "bigint"},
		{Name: "used_at", ColumnName: "used_at", ColumnType: "measurement", DataType: "bigint", IsNullable: true},
		{
			Name:         "oauth_connect_id",
			ColumnName:   "oauth_connect_id",
			ColumnType:   "label",
			DataType:     "INTEGER",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				DataSource: "self",
				Namespace:  "oauth_connect",
				KeyName:    "id",
			},
		},
		{
			Name:         "user_account_id",
			ColumnName:   "user_account_id",
			ColumnType:   "alias",
			DataType:     "INTEGER",
			IsForeignKey: true,
			IsNullable:   true,
			ForeignKeyData: api2go.ForeignKeyData{
				DataSource: "self",
				Namespace:  "user_account",
				KeyName:    "id",
			},
		},
	}, resource.StandardColumns...)
	stateInfo := table_info.TableInfo{TableName: "oauth_state", Columns: stateColumns}
	stateModel := api2go.NewApi2GoModel("oauth_state", stateColumns, int64(auth.DEFAULT_PERMISSION), nil)

	cruds := map[string]*resource.DbResource{}
	connectCrud, err := resource.NewDbResource(connectModel, db, nil, cruds, configStore, client, connectInfo)
	if err != nil {
		t.Fatalf("create oauth_connect resource: %v", err)
	}
	stateCrud, err := resource.NewDbResource(stateModel, db, nil, cruds, configStore, client, stateInfo)
	if err != nil {
		t.Fatalf("create oauth_state resource: %v", err)
	}
	cruds["oauth_connect"] = connectCrud
	cruds["oauth_state"] = stateCrud
	connectCrud.Cruds = cruds
	stateCrud.Cruds = cruds

	sessionUser := &auth.SessionUser{
		UserId:          1,
		UserReferenceId: userRef,
		Groups: auth.GroupPermissionList{{
			GroupReferenceId: adminRef,
			Permission:       auth.DEFAULT_PERMISSION,
		}},
	}
	return db, configStore, cruds, connectRef, sessionUser
}
