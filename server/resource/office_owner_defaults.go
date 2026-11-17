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
	"github.com/google/uuid"
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
	request := func(method, table string) api2go.Request {
		r, _ := http.NewRequestWithContext(context.WithValue(context.Background(), "user", owner), method, "/api/"+table, nil)
		return api2go.Request{PlainRequest: r}
	}
	makePrivate := func(table, ref string) error {
		if _, err := uuid.Parse(ref); err != nil {
			return fmt.Errorf("create %s returned no reference ID", table)
		}
		// The running resource model still has the bootstrap defaults; the
		// administrator restart only reloads it after this claim commits.
		model := api2go.NewApi2GoModelWithData(table, nil, 0, nil, map[string]interface{}{
			"reference_id": ref, "permission": int64(auth.UserCRUD | auth.UserExecute),
		})
		if _, err := dbResource.Cruds[table].UpdateWithTransaction(model, request(http.MethodPatch, table), tx); err != nil {
			return fmt.Errorf("make %s private: %w", table, err)
		}
		return nil
	}
	create := func(table string, attrs map[string]interface{}) (string, error) {
		resp, err := dbResource.Cruds[table].CreateWithTransaction(api2go.NewApi2GoModelWithData(table, nil, 0, nil, attrs),
			request(http.MethodPost, table), tx)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", table, err)
		}
		model, ok := resp.Result().(api2go.Api2GoModel)
		if !ok {
			return "", fmt.Errorf("create %s returned an invalid resource", table)
		}
		ref := model.GetID()
		return ref, makePrivate(table, ref)
	}
	owned := goqu.Ex{USER_ACCOUNT_ID_COLUMN: userID}
	calendars, err := GetObjectByWhereClauseWithTransaction(calendarCollectionTable, tx, owned)
	if err != nil {
		return err
	}
	if len(calendars) == 0 {
		if _, err := create(calendarCollectionTable,
			map[string]interface{}{"name": "personal", "display_name": "Personal", "description": ""}); err != nil {
			return fmt.Errorf("create Personal calendar: %w", err)
		}
	}

	accounts, err := GetObjectByWhereClauseWithTransaction("mail_account", tx, owned)
	if err != nil || len(accounts) != 0 {
		return err
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
	inbox, err := dbResource.Cruds["mail_account"].CreateMailAccountBox(account, owner, "INBOX", tx)
	if err != nil {
		return err
	}
	return makePrivate("mail_box", daptinid.InterfaceToDIR(inbox["reference_id"]).String())
}
