package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestUpdateWorldTableSyncsTableAccessGroups(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table user_account (id integer primary key, email text)`,
		`create table usergroup (id integer primary key, name text)`,
		`create table world (
			id integer primary key,
			table_name text,
			world_schema_json text,
			permission integer,
			reference_id blob,
			user_account_id integer,
			is_top_level bool,
			is_hidden bool,
			default_order text,
			is_join_table bool,
			icon text
		)`,
		`create table world_world_id_has_usergroup_usergroup_id (
			id integer primary key,
			world_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
		`insert into user_account (id, email) values (1, 'guest@cms.go')`,
		`insert into user_account (id, email) values (2, 'admin@example.com')`,
		`insert into usergroup (id, name) values (1, 'users')`,
		`insert into world (id, table_name, world_schema_json, permission, reference_id, user_account_id, is_top_level, is_hidden, default_order, is_join_table, icon)
			values (10, 'document', '{"TableName":"document"}', 1, randomblob(16), 2, true, false, '+id', false, 'fa-file')`,
		`insert into world (id, table_name, world_schema_json, permission, reference_id, user_account_id, is_top_level, is_hidden, default_order, is_join_table, icon)
			values (11, 'world_world_id_has_usergroup_usergroup_id', '{"TableName":"world_world_id_has_usergroup_usergroup_id","DefaultPermission":32768}', 1, randomblob(16), 2, false, true, '+id', true, 'fa-link')`,
		`insert into world_world_id_has_usergroup_usergroup_id (id, world_id, usergroup_id, reference_id, permission)
			values (20, 10, 1, randomblob(16), 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	accessPermission := auth.AuthPermission(999424)
	config := CmsConfig{
		Tables: []table_info.TableInfo{
			{
				TableName: "document",
				AccessGroups: table_info.DefaultGroupList{
					{Name: "users", Permission: &accessPermission},
				},
			},
			{
				TableName:         "world_world_id_has_usergroup_usergroup_id",
				DefaultPermission: auth.AuthPermission(32768),
			},
		},
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = UpdateWorldTable(&config, tx)
	if err != nil {
		t.Fatalf("update world table: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit world update: %v", err)
	}

	tx, err = db.Beginx()
	if err != nil {
		t.Fatalf("begin second tx: %v", err)
	}
	err = UpdateWorldTable(&config, tx)
	if err != nil {
		t.Fatalf("second update world table: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit second world update: %v", err)
	}

	var relationCount int
	var relationPermission int64
	err = db.QueryRow(`
		select count(*), coalesce(max(permission), 0)
		from world_world_id_has_usergroup_usergroup_id
		where world_id = 10 and usergroup_id = 1`).
		Scan(&relationCount, &relationPermission)
	if err != nil {
		t.Fatalf("scan world relation: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("expected one world access group relation, got %d", relationCount)
	}
	if relationPermission != int64(accessPermission) {
		t.Fatalf("expected world access group permission %d, got %d", accessPermission, relationPermission)
	}
}
