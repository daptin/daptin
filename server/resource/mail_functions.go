package resource

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// Returns the user account row of a user by looking up on email
func (dbResource *DbResource) GetUserMailAccountRowByEmail(username string, transaction *sqlx.Tx) (map[string]interface{}, error) {

	mailAccount, _, err := dbResource.Cruds["mail_account"].GetRowsByWhereClause("mail_account",
		nil, transaction, goqu.Ex{"username": username})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail account")

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) GetMailAccountBox(mailAccountId int64, mailBoxName string, transaction *sqlx.Tx) (map[string]interface{}, error) {

	mailAccount, _, err := dbResource.Cruds["mail_box"].GetRowsByWhereClauseWithTransaction(
		"mail_box", nil, transaction, goqu.Ex{"mail_account_id": mailAccountId}, goqu.Ex{"name": mailBoxName})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail box")

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) CreateMailAccountBox(mailAccountId string,
	sessionUser *auth.SessionUser, mailBoxName string, transaction *sqlx.Tx) (map[string]interface{}, error) {

	mailBoxUrl, _ := url.Parse("/api/mail_box")
	httpRequest := &http.Request{
		Method: "POST",
		URL:    mailBoxUrl,
	}

	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	resp, err := dbResource.Cruds["mail_box"].CreateWithTransaction(api2go.NewApi2GoModelWithData("mail_box", nil, 0, nil, map[string]interface{}{
		"name":            mailBoxName,
		"mail_account_id": mailAccountId,
		"uidvalidity":     time.Now().Unix(),
		"nextuid":         1,
		"subscribed":      true,
		"attributes":      "",
		"flags":           "\\*",
		"permanent_flags": "\\*",
	}), api2go.Request{
		PlainRequest: httpRequest,
	}, transaction)

	return resp.Result().(api2go.Api2GoModel).GetAttributes(), err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) DeleteMailAccountBox(mailAccountId int64, mailBoxName string) error {

	transaction, err := dbResource.Cruds["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	box, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            mailBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail").Prepared(true).
		Where(goqu.Ex{"mail_box_id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		return err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail_box").Prepared(true).Where(goqu.Ex{"id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) RenameMailAccountBox(mailAccountId int64, oldBoxName string, newBoxName string) error {

	transaction, err := dbResource.Cruds["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}

	box, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            oldBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.
		Update("mail_box").Prepared(true).
		Set(goqu.Record{"name": newBoxName}).
		Where(goqu.Ex{"id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) SetMailBoxSubscribed(mailAccountId int64, mailBoxName string, subscribed bool, transaction *sqlx.Tx) error {

	query, args, err := statementbuilder.Squirrel.
		Update("mail_box").Prepared(true).
		Set(goqu.Record{"subscribed": subscribed}).
		Where(goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            mailBoxName,
		}).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)

	return err

}
