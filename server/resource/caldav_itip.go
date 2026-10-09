package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	guerrillamail "github.com/artpar/go-guerrilla/mail"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// scheduleCalendarChange queues iMIP through the configured mail account. The
// calendar write, outbox row, and message record share the DAV transaction.
func (b *DaptinDAVBackend) scheduleCalendarChange(collection map[string]interface{}, eventRef daptinid.DaptinReferenceId, previous, current []byte, statuses map[string]map[string]string, tx *sqlx.Tx) error {
	accountRef := daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"])
	if accountRef == daptinid.NullReferenceId {
		return nil
	}
	account, _, err := b.cruds["mail_account"].GetSingleRowByReferenceIdWithTransaction("mail_account", accountRef, nil, tx)
	if err != nil {
		return err
	}
	sender, _ := account["username"].(string)
	sender = strings.ToLower(strings.TrimSpace(sender))
	if sender == "" {
		return errors.New("calendar scheduling mail account has no address")
	}
	oldEvents, err := itipEvents(previous)
	if err != nil {
		return err
	}
	newEvents, err := itipEvents(current)
	if err != nil {
		return err
	}
	if err := b.validateSchedulingWrite(collection, eventRef, sender, previous, current, tx); err != nil {
		return err
	}
	for key, old := range oldEvents {
		if now, present := newEvents[key]; present {
			if itipAddress(old.Props.Get("ORGANIZER")) == sender {
				for recipient := range itipAttendees(old) {
					_, retained := itipAttendees(now)[recipient]
					if recipient != sender && itipServerSchedules(old, recipient) && (!retained || !itipServerSchedules(now, recipient)) {
						if err := b.queueITIP(collection, account, eventRef, sender, recipient, "CANCEL", previous, old, false, false, tx); err != nil {
							return err
						}
					}
				}
			}
			continue
		}
		if itipAddress(old.Props.Get("ORGANIZER")) == sender {
			for recipient := range itipAttendees(old) {
				if recipient == sender || !itipServerSchedules(old, recipient) {
					continue
				}
				if err := b.queueITIP(collection, account, eventRef, sender, recipient, "CANCEL", previous, old, true, false, tx); err != nil {
					return err
				}
			}
		} else if len(current) == 0 && strings.ToUpper(b.headers.Get("Schedule-Reply")) != "F" && itipOrganizerServerSchedules(old) {
			if _, invited := itipAttendees(old)[sender]; !invited {
				continue
			}
			declined, err := itipDeclinedReply(previous, key, sender)
			if err != nil {
				return err
			}
			if err := b.queueITIP(collection, account, eventRef, sender, itipAddress(old.Props.Get("ORGANIZER")), "REPLY", declined, old, false, false, tx); err != nil {
				return err
			}
		}
	}
	for key, now := range newEvents {
		old, existed := oldEvents[key]
		organizer := itipAddress(now.Props.Get("ORGANIZER"))
		if organizer == sender {
			material := !existed
			if existed {
				material, err = itipMaterialChange(old, now)
				if err != nil {
					return err
				}
			}
			for recipient := range itipAttendees(now) {
				if recipient == sender || !itipServerSchedules(now, recipient) {
					continue
				}
				force := itipForceSend(now, "ATTENDEE", recipient, "REQUEST")
				if !material && !force {
					continue
				}
				if err := b.queueITIP(collection, account, eventRef, sender, recipient, "REQUEST", current, now, false, force, tx); err != nil {
					return err
				}
				markITIPStatus(statuses, key, recipient, "1.0")
			}
		} else if organizer != "" {
			status := itipAttendees(now)[sender]
			force := itipForceSend(now, "ORGANIZER", organizer, "REPLY")
			if status != "" && status != "NEEDS-ACTION" && itipOrganizerServerSchedules(now) && (!existed || itipAttendees(old)[sender] != status || force) {
				if err := b.queueITIP(collection, account, eventRef, sender, organizer, "REPLY", current, now, false, force, tx); err != nil {
					return err
				}
				markITIPStatus(statuses, key, organizer, "1.0")
			}
		}
	}
	return nil
}

func itipForceSend(event ical.Event, property, address, method string) bool {
	for _, candidate := range event.Props.Values(property) {
		if itipAddress(&candidate) == address && strings.EqualFold(candidate.Params.Get("SCHEDULE-FORCE-SEND"), method) {
			return true
		}
	}
	return false
}

func markITIPStatus(statuses map[string]map[string]string, key, address, status string) {
	if statuses == nil {
		return
	}
	if statuses[key] == nil {
		statuses[key] = make(map[string]string)
	}
	statuses[key][address] = status
}

func itipDeclinedReply(previous []byte, key, address string) ([]byte, error) {
	calendar, err := ical.NewDecoder(bytes.NewReader(previous)).Decode()
	if err != nil {
		return nil, err
	}
	for _, event := range itipComponents(calendar) {
		if itipProp(event.Props.Get("UID"))+"\x00"+itipProp(event.Props.Get("RECURRENCE-ID")) != key {
			continue
		}
		for i := range event.Props["ATTENDEE"] {
			if itipAddress(&event.Props["ATTENDEE"][i]) == address {
				if event.Props["ATTENDEE"][i].Params == nil {
					event.Props["ATTENDEE"][i].Params = ical.Params{}
				}
				event.Props["ATTENDEE"][i].Params.Set("PARTSTAT", "DECLINED")
			}
		}
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func itipEvents(data []byte) (map[string]ical.Event, error) {
	events := map[string]ical.Event{}
	if len(data) == 0 {
		return events, nil
	}
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return nil, err
	}
	for _, event := range itipComponents(cal) {
		uid := itipProp(event.Props.Get("UID"))
		if uid == "" {
			return nil, errors.New("scheduled event requires UID")
		}
		key := uid + "\x00" + itipProp(event.Props.Get("RECURRENCE-ID"))
		if _, duplicate := events[key]; duplicate {
			return nil, errors.New("calendar object has duplicate event UID and recurrence")
		}
		events[key] = event
	}
	return events, nil
}

func itipComponents(calendar *ical.Calendar) []ical.Event {
	if calendar == nil {
		return nil
	}
	components := make([]ical.Event, 0, len(calendar.Children))
	for _, child := range calendar.Children {
		if child.Name == ical.CompEvent || child.Name == ical.CompToDo {
			components = append(components, ical.Event{Component: child})
		}
	}
	return components
}

func itipProp(prop *ical.Prop) string {
	if prop == nil {
		return ""
	}
	return prop.Value
}

func itipAddress(prop *ical.Prop) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(itipProp(prop)), "mailto:")))
}

func itipAttendees(event ical.Event) map[string]string {
	attendees := map[string]string{}
	for _, prop := range event.Props.Values("ATTENDEE") {
		address := itipAddress(&prop)
		if address != "" {
			attendees[address] = strings.ToUpper(prop.Params.Get("PARTSTAT"))
		}
	}
	return attendees
}

func itipServerSchedules(event ical.Event, address string) bool {
	for _, prop := range event.Props.Values("ATTENDEE") {
		if itipAddress(&prop) == address {
			agent := strings.ToUpper(prop.Params.Get("SCHEDULE-AGENT"))
			return agent == "" || agent == "SERVER"
		}
	}
	return false
}

func itipOrganizerServerSchedules(event ical.Event) bool {
	organizer := event.Props.Get("ORGANIZER")
	if organizer == nil {
		return false
	}
	agent := strings.ToUpper(organizer.Params.Get("SCHEDULE-AGENT"))
	return agent == "" || agent == "SERVER"
}

func (b *DaptinDAVBackend) queueITIP(collection, account map[string]interface{}, eventRef daptinid.DaptinReferenceId, sender, recipient, method string, source []byte, event ical.Event, cancelEntire, force bool, tx *sqlx.Tx) error {
	cal, err := ical.NewDecoder(bytes.NewReader(source)).Decode()
	if err != nil {
		return err
	}
	// A scheduling message names one recurrence component. Retain timezone
	// definitions, but do not cancel or request unrelated instances.
	children := cal.Children[:0]
	for _, child := range cal.Children {
		if child.Name == "VTIMEZONE" || (child.Name == event.Component.Name &&
			itipProp(child.Props.Get("UID")) == itipProp(event.Props.Get("UID")) &&
			itipProp(child.Props.Get("RECURRENCE-ID")) == itipProp(event.Props.Get("RECURRENCE-ID"))) {
			if child.Name == event.Component.Name {
				child.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
				for _, name := range []string{"ATTENDEE", "ORGANIZER"} {
					for i := range child.Props[name] {
						for _, parameter := range []string{"SCHEDULE-AGENT", "SCHEDULE-FORCE-SEND", "SCHEDULE-STATUS"} {
							child.Props[name][i].Params.Del(parameter)
						}
					}
				}
			}
			if child.Name == event.Component.Name && (method == "CANCEL" || method == "REPLY") {
				attendees := child.Props["ATTENDEE"][:0]
				for _, attendee := range child.Props["ATTENDEE"] {
					if (method == "CANCEL" && itipAddress(&attendee) == recipient) ||
						(method == "REPLY" && itipAddress(&attendee) == sender) {
						attendees = append(attendees, attendee)
					}
				}
				child.Props["ATTENDEE"] = attendees
				if method == "CANCEL" {
					sequence := 0
					if prop := child.Props.Get("SEQUENCE"); prop != nil {
						sequence, err = strconv.Atoi(prop.Value)
					}
					if err != nil || sequence < 0 {
						return errors.New("cancelled event has invalid SEQUENCE")
					}
					child.Props.SetText("SEQUENCE", strconv.Itoa(sequence+1))
					if cancelEntire {
						child.Props.SetText("STATUS", "CANCELLED")
					} else {
						child.Props.Del("STATUS")
					}
				}
			}
			children = append(children, child)
		}
	}
	cal.Children = children
	cal.Props.SetText("METHOD", method)
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(cal); err != nil {
		return err
	}
	body := encoded.String()
	uid := itipProp(event.Props.Get("UID"))
	recurrence := itipProp(event.Props.Get("RECURRENCE-ID"))
	sequence, _ := strconv.Atoi(itipProp(event.Props.Get("SEQUENCE")))
	if method == "CANCEL" {
		sequence++
	}
	forceNonce := ""
	if force {
		forceNonce = uuid.NewString()
	}
	keyBytes := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", collection["reference_id"], eventRef, method, sender, recipient, body, forceNonce)))
	key := hex.EncodeToString(keyBytes[:])
	existing, err := GetReferenceIdByWhereClauseWithTransaction("cal_mail", tx, goqu.Ex{"message_key": key})
	if err != nil {
		return err
	}
	if len(existing) != 0 {
		return nil
	}
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(itipProp(event.Props.Get("SUMMARY")))
	if subject == "" {
		subject = "Calendar event"
	}
	outboxRef, err := b.queueITIPMail(account, sender, recipient, method, subject, body, tx)
	if err != nil {
		return err
	}
	model := api2go.NewApi2GoModelWithData("cal_mail", nil, int64(b.cruds["cal_mail"].TableInfo().DefaultPermission), nil, map[string]interface{}{
		"message_key": key, "event_reference": eventRef.String(), "direction": "outbound", "method": method,
		"uid": uid, "recurrence_id": recurrence, "sequence": sequence, "sender_address": sender,
		"recipient_address": recipient, "icalendar": body, "state": "submitted",
		"collection_id": collection["reference_id"], "outbox_id": outboxRef.String(),
	})
	_, err = b.cruds["cal_mail"].createWithoutFilterAfterAuthorization(model, b.request(http.MethodPost, "/api/cal_mail"), tx)
	return err
}

// Calendar MIME composition stays in the CalDAV adapter. Mail accounts,
// signing certificates, Sent mail, and delivery remain ordinary Daptin mail
// resources; the shared mail.send performer has no calendar branches.
func (b *DaptinDAVBackend) queueITIPMail(account map[string]interface{}, sender, recipient, method, subject, calendarBody string, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	from, err := guerrillamail.NewAddress(sender)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	to, err := guerrillamail.NewAddress(recipient)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	serverRef := daptinid.InterfaceToDIR(account["mail_server_id"])
	if serverRef == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, errors.New("scheduling mail account has no mail server")
	}
	server, _, err := b.cruds["mail_server"].GetSingleRowByReferenceIdWithTransaction("mail_server", serverRef, nil, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if strings.TrimSpace(fmt.Sprint(server["hostname"])) == "" {
		return daptinid.NullReferenceId, errors.New("scheduling mail server has no hostname")
	}
	certManager, err := NewCertificateManager(b.cruds, b.cruds["mail"].ConfigStore, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	cert, err := certManager.GetTLSConfig(from.Host, false, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	keyBlock, _ := pem.Decode([]byte(cert.PrivatePEMDecrypted))
	if keyBlock == nil {
		return daptinid.NullReferenceId, errors.New("scheduling mail certificate has no private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	selector := "d1"
	if value, err := b.cruds["mail"].ConfigStore.GetConfigValueFor("mail.dkim_selector", "backend", tx); err == nil && value != "" {
		selector = value
	}
	var mimeBody bytes.Buffer
	outer := multipart.NewWriter(&mimeBody)
	var plainBody bytes.Buffer
	plain := multipart.NewWriter(&plainBody)
	plainPart, err := plain.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	_, _ = plainPart.Write([]byte(subject))
	if err := plain.Close(); err != nil {
		return daptinid.NullReferenceId, err
	}
	parentPart, err := outer.CreatePart(textproto.MIMEHeader{"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%q", plain.Boundary())}})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	_, _ = parentPart.Write(plainBody.Bytes())
	calendarPart, err := outer.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {fmt.Sprintf("text/calendar; charset=UTF-8; method=%s", method)},
		"Content-Disposition":       {`attachment; filename="invite.ics"`},
		"Content-Transfer-Encoding": {"base64"},
	})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	encodedCalendar := base64.StdEncoding.EncodeToString([]byte(calendarBody))
	for len(encodedCalendar) > 76 {
		_, _ = calendarPart.Write([]byte(encodedCalendar[:76] + "\r\n"))
		encodedCalendar = encodedCalendar[76:]
	}
	_, _ = calendarPart.Write([]byte(encodedCalendar))
	if err := outer.Close(); err != nil {
		return daptinid.NullReferenceId, err
	}
	message := fmt.Sprintf("From: %s\r\nSubject: %s\r\nTo: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n%s",
		from.String(), subject, to.String(), time.Now().Format(time.RFC822Z), uuid.NewString(), from.Host, outer.Boundary(), mimeBody.String())
	var signed bytes.Buffer
	if err := dkim.Sign(&signed, strings.NewReader(message), &dkim.SignOptions{
		Selector: selector, HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization: dkim.CanonicalizationRelaxed, Domain: from.Host, Signer: privateKey,
	}); err != nil {
		return daptinid.NullReferenceId, err
	}
	if _, err := b.cruds["mail"].AppendSentMailForSender(sender, signed.Bytes(), tx); err != nil {
		return daptinid.NullReferenceId, err
	}
	accountOwner, err := b.cruds["mail"].MailAccountSessionUser(account, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	outboxReq := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/api/outbox"}}).
		WithContext(context.WithValue(context.Background(), "user", accountOwner))}
	outbox := api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, map[string]interface{}{
		"from_address": sender, "to_address": recipient, "to_host": to.Host,
		"mail_server_id": serverRef.String(), "mail": b.cruds["outbox"].MailColumnValue("outbox", "mail", signed.Bytes(), subject),
		"sent": false, "retry_count": 0, "next_retry_at": time.Now(),
	})
	created, err := b.cruds["outbox"].createWithoutFilterAfterAuthorization(outbox, outboxReq, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	outboxRef := daptinid.InterfaceToDIR(created["reference_id"])
	if outboxRef == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, errors.New("queued scheduling mail has no outbox reference")
	}
	return outboxRef, nil
}
