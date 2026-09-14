package resource

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/task"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

func TestTaskSchedulerRegistersMeteringReservationRecovery(t *testing.T) {
	_, cruds, _ := newCanonicalMeteringDatabase(t)
	scheduler, err := NewTaskScheduler(cruds)
	if err != nil {
		t.Fatal(err)
	}
	entries := scheduler.cronService.Entries()
	if len(entries) != 1 {
		t.Fatalf("internal scheduler jobs = %d, want 1", len(entries))
	}
	now := time.Now()
	delay := entries[0].Schedule.Next(now).Sub(now)
	if delay <= 0 || delay > 10*time.Second {
		t.Fatalf("metering recovery interval = %s, want 0s < interval <= 10s", delay)
	}
}

func TestTaskRelationshipDatabaseMatrix(t *testing.T) {
	t.Cleanup(func() { statementbuilder.InitialiseStatementBuilder("sqlite3") })
	tests := []struct {
		name        string
		dialect     string
		driver      string
		dsn         string
		referenceDB string
		temporary   string
		identity    string
	}{
		{
			name: "sqlite", dialect: "sqlite3", driver: "sqlite3",
			dsn:         fmt.Sprintf("file:task-relationship-%s?mode=memory&cache=shared", uuid.NewString()),
			referenceDB: "blob", temporary: "", identity: "integer primary key autoincrement",
		},
		{
			name: "postgres", dialect: "postgres", driver: "postgres",
			dsn: os.Getenv("DAPTIN_TEST_POSTGRES_DSN"), referenceDB: "bytea", temporary: "temporary ", identity: "bigserial primary key",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database to run the PostgreSQL contract")
			}
			database, err := sqlx.Open(test.driver, test.dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.SetMaxOpenConns(1)
			if err := database.Ping(); err != nil {
				t.Fatal(err)
			}
			statementbuilder.InitialiseStatementBuilder(test.dialect)
			for _, ddl := range []string{
				fmt.Sprintf("create %stable user_account (id bigint primary key, reference_id %s not null, email text, auth_version bigint not null default 1)", test.temporary, test.referenceDB),
				fmt.Sprintf("create %stable usergroup (id bigint primary key, name text)", test.temporary),
				fmt.Sprintf("create %stable task (id %s, reference_id %s not null, name text, action_name text, entity_name text, job_type text, schedule text, active boolean, attributes text, as_user_id bigint, user_account_id bigint, permission bigint, created_at timestamp)", test.temporary, test.identity, test.referenceDB),
			} {
				if _, err := database.Exec(ddl); err != nil {
					t.Fatal(err)
				}
			}
			userReferenceId := daptinid.DaptinReferenceId(uuid.New())
			taskReferenceId := daptinid.DaptinReferenceId(uuid.New())
			if _, err := database.Exec(database.Rebind("insert into user_account (id, reference_id, email) values (?, ?, ?)"), int64(42), userReferenceId[:], "actor@example.test"); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec("insert into usergroup (id, name) values (2, 'administrators')"); err != nil {
				t.Fatal(err)
			}
			insertTask := database.Rebind("insert into task (id, reference_id, name, action_name, entity_name, job_type, schedule, active, attributes, as_user_id) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			if _, err := database.Exec(insertTask, int64(100), taskReferenceId[:], "active-task", "run", "world", "test", "@every 1h", true, "{}", int64(42)); err != nil {
				t.Fatal(err)
			}
			inactiveTaskReferenceId := daptinid.DaptinReferenceId(uuid.New())
			if _, err := database.Exec(insertTask, int64(101), inactiveTaskReferenceId[:], "inactive-task", "run", "world", "test", "@every 1h", false, "{}", int64(42)); err != nil {
				t.Fatal(err)
			}

			crud := &DbResource{connection: database}
			tasks, err := crud.GetAllTasks()
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 1 {
				t.Fatalf("loaded %d tasks, want one active task", len(tasks))
			}
			if tasks[0].ReferenceId != taskReferenceId {
				t.Fatalf("task reference = %s, want %s", tasks[0].ReferenceId, taskReferenceId)
			}
			if tasks[0].AsUserReferenceId != userReferenceId {
				t.Fatalf("task user reference = %s, want %s", tasks[0].AsUserReferenceId, userReferenceId)
			}
			if tasks[0].AsUserEmail != "" {
				t.Fatalf("persisted relation leaked into email field: %q", tasks[0].AsUserEmail)
			}

			tx, err := database.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			config := &CmsConfig{Tasks: []task.Task{
				{
					Name: "active-task", ActionName: "run", EntityName: "world", JobType: "updated",
					Schedule: "@every 2h", Active: true, Attributes: map[string]interface{}{},
					AsUserEmail: "actor@example.test",
				},
				{
					Name: "configured-task", ActionName: "run", EntityName: "world", JobType: "new",
					Schedule: "@every 3h", Active: true, Attributes: map[string]interface{}{},
					AsUserEmail: "actor@example.test",
				},
			}}
			if err := UpdateTasksData(config, tx); err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			var activeSchedule, inactiveSchedule string
			var activeUserId int64
			if err := database.QueryRowx(database.Rebind("select schedule, as_user_id from task where name = ?"), "active-task").Scan(&activeSchedule, &activeUserId); err != nil {
				t.Fatal(err)
			}
			if err := database.QueryRowx(database.Rebind("select schedule from task where name = ?"), "inactive-task").Scan(&inactiveSchedule); err != nil {
				t.Fatal(err)
			}
			if activeSchedule != "@every 2h" || activeUserId != 42 {
				t.Fatalf("configured task = schedule %q user %d, want @every 2h as user 42", activeSchedule, activeUserId)
			}
			if inactiveSchedule != "@every 1h" {
				t.Fatalf("unrelated task schedule = %q, want @every 1h", inactiveSchedule)
			}
			var configuredUserId, configuredOwnerId, configuredPermission int64
			if err := database.QueryRowx(database.Rebind("select as_user_id, user_account_id, permission from task where name = ?"), "configured-task").
				Scan(&configuredUserId, &configuredOwnerId, &configuredPermission); err != nil {
				t.Fatal(err)
			}
			if configuredUserId != 42 || configuredOwnerId != 42 || configuredPermission != int64(auth.DEFAULT_PERMISSION) {
				t.Fatalf("configured task context = actor %d owner %d permission %d", configuredUserId, configuredOwnerId, configuredPermission)
			}

			tx, err = database.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			invalidConfig := &CmsConfig{Tasks: []task.Task{
				{
					Name: "active-task", ActionName: "run", EntityName: "world", JobType: "invalid-update",
					Schedule: "@every 9h", Active: true, Attributes: map[string]interface{}{},
					AsUserEmail: "actor@example.test",
				},
				{
					Name: "invalid-task", ActionName: "run", EntityName: "world", JobType: "invalid",
					Schedule: "@every 1h", Active: true, Attributes: map[string]interface{}{},
					AsUserEmail: "missing@example.test",
				},
			}}
			if err := UpdateTasksData(invalidConfig, tx); err == nil {
				_ = tx.Rollback()
				t.Fatal("expected an unknown configured execution user to fail")
			}
			if err := tx.QueryRowx(tx.Rebind("select schedule from task where name = ?"), "active-task").Scan(&activeSchedule); err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			_ = tx.Rollback()
			if activeSchedule != "@every 2h" {
				t.Fatalf("task was mutated before identity prevalidation completed: %q", activeSchedule)
			}
		})
	}
}

func TestTaskSchedulerRejectsMissingExecutionUser(t *testing.T) {
	scheduler := &DefaultTaskScheduler{
		cruds:       map[string]*DbResource{"world": {}, "task": {}},
		cronService: cron.New(),
	}
	err := scheduler.AddTask(task.Task{Name: "missing-user", EntityName: "world", Schedule: "@every 1h"})
	if err == nil {
		t.Fatal("expected a task without as_user_id to be rejected")
	}
	if len(scheduler.cronService.Entries()) != 0 {
		t.Fatal("task without as_user_id was registered")
	}
}

func TestTaskExecutionUserUsesPersistedReferenceAndCurrentGroups(t *testing.T) {
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database, err := sqlx.Open("sqlite3", fmt.Sprintf("file:task-user-%s?mode=memory&cache=shared", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)

	userResource, userReference := testUserAccountResource(t, database)
	userResource.Cruds[USER_ACCOUNT_TABLE_NAME] = userResource
	if _, err := database.Exec(`alter table user_account_user_account_id_has_usergroup_usergroup_id add column reference_id blob`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`alter table user_account_user_account_id_has_usergroup_usergroup_id add column permission integer`); err != nil {
		t.Fatal(err)
	}
	groupReference := daptinid.DaptinReferenceId(uuid.New())
	relationReference := daptinid.DaptinReferenceId(uuid.New())
	if _, err := database.Exec(`insert into usergroup (id, name, reference_id) values (?, ?, ?)`,
		int64(2), "operators", groupReference[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`insert into user_account_user_account_id_has_usergroup_usergroup_id
		(id, user_account_id, usergroup_id, created_at, reference_id, permission) values (?, ?, ?, ?, ?, ?)`,
		int64(1), int64(1), int64(2), time.Now(), relationReference[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatal(err)
	}

	taskResource := &DbResource{
		connection: database,
		Cruds:      map[string]*DbResource{USER_ACCOUNT_TABLE_NAME: userResource},
	}
	instance := &ActiveTaskInstance{
		Task:       task.Task{AsUserReferenceId: daptinid.DaptinReferenceId(userReference)},
		DbResource: taskResource,
	}
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	sessionUser, err := instance.resolveSessionUser(tx)
	_ = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if sessionUser.UserId != 1 || sessionUser.UserReferenceId != daptinid.DaptinReferenceId(userReference) {
		t.Fatalf("resolved wrong user: %#v", sessionUser)
	}
	if len(sessionUser.Groups) != 1 || sessionUser.Groups[0].GroupReferenceId != groupReference {
		t.Fatalf("resolved groups = %#v, want current operators group", sessionUser.Groups)
	}

	if _, err := database.Exec(`delete from user_account where id = ?`, int64(1)); err != nil {
		t.Fatal(err)
	}
	tx, err = database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_, err = instance.resolveSessionUser(tx)
	_ = tx.Rollback()
	if err == nil {
		t.Fatal("deleted execution user was accepted")
	}
}
