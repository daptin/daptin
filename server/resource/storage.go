package resource

import (
	"database/sql"
	"github.com/Masterminds/squirrel"
	uuid "github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
)

func CreateDefaultLocalStorage(db database.DatabaseConnection, localStoragePath string) error {

	query, vars, err := statementbuilder.Squirrel.Select("reference_id").From("cloud_store").Where(squirrel.Eq{
		"name": "localstore",
	}).ToSql()

	if err != nil {
		return err
	}

	res := db.QueryRow(query, vars...)
	var storageReferenceId string
	err = res.Scan(&storageReferenceId)
	if err != nil {
		if err == sql.ErrNoRows {

			adminUserId, adminGroupId := GetAdminUserIdAndUserGroupId(db)
			newUuid, _ := uuid.NewV4()
			query, vars, err = statementbuilder.Squirrel.Insert("cloud_store").
				Columns("reference_id", "name", "store_type", "store_provider", "root_path", "store_parameters", "user_account_id", "permission").
				Values(newUuid.String(), "localstore", "local", "local", localStoragePath, "", adminUserId, auth.DEFAULT_PERMISSION).ToSql()

			if err != nil {
				return err
			}

			_, err = db.Exec(query, vars...)
			if err != nil {
				return err
			}

			query, vars, err = statementbuilder.Squirrel.Select("id").From("cloud_store").Where(squirrel.Eq{
				"reference_id": newUuid.String(),
			}).ToSql()
			if err != nil {
				return err
			}

			row := db.QueryRowx(query, vars...)
			if row.Err() != nil {
				return row.Err()
			}
			var id int64
			err = row.Scan(&id)
			if err != nil {
				return err
			}

			groupRefId, _ := uuid.NewV4()
			query, vars, err = statementbuilder.Squirrel.Insert("cloud_store_cloud_store_id_has_usergroup_usergroup_id").
				Columns("cloud_store_id", "usergroup_id", "reference_id", "permission").
				Values(id, adminGroupId, groupRefId.String(), auth.DEFAULT_PERMISSION).ToSql()

			if err != nil {
				return err
			}

			_, err = db.Exec(query, vars...)

			return err

		} else {
			return err
		}
	} else {
		return nil
	}

}
