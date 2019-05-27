package resource

import (
	"errors"
	"github.com/emersion/go-imap/backend"
)

type DaptinImapBackend struct {
	cruds map[string]*DbResource
}

func (be *DaptinImapBackend) Login(username, password string) (backend.User, error) {

	userMailAccount, err := be.cruds[USER_ACCOUNT_TABLE_NAME].GetUserMailAccountRowByEmail(username)
	if err != nil {
		return nil, err
	}

	if BcryptCheckStringHash(password, userMailAccount["password"].(string)) {

		return &DaptinImapUser{
			username:               username,
			mailAccountId:          userMailAccount["id"].(int64),
			mailAccountReferenceId: userMailAccount["reference_id"].(string),
			dbResource:             be.cruds,
		}, nil
	}

	return nil, errors.New("bad username or password")
}

func NewImapServer(cruds map[string]*DbResource) *DaptinImapBackend {
	user := &DaptinImapUser{}

	user.mailboxes = map[string]*backend.Mailbox{}
	return &DaptinImapBackend{
		cruds: cruds,
	}
}
