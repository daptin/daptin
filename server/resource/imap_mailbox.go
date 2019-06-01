package resource

import (
	"bytes"
	"encoding/base64"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/artpar/parsemail"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-message"
	"gopkg.in/Masterminds/squirrel.v1"
	"log"
	"net/http"
	"strings"
	"time"
)

type DaptinImapMailBox struct {
	name                   string
	dbResource             map[string]*DbResource
	mailAccountId          int64
	mailAccountReferenceId string
	userAccountId          int64
	mailBoxId              int64
	info                   imap.MailboxInfo
	status                 imap.MailboxStatus
}

// Name returns this mailbox name.
func (dimb *DaptinImapMailBox) Name() string {
	return dimb.name
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

	iMap := make(map[imap.StatusItem]bool)

	for _, item := range items {
		iMap[item] = true
	}

	mbs := imap.NewMailboxStatus(dimb.name, items)
	mbs.Flags = dimb.status.Flags
	mbs.PermanentFlags = []string{"\\*"}
	mbs.UnseenSeqNum = dimb.dbResource["mail_box"].GetFirstUnseenMailSequence(dimb.mailBoxId)

	//vals := make([]interface{}, 0)
	for item := range dimb.status.Items {
		switch item {
		case imap.StatusMessages:
			mbs.Messages = dimb.status.Messages
		case imap.StatusRecent:
			mbs.Recent = dimb.status.Recent
		case imap.StatusUnseen:
			mbs.Unseen = dimb.status.Unseen
		case imap.StatusUidNext:
			mbs.UidNext = dimb.status.UidNext
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
	dimb.status = *newStatus

	return nil
}

// ListMessages returns a list of messages. seqset must be interpreted as UIDs
// if uid is set to true and as message sequence numbers otherwise. See RFC
// 3501 section 6.4.5 for a list of items that can be requested.
//
// Messages must be sent to ch. When the function returns, ch must be closed.
func (dimb *DaptinImapMailBox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {

	for _, seq := range seqset.Set {
		log.Printf("Fetch request [%v] from %v to %v", uid, seq.Start, seq.Stop)

		seqNo := seq.Start
		var mails []map[string]interface{}
		var err error
		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, seq.Start, seq.Stop)
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
			messageBytes := bytes.NewReader(bodyContents)
			//log.Printf("Message: %v", string(bodyContents))
			//parsedMail, err := parsemail.Parse(messageBytes)

			messageEntity, err := message.Read(messageBytes)

			if err != nil {
				log.Printf("Failed to parse email body: %v", err)
				continue
			}
			returnMail := imap.NewMessage(seqNo, items)
			returnMail.Size = uint32(mailContent["size"].(int64))
			//responseItems := make([]interface{}, 0)
			for _, item := range items {

				switch item {
				case imap.FetchEnvelope:
					enve, _ := backendutil.FetchEnvelope(messageEntity.Header)
					returnMail.Envelope = enve
					//responseItems = append(responseItems, imap.FetchEnvelope, []interface{}{
					//	enve.Date.Format(time.RFC1123),
					//	enve.Subject,
					//	enve.From,
					//	enve.Sender,
					//	enve.ReplyTo,
					//	enve.To,
					//	enve.Cc,
					//	enve.Bcc,
					//	enve.InReplyTo,
					//	enve.MessageId,
					//})
					//returnMail.Envelope = enve
				case imap.FetchBody, imap.FetchBodyStructure:
					bs, _ := backendutil.FetchBodyStructure(messageEntity, item == imap.FetchBodyStructure)
					returnMail.BodyStructure = bs
					//responseItems = append(responseItems, imap.FetchBodyStructure, bs)
				case imap.FetchFlags:
					flagList := strings.Split(mailContent["flags"].(string), ",")
					//lst := make([]interface{}, 0)
					//for _, f := range flagList {
					//	lst = append(lst, f)
					//}
					//responseItems = append(responseItems, imap.FetchFlags, lst)
					returnMail.Flags = flagList
				case imap.FetchInternalDate:
					//returnMail.InternalDate = mailContent["created_at"].(time.Time)
					returnMail.InternalDate = mailContent["created_at"].(time.Time)
					//responseItems = append(responseItems, imap.FetchInternalDate, mailContent["created_at"].(time.Time))
				case imap.FetchRFC822Size:
					//returnMail.Size = uint32(mailContent["size"].(int64))
					returnMail.Size = uint32(mailContent["size"].(int64))
					//responseItems = append(responseItems, imap.FetchRFC822Size, uint32(mailContent["size"].(int64)))
				case imap.FetchUid:
					returnMail.Uid = uint32(mailContent["uid"].(int64))
					//responseItems = append(responseItems, imap.FetchUid, uint32(mailContent["uid"].(int64)))
				default:
					section, err := imap.ParseBodySectionName(item)
					if err != nil {
						break
					}

					l, _ := backendutil.FetchBodySection(messageEntity, section)
					returnMail.Body[section] = l
					//responseItems = append(responseItems, string(item), l)
				}
			}
			//err = returnMail.Parse(responseItems)
			//if err != nil {
			//	log.Printf("Failed to parse fields: %v", err)
			//	continue
			//}

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

	httpRequest := http.Request{

	}

	queries := make([]Query, 0)

	if criteria.Uid != nil {



		queries = append(queries, Query{
			ColumnName: "uid",
			Operator: "contains",
			Value: criteria.Uid.Set,
		})
	}

	searchRequest := api2go.Request{
		PlainRequest: &httpRequest,
	}

	dimb.dbResource["mail"].PaginatedFindAllWithoutFilters(searchRequest)

	return nil, nil
}

// CreateMessage appends a new message to this mailbox. The \Recent flag will
// be added no matter flags is empty or not. If date is nil, the current time
// will be used.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (dimb *DaptinImapMailBox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {

	mailBody := make([]byte, 100000)
	mailBodyLen, err := body.Read(mailBody)
	if err != nil {
		return err
	}

	apiRequest := api2go.Request{
		PlainRequest: &http.Request{

		},
	}

	if !HasFlag(flags, "\\Recent") {
		flags = append(flags, "\\Recent")
	}

	messageEntity, err := message.Read(bytes.NewReader(mailBody))
	if err != nil {
		return err
	}

	enve, _ := backendutil.FetchEnvelope(messageEntity.Header)
	mailContents := make([]byte, 100000)
	n, err := messageEntity.Body.Read(mailContents)
	if err != nil {
		return err
	}

	parsedmail, err := parsemail.Parse(bytes.NewReader(mailBody))
	log.Printf("%v length of the new message", n)

	msgId, err := uuid.NewV4()
	hash := GetMD5Hash(string(mailBody))
	model := api2go.Api2GoModel{
		Data: map[string]interface{}{
			"message_id":       msgId,
			"mail_id":          hash,
			"from_address":     enve.From,
			"to_address":       enve.To,
			"sender_address":   enve.Sender,
			"subject":          enve.Subject,
			"body":             string(mailContents),
			"mail":             mailBody,
			"spam_score":       0,
			"hash":             hash,
			"internal_date":    parsedmail.Date,
			"uid":              dimb.status.UidNext,
			"content_type":     messageEntity.Header.Get("Content-Type"),
			"reply_to_address": enve.ReplyTo,
			"recipient":        enve.To,
			"has_attachment":   len(parsedmail.Attachments),
			"ip_addr":          "",
			"return_path":      "",
			"is_tls":           false,
			"mail_box_id":      dimb.mailBoxId,
			"user_account_id":  dimb.userAccountId,
			"seen":             false,
			"recent":           true,
			"flags":            strings.Join(flags, ","),
			"size":             mailBodyLen,
		},
	}
	_, err = dimb.dbResource["mail"].CreateWithoutFilter(&model, apiRequest)

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
			err = dimb.dbResource["mail_box"].UpdateMailFlags(dimb.mailBoxId, mailRow["id"].(int64), strings.Join(newFlags, ","))
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
		PlainRequest: &http.Request{

		},
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
			mail["mail_box_id"] = destinationMailBoxId

			delete(mail, "reference_id")
			delete(mail, "updated_at")
			delete(mail, "created_at")
			delete(mail, "id")

			mailFlags := strings.Split(mail["flags"].(string), ",")
			if !HasFlag(mailFlags, "\\Recent") {
				mailFlags = append(mailFlags, "\\Recent")
				mail["flags"] = strings.Join(mailFlags, ",")
			}

			_, err = dimb.dbResource["mail_box"].CreateWithoutFilter(&api2go.Api2GoModel{
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

	err := dimb.dbResource["mail_box"].ExpungeMailBox(dimb.mailBoxId)
	return err
}
