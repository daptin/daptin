package resource

import (
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGeneratedRelationColumnsAreIndexed(t *testing.T) {
	config := CmsConfig{
		Tables: []table_info.TableInfo{
			{TableName: "document"},
			{TableName: "usergroup"},
		},
	}

	CheckRelations(&config)

	var ownerIndexed, documentGroupIndexed, usergroupIndexed bool
	for _, table := range config.Tables {
		switch table.TableName {
		case "document":
			for _, column := range table.Columns {
				if column.ColumnName == "user_account_id" {
					ownerIndexed = column.IsIndexed
				}
			}
		case "document_document_id_has_usergroup_usergroup_id":
			for _, column := range table.Columns {
				if column.ColumnName == "document_id" {
					documentGroupIndexed = column.IsIndexed
				}
				if column.ColumnName == "usergroup_id" {
					usergroupIndexed = column.IsIndexed
				}
			}
		}
	}

	if !ownerIndexed {
		t.Fatalf("expected generated owner relation column to be indexed")
	}
	if !documentGroupIndexed || !usergroupIndexed {
		t.Fatalf("expected generated usergroup join columns to be indexed, got document_id=%v usergroup_id=%v", documentGroupIndexed, usergroupIndexed)
	}
}

func TestStandardGeneratedTableNamesArePortable(t *testing.T) {
	const exchangeRunJoin = "exchange_run_exchange_run_id_has_usergroup_usergroup_id"
	config := CmsConfig{
		Tables:    standardTablesForTest(nil),
		Relations: append([]api2go.TableRelation(nil), StandardRelations...),
	}
	CheckRelations(&config)

	foundExchangeRunJoin := false
	for _, table := range config.Tables {
		if len([]byte(table.TableName)) > 63 {
			t.Errorf("generated table %q is %d bytes", table.TableName, len([]byte(table.TableName)))
		}
		if table.TableName == exchangeRunJoin {
			foundExchangeRunJoin = true
		}
	}
	if !foundExchangeRunJoin {
		t.Fatalf("generated exchange run usergroup table %q is missing", exchangeRunJoin)
	}
}

func TestDocumentSchemaIsPortableToMySQL(t *testing.T) {
	document := tableFromConfig(t, &CmsConfig{Tables: StandardTables}, "document")
	want := map[string]struct {
		dataType  string
		isIndexed bool
	}{
		"document_name":      {dataType: "varchar(768)", isIndexed: true},
		"document_path":      {dataType: "varchar(2048)", isIndexed: false},
		"document_extension": {dataType: "varchar(100)", isIndexed: true},
		"mime_type":          {dataType: "varchar(255)", isIndexed: true},
	}

	for columnName, expected := range want {
		column, ok := document.GetColumnByName(columnName)
		if !ok {
			t.Fatalf("document.%s is missing", columnName)
		}
		if column.DataType != expected.dataType || column.IsIndexed != expected.isIndexed {
			t.Errorf("document.%s = type %q indexed %v, want type %q indexed %v",
				columnName, column.DataType, column.IsIndexed, expected.dataType, expected.isIndexed)
		}
	}

	query := MakeCreateTableQuery(document, "mysql")
	for _, invalid := range []string{"varchar(99999)"} {
		if strings.Contains(query, invalid) {
			t.Errorf("MySQL document schema contains invalid definition %q:\n%s", invalid, query)
		}
	}
	for _, expected := range []string{
		"document_name varchar(768) not null",
		"document_path varchar(2048) not null",
		"document_extension varchar(100) not null",
		"mime_type varchar(255) not null",
	} {
		if !strings.Contains(query, expected) {
			t.Errorf("MySQL document schema is missing %q:\n%s", expected, query)
		}
	}
}

func TestCreateIndexesCreatesConfiguredColumnIndexes(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`create table document (
		id integer primary key,
		user_account_id integer,
		title text
	)`); err != nil {
		t.Fatalf("create document table: %v", err)
	}

	config := CmsConfig{
		Tables: []table_info.TableInfo{
			{
				TableName:        "document",
				CompositeIndexes: [][]string{{"user_account_id", "id"}},
				Columns: []api2go.ColumnInfo{
					{
						ColumnName:   "user_account_id",
						Name:         "user_account",
						ColumnType:   "alias",
						DataType:     "int(11)",
						IsForeignKey: true,
						IsIndexed:    true,
						ForeignKeyData: api2go.ForeignKeyData{
							DataSource: "self",
							Namespace:  "user_account",
							KeyName:    "id",
						},
					},
					{
						ColumnName: "title",
						Name:       "title",
						ColumnType: "label",
						DataType:   "varchar(100)",
						IsIndexed:  false,
					},
				},
			},
		},
	}

	CreateIndexes(&config, db)

	assertSqliteIndexExists(t, db, columnIndexName("document", "user_account_id"))
	assertSqliteIndexColumns(t, db, compositeIndexName("document", []string{"user_account_id", "id"}), "user_account_id", "id")
	assertSqliteIndexMissing(t, db, "document", "title")
}

func TestCreateIndexesRejectsSingleColumnCompositeIndex(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`create table document (id integer primary key, title text)`); err != nil {
		t.Fatalf("create document table: %v", err)
	}
	config := CmsConfig{Tables: []table_info.TableInfo{{
		TableName:        "document",
		CompositeIndexes: [][]string{{"title"}},
	}}}

	CreateIndexes(&config, db)
	assertSqliteIndexMissing(t, db, "document", "title")
}

func TestCreateRelationsCreatesUsergroupAccessIndex(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	const joinTable = "document_document_id_has_usergroup_usergroup_id"
	if _, err := db.Exec(`create table document_document_id_has_usergroup_usergroup_id (
		id integer primary key,
		document_id integer,
		usergroup_id integer
	)`); err != nil {
		t.Fatalf("create join table: %v", err)
	}

	usergroupAccessIndex := usergroupAccessIndexName(joinTable, "usergroup_id", "document_id")
	if len(usergroupAccessIndex) > 60 {
		t.Fatalf("index name is too long: %s", usergroupAccessIndex)
	}

	config := CmsConfig{
		Tables: []table_info.TableInfo{
			{
				TableName: joinTable,
				Columns: []api2go.ColumnInfo{
					{
						ColumnName:   "document_id",
						Name:         "document_id",
						ColumnType:   "alias",
						DataType:     "int(11)",
						IsForeignKey: true,
						ForeignKeyData: api2go.ForeignKeyData{
							DataSource: "self",
							Namespace:  "document",
							KeyName:    "id",
						},
					},
					{
						ColumnName:   "usergroup_id",
						Name:         "usergroup_id",
						ColumnType:   "alias",
						DataType:     "int(11)",
						IsForeignKey: true,
						ForeignKeyData: api2go.ForeignKeyData{
							DataSource: "self",
							Namespace:  "usergroup",
							KeyName:    "id",
						},
					},
				},
			},
		},
	}

	CreateRelations(&config, db)

	assertSqliteIndexMissing(t, db, joinTable, "document_id")
	assertSqliteIndexMissing(t, db, joinTable, "usergroup_id")
	assertSqliteIndexColumns(t, db, usergroupAccessIndex, "usergroup_id", "document_id")
}

func columnIndexName(tableName, columnName string) string {
	return "i" + GetMD5HashString("index_"+tableName+"_"+columnName+"_index")
}

func assertSqliteIndexExists(t *testing.T, db *sqlx.DB, indexName string) {
	t.Helper()

	var count int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'index' and name = ?`,
		indexName).Scan(&count); err != nil {
		t.Fatalf("count index %s: %v", indexName, err)
	}
	if count != 1 {
		t.Fatalf("expected index %s to be created, got %d", indexName, count)
	}
}

func assertSqliteIndexMissing(t *testing.T, db *sqlx.DB, tableName, columnName string) {
	t.Helper()

	var count int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'index' and tbl_name = ? and sql like ?`,
		tableName, "%("+columnName+")%").Scan(&count); err != nil {
		t.Fatalf("count indexes on %s.%s: %v", tableName, columnName, err)
	}
	if count != 0 {
		t.Fatalf("expected %s.%s to stay unindexed, got %d indexes", tableName, columnName, count)
	}
}

func assertSqliteIndexColumns(t *testing.T, db *sqlx.DB, indexName string, columnNames ...string) {
	t.Helper()

	rows, err := db.Query(`pragma index_info(` + indexName + `)`)
	if err != nil {
		t.Fatalf("read index columns for %s: %v", indexName, err)
	}
	defer rows.Close()

	actualColumns := make([]string, 0)
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			t.Fatalf("scan index column for %s: %v", indexName, err)
		}
		actualColumns = append(actualColumns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index columns for %s: %v", indexName, err)
	}

	if len(actualColumns) != len(columnNames) {
		t.Fatalf("expected index %s columns %v, got %v", indexName, columnNames, actualColumns)
	}
	for i := range columnNames {
		if actualColumns[i] != columnNames[i] {
			t.Fatalf("expected index %s columns %v, got %v", indexName, columnNames, actualColumns)
		}
	}
}

func TestMySQLTextDefaultUsesExpressionSyntax(t *testing.T) {
	column := api2go.ColumnInfo{
		ColumnName:   "document",
		DataType:     "text",
		DefaultValue: "'{}'",
	}
	if got, want := getColumnLine(&column, "mysql"), "document text not null default ('{}')"; got != want {
		t.Fatalf("MySQL text column = %q, want %q", got, want)
	}
	if got, want := getColumnLine(&column, "postgres"), "document text not null default '{}'"; got != want {
		t.Fatalf("PostgreSQL text column = %q, want %q", got, want)
	}
}
