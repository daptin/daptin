package actions

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
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

func TestOtpLoginVerifyDoesNotPrintIssuedToken(t *testing.T) {
	db, cruds, userRef, otpSecret := setupAuthTokenOutputTestDB(t)

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	code, err := totp.GenerateCodeCustom(otpSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    otpPeriodSeconds,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("generate otp code: %v", err)
	}

	performer := &otpLoginVerifyActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte("12345678901234567890123456789012"),
		secret:           []byte("test-secret"),
		tokenLifeTime:    3,
		jwtTokenIssuer:   "issuer",
	}

	var responses []actionresponse.ActionResponse
	var errs []error
	output := captureProcessOutput(t, func() {
		_, responses, errs = performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
			"email":       "otp@example.com",
			"otp":         code,
			"sessionUser": &auth.SessionUser{UserId: 1, UserReferenceId: userRef},
		}, tx)
	})
	if len(errs) > 0 {
		t.Fatalf("otp verify returned errors: %v", errs)
	}
	if len(responses) == 0 || responses[0].ResponseType != "client.store.set" {
		t.Fatalf("expected token store response, got %v", responses)
	}
	attrs := responses[0].Attributes.(map[string]interface{})
	tokenString := attrs["value"].(string)
	claims := parseTestSessionToken(t, tokenString, []byte("test-secret"))
	if claims["sub"] != userRef.String() {
		t.Fatalf("expected subject %s, got %v", userRef.String(), claims["sub"])
	}
	if strings.Contains(output, tokenString) {
		t.Fatalf("stdout leaked issued OTP login token")
	}
}

func TestOtpLoginVerifyDoesNotLogSubmittedOtpOnFailure(t *testing.T) {
	db, cruds, _, _ := setupAuthTokenOutputTestDB(t)

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	performer := &otpLoginVerifyActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte("12345678901234567890123456789012"),
		secret:           []byte("test-secret"),
		tokenLifeTime:    3,
		jwtTokenIssuer:   "issuer",
	}

	submittedOtp := "not-the-code"
	var errs []error
	output := captureProcessOutput(t, func() {
		_, _, errs = performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
			"email":       "otp@example.com",
			"otp":         submittedOtp,
			"sessionUser": &auth.SessionUser{UserId: 1},
		}, tx)
	})
	if len(errs) == 0 || errs[0].Error() != "Invalid OTP" {
		t.Fatalf("expected invalid OTP error, got %v", errs)
	}
	if strings.Contains(output, "otp@example.com") {
		t.Fatalf("output leaked account email on invalid OTP")
	}
	if strings.Contains(output, submittedOtp) {
		t.Fatalf("output leaked submitted OTP on invalid OTP")
	}
}

func TestOtpPasswordResetVerificationDoesNotIssueSessionToken(t *testing.T) {
	db, cruds, _, otpSecret := setupAuthTokenOutputTestDB(t)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	code, err := totp.GenerateCodeCustom(otpSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    otpPeriodSeconds,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("generate OTP: %v", err)
	}
	performer := &otpLoginVerifyActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte("12345678901234567890123456789012"),
		secret:           []byte("test-secret"),
		tokenLifeTime:    3,
		jwtTokenIssuer:   "issuer",
	}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"email":   "otp@example.com",
		"otp":     code,
		"purpose": "password_reset",
	}, tx)
	if len(errs) > 0 {
		t.Fatalf("password-reset OTP returned errors: %v", errs)
	}
	if len(responses) != 0 {
		t.Fatalf("password-reset OTP must not issue a session response: %v", responses)
	}
}

func TestOtpLoginRejectsUnverifiedEnrollment(t *testing.T) {
	db, cruds, _, otpSecret := setupAuthTokenOutputTestDB(t)
	if _, err := db.Exec(`update user_otp_account set verified = 0 where id = 1`); err != nil {
		t.Fatalf("mark profile unverified: %v", err)
	}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	code, err := totp.GenerateCodeCustom(otpSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    otpPeriodSeconds,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("generate OTP: %v", err)
	}
	performer := &otpLoginVerifyActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte("12345678901234567890123456789012"),
		secret:           []byte("test-secret"),
		tokenLifeTime:    3,
		jwtTokenIssuer:   "issuer",
	}
	_, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"email":       "otp@example.com",
		"otp":         code,
		"sessionUser": &auth.SessionUser{UserId: 1},
	}, tx)
	if len(errs) == 0 || errs[0].Error() != "OTP is not enrolled for this account" {
		t.Fatalf("expected unverified enrollment rejection, got responses=%v errors=%v", responses, errs)
	}
}

func TestPasswordResetBeginDoesNotPrintRecoveryToken(t *testing.T) {
	db, cruds, _, _ := setupAuthTokenOutputTestDB(t)

	var sentBody string
	cruds["mail"] = &resource.DbResource{
		MailSender: func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
			sentBody = e.Data.String()
			return backends.NewResult("200 OK"), nil
		},
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	performer := &generatePasswordResetActionPerformer{
		cruds:                  cruds,
		secret:                 []byte("test-secret"),
		tokenLifeTime:          3,
		jwtTokenIssuer:         "issuer",
		passwordResetEmailFrom: "reset@example.com",
	}

	var errs []error
	output := captureProcessOutput(t, func() {
		_, _, errs = performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
			"email": "otp@example.com",
		}, tx)
	})
	if len(errs) > 0 {
		t.Fatalf("password reset returned errors: %v", errs)
	}

	resetTokenBase64 := strings.TrimPrefix(sentBody, "Reset your password by clicking on this link: ")
	if resetTokenBase64 == sentBody || resetTokenBase64 == "" {
		t.Fatalf("reset mail did not contain expected token body: %q", sentBody)
	}
	rawTokenBytes, err := base64.StdEncoding.DecodeString(resetTokenBase64)
	if err != nil {
		t.Fatalf("decode reset token: %v", err)
	}
	rawToken := string(rawTokenBytes)
	if strings.Contains(output, resetTokenBase64) {
		t.Fatalf("stdout leaked base64 password reset token")
	}
	if strings.Contains(output, rawToken) {
		t.Fatalf("stdout leaked raw password reset token")
	}
}

func captureProcessOutput(t *testing.T, run func()) string {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}
	defer stdoutReader.Close()
	defer stderrReader.Close()

	var logOutput bytes.Buffer
	standardLogger := log.StandardLogger()
	originalLogOutput := standardLogger.Out
	log.SetOutput(&logOutput)
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		log.SetOutput(originalLogOutput)
	}()
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	run()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(stdout) + string(stderr) + logOutput.String()
}

func setupAuthTokenOutputTestDB(t *testing.T) (*sqlx.DB, map[string]*resource.DbResource, daptinid.DaptinReferenceId, string) {
	t.Helper()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)

	statements := []string{
		`create table usergroup (
			id integer primary key,
			name text,
			reference_id blob not null unique,
			permission integer
		)`,
		`create table user_account (
			id integer primary key,
			name text,
			email text,
			password text,
			auth_version integer not null default 1,
			user_account_id integer,
			permission integer,
			version integer not null default 1,
			created_at timestamp,
			updated_at timestamp,
			reference_id blob not null unique
		)`,
		`create table user_otp_account (
			id integer primary key,
			mobile_number text,
			otp_secret text,
			verified integer,
			otp_of_account integer,
			user_account_id integer,
			permission integer,
			version integer not null default 1,
			created_at timestamp,
			updated_at timestamp,
			reference_id blob not null unique
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	userRef := daptinid.DaptinReferenceId(uuid.New())
	groupRef := uuid.New()
	otpRef := uuid.New()
	now := time.Now().UTC()

	if _, err := db.Exec(`insert into usergroup (id, name, reference_id, permission) values (?, ?, ?, ?)`,
		2, "administrators", groupRef[:], int64(auth.ALLOW_ALL_PERMISSIONS)); err != nil {
		db.Close()
		t.Fatalf("insert usergroup: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, name, email, auth_version, user_account_id, permission, version, created_at, updated_at, reference_id) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "OTP User", "otp@example.com", 5, 1, int64(auth.DEFAULT_PERMISSION|auth.UserRefer), 1, now, now, userRef[:]); err != nil {
		db.Close()
		t.Fatalf("insert user_account: %v", err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "issuer",
		AccountName: "otp@example.com",
		Period:      otpPeriodSeconds,
		Digits:      6,
		SecretSize:  20,
	})
	if err != nil {
		db.Close()
		t.Fatalf("generate otp secret: %v", err)
	}
	encryptedSecret, err := resource.Encrypt([]byte("12345678901234567890123456789012"), key.Secret())
	if err != nil {
		db.Close()
		t.Fatalf("encrypt otp secret: %v", err)
	}
	if _, err := db.Exec(`insert into user_otp_account (id, mobile_number, otp_secret, verified, otp_of_account, user_account_id, permission, version, created_at, updated_at, reference_id) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "", encryptedSecret, 1, 1, 1, int64(auth.DEFAULT_PERMISSION), 1, now, now, otpRef[:]); err != nil {
		db.Close()
		t.Fatalf("insert user_otp_account: %v", err)
	}

	userColumns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", DataType: "varchar(80)", ColumnType: "label"},
		{Name: "email", ColumnName: "email", DataType: "varchar(80)", ColumnType: "email"},
		{Name: "password", ColumnName: "password", DataType: "varchar(100)", ColumnType: "password", IsNullable: true},
		{Name: auth.AuthVersionColumn, ColumnName: auth.AuthVersionColumn, DataType: "INTEGER", ColumnType: "measurement", DefaultValue: "1", ExcludeFromApi: true},
	}
	userColumns = append(userColumns, resource.StandardColumns...)
	otpColumns := []api2go.ColumnInfo{
		{Name: "mobile_number", ColumnName: "mobile_number", DataType: "varchar(20)", ColumnType: "label", IsNullable: true},
		{Name: "otp_secret", ColumnName: "otp_secret", DataType: "varchar(100)", ColumnType: "encrypted", ExcludeFromApi: true},
		{Name: "verified", ColumnName: "verified", DataType: "bool", ColumnType: "truefalse", DefaultValue: "false"},
		{
			Name:         "otp_of_account",
			ColumnName:   "otp_of_account",
			DataType:     "int",
			ColumnType:   "belongs_to",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				DataSource: "self",
				Namespace:  resource.USER_ACCOUNT_TABLE_NAME,
				KeyName:    "id",
			},
		},
	}
	otpColumns = append(otpColumns, resource.StandardColumns...)

	client, _ := startOTPTestOlric(t)

	oldCache := resource.OlricCache
	oldUserAccountCrud := resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME]
	oldOtpCrud := resource.CRUD_MAP["user_otp_account"]
	t.Cleanup(func() {
		resource.OlricCache = oldCache
		if oldUserAccountCrud == nil {
			delete(resource.CRUD_MAP, resource.USER_ACCOUNT_TABLE_NAME)
		} else {
			resource.CRUD_MAP[resource.USER_ACCOUNT_TABLE_NAME] = oldUserAccountCrud
		}
		if oldOtpCrud == nil {
			delete(resource.CRUD_MAP, "user_otp_account")
		} else {
			resource.CRUD_MAP["user_otp_account"] = oldOtpCrud
		}
		db.Close()
	})

	cruds := make(map[string]*resource.DbResource)
	userCrud := testDbResource(t, db, client, resource.USER_ACCOUNT_TABLE_NAME, userColumns, nil, cruds)
	otpCrud := testDbResource(t, db, client, "user_otp_account", otpColumns, []api2go.TableRelation{
		api2go.NewTableRelationWithNames("user_otp_account", "primary_user_otp", "belongs_to", resource.USER_ACCOUNT_TABLE_NAME, "otp_of_account"),
	}, cruds)
	cruds[resource.USER_ACCOUNT_TABLE_NAME] = userCrud
	cruds["user_otp_account"] = otpCrud
	userCrud.Cruds = cruds
	otpCrud.Cruds = cruds

	return db, cruds, userRef, key.Secret()
}

func testDbResource(t *testing.T, db *sqlx.DB, client *olric.EmbeddedClient, tableName string, columns []api2go.ColumnInfo, relations []api2go.TableRelation, cruds map[string]*resource.DbResource) *resource.DbResource {
	t.Helper()

	tableInfo := table_info.TableInfo{
		TableName:         tableName,
		Columns:           columns,
		Relations:         relations,
		DefaultPermission: auth.DEFAULT_PERMISSION,
	}
	model := api2go.NewApi2GoModel(tableName, columns, int64(auth.DEFAULT_PERMISSION), relations)
	crud, err := resource.NewDbResource(model, db, &resource.MiddlewareSet{}, cruds, nil, client, tableInfo)
	if err != nil {
		t.Fatalf("create %s resource: %v", tableName, err)
	}
	return crud
}
