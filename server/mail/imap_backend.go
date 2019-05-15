package mail

import (
	"errors"
	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-imap/backend"
)

type DaptinImapBackend struct {
	cruds map[string]*resource.DbResource
}

func (be *DaptinImapBackend) Login(username, password string) (backend.User, error) {
	return nil, errors.New("Bad username or password")
}

func NewImapServer(cruds map[string]*resource.DbResource) *DaptinImapBackend {
	user := &DaptinImapUser{}

	user.mailboxes = map[string]*backend.Mailbox{}
	return &DaptinImapBackend{
		cruds: cruds,
	}
}
