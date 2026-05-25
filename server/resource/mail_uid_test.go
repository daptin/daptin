package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestAllocateMailBoxUidAdvancesPersistedNextUid(t *testing.T) {
	db := newMailUidTestDB(t)
	crud := &DbResource{db: db}

	tx := db.MustBegin()
	uid, err := crud.AllocateMailBoxUid(1, tx)
	if err != nil {
		t.Fatalf("AllocateMailBoxUid failed: %v", err)
	}
	if uid != 1 {
		t.Fatalf("uid = %d, want 1", uid)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	nextuid, err := mailUidTestNextUid(db, 1)
	if err != nil {
		t.Fatalf("select nextuid: %v", err)
	}
	if nextuid != 2 {
		t.Fatalf("nextuid = %d, want 2", nextuid)
	}
}

func TestAllocateMailBoxUidUsesExistingMessageFloor(t *testing.T) {
	db := newMailUidTestDB(t)
	crud := &DbResource{db: db}

	if err := mailUidTestInsert(db, "mail", []interface{}{"id", "mail_box_id", "uid"}, []interface{}{5, 1, 0}); err != nil {
		t.Fatalf("insert legacy mail: %v", err)
	}

	tx := db.MustBegin()
	uid, err := crud.AllocateMailBoxUid(1, tx)
	if err != nil {
		t.Fatalf("AllocateMailBoxUid failed: %v", err)
	}
	if uid != 6 {
		t.Fatalf("uid = %d, want 6", uid)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	nextuid, err := mailUidTestNextUid(db, 1)
	if err != nil {
		t.Fatalf("select nextuid: %v", err)
	}
	if nextuid != 7 {
		t.Fatalf("nextuid = %d, want 7", nextuid)
	}
}

func newMailUidTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []table_info.TableInfo{
		{
			TableName: "mail_box",
			Columns: []api2go.ColumnInfo{
				{Name: "id", ColumnName: "id", DataType: "INTEGER", IsPrimaryKey: true, IsAutoIncrement: true, ExcludeFromApi: true, ColumnType: "id"},
				{Name: "nextuid", ColumnName: "nextuid", DataType: "int(11)", ColumnType: "value", DefaultValue: "1"},
			},
		},
		{
			TableName: "mail",
			Columns: []api2go.ColumnInfo{
				{Name: "id", ColumnName: "id", DataType: "INTEGER", IsPrimaryKey: true, IsAutoIncrement: true, ExcludeFromApi: true, ColumnType: "id"},
				{Name: "mail_box_id", ColumnName: "mail_box_id", DataType: "int(11)", ColumnType: "value"},
				{Name: "uid", ColumnName: "uid", DataType: "int(11)", ColumnType: "value", DefaultValue: "0"},
			},
		},
	} {
		if err := CreateTable(&table, db); err != nil {
			t.Fatalf("create table %s: %v", table.TableName, err)
		}
	}

	if err := mailUidTestInsert(db, "mail_box", []interface{}{"id", "nextuid"}, []interface{}{1, 1}); err != nil {
		t.Fatalf("insert mailbox: %v", err)
	}

	return db
}

func mailUidTestInsert(db *sqlx.DB, tableName string, cols []interface{}, vals []interface{}) error {
	query, args, err := statementbuilder.Squirrel.Insert(tableName).Prepared(true).Cols(cols...).Vals(vals).ToSQL()
	if err != nil {
		return err
	}
	_, err = db.Exec(query, args...)
	return err
}

func mailUidTestNextUid(db *sqlx.DB, mailBoxId int64) (int64, error) {
	query, args, err := statementbuilder.Squirrel.Select("nextuid").Prepared(true).From("mail_box").Where(goqu.Ex{"id": mailBoxId}).ToSQL()
	if err != nil {
		return 0, err
	}
	var nextuid int64
	err = db.QueryRowx(query, args...).Scan(&nextuid)
	return nextuid, err
}
