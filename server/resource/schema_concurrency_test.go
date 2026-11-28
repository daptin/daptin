package resource

import (
	"database/sql"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	"strings"
	"sync"
	"testing"
	"time"
)

type schemaBarrierDB struct {
	*sqlx.DB
	ready  chan<- struct{}
	gate   <-chan struct{}
	create bool
}

func (db schemaBarrierDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	lower := strings.ToLower(query)
	if (db.create && strings.HasPrefix(lower, "create table agprobe")) || (!db.create && strings.Contains(lower, "add column fnew")) {
		db.ready <- struct{}{}
		<-db.gate
	}
	return db.DB.Exec(query, args...)
}
func TestConcurrentSchemaChecksRetainModels(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(fmt.Sprint("create=", create), func(t *testing.T) {
			db, err := sqlx.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			base := table_info.TableInfo{TableName: "agprobe", Columns: []api2go.ColumnInfo{{Name: "name", ColumnName: "name", ColumnType: "label", DataType: "varchar(100)", IsNullable: true}}}
			if !create {
				if err := CheckTable(&base, db); err != nil {
					t.Fatal(err)
				}
			}
			ready := make(chan struct{}, 3)
			gate := make(chan struct{})
			closed := false
			defer func() {
				if !closed {
					close(gate)
				}
			}()
			wrapped := schemaBarrierDB{DB: db, ready: ready, gate: gate, create: create}
			retained := make([]int, 3)
			var workers sync.WaitGroup
			for i := 0; i < 3; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					config := CmsConfig{Tables: []table_info.TableInfo{{TableName: "agprobe", Columns: []api2go.ColumnInfo{{Name: "name", ColumnName: "name", ColumnType: "label", DataType: "varchar(100)", IsNullable: true}, {Name: "fnew", ColumnName: "fnew", ColumnType: "label", DataType: "varchar(100)", IsNullable: true}}}}}
					CheckAllTableStatus(&config, wrapped)
					retained[i] = len(config.Tables)
				}()
			}
			for i := 0; i < 3; i++ {
				select {
				case <-ready:
				case <-time.After(5 * time.Second):
					t.Fatal("schema check did not reach DDL barrier")
				}
			}
			close(gate)
			closed = true
			workers.Wait()
			kept := 0
			for _, n := range retained {
				kept += n
			}
			t.Logf("models retained after concurrent column additions: %v", retained)
			if kept != 3 {
				t.Fatalf("concurrent schema application dropped models: %v", retained)
			}
		})
	}
}
