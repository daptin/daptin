package statementbuilder

import "gopkg.in/Masterminds/squirrel.v1"

var Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

func InitialiseStatementBuilder(dbTypeName string) {

	if dbTypeName == "postgres" {
		Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	} else {
		Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	}
}
