package resource

import (
	"database/sql"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	uuid "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func CreateDefaultLocalStorage(transaction *sqlx.Tx, localStoragePath string) error {
	log.Tracef("CreateDefaultLocalStorage")
	query, vars, err := statementbuilder.Squirrel.Select("reference_id").Prepared(true).From("cloud_store").
		Where(goqu.Ex{
			"name": "localstore",
		}).ToSQL()

	if err != nil {
		return err
	}

	stmt1, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[26] failed to prepare statment: %v", err)
		return err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	res := stmt1.QueryRow(vars...)
	var storageReferenceId string
	err = res.Scan(&storageReferenceId)
	stmt1.Close()
	if err != nil {
		if err == sql.ErrNoRows {

			adminUserId, adminGroupId := GetAdminUserIdAndUserGroupId(transaction)
			newUuid, _ := uuid.NewV7()
			query, vars, err = statementbuilder.Squirrel.Insert("cloud_store").Prepared(true).
				Cols("reference_id", "name", "store_type", "store_provider", "root_path", "store_parameters", "user_account_id", "permission").
				Vals([]interface{}{newUuid[:], "localstore", "local", "local", localStoragePath, "", adminUserId, auth.DEFAULT_PERMISSION}).ToSQL()

			if err != nil {
				return err
			}

			_, err = transaction.Exec(query, vars...)
			if err != nil {
				return err
			}

			query, vars, err = statementbuilder.Squirrel.Select("id").From("cloud_store").Prepared(true).
				Where(goqu.Ex{
					"reference_id": newUuid[:],
				}).ToSQL()
			if err != nil {
				return err
			}

			stmt1, err := transaction.Preparex(query)
			if err != nil {
				log.Errorf("[67] failed to prepare statment: %v", err)
			}
			defer func(stmt1 *sqlx.Stmt) {
				err := stmt1.Close()
				if err != nil {
					log.Errorf("failed to close prepared statement: %v", err)
				}
			}(stmt1)

			row := stmt1.QueryRowx(vars...)
			if row.Err() != nil {
				return row.Err()
			}
			var id int64
			err = row.Scan(&id)
			if err != nil {
				return err
			}
			stmt1.Close()

			groupRefId, _ := uuid.NewV7()
			query, vars, err = statementbuilder.Squirrel.Insert("cloud_store_cloud_store_id_has_usergroup_usergroup_id").Prepared(true).
				Cols("cloud_store_id", "usergroup_id", "reference_id", "permission").
				Vals([]interface{}{id, adminGroupId, groupRefId[:], auth.DEFAULT_PERMISSION}).ToSQL()

			if err != nil {
				return err
			}

			_, err = transaction.Exec(query, vars...)

			return err

		} else {
			return err
		}
	} else {
		return nil
	}

}
