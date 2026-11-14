package resource

import (
	"context"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type DaptinImapUser struct {
	dbResource             map[string]*DbResource
	username               string
	userAccountId          int64
	mailboxes              map[string]*backend.Mailbox
	mailAccountId          int64
	mailAccountReferenceId string
	sessionUser            *auth.SessionUser
}

// User represents a user in the mail storage system. A user operation always
// deals with mailboxes.
// Username returns this user's username.
func (diu *DaptinImapUser) Username() string {
	return diu.username
}

// ListMailboxes returns a list of mailboxes belonging to this user. If
// subscribed is set to true, only returns subscribed mailboxes.
func (diu *DaptinImapUser) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	log.Printf("[IMAP] ListMailboxes: starting")
	var boxes []backend.Mailbox
	log.Printf("[IMAP] ListMailboxes: attempting to begin transaction")
	transaction, err := diu.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		log.Printf("[IMAP] ListMailboxes: failed to begin transaction: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] ListMailboxes: transaction started")
	defer transaction.Rollback()
	caller, err := diu.dbResource["mail_box"].MailSessionUser(diu.sessionUser, transaction)
	if err != nil {
		return nil, err
	}
	if _, err = diu.dbResource["mail_box"].readMailResourceForSession("mail_account", daptinid.InterfaceToDIR(diu.mailAccountReferenceId), caller, transaction); err != nil {
		return nil, err
	}
	mailBoxes, err := diu.dbResource["mail_box"].GetAllObjectsWithWhereWithTransaction(
		"mail_box", transaction, goqu.Ex{"mail_account_id": diu.mailAccountId})
	if err != nil {
		return boxes, err
	}

	byRef := map[string]map[string]interface{}{}
	for _, box := range mailBoxes {
		byRef[daptinid.InterfaceToDIR(box["reference_id"]).String()] = box
	}
	mailBoxes = nil
	for page := 1; ; page++ {
		query, err := encodingjson.Marshal([]Query{{ColumnName: "mail_account_id", Operator: "=", Value: diu.mailAccountReferenceId}})
		if err != nil {
			return nil, err
		}
		location, _ := url.Parse("/api/mail_box")
		requestContext := context.WithValue(context.Background(), "user", caller)
		if page > 1 {
			requestContext = WithMeteringInternal(requestContext)
		}
		request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodGet, URL: location}).WithContext(requestContext),
			QueryParams: map[string][]string{"query": {string(query)}, "page[size]": {"200"}, "page[number]": {fmt.Sprint(page)}}}
		_, response, err := diu.dbResource["mail_box"].PaginatedFindAllWithTransaction(request, transaction)
		if err != nil {
			return nil, err
		}
		models, ok := response.Result().([]api2go.Api2GoModel)
		if !ok {
			return nil, errors.New("native folder discovery returned invalid rows")
		}
		for _, model := range models {
			if box, found := byRef[model.GetID()]; found {
				mailBoxes = append(mailBoxes, box)
			}
		}
		if len(models) < 200 {
			break
		}
	}

	hasTrash := false
	hasDraft := false
	hasSent := false
	hasSpam := false
	hasArchive := false
	hasInbox := false

	for _, box := range mailBoxes {
		if box["user_account_id"] == nil {
			continue
		}
		boxName, _ := box["name"].(string)
		if boxName == "" {
			continue
		}
		boxId, _ := box["id"].(int64)
		if boxId == 0 {
			continue
		}
		boxAttrs, _ := box["attributes"].(string)
		mb := DaptinImapMailBox{
			dbResource:         diu.dbResource,
			name:               boxName,
			sessionUser:        diu.sessionUser,
			mailAccountId:      diu.mailAccountId,
			mailBoxReferenceId: daptinid.InterfaceToDIR(box["reference_id"]).String(),
			knownKeywords:      make(map[string]bool),
			mailBoxId:          boxId,
			info: imap.MailboxInfo{
				Attributes: strings.Split(boxAttrs, ","),
				Delimiter:  "/",
				Name:       boxName,
			},
		}

		mailBoxName := strings.ToLower(mb.name)
		if mailBoxName == "trash" {
			hasTrash = true
		} else if mailBoxName == "inbox" {
			hasInbox = true
		} else if mailBoxName == "draft" {
			hasDraft = true
		} else if mailBoxName == "sent" {
			hasSent = true
		} else if mailBoxName == "spam" {
			hasSpam = true
		} else if mailBoxName == "archive" {
			hasArchive = true
		}
		boxes = append(boxes, &mb)
	}

	if !hasDraft {
		err = diu.CreateMailboxWithTransaction("Draft", transaction)
		if err != nil {
			log.Printf("Failed to create draft mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("Draft", transaction)
		if err != nil {
			log.Printf("Failed to fetch draft mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}

	}

	if !hasSpam {
		err = diu.CreateMailboxWithTransaction("Spam", transaction)
		if err != nil {
			log.Printf("Failed to create Spam mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("Spam", transaction)
		if err != nil {
			log.Printf("Failed to fetch Spam mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}
	}

	if !hasInbox {
		err = diu.CreateMailboxWithTransaction("INBOX", transaction)
		if err != nil {
			log.Printf("Failed to create Inbox mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("INBOX", transaction)
		if err != nil {
			log.Printf("Failed to fetch Inbox mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}
	}

	if !hasArchive {
		err = diu.CreateMailboxWithTransaction("Archive", transaction)
		if err != nil {
			log.Printf("Failed to create Archive mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("Archive", transaction)
		if err != nil {
			log.Printf("Failed to fetch Archive mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}
	}

	if !hasTrash {
		err = diu.CreateMailboxWithTransaction("Trash", transaction)
		if err != nil {
			log.Printf("Failed to create trash mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("Trash", transaction)
		if err != nil {
			log.Printf("Failed to fetch trash mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}

	}
	if !hasSent {
		err = diu.CreateMailboxWithTransaction("Sent", transaction)
		if err != nil {
			log.Printf("Failed to create Sent mailbox for imap account [%v]: %v", diu.username, err)
		}
		mailBox, err := diu.GetMailboxWithTransaction("Sent", transaction)
		if err != nil {
			log.Printf("Failed to fetch Sent mailbox for imap account [%v]: %v", diu.username, err)
		} else {
			boxes = append(boxes, mailBox)
		}

	}

	if subscribed {
		filtered := make([]backend.Mailbox, 0, len(boxes))
		for _, mailbox := range boxes {
			ref := mailbox.(*DaptinImapMailBox).mailBoxReferenceId
			if row, found := byRef[ref]; found {
				value, err := resourceRowBool(row["subscribed"])
				if err != nil {
					return nil, err
				}
				if !value {
					continue
				}
			}
			filtered = append(filtered, mailbox)
		}
		boxes = filtered
	}
	if err = transaction.Commit(); err != nil {
		return nil, err
	}
	return boxes, nil
}

// GetMailbox returns a mailbox. If it doesn't exist, it returns
// ErrNoSuchMailbox.
func (diu *DaptinImapUser) GetMailboxWithTransaction(name string, transaction *sqlx.Tx) (backend.Mailbox, error) {

	box, err := diu.dbResource["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": diu.mailAccountId,
			"name":            name,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(box) == 0 {
		return nil, errors.New("no such mailbox")
	}

	_, caller, err := diu.dbResource["mail_box"].readMailBoxForSession(daptinid.InterfaceToDIR(box[0]["reference_id"]), diu.sessionUser, transaction)
	if err != nil {
		return nil, err
	}

	boxId, ok := box[0]["id"].(int64)
	if !ok {
		return nil, fmt.Errorf("invalid mailbox id type: %T", box[0]["id"])
	}

	mbStatus, err := diu.dbResource["mail_box"].GetMailBoxStatus(diu.mailAccountId, boxId, transaction)
	if err != nil {
		return nil, err
	}

	boxName, _ := box[0]["name"].(string)
	boxFlags, _ := box[0]["flags"].(string)
	boxPermanentFlags, _ := box[0]["permanent_flags"].(string)
	mbStatus.Name = boxName
	mbStatus.Flags = strings.Split(boxFlags, ",")
	mbStatus.PermanentFlags = strings.Split(boxPermanentFlags, ",")

	// Load known keywords from existing messages
	kwMap := make(map[string]bool)
	kwList, kwErr := diu.dbResource["mail_box"].GetMailBoxKeywords(boxId, transaction)
	if kwErr == nil {
		for _, kw := range kwList {
			kwMap[kw] = true
		}
	}

	mbName := boxName
	mbAttrs, _ := box[0]["attributes"].(string)

	mb := DaptinImapMailBox{
		dbResource:         diu.dbResource,
		name:               mbName,
		sessionUser:        caller,
		mailBoxId:          boxId,
		mailAccountId:      diu.mailAccountId,
		lock:               sync.Mutex{},
		knownKeywords:      kwMap,
		lastKnownMessages:  mbStatus.Messages,
		mailBoxReferenceId: daptinid.InterfaceToDIR(box[0]["reference_id"]).String(),
		info: imap.MailboxInfo{
			Attributes: strings.Split(mbAttrs, ","),
			Delimiter:  "/",
			Name:       mbName,
		},
		status: mbStatus,
	}

	return &mb, nil

}

// GetMailbox returns a mailbox. If it doesn't exist, it returns
// ErrNoSuchMailbox.
func (diu *DaptinImapUser) GetMailbox(name string) (backend.Mailbox, error) {
	log.Printf("[IMAP] GetMailbox(%s): starting", name)
	transaction, err := diu.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		log.Printf("[IMAP] GetMailbox(%s): failed to begin tx: %v", name, err)
		return nil, err
	}
	defer transaction.Rollback()

	result, err := diu.GetMailboxWithTransaction(name, transaction)
	if err != nil {
		return nil, err
	}
	transaction.Commit()
	log.Printf("[IMAP] GetMailbox(%s): returning result=%v", name, result != nil)
	return result, nil
}

// CreateMailbox creates a new mailbox.
//
// If the mailbox already exists, an error must be returned. If the mailbox
// name is suffixed with the server's hierarchy separator character, this is a
// declaration that the client intends to create mailbox names under this name
// in the hierarchy.
//
// If the server's hierarchy separator character appears elsewhere in the
// name, the server SHOULD create any superior hierarchical names that are
// needed for the CREATE command to be successfully completed.  In other
// words, an attempt to create "foo/bar/zap" on a server in which "/" is the
// hierarchy separator character SHOULD create foo/ and foo/bar/ if they do
// not already exist.
//
// If a new mailbox is created with the same name as a mailbox which was
// deleted, its unique identifiers MUST be greater than any unique identifiers
// used in the previous incarnation of the mailbox UNLESS the new incarnation
// has a different unique identifier validity value.
func (diu *DaptinImapUser) CreateMailboxWithTransaction(name string, transaction *sqlx.Tx) error {

	ref, err := GetIdToReferenceIdWithTransaction("mail_account", diu.mailAccountId, transaction)
	if err != nil {
		return err
	}
	_, err = diu.dbResource["mail_box"].CreateMailFolderWithTransaction(ref.String(), name, diu.sessionUser, transaction)
	return err

}

// CreateMailbox creates a new mailbox.
//
// If the mailbox already exists, an error must be returned. If the mailbox
// name is suffixed with the server's hierarchy separator character, this is a
// declaration that the client intends to create mailbox names under this name
// in the hierarchy.
//
// If the server's hierarchy separator character appears elsewhere in the
// name, the server SHOULD create any superior hierarchical names that are
// needed for the CREATE command to be successfully completed.  In other
// words, an attempt to create "foo/bar/zap" on a server in which "/" is the
// hierarchy separator character SHOULD create foo/ and foo/bar/ if they do
// not already exist.
//
// If a new mailbox is created with the same name as a mailbox which was
// deleted, its unique identifiers MUST be greater than any unique identifiers
// used in the previous incarnation of the mailbox UNLESS the new incarnation
// has a different unique identifier validity value.
func (diu *DaptinImapUser) CreateMailbox(name string) error {

	transaction, err := diu.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	err = diu.CreateMailboxWithTransaction(name, transaction)
	if err != nil {
		return err
	}
	return transaction.Commit()

}

// DeleteMailbox permanently remove the mailbox with the given name. It is an
// error to // attempt to delete INBOX or a mailbox name that does not exist.
//
// The DELETE command MUST NOT remove inferior hierarchical names. For
// example, if a mailbox "foo" has an inferior "foo.bar" (assuming "." is the
// hierarchy delimiter character), removing "foo" MUST NOT remove "foo.bar".
//
// The value of the highest-used unique identifier of the deleted mailbox MUST
// be preserved so that a new mailbox created with the same name will not
// reuse the identifiers of the former incarnation, UNLESS the new incarnation
// has a different unique identifier validity value.
func (diu *DaptinImapUser) DeleteMailbox(name string) error {
	if strings.EqualFold(name, "INBOX") {
		return errors.New("cannot delete INBOX")
	}
	return diu.dbResource["mail"].DeleteMailAccountBox(diu.mailAccountId, name, diu.sessionUser)
}

// RenameMailbox changes the name of a mailbox. It is an error to attempt to
// rename from a mailbox name that does not exist or to a mailbox name that
// already exists.
//
// If the name has inferior hierarchical names, then the inferior hierarchical
// names MUST also be renamed.  For example, a rename of "foo" to "zap" will
// rename "foo/bar" (assuming "/" is the hierarchy delimiter character) to
// "zap/bar".
//
// If the server's hierarchy separator character appears in the name, the
// server SHOULD create any superior hierarchical names that are needed for
// the RENAME command to complete successfully.  In other words, an attempt to
// rename "foo/bar/zap" to baz/rag/zowie on a server in which "/" is the
// hierarchy separator character SHOULD create baz/ and baz/rag/ if they do
// not already exist.
//
// The value of the highest-used unique identifier of the old mailbox name
// MUST be preserved so that a new mailbox created with the same name will not
// reuse the identifiers of the former incarnation, UNLESS the new incarnation
// has a different unique identifier validity value.
//
// Renaming INBOX is permitted, and has special behavior.  It moves all
// messages in INBOX to a new mailbox with the given name, leaving INBOX
// empty.  If the server implementation supports inferior hierarchical names
// of INBOX, these are unaffected by a rename of INBOX.
func (diu *DaptinImapUser) RenameMailbox(existingName, newName string) error {
	return diu.dbResource["mail_box"].RenameMailAccountBox(diu.mailAccountId, existingName, newName, diu.sessionUser)

}

// Logout is called when this User will no longer be used, likely because the
// client closed the connection.
func (diu *DaptinImapUser) Logout() error {
	log.Printf("[IMAP] Logout: called for user")
	return nil
}
