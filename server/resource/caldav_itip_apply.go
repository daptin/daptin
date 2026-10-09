package resource

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const itipLegacyCandidateLimit = 1000

// CalDAV writes and incoming scheduling use the same searchable event identity.
// A calendar object may contain a master event and recurrence overrides, all
// with the same UID and organizer.
func itipCalendarIdentity(calendar *ical.Calendar) (string, string) {
	if calendar == nil {
		return "", ""
	}
	var uid, organizer string
	for _, event := range calendar.Events() {
		candidateUID := itipProp(event.Props.Get("UID"))
		candidateOrganizer := itipAddress(event.Props.Get("ORGANIZER"))
		if candidateUID == "" || (uid != "" && (uid != candidateUID || organizer != candidateOrganizer)) {
			return "", ""
		}
		uid, organizer = candidateUID, candidateOrganizer
	}
	return uid, organizer
}

func itipStamp(event ical.Event) (time.Time, error) {
	stamp, err := event.Props.DateTime("DTSTAMP", time.UTC)
	if err != nil || stamp.IsZero() {
		return time.Time{}, errors.New("iTIP event has invalid DTSTAMP")
	}
	return stamp, nil
}

func itipSequence(event ical.Event) (int, error) {
	if prop := event.Props.Get("SEQUENCE"); prop != nil {
		sequence, err := strconv.Atoi(prop.Value)
		if err != nil || sequence < 0 {
			return 0, errors.New("iTIP event has invalid SEQUENCE")
		}
		return sequence, nil
	}
	return 0, nil
}

func itipNewer(incoming, stored ical.Event) (bool, error) {
	incomingSequence, err := itipSequence(incoming)
	if err != nil {
		return false, err
	}
	storedSequence, err := itipSequence(stored)
	if err != nil {
		return false, err
	}
	if incomingSequence != storedSequence {
		return incomingSequence > storedSequence, nil
	}
	incomingStamp, err := itipStamp(incoming)
	if err != nil {
		return false, err
	}
	storedStamp, err := itipStamp(stored)
	if err != nil {
		return false, err
	}
	return incomingStamp.After(storedStamp), nil
}

func itipScheduleChanged(old, incoming ical.Event) bool {
	for _, name := range []string{"DTSTART", "DTEND", "DURATION", "RRULE", "RDATE", "EXDATE"} {
		if fmt.Sprint(old.Props[name]) != fmt.Sprint(incoming.Props[name]) {
			return true
		}
	}
	return false
}

func itipPreserveAttendeeState(old, incoming ical.Event, address string) {
	if itipScheduleChanged(old, incoming) {
		for i := range incoming.Props["ATTENDEE"] {
			if itipAddress(&incoming.Props["ATTENDEE"][i]) == address {
				incoming.Props["ATTENDEE"][i].Params.Set("PARTSTAT", "NEEDS-ACTION")
			}
		}
		return
	}
	for _, attendee := range old.Props.Values("ATTENDEE") {
		if itipAddress(&attendee) != address {
			continue
		}
		for i := range incoming.Props["ATTENDEE"] {
			if itipAddress(&incoming.Props["ATTENDEE"][i]) == address {
				if status := attendee.Params.Get("PARTSTAT"); status != "" {
					incoming.Props["ATTENDEE"][i].Params.Set("PARTSTAT", status)
				}
			}
		}
	}
	hasAlarm := false
	for _, child := range incoming.Children {
		hasAlarm = hasAlarm || child.Name == "VALARM"
	}
	if !hasAlarm {
		for _, child := range old.Children {
			if child.Name == "VALARM" {
				incoming.Children = append(incoming.Children, child)
			}
		}
	}
}

// applyInvitation runs inside mail.process_itip's transaction. It writes as
// the calendar owner so normal resource ownership, grants and metering apply.
func (a *itipProcessMailAction) applyInvitation(collectionRef, accountRef daptinid.DaptinReferenceId,
	accountAddress, sender, method, uid, recurrence string, data []byte,
	incoming ical.Event, recipientID int64, tx *sqlx.Tx) (daptinid.DaptinReferenceId, string, error) {
	collection, _, err := a.cruds[calendarCollectionTable].GetSingleRowByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, nil, tx)
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	if daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"]) != accountRef {
		return daptinid.NullReferenceId, "", errors.New("calendar is not connected to the receiving mail account")
	}
	ownerRef := daptinid.InterfaceToDIR(collection["user_account_id"])
	owner, err := exchangeSessionUser(a.cruds, recipientID, tx)
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	if owner.UserReferenceId != ownerRef {
		return daptinid.NullReferenceId, "", errors.New("invitation calendar has a different owner")
	}
	b := NewCalDAVBackend(a.cruds, owner, nil)
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return daptinid.NullReferenceId, "", err
	}
	collectionPath := "/caldav/" + ownerRef.String() + "/calendars/" + fmt.Sprint(collection["name"]) + "/"
	row, err := a.findInvitationEvent(b, collection, collectionPath, uid, sender, tx)
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	latest, err := a.latestInvitationVersion(collectionRef, uid, sender, accountAddress, recurrence, tx)
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	if latest != nil {
		newer, err := itipNewer(incoming, *latest)
		if err != nil {
			return daptinid.NullReferenceId, "", err
		}
		if !newer {
			if row != nil {
				return daptinid.InterfaceToDIR(row["reference_id"]), "ignored", nil
			}
			return daptinid.NullReferenceId, "ignored", nil
		}
	}
	if row == nil && method == "CANCEL" {
		if recurrence == "" {
			return daptinid.NullReferenceId, "applied", nil
		}
		return daptinid.NullReferenceId, "pending", nil
	}
	if row == nil && !b.calendarEditAllowed(collection, tx, true) || row != nil && !b.calendarEditAllowed(collection, tx, false) {
		return daptinid.NullReferenceId, "", errors.New("calendar edit access denied")
	}
	var stored *ical.Calendar
	var requestPath string
	if row != nil {
		requestPath = fmt.Sprint(row["rpath"])
		content, err := b.contentBytes(calendarObjectTable, row)
		if err != nil {
			return daptinid.NullReferenceId, "", err
		}
		stored, err = ical.NewDecoder(bytes.NewReader(content)).Decode()
		if err != nil {
			return daptinid.NullReferenceId, "", err
		}
	} else {
		stored = ical.NewCalendar()
		stored.Props.SetText("VERSION", "2.0")
		stored.Props.SetText("PRODID", "-//Daptin//CalDAV Scheduling//EN")
		requestPath = path.Join(collectionPath, uuid.NewString()+".ics")
	}
	var previous *ical.Event
	for _, event := range stored.Events() {
		if itipProp(event.Props.Get("UID")) == uid && itipProp(event.Props.Get("RECURRENCE-ID")) == recurrence {
			copy := event
			previous = &copy
			break
		}
	}
	if previous != nil && latest == nil {
		newer, err := itipNewer(incoming, *previous)
		if err != nil {
			return daptinid.NullReferenceId, "", err
		}
		if !newer {
			return daptinid.InterfaceToDIR(row["reference_id"]), "ignored", nil
		}
	}
	eventRef := daptinid.NullReferenceId
	if row != nil {
		eventRef = daptinid.InterfaceToDIR(row["reference_id"])
	}
	if method == "CANCEL" && recurrence == "" {
		if err := b.calendarDelete(calendarObjectTable, requestPath, row, tx); err != nil {
			return daptinid.NullReferenceId, "", err
		}
		return eventRef, "applied", nil
	}
	if method == "CANCEL" && previous == nil {
		// An instance cancellation needs a master or an existing instance.
		master := false
		for _, event := range stored.Events() {
			master = master || itipProp(event.Props.Get("RECURRENCE-ID")) == ""
		}
		if !master {
			return eventRef, "ignored", nil
		}
	}
	if previous != nil {
		children := stored.Children[:0]
		for _, child := range stored.Children {
			if child.Name != ical.CompEvent || itipProp(child.Props.Get("UID")) != uid || itipProp(child.Props.Get("RECURRENCE-ID")) != recurrence {
				children = append(children, child)
			}
		}
		stored.Children = children
	}
	if method == "REQUEST" && previous != nil {
		itipPreserveAttendeeState(*previous, incoming, accountAddress)
	}
	if method == "CANCEL" {
		incoming.Props.SetText("STATUS", "CANCELLED")
	}
	stored.Children = append(stored.Children, incoming.Component)
	message, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	for _, child := range message.Children {
		if child.Name != "VTIMEZONE" {
			continue
		}
		found := false
		for _, existing := range stored.Children {
			if existing.Name == "VTIMEZONE" && itipProp(existing.Props.Get("TZID")) == itipProp(child.Props.Get("TZID")) {
				found = true
				break
			}
		}
		if !found {
			stored.Children = append(stored.Children, child)
		}
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(stored); err != nil {
		return daptinid.NullReferenceId, "", err
	}
	attrs := map[string]interface{}{
		"content": b.contentValue(calendarObjectTable, requestPath, ical.MIMEType, encoded.Bytes()),
		"uid":     uid, "organizer_address": sender,
	}
	if row != nil {
		changeTag := method == "CANCEL" || previous == nil
		if previous != nil && !changeTag {
			changeTag, err = itipMaterialChange(*previous, incoming)
			if err != nil {
				return daptinid.NullReferenceId, "", err
			}
		}
		if changeTag {
			attrs["schedule_tag"] = uuid.NewString()
		}
		delete(row, "version")
		if err := b.calendarUpdate(calendarObjectTable, requestPath, row, attrs, tx); err != nil {
			return daptinid.NullReferenceId, "", err
		}
		return eventRef, "applied", nil
	}
	attrs["rpath"] = requestPath
	attrs["collection_id"] = collectionRef.String()
	attrs["schedule_tag"] = uuid.NewString()
	if err := b.calendarCreateEvent(requestPath, collection, attrs, tx); err != nil {
		return daptinid.NullReferenceId, "", err
	}
	created, err := b.calendarObjectRow(requestPath, collection, tx)
	if err != nil {
		return daptinid.NullReferenceId, "", err
	}
	return daptinid.InterfaceToDIR(created["reference_id"]), "applied", nil
}

func (a *itipProcessMailAction) findInvitationEvent(b *DaptinDAVBackend, collection map[string]interface{}, collectionPath, uid, organizer string, tx *sqlx.Tx) (map[string]interface{}, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	rows, err := b.calendarRowsWithTransaction(calendarObjectTable, collectionPath, tx,
		Query{ColumnName: "collection_id", Operator: "=", Value: collectionRef.String()},
		Query{ColumnName: "uid", Operator: "=", Value: uid})
	if err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return nil, errors.New("ambiguous calendar event UID")
	}
	var matched map[string]interface{}
	if len(rows) == 1 {
		content, err := b.contentBytes(calendarObjectTable, rows[0])
		if err != nil {
			return nil, err
		}
		calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
		if err != nil {
			return nil, err
		}
		actualUID, actualOrganizer := itipCalendarIdentity(calendar)
		if actualUID != uid || actualOrganizer != organizer {
			return nil, errors.New("calendar event UID belongs to a different organizer")
		}
		matched = rows[0]
	}
	collectionID, err := ResourceRowInt64(collection["id"])
	if err != nil {
		return nil, err
	}
	// Older CalDAV rows predate the searchable identity columns. Bound the
	// resource read and refuse ambiguous or excessively large migrations.
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
		itipLegacyCandidateLimit+1, goqu.Ex{"collection_id": collectionID, "uid": nil})
	if err != nil {
		return nil, err
	}
	if len(refs) > itipLegacyCandidateLimit {
		return nil, errors.New("too many existing calendar events to match invitation")
	}
	for offset := 0; offset < len(refs); offset += 200 {
		end := offset + 200
		if end > len(refs) {
			end = len(refs)
		}
		candidates, err := b.cruds[calendarObjectTable].readByReferenceIDsAfterAuthorizationWithTransaction(
			refs[offset:end], map[string]bool{"content": true}, b.request(http.MethodGet, collectionPath), tx)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			content, err := b.contentBytes(calendarObjectTable, candidate)
			if err != nil {
				return nil, err
			}
			calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
			if err != nil {
				return nil, err
			}
			candidateUID, candidateOrganizer := itipCalendarIdentity(calendar)
			if candidateUID == uid {
				if candidateOrganizer != organizer {
					return nil, errors.New("calendar event UID belongs to a different organizer")
				}
				if matched != nil {
					return nil, errors.New("ambiguous calendar event UID")
				}
				matched = candidate
			}
		}
	}
	return matched, nil
}

func (a *itipProcessMailAction) latestInvitationVersion(collectionRef daptinid.DaptinReferenceId, uid, sender, recipient, recurrence string, tx *sqlx.Tx) (*ical.Event, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return nil, err
	}
	rows, _, err := a.cruds["cal_mail"].GetRowsByWhereClauseWithTransaction("cal_mail", nil, tx, goqu.Ex{
		"collection_id": collectionID, "direction": "inbound", "uid": uid,
		"sender_address": sender, "recipient_address": recipient, "state": "applied",
	})
	if err != nil {
		return nil, err
	}
	var latest *ical.Event
	for _, row := range rows {
		if value := StringOrEmpty(row["recurrence_id"]); value != recurrence && value != "" {
			continue
		}
		calendar, err := ical.NewDecoder(strings.NewReader(StringOrEmpty(row["icalendar"]))).Decode()
		if err != nil {
			return nil, err
		}
		for _, event := range calendar.Events() {
			if itipProp(event.Props.Get("UID")) != uid ||
				(itipProp(event.Props.Get("RECURRENCE-ID")) != recurrence &&
					!(recurrence != "" && row["method"] == "CANCEL" && itipProp(event.Props.Get("RECURRENCE-ID")) == "")) {
				continue
			}
			if latest == nil {
				copy := event
				latest = &copy
				continue
			}
			newer, err := itipNewer(event, *latest)
			if err != nil {
				return nil, err
			}
			if newer {
				copy := event
				latest = &copy
			}
		}
	}
	return latest, nil
}

func (a *itipProcessMailAction) applyPendingCancellations(collectionRef, accountRef daptinid.DaptinReferenceId,
	accountAddress, sender, uid string, recipientID int64, tx *sqlx.Tx) error {
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return err
	}
	rows, _, err := a.cruds["cal_mail"].GetRowsByWhereClauseWithTransaction("cal_mail", nil, tx, goqu.Ex{
		"collection_id": collectionID, "direction": "inbound", "method": "CANCEL",
		"uid": uid, "sender_address": sender, "recipient_address": accountAddress, "state": "pending",
	})
	if err != nil {
		return err
	}
	owner, err := exchangeSessionUser(a.cruds, recipientID, tx)
	if err != nil {
		return err
	}
	b := NewCalDAVBackend(a.cruds, owner, nil)
	for _, row := range rows {
		data := []byte(StringOrEmpty(row["icalendar"]))
		calendar, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
		if err != nil {
			return err
		}
		recurrence := StringOrEmpty(row["recurrence_id"])
		for _, event := range calendar.Events() {
			if itipProp(event.Props.Get("UID")) != uid || itipProp(event.Props.Get("RECURRENCE-ID")) != recurrence {
				continue
			}
			eventRef, state, err := a.applyInvitation(collectionRef, accountRef, accountAddress, sender,
				"CANCEL", uid, recurrence, data, event, recipientID, tx)
			if err != nil {
				return err
			}
			if state == "pending" {
				break
			}
			ref := daptinid.InterfaceToDIR(row["reference_id"])
			attrs := map[string]interface{}{"reference_id": ref.String(), "state": state}
			if eventRef != daptinid.NullReferenceId {
				attrs["event_reference"] = eventRef.String()
			}
			model := api2go.NewApi2GoModelWithData("cal_mail", nil, 0, nil, attrs)
			if _, err := a.cruds["cal_mail"].updateAfterAuthorizationWithTransaction(model,
				b.request(http.MethodPatch, "/api/cal_mail/"+ref.String()), tx); err != nil {
				return err
			}
			break
		}
	}
	return nil
}
