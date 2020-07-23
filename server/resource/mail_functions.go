package resource

import (
	"context"
	"errors"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"net/http"
	"time"
)

// Returns the user account row of a user by looking up on email
func (d *DbResource) GetUserMailAccountRowByEmail(username string) (map[string]interface{}, error) {

	mailAccount, _, err := d.Cruds["mail_account"].GetRowsByWhereClause("mail_account",
		squirrel.Eq{"username": username})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail account")

}

// Returns the user mail account box row of a user
func (d *DbResource) GetMailAccountBox(mailAccountId int64, mailBoxName string) (map[string]interface{}, error) {

	mailAccount, _, err := d.Cruds["mail_box"].GetRowsByWhereClause("mail_box", squirrel.Eq{"mail_account_id": mailAccountId}, squirrel.Eq{"name": mailBoxName})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail box")

}

// Returns the user mail account box row of a user
func (d *DbResource) CreateMailAccountBox(mailAccountId string, sessionUser *auth.SessionUser, mailBoxName string) (map[string]interface{}, error) {

	httpRequest := &http.Request{
		Method: "POST",
	}

	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	resp, err := d.Cruds["mail_box"].Create(&api2go.Api2GoModel{
		Data: map[string]interface{}{
			"name":            mailBoxName,
			"mail_account_id": mailAccountId,
			"uidvalidity":     time.Now().Unix(),
			"nextuid":         1,
			"subscribed":      true,
			"attributes":      "",
			"flags":           "\\*",
			"permanent_flags": "\\*",
		},
	}, api2go.Request{
		PlainRequest: httpRequest,
	})

	return resp.Result().(*api2go.Api2GoModel).Data, err

}

// Returns the user mail account box row of a user
func (d *DbResource) DeleteMailAccountBox(mailAccountId int64, mailBoxName string) error {

	box, err := d.Cruds["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
			"mail_account_id": mailAccountId,
			"name":            mailBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail").Where(squirrel.Eq{"mail_box_id": box[0]["id"]}).ToSql()
	if err != nil {
		return err
	}

	_, err = d.db.Exec(query, args...)
	if err != nil {
		return err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail_box").Where(squirrel.Eq{"id": box[0]["id"]}).ToSql()
	if err != nil {
		return err
	}

	_, err = d.db.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (d *DbResource) RenameMailAccountBox(mailAccountId int64, oldBoxName string, newBoxName string) error {

	box, err := d.Cruds["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
			"mail_account_id": mailAccountId,
			"name":            oldBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.Update("mail_box").Set("name", newBoxName).Where(squirrel.Eq{"id": box[0]["id"]}).ToSql()
	if err != nil {
		return err
	}

	_, err = d.db.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (d *DbResource) SetMailBoxSubscribed(mailAccountId int64, mailBoxName string, subscribed bool) error {

	query, args, err := statementbuilder.Squirrel.Update("mail_box").Set("subscribed", subscribed).Where(squirrel.Eq{
		"mail_account_id": mailAccountId,
		"name":            mailBoxName,
	}).ToSql()
	if err != nil {
		return err
	}

	_, err = d.db.Exec(query, args...)

	return err

}
