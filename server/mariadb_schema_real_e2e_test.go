package server

import (
	"os"
	"sort"
	"testing"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func TestMariaDBBuiltInSchemaManifestRealE2E(t *testing.T) {
	dsn := os.Getenv("DAPTIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_MYSQL_DSN to an empty disposable MariaDB database")
	}

	database, err := sqlx.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MariaDB: %v", err)
	}
	defer database.Close()
	if err := database.Ping(); err != nil {
		t.Fatalf("ping MariaDB: %v", err)
	}

	var initialTableCount int
	if err := database.Get(&initialTableCount, `select count(*) from information_schema.tables where table_schema = database()`); err != nil {
		t.Fatalf("count initial MariaDB tables: %v", err)
	}
	if initialTableCount != 0 {
		t.Fatalf("MariaDB schema must be empty and disposable, found %d tables", initialTableCount)
	}
	statementbuilder.InitialiseStatementBuilder("mysql")
	auth.PrepareAuthQueries()
	t.Cleanup(func() {
		statementbuilder.InitialiseStatementBuilder("sqlite3")
		auth.PrepareAuthQueries()
	})

	expectedConfig, loadErrors := LoadConfigFiles()
	if len(loadErrors) > 0 {
		t.Fatalf("load built-in schema: %v", loadErrors)
	}
	resource.CheckRelations(&expectedConfig)
	resource.CheckAuditTables(&expectedConfig)
	resource.CheckTranslationTables(&expectedConfig)
	expectedTables := tableNames(expectedConfig)

	actualConfig, loadErrors := LoadConfigFiles()
	if len(loadErrors) > 0 {
		t.Fatalf("reload built-in schema: %v", loadErrors)
	}
	InitialiseServerResources(&actualConfig, database)
	assertMariaDBTableManifest(t, database, expectedTables)
	assertMariaDBDocumentSchema(t, database)

	restartConfig, loadErrors := LoadConfigFiles()
	if len(loadErrors) > 0 {
		t.Fatalf("reload built-in schema for restart: %v", loadErrors)
	}
	InitialiseServerResources(&restartConfig, database)
	assertMariaDBTableManifest(t, database, expectedTables)
	assertMariaDBDocumentSchema(t, database)
}

func assertMariaDBDocumentSchema(t *testing.T, database *sqlx.DB) {
	t.Helper()

	lengths := make(map[string]int64)
	rows, err := database.Queryx(`select column_name, character_maximum_length
		from information_schema.columns
		where table_schema = database() and table_name = 'document'
		and column_name in ('document_name', 'document_path', 'document_extension', 'mime_type')`)
	if err != nil {
		t.Fatalf("read MariaDB document columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var length int64
		if err := rows.Scan(&name, &length); err != nil {
			t.Fatalf("scan MariaDB document column: %v", err)
		}
		lengths[name] = length
	}
	wantLengths := map[string]int64{
		"document_name": 768, "document_path": 2048, "document_extension": 100, "mime_type": 255,
	}
	for name, want := range wantLengths {
		if lengths[name] != want {
			t.Errorf("MariaDB document.%s length = %d, want %d", name, lengths[name], want)
		}
	}

	var indexedColumns []string
	if err := database.Select(&indexedColumns, `select distinct column_name
		from information_schema.statistics
		where table_schema = database() and table_name = 'document'`); err != nil {
		t.Fatalf("read MariaDB document indexes: %v", err)
	}
	indexed := make(map[string]bool, len(indexedColumns))
	for _, name := range indexedColumns {
		indexed[name] = true
	}
	for _, name := range []string{"document_name", "document_extension", "mime_type"} {
		if !indexed[name] {
			t.Errorf("MariaDB document.%s index is missing", name)
		}
	}
	if indexed["document_path"] {
		t.Error("MariaDB document.document_path must not have an oversized full-column index")
	}
}

func tableNames(config resource.CmsConfig) []string {
	names := make([]string, 0, len(config.Tables))
	seen := make(map[string]bool, len(config.Tables))
	for _, table := range config.Tables {
		if table.TableName == "" || seen[table.TableName] {
			continue
		}
		seen[table.TableName] = true
		names = append(names, table.TableName)
	}
	sort.Strings(names)
	return names
}

func assertMariaDBTableManifest(t *testing.T, database *sqlx.DB, expected []string) {
	t.Helper()

	var actual []string
	if err := database.Select(&actual, `select table_name from information_schema.tables where table_schema = database()`); err != nil {
		t.Fatalf("read MariaDB table manifest: %v", err)
	}
	sort.Strings(actual)
	if len(actual) != len(expected) {
		t.Fatalf("MariaDB table count = %d, want %d\nactual: %v\nexpected: %v", len(actual), len(expected), actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("MariaDB table manifest differs at %d: got %q, want %q\nactual: %v\nexpected: %v",
				index, actual[index], expected[index], actual, expected)
		}
	}
}
