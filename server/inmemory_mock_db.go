//go:build test
// +build test

package server

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strings"
)

func NewInMemoryTestDatabase(db *sqlx.DB) *InMemoryTestDatabase {
	return &InMemoryTestDatabase{
		db: db,
	}
}

func (imtd *InMemoryTestDatabase) GetQueries() []string {
	return imtd.queries
}

func (imtd *InMemoryTestDatabase) HasExecuted(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))

	for _, qu := range imtd.queries {
		q := strings.ToLower(qu)
		if BeginsWithCheck(q, query) {
			return true
		}
	}

	log.Printf("%v", strings.Join(imtd.queries, "\n"))

	return false
}

func (imtd *InMemoryTestDatabase) HasExecutedAll(queries ...string) bool {

	executedAll := true

	for _, query := range queries {
		found := false
		query = strings.TrimSpace(query)

		for _, qu := range imtd.queries {
			if strings.TrimSpace(qu) == query {
				found = true
				break
			}
		}

		if !found {
			log.Printf("%v", strings.Join(imtd.queries, "\n"))
			executedAll = false
			break
		}

	}

	return executedAll
}

type InMemoryTestDatabase struct {
	db      *sqlx.DB
	rowx    *sqlx.Row
	row     *sql.Row
	stmt    *sqlx.Stmt
	tx      *sqlx.Tx
	result  sql.Result
	queries []string
}

func (imtd *InMemoryTestDatabase) DriverName() string {
	return "sqlite3"
}

func (imtd *InMemoryTestDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {

	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	res, err := imtd.db.Exec(query, args...)
	return &InMemoryTestDatabase{
		result: res,
	}, err
}
func (imtd *InMemoryTestDatabase) LastInsertId() (int64, error) {
	return imtd.result.LastInsertId()
}
func (imtd *InMemoryTestDatabase) RowsAffected() (int64, error) {
	return imtd.result.RowsAffected()
}

func (imtd *InMemoryTestDatabase) Prepare(query string) (*sql.Stmt, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Prepare(query)

}

func (imtd *InMemoryTestDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Query(query, args...)
}
func (imtd *InMemoryTestDatabase) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)

	return imtd.db.Queryx(query, args...)

}
func (imtd *InMemoryTestDatabase) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.QueryRowx(query, args...)
}

func (imtd *InMemoryTestDatabase) Rebind(query string) string {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Rebind(query)
}
func (imtd *InMemoryTestDatabase) BindNamed(query string, args interface{}) (string, []interface{}, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.BindNamed(query, args)

}

func (imtd *InMemoryTestDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Select(dest, query, args...)

}

func (imtd *InMemoryTestDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Get(dest, query, args...)

}

func (imtd *InMemoryTestDatabase) MustBegin() *sqlx.Tx {
	return imtd.db.MustBegin()

}

func (imtd *InMemoryTestDatabase) Preparex(query string) (*sqlx.Stmt, error) {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.Preparex(query)

}

func (imtd *InMemoryTestDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	if imtd.queries == nil {
		imtd.queries = make([]string, 0)
	}

	imtd.queries = append(imtd.queries, query)
	return imtd.db.QueryRow(query, args...)
}

func (imtd *InMemoryTestDatabase) Beginx() (*sqlx.Tx, error) {
	return imtd.db.Beginx()
}
func (database *InMemoryTestDatabase) ResetQueries() {
	database.queries = make([]string, 0)
}
