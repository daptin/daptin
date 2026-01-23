package resource

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"github.com/daptin/daptin/server/id"
	"io"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/artpar/parsemail"
	"github.com/bjarneh/latinx"
	"github.com/daptin/daptin/server/auth"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/textproto"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"

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
	sequenceToMail     map[uint32]*imap.Message
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
	defer transaction.Commit()
	mbsCurrent, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, transaction)
	dimb.status = mbsCurrent

	mbs := imap.NewMailboxStatus(dimb.name, items)
	mbs.Flags = dimb.status.Flags
	mbs.PermanentFlags = dimb.status.PermanentFlags

	mbs.UnseenSeqNum = dimb.dbResource["mail_box"].GetFirstUnseenMailSequence(dimb.mailBoxId, transaction)
	//vals := make([]interface{}, 0)
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
	defer transaction.Commit()
	return dimb.dbResource["mail_box"].SetMailBoxSubscribed(dimb.mailAccountId, dimb.name, subscribed, transaction)
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
	defer transaction.Commit()
	box, err := dimb.dbResource["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", transaction,
		goqu.Ex{
			"mail_account_id": dimb.mailAccountId,
			"name":            dimb.name,
		},
	)
	if err != nil || len(box) == 0 {
		return err
	}

	dimb.info = imap.MailboxInfo{
		Attributes: strings.Split(box[0]["attributes"].(string), ";"),
		Delimiter:  "\\",
		Name:       box[0]["name"].(string),
	}

	newStatus, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId, transaction)
	newStatus.Name = dimb.name
	dimb.status = newStatus

	return nil
}

// ListMessages returns a list of messages. seqset must be interpreted as UIDs
// if uid is set to true and as message sequence numbers otherwise. See RFC
// 3501 section 6.4.5 for a list of items that can be requested.
//
// Messages must be sent to ch. When the function returns, ch must be closed.
func (dimb *DaptinImapMailBox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {

	for _, seq := range seqset.Set {
		//log.Printf("Fetch request [%v] from %v to %v", uid, seq.Start, seq.Stop)

		seqNo := seq.Start
		var mails []map[string]interface{}
		var err error
		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop, nil)
		} else {
			startAt := seq.Start
			stopAt := seq.Stop

			for {

				if dimb.sequenceToMail[startAt] == nil {
					break
				}

				ch <- dimb.sequenceToMail[startAt]
				startAt = startAt + 1
			}

			if startAt > stopAt {
				continue
			}

			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, startAt, stopAt, nil)
		}

		if err != nil {
			return err
		}

		for _, mailContent := range mails {
			//log.Printf("Return mailContent: %v", mailContent)

			bodyContents, e := base64.StdEncoding.DecodeString(mailContent["mail"].(string))
			if e != nil {
				CheckErr(e, "Failed to decode mail contents")
				continue
			}
			//messageBytes := bytes.NewReader(bodyContents)
			//log.Printf("Message: %v", string(bodyContents))
			//parsedMail, err := parsemail.Parse(messageBytes)

			//messageEntity, err := message.Read(messageBytes)

			if err != nil {
				log.Printf("Failed to parse email body: %v", err)
				continue
			}
			returnMail := imap.NewMessage(seqNo, items)
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
						uid := mailContent["id"].(int64)
						returnMail.Uid = uint32(uid)
					default:
						log.Printf("Fetch default [%v] update flags: %v", subItems, flagList)

						section, err := imap.ParseBodySectionName(subItems)
						if CheckErr(err, "failed to parse item name") {
							skipMail = true
							break
						}

						if !section.Peek {
							if HasAnyFlag(flagList, []string{imap.RecentFlag}) {
								flagList = backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{imap.RecentFlag})
								log.Printf("New flags: [%v]", flagList)
								err := dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), flagList)
								if err != nil {
									log.Printf("Failed to update recent flag for mail[%v]: %v", mailContent["id"], err)
								}
							}
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
						flagList = backendutil.UpdateFlags(flagList, imap.AddFlags, []string{imap.SeenFlag})
						err = dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), flagList)
						CheckErr(err, "Failed to update mail with seen flag")

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

			dimb.sequenceToMail[seqNo] = returnMail
			ch <- returnMail
			seqNo += 1
		}

	}

	close(ch)

	return nil
}

// SearchMessages searches messages. The returned list must contain UIDs if
// uid is set to true, or sequence numbers otherwise.
func (dimb *DaptinImapMailBox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	log.Printf("[IMAP] SearchMessages called uid=%v mailBoxId=%v mailBoxReferenceId=%v", uid, dimb.mailBoxId, dimb.mailBoxReferenceId)
	if dimb.sessionUser != nil {
		log.Printf("[IMAP] SearchMessages sessionUser: id=%v email=%v", dimb.sessionUser.UserId, dimb.sessionUser.UserReferenceId)
	} else {
		log.Printf("[IMAP] SearchMessages sessionUser is NIL")
	}

	httpRequest := &http.Request{}
	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", dimb.sessionUser))

	//filterParams := make(map[string][]string)

	// Always filter by current mailbox - use reference_id for foreign key filter
	queries := []Query{
		{
			ColumnName: "mail_box_id",
			Operator:   "is",
			Value:      dimb.mailBoxReferenceId, // Use reference_id (UUID) for foreign key
		},
	}

	if criteria.Uid != nil && len(criteria.Uid.Set) > 0 {
		setRange := criteria.Uid.Set[0]
		queries = append(queries, Query{
			ColumnName: "id",
			Operator:   "after",
			Value:      setRange.Start - 1,
		}, Query{
			ColumnName: "id",
			Operator:   "before",
			Value:      setRange.Stop + 1,
		})
	}

	if len(criteria.WithFlags) > 0 {
		for _, flag := range criteria.WithFlags {
			switch strings.ToLower(flag) {
			case "\\deleted":
				queries = append(queries, Query{
					ColumnName: "deleted",
					Operator:   "is",
					Value:      true,
				})
			}
		}
	}

	if len(criteria.WithoutFlags) > 0 {
		for _, flag := range criteria.WithFlags {
			switch strings.ToLower(flag) {
			case "\\deleted":
				queries = append(queries, Query{
					ColumnName: "deleted",
					Operator:   "is",
					Value:      false,
				})
			}
		}
	}

	if len(criteria.Header) > 0 {
		for headerName, flag := range criteria.Header {
			switch strings.ToLower(headerName) {
			case "Message-ID":
				queries = append(queries, Query{
					ColumnName: "message_id",
					Operator:   "is",
					Value:      flag,
				})
			}
		}
	}

	queryJson, _ := json.Marshal(queries)

	searchRequest := api2go.Request{
		PlainRequest: httpRequest,
		QueryParams: map[string][]string{
			"fields": {
				"id",
			},
			"query": {
				string(queryJson),
			},
		},
	}

	log.Printf("[IMAP] Search query for mail: %v", searchRequest.QueryParams)
	log.Printf("[IMAP] SearchMessages: attempting to begin transaction")
	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [383]")
		return nil, err
	}
	log.Printf("[IMAP] SearchMessages: transaction started")
	defer transaction.Commit()

	results, _, _, _, err := dimb.dbResource["mail"].PaginatedFindAllWithoutFilters(searchRequest, transaction)

	if err != nil {
		transaction.Rollback()
		return nil, err
	}

	ids := make([]uint32, 0)
	log.Printf("Mail search results: %v", results)
	for i, res := range results {
		if uid {
			id, err := dimb.dbResource["mail"].GetReferenceIdToId("mail",
				daptinid.InterfaceToDIR(res["reference_id"]), transaction)
			if err != nil {
				CheckErr(err, "Failed to get id from reference id")
				continue
			}
			ids = append(ids, uint32(id))
		} else {
			ids = append(ids, uint32(i+1))
		}
	}

	return ids, nil
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

	httpRequest := &http.Request{
		Method: "POST",
	}

	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", dimb.sessionUser))

	apiRequest := api2go.Request{
		PlainRequest: httpRequest,
	}

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

	base64MailContents := base64.StdEncoding.EncodeToString(mailBody)

	parsedmail, err := parsemail.Parse(bytes.NewReader(mailBody))
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

	msgId := parsedmail.MessageID
	if len(msgId) < 1 {
		msgId3, _ := uuid.NewV7()
		msgId = msgId3.String()
	}
	hash := GetMD5Hash(mailBody)

	toAddress := ""
	if len(parsedmail.To) > 0 {
		toAddress = parsedmail.To[0].String()
	}

	replyTo := ""
	if len(parsedmail.ReplyTo) > 0 {
		replyTo = parsedmail.ReplyTo[0].String()
	}

	fromAddress := ""

	if len(parsedmail.From) > 0 {
		fromAddress = parsedmail.From[0].String()
	}

	sender := fromAddress
	if parsedmail.Sender != nil {
		sender = parsedmail.Sender.String()
	}

	// Permission 768 = Owner read (256) + Owner write (512)
	// This ensures only the mail owner can read/write their mail
	model := api2go.NewApi2GoModelWithData("mail", nil, 768, nil, map[string]interface{}{
		"message_id":       parsedmail.MessageID,
		"mail_id":          hash,
		"from_address":     fromAddress,
		"to_address":       toAddress,
		"sender_address":   sender,
		"subject":          parsedmail.Subject,
		"body":             textBody,
		"mail":             base64MailContents,
		"spam_score":       0,
		"hash":             hash,
		"internal_date":    parsedmail.Date,
		"content_type":     messageEntity.Header.Get("Content-Type"),
		"reply_to_address": replyTo,
		"recipient":        toAddress,
		"has_attachment":   len(parsedmail.Attachments),
		"ip_addr":          "",
		"return_path":      "",
		"is_tls":           false,
		"mail_box_id":      dimb.mailBoxReferenceId,
		"user_account_id":  dimb.sessionUser.UserReferenceId.String(),
		"seen":             false,
		"recent":           true,
		"flags":            strings.Join(flags, ","),
		"size":             len(mailBody),
	})
	//txDbResource := NewFromDbResourceWithTransaction(dimb.dbResource["mail"], tx)
	//uidNext, err := txDbResource.GetMailboxNextUid(dimb.mailBoxId)
	//log.Printf("Assign next UID: %v", uidNext)
	//model.Data["uid"] = uidNext
	_, err = dimb.dbResource["mail"].Create(model, apiRequest)
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
	}

	return err
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
	var mails []map[string]interface{}
	var err error
	for _, seq := range seqset.Set {
		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop, nil)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, seq.Start, seq.Stop, nil)
		}

		if err != nil {
			return err
		}

		for _, mailRow := range mails {
			currentFlags := strings.Split(mailRow["flags"].(string), ",")
			newFlags := backendutil.UpdateFlags(currentFlags, operation, flags)
			log.Printf("New flags: [%v]", newFlags)
			hasDupe := false
			fla := map[string]bool{}
			for _, f := range newFlags {
				if fla[f] {
					hasDupe = true
					fla[f] = true
					log.Printf("Duplicate flag: %v", f)
				}
			}

			if hasDupe {
				log.Printf("Duplicate flag: %v", newFlags)
			}
			err = dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailRow["id"].(int64), newFlags)
			if err != nil {
				return err
			}
		}
	}

	return nil
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

	var mails []map[string]interface{}
	var err error

	transaction, err := dimb.dbResource["mail"].Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [644]")
		return err
	}

	destinationMailBoxId, err := dimb.dbResource["mail_box"].GetMailAccountBox(dimb.mailAccountId, dest, transaction)
	if err != nil {
		return err
	}

	req := api2go.Request{
		PlainRequest: &http.Request{},
	}

	for _, set := range seqset.Set {

		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, set.Start, set.Stop, transaction)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, set.Start, set.Stop, transaction)
		}

		if err != nil {
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "Failed to rollback")
			return err
		}

		for _, mail := range mails {
			mail["mail_box_id"] = destinationMailBoxId["reference_id"]

			delete(mail, "reference_id")
			delete(mail, "updated_at")
			delete(mail, "created_at")
			delete(mail, "id")
			mail["recent"] = true
			mailFlags := strings.Split(mail["flags"].(string), ",")
			if !HasAnyFlag(mailFlags, []string{imap.RecentFlag}) {
				mailFlags = backendutil.UpdateFlags(mailFlags, imap.AddFlags, []string{imap.RecentFlag})
				log.Printf("New flags: [%v]", mailFlags)
				mail["flags"] = strings.Join(mailFlags, ",")
			}

			_, err = dimb.dbResource["mail"].CreateWithoutFilter(api2go.NewApi2GoModelWithData(
				"mail", nil, 0, nil, mail), req, transaction)
			if err != nil {
				rollbackErr := transaction.Rollback()
				CheckErr(rollbackErr, "Failed to rollback")
				return err
			}
		}

	}
	transaction.Commit()
	return err
}

// Expunge permanently removes all messages that have the \Deleted flag set
// from the currently selected mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via an expunge update.
func (dimb *DaptinImapMailBox) Expunge() error {

	deleteCount, err := dimb.dbResource["mail_box"].ExpungeMailBox(dimb.mailBoxId)
	log.Printf("%v messages were deleted", deleteCount)

	if err != nil {
		log.Printf("Failed to expunge mails: %v", err)
	}
	return err
}
