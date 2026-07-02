package resource

import (
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestUpdateActionTableSyncsSchemaPermissionAndDefaultGroups(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table user_account (id integer primary key, email text)`,
		`create table usergroup (id integer primary key, name text)`,
		`create table world (id integer primary key, table_name text, world_schema_json text)`,
		`create table action (
			id integer primary key,
			action_name text,
			label text,
			world_id integer,
			action_schema text,
			instance_optional bool,
			user_account_id integer,
			reference_id blob,
			permission integer
		)`,
		`create table action_action_id_has_usergroup_usergroup_id (
			id integer primary key,
			action_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
		`insert into user_account (id, email) values (1, 'admin@example.com')`,
		`insert into usergroup (id, name) values (1, 'administrators')`,
		`insert into usergroup (id, name) values (2, 'operators')`,
		`insert into world (id, table_name, world_schema_json) values (1, 'gig', '{"TableName":"gig"}')`,
		`insert into world (id, table_name, world_schema_json) values (2, 'action', '{"TableName":"action","DefaultGroups":[{"Name":"administrators","Permission":524288}]}')`,
		`insert into world (id, table_name, world_schema_json) values (3, 'action_action_id_has_usergroup_usergroup_id', '{"TableName":"action_action_id_has_usergroup_usergroup_id","DefaultPermission":32768}')`,
		`insert into action (id, action_name, label, world_id, action_schema, instance_optional, user_account_id, reference_id, permission)
			values (10, 'post_gig', 'Post gig', 1, '{}', true, 1, randomblob(16), 0)`,
		`insert into action_action_id_has_usergroup_usergroup_id (id, action_id, usergroup_id, reference_id, permission)
				values (20, 10, 1, randomblob(16), 1)`,
		`insert into action_action_id_has_usergroup_usergroup_id (id, action_id, usergroup_id, reference_id, permission)
				values (21, 10, 2, randomblob(16), 131072)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	actionPermission := auth.AuthPermission(32)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = UpdateActionTable(&CmsConfig{
		Actions: []actionresponse.Action{
			{
				Name:             "post_gig",
				Label:            "Post gig",
				OnType:           "gig",
				InstanceOptional: true,
				Permission:       &actionPermission,
			},
		},
	}, tx)
	if err != nil {
		t.Fatalf("update action table: %v", err)
	}

	var storedActionPermission int64
	err = db.QueryRow(`select permission from action where action_name = 'post_gig'`).Scan(&storedActionPermission)
	if err != nil {
		t.Fatalf("scan action permission: %v", err)
	}
	if storedActionPermission != int64(actionPermission) {
		t.Fatalf("expected action permission %d, got %d", actionPermission, storedActionPermission)
	}

	var relationCount int
	var storedRelationPermission int64
	err = db.QueryRow(`select count(*), max(permission) from action_action_id_has_usergroup_usergroup_id where action_id = 10 and usergroup_id = 1`).
		Scan(&relationCount, &storedRelationPermission)
	if err != nil {
		t.Fatalf("scan relation permission: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("expected one action usergroup relation, got %d", relationCount)
	}
	if storedRelationPermission != 524288 {
		t.Fatalf("expected relation permission 524288, got %d", storedRelationPermission)
	}

	var totalRelationCount int
	err = db.QueryRow(`select count(*) from action_action_id_has_usergroup_usergroup_id where action_id = 10`).Scan(&totalRelationCount)
	if err != nil {
		t.Fatalf("scan total relation count: %v", err)
	}
	if totalRelationCount != 2 {
		t.Fatalf("expected startup sync to preserve custom relation rows, got %d rows", totalRelationCount)
	}

	var customRelationPermission int64
	err = db.QueryRow(`select permission from action_action_id_has_usergroup_usergroup_id where action_id = 10 and usergroup_id = 2`).
		Scan(&customRelationPermission)
	if err != nil {
		t.Fatalf("scan custom relation permission: %v", err)
	}
	if customRelationPermission != 131072 {
		t.Fatalf("expected custom relation permission to remain 131072, got %d", customRelationPermission)
	}
}

func TestUpdateActionTableSyncsSpecificActionAccessGroups(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table user_account (id integer primary key, email text)`,
		`create table usergroup (id integer primary key, name text)`,
		`create table world (id integer primary key, table_name text, world_schema_json text)`,
		`create table action (
			id integer primary key,
			action_name text,
			label text,
			world_id integer,
			action_schema text,
			instance_optional bool,
			user_account_id integer,
			reference_id blob,
			permission integer
		)`,
		`create table action_action_id_has_usergroup_usergroup_id (
			id integer primary key,
			action_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
		`insert into user_account (id, email) values (1, 'admin@example.com')`,
		`insert into usergroup (id, name) values (1, 'users')`,
		`insert into world (id, table_name, world_schema_json) values (1, 'document', '{"TableName":"document"}')`,
		`insert into world (id, table_name, world_schema_json) values (2, 'action', '{"TableName":"action"}')`,
		`insert into world (id, table_name, world_schema_json) values (3, 'action_action_id_has_usergroup_usergroup_id', '{"TableName":"action_action_id_has_usergroup_usergroup_id","DefaultPermission":32768}')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	accessPermission := auth.AuthPermission(524288)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = UpdateActionTable(&CmsConfig{
		Actions: []actionresponse.Action{
			{
				Name:   "set_document_private",
				Label:  "Set document private",
				OnType: "document",
				AccessGroups: table_info.DefaultGroupList{
					{Name: "users", Permission: &accessPermission},
				},
			},
			{
				Name:   "set_document_public",
				Label:  "Set document public",
				OnType: "document",
			},
		},
	}, tx)
	if err != nil {
		t.Fatalf("update action table: %v", err)
	}

	var privateRelationCount int
	var privateRelationPermission int64
	err = db.QueryRow(`
		select count(*), coalesce(max(r.permission), 0)
		from action_action_id_has_usergroup_usergroup_id r
		join action a on a.id = r.action_id
		where a.action_name = 'set_document_private' and r.usergroup_id = 1`).
		Scan(&privateRelationCount, &privateRelationPermission)
	if err != nil {
		t.Fatalf("scan private action relation: %v", err)
	}
	if privateRelationCount != 1 {
		t.Fatalf("expected one private action relation, got %d", privateRelationCount)
	}
	if privateRelationPermission != int64(accessPermission) {
		t.Fatalf("expected private action relation permission %d, got %d", accessPermission, privateRelationPermission)
	}

	var publicRelationCount int
	err = db.QueryRow(`
		select count(*)
		from action_action_id_has_usergroup_usergroup_id r
		join action a on a.id = r.action_id
		where a.action_name = 'set_document_public' and r.usergroup_id = 1`).
		Scan(&publicRelationCount)
	if err != nil {
		t.Fatalf("scan public action relation: %v", err)
	}
	if publicRelationCount != 0 {
		t.Fatalf("expected public action to have no users relation, got %d", publicRelationCount)
	}
}

func TestUpdateActionTableAccessGroupsOverrideBroadDefaultGroups(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table user_account (id integer primary key, email text)`,
		`create table usergroup (id integer primary key, name text)`,
		`create table world (id integer primary key, table_name text, world_schema_json text)`,
		`create table action (
			id integer primary key,
			action_name text,
			label text,
			world_id integer,
			action_schema text,
			instance_optional bool,
			user_account_id integer,
			reference_id blob,
			permission integer
		)`,
		`create table action_action_id_has_usergroup_usergroup_id (
			id integer primary key,
			action_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer
		)`,
		`insert into user_account (id, email) values (1, 'admin@example.com')`,
		`insert into usergroup (id, name) values (1, 'users')`,
		`insert into world (id, table_name, world_schema_json) values (1, 'document', '{"TableName":"document"}')`,
		`insert into world (id, table_name, world_schema_json) values (2, 'action', '{"TableName":"action","DefaultGroups":[{"Name":"users","Permission":1}]}')`,
		`insert into world (id, table_name, world_schema_json) values (3, 'action_action_id_has_usergroup_usergroup_id', '{"TableName":"action_action_id_has_usergroup_usergroup_id","DefaultPermission":32768}')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	actionPermission := auth.AuthPermission(524288)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = UpdateActionTable(&CmsConfig{
		Actions: []actionresponse.Action{
			{
				Name:   "set_document_private",
				Label:  "Set document private",
				OnType: "document",
				AccessGroups: table_info.DefaultGroupList{
					{Name: "users", Permission: &actionPermission},
				},
			},
		},
	}, tx)
	if err != nil {
		t.Fatalf("update action table: %v", err)
	}

	var relationCount int
	var relationPermission int64
	err = db.QueryRow(`
		select count(*), coalesce(max(r.permission), 0)
		from action_action_id_has_usergroup_usergroup_id r
		join action a on a.id = r.action_id
		where a.action_name = 'set_document_private' and r.usergroup_id = 1`).
		Scan(&relationCount, &relationPermission)
	if err != nil {
		t.Fatalf("scan action relation: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("expected one action relation, got %d", relationCount)
	}
	if relationPermission != int64(actionPermission) {
		t.Fatalf("expected action access group to override broad permission to %d, got %d", actionPermission, relationPermission)
	}
}
