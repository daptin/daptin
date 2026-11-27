package database

import (
	"context"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"testing"
	"time"
)

func TestTransactionContextCancelsExistingSQLHelpers(t *testing.T) {
	for _, operation := range []string{"query", "prepared query", "exec"} {
		t.Run(operation, func(t *testing.T) {
			db, err := Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			tx, err := db.BeginTxx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			query := `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1000000000) SELECT sum(x) FROM n`
			started := time.Now()
			switch operation {
			case "query":
				var value int64
				err = tx.QueryRowx(query).Scan(&value)
			case "prepared query":
				stmt, prepareErr := tx.Preparex(query)
				if prepareErr != nil {
					t.Fatal(prepareErr)
				}
				defer stmt.Close()
				var value int64
				err = stmt.QueryRowx().Scan(&value)
			case "exec":
				_, err = tx.Exec(query)
			}
			if err == nil || time.Since(started) > time.Second {
				t.Fatalf("query escaped transaction deadline: elapsed=%s err=%v", time.Since(started), err)
			}
			_ = tx.Rollback()
			var value int
			if err := db.Get(&value, "SELECT 42"); err != nil || value != 42 {
				t.Fatalf("cancelled context leaked into pooled connection: %d %v", value, err)
			}
		})
	}
}

func TestMySQLTransactionContextCancelsExistingSQLHelpers(t *testing.T) {
	dsn := os.Getenv("DAPTIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable MySQL")
	}
	db, err := Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, prepared := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		started := time.Now()
		if prepared {
			stmt, prepareErr := tx.Preparex("SELECT SLEEP(10)")
			if prepareErr != nil {
				cancel()
				t.Fatal(prepareErr)
			}
			_, err = stmt.Exec()
			_ = stmt.Close()
		} else {
			_, err = tx.Exec("SELECT SLEEP(10)")
		}
		cancel()
		_ = tx.Rollback()
		if err == nil || time.Since(started) > 2*time.Second {
			t.Fatalf("MySQL query escaped deadline: prepared=%v elapsed=%s err=%v", prepared, time.Since(started), err)
		}
	}
	var value int
	if err := db.Get(&value, "SELECT 42"); err != nil || value != 42 {
		t.Fatalf("pooled connection invalid after cancellation: %d %v", value, err)
	}
}
