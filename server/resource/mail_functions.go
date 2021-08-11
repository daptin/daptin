package resource

import (
	"context"
	"errors"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"net/http"
	"time"
)

// Returns the user account row of a user by looking up on email
func (dbResource *DbResource) GetUserMailAccountRowByEmail(username string) (map[string]interface{}, error) {

	mailAccount, _, err := dbResource.Cruds["mail_account"].GetRowsByWhereClause("mail_account",
		nil, goqu.Ex{"username": username})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail account")

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) GetMailAccountBox(mailAccountId int64, mailBoxName string) (map[string]interface{}, error) {

	mailAccount, _, err := dbResource.Cruds["mail_box"].GetRowsByWhereClause("mail_box", nil, goqu.Ex{"mail_account_id": mailAccountId}, goqu.Ex{"name": mailBoxName})

	if len(mailAccount) > 0 {

		return mailAccount[0], err
	}

	return nil, errors.New("no such mail box")

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) CreateMailAccountBox(mailAccountId string, sessionUser *auth.SessionUser, mailBoxName string) (map[string]interface{}, error) {

	httpRequest := &http.Request{
		Method: "POST",
	}

	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	resp, err := dbResource.Cruds["mail_box"].Create(api2go.Api2GoModel{
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

	return resp.Result().(api2go.Api2GoModel).Data, err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) DeleteMailAccountBox(mailAccountId int64, mailBoxName string) error {

	box, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhere("mail_box",
		goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            mailBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail").Where(goqu.Ex{"mail_box_id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dbResource.db.Exec(query, args...)
	if err != nil {
		return err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail_box").Where(goqu.Ex{"id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dbResource.db.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) RenameMailAccountBox(mailAccountId int64, oldBoxName string, newBoxName string) error {

	box, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhere("mail_box",
		goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            oldBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	query, args, err := statementbuilder.Squirrel.
		Update("mail_box").
		Set(goqu.Record{"name": newBoxName}).
		Where(goqu.Ex{"id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dbResource.db.Exec(query, args...)

	return err

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) SetMailBoxSubscribed(mailAccountId int64, mailBoxName string, subscribed bool) error {

	query, args, err := statementbuilder.Squirrel.
		Update("mail_box").
		Set(goqu.Record{"subscribed": subscribed}).
		Where(goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            mailBoxName,
		}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dbResource.db.Exec(query, args...)

	return err

}
