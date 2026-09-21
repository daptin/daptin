package resource

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/artpar/parsemail"
	"github.com/bjarneh/latinx"
	"github.com/daptin/daptin/server/auth"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/textproto"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"strings"
	"sync"
	"time"
	"unicode/utf8"
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

func (dimb *DaptinImapMailBox) readSelectedMails(selections []MailSelection, includeBody bool,
	transaction *sqlx.Tx) (map[daptinid.DaptinReferenceId]map[string]interface{}, error) {
	result := make(map[daptinid.DaptinReferenceId]map[string]interface{}, len(selections))
	for start := 0; start < len(selections); start += imapResourceReadBatchSize {
		stop := start + imapResourceReadBatchSize
		if stop > len(selections) {
			stop = len(selections)
		}

		referenceIDs := make([]daptinid.DaptinReferenceId, 0, stop-start)
		for _, selection := range selections[start:stop] {
			referenceIDs = append(referenceIDs, selection.ReferenceID)
		}

		requestURL, _ := url.Parse("/api/mail")
		requestContext := context.WithValue(context.Background(), "user", dimb.sessionUser)
		if start > 0 {
			requestContext = WithMeteringInternal(requestContext)
		}
		request := api2go.Request{
			PlainRequest: (&http.Request{Method: http.MethodGet, URL: requestURL}).WithContext(requestContext),
		}
		var includedRelations map[string]bool
		if includeBody {
			includedRelations = map[string]bool{"mail": true}
		}

		rows, err := dimb.dbResource["mail"].readByReferenceIDsAfterAuthorizationWithTransaction(
			referenceIDs, includedRelations, request, transaction)
		if err != nil {
			return nil, err
		}
		for _, attributes := range rows {
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

	// Use direct queries (no transaction) for read-only Status to reduce lock contention
	mbsCurrent, err := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, nil)
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

	mbs.UnseenSeqNum = dimb.dbResource["mail_box"].GetFirstUnseenMailSequence(dimb.mailBoxId, nil)
	for _, item := range items {
		switch imap.StatusItem(item) {
		case imap.StatusMessages:
			mbs.Messages = dimb.status.Messages
		case imap.StatusRecent:
			mbs.Recent = dimb.status.Recent
		case imap.StatusUnseen:
			mbs.Unseen = dimb.status.Unseen
		case imap.StatusUidNext:
			nextUid, _ := dimb.dbResource["mail_box"].GetMailboxNextUid(dimb.mailBoxId, nil)
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
	box, err := dimb.dbResource["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": dimb.mailAccountId,
			"name":            dimb.name,
		},
	)
	if err != nil || len(box) == 0 {
		return err
	}

	attrs, _ := box[0]["attributes"].(string)
	dimb.info = imap.MailboxInfo{
		Attributes: strings.Split(attrs, ";"),
		Delimiter:  "\\",
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
					if err != nil || l.Len() == 0 {
						log.Printf("Failed to fetch body section for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}

					if !section.Peek {
						// Remove \Recent flag on non-PEEK fetch
						if HasAnyFlag(flagList, []string{imap.RecentFlag}) {
							flagList = backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{imap.RecentFlag})
						}
						// Set \Seen flag only on non-PEEK fetch
						flagList = backendutil.UpdateFlags(flagList, imap.AddFlags, []string{imap.SeenFlag})
						err = dimb.dbResource["mail"].UpdateMailFlags(
							daptinid.InterfaceToDIR(mailContent["reference_id"]), flagList, dimb.sessionUser, transaction)
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

	mailBody, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	requestURL, _ := url.Parse("/api/mail")
	httpRequest := &http.Request{Method: http.MethodPost, URL: requestURL}

	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", dimb.sessionUser))

	apiRequest := api2go.Request{
		PlainRequest: httpRequest,
	}

	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	if !HasAnyFlag(flags, []string{imap.RecentFlag}) {
		flags = backendutil.UpdateFlags(flags, imap.AddFlags, []string{imap.RecentFlag})
		log.Printf("New flags: [%v]", flags)
	}

	messageEntity, err := message.Read(bytes.NewReader(mailBody))
	if err != nil {
		return err
	}

	//enve, _ := backendutil.FetchEnvelope(messageEntity.Header)
	//mailContents, err := io.ReadAll(messageEntity.Body)
	//if err != nil {
	//	return err
	//}
	//mailContents = mailContents[0:n]

	hash := GetMD5Hash(mailBody)
	storedMailContents := dimb.dbResource["mail"].MailColumnValue("mail", "mail", mailBody, hash)

	parsedmail, err := parsemail.Parse(bytes.NewReader(mailBody))
	if err != nil {
		return err
	}
	//log.Printf("%v length of the new message", len(mailBody), parsedmail.From, parsedmail.Subject)

	textBody := parsedmail.TextBody
	if strings.Index(strings.ToLower(parsedmail.Header.Get("Content-type")), "iso-8859-1") > -1 {
		converter := latinx.Get(latinx.ISO_8859_1)
		textBodyBytes, err := converter.Decode([]byte(textBody))
		if err != nil {
			log.Printf("Failed to convert iso 8859 to utf8: %v", err)
		}
		textBody = string(textBodyBytes)
	}

	mailDate, _, err := fieldtypes.GetDateTime(parsedmail.Header.Get("Date"))
	if err != nil {
		log.Printf("Failed to parse mail date: %s == %v", parsedmail.Header.Get("Date"), err)
	} else {
		parsedmail.Date = mailDate
	}
	searchMetadata := mailSearchMetadataFromParsed(parsedmail)
	searchMetadata.Body = textBody

	msgId := searchMetadata.MessageID
	if len(msgId) < 1 {
		msgId3, _ := uuid.NewV7()
		msgId = msgId3.String()
	}
	uid, err := dimb.dbResource["mail_box"].AllocateMailBoxUid(dimb.mailBoxId, transaction)
	if err != nil {
		return err
	}

	if date.IsZero() {
		date = time.Now()
	}

	// Permission 768 = Owner read (256) + Owner write (512)
	// This ensures only the mail owner can read/write their mail
	model := api2go.NewApi2GoModelWithData("mail", nil, 768, nil, map[string]interface{}{
		"message_id":       msgId,
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
		"hash":             hash,
		"internal_date":    date,
		"sent_date":        nullableMailDate(searchMetadata.SentDate),
		"content_type":     messageEntity.Header.Get("Content-Type"),
		"reply_to_address": searchMetadata.ReplyTo,
		"recipient":        searchMetadata.To,
		"has_attachment":   len(parsedmail.Attachments),
		"ip_addr":          "",
		"return_path":      "",
		"is_tls":           false,
		"mail_box_id":      dimb.mailBoxReferenceId,
		"user_account_id":  dimb.sessionUser.UserReferenceId.String(),
		"uid":              uid,
		"seen":             HasAnyFlag(flags, []string{imap.SeenFlag}),
		"recent":           true,
		"flags":            strings.Join(flags, ","),
		"size":             len(mailBody),
	})
	//txDbResource := NewFromDbResourceWithTransaction(dimb.dbResource["mail"], tx)
	//uidNext, err := txDbResource.GetMailboxNextUid(dimb.mailBoxId)
	//log.Printf("Assign next UID: %v", uidNext)
	//model.Data["uid"] = uidNext
	_, err = dimb.dbResource["mail"].createAfterAuthorizationWithTransaction(model, apiRequest, transaction)
	//log.Printf("UID size [%s]", len(mailBody))

	//if err != nil {
	//	log.Printf("Failed to create email: %v", err)
	//	err = tx.Rollback()
	//} else {
	//	err = tx.Commit()
	//}

	if err != nil {
		log.Println(utf8.ValidString(parsedmail.TextBody))
		log.Printf("Failed to insert: %v", parsedmail.TextBody)
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
		currentFlags := strings.Split(mailRow["flags"].(string), ",")
		newFlags := backendutil.UpdateFlags(currentFlags, operation, flags)
		log.Printf("New flags: [%v]", newFlags)
		// Deduplicate flags
		fla := map[string]bool{}
		dedupedFlags := make([]string, 0, len(newFlags))
		for _, f := range newFlags {
			if !fla[f] {
				fla[f] = true
				dedupedFlags = append(dedupedFlags, f)
			}
		}
		newFlags = dedupedFlags
		err = dimb.dbResource["mail"].UpdateMailFlags(
			daptinid.InterfaceToDIR(mailRow["reference_id"]), newFlags, dimb.sessionUser, transaction)
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

	requestURL, _ := url.Parse("/api/mail")
	httpRequest := (&http.Request{
		Method: http.MethodPost,
		URL:    requestURL,
	}).WithContext(context.WithValue(context.Background(), "user", dimb.sessionUser))
	req := api2go.Request{
		PlainRequest: httpRequest,
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
		uid, err := dimb.dbResource["mail_box"].AllocateMailBoxUid(destinationMailBoxId["id"].(int64), transaction)
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			return err
		}
		mailBytes, err := dimb.dbResource["mail"].MailColumnBytes("mail", "mail", mail["mail"])
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			return err
		}
		storageKey, _ := mail["hash"].(string)
		if storageKey == "" {
			storageKey, _ = mail["mail_id"].(string)
		}
		mail["mail"] = dimb.dbResource["mail"].MailColumnValue("mail", "mail", mailBytes, storageKey)
		mail["mail_box_id"] = destinationMailBoxId["reference_id"]

		delete(mail, "reference_id")
		delete(mail, "updated_at")
		delete(mail, "created_at")
		delete(mail, "id")
		mail["uid"] = uid
		mail["recent"] = true
		mailFlags := strings.Split(mail["flags"].(string), ",")
		if !HasAnyFlag(mailFlags, []string{imap.RecentFlag}) {
			mailFlags = backendutil.UpdateFlags(mailFlags, imap.AddFlags, []string{imap.RecentFlag})
			log.Printf("New flags: [%v]", mailFlags)
			mail["flags"] = strings.Join(mailFlags, ",")
		}
		_, err = dimb.dbResource["mail"].createAfterAuthorizationWithTransaction(api2go.NewApi2GoModelWithData(
			"mail", nil, 768, nil, mail), req, transaction)
		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
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
	// Read status without transaction to reduce lock contention
	status, err := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, nil)
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
	err = dimb.dbResource["mail_box"].ClearRecentFlags(dimb.mailBoxId, nil)
	return err
}
