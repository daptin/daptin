package resource

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/jmoiron/sqlx"
)

var errITIPStaleReply = errors.New("reply sequence is stale")

type itipProcessMailAction struct {
	cruds map[string]*DbResource
}

func NewITIPProcessMailAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &itipProcessMailAction{cruds: cruds}
}

func (*itipProcessMailAction) Name() string { return "itip.process_mail" }

func (a *itipProcessMailAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("mail processing requires an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || !IsAdminWithTransaction(caller, tx) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar mail processing denied"), "calendar mail processing denied", http.StatusForbidden)}
	}
	mailRef := daptinid.InterfaceToDIR(fields["mail_id"])
	if mailRef == daptinid.NullReferenceId {
		return nil, nil, []error{errors.New("mail_id must be a reference ID")}
	}
	row, _, err := a.cruds["mail"].GetSingleRowByReferenceIdWithTransaction("mail", mailRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	boxRef := daptinid.InterfaceToDIR(row["mail_box_id"])
	box, _, err := a.cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", boxRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if strings.EqualFold(fmt.Sprint(box["name"]), "Sent") || strings.EqualFold(fmt.Sprint(box["name"]), "Spam") {
		return nil, []actionresponse.ActionResponse{NewActionResponse("itip.process_mail", map[string]interface{}{
			"mail_id": mailRef.String(), "events": 0,
		})}, nil
	}
	accountRef := daptinid.InterfaceToDIR(box["mail_account_id"])
	account, _, err := a.cruds["mail_account"].GetSingleRowByReferenceIdWithTransaction("mail_account", accountRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	accountID, err := GetReferenceIdToIdWithTransaction("mail_account", accountRef, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	connected, err := GetReferenceIdByWhereClauseWithTransaction("collection", tx, goqu.Ex{"scheduling_mail_account_id": accountID})
	if err != nil {
		return nil, nil, []error{err}
	}
	var selectedCollection daptinid.DaptinReferenceId
	if value := fields["collection_id"]; value != nil && value != "" {
		id, ok := value.(string)
		if !ok {
			return nil, nil, []error{errors.New("collection_id must be a reference ID")}
		}
		selectedCollection = daptinid.InterfaceToDIR(strings.TrimSpace(id))
		if selectedCollection == daptinid.NullReferenceId {
			return nil, nil, []error{errors.New("collection_id must be a reference ID")}
		}
		linked := false
		for _, ref := range connected {
			if ref == selectedCollection {
				linked = true
				break
			}
		}
		if !linked {
			return nil, nil, []error{errors.New("selected calendar is not connected to the receiving mail account")}
		}
	}
	if len(connected) == 0 {
		return nil, []actionresponse.ActionResponse{NewActionResponse("itip.process_mail", map[string]interface{}{"mail_id": mailRef.String(), "events": 0})}, nil
	}
	var recipientID int64
	ambiguousRecipient := false
	for _, collectionRef := range connected {
		if selectedCollection != daptinid.NullReferenceId && collectionRef != selectedCollection {
			continue
		}
		collection, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", collectionRef, nil, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		ownerRef := daptinid.InterfaceToDIR(collection["user_account_id"])
		ownerID, err := a.cruds["user_account"].GetReferenceIdToId("user_account", ownerRef, tx)
		if err != nil || ownerID == 0 {
			return nil, nil, []error{errors.New("connected calendar has no owner")}
		}
		if recipientID != 0 && recipientID != ownerID {
			ambiguousRecipient = true
		}
		recipientID = ownerID
	}
	if ambiguousRecipient {
		recipientID = 0
	}
	applyCollection := selectedCollection
	if applyCollection == daptinid.NullReferenceId && recipientID != 0 {
		if len(connected) == 1 {
			applyCollection = connected[0]
		}
	}
	accountAddress := strings.ToLower(strings.TrimSpace(fmt.Sprint(account["username"])))
	from, err := mail.ParseAddress(fmt.Sprint(row["from_address"]))
	if err != nil {
		return nil, nil, []error{err}
	}
	sender := strings.ToLower(from.Address)
	if sender == "" || !strings.EqualFold(fmt.Sprint(row["recipient"]), accountAddress) {
		return nil, nil, []error{errors.New("mail does not belong to the addressed scheduling account")}
	}
	unpack := a.cruds["mail"].GetActionHandler("mail.unpack")
	if unpack == nil {
		return nil, nil, []error{errors.New("mail.unpack is unavailable")}
	}
	parsed, _, errs := unpack.DoAction(actionresponse.Outcome{Type: "mail.unpack"}, map[string]interface{}{"mail": row["mail"]}, tx)
	if len(errs) != 0 {
		return nil, nil, errs
	}
	model, ok := parsed.Result().(api2go.Api2GoModel)
	if !ok {
		return nil, nil, []error{errors.New("mail.unpack returned an invalid result")}
	}
	parts, ok := model.GetAttributes()["parts"].([]map[string]interface{})
	if !ok {
		return nil, nil, []error{errors.New("mail.unpack returned no MIME parts")}
	}
	processed := 0
	for _, part := range parts {
		if part["media_type"] != "text/calendar" {
			continue
		}
		content, ok := part["content"].([]interface{})
		if !ok || len(content) != 1 {
			return nil, nil, []error{errors.New("calendar MIME part has no content")}
		}
		file, ok := content[0].(map[string]interface{})
		if !ok {
			return nil, nil, []error{errors.New("calendar MIME part is invalid")}
		}
		data, err := base64.StdEncoding.DecodeString(fmt.Sprint(file["contents"]))
		if err != nil {
			return nil, nil, []error{err}
		}
		calendar, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
		if err != nil {
			return nil, nil, []error{err}
		}
		method := strings.ToUpper(itipProp(calendar.Props.Get("METHOD")))
		if method != "REQUEST" && method != "REPLY" && method != "CANCEL" {
			return nil, nil, []error{errors.New("unsupported iTIP method")}
		}
		for _, event := range itipComponents(calendar) {
			if err := a.processEvent(mailRef, accountRef, accountAddress, sender, method, data, event, caller, recipientID, selectedCollection, applyCollection, tx); err != nil {
				return nil, nil, []error{err}
			}
			processed++
		}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("itip.process_mail", map[string]interface{}{"mail_id": mailRef.String(), "events": processed})}, nil
}

func (a *itipProcessMailAction) processEvent(mailRef, accountRef daptinid.DaptinReferenceId, accountAddress, sender, method string, data []byte, incoming ical.Event, caller *auth.SessionUser, recipientID int64, selectedCollection, applyCollection daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	uid := itipProp(incoming.Props.Get("UID"))
	if uid == "" {
		return errors.New("iTIP event has no UID")
	}
	for _, name := range []string{"ATTENDEE", "ORGANIZER"} {
		for i := range incoming.Props[name] {
			for _, parameter := range []string{"SCHEDULE-AGENT", "SCHEDULE-FORCE-SEND", "SCHEDULE-STATUS"} {
				incoming.Props[name][i].Params.Del(parameter)
			}
		}
	}
	recurrence := itipProp(incoming.Props.Get("RECURRENCE-ID"))
	sequence := 0
	if prop := incoming.Props.Get("SEQUENCE"); prop != nil {
		var err error
		sequence, err = strconv.Atoi(prop.Value)
		if err != nil || sequence < 0 {
			return errors.New("iTIP event has invalid SEQUENCE")
		}
	}
	keyBytes := sha256.Sum256([]byte(strings.Join([]string{accountRef.String(), sender, method, uid,
		recurrence, strconv.Itoa(sequence), string(data)}, "\x00")))
	key := hex.EncodeToString(keyBytes[:])
	previous, err := GetReferenceIdByWhereClauseWithTransaction("cal_mail", tx, goqu.Ex{"message_key": key})
	if err != nil {
		return err
	}
	var pendingRef daptinid.DaptinReferenceId
	if len(previous) != 0 {
		pendingRef = previous[0]
		prior, _, err := a.cruds["cal_mail"].GetSingleRowByReferenceIdWithTransaction("cal_mail", pendingRef, nil, tx)
		if err != nil {
			return err
		}
		if prior["state"] != "pending" || method == "REPLY" {
			return nil
		}
	}
	state := "pending"
	var collectionRef, eventRef daptinid.DaptinReferenceId
	if method == "REPLY" {
		if itipAddress(incoming.Props.Get("ORGANIZER")) != accountAddress {
			return errors.New("reply organizer does not match addressed account")
		}
		partstat, present := itipAttendees(incoming)[sender]
		if !present || (partstat != "ACCEPTED" && partstat != "DECLINED" && partstat != "TENTATIVE") {
			return errors.New("reply attendee or PARTSTAT is invalid")
		}
		status, err := itipReplyStatus(incoming)
		if err != nil {
			return err
		}
		collectionRef, eventRef, err = a.matchInvitation(accountRef, uid, recurrence, sequence, sender, tx)
		if err != nil {
			return err
		}
		if selectedCollection != daptinid.NullReferenceId && collectionRef != selectedCollection {
			return errors.New("reply does not belong to the selected calendar")
		}
		collection, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", collectionRef, nil, tx)
		if err != nil {
			return err
		}
		ownerRef := daptinid.InterfaceToDIR(collection["user_account_id"])
		recipientID, err = a.cruds["user_account"].GetReferenceIdToId("user_account", ownerRef, tx)
		if err != nil || recipientID == 0 {
			return errors.New("invitation collection has no owner")
		}
		if err := a.applyReply(collectionRef, eventRef, uid, recurrence, sequence, sender, partstat, status, caller, tx); err != nil {
			if !errors.Is(err, errITIPStaleReply) {
				return err
			}
			state = "ignored"
		} else {
			state = "applied"
		}
	} else {
		if recipientID == 0 {
			return errors.New("incoming invitation has no unique calendar principal")
		}
		if itipAddress(incoming.Props.Get("ORGANIZER")) != sender {
			return errors.New("invitation organizer does not match mail sender")
		}
		if _, invited := itipAttendees(incoming)[accountAddress]; !invited {
			return errors.New("invitation does not name the addressed account")
		}
		if applyCollection != daptinid.NullReferenceId {
			collectionRef = applyCollection
			eventRef, state, err = a.applyInvitation(collectionRef, accountRef, accountAddress, sender, method, uid, recurrence, data, incoming, recipientID, tx)
			if err != nil {
				return err
			}
		}
	}
	recipient, err := exchangeSessionUser(a.cruds, recipientID, tx)
	if err != nil {
		return err
	}
	attrs := map[string]interface{}{
		"message_key": key, "direction": "inbound", "method": method, "uid": uid,
		"recurrence_id": recurrence, "sequence": sequence, "sender_address": sender,
		"recipient_address": accountAddress, "icalendar": string(data), "state": state,
		"mail_id": mailRef.String(),
	}
	if collectionRef != daptinid.NullReferenceId {
		attrs["collection_id"] = collectionRef.String()
		attrs["event_reference"] = eventRef.String()
	}
	b := NewCalDAVBackend(a.cruds, recipient, nil)
	if pendingRef != daptinid.NullReferenceId {
		model := api2go.NewApi2GoModelWithData("cal_mail", nil, 0, nil, map[string]interface{}{
			"reference_id": pendingRef.String(), "state": state, "collection_id": attrs["collection_id"],
			"event_reference": attrs["event_reference"],
		})
		_, err = a.cruds["cal_mail"].updateAfterAuthorizationWithTransaction(model, b.request(http.MethodPatch, "/api/cal_mail/"+pendingRef.String()), tx)
		if err != nil {
			return err
		}
	} else {
		model := api2go.NewApi2GoModelWithData("cal_mail", nil, int64(a.cruds["cal_mail"].TableInfo().DefaultPermission), nil, attrs)
		_, err = a.cruds["cal_mail"].createWithoutFilterAfterAuthorization(model, b.request(http.MethodPost, "/api/cal_mail"), tx)
		if err != nil {
			return err
		}
	}
	if method == "REQUEST" && collectionRef != daptinid.NullReferenceId && state != "pending" {
		return a.applyPendingCancellations(collectionRef, accountRef, accountAddress, sender, uid, recipientID, tx)
	}
	return nil
}

func (a *itipProcessMailAction) matchInvitation(accountRef daptinid.DaptinReferenceId, uid, recurrence string, sequence int, sender string, tx *sqlx.Tx) (daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, error) {
	rows, _, err := a.cruds["cal_mail"].GetRowsByWhereClauseWithTransaction("cal_mail", nil, tx, goqu.Ex{
		"direction": "outbound", "method": "REQUEST", "uid": uid, "recurrence_id": recurrence,
		"sequence": sequence, "recipient_address": sender,
	})
	if err != nil {
		return daptinid.NullReferenceId, daptinid.NullReferenceId, err
	}
	var collectionRef, eventRef daptinid.DaptinReferenceId
	for _, row := range rows {
		candidate := daptinid.InterfaceToDIR(row["collection_id"])
		collection, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", candidate, nil, tx)
		if err != nil || daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"]) != accountRef {
			continue
		}
		candidateEvent := daptinid.InterfaceToDIR(row["event_reference"])
		if candidateEvent == daptinid.NullReferenceId {
			continue
		}
		if eventRef != daptinid.NullReferenceId && eventRef != candidateEvent {
			return daptinid.NullReferenceId, daptinid.NullReferenceId, errors.New("ambiguous invitation UID")
		}
		collectionRef, eventRef = candidate, candidateEvent
	}
	if eventRef == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, daptinid.NullReferenceId, errors.New("no matching invitation")
	}
	return collectionRef, eventRef, nil
}

func itipReplyStatus(event ical.Event) (string, error) {
	properties := event.Props.Values("REQUEST-STATUS")
	if len(properties) == 0 {
		return "2.0", nil
	}
	if len(properties) > 16 {
		return "", errors.New("reply has too many REQUEST-STATUS values")
	}
	codes := make([]string, 0, len(properties))
	for _, property := range properties {
		code := strings.TrimSpace(strings.SplitN(property.Value, ";", 2)[0])
		parts := strings.Split(code, ".")
		if len(parts) != 2 || len(parts[0]) != 1 || len(parts[1]) == 0 {
			return "", errors.New("reply has invalid REQUEST-STATUS")
		}
		for _, digit := range parts[0] + parts[1] {
			if digit < '0' || digit > '9' {
				return "", errors.New("reply has invalid REQUEST-STATUS")
			}
		}
		codes = append(codes, code)
	}
	return strings.Join(codes, ","), nil
}

func (a *itipProcessMailAction) applyReply(collectionRef, eventRef daptinid.DaptinReferenceId, uid, recurrence string, sequence int, sender, partstat, status string, caller *auth.SessionUser, tx *sqlx.Tx) error {
	if err := a.cruds[calendarCollectionTable].lockRowByWhereWithTransaction(tx,
		goqu.Ex{"reference_id": collectionRef[:]}); err != nil {
		return err
	}
	row, _, err := a.cruds["calendar"].GetSingleRowByReferenceIdWithTransaction("calendar", eventRef, nil, tx)
	if err != nil {
		return err
	}
	if daptinid.InterfaceToDIR(row["collection_id"]) != collectionRef {
		return errors.New("invited event moved to another calendar")
	}
	b := NewCalDAVBackend(a.cruds, caller, nil)
	content, err := b.contentBytes("calendar", row)
	if err != nil {
		return err
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
	if err != nil {
		return err
	}
	changed := false
	for _, event := range itipComponents(calendar) {
		if itipProp(event.Props.Get("UID")) != uid || itipProp(event.Props.Get("RECURRENCE-ID")) != recurrence {
			continue
		}
		currentSequence := 0
		if prop := event.Props.Get("SEQUENCE"); prop != nil {
			currentSequence, err = strconv.Atoi(prop.Value)
			if err != nil {
				return err
			}
		}
		if currentSequence != sequence {
			return errITIPStaleReply
		}
		for i, attendee := range event.Props["ATTENDEE"] {
			if itipAddress(&attendee) == sender {
				if event.Props["ATTENDEE"][i].Params == nil {
					event.Props["ATTENDEE"][i].Params = ical.Params{}
				}
				event.Props["ATTENDEE"][i].Params.Set("PARTSTAT", partstat)
				event.Props["ATTENDEE"][i].Params.Set("SCHEDULE-STATUS", status)
				changed = true
			}
		}
	}
	if !changed {
		return errors.New("invited attendee is no longer on the event")
	}
	var updated bytes.Buffer
	if err := ical.NewEncoder(&updated).Encode(calendar); err != nil {
		return err
	}
	requestPath := fmt.Sprint(row["rpath"])
	// GetSingleRow includes the persistence version. The DAV update helper
	// accepts an unversioned resource snapshot and computes changed columns.
	delete(row, "version")
	return b.calendarUpdate("calendar", requestPath, row, map[string]interface{}{
		"content": b.contentValue("calendar", requestPath, ical.MIMEType, updated.Bytes()),
	}, tx)
}
