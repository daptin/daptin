package resource

import (
	stdjson "encoding/json"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestBecomeAdminOwnRowsTransitionsOnlyBootstrapPermission(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`create table category (
		id integer primary key,
		user_account_id integer,
		permission integer
	)`)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	_, err = db.Exec(`insert into category (id, user_account_id, permission) values
		(1, 0, ?),
		(2, 0, ?)`,
		int64(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN),
		int64(15811),
	)
	if err != nil {
		t.Fatalf("insert category rows: %v", err)
	}

	columns := []api2go.ColumnInfo{
		{ColumnName: USER_ACCOUNT_ID_COLUMN},
		{ColumnName: "permission"},
	}
	crud := &DbResource{
		model: api2go.NewApi2GoModel("category", columns, int64(auth.DEFAULT_PERMISSION), nil),
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := becomeAdminOwnRows(crud, 42, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("become admin own rows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	rows, err := db.Queryx(`select id, user_account_id, permission from category order by id`)
	if err != nil {
		t.Fatalf("select category rows: %v", err)
	}
	defer rows.Close()

	got := map[int]struct {
		userId     int64
		permission int64
	}{}
	for rows.Next() {
		var id int
		var userId int64
		var permission int64
		if err := rows.Scan(&id, &userId, &permission); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		got[id] = struct {
			userId     int64
			permission int64
		}{userId: userId, permission: permission}
	}

	if got[1].userId != 42 || got[1].permission != int64(auth.DEFAULT_PERMISSION) {
		t.Fatalf("expected bootstrap row to be owned by 42 with default permission, got %+v", got[1])
	}
	if got[2].userId != 42 || got[2].permission != 15811 {
		t.Fatalf("expected explicit row permission to be preserved, got %+v", got[2])
	}
}

func TestUpdateDefaultPermissionInSchemaJsonTransitionsOnlyBootstrapDefaults(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`create table world (
		id integer primary key,
		table_name text,
		world_schema_json text
	)`)
	if err != nil {
		t.Fatalf("create world: %v", err)
	}

	_, err = db.Exec(`insert into world (id, table_name, world_schema_json) values
		(1, 'private_table', ?),
		(2, 'public_table', ?),
		(3, 'legacy_table', ?),
		(4, 'private_table_audit', ?)`,
		`{"TableName":"private_table","DefaultPermission":`+intString(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)+`}`,
		`{"TableName":"public_table","DefaultPermission":15811}`,
		`{"TableName":"legacy_table"}`,
		`{"TableName":"private_table_audit","DefaultPermission":`+intString(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)+`}`,
	)
	if err != nil {
		t.Fatalf("insert world rows: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = updateDefaultPermissionInSchemaJson(tx, auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN, auth.DEFAULT_PERMISSION, auth.UserRead|auth.GroupRead)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("update schema json: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	assertSchemaDefaultPermission(t, db, "private_table", int64(auth.DEFAULT_PERMISSION))
	assertSchemaDefaultPermission(t, db, "public_table", 15811)
	assertSchemaDefaultPermission(t, db, "legacy_table", int64(auth.DEFAULT_PERMISSION))
	assertSchemaDefaultPermission(t, db, "private_table_audit", int64(auth.UserRead|auth.GroupRead))
}

func TestBecomeAdminTransitionActionPermissionsPreservesSchemaPermissions(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`create table action (
		id integer primary key,
		action_name text,
		world_id integer,
		permission integer
	)`)
	if err != nil {
		t.Fatalf("create action: %v", err)
	}

	publicRoutePermission := int64(auth.GuestExecute | auth.GuestPeek | auth.UserRead | auth.UserExecute | auth.GroupRead | auth.GroupExecute)
	lockedActionPermission := int64(auth.None)
	bootstrapActionPermission := int64(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)
	_, err = db.Exec(`create table world (
		id integer primary key,
		table_name text
	)`)
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	_, err = db.Exec(`insert into world (id, table_name) values
		(1, 'user_account'),
		(2, 'other_resource')`)
	if err != nil {
		t.Fatalf("insert worlds: %v", err)
	}

	_, err = db.Exec(`insert into action (id, action_name, world_id, permission) values
		(1, 'bootstrap_action', 2, ?),
		(2, 'get_canaster_document_by_public_path', 2, ?),
		(3, 'set_canaster_document_private', 2, ?),
		(4, 'signin', 1, ?),
		(5, 'signup', 1, ?),
		(6, 'signup', 2, ?)`,
		bootstrapActionPermission,
		publicRoutePermission,
		lockedActionPermission,
		bootstrapActionPermission,
		publicRoutePermission,
		publicRoutePermission,
	)
	if err != nil {
		t.Fatalf("insert action rows: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := becomeAdminTransitionActionPermissions(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("transition action permissions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	postBootstrapPermission := int64(auth.UserRead | auth.UserExecute | auth.GroupCRUD | auth.GroupExecute | auth.GroupRefer)
	assertActionPermission(t, db, 1, postBootstrapPermission)
	assertActionPermission(t, db, 2, publicRoutePermission)
	assertActionPermission(t, db, 3, lockedActionPermission)
	assertActionPermission(t, db, 4, publicRoutePermission)
	assertActionPermission(t, db, 5, postBootstrapPermission)
	assertActionPermission(t, db, 6, publicRoutePermission)
}

func TestApplyBecomeAdminLiveDefaultsTransitionsBootstrapPermissions(t *testing.T) {
	normal := becomeAdminLiveCrud("project", auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)
	audit := becomeAdminLiveCrud("project_audit", auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)
	explicit := becomeAdminLiveCrud("public_project", 15811)

	ApplyBecomeAdminLiveDefaults(map[string]*DbResource{
		"project":        normal,
		"project_audit":  audit,
		"public_project": explicit,
	})

	if normal.defaultPermission() != int64(auth.DEFAULT_PERMISSION) || normal.TableInfo().DefaultPermission != auth.DEFAULT_PERMISSION {
		t.Fatalf("normal table default = model %d table %d, want %d",
			normal.defaultPermission(), normal.TableInfo().DefaultPermission, auth.DEFAULT_PERMISSION)
	}
	if normal.TableInfo().Permission != auth.DEFAULT_PERMISSION {
		t.Fatalf("normal table permission = %d, want %d", normal.TableInfo().Permission, auth.DEFAULT_PERMISSION)
	}
	if audit.defaultPermission() != int64(AuditRowDefaultPermission) || audit.TableInfo().DefaultPermission != AuditRowDefaultPermission {
		t.Fatalf("audit table default = model %d table %d, want %d",
			audit.defaultPermission(), audit.TableInfo().DefaultPermission, AuditRowDefaultPermission)
	}
	if audit.TableInfo().Permission != AuditTablePermission {
		t.Fatalf("audit table permission = %d, want %d", audit.TableInfo().Permission, AuditTablePermission)
	}
	if explicit.defaultPermission() != 15811 || explicit.TableInfo().DefaultPermission != 15811 || explicit.TableInfo().Permission != 15811 {
		t.Fatalf("explicit table changed: model %d default %d permission %d",
			explicit.defaultPermission(), explicit.TableInfo().DefaultPermission, explicit.TableInfo().Permission)
	}
}

// Exercise the actual insert path while live metadata is being published, including
// a table whose explicit permission must not change.
func TestBecomeAdminLiveDefaultsConcurrentCreate(t *testing.T) {
	for _, initial := range []auth.AuthPermission{auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN, 15811} {
		t.Run(strconv.FormatInt(int64(initial), 10), func(t *testing.T) {
			db, err := sqlx.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			// This fixture exercises the resource persistence layer.
			_, err = db.Exec(`create table project (
				id integer primary key, reference_id blob, permission integer,
				created_at timestamp, updated_at timestamp)`)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := db.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			project := becomeAdminLiveCrud("project", initial)
			cruds := map[string]*DbResource{"project": project}
			project.Cruds = cruds
			original := project.TableInfo()
			stop, done := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				for {
					select {
					case <-stop:
						return
					default:
						ApplyBecomeAdminLiveDefaults(cruds)
						runtime.Gosched()
					}
				}
			}()
			defer func() { close(stop); <-done }()
			want := becomeAdminLiveDefaultPermission("project", initial)
			for i := 0; i < 100; i++ {
				row, err := project.CreateWithoutFilter(
					api2go.NewApi2GoModelWithData("project", nil, 0, nil, map[string]interface{}{}),
					api2go.Request{PlainRequest: httptest.NewRequest("POST", "/api/project", nil)}, tx)
				if err != nil {
					t.Fatal(err)
				}
				got := auth.AuthPermission(row["permission"].(int64))
				if got != initial && got != want {
					t.Fatalf("insert permission = %d, want %d or %d", got, initial, want)
				}
				table := project.TableInfo()
				if table.DefaultPermission != initial && table.DefaultPermission != want {
					t.Fatalf("table default = %d", table.DefaultPermission)
				}
				if original.DefaultPermission != initial || original.Permission != initial {
					t.Fatal("published transition mutated previously returned metadata")
				}
			}
			ApplyBecomeAdminLiveDefaults(cruds)
			row, err := project.CreateWithoutFilter(
				api2go.NewApi2GoModelWithData("project", nil, 0, nil, map[string]interface{}{}),
				api2go.Request{PlainRequest: httptest.NewRequest("POST", "/api/project", nil)}, tx)
			if err != nil {
				t.Fatal(err)
			}
			if got := auth.AuthPermission(row["permission"].(int64)); got != want {
				t.Fatalf("post-transition insert permission = %d, want %d", got, want)
			}
		})
	}
}

func TestBecomeAdminCommitFailureLeavesLiveDefaults(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	project := becomeAdminLiveCrud("project", auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback tx: %v", err)
	}
	err = CommitBecomeAdmin(tx, map[string]*DbResource{"project": project})
	if err == nil {
		t.Fatal("expected commit error")
	}
	if project.defaultPermission() != int64(auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN) {
		t.Fatalf("live default permission = %d, want pre-admin %d",
			project.defaultPermission(), auth.DEFAULT_PERMISSION_WHEN_NO_ADMIN)
	}
}

func becomeAdminLiveCrud(tableName string, permission auth.AuthPermission) *DbResource {
	return &DbResource{
		model: api2go.NewApi2GoModel(tableName, nil, int64(permission), nil),
		tableInfo: &table_info.TableInfo{
			TableName:         tableName,
			DefaultPermission: permission,
			Permission:        permission,
		},
	}
}

func TestNewImportAdminSessionUserIncludesAdminGroup(t *testing.T) {
	adminUserId := int64(12)
	adminUserRefId := daptinid.DaptinReferenceId(uuid.New())
	adminGroupId := daptinid.DaptinReferenceId(uuid.New())

	sessionUser := newImportAdminSessionUser(adminUserId, adminUserRefId, map[string]*DbResource{
		"usergroup": {
			AdministratorGroupId: adminGroupId,
		},
	})

	if sessionUser.UserId != adminUserId {
		t.Fatalf("expected user id %d, got %d", adminUserId, sessionUser.UserId)
	}
	if sessionUser.UserReferenceId != adminUserRefId {
		t.Fatalf("expected admin user reference id to be preserved")
	}
	if len(sessionUser.Groups) != 1 {
		t.Fatalf("expected one admin group, got %d", len(sessionUser.Groups))
	}
	if sessionUser.Groups[0].GroupReferenceId != adminGroupId {
		t.Fatalf("expected admin group reference id to be set")
	}
}

func assertActionPermission(t *testing.T, db *sqlx.DB, actionID int64, expected int64) {
	t.Helper()
	var permission int64
	if err := db.QueryRow(`select permission from action where id = ?`, actionID).Scan(&permission); err != nil {
		t.Fatalf("select action permission for %d: %v", actionID, err)
	}
	if permission != expected {
		t.Fatalf("expected action %d permission %d, got %d", actionID, expected, permission)
	}
}

func assertSchemaDefaultPermission(t *testing.T, db *sqlx.DB, tableName string, expected int64) {
	t.Helper()
	var schemaJSON string
	if err := db.QueryRow(`select world_schema_json from world where table_name = ?`, tableName).Scan(&schemaJSON); err != nil {
		t.Fatalf("select schema json for %s: %v", tableName, err)
	}

	var schema map[string]interface{}
	if err := stdjson.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		t.Fatalf("unmarshal schema json for %s: %v", tableName, err)
	}

	got, ok := schema["DefaultPermission"].(float64)
	if !ok {
		t.Fatalf("schema json for %s has no numeric DefaultPermission: %#v", tableName, schema["DefaultPermission"])
	}
	if int64(got) != expected {
		t.Fatalf("expected DefaultPermission %d for %s, got %d", expected, tableName, int64(got))
	}
}

func intString(permission auth.AuthPermission) string {
	return strconv.FormatInt(int64(permission), 10)
}
