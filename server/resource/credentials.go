package resource

import (
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
		"credential", "name", credentialName, transaction)
	if err != nil {
		return nil, err
	}

	encryptionSecret, _ := d.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, err
	}
	return &dbresourceinterface.Credential{
		Name:    credentialName,
		DataMap: decryptedSpecMap,
	}, nil
}

func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*dbresourceinterface.Credential, error) {
	credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
		"credential", "reference_id", referenceId[:], transaction)
	if err != nil {
		return nil, err
	}

	encryptionSecret, _ := d.ConfigStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, err
	}
	return &dbresourceinterface.Credential{
		Name:    credentialRow["name"].(string),
		DataMap: decryptedSpecMap,
	}, nil
}
