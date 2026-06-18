package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const mailMessageFileType = "message/rfc822"

func (dbResource *DbResource) MailColumnValue(tableName, columnName string, messageBytes []byte, nameHint string) interface{} {
	encoded := base64.StdEncoding.EncodeToString(messageBytes)
	tableResource := dbResource.Cruds[tableName]
	if tableResource == nil || tableResource.TableInfo() == nil {
		return encoded
	}
	column, ok := tableResource.TableInfo().GetColumnByName(columnName)
	if !ok || column == nil || !column.IsForeignKey || column.ForeignKeyData.DataSource != "cloud_store" {
		return encoded
	}

	return []interface{}{
		map[string]interface{}{
			"name":     mailMessageFileName(nameHint, messageBytes),
			"path":     "",
			"type":     mailMessageFileType,
			"contents": encoded,
		},
	}
}

func (dbResource *DbResource) MailColumnBytes(tableName, columnName string, columnValue interface{}) ([]byte, error) {
	tableResource := dbResource.Cruds[tableName]
	if tableResource != nil && tableResource.TableInfo() != nil {
		column, ok := tableResource.TableInfo().GetColumnByName(columnName)
		if ok && column != nil && column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {
			switch value := columnValue.(type) {
			case []map[string]interface{}:
				return mailFileContents(value)
			case []interface{}:
				files := make([]map[string]interface{}, 0, len(value))
				for _, file := range value {
					fileMap, ok := file.(map[string]interface{})
					if !ok {
						return nil, errors.New("mail file metadata is invalid")
					}
					files = append(files, fileMap)
				}
				return mailFileContents(files)
			case string:
				return base64.StdEncoding.DecodeString(value)
			case []byte:
				return base64.StdEncoding.DecodeString(string(value))
			default:
				return nil, errors.New("mail file contents are not included")
			}
		}
	}

	switch value := columnValue.(type) {
	case string:
		return base64.StdEncoding.DecodeString(value)
	case []byte:
		return base64.StdEncoding.DecodeString(string(value))
	default:
		return nil, errors.New("mail column has unsupported value")
	}
}

func mailFileContents(files []map[string]interface{}) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("mail file list is empty")
	}
	contents, ok := files[0]["contents"].(string)
	if !ok {
		return nil, errors.New("mail file contents are not included")
	}
	return base64.StdEncoding.DecodeString(contents)
}

func isBuiltInMailBodyColumn(tableName, columnName string) bool {
	return columnName == "mail" && (tableName == "mail" || tableName == "outbox")
}

func dbBackedMailColumnFileList(row map[string]interface{}, columnValue interface{}, includeContents bool) ([]map[string]interface{}, bool) {
	encoded, ok := mailColumnBase64String(columnValue)
	if !ok || encoded == "" {
		return nil, false
	}

	name := dbBackedMailFileName(row)
	file := map[string]interface{}{
		"name": name,
		"path": "",
		"src":  name,
		"type": mailMessageFileType,
	}
	if includeContents {
		file["contents"] = encoded
	}
	if mailBytes, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		file["size"] = len(mailBytes)
		file["md5"] = GetMD5Hash(mailBytes)
	}

	return []map[string]interface{}{file}, true
}

func mailColumnBase64String(columnValue interface{}) (string, bool) {
	switch value := columnValue.(type) {
	case string:
		return value, true
	case []byte:
		return string(value), true
	default:
		return "", false
	}
}

func dbBackedMailFileName(row map[string]interface{}) string {
	for _, key := range []string{"hash", "mail_id", "message_id", "reference_id"} {
		if value, ok := row[key]; ok && value != nil {
			name := strings.TrimSuffix(strings.TrimSpace(fmt.Sprintf("%v", value)), ".eml")
			name = filepath.Base(sanitizeMailFileName(name))
			name = strings.Trim(name, "._-")
			if name != "" {
				return name + ".eml"
			}
		}
	}
	return "message.eml"
}

func mailMessageFileName(nameHint string, messageBytes []byte) string {
	name := strings.TrimSuffix(strings.TrimSpace(nameHint), ".eml")
	if name == "" {
		name = GetMD5Hash(messageBytes)
	}
	name = filepath.Base(sanitizeMailFileName(name))
	name = strings.Trim(name, "._-")
	if name == "" {
		name = "message"
	}
	if len(name) > 96 {
		name = name[:96]
	}
	u, err := uuid.NewV7()
	if err != nil {
		return name + ".eml"
	}
	return name + "-" + u.String() + ".eml"
}

func sanitizeMailFileName(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

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
	defer transaction.Rollback()

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
	if err != nil {
		return err
	}

	return transaction.Commit()

}

// RenameMailAccountBox renames a mailbox. Per RFC 3strstrings, renaming INBOX
// moves all messages to the new mailbox and leaves INBOX empty.
func (dbResource *DbResource) RenameMailAccountBox(mailAccountId int64, oldBoxName string, newBoxName string) error {

	transaction, err := dbResource.Cruds["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	box, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": mailAccountId,
			"name":            oldBoxName,
		},
	)
	if err != nil || len(box) == 0 {
		return errors.New("mailbox does not exist")
	}

	if strings.EqualFold(oldBoxName, "INBOX") {
		// RFC 3501: Renaming INBOX creates new mailbox and moves messages, INBOX stays empty
		// First check if target already exists
		existing, _ := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
			goqu.Ex{"mail_account_id": mailAccountId, "name": newBoxName})
		if len(existing) > 0 {
			return errors.New("target mailbox already exists")
		}

		// Create the new mailbox by duplicating INBOX's row with new name
		oldBoxId := box[0]["id"]
		newRefId, _ := uuid.NewV7()
		query, args, err := statementbuilder.Squirrel.
			Insert("mail_box").Prepared(true).
			Cols("name", "mail_account_id", "uidvalidity", "nextuid", "subscribed", "attributes", "flags", "permanent_flags", "reference_id", "permission").
			Vals(goqu.Vals{newBoxName, mailAccountId, time.Now().Unix(), 1, true, "", "\\*", "\\*", newRefId.String(), box[0]["permission"]}).
			ToSQL()
		if err != nil {
			return err
		}
		_, err = transaction.Exec(query, args...)
		if err != nil {
			return err
		}

		// Move all messages from INBOX to new mailbox
		newBox, _ := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
			goqu.Ex{"mail_account_id": mailAccountId, "name": newBoxName})
		if len(newBox) > 0 {
			moveQuery, moveArgs, moveErr := statementbuilder.Squirrel.
				Update("mail").Prepared(true).
				Set(goqu.Record{"mail_box_id": newBox[0]["id"]}).
				Where(goqu.Ex{"mail_box_id": oldBoxId}).ToSQL()
			if moveErr != nil {
				return moveErr
			}
			_, err = transaction.Exec(moveQuery, moveArgs...)
			if err != nil {
				return err
			}
		}
	} else {
		query, args, err := statementbuilder.Squirrel.
			Update("mail_box").Prepared(true).
			Set(goqu.Record{"name": newBoxName}).
			Where(goqu.Ex{"id": box[0]["id"]}).ToSQL()
		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		if err != nil {
			return err
		}
	}

	return transaction.Commit()

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
