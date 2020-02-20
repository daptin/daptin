package resource

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/artpar/go.uuid"
	"github.com/artpar/parsemail"
	"github.com/bjarneh/latinx"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/columntypes"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/textproto"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"

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

	mbsCurrent, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId)
	dimb.status = mbsCurrent

	mbs := imap.NewMailboxStatus(dimb.name, items)
	mbs.Flags = dimb.status.Flags
	mbs.PermanentFlags = dimb.status.PermanentFlags

	mbs.UnseenSeqNum = dimb.dbResource["mail_box"].GetFirstUnseenMailSequence(dimb.mailBoxId)
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
			nextUid, _ := dimb.dbResource["mail_box"].GetMailboxNextUid(dimb.mailBoxId)
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
	return dimb.dbResource["mail_box"].SetMailBoxSubscribed(dimb.mailAccountId, dimb.name, subscribed)
}

// Check requests a checkpoint of the currently selected mailbox. A checkpoint
// refers to any implementation-dependent housekeeping associated with the
// mailbox (e.g., resolving the server's in-memory state of the mailbox with
// the state on its disk). A checkpoint MAY take a non-instantaneous amount of
// real time to complete. If a server implementation has no such housekeeping
// considerations, CHECK is equivalent to NOOP.
func (dimb *DaptinImapMailBox) Check() error {

	box, err := dimb.dbResource["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
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

	newStatus, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId)
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
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop)
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

			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, startAt, stopAt)
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
			for _, item := range items {

				if skipMail {
					break
				}

				switch item {
				case imap.FetchEnvelope:

					flagList := strings.Split(mailContent["flags"].(string), ",")
					log.Printf("Mail flags: %v", flagList)
					if HasFlag(flagList, imap.RecentFlag) {
						newFlags := backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{imap.RecentFlag})
						err := dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), newFlags)
						if err != nil {
							log.Printf("Failed to update recent flag for mail[%v]: %v", mailContent["id"], err)
						}
					}
					if HasFlag(flagList, "RECENT") {
						newFlags := backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{"RECENT"})
						err := dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), newFlags)
						if err != nil {
							log.Printf("Failed to update recent flag for mail[%v]: %v", mailContent["id"], err)
						}
					}
					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, _ := textproto.ReadHeader(bodyReader)

					enve, err := backendutil.FetchEnvelope(header)
					if err != nil {
						log.Printf("Failed to fetch envelop for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}
					returnMail.Envelope = enve
				case imap.FetchBody, imap.FetchBodyStructure:
					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, err := textproto.ReadHeader(bodyReader)

					if item == imap.FetchBody {
						flagList := strings.Split(mailContent["flags"].(string), ",")
						if HasFlag(flagList, imap.RecentFlag) {
							newFlags := backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{imap.RecentFlag})
							err := dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), newFlags)
							if err != nil {
								log.Printf("Failed to update recent flag for mail[%v]: %v", mailContent["id"], err)
							}
						}

					}
					bs, err := backendutil.FetchBodyStructure(header, bodyReader, item == imap.FetchBodyStructure)
					if err != nil {
						log.Printf("Failed to fetch body structure for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}
					returnMail.BodyStructure = bs
				case imap.FetchFlags:
					flagList := strings.Split(mailContent["flags"].(string), ",")
					returnMail.Flags = flagList

				case imap.FetchInternalDate:
					returnMail.InternalDate = mailContent["internal_date"].(time.Time)
				case imap.FetchRFC822Size:
					returnMail.Size = uint32(mailContent["size"].(int64))
				case imap.FetchUid:
					uid := mailContent["id"].(int64)
					returnMail.Uid = uint32(uid)
				default:

					bodyReader := bufio.NewReader(bytes.NewReader(bodyContents))
					header, err := textproto.ReadHeader(bodyReader)

					section, err := imap.ParseBodySectionName(item)
					if err != nil {
						log.Printf("Failed to fetch structure for email [%v] == %v", mailContent["id"], err)
						skipMail = true
						break
					}

					if !section.Peek {
						flagList := strings.Split(mailContent["flags"].(string), ",")
						if HasFlag(flagList, imap.RecentFlag) {
							newFlags := backendutil.UpdateFlags(flagList, imap.RemoveFlags, []string{imap.RecentFlag})
							err := dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailContent["id"].(int64), newFlags)
							if err != nil {
								log.Printf("Failed to update recent flag for mail[%v]: %v", mailContent["id"], err)
							}
						}
					}

					l, err := backendutil.FetchBodySection(header, bodyReader, section)
					if err != nil || l.Len() == 0 {
						log.Printf("Failed to fetch body section for email [%v] == %v", mailContent["id"], err)
						// skipMail = true
						// break
					}

					returnMail.Body[section] = l
					//responseItems = append(responseItems, string(item), l)
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

	httpRequest := http.Request{}

	//filterParams := make(map[string][]string)

	queries := make([]Query, 0)

	if criteria.Uid != nil {
		queries = append(queries, Query{
			ColumnName: "id",
			Operator:   "contains",
			Value:      criteria.Uid.Set,
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

	queryJson, _ := json.Marshal(queries)

	searchRequest := api2go.Request{
		PlainRequest: &httpRequest,
		QueryParams: map[string][]string{
			"fields": {
				"id",
			},
			"query": {
				string(queryJson),
			},
		},
	}

	results, _, _, err := dimb.dbResource["mail"].PaginatedFindAllWithoutFilters(searchRequest)

	if err != nil {
		return nil, err
	}

	ids := make([]uint32, 0)
	log.Printf("Mail search results: %v", results)
	for i, res := range results {
		if uid {
			ids = append(ids, uint32(res["id"].(int64)))
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

	mailBody, err := ioutil.ReadAll(body)
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

	if !HasFlag(flags, "\\Recent") {
		flags = append(flags, "\\Recent")
	}

	messageEntity, err := message.Read(bytes.NewReader(mailBody))
	if err != nil {
		return err
	}

	//enve, _ := backendutil.FetchEnvelope(messageEntity.Header)
	//mailContents, err := ioutil.ReadAll(messageEntity.Body)
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
		msgIdNew, _ := uuid.NewV4()
		msgId = msgIdNew.String()
	}
	hash := GetMD5Hash(string(mailBody))

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

	model := api2go.Api2GoModel{
		Data: map[string]interface{}{
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
			"user_account_id":  dimb.sessionUser.UserId,
			"seen":             false,
			"recent":           true,
			"flags":            strings.Join(flags, ","),
			"size":             len(mailBody),
		},
	}

	//tx, err := dimb.dbResource["mail"].connection.Beginx()
	//if err != nil {
	//	return err
	//}

	//txDbResource := NewFromDbResourceWithTransaction(dimb.dbResource["mail"], tx)
	//uidNext, err := txDbResource.GetMailboxNextUid(dimb.mailBoxId)
	//log.Printf("Assign next UID: %v", uidNext)
	//model.Data["uid"] = uidNext
	_, err = dimb.dbResource["mail"].Create(&model, apiRequest)
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
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, seq.Start, seq.Stop)
		}

		if err != nil {
			return err
		}

		for _, mailRow := range mails {
			currentFlags := strings.Split(mailRow["flags"].(string), ",")
			newFlags := backendutil.UpdateFlags(currentFlags, operation, flags)
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

	destinationMailBoxId, err := dimb.dbResource["mail_box"].GetMailAccountBox(dimb.mailAccountId, dest)
	if err != nil {
		return err
	}

	req := api2go.Request{
		PlainRequest: &http.Request{},
	}

	for _, set := range seqset.Set {

		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, set.Start, set.Stop)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, set.Start, set.Stop)
		}

		if err != nil {
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
			if !HasFlag(mailFlags, "\\Recent") {
				mailFlags = append(mailFlags, "\\Recent")
				mail["flags"] = strings.Join(mailFlags, ",")
			}

			_, err = dimb.dbResource["mail"].CreateWithoutFilter(&api2go.Api2GoModel{
				Data: mail,
			}, req)
		}

	}
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
