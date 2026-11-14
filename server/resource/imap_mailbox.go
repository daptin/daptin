package resource

import (
	"bufio"
	"bytes"
	"context"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/textproto"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"strings"
	"sync"
	"time"
)

type DaptinImapMailBox struct {
	name               string
	sessionUser        *auth.SessionUser
	dbResource         map[string]*DbResource
	mailAccountId      int64
	lock               sync.Mutex
	mailBoxId          int64
	mailBoxReferenceId string
	info               imap.MailboxInfo
	status             *imap.MailboxStatus
	lastKnownMessages  uint32
	pendingUpdate      *imap.MailboxStatus
	knownKeywords      map[string]bool
}

const imapResourceReadBatchSize = 200

func (dimb *DaptinImapMailBox) authorizeRead(transaction *sqlx.Tx) (map[string]interface{}, *auth.SessionUser, error) {
	boxRef, err := mailReference(dimb.mailBoxReferenceId)
	if err != nil {
		return nil, nil, err
	}
	box, caller, err := dimb.dbResource["mail"].readMailBoxForSession(boxRef, dimb.sessionUser, transaction)
	if err != nil {
		return nil, nil, err
	}
	accountID, err := dimb.dbResource["mail_account"].GetReferenceIdToId("mail_account", daptinid.InterfaceToDIR(box["mail_account_id"]), transaction)
	if err != nil || accountID != dimb.mailAccountId {
		return nil, nil, errors.New("selected mailbox changed account")
	}
	return box, caller, nil
}

func (dimb *DaptinImapMailBox) readSelectedMails(selections []MailSelection, includeBody bool,
	transaction *sqlx.Tx) (map[daptinid.DaptinReferenceId]map[string]interface{}, error) {
	_, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return nil, err
	}
	result := make(map[daptinid.DaptinReferenceId]map[string]interface{}, len(selections))
	for start := 0; start < len(selections); start += imapResourceReadBatchSize {
		stop := start + imapResourceReadBatchSize
		if stop > len(selections) {
			stop = len(selections)
		}

		referenceIDs := make([]string, 0, stop-start)
		for _, selection := range selections[start:stop] {
			referenceIDs = append(referenceIDs, selection.ReferenceID.String())
		}

		requestURL, _ := url.Parse("/api/mail")
		requestContext := context.WithValue(context.Background(), "user", caller)
		if start > 0 {
			requestContext = WithMeteringInternal(requestContext)
		}
		request := api2go.Request{
			PlainRequest: (&http.Request{Method: http.MethodGet, URL: requestURL}).WithContext(requestContext),
		}
		filters, err := encodingjson.Marshal([]Query{
			{ColumnName: "reference_id", Operator: "in", Value: referenceIDs},
			{ColumnName: "mail_box_id", Operator: "=", Value: dimb.mailBoxReferenceId},
		})
		if err != nil {
			return nil, err
		}
		request.QueryParams = map[string][]string{"query": {string(filters)}, "page[size]": {fmt.Sprint(imapResourceReadBatchSize)}, "page[number]": {"1"}}
		if includeBody {
			request.QueryParams["included_relations"] = []string{"mail"}
		}
		// The normal resource read pipeline applies current row permissions in
		// the bounded metadata query before MIME hydration and metering.
		_, response, err := dimb.dbResource["mail"].PaginatedFindAllWithTransaction(request, transaction)
		if err != nil {
			return nil, err
		}
		models, ok := response.Result().([]api2go.Api2GoModel)
		if !ok {
			return nil, errors.New("native mail read returned invalid rows")
		}
		if len(models) != len(referenceIDs) {
			return nil, mailDraftError(403, "selected messages are no longer readable")
		}
		for _, model := range models {
			attributes := model.GetAttributes()
			attributes["reference_id"] = model.GetID()

			referenceID := daptinid.InterfaceToDIR(attributes["reference_id"])
			if referenceID == daptinid.NullReferenceId {
				continue
			}
			attributes["reference_id"] = referenceID
			result[referenceID] = attributes
		}
	}
	return result, nil
}

// ConsumePollUpdate returns the pending mailbox status if Poll() found new
// messages, and clears the pending state. Returns nil if no update is pending.
func (dimb *DaptinImapMailBox) ConsumePollUpdate() *imap.MailboxStatus {
	dimb.lock.Lock()
	defer dimb.lock.Unlock()
	if dimb.pendingUpdate != nil {
		st := dimb.pendingUpdate
		dimb.pendingUpdate = nil
		dimb.lastKnownMessages = st.Messages
		return st
	}
	return nil
}

// Name returns this mailbox name.
func (dimb *DaptinImapMailBox) Name() string {
	return dimb.name
}

func init() {
}

// Info returns this mailbox info.
func (dimb *DaptinImapMailBox) Info() (*imap.MailboxInfo, error) {
	return &dimb.info, nil
}

// Status returns this mailbox status. The fields Name, Flags, PermanentFlags
// and UnseenSeqNum in the returned MailboxStatus must be always populated.
// This function does not affect the state of any messages in the mailbox. See
// RFC 3501 section 6.3.10 for a list of items that can be requested.
func (dimb *DaptinImapMailBox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {

	//iMap := make(map[imap.StatusItem]bool)

	//for _, item := range items {
	//	iMap[item] = true
	//}

	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	if _, _, err = dimb.authorizeRead(transaction); err != nil {
		return nil, err
	}
	mbsCurrent, err := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, transaction)
	if err != nil || mbsCurrent == nil {
		return nil, fmt.Errorf("failed to get mailbox status: %w", err)
	}
	// Preserve base Flags/PermanentFlags (GetMailBoxStatus doesn't set them)
	var baseFlags, basePermanentFlags []string
	if dimb.status != nil {
		baseFlags = dimb.status.Flags
		basePermanentFlags = dimb.status.PermanentFlags
	}
	dimb.status = mbsCurrent
	dimb.status.Flags = baseFlags
	dimb.status.PermanentFlags = basePermanentFlags

	mbs := imap.NewMailboxStatus(dimb.name, items)

	// Include cached keywords in FLAGS so clients know they exist
	dimb.lock.Lock()
	keywords := make([]string, 0, len(dimb.knownKeywords))
	for k := range dimb.knownKeywords {
		keywords = append(keywords, k)
	}
	dimb.lock.Unlock()
	mbs.Flags = append(append([]string{}, dimb.status.Flags...), keywords...)
	mbs.PermanentFlags = append(append([]string{}, dimb.status.PermanentFlags...), keywords...)
	mbs.PermanentFlags = append(mbs.PermanentFlags, "\\*")

	mbs.UnseenSeqNum = dimb.dbResource["mail_box"].GetFirstUnseenMailSequence(dimb.mailBoxId, transaction)
	for _, item := range items {
		switch imap.StatusItem(item) {
		case imap.StatusMessages:
			mbs.Messages = dimb.status.Messages
		case imap.StatusRecent:
			mbs.Recent = dimb.status.Recent
		case imap.StatusUnseen:
			mbs.Unseen = dimb.status.Unseen
		case imap.StatusUidNext:
			nextUid, _ := dimb.dbResource["mail_box"].GetMailboxNextUid(dimb.mailBoxId, transaction)
			mbs.UidNext = nextUid
		case imap.StatusUidValidity:
			mbs.UidValidity = dimb.status.UidValidity
		}
	}
	return mbs, nil
}

// SetSubscribed adds or removes the mailbox to the server's set of "active"
// or "subscribed" mailboxes.
func (dimb *DaptinImapMailBox) SetSubscribed(subscribed bool) error {
	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	err = dimb.dbResource["mail_box"].SetMailBoxSubscribed(dimb.mailAccountId, dimb.name, subscribed, dimb.sessionUser, transaction)
	if err != nil {
		return err
	}
	return transaction.Commit()
}

// Check requests a checkpoint of the currently selected mailbox. A checkpoint
// refers to any implementation-dependent housekeeping associated with the
// mailbox (e.g., resolving the server's in-memory state of the mailbox with
// the state on its disk). A checkpoint MAY take a non-instantaneous amount of
// real time to complete. If a server implementation has no such housekeeping
// considerations, CHECK is equivalent to NOOP.
func (dimb *DaptinImapMailBox) Check() error {

	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	current, _, err := dimb.authorizeRead(transaction)
	if err != nil {
		return err
	}
	box := []map[string]interface{}{current}

	attrs, _ := box[0]["attributes"].(string)
	dimb.info = imap.MailboxInfo{
		Attributes: strings.Split(attrs, ";"),
		Delimiter:  "/",
		Name:       box[0]["name"].(string),
	}

	newStatus, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, transaction)
	newStatus.Name = dimb.name
	dimb.status = newStatus

	return transaction.Commit()
}

// ListMessages returns a list of messages. seqset must be interpreted as UIDs
// if uid is set to true and as message sequence numbers otherwise. See RFC
// 3501 section 6.4.5 for a list of items that can be requested.
//
// Messages must be sent to ch. When the function returns, ch must be closed.
func (dimb *DaptinImapMailBox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	selections, err := dimb.dbResource["mail"].SelectMailBoxCandidates(dimb.mailBoxId, uid, seqset, transaction)
	if err != nil {
		return err
	}
	mails, err := dimb.readSelectedMails(selections, true, transaction)
	if err != nil {
		return err
	}

	for _, selection := range selections {
		mailContent, ok := mails[selection.ReferenceID]
		if !ok {
			continue
		}
		//log.Printf("Return mailContent: %v", mailContent)

		bodyContents, e := dimb.dbResource["mail"].MailColumnBytes("mail", "mail", mailContent["mail"])
		if e != nil {
			CheckErr(e, "Failed to read mail contents")
			continue
		}
		//messageBytes := bytes.NewReader(bodyContents)
		//log.Printf("Message: %v", string(bodyContents))
		//parsedMail, err := parsemail.Parse(messageBytes)

		//messageEntity, err := message.Read(messageBytes)

		returnMail := imap.NewMessage(selection.SequenceNumber, items)
		returnMail.Size = uint32(mailContent["size"].(int64))

		skipMail := false

		//responseItems := make([]interface{}, 0)
		for _, item1 := range items {

			for _, subItems := range item1.Expand() {

				if skipMail {
					break
				}

				flagList := strings.Split(mailContent["flags"].(string), ",")
				//log.Printf("Mail flags: %v at fetch item [%v]", flagList, subItems)

				switch subItems {
				case imap.FetchEnvelope:

					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, _ := textproto.ReadHeader(bodyReader)

					enve, err := backendutil.FetchEnvelope(header)
					if err != nil {
						log.Printf("Failed to fetch envelop for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}
					returnMail.Envelope = enve
				case imap.FetchBodyStructure:
					log.Printf("Fetch Body [%v] update flags: ", subItems == imap.FetchBodyStructure)
					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, err := textproto.ReadHeader(bodyReader)

					bs, err := backendutil.FetchBodyStructure(header, bodyReader, subItems == imap.FetchBodyStructure)
					if err != nil {
						log.Printf("Failed to fetch body structure for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}
					returnMail.BodyStructure = bs
				case imap.FetchFlags:
					returnMail.Flags = flagList

				case imap.FetchInternalDate:
					returnMail.InternalDate = mailContent["internal_date"].(time.Time)
				case imap.FetchRFC822Size:
					returnMail.Size = uint32(mailContent["size"].(int64))
				case imap.FetchUid:
					returnMail.Uid = selection.UID
				default:
					log.Printf("Fetch default [%v] update flags: %v", subItems, flagList)

					section, err := imap.ParseBodySectionName(subItems)
					if CheckErr(err, "failed to parse item name") {
						skipMail = true
						break
					}

					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, err := textproto.ReadHeader(bodyReader)

					log.Printf("Fetch default section peek [%v]: %v", section, section.Peek)

					l, err := backendutil.FetchBodySection(header, bodyReader, section)
					if err != nil {
						log.Printf("Failed to fetch body section for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}

					if !section.Peek {
						// Set \Seen flag only on non-PEEK fetch
						flagList = backendutil.UpdateFlags(flagList, imap.AddFlags, []string{imap.SeenFlag})
						err = dimb.dbResource["mail"].MutateMailFlags(
							daptinid.InterfaceToDIR(mailContent["reference_id"]), imap.AddFlags, []string{imap.SeenFlag}, dimb.sessionUser, transaction)
						if err != nil {
							return err
						}
					}

					returnMail.Body[section] = l
					//responseItems = append(responseItems, string(item), l)
				}
			}
		}

		if skipMail {
			continue
		}
		//err = returnMail.Parse(responseItems)
		//if err != nil {
		//	log.Printf("Failed to parse fields: %v", err)
		//	continue
		//}

		ch <- returnMail
	}

	return transaction.Commit()
}

// SearchMessages searches messages. The returned list must contain UIDs if
// uid is set to true, or sequence numbers otherwise.
func (dimb *DaptinImapMailBox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	selections, err := dimb.dbResource["mail"].SearchMailBoxCandidates(dimb.mailBoxId, criteria, transaction)
	if err != nil {
		return nil, err
	}
	mails, err := dimb.readSelectedMails(selections, false, transaction)
	if err != nil {
		return nil, err
	}
	result := make([]uint32, 0, len(mails))
	for _, selection := range selections {
		if _, ok := mails[selection.ReferenceID]; !ok {
			continue
		}
		if uid {
			result = append(result, selection.UID)
		} else {
			result = append(result, selection.SequenceNumber)
		}
	}
	if err := transaction.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateMessage appends a new message to this mailbox. The \Recent flag will
// be added no matter flags is empty or not. If date is nil, the current time
// will be used.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (dimb *DaptinImapMailBox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	raw, err := io.ReadAll(io.LimitReader(body, 25*1024*1024+1))
	if err != nil {
		return err
	}
	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if !HasFlag(flags, imap.RecentFlag) {
		flags = backendutil.UpdateFlags(flags, imap.AddFlags, []string{imap.RecentFlag})
	}
	_, err = dimb.dbResource["mail"].appendMailMessageWithTransaction(map[string]interface{}{
		"id": dimb.mailBoxId, "reference_id": dimb.mailBoxReferenceId,
	}, raw, flags, date, dimb.sessionUser, transaction)
	if err != nil {
		return err
	}
	return transaction.Commit()
}

func HasFlag(flags []string, flagToFind string) bool {

	flagToFind = strings.ToLower(flagToFind)
	for _, f := range flags {
		if strings.ToLower(f) == flagToFind {
			return true
		}
	}

	return false
}

func HasAnyFlag(flags []string, flagToFind []string) bool {

	log.Printf("Check for flags [%v] in [%v]", flagToFind, flags)
	for _, f := range flags {
		f = strings.ToLower(f)
		for _, f1 := range flagToFind {
			if strings.ToLower(f1) == f {
				return true
			}
		}
	}

	log.Printf("[%v] not found in [%v]", flagToFind, flags)
	return false
}

// UpdateMessagesFlags alters flags for the specified message(s).
//
// If the Backend implements Updater, it must notify the client immediately
// via a message update.
func (dimb *DaptinImapMailBox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error {

	log.Printf("Update messages flags: [%v] :[%v]: %v", seqset, operation, flags)

	// Track new keywords when flags are being set or added
	if operation == imap.SetFlags || operation == imap.AddFlags {
		dimb.lock.Lock()
		for _, f := range flags {
			f = strings.TrimSpace(f)
			if f != "" && f[0] != '\\' {
				dimb.knownKeywords[f] = true
			}
		}
		dimb.lock.Unlock()
	}

	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	selections, err := dimb.dbResource["mail"].SelectMailBoxCandidates(dimb.mailBoxId, uid, seqset, transaction)
	if err != nil {
		return err
	}
	mails, err := dimb.readSelectedMails(selections, false, transaction)
	if err != nil {
		return err
	}
	for _, selection := range selections {
		mailRow, ok := mails[selection.ReferenceID]
		if !ok {
			continue
		}
		err = dimb.dbResource["mail"].MutateMailFlags(
			daptinid.InterfaceToDIR(mailRow["reference_id"]), operation, flags, dimb.sessionUser, transaction)
		if err != nil {
			return err
		}
	}

	return transaction.Commit()
}

// CopyMessages copies the specified message(s) to the end of the specified
// destination mailbox. The flags and internal date of the message(s) SHOULD
// be preserved, and the Recent flag SHOULD be set, in the copy.
//
// If the destination mailbox does not exist, a server SHOULD return an error.
// It SHOULD NOT automatically create the mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (dimb *DaptinImapMailBox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {

	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [644]")
		return err
	}
	defer transaction.Rollback()

	destinationMailBoxId, err := dimb.dbResource["mail_box"].GetMailAccountBox(dimb.mailAccountId, dest, transaction)
	if err != nil {
		return err
	}

	selections, err := dimb.dbResource["mail"].SelectMailBoxCandidates(dimb.mailBoxId, uid, seqset, transaction)
	if err != nil {
		return err
	}
	mails, err := dimb.readSelectedMails(selections, true, transaction)
	if err != nil {
		return err
	}
	for _, selection := range selections {
		mail, ok := mails[selection.ReferenceID]
		if !ok {
			continue
		}
		raw, err := dimb.dbResource["mail"].MailColumnBytes("mail", "mail", mail["mail"])
		if err != nil {
			return err
		}
		flags := canonicalMailFlags(strings.Split(StringOrEmpty(mail["flags"]), ","))
		flags = backendutil.UpdateFlags(flags, imap.AddFlags, []string{imap.RecentFlag})
		date, _ := mail["internal_date"].(time.Time)
		if date.IsZero() {
			date, _ = time.Parse(time.RFC3339, StringOrEmpty(mail["internal_date"]))
		}
		// A COPY creates a new canonical record. Only MIME, flags and date are
		// copied; compose/submission identities belong to the original record.
		if _, err = dimb.dbResource["mail"].appendMailMessageWithTransaction(destinationMailBoxId, raw, flags, date, dimb.sessionUser, transaction); err != nil {
			return err
		}

	}
	return transaction.Commit()
}

// Expunge permanently removes all messages that have the \Deleted flag set
// from the currently selected mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via an expunge update.
func (dimb *DaptinImapMailBox) Expunge() error {

	deleteCount, err := dimb.dbResource["mail_box"].ExpungeMailBox(dimb.mailBoxId, dimb.sessionUser)
	log.Printf("%v messages were deleted", deleteCount)

	if err != nil {
		log.Printf("Failed to expunge mails: %v", err)
	}

	return err
}

// Poll implements backend.MailboxPoller. Called by go-imap on NOOP to check
// for new messages and send EXISTS updates to the client.
func (dimb *DaptinImapMailBox) Poll() error {
	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, _, err = dimb.authorizeRead(transaction); err != nil {
		return err
	}
	status, err := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, transaction)
	if err != nil {
		return err
	}

	// Only signal update when count increases to avoid backwards EXISTS (Issue 6)
	if status.Messages > dimb.lastKnownMessages {
		status.Name = dimb.name
		dimb.lock.Lock()
		dimb.pendingUpdate = status
		dimb.status = status
		dimb.lock.Unlock()
	}

	// Clear recent flags (write operation, uses db directly)
	if err = dimb.dbResource["mail_box"].ClearRecentFlags(dimb.mailBoxId, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}
