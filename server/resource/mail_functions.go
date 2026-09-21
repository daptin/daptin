package resource

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	netmail "net/mail"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/parsemail"
	"github.com/bjarneh/latinx"
	"github.com/daptin/daptin/server/auth"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
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
	resp, err := dbResource.Cruds["mail_box"].createAfterAuthorizationWithTransaction(api2go.NewApi2GoModelWithData("mail_box", nil, 0, nil, map[string]interface{}{
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
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Result() == nil {
		return nil, errors.New("failed to create mail box")
	}

	return resp.Result().(api2go.Api2GoModel).GetAttributes(), err

}

// ResolveMailSenderAccount returns the mail account selected by the From
// address after verifying that it belongs to the active Daptin identity.
func (dbResource *DbResource) ResolveMailSenderAccount(fromAddress string, sessionUser *auth.SessionUser, transaction *sqlx.Tx) (map[string]interface{}, error) {
	if transaction == nil {
		return nil, errors.New("mail sender authorization requires a transaction")
	}
	if sessionUser == nil || sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return nil, errors.New("mail sender has no authenticated resource identity")
	}

	senderAddress, err := normalizedMailAddress(fromAddress)
	if err != nil {
		return nil, err
	}
	mailAccount, err := dbResource.GetUserMailAccountRowByEmail(senderAddress, transaction)
	if err != nil {
		return nil, fmt.Errorf("sender mail account not found [%s]: %w", senderAddress, err)
	}
	if daptinid.InterfaceToDIR(mailAccount[USER_ACCOUNT_ID_COLUMN]) != sessionUser.UserReferenceId {
		return nil, errors.New("authenticated user does not own sender mail account")
	}
	return mailAccount, nil
}

// MailAccountSessionUser resolves the Daptin identity established by a
// successfully authenticated protocol account.
func (dbResource *DbResource) MailAccountSessionUser(mailAccount map[string]interface{}, transaction *sqlx.Tx) (*auth.SessionUser, error) {
	userCrud := dbResource.Cruds[USER_ACCOUNT_TABLE_NAME]
	if userCrud == nil {
		return nil, errors.New("user_account resource is not configured")
	}
	userReference := daptinid.InterfaceToDIR(mailAccount[USER_ACCOUNT_ID_COLUMN])
	if userReference == daptinid.NullReferenceId {
		return nil, errors.New("mail account has no resource owner")
	}
	userID, err := userCrud.GetReferenceIdToId(USER_ACCOUNT_TABLE_NAME, userReference, transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve mail account owner: %w", err)
	}
	if userID == 0 {
		return nil, errors.New("mail account has no resource owner")
	}
	return &auth.SessionUser{
		UserId:          userID,
		UserReferenceId: userReference,
		Groups:          userCrud.GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, transaction, "id", userID),
	}, nil
}

// CreateInboundMailWithTransaction is the SMTP adapter boundary. The recipient
// account and destination mailbox must both match the server-created session
// before SMTP may enter the shared resource lifecycle after authorization.
func (dbResource *DbResource) CreateInboundMailWithTransaction(obj interface{}, req api2go.Request,
	transaction *sqlx.Tx) (api2go.Responder, error) {
	if dbResource.model.GetName() != "mail" {
		return nil, errors.New("inbound mail can only create mail resources")
	}
	data, ok := obj.(api2go.Api2GoModel)
	if !ok || req.PlainRequest == nil {
		return nil, errors.New("invalid inbound mail request")
	}
	sessionUser, _ := req.PlainRequest.Context().Value("user").(*auth.SessionUser)
	if sessionUser == nil || sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return nil, errors.New("inbound mail recipient has no authenticated resource identity")
	}
	attributes := data.GetAllAsAttributes()
	if daptinid.InterfaceToDIR(attributes[USER_ACCOUNT_ID_COLUMN]) != sessionUser.UserReferenceId {
		return nil, errors.New("inbound mail recipient does not match resource owner")
	}
	mailboxReference := daptinid.InterfaceToDIR(attributes["mail_box_id"])
	mailbox, err := dbResource.Cruds["mail_box"].GetReferenceIdToObjectWithTransaction("mail_box", mailboxReference, transaction)
	if err != nil {
		return nil, err
	}
	mailAccountID, ok := mailbox["mail_account_id"].(int64)
	if !ok || mailAccountID == 0 {
		return nil, errors.New("inbound mail destination has no mail account")
	}
	mailAccount, _, err := dbResource.Cruds["mail_account"].GetSingleRowById("mail_account", mailAccountID, nil, transaction)
	if err != nil {
		return nil, err
	}
	if daptinid.InterfaceToDIR(mailAccount[USER_ACCOUNT_ID_COLUMN]) != sessionUser.UserReferenceId {
		return nil, errors.New("inbound mail destination does not belong to recipient")
	}
	return dbResource.createAfterAuthorizationWithTransaction(obj, req, transaction)
}

func (dbResource *DbResource) AppendSentMailForSender(fromAddress string, sessionUser *auth.SessionUser, messageBytes []byte, transaction *sqlx.Tx) (map[string]interface{}, error) {
	if transaction == nil {
		return nil, errors.New("sent mailbox append requires a transaction")
	}

	senderAddress, err := normalizedMailAddress(fromAddress)
	if err != nil {
		return nil, err
	}

	mailAccount, err := dbResource.ResolveMailSenderAccount(senderAddress, sessionUser, transaction)
	if err != nil {
		return nil, err
	}

	mailAccountId, ok := mailAccount["id"].(int64)
	if !ok || mailAccountId == 0 {
		return nil, fmt.Errorf("invalid mail account id for sender [%s]", senderAddress)
	}

	sentBox, err := dbResource.GetMailAccountBox(mailAccountId, "Sent", transaction)
	if err != nil {
		err = dbResource.Cruds["mail_account"].LockMailAccountForMailboxCreation(mailAccountId, transaction)
		if err != nil {
			return nil, err
		}
		sentBox, err = dbResource.GetMailAccountBox(mailAccountId, "Sent", transaction)
		if err != nil {
			_, err = dbResource.CreateMailAccountBox(
				daptinid.InterfaceToDIR(mailAccount["reference_id"]).String(),
				sessionUser,
				"Sent",
				transaction,
			)
			if err != nil {
				return nil, err
			}
			sentBox, err = dbResource.GetMailAccountBox(mailAccountId, "Sent", transaction)
			if err != nil {
				return nil, err
			}
		}
	}

	mailBoxId, ok := sentBox["id"].(int64)
	if !ok || mailBoxId == 0 {
		return nil, fmt.Errorf("invalid Sent mailbox id for sender [%s]", senderAddress)
	}
	uid, err := dbResource.Cruds["mail_box"].AllocateMailBoxUid(mailBoxId, transaction)
	if err != nil {
		return nil, err
	}

	attrs, err := dbResource.sentMailAttributes(messageBytes, sentBox, uid)
	if err != nil {
		return nil, err
	}

	requestURL, _ := url.Parse("/api/mail")
	httpRequest := (&http.Request{
		Method: "POST",
		URL:    requestURL,
	}).WithContext(context.WithValue(context.Background(), "user", sessionUser))

	resp, err := dbResource.Cruds["mail"].createAfterAuthorizationWithTransaction(
		api2go.NewApi2GoModelWithData("mail", nil, 768, nil, attrs),
		api2go.Request{PlainRequest: httpRequest},
		transaction,
	)
	if err != nil {
		return nil, err
	}
	return resp.Result().(api2go.Api2GoModel).GetAttributes(), nil
}

func (dbResource *DbResource) sentMailAttributes(messageBytes []byte, sentBox map[string]interface{}, uid uint32) (map[string]interface{}, error) {
	messageEntity, err := message.Read(bytes.NewReader(messageBytes))
	if err != nil {
		return nil, err
	}

	hash := GetMD5Hash(messageBytes)
	storedMailContents := dbResource.MailColumnValue("mail", "mail", messageBytes, hash)
	parsedMail, err := parsemail.Parse(bytes.NewReader(messageBytes))
	if err != nil {
		return nil, err
	}

	textBody := parsedMail.TextBody
	if strings.Contains(strings.ToLower(parsedMail.Header.Get("Content-Type")), "iso-8859-1") {
		converter := latinx.Get(latinx.ISO_8859_1)
		textBodyBytes, err := converter.Decode([]byte(textBody))
		if err == nil {
			textBody = string(textBodyBytes)
		}
	}

	mailDate := parsedMail.Date
	if parsedDate, _, err := fieldtypes.GetDateTime(parsedMail.Header.Get("Date")); err == nil {
		mailDate = parsedDate
	}
	if mailDate.IsZero() {
		mailDate = time.Now()
	}
	searchMetadata := mailSearchMetadataFromParsed(parsedMail)
	searchMetadata.Body = textBody

	messageId := searchMetadata.MessageID
	if strings.TrimSpace(messageId) == "" {
		messageId = uuid.NewString()
	}

	return map[string]interface{}{
		"message_id":       messageId,
		"mail_id":          hash,
		"from_address":     searchMetadata.From,
		"to_address":       searchMetadata.To,
		"cc_address":       searchMetadata.Cc,
		"bcc_address":      searchMetadata.Bcc,
		"sender_address":   searchMetadata.Sender,
		"subject":          searchMetadata.Subject,
		"body":             searchMetadata.Body,
		"mail":             storedMailContents,
		"spam_score":       0,
		"spam":             false,
		"hash":             hash,
		"internal_date":    time.Now(),
		"sent_date":        nullableMailDate(mailDate),
		"content_type":     messageEntity.Header.Get("Content-Type"),
		"reply_to_address": searchMetadata.ReplyTo,
		"recipient":        searchMetadata.To,
		"has_attachment":   len(parsedMail.Attachments) > 0,
		"ip_addr":          "",
		"return_path":      searchMetadata.From,
		"is_tls":           false,
		"mail_box_id":      sentBox["reference_id"],
		"uid":              uid,
		"seen":             true,
		"recent":           false,
		"deleted":          false,
		"flags":            imap.SeenFlag,
		"size":             len(messageBytes),
	}, nil
}

func normalizedMailAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("mail address is empty")
	}
	parsed, err := netmail.ParseAddress(address)
	if err == nil && parsed != nil {
		return strings.TrimSpace(parsed.Address), nil
	}
	if strings.Contains(address, "@") {
		return address, nil
	}
	return "", err
}

// Returns the user mail account box row of a user
func (dbResource *DbResource) DeleteMailAccountBox(mailAccountId int64, mailBoxName string, sessionUser *auth.SessionUser) error {

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

	mailRequestURL, _ := url.Parse("/api/mail")
	mailRequest := api2go.Request{PlainRequest: (&http.Request{
		Method: http.MethodDelete,
		URL:    mailRequestURL,
	}).WithContext(context.WithValue(context.Background(), "user", sessionUser))}

	query, args, err := statementbuilder.Squirrel.Select("reference_id").Prepared(true).
		From("mail").Where(goqu.Ex{"mail_box_id": box[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}
	rows, err := transaction.Queryx(query, args...)
	if err != nil {
		return err
	}
	mailReferenceIDs := make([]daptinid.DaptinReferenceId, 0)
	for rows.Next() {
		var referenceID daptinid.DaptinReferenceId
		if err := rows.Scan(&referenceID); err != nil {
			rows.Close()
			return err
		}
		mailReferenceIDs = append(mailReferenceIDs, referenceID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, referenceID := range mailReferenceIDs {
		if _, err := dbResource.Cruds["mail"].deleteAfterAuthorizationWithTransaction(referenceID, mailRequest, transaction); err != nil {
			return err
		}
	}

	mailBoxRequestURL, _ := url.Parse("/api/mail_box")
	mailBoxRequest := api2go.Request{PlainRequest: (&http.Request{
		Method: http.MethodDelete,
		URL:    mailBoxRequestURL,
	}).WithContext(context.WithValue(context.Background(), "user", sessionUser))}
	mailBoxReferenceID := daptinid.InterfaceToDIR(box[0]["reference_id"])
	if _, err := dbResource.Cruds["mail_box"].deleteAfterAuthorizationWithTransaction(mailBoxReferenceID, mailBoxRequest, transaction); err != nil {
		return err
	}

	return transaction.Commit()

}

// RenameMailAccountBox renames a mailbox. Per RFC 3strstrings, renaming INBOX
// moves all messages to the new mailbox and leaves INBOX empty.
func (dbResource *DbResource) RenameMailAccountBox(mailAccountId int64, oldBoxName string, newBoxName string,
	sessionUser *auth.SessionUser) error {

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

	requestURL, _ := url.Parse("/api/mail_box")
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: requestURL}).WithContext(
		context.WithValue(context.Background(), "user", sessionUser))}

	if strings.EqualFold(oldBoxName, "INBOX") {
		// RFC 3501: Renaming INBOX creates new mailbox and moves messages, INBOX stays empty
		// First check if target already exists
		existing, _ := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
			goqu.Ex{"mail_account_id": mailAccountId, "name": newBoxName})
		if len(existing) > 0 {
			return errors.New("target mailbox already exists")
		}

		oldBoxId := box[0]["id"]
		accountReference, err := GetIdToReferenceIdWithTransaction("mail_account", mailAccountId, transaction)
		if err != nil {
			return err
		}
		if _, err = dbResource.CreateMailAccountBox(accountReference.String(), sessionUser, newBoxName, transaction); err != nil {
			return err
		}

		// Move all messages from INBOX to new mailbox
		newBox, _ := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
			goqu.Ex{"mail_account_id": mailAccountId, "name": newBoxName})
		if len(newBox) > 0 {
			mails, err := dbResource.Cruds["mail"].GetAllObjectsWithWhereWithTransaction("mail", transaction,
				goqu.Ex{"mail_box_id": oldBoxId})
			if err != nil {
				return err
			}
			mailURL, _ := url.Parse("/api/mail")
			mailRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: mailURL}).WithContext(
				context.WithValue(context.Background(), "user", sessionUser))}
			for _, mail := range mails {
				model := api2go.NewApi2GoModelWithData("mail", nil, 0, nil, map[string]interface{}{
					"mail_box_id": daptinid.InterfaceToDIR(newBox[0]["reference_id"]).String(),
				})
				model.SetID(daptinid.InterfaceToDIR(mail["reference_id"]).String())
				if _, err := dbResource.Cruds["mail"].updateAfterAuthorizationWithTransaction(model, mailRequest, transaction); err != nil {
					return err
				}
			}
		}
	} else {
		model := api2go.NewApi2GoModelWithData("mail_box", nil, 0, nil, map[string]interface{}{"name": newBoxName})
		model.SetID(daptinid.InterfaceToDIR(box[0]["reference_id"]).String())
		if _, err = dbResource.Cruds["mail_box"].updateAfterAuthorizationWithTransaction(model, request, transaction); err != nil {
			return err
		}
	}

	return transaction.Commit()

}

// Returns the user mail account box row of a user
func (dbResource *DbResource) SetMailBoxSubscribed(mailAccountId int64, mailBoxName string, subscribed bool,
	sessionUser *auth.SessionUser, transaction *sqlx.Tx) error {
	box, err := dbResource.GetMailAccountBox(mailAccountId, mailBoxName, transaction)
	if err != nil {
		return err
	}
	model := api2go.NewApi2GoModelWithData("mail_box", nil, 0, nil, map[string]interface{}{"subscribed": subscribed})
	model.SetID(daptinid.InterfaceToDIR(box["reference_id"]).String())
	requestURL, _ := url.Parse("/api/mail_box")
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: requestURL}).WithContext(
		context.WithValue(context.Background(), "user", sessionUser))}
	_, err = dbResource.Cruds["mail_box"].updateAfterAuthorizationWithTransaction(model, request, transaction)
	return err

}
