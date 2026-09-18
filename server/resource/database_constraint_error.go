package resource

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/artpar/api2go/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

const (
	uniqueConstraintMessage  = "unique constraint violated"
	requiredValueMessage     = "required value missing"
	postgresUniqueViolation  = pq.ErrorCode("23505")
	postgresNotNullViolation = pq.ErrorCode("23502")
	mysqlDuplicateEntry      = uint16(1062)
	mysqlColumnCannotBeNull  = uint16(1048)
	mysqlNoDefaultForField   = uint16(1364)
)

// normalizeDatabaseConstraintError keeps database-specific diagnostics in the
// wrapped server-side error while exposing a stable client-facing contract.
func normalizeDatabaseConstraintError(err error) error {
	if err == nil {
		return nil
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.ExtendedCode {
		case sqlite3.ErrConstraintUnique, sqlite3.ErrConstraintPrimaryKey:
			return newDatabaseConstraintHTTPError(err, uniqueConstraintMessage, http.StatusConflict)
		case sqlite3.ErrConstraintNotNull:
			return newDatabaseConstraintHTTPError(err, requiredValueMessage, http.StatusUnprocessableEntity)
		}
	}

	var postgresErr *pq.Error
	if errors.As(err, &postgresErr) {
		switch postgresErr.Code {
		case postgresUniqueViolation:
			return newDatabaseConstraintHTTPError(err, uniqueConstraintMessage, http.StatusConflict)
		case postgresNotNullViolation:
			return newDatabaseConstraintHTTPError(err, requiredValueMessage, http.StatusUnprocessableEntity)
		}
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case mysqlDuplicateEntry:
			return newDatabaseConstraintHTTPError(err, uniqueConstraintMessage, http.StatusConflict)
		case mysqlColumnCannotBeNull, mysqlNoDefaultForField:
			return newDatabaseConstraintHTTPError(err, requiredValueMessage, http.StatusUnprocessableEntity)
		}
	}

	return err
}

func newDatabaseConstraintHTTPError(err error, message string, status int) api2go.HTTPError {
	log.Errorf("Database constraint violation: %v", err)
	httpErr := api2go.NewHTTPError(nil, message, status)
	httpErr.Errors = []api2go.Error{{
		Status: strconv.Itoa(status),
		Title:  message,
	}}
	return httpErr
}
