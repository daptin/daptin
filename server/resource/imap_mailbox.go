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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/textproto"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
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
	knownKeywords      map[string]bool
	sequenceView       IMAPSequenceView
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

func (dimb *DaptinImapMailBox) canUpdateMailbox(box map[string]interface{}, caller *auth.SessionUser, tx *sqlx.Tx) bool {
	crud := dimb.dbResource["mail"]
	return crud.mailAccountCanWrite(daptinid.InterfaceToDIR(box["mail_account_id"]), caller, tx) &&
		crud.mailRowPermission("mail_box", daptinid.InterfaceToDIR(box["reference_id"]), caller, false, tx) == nil
}

func (dimb *DaptinImapMailBox) readSelectedMails(selections []MailSelection, includeBody bool,
	transaction *sqlx.Tx) (map[daptinid.DaptinReferenceId]map[string]interface{}, error) {
	_, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return nil, err
	}
	return dimb.readIMAPMailRows(selections, caller, imapMailReadOptions{includeBody: includeBody, requireAll: true}, transaction)
}

type imapMailReadOptions struct {
	includeBody bool
	requireAll  bool
	internal    bool
	fields      string
}

func (dimb *DaptinImapMailBox) readIMAPMailRows(selections []MailSelection, caller *auth.SessionUser,
	options imapMailReadOptions, transaction *sqlx.Tx) (map[daptinid.DaptinReferenceId]map[string]interface{}, error) {
	result := make(map[daptinid.DaptinReferenceId]map[string]interface{}, len(selections))
	for start := 0; start < len(selections); start += imapResourceReadBatchSize {
		stop := start + imapResourceReadBatchSize
		if stop > len(selections) {
			stop = len(selections)
		}

		referenceIDs := make([]string, 0, stop-start)
		expectedUIDs := make(map[daptinid.DaptinReferenceId]uint32, stop-start)
		for _, selection := range selections[start:stop] {
			referenceIDs = append(referenceIDs, selection.ReferenceID.String())
			expectedUIDs[selection.ReferenceID] = selection.UID
		}

		requestURL, _ := url.Parse("/api/mail")
		requestContext := context.WithValue(context.Background(), "user", caller)
		if options.internal || start > 0 {
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
		if options.fields != "" {
			request.QueryParams["fields"] = []string{options.fields + ",uid"}
		}
		if options.includeBody {
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
		var matched int
		for _, model := range models {
			attributes := model.GetAttributes()
			attributes["reference_id"] = model.GetID()

			referenceID := daptinid.InterfaceToDIR(attributes["reference_id"])
			if expected, ok := expectedUIDs[referenceID]; !ok || !sameIMAPTarget(attributes, expected) {
				continue
			}
			matched++
			attributes["reference_id"] = referenceID
			result[referenceID] = attributes
		}
		if options.requireAll && matched != len(referenceIDs) {
			return nil, mailDraftError(403, "selected messages are no longer readable")
		}
	}
	return result, nil
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
	box, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
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
	mbs.ReadOnly = !dimb.canUpdateMailbox(box, caller, transaction)

	// Include cached keywords in FLAGS so clients know they exist
	dimb.lock.Lock()
	keywords := make([]string, 0, len(dimb.knownKeywords))
	for k := range dimb.knownKeywords {
		keywords = append(keywords, k)
	}
	dimb.lock.Unlock()
	sort.Strings(keywords)
	// Older and newly created Daptin folders may store just "\*" in their
	// flag metadata. It permits new keywords; it does not advertise system
	// flags. List the system flags supported by STORE for all such folders.
	mbs.Flags = advertisedIMAPFlags(dimb.status.Flags, keywords, false)
	if mbs.ReadOnly {
		mbs.PermanentFlags = []string{}
	} else {
		mbs.PermanentFlags = advertisedIMAPFlags(dimb.status.PermanentFlags, keywords, true)
	}

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

func advertisedIMAPFlags(stored, keywords []string, permanent bool) []string {
	values := []string{imap.AnsweredFlag, imap.FlaggedFlag, imap.DeletedFlag, imap.SeenFlag, imap.DraftFlag}
	values = append(values, stored...)
	values = append(values, keywords...)
	flags := make([]string, 0, len(values)+1)
	for _, flag := range canonicalMailFlags(values) {
		if flag == imap.TryCreateFlag || (permanent && flag == imap.RecentFlag) {
			continue
		}
		flags = append(flags, flag)
	}
	if permanent {
		flags = append(flags, imap.TryCreateFlag)
	}
	return flags
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
	return dimb.listMessages(uid, seqset, items, ch, false)
}

// ListMessagesReadOnly serves the original FETCH items without the implicit
// Seen change from non-PEEK body requests.
func (dimb *DaptinImapMailBox) ListMessagesReadOnly(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	return dimb.listMessages(uid, seqset, items, ch, true)
}

func (dimb *DaptinImapMailBox) listMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message, readOnly bool) error {
	defer close(ch)

	var expanded []imap.FetchItem
	for _, item := range items {
		expanded = append(expanded, item.Expand()...)
	}
	items = expanded
	needsMIME := slices.ContainsFunc(items, func(item imap.FetchItem) bool {
		switch item {
		case imap.FetchFlags, imap.FetchInternalDate, imap.FetchRFC822Size, imap.FetchUid:
			return false
		default:
			return true
		}
	})
	extendedBody := slices.Contains(items, imap.FetchBodyStructure)
	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	selections, err := dimb.selectAnnouncedMessages(uid, seqset, transaction)
	if err != nil {
		return err
	}
	_, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return err
	}
	// Authorize the requested set before starting responses, without opening MIME.
	mails, err := dimb.readIMAPMailRows(selections, caller,
		imapMailReadOptions{fields: "reference_id,flags,recent,internal_date,size", requireAll: true}, transaction)
	if err != nil {
		return err
	}
	var checkedSeenWrite, canMarkSeen bool
	var seenCaller *auth.SessionUser
	var mimeRows map[daptinid.DaptinReferenceId]map[string]interface{}
	windowEnd := 0
	rawSize := func(selection MailSelection) int64 {
		size, _ := ResourceRowInt64(mails[selection.ReferenceID]["size"])
		return size
	}

	for index, selection := range selections {
		mailContent, ok := mails[selection.ReferenceID]
		if !ok {
			return errors.New("native FETCH metadata missing for an authorized message")
		}
		var bodyContents []byte
		if needsMIME {
			if index == windowEnd {
				windowEnd = index + imapMIMEHydrationWindow(selections[index:], rawSize)
				mimeRows = nil
				mimeRows, err = dimb.readIMAPMailRows(selections[index:windowEnd], caller,
					imapMailReadOptions{fields: "mail", includeBody: true, requireAll: true, internal: true}, transaction)
				if err != nil {
					return err
				}
			}
			bodyContents, err = dimb.dbResource["mail"].MailColumnBytes("mail", "mail", mimeRows[selection.ReferenceID]["mail"])
			if err != nil {
				return fmt.Errorf("could not read original MIME for UID %d: %w", selection.UID, err)
			}
		}
		returnMail := imap.NewMessage(selection.SequenceNumber, items)
		returnMail.Size = uint32(rawSize(selection))
		flagList := mailFlagsForRow(mailContent)

		skipMail := false

		for _, item := range items {
			if skipMail {
				break
			}

			switch item {
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
			case imap.FetchBody, imap.FetchBodyStructure:
				if returnMail.BodyStructure != nil {
					continue
				}
				part, err := readIMAPMIMEMessage(bytes.NewReader(bodyContents))
				if err != nil {
					log.Printf("Failed to fetch MIME header for email [%v]: %v", mailContent["id"], err)
					skipMail = true
					break
				}
				bs, err := part.bodyStructure(extendedBody, 0)
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
				// Size comes from the sparse message row.
			case imap.FetchUid:
				returnMail.Uid = selection.UID
			default:
				section, err := imap.ParseBodySectionName(item)
				if CheckErr(err, "failed to parse item name") {
					skipMail = true
					break
				}

				l, err := fetchIMAPBodySection(bodyContents, section)
				if errors.Is(err, errIMAPNoSuchPart) {
					returnMail.Body[section] = nil
					continue
				}
				if err != nil {
					return fmt.Errorf("could not fetch MIME section for UID %d", selection.UID)
				}

				if !readOnly && !section.Peek && !HasFlag(flagList, imap.SeenFlag) {
					// A write grant can be removed after SELECT or
					// withheld on one message. Its valid read grant
					// still permits FETCH, without the implicit write.
					if !checkedSeenWrite {
						box, current, err := dimb.authorizeRead(transaction)
						if err != nil {
							return err
						}
						checkedSeenWrite = true
						seenCaller = current
						canMarkSeen = dimb.canUpdateMailbox(box, current, transaction)
					}
					crud := dimb.dbResource["mail"]
					ref := daptinid.InterfaceToDIR(mailContent["reference_id"])
					if canMarkSeen && crud.mailRowPermission("mail", ref, seenCaller, false, transaction) == nil {
						written, changed, err := crud.mutateMailFlags(ref, imap.AddFlags, []string{imap.SeenFlag}, seenCaller, transaction, dimb.mailBoxReferenceId, selection.UID)
						if err != nil {
							return err
						}
						flagList = written
						returnMail.Flags = flagList
						if changed {
							returnMail.Items[imap.FetchFlags] = nil
						}
					}
				}

				returnMail.Body[section] = l
			}
		}

		if skipMail {
			continue
		}
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
	_, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return nil, err
	}
	crud := dimb.dbResource["mail"]
	candidates, plan, err := crud.selectIMAPSearchCandidates(dimb.mailBoxId, criteria, transaction, &dimb.sequenceView)
	if err != nil {
		return nil, err
	}
	result := make([]uint32, 0)
	for start := 0; start < len(candidates); start += imapResourceReadBatchSize {
		batch := candidates[start:min(start+imapResourceReadBatchSize, len(candidates))]
		selections := make([]MailSelection, 0, len(batch))
		for _, candidate := range batch {
			selections = append(selections, candidate.MailSelection)
		}
		visible, err := dimb.readIMAPMailRows(selections, caller,
			imapMailReadOptions{fields: "reference_id", internal: start > 0}, transaction)
		if err != nil {
			return nil, err
		}
		var pending []imapSearchHydration
		matches := make([]bool, len(batch))
		for index, candidate := range batch {
			if _, ok := visible[candidate.ReferenceID]; !ok {
				continue
			}
			matches[index], err = plan.matches(candidate.predicates, &imapSearchContent{load: imapSearchNeedsContent})
			if errors.Is(err, errIMAPSearchNeedsContent) {
				pending = append(pending, imapSearchHydration{index, candidate.size})
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		for len(pending) > 0 {
			windowSize := imapSearchHydrationWindow(pending)
			hydration := make([]MailSelection, 0, windowSize)
			for _, entry := range pending[:windowSize] {
				hydration = append(hydration, batch[entry.index].MailSelection)
			}
			hydrated, err := dimb.readIMAPMailRows(hydration, caller,
				imapMailReadOptions{fields: "mail", includeBody: true, internal: true}, transaction)
			if err != nil {
				return nil, err
			}
			for _, entry := range pending[:windowSize] {
				candidate := batch[entry.index]
				row, ok := hydrated[candidate.ReferenceID]
				if !ok {
					continue // recheck row and mailbox scope in the hydration query
				}
				content := imapSearchContent{load: func() ([]byte, error) {
					return crud.MailColumnBytes("mail", "mail", row["mail"])
				}}
				matches[entry.index], err = plan.matches(candidate.predicates, &content)
				if err != nil {
					return nil, err
				}
			}
			pending = pending[windowSize:]
		}
		for index, candidate := range batch {
			if matches[index] {
				if uid {
					result = append(result, candidate.UID)
				} else {
					result = append(result, candidate.SequenceNumber)
				}
			}
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

	selections, err := dimb.selectAnnouncedMessages(uid, seqset, transaction)
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
		_, _, err = dimb.dbResource["mail"].mutateMailFlags(
			daptinid.InterfaceToDIR(mailRow["reference_id"]), operation, flags, dimb.sessionUser, transaction, dimb.mailBoxReferenceId, selection.UID)
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

	box, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return err
	}
	dest = imap.CanonicalMailboxName(dest)
	destinationMailBoxId, err := dimb.dbResource["mail_box"].GetMailAccountBox(dimb.mailAccountId, dest, transaction)
	if err != nil {
		canCreate := errors.Is(err, errMailBoxNotFound) &&
			dimb.dbResource["mail"].mailAccountCanWrite(daptinid.InterfaceToDIR(box["mail_account_id"]), caller, transaction)
		return imapDestinationError(dest, err, canCreate)
	}

	selections, err := dimb.selectAnnouncedMessages(uid, seqset, transaction)
	if err != nil {
		return err
	}
	mails, err := dimb.readIMAPMailRows(selections, caller, imapMailReadOptions{includeBody: true, requireAll: true}, transaction)
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
		flags := mailFlagsForRow(mail)
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

func imapDestinationError(name string, err error, canCreate bool) error {
	if errors.Is(err, errMailBoxNotFound) && canCreate && validateMailFolderName(name) == nil {
		return &imap.ErrStatusResp{Resp: &imap.StatusResp{
			Type: imap.StatusRespNo, Code: imap.CodeTryCreate, Info: "No such destination mailbox",
		}}
	}
	return err
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

// ReadIMAPSelection reads the ordered mailbox UIDs and the Recent count from
// one metadata query. The protocol connection owns its last announced view.
func (dimb *DaptinImapMailBox) ReadIMAPSelection(consumeRecent bool) ([]uint32, uint32, error) {
	transaction, err := dimb.dbResource["mail_box"].Connection().Beginx()
	if err != nil {
		return nil, 0, err
	}
	defer transaction.Rollback()
	box, caller, err := dimb.authorizeRead(transaction)
	if err != nil {
		return nil, 0, err
	}
	uids, recent, err := dimb.dbResource["mail"].imapSelectionUIDs(dimb.mailBoxId, transaction)
	if err != nil {
		return nil, 0, err
	}
	if consumeRecent && recent > 0 && dimb.canUpdateMailbox(box, caller, transaction) {
		if err = dimb.dbResource["mail_box"].ClearRecentFlags(dimb.mailBoxId, transaction); err != nil {
			return nil, 0, err
		}
	}
	return uids, recent, transaction.Commit()
}
