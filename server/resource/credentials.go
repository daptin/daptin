package resource

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	credentialRow, err := d.getCredentialRowByWhere("name", credentialName, transaction)
	if err != nil {
		return nil, err
	}

	return d.credentialFromRow(credentialRow, transaction)
}

func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	credentialRow, err := d.getCredentialRowByReferenceId(referenceId, transaction)
	if err != nil {
		return nil, err
	}

	return d.credentialFromRow(credentialRow, transaction)
}

func (d *DbResource) GetCredentialByReferenceIdForIntegrationExecution(referenceId daptinid.DaptinReferenceId, sessionUser *auth.SessionUser, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	if referenceId == daptinid.NullReferenceId {
		return nil, fmt.Errorf("credential_id is required for custom credential integration execution")
	}
	if sessionUser == nil || sessionUser.UserId == 0 || sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return nil, fmt.Errorf("custom credential integration execution requires an authenticated user")
	}

	credentialRow, err := d.getCredentialRowByReferenceId(referenceId, transaction)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("credential is not available for this user")
		}
		return nil, err
	}

	permission := d.GetObjectPermissionByIdWithTransaction("credential", credentialRow.id, transaction)
	if !permission.CanRead(sessionUser.UserReferenceId, sessionUser.Groups, d.AdministratorGroupId) {
		return nil, fmt.Errorf("credential is not available for this user")
	}

	return d.credentialFromRow(credentialRow, transaction)
}

type credentialRow struct {
	id      int64
	name    string
	content string
}

func (d *DbResource) credentialFromRow(credentialRow credentialRow, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	encryptionSecret, err := d.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	if err != nil {
		return nil, err
	}

	decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow.content)
	if err != nil {
		return nil, err
	}

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, err
	}
	return &dbresourceinterface.Credential{
		Name:    credentialRow.name,
		DataMap: decryptedSpecMap,
	}, nil
}

func (d *DbResource) getCredentialRowByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (credentialRow, error) {
	return d.getCredentialRowByWhere("reference_id", referenceId[:], transaction)
}

func (d *DbResource) getCredentialRowByWhere(column string, value interface{}, transaction *sqlx.Tx) (credentialRow, error) {
	s, v, err := statementbuilder.Squirrel.Select("id", "name", "content").Prepared(true).
		From("credential").Where(goqu.Ex{column: value}).ToSQL()
	if err != nil {
		return credentialRow{}, err
	}

	stmt, err := transaction.Preparex(s)
	if err != nil {
		return credentialRow{}, err
	}
	defer stmt.Close()

	var row credentialRow
	if err := stmt.QueryRowx(v...).Scan(&row.id, &row.name, &row.content); err != nil {
		return credentialRow{}, err
	}
	return row, nil
}
