package resource

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestConfigUpdatePreservesLongPreviousValue(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store, err := NewConfigStore(db)
	if err != nil {
		t.Fatalf("create config store: %v", err)
	}

	var declaredType string
	if err := db.Get(&declaredType, `SELECT type FROM pragma_table_info('_config') WHERE name = 'previousvalue'`); err != nil {
		t.Fatalf("inspect previousvalue type: %v", err)
	}
	if !strings.EqualFold(declaredType, "varchar(5000)") {
		t.Fatalf("previousvalue type = %q, want varchar(5000)", declaredType)
	}

	oldValue := `{"version":"1","allowed_origins":[],"allowed_methods":[],"allowed_headers":[],"exposed_headers":[],"allow_credentials":false,"max_age":0}`
	setConfigValue(t, db, store, "cors.config", oldValue)
	setConfigValue(t, db, store, "cors.config", oldValue)

	var previousValue string
	if err := db.Get(&previousValue, `SELECT previousvalue FROM _config WHERE name = 'cors.config'`); err != nil {
		t.Fatalf("read previous config value: %v", err)
	}
	if previousValue != oldValue {
		t.Fatalf("previous config value was not preserved: got %q, want %q", previousValue, oldValue)
	}
}

func TestConfigStoreWidensLegacyPostgresPreviousValue(t *testing.T) {
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to run the PostgreSQL schema migration test")
	}

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := fmt.Sprintf("config_migration_%d", time.Now().UnixNano())
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = db.Exec(`SET search_path TO public`)
		_, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	}()
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatalf("set test search path: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE _config (
		id serial primary key,
		name varchar(100) not null,
		configtype varchar(100) not null,
		configstate varchar(100) not null,
		configenv varchar(100) not null,
		value varchar(5000),
		valuetype varchar(100),
		previousvalue varchar(100),
		created_at timestamp not null default current_timestamp,
		updated_at timestamp
	)`); err != nil {
		t.Fatalf("create legacy config table: %v", err)
	}

	store, err := NewConfigStore(db)
	if err != nil {
		t.Fatalf("upgrade legacy config table: %v", err)
	}

	oldValue := strings.Repeat("x", 101)
	setConfigValue(t, db, store, "long.config", oldValue)
	setConfigValue(t, db, store, "long.config", "replacement")

	var previousValue string
	if err := db.Get(&previousValue, `SELECT previousvalue FROM _config WHERE name = 'long.config'`); err != nil {
		t.Fatalf("read migrated previous value: %v", err)
	}
	if previousValue != oldValue {
		t.Fatalf("migrated previous value length = %d, want %d", len(previousValue), len(oldValue))
	}
}

func setConfigValue(t *testing.T, db *sqlx.DB, store *ConfigStore, key, value string) {
	t.Helper()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin config transaction: %v", err)
	}
	if err := store.SetConfigValueFor(key, value, "backend", tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set config value: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit config value: %v", err)
	}
}
