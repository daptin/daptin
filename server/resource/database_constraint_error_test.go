package resource

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

func TestNormalizeDatabaseConstraintError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		title   string
		private string
	}{
		{name: "sqlite unique", err: sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique}, status: http.StatusConflict, title: uniqueConstraintMessage},
		{name: "sqlite primary key", err: sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintPrimaryKey}, status: http.StatusConflict, title: uniqueConstraintMessage},
		{name: "sqlite not null", err: sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintNotNull}, status: http.StatusUnprocessableEntity, title: requiredValueMessage},
		{name: "postgres unique", err: &pq.Error{Code: postgresUniqueViolation, Message: "private postgres detail"}, status: http.StatusConflict, title: uniqueConstraintMessage, private: "private postgres detail"},
		{name: "postgres not null", err: &pq.Error{Code: postgresNotNullViolation, Message: "private postgres detail"}, status: http.StatusUnprocessableEntity, title: requiredValueMessage, private: "private postgres detail"},
		{name: "mysql unique", err: &mysql.MySQLError{Number: mysqlDuplicateEntry, Message: "private mysql detail"}, status: http.StatusConflict, title: uniqueConstraintMessage, private: "private mysql detail"},
		{name: "mysql null", err: &mysql.MySQLError{Number: mysqlColumnCannotBeNull, Message: "private mysql detail"}, status: http.StatusUnprocessableEntity, title: requiredValueMessage, private: "private mysql detail"},
		{name: "mysql missing default", err: &mysql.MySQLError{Number: mysqlNoDefaultForField, Message: "private mysql detail"}, status: http.StatusUnprocessableEntity, title: requiredValueMessage, private: "private mysql detail"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapped := normalizeDatabaseConstraintError(fmt.Errorf("execute write: %w", test.err))
			var httpErr api2go.HTTPError
			if !errors.As(mapped, &httpErr) {
				t.Fatalf("mapped error = %T %v, want api2go.HTTPError", mapped, mapped)
			}
			if got := httpErr.Status(); got != test.status {
				t.Fatalf("status = %d, want %d", got, test.status)
			}
			if len(httpErr.Errors) != 1 || httpErr.Errors[0].Title != test.title {
				t.Fatalf("public errors = %#v, want title %q", httpErr.Errors, test.title)
			}
			if test.private != "" && strings.Contains(mapped.Error(), test.private) {
				t.Fatalf("mapped error leaked database detail: %v", mapped)
			}
		})
	}
}

func TestNormalizeDatabaseConstraintErrorPreservesUnknownError(t *testing.T) {
	original := errors.New("database unavailable")
	if got := normalizeDatabaseConstraintError(original); got != original {
		t.Fatalf("unknown error changed from %v to %v", original, got)
	}
}
