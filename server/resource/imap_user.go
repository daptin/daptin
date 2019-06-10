package resource

import (
	"errors"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend"
	"github.com/daptin/daptin/server/auth"
	"gopkg.in/Masterminds/squirrel.v1"
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

	var boxes []backend.Mailbox
	mailBoxes, err := diu.dbResource["mail_box"].GetAllObjectsWithWhere("mail_box", squirrel.Eq{"mail_account_id": diu.mailAccountId})
	if err != nil || len(mailBoxes) == 0 {
		return boxes, err
	}

	for _, box := range mailBoxes {
		if box["user_account_id"] == nil {
			continue
		}
		s := box["user_account_id"].(string)
		mb := DaptinImapMailBox{
			dbResource:         diu.dbResource,
			name:               box["name"].(string),
			sessionUser:        diu.sessionUser,
			mailBoxReferenceId: box["reference_id"].(string),
			mailBoxId:          box["id"].(int64),
			userAccountId:      s,
			info: imap.MailboxInfo{
				Attributes: strings.Split(box["attributes"].(string), ";"),
				Delimiter:  "\\",
				Name:       box["name"].(string),
			},
		}
		boxes = append(boxes, &mb)
	}

	return boxes, nil
}

// GetMailbox returns a mailbox. If it doesn't exist, it returns
// ErrNoSuchMailbox.
func (diu *DaptinImapUser) GetMailbox(name string) (backend.Mailbox, error) {

	box, err := diu.dbResource["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
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

	mbStatus, err := diu.dbResource["mail_box"].GetMailBoxStatus(diu.mailAccountId, box[0]["id"].(int64))
	if err != nil {
		return nil, err
	}

	mbStatus.Name = box[0]["name"].(string)
	mbStatus.Flags = strings.Split(box[0]["flags"].(string), ",")
	mbStatus.PermanentFlags = strings.Split(box[0]["permanent_flags"].(string), ",")

	mb := DaptinImapMailBox{
		dbResource:         diu.dbResource,
		name:               box[0]["name"].(string),
		sessionUser:        diu.sessionUser,
		mailBoxId:          box[0]["id"].(int64),
		lock:               sync.Mutex{},
		mailBoxReferenceId: box[0]["reference_id"].(string),
		info: imap.MailboxInfo{
			Attributes: strings.Split(box[0]["attributes"].(string), ","),
			Delimiter:  "\\",
			Name:       box[0]["name"].(string),
		},
		status: *mbStatus,
	}

	return &mb, nil

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

	box, err := diu.dbResource["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
			"mail_account_id": diu.mailAccountId,
			"name":            name,
		},
	)
	if len(box) > 1 {
		return errors.New("mailbox already exists")
	}

	mailAccount, err := diu.dbResource["mail_box"].GetUserMailAccountRowByEmail(diu.username)

	_, err = diu.dbResource["mail_box"].CreateMailAccountBox(
		mailAccount["reference_id"].(string),
		diu.sessionUser,
		name)

	return err

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
	return diu.dbResource["mail"].DeleteMailAccountBox(diu.mailAccountId, name)
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
	return diu.dbResource["mail_box"].RenameMailAccountBox(diu.mailAccountId, existingName, newName)

}

// Logout is called when this User will no longer be used, likely because the
// client closed the connection.
func (diu *DaptinImapUser) Logout() error {
	return nil
}
