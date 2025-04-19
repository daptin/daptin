package resource

import (
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

type Credential struct {
	DataMap map[string]interface{}
	Name    string
}

func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*Credential, error) {
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
	return &Credential{
		Name:    credentialName,
		DataMap: decryptedSpecMap,
	}, nil
}

func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*Credential, error) {
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
	return &Credential{
		Name:    credentialRow["name"].(string),
		DataMap: decryptedSpecMap,
	}, nil
}
