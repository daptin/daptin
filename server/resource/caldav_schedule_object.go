package resource

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/emersion/go-ical"
	"github.com/jmoiron/sqlx"
)

// The content ETag changes for every server update. A schedule tag does not
// change when an incoming RSVP only changes another attendee's PARTSTAT.
func calendarScheduleTag(row map[string]interface{}, content []byte) string {
	if row != nil {
		if tag := strings.TrimSpace(StringOrEmpty(row["schedule_tag"])); tag != "" {
			return tag
		}
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
	if err != nil {
		return GetMD5Hash(content)
	}
	for _, event := range calendar.Events() {
		for i := range event.Props["ATTENDEE"] {
			event.Props["ATTENDEE"][i].Params.Del("PARTSTAT")
			event.Props["ATTENDEE"][i].Params.Del("SCHEDULE-STATUS")
		}
		for i := range event.Props["ORGANIZER"] {
			event.Props["ORGANIZER"][i].Params.Del("SCHEDULE-STATUS")
		}
	}
	normalized, err := stdjson.Marshal(calendar)
	if err != nil {
		return GetMD5Hash(content)
	}
	return GetMD5Hash(normalized)
}

func checkScheduleTagCondition(header string, exists bool, row map[string]interface{}, content []byte) error {
	if header == "" {
		return nil
	}
	if !exists || header != strconv.Quote(calendarScheduleTag(row, content)) {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("schedule tag does not match"))
	}
	return nil
}

func itipMaterialChange(old, incoming ical.Event) (bool, error) {
	normalize := func(event ical.Event) (*ical.Component, error) {
		calendar := ical.NewCalendar()
		calendar.Props.SetText("VERSION", "2.0")
		calendar.Props.SetText("PRODID", "-//Daptin//Scheduling//EN")
		calendar.Children = append(calendar.Children, event.Component)
		var encoded bytes.Buffer
		if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
			return nil, err
		}
		copy, err := ical.NewDecoder(&encoded).Decode()
		if err != nil || len(copy.Events()) != 1 {
			return nil, errors.New("invalid scheduling event")
		}
		component := copy.Events()[0].Component
		for i := range component.Props["ATTENDEE"] {
			component.Props["ATTENDEE"][i].Params.Del("PARTSTAT")
			component.Props["ATTENDEE"][i].Params.Del("SCHEDULE-STATUS")
		}
		for i := range component.Props["ORGANIZER"] {
			component.Props["ORGANIZER"][i].Params.Del("SCHEDULE-STATUS")
		}
		return component, nil
	}
	oldComponent, err := normalize(old)
	if err != nil {
		return false, err
	}
	newComponent, err := normalize(incoming)
	if err != nil {
		return false, err
	}
	return !reflect.DeepEqual(oldComponent, newComponent), nil
}

// A matching schedule tag allows the client to retain server-written response
// state for other attendees while editing its own copy of the event.
func mergeScheduleAttendees(current []byte, incoming *ical.Calendar, ownerAddress string) error {
	oldEvents, err := itipEvents(current)
	if err != nil {
		return err
	}
	for _, event := range incoming.Events() {
		key := itipProp(event.Props.Get("UID")) + "\x00" + itipProp(event.Props.Get("RECURRENCE-ID"))
		old, ok := oldEvents[key]
		if !ok {
			continue
		}
		states := make(map[string]ical.Params)
		for _, attendee := range old.Props.Values("ATTENDEE") {
			address := itipAddress(&attendee)
			if address != "" && address != ownerAddress {
				states[address] = attendee.Params
			}
		}
		for i := range event.Props["ATTENDEE"] {
			address := itipAddress(&event.Props["ATTENDEE"][i])
			if params, ok := states[address]; ok {
				if event.Props["ATTENDEE"][i].Params == nil {
					event.Props["ATTENDEE"][i].Params = ical.Params{}
				}
				for _, name := range []string{"PARTSTAT", "SCHEDULE-STATUS"} {
					if value := params.Get(name); value != "" {
						event.Props["ATTENDEE"][i].Params.Set(name, value)
					} else {
						event.Props["ATTENDEE"][i].Params.Del(name)
					}
				}
			}
		}
	}
	return nil
}

// A connected calendar is a scheduling identity. A client may edit its own
// attendee copy, but may not impersonate another organizer or attendee.
func (b *DaptinDAVBackend) validateSchedulingWrite(collection map[string]interface{}, eventRef daptinid.DaptinReferenceId,
	sender string, previous, current []byte, tx *sqlx.Tx) error {
	oldEvents, err := itipEvents(previous)
	if err != nil {
		return err
	}
	newEvents, err := itipEvents(current)
	if err != nil {
		return err
	}
	if len(oldEvents) != 0 && len(newEvents) != 0 {
		attendeeCopy := false
		for _, old := range oldEvents {
			attendeeCopy = itipAddress(old.Props.Get("ORGANIZER")) != sender
			break
		}
		if attendeeCopy {
			allowed, err := itipAllowedCalendarEnvelopeChange(previous, current)
			if err != nil {
				return err
			}
			if !allowed {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("attendee changed organizer-owned calendar fields"))
			}
		}
	}
	var uid, organizer string
	for _, event := range newEvents {
		candidateUID := itipProp(event.Props.Get("UID"))
		candidateOrganizer := itipAddress(event.Props.Get("ORGANIZER"))
		if uid != "" && (candidateUID != uid || candidateOrganizer != organizer) {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar object has conflicting scheduling identities"))
		}
		uid, organizer = candidateUID, candidateOrganizer
		if organizer == "" {
			if len(itipAttendees(event)) != 0 {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("attendee event has no organizer"))
			}
			continue
		}
		if organizer != sender {
			if _, invited := itipAttendees(event)[sender]; !invited {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar account is not an attendee"))
			}
		}
	}
	for key, old := range oldEvents {
		if uid != "" && itipProp(old.Props.Get("UID")) != uid {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("scheduling UID cannot change"))
		}
		now, exists := newEvents[key]
		if !exists {
			if len(newEvents) != 0 && itipAddress(old.Props.Get("ORGANIZER")) != organizer {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("scheduling organizer cannot change"))
			}
			continue
		}
		if itipAddress(old.Props.Get("ORGANIZER")) != itipAddress(now.Props.Get("ORGANIZER")) {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("scheduling organizer cannot change"))
		}
		if organizer != "" && organizer != sender {
			allowed, err := itipAllowedAttendeeChange(old, now, sender)
			if err != nil {
				return err
			}
			if !allowed {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("attendee changed organizer-owned event fields"))
			}
		}
	}
	if len(oldEvents) != 0 {
		for key, now := range newEvents {
			if _, exists := oldEvents[key]; !exists && itipAddress(now.Props.Get("ORGANIZER")) != sender {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("attendee added an unverified recurrence instance"))
			}
		}
	}
	if uid == "" {
		return nil
	}
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	rows, err := b.calendarRowsWithTransaction(calendarObjectTable, "/caldav/"+b.sessionUser.UserReferenceId.String()+"/calendars/", tx,
		Query{ColumnName: "collection_id", Operator: "=", Value: collectionRef.String()},
		Query{ColumnName: "uid", Operator: "=", Value: uid})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if daptinid.InterfaceToDIR(row["reference_id"]) != eventRef {
			return webdav.NewHTTPError(http.StatusConflict, errors.New("calendar UID already exists in this collection"))
		}
	}
	return nil
}

func itipAllowedCalendarEnvelopeChange(previous, current []byte) (bool, error) {
	decode := func(data []byte) (*ical.Calendar, error) {
		calendar, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
		if err != nil {
			return nil, err
		}
		calendar.Props.Del("CALSCALE")
		calendar.Props.Del("PRODID")
		return calendar, nil
	}
	old, err := decode(previous)
	if err != nil {
		return false, err
	}
	now, err := decode(current)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(old.Props, now.Props) {
		return false, nil
	}
	otherChildren := func(calendar *ical.Calendar) []*ical.Component {
		children := make([]*ical.Component, 0)
		for _, child := range calendar.Children {
			if child.Name != "VEVENT" {
				children = append(children, child)
			}
		}
		return children
	}
	return reflect.DeepEqual(otherChildren(old), otherChildren(now)), nil
}

func itipAllowedAttendeeChange(old, incoming ical.Event, attendeeAddress string) (bool, error) {
	// An attendee may exclude additional recurrence instances, but may not
	// reinstate instances that were already excluded by the organizer.
	newExdates := make(map[string]bool)
	for _, prop := range incoming.Props.Values("EXDATE") {
		for _, value := range strings.Split(prop.Value, ",") {
			newExdates[value] = true
		}
	}
	for _, prop := range old.Props.Values("EXDATE") {
		for _, value := range strings.Split(prop.Value, ",") {
			if !newExdates[value] {
				return false, nil
			}
		}
	}
	normalize := func(event ical.Event) (*ical.Component, error) {
		calendar := ical.NewCalendar()
		calendar.Props.SetText("VERSION", "2.0")
		calendar.Props.SetText("PRODID", "-//Daptin//Scheduling//EN")
		calendar.Children = append(calendar.Children, event.Component)
		var encoded bytes.Buffer
		if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
			return nil, err
		}
		copy, err := ical.NewDecoder(&encoded).Decode()
		if err != nil || len(copy.Events()) != 1 {
			return nil, errors.New("invalid scheduling event")
		}
		component := copy.Events()[0].Component
		for _, name := range []string{"TRANSP", "PERCENT-COMPLETE", "COMPLETED", "CREATED", "DTSTAMP", "LAST-MODIFIED", "EXDATE"} {
			component.Props.Del(name)
		}
		for i := range component.Props["ATTENDEE"] {
			if itipAddress(&component.Props["ATTENDEE"][i]) == attendeeAddress {
				component.Props["ATTENDEE"][i].Params.Del("PARTSTAT")
			}
			if strings.ToUpper(component.Props["ATTENDEE"][i].Params.Get("SCHEDULE-AGENT")) == "CLIENT" {
				component.Props["ATTENDEE"][i].Params.Del("SCHEDULE-STATUS")
			}
		}
		children := component.Children[:0]
		for _, child := range component.Children {
			if child.Name != "VALARM" {
				children = append(children, child)
			}
		}
		component.Children = children
		return component, nil
	}
	oldComponent, err := normalize(old)
	if err != nil {
		return false, err
	}
	newComponent, err := normalize(incoming)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(oldComponent, newComponent), nil
}
