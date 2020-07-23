package statementbuilder

import "github.com/Masterminds/squirrel"

var Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

func InitialiseStatementBuilder(dbTypeName string) {

	if dbTypeName == "postgres" {
		Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	} else {
		Squirrel = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	}
}
