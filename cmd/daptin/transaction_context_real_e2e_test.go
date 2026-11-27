package main

import (
	"context"
	"github.com/daptin/daptin/server"
	"os"
	"testing"
	"time"
)

func TestTransactionContextCancellationRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" || os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") == "" {
		t.Skip("requires private PostgreSQL E2E fixture")
	}
	options := mailProtocolDatabase(t)
	db, err := server.GetDbConnection("postgres", options.connectionString)
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
			stmt, prepareErr := tx.Preparex("SELECT pg_sleep(10)")
			if prepareErr != nil {
				cancel()
				t.Fatal(prepareErr)
			}
			_, err = stmt.Exec()
			_ = stmt.Close()
		} else {
			_, err = tx.Exec("SELECT pg_sleep(10)")
		}
		cancel()
		_ = tx.Rollback()
		if err == nil || time.Since(started) > 2*time.Second {
			t.Fatalf("PostgreSQL query escaped deadline: prepared=%v elapsed=%s err=%v", prepared, time.Since(started), err)
		}
	}
}
