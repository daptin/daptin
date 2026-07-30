package resource

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const postgresAuthorizedDocumentIDsQuery = `
select distinct(document.id)
from document
left join document_document_id_has_usergroup_usergroup_id access
  on document.id = access.document_id
where ((document.permission & 2) = 2)
   or ((access.permission & 32768) = 32768 and access.usergroup_id = $1)
   or (document.user_account_id = $2 and (document.permission & 256) = 256)
order by document.id
limit 100`

// BenchmarkPostgresAuthorizedListScaling exercises the two queries used by
// PaginatedFindAllWithoutFilters at small and accumulated private-row counts.
// Run with:
//
//	DAPTIN_TEST_POSTGRES_DSN='postgres://...' go test ./server/resource \
//	  -run '^$' -bench BenchmarkPostgresAuthorizedListScaling -benchmem
//
// The benchmark uses temporary tables and does not modify persistent schemas.
func BenchmarkPostgresAuthorizedListScaling(b *testing.B) {
	db := openPostgresIntegrationDatabase(b)
	defer db.Close()
	createPostgresAuthorizationBenchmarkTables(b, db)

	for _, inaccessibleRows := range []int{0, 100, 10_000} {
		inaccessibleRows := inaccessibleRows
		b.Run(fmt.Sprintf("rows_%d/sparse", inaccessibleRows+1), func(b *testing.B) {
			seedPostgresAuthorizationBenchmark(b, db, inaccessibleRows)
			benchmarkPostgresAuthorizedList(b, db, true)
		})
		b.Run(fmt.Sprintf("rows_%d/normal", inaccessibleRows+1), func(b *testing.B) {
			seedPostgresAuthorizationBenchmark(b, db, inaccessibleRows)
			benchmarkPostgresAuthorizedList(b, db, false)
		})
	}
}

func openPostgresIntegrationDatabase(tb testing.TB) *sqlx.DB {
	tb.Helper()
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		tb.Skip("set DAPTIN_TEST_POSTGRES_DSN to run PostgreSQL integration benchmarks")
	}
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		tb.Fatalf("open PostgreSQL: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		tb.Fatalf("ping PostgreSQL: %v", err)
	}
	return db
}

func createPostgresAuthorizationBenchmarkTables(tb testing.TB, db *sqlx.DB) {
	tb.Helper()
	statements := []string{
		`create temporary table document (
			id bigint primary key,
			reference_id uuid not null,
			document_name text not null,
			document_path text not null,
			document_extension text not null,
			permission bigint not null,
			user_account_id bigint not null,
			updated_at timestamptz not null default now()
		) on commit preserve rows`,
		`create temporary table document_document_id_has_usergroup_usergroup_id (
			id bigint generated always as identity primary key,
			document_id bigint not null,
			usergroup_id bigint not null,
			permission bigint not null
		) on commit preserve rows`,
		`create index document_owner_auth_idx on document (user_account_id)`,
		`create index document_group_auth_idx on document_document_id_has_usergroup_usergroup_id (usergroup_id, document_id)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			tb.Fatalf("create PostgreSQL benchmark fixture: %v", err)
		}
	}
}

func seedPostgresAuthorizationBenchmark(tb testing.TB, db *sqlx.DB, inaccessibleRows int) {
	tb.Helper()
	if _, err := db.Exec(`truncate document, document_document_id_has_usergroup_usergroup_id restart identity`); err != nil {
		tb.Fatalf("truncate PostgreSQL benchmark fixture: %v", err)
	}
	if _, err := db.Exec(`insert into document
		(id, reference_id, document_name, document_path, document_extension, permission, user_account_id)
		values (1, '00000000-0000-0000-0000-000000000001', 'owned', '/owned', 'txt', 256, 1)`); err != nil {
		tb.Fatalf("insert caller-owned document: %v", err)
	}
	if inaccessibleRows > 0 {
		if _, err := db.Exec(`insert into document
			(id, reference_id, document_name, document_path, document_extension, permission, user_account_id)
			select n, md5(n::text)::uuid, 'private-' || n, '/private/' || n, 'txt', 256, n
			from generate_series(2, $1) n`, inaccessibleRows+1); err != nil {
			tb.Fatalf("insert inaccessible documents: %v", err)
		}
	}
	// Model the generated per-row usergroup relation used by the real list
	// query. The caller belongs to group 1; accumulated rows belong to unrelated
	// groups and must not increase visible cardinality.
	if _, err := db.Exec(`insert into document_document_id_has_usergroup_usergroup_id
		(document_id, usergroup_id, permission)
		select id, case when id = 1 then 1 else id + 1 end, 32768 from document`); err != nil {
		tb.Fatalf("insert document usergroup access rows: %v", err)
	}
	if _, err := db.Exec(`analyze document; analyze document_document_id_has_usergroup_usergroup_id`); err != nil {
		tb.Fatalf("analyze PostgreSQL benchmark fixture: %v", err)
	}
}

func benchmarkPostgresAuthorizedList(b *testing.B, db *sqlx.DB, sparse bool) {
	projection := `reference_id, document_name, document_path, document_extension, permission, user_account_id, updated_at`
	if sparse {
		projection = `reference_id, document_name`
	}
	detailQuery := `select ` + projection + ` from document where id = any($1)`

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var ids []int64
		if err := db.SelectContext(ctx, &ids, postgresAuthorizedDocumentIDsQuery, int64(1), int64(1)); err != nil {
			b.Fatalf("list authorized document ids: %v", err)
		}
		if len(ids) != 1 || ids[0] != 1 {
			b.Fatalf("expected only caller-owned document 1, got %v", ids)
		}
		rows, err := db.QueryxContext(ctx, detailQuery, pqInt64Array(ids))
		if err != nil {
			b.Fatalf("load authorized documents: %v", err)
		}
		for rows.Next() {
			values := make(map[string]interface{})
			if err := rows.MapScan(values); err != nil {
				rows.Close()
				b.Fatalf("scan authorized document: %v", err)
			}
		}
		if err := rows.Close(); err != nil {
			b.Fatalf("close authorized document rows: %v", err)
		}
	}
}

// pqInt64Array uses PostgreSQL's text array input without coupling production
// code to a driver-specific helper.
func pqInt64Array(values []int64) string {
	if len(values) == 0 {
		return "{}"
	}
	result := "{"
	for i, value := range values {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", value)
	}
	return result + "}"
}
