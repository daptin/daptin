package statementbuilder

import (
	"github.com/doug-martin/goqu/v9"
)

import _ "github.com/doug-martin/goqu/v9/dialect/mysql"
import _ "github.com/doug-martin/goqu/v9/dialect/postgres"
import _ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
import _ "github.com/doug-martin/goqu/v9/dialect/sqlserver"

var Squirrel = goqu.Dialect("sqlite")

func InitialiseStatementBuilder(dbTypeName string) {

	Squirrel = goqu.Dialect(dbTypeName)

}
