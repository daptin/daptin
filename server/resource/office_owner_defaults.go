package resource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// This exact address is reserved for the standalone Office mailbox.
const officeLocalMailHost = "office.invalid"
const officeLocalMailbox = "owner@" + officeLocalMailHost

func isOfficeLocalMailbox(account map[string]interface{}) bool {
	return StringOrEmpty(account["username"]) == officeLocalMailbox
}

// ProvisionOfficeOwnerDefaults runs inside the one-time administrator claim
// transaction. The caller has just added the owner to administrators.
func (dbResource *DbResource) ProvisionOfficeOwnerDefaults(userID int64, ref daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	owner := &auth.SessionUser{UserId: userID, UserReferenceId: ref,
		Groups: dbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, tx, "id", userID)}
	if !IsAdminWithTransaction(owner, tx) {
		return errors.New("Office defaults require the new administrator")
	}
	owned := goqu.Ex{USER_ACCOUNT_ID_COLUMN: userID}
	calendars, err := GetObjectByWhereClauseWithTransaction(calendarCollectionTable, tx, owned)
	if err != nil {
		return err
	}
	if len(calendars) == 0 {
		dav := NewCalDAVBackend(dbResource.Cruds, owner, nil)
		if err := dav.calendarCreate(calendarCollectionTable, dav.homePath()+"personal/",
			map[string]interface{}{"name": "personal", "display_name": "Personal", "description": ""}, tx); err != nil {
			return fmt.Errorf("create Personal calendar: %w", err)
		}
	}

	accounts, err := GetObjectByWhereClauseWithTransaction("mail_account", tx, owned)
	if err != nil || len(accounts) != 0 {
		return err
	}
	create := func(resource string, attrs map[string]interface{}) (string, error) {
		r, err := http.NewRequestWithContext(context.WithValue(context.Background(), "user", owner), http.MethodPost, "/api/"+resource, nil)
		if err != nil {
			return "", err
		}
		crud := dbResource.Cruds[resource]
		resp, err := crud.CreateWithTransaction(api2go.NewApi2GoModelWithData(resource, nil, 0, nil, attrs),
			api2go.Request{PlainRequest: r}, tx)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", resource, err)
		}
		model, ok := resp.Result().(api2go.Api2GoModel)
		if !ok || model.GetID() == "" {
			return "", fmt.Errorf("create %s returned no reference ID", resource)
		}
		return model.GetID(), nil
	}
	server, err := create("mail_server", map[string]interface{}{
		"hostname": officeLocalMailHost, "is_enabled": false, "listen_interface": "127.0.0.1:0",
	})
	if err != nil {
		return err
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return err
	}
	password := hex.EncodeToString(secret[:])
	account, err := create("mail_account", map[string]interface{}{
		"username": officeLocalMailbox, "mail_server_id": server, "password": password, "password_md5": password,
	})
	if err != nil {
		return err
	}
	_, err = dbResource.Cruds["mail_account"].CreateMailAccountBox(account, owner, "INBOX", tx)
	return err
}
