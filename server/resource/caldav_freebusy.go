package resource

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/teambition/rrule-go"
)

const (
	calendarBusyLayout        = "20060102T150405Z"
	calendarBusyMaxCandidates = 1000
	calendarBusyMaxInstances  = 2000
)

type calendarBusyInterval struct {
	start, end time.Time
	kind       string
}

func (b *DaptinDAVBackend) FreeBusy(_ context.Context, requestPath string, start, end time.Time, includeChildren bool) (*ical.Calendar, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusForbidden, err)
	}
	if end.Sub(start) > 366*24*time.Hour {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("free/busy range exceeds one year"))
	}
	tx, err := b.cruds[calendarObjectTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collectionOwnerID, err := b.cruds[calendarCollectionTable].GetReferenceIdToId(USER_ACCOUNT_TABLE_NAME, owner, tx)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	collections, err := GetObjectByWhereClauseWithTransaction(calendarCollectionTable, tx,
		goqu.Ex{"user_account_id": collectionOwnerID, "name": name})
	if err != nil {
		return nil, err
	}
	if len(collections) != 1 {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	allowed := func(table string, ref interface{}) bool {
		crud := b.cruds[table]
		permission := GetObjectPermissionByReferenceIdWithTransaction(table, daptinid.InterfaceToDIR(ref), tx)
		return permission.CanPeek(b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId) ||
			permission.CanRead(b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId)
	}
	for _, table := range []string{calendarCollectionTable, calendarObjectTable} {
		permission := b.cruds[table].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", table, tx)
		admin := b.cruds[table].AdministratorGroupId
		if !permission.CanPeek(b.sessionUser.UserReferenceId, b.sessionUser.Groups, admin) &&
			!permission.CanRead(b.sessionUser.UserReferenceId, b.sessionUser.Groups, admin) {
			return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("free/busy access denied"))
		}
	}
	if !allowed(calendarCollectionTable, collections[0]["reference_id"]) {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("free/busy access denied"))
	}
	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, "-//Daptin//CalDAV Free Busy//EN")
	busy := ical.NewComponent(ical.CompFreeBusy)
	busy.Props.SetText(ical.PropUID, uuid.NewString())
	busy.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	busy.Props.SetDateTime(ical.PropDateTimeStart, start.UTC())
	busy.Props.SetDateTime(ical.PropDateTimeEnd, end.UTC())
	calendar.Children = append(calendar.Children, busy)
	if !includeChildren {
		return calendar, nil
	}
	collectionID, err := ResourceRowInt64(collections[0]["id"])
	if err != nil {
		return nil, err
	}
	references, err := GetLimitedReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
		calendarBusyMaxCandidates+1, goqu.Ex{"collection_id": collectionID})
	if err != nil {
		return nil, err
	}
	if len(references) > calendarBusyMaxCandidates {
		return nil, webdav.NewHTTPError(http.StatusInsufficientStorage, fmt.Errorf("too many calendar objects for free/busy report"))
	}
	authorized := make([]daptinid.DaptinReferenceId, 0, len(references))
	for _, reference := range references {
		if allowed(calendarObjectTable, reference) {
			authorized = append(authorized, reference)
		}
	}
	instances := 0
	for offset := 0; offset < len(authorized); offset += 200 {
		limit := offset + 200
		if limit > len(authorized) {
			limit = len(authorized)
		}
		rows, err := b.cruds[calendarObjectTable].readByReferenceIDsAfterAuthorizationWithTransaction(
			authorized[offset:limit], map[string]bool{"content": true}, b.request(http.MethodGet, requestPath), tx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			data, err := b.contentBytes(calendarObjectTable, row)
			if err != nil {
				return nil, err
			}
			stored, err := ical.NewDecoder(strings.NewReader(string(data))).Decode()
			if err != nil {
				return nil, err
			}
			intervals, err := calendarBusyIntervals(stored, start, end, &instances)
			if err != nil {
				return nil, err
			}
			for _, interval := range intervals {
				prop := ical.NewProp(ical.PropFreeBusy)
				prop.Value = interval.start.UTC().Format(calendarBusyLayout) + "/" + interval.end.UTC().Format(calendarBusyLayout)
				if interval.kind != "BUSY" {
					prop.Params.Set("FBTYPE", interval.kind)
				}
				busy.Props.Add(prop)
			}
		}
	}
	return calendar, nil
}

func calendarBusyIntervals(calendar *ical.Calendar, start, end time.Time, count *int) ([]calendarBusyInterval, error) {
	intervals := make([]calendarBusyInterval, 0)
	overrides := make(map[string]bool)
	for _, component := range calendar.Children {
		if component.Name != ical.CompEvent || component.Props.Get(ical.PropRecurrenceID) == nil {
			continue
		}
		uid, _ := component.Props.Text(ical.PropUID)
		original, err := component.Props.DateTime(ical.PropRecurrenceID, time.UTC)
		if err != nil {
			return nil, err
		}
		overrides[uid+"/"+original.UTC().Format(calendarBusyLayout)] = true
	}
	for _, component := range calendar.Children {
		switch component.Name {
		case ical.CompEvent:
			event := ical.Event{Component: component}
			status, err := event.Status()
			if err != nil {
				return nil, err
			}
			transparency, _ := component.Props.Text("TRANSP")
			if status == ical.EventCancelled || strings.EqualFold(transparency, "TRANSPARENT") {
				continue
			}
			kind := "BUSY"
			if status == ical.EventTentative {
				kind = "BUSY-TENTATIVE"
			}
			eventStart, err := event.DateTimeStart(time.UTC)
			if err != nil {
				return nil, err
			}
			eventEnd, err := event.DateTimeEnd(time.UTC)
			if err != nil || !eventEnd.After(eventStart) {
				return nil, fmt.Errorf("invalid event time range: %v", err)
			}
			uid, _ := component.Props.Text(ical.PropUID)
			if component.Props.Get(ical.PropRecurrenceID) != nil {
				if interval, ok := clipCalendarBusy(eventStart, eventEnd, start, end, kind); ok {
					intervals = append(intervals, interval)
				}
				continue
			}
			roption, err := component.Props.RecurrenceRule()
			if err != nil {
				return nil, err
			}
			var rset *rrule.Set
			if roption != nil || len(component.Props[ical.PropRecurrenceDates]) > 0 {
				rset = &rrule.Set{}
				rset.DTStart(eventStart)
				if roption != nil {
					rule, err := rrule.NewRRule(*roption)
					if err != nil {
						return nil, err
					}
					rset.RRule(rule)
				} else {
					rset.RDate(eventStart)
				}
			}
			if rset != nil {
				for _, date := range component.Props[ical.PropRecurrenceDates] {
					for _, value := range strings.Split(date.Value, ",") {
						dateValue := date
						parts := strings.SplitN(value, "/", 2)
						dateValue.Params = make(ical.Params, len(date.Params))
						for key, values := range date.Params {
							dateValue.Params[key] = append([]string(nil), values...)
						}
						dateValue.Params.Del("VALUE")
						dateValue.Value = parts[0]
						occurred, err := dateValue.DateTime(time.UTC)
						if err != nil {
							return nil, err
						}
						if len(parts) == 2 {
							var periodEnd time.Time
							if strings.HasPrefix(parts[1], "P") || strings.HasPrefix(parts[1], "+P") {
								duration := ical.NewProp(ical.PropDuration)
								duration.Value = parts[1]
								length, err := duration.Duration()
								if err != nil {
									return nil, err
								}
								periodEnd = occurred.Add(length)
							} else {
								dateValue.Value = parts[1]
								periodEnd, err = dateValue.DateTime(time.UTC)
								if err != nil {
									return nil, err
								}
							}
							if !overrides[uid+"/"+occurred.UTC().Format(calendarBusyLayout)] {
								if interval, ok := clipCalendarBusy(occurred, periodEnd, start, end, kind); ok {
									intervals = append(intervals, interval)
								}
							}
							continue
						}
						rset.RDate(occurred)
					}
				}
				for _, date := range component.Props[ical.PropExceptionDates] {
					for _, value := range strings.Split(date.Value, ",") {
						dateValue := date
						dateValue.Value = value
						excluded, err := dateValue.DateTime(time.UTC)
						if err != nil {
							return nil, err
						}
						rset.ExDate(excluded)
					}
				}
				cursor := rset.After(start.Add(-eventEnd.Sub(eventStart)).Add(-time.Nanosecond), false)
				for !cursor.IsZero() && cursor.Before(end) {
					*count++
					if *count > calendarBusyMaxInstances {
						return nil, fmt.Errorf("too many recurring free/busy instances")
					}
					if !overrides[uid+"/"+cursor.UTC().Format(calendarBusyLayout)] {
						if interval, ok := clipCalendarBusy(cursor, cursor.Add(eventEnd.Sub(eventStart)), start, end, kind); ok {
							intervals = append(intervals, interval)
						}
					}
					cursor = rset.After(cursor, false)
				}
				continue
			}
			if interval, ok := clipCalendarBusy(eventStart, eventEnd, start, end, kind); ok {
				intervals = append(intervals, interval)
			}
		case ical.CompFreeBusy:
			for _, property := range component.Props[ical.PropFreeBusy] {
				kind := strings.ToUpper(property.Params.Get("FBTYPE"))
				if kind == "" {
					kind = "BUSY"
				}
				if kind == "FREE" {
					continue
				}
				for _, period := range strings.Split(property.Value, ",") {
					parts := strings.SplitN(period, "/", 2)
					if len(parts) != 2 {
						return nil, fmt.Errorf("invalid VFREEBUSY period")
					}
					periodStart, err := time.Parse(calendarBusyLayout, parts[0])
					if err != nil {
						return nil, err
					}
					periodEnd, err := time.Parse(calendarBusyLayout, parts[1])
					if err != nil {
						duration := ical.NewProp(ical.PropDuration)
						duration.Value = parts[1]
						value, parseErr := duration.Duration()
						if parseErr != nil {
							return nil, parseErr
						}
						periodEnd = periodStart.Add(value)
					}
					if interval, ok := clipCalendarBusy(periodStart, periodEnd, start, end, kind); ok {
						intervals = append(intervals, interval)
					}
				}
			}
		}
	}
	return intervals, nil
}

func clipCalendarBusy(from, to, start, end time.Time, kind string) (calendarBusyInterval, bool) {
	if !to.After(start) || !from.Before(end) {
		return calendarBusyInterval{}, false
	}
	if from.Before(start) {
		from = start
	}
	if to.After(end) {
		to = end
	}
	return calendarBusyInterval{start: from, end: to, kind: kind}, from.Before(to)
}
