package resource

import (
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
)

// RemoveObsoleteSystemActions removes system-owned actions whose implementation
// no longer exists. This prevents upgraded databases from retaining dead APIs.
func RemoveObsoleteSystemActions(db database.DatabaseConnection) error {
	query, args, err := statementbuilder.Squirrel.Delete("action").
		Prepared(true).
		Where(goqu.Ex{"action_name": "restart_daptin"}).
		ToSQL()
	if err != nil {
		return err
	}
	_, err = db.Exec(query, args...)
	return err
}
