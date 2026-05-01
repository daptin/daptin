package actions

import (
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func parseTestSessionToken(t *testing.T, tokenString string, secret []byte) jwt.MapClaims {
	t.Helper()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected token to be valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("expected map claims, got %T", token.Claims)
	}
	return claims
}

func TestNewAuthSessionTokenIncludesAuthVersion(t *testing.T) {
	secret := []byte("test-secret")
	refId := uuid.New()
	issuedAt := time.Now().UTC()
	tokenString, err := newAuthSessionToken(secret, 3, "issuer", map[string]interface{}{
		"email":        "user@example.com",
		"name":         "Test User",
		"reference_id": refId,
		"auth_version": int64(7),
	}, issuedAt, map[string]interface{}{
		"picture": "https://example.com/avatar.png",
	})
	if err != nil {
		t.Fatalf("create session token: %v", err)
	}

	claims := parseTestSessionToken(t, tokenString, secret)
	if claims["email"] != "user@example.com" {
		t.Fatalf("expected email claim, got %v", claims["email"])
	}
	if claims["sub"] != refId.String() {
		t.Fatalf("expected sub %s, got %v", refId.String(), claims["sub"])
	}
	if claims[auth.AuthVersionClaim] != float64(7) {
		t.Fatalf("expected auth_version 7, got %v", claims[auth.AuthVersionClaim])
	}
	if claims["picture"] != "https://example.com/avatar.png" {
		t.Fatalf("expected extra picture claim, got %v", claims["picture"])
	}
}

func TestNewAuthSessionTokenDefaultsAuthVersionForExistingUsers(t *testing.T) {
	secret := []byte("test-secret")
	tokenString, err := newAuthSessionToken(secret, 3, "issuer", map[string]interface{}{
		"email":        "legacy@example.com",
		"name":         "Legacy User",
		"reference_id": uuid.New(),
	}, time.Now().UTC(), nil)
	if err != nil {
		t.Fatalf("create session token: %v", err)
	}

	claims := parseTestSessionToken(t, tokenString, secret)
	if claims[auth.AuthVersionClaim] != float64(1) {
		t.Fatalf("expected default auth_version 1, got %v", claims[auth.AuthVersionClaim])
	}
}

func TestGenerateJwtTokenActionIssuesAuthVersionClaim(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	userRef := uuid.New()
	groupRef := uuid.New()
	passwordHash, err := resource.BcryptHashString("CorrectPass123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	statements := []string{
		`create table usergroup (id integer primary key, name text, reference_id blob)`,
		`create table user_account (
			id integer primary key,
			name text,
			email text,
			password text,
			auth_version integer not null default 1,
			version integer not null default 1,
			created_at timestamp,
			updated_at timestamp,
			reference_id blob not null,
			permission integer
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}
	if _, err := db.Exec(`insert into usergroup (id, name, reference_id) values (?, ?, ?)`, 2, "administrators", groupRef[:]); err != nil {
		t.Fatalf("insert admin group: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, name, email, password, auth_version, version, reference_id, permission) values (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "Test User", "user@example.com", passwordHash, 4, 1, userRef[:], 0); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	cfg := olricConfig.New("local")
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("create olric: %v", err)
	}
	client := emb.NewEmbeddedClient()
	oldCache := resource.OlricCache
	resource.OlricCache = nil
	defer func() { resource.OlricCache = oldCache }()

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", DataType: "varchar(80)", ColumnType: "label"},
		{Name: "email", ColumnName: "email", DataType: "varchar(80)", ColumnType: "email"},
		{Name: "password", ColumnName: "password", DataType: "varchar(100)", ColumnType: "password", IsNullable: true},
		{Name: auth.AuthVersionColumn, ColumnName: auth.AuthVersionColumn, DataType: "INTEGER", ColumnType: "measurement", DefaultValue: "1", ExcludeFromApi: true},
	}
	columns = append(columns, resource.StandardColumns...)
	model := api2go.NewApi2GoModel(resource.USER_ACCOUNT_TABLE_NAME, columns, int64(auth.DEFAULT_PERMISSION), nil)
	userCrud, err := resource.NewDbResource(model, db, nil, map[string]*resource.DbResource{}, nil, client, table_info.TableInfo{
		TableName: resource.USER_ACCOUNT_TABLE_NAME,
		Columns:   columns,
	})
	if err != nil {
		t.Fatalf("create user resource: %v", err)
	}
	cruds := map[string]*resource.DbResource{resource.USER_ACCOUNT_TABLE_NAME: userCrud}
	userCrud.Cruds = cruds

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	performer := generateJwtTokenActionPerformer{
		cruds:          cruds,
		secret:         []byte("test-secret"),
		tokenLifeTime:  3,
		jwtTokenIssuer: "issuer",
	}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"email":    "user@example.com",
		"password": "CorrectPass123!",
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("jwt action returned errors: %v", errs)
	}
	if len(responses) == 0 || responses[0].ResponseType != "client.store.set" {
		t.Fatalf("expected client.store.set response, got %v", responses)
	}
	attrs := responses[0].Attributes.(map[string]interface{})
	claims := parseTestSessionToken(t, attrs["value"].(string), []byte("test-secret"))
	if claims[auth.AuthVersionClaim] != float64(4) {
		t.Fatalf("expected auth_version 4, got %v", claims[auth.AuthVersionClaim])
	}
}
