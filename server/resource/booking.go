package resource

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type bookableAction struct {
	cruds map[string]*DbResource
	name  string
}

func NewBookableAction(cruds map[string]*DbResource, name string) actionresponse.ActionPerformerInterface {
	return &bookableAction{cruds: cruds, name: name}
}

func (a *bookableAction) Name() string { return "bookable." + a.name }

func (a *bookableAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("booking requires an action transaction")}
	}
	var result map[string]interface{}
	var err error
	switch a.name {
	case "set_calendar":
		result, err = a.setCalendar(fields, tx)
	case "resolve":
		result, err = a.resolve(fields, tx)
	case "slots":
		result, err = a.slots(fields, tx)
	case "reserve":
		result, err = a.reserve(fields, tx)
	case "resolve_booking":
		result, err = a.resolveBooking(fields, tx)
	case "status", "cancel", "reschedule":
		result, err = a.manage(fields, tx)
	default:
		err = errors.New("unknown booking action")
	}
	if err != nil {
		return nil, nil, []error{err}
	}
	model := api2go.NewApi2GoModelWithData("bookable", nil, 0, nil, result)
	return api2go.Response{Res: model}, []actionresponse.ActionResponse{NewActionResponse(a.Name(), result)}, nil
}

func bookingHTTP(status int, message string) error {
	return api2go.NewHTTPError(errors.New(message), message, status)
}

func bookingRef(value interface{}) daptinid.DaptinReferenceId {
	return daptinid.InterfaceToDIR(value)
}

func bookingLinkedRef(table string, value interface{}, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	if ref := daptinid.InterfaceToDIR(value); ref != daptinid.NullReferenceId {
		return ref, nil
	}
	id, err := ResourceRowInt64(value)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	return GetIdToReferenceIdWithTransaction(table, id, tx)
}

func bookingLinkedID(table string, value interface{}, tx *sqlx.Tx) (int64, error) {
	if ref := daptinid.InterfaceToDIR(value); ref != daptinid.NullReferenceId {
		return GetReferenceIdToIdWithTransaction(table, ref, tx)
	}
	return ResourceRowInt64(value)
}

func (a *bookableAction) loadPage(ref daptinid.DaptinReferenceId, lock, requirePublished bool, tx *sqlx.Tx) (map[string]interface{}, error) {
	if ref == daptinid.NullReferenceId {
		return nil, bookingHTTP(http.StatusBadRequest, "bookable_ref must be a reference ID")
	}
	if lock {
		if err := a.cruds["bookable"].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": ref[:]}); err != nil {
			return nil, err
		}
	}
	row, _, err := a.cruds["bookable"].GetSingleRowByReferenceIdWithTransaction("bookable", ref, nil, tx)
	if err != nil || (requirePublished && !bookingBool(row["published"])) {
		return nil, bookingHTTP(http.StatusNotFound, "bookable not found")
	}
	return row, nil
}

func (a *bookableAction) loadReservePage(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	ref := bookingRef(fields["bookable_ref"])
	page, err := a.loadPage(ref, true, false, tx)
	if err != nil {
		return nil, err
	}
	if bookingBool(page["published"]) {
		return page, nil
	}
	key, err := uuid.Parse(StringOrEmpty(fields["attempt_key"]))
	if err != nil || key.Version() != 4 {
		return nil, bookingHTTP(http.StatusNotFound, "bookable not found")
	}
	if existing, err := a.existingAttempt(bookingHash(ref.String(), key.String()), tx); err != nil {
		return nil, err
	} else if existing == nil {
		return nil, bookingHTTP(http.StatusNotFound, "bookable not found")
	}
	return page, nil
}

func bookingBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int64:
		return v != 0
	case int:
		return v != 0
	case []byte:
		return string(v) == "1" || strings.EqualFold(string(v), "true")
	default:
		return strings.EqualFold(fmt.Sprint(value), "true") || fmt.Sprint(value) == "1"
	}
}

func bookingNumber(row map[string]interface{}, key string) (int, error) {
	value, err := ResourceRowInt64(row[key])
	if err != nil {
		return 0, fmt.Errorf("invalid bookable %s: %w", key, err)
	}
	return int(value), nil
}

func (a *bookableAction) owner(page map[string]interface{}, tx *sqlx.Tx) (*auth.SessionUser, error) {
	id, err := bookingLinkedID(USER_ACCOUNT_TABLE_NAME, page["user_account_id"], tx)
	if err != nil {
		return nil, err
	}
	ref, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, id, tx)
	if err != nil {
		return nil, err
	}
	groups := a.cruds["bookable"].GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, tx, "id", id)
	return &auth.SessionUser{UserId: id, UserReferenceId: ref, Groups: groups}, nil
}

func (a *bookableAction) resolve(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	var page map[string]interface{}
	var err error
	if _, reserve := fields["attempt_key"]; reserve {
		page, err = a.loadReservePage(fields, tx)
	} else {
		page, err = a.loadPage(bookingRef(fields["bookable_ref"]), false, true, tx)
	}
	if err != nil {
		return nil, err
	}
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"owner_ref": owner.UserReferenceId.String()}, nil
}

func (a *bookableAction) setCalendar(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	pageRef := bookingRef(fields["bookable_ref"])
	calendarRef := bookingRef(fields["calendar_ref"])
	if pageRef == daptinid.NullReferenceId || calendarRef == daptinid.NullReferenceId {
		return nil, bookingHTTP(http.StatusBadRequest, "bookable_ref and calendar_ref must be reference IDs")
	}
	if err := a.cruds["bookable"].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": pageRef[:]}); err != nil {
		return nil, err
	}
	page, _, err := a.cruds["bookable"].GetSingleRowByReferenceIdWithTransaction("bookable", pageRef, nil, tx)
	if err != nil {
		return nil, bookingHTTP(http.StatusNotFound, "bookable not found")
	}
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, err
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller.UserReferenceId != owner.UserReferenceId {
		return nil, bookingHTTP(http.StatusForbidden, "bookable management denied")
	}
	pageCRUD := a.cruds["bookable"]
	pageTable := pageCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "bookable", tx)
	pageRow := GetObjectPermissionByReferenceIdWithTransaction("bookable", pageRef, tx)
	if !pageTable.CanUpdate(owner.UserReferenceId, owner.Groups, pageCRUD.AdministratorGroupId) ||
		!pageRow.CanUpdate(owner.UserReferenceId, owner.Groups, pageCRUD.AdministratorGroupId) {
		return nil, bookingHTTP(http.StatusForbidden, "bookable update denied")
	}
	enabled, ok := fields["enabled"].(bool)
	if !ok {
		return nil, bookingHTTP(http.StatusBadRequest, "enabled must be a boolean")
	}
	if enabled && (!pageTable.CanRefer(owner.UserReferenceId, owner.Groups, pageCRUD.AdministratorGroupId) ||
		!pageRow.CanRefer(owner.UserReferenceId, owner.Groups, pageCRUD.AdministratorGroupId)) {
		return nil, bookingHTTP(http.StatusForbidden, "bookable reference denied")
	}
	pageID, err := bookingLinkedID("bookable", page["id"], tx)
	if err != nil {
		return nil, err
	}
	calendarID, err := GetReferenceIdToIdWithTransaction("collection", calendarRef, tx)
	if err != nil {
		return nil, bookingHTTP(http.StatusNotFound, "calendar not found")
	}
	join := "bookable_bookable_id_has_collection_collection_id"
	links, err := GetObjectByWhereClauseWithTransaction(join, tx, goqu.Ex{"bookable_id": pageID, "collection_id": calendarID})
	if err != nil {
		return nil, err
	}
	if enabled {
		collection := a.cruds["collection"]
		table := collection.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "collection", tx)
		row := GetObjectPermissionByReferenceIdWithTransaction("collection", calendarRef, tx)
		admin := collection.AdministratorGroupId
		if (!table.CanPeek(owner.UserReferenceId, owner.Groups, admin) && !table.CanRead(owner.UserReferenceId, owner.Groups, admin)) ||
			(!row.CanPeek(owner.UserReferenceId, owner.Groups, admin) && !row.CanRead(owner.UserReferenceId, owner.Groups, admin)) ||
			!table.CanRefer(owner.UserReferenceId, owner.Groups, admin) || !row.CanRefer(owner.UserReferenceId, owner.Groups, admin) {
			return nil, bookingHTTP(http.StatusForbidden, "required calendar reference denied")
		}
		if len(links) == 0 {
			model := api2go.NewApi2GoModelWithData(join, nil, 0, nil, map[string]interface{}{"bookable_id": pageRef.String(), "collection_id": calendarRef.String()})
			if _, err := a.cruds[join].createAfterAuthorizationWithTransaction(model, a.request(http.MethodPost, "/action/bookable/set_calendar", owner), tx); err != nil {
				return nil, err
			}
		}
	} else {
		for _, link := range links {
			if _, err := a.cruds[join].deleteAfterAuthorizationWithTransaction(bookingRef(link["reference_id"]), a.request(http.MethodDelete, "/action/bookable/set_calendar", owner), tx); err != nil {
				return nil, err
			}
		}
	}
	return map[string]interface{}{"bookable_ref": pageRef.String(), "calendar_ref": calendarRef.String(), "enabled": enabled}, nil
}

type bookingCalendars struct {
	destination map[string]interface{}
	sources     []map[string]interface{}
	owner       *auth.SessionUser
	backend     *DaptinDAVBackend
}

func (a *bookableAction) calendars(page map[string]interface{}, owner *auth.SessionUser, tx *sqlx.Tx) (*bookingCalendars, error) {
	pageID, err := ResourceRowInt64(page["id"])
	if err != nil {
		return nil, err
	}
	destinationID, err := bookingLinkedID("collection", page["destination_collection_id"], tx)
	if err != nil {
		return nil, bookingHTTP(http.StatusConflict, "destination calendar is missing")
	}
	destinationRef, err := GetIdToReferenceIdWithTransaction("collection", destinationID, tx)
	if err != nil {
		return nil, err
	}
	links, err := GetObjectByWhereClauseWithTransaction("bookable_bookable_id_has_collection_collection_id", tx, goqu.Ex{"bookable_id": pageID})
	if err != nil {
		return nil, err
	}
	if len(links) == 0 || len(links) > 20 {
		return nil, bookingHTTP(http.StatusConflict, "bookable needs 1 to 20 required calendars")
	}
	refs := []daptinid.DaptinReferenceId{destinationRef}
	for _, link := range links {
		id, err := bookingLinkedID("collection", link["collection_id"], tx)
		if err != nil {
			return nil, err
		}
		ref, err := GetIdToReferenceIdWithTransaction("collection", id, tx)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	b := NewCalDAVBackend(a.cruds, owner, nil)
	rows := make(map[daptinid.DaptinReferenceId]map[string]interface{})
	for _, ref := range refs {
		if rows[ref] != nil {
			continue
		}
		row, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", ref, nil, tx)
		if err != nil {
			return nil, err
		}
		rows[ref] = row
	}
	if a.name != "slots" {
		for _, ref := range refs {
			if err := b.lockCalendarCollection(rows[ref], tx); err != nil {
				return nil, err
			}
		}
	}
	collectionCRUD := a.cruds["collection"]
	collectionTable := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "collection", tx)
	collectionAdmin := collectionCRUD.AdministratorGroupId
	eventCRUD := a.cruds["calendar"]
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "calendar", tx)
	eventAdmin := eventCRUD.AdministratorGroupId
	user, groups := owner.UserReferenceId, owner.Groups
	destinationGrant := GetObjectPermissionByReferenceIdWithTransaction("collection", destinationRef, tx)
	if !collectionTable.CanCreate(user, groups, collectionAdmin) || !collectionTable.CanRefer(user, groups, collectionAdmin) ||
		!destinationGrant.CanCreate(user, groups, collectionAdmin) || !destinationGrant.CanRefer(user, groups, collectionAdmin) ||
		!eventTable.CanCreate(user, groups, eventAdmin) || !eventTable.CanRead(user, groups, eventAdmin) {
		return nil, bookingHTTP(http.StatusConflict, "destination calendar is not writable by the bookable owner")
	}
	sources := make([]map[string]interface{}, 0, len(links))
	seen := map[daptinid.DaptinReferenceId]bool{}
	for _, link := range links {
		id, _ := bookingLinkedID("collection", link["collection_id"], tx)
		ref, _ := GetIdToReferenceIdWithTransaction("collection", id, tx)
		if seen[ref] {
			continue
		}
		seen[ref] = true
		grant := GetObjectPermissionByReferenceIdWithTransaction("collection", ref, tx)
		if (!collectionTable.CanPeek(user, groups, collectionAdmin) && !collectionTable.CanRead(user, groups, collectionAdmin)) ||
			(!grant.CanPeek(user, groups, collectionAdmin) && !grant.CanRead(user, groups, collectionAdmin)) ||
			!collectionTable.CanRefer(user, groups, collectionAdmin) || !grant.CanRefer(user, groups, collectionAdmin) ||
			(!eventTable.CanPeek(user, groups, eventAdmin) && !eventTable.CanRead(user, groups, eventAdmin)) {
			return nil, bookingHTTP(http.StatusConflict, "required calendar is not readable by the bookable owner")
		}
		sources = append(sources, rows[ref])
	}
	return &bookingCalendars{destination: rows[destinationRef], sources: sources, owner: owner, backend: b}, nil
}

type bookingWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type bookingPolicy struct {
	zone                                                                         *time.Location
	duration, increment, before, after, notice, horizon, capacity, daily, weekly int
	hours                                                                        map[string][]bookingWindow
}

type bookingQuestion struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

func bookingQuestions(page map[string]interface{}) ([]bookingQuestion, error) {
	raw := StringOrEmpty(page["questions"])
	if raw == "" || raw == "null" {
		return nil, nil
	}
	if len(raw) > 16000 {
		return nil, bookingHTTP(http.StatusConflict, "bookable questions are too large")
	}
	var questions []bookingQuestion
	if err := encodingjson.Unmarshal([]byte(raw), &questions); err != nil || len(questions) > 20 {
		return nil, bookingHTTP(http.StatusConflict, "invalid bookable questions")
	}
	seen := map[string]bool{}
	for _, question := range questions {
		if len(question.ID) == 0 || len(question.ID) > 40 || len(question.Label) == 0 || len(question.Label) > 200 || seen[question.ID] {
			return nil, bookingHTTP(http.StatusConflict, "invalid bookable questions")
		}
		for _, char := range question.ID {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_') {
				return nil, bookingHTTP(http.StatusConflict, "invalid bookable question ID")
			}
		}
		seen[question.ID] = true
	}
	return questions, nil
}

func canonicalBookingAnswers(value interface{}) (string, map[string]string, error) {
	var raw []byte
	var err error
	switch typed := value.(type) {
	case nil:
		raw = []byte("{}")
	case string:
		if strings.TrimSpace(typed) == "" {
			raw = []byte("{}")
		} else {
			raw = []byte(typed)
		}
	default:
		raw, err = encodingjson.Marshal(typed)
		if err != nil {
			return "", nil, bookingHTTP(http.StatusBadRequest, "invalid question answers")
		}
	}
	if len(raw) > 20000 {
		return "", nil, bookingHTTP(http.StatusBadRequest, "question answers are too large")
	}
	var answers map[string]string
	if err := encodingjson.Unmarshal(raw, &answers); err != nil || answers == nil {
		return "", nil, bookingHTTP(http.StatusBadRequest, "invalid question answers")
	}
	encoded, err := encodingjson.Marshal(answers)
	if err != nil {
		return "", nil, err
	}
	return string(encoded), answers, nil
}

func bookingAnswers(page map[string]interface{}, value interface{}) (string, error) {
	questions, err := bookingQuestions(page)
	if err != nil {
		return "", err
	}
	encoded, answers, err := canonicalBookingAnswers(value)
	if err != nil {
		return "", err
	}
	allowed := make(map[string]bookingQuestion, len(questions))
	for _, question := range questions {
		allowed[question.ID] = question
	}
	for key, answer := range answers {
		if _, ok := allowed[key]; !ok || len(answer) > 1000 {
			return "", bookingHTTP(http.StatusBadRequest, "invalid question answer")
		}
	}
	for _, question := range questions {
		if question.Required && strings.TrimSpace(answers[question.ID]) == "" {
			return "", bookingHTTP(http.StatusBadRequest, "required question answer is missing")
		}
	}
	return encoded, nil
}

func parseBookingPolicy(page map[string]interface{}) (*bookingPolicy, error) {
	zone, err := time.LoadLocation(StringOrEmpty(page["time_zone"]))
	if err != nil {
		return nil, bookingHTTP(http.StatusConflict, "invalid bookable time zone")
	}
	p := &bookingPolicy{zone: zone}
	for _, item := range []struct {
		name   string
		target *int
	}{
		{"duration_minutes", &p.duration}, {"increment_minutes", &p.increment},
		{"buffer_before_minutes", &p.before}, {"buffer_after_minutes", &p.after},
		{"minimum_notice_minutes", &p.notice}, {"horizon_days", &p.horizon},
		{"capacity", &p.capacity}, {"daily_limit", &p.daily}, {"weekly_limit", &p.weekly},
	} {
		*item.target, err = bookingNumber(page, item.name)
		if err != nil {
			return nil, err
		}
	}
	if p.duration < 1 || p.duration > 1440 || p.increment < 1 || p.increment > 1440 ||
		p.before < 0 || p.before > 1440 || p.after < 0 || p.after > 1440 || p.notice < 0 || p.notice > 525600 || p.horizon < 1 || p.horizon > 366 ||
		p.capacity < 1 || p.daily < 0 || p.weekly < 0 {
		return nil, bookingHTTP(http.StatusConflict, "invalid bookable limits")
	}
	raw := []byte(StringOrEmpty(page["weekly_hours"]))
	if len(raw) > 8192 {
		return nil, bookingHTTP(http.StatusConflict, "weekly hours are too large")
	}
	if err := encodingjson.Unmarshal(raw, &p.hours); err != nil {
		return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
	}
	validDays := map[string]bool{"sun": true, "mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true}
	if len(p.hours) > 7 {
		return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
	}
	for day, windows := range p.hours {
		if !validDays[day] || len(windows) > 8 {
			return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
		}
		previousEnd := ""
		for _, window := range windows {
			if _, err := time.Parse("15:04", window.Start); err != nil {
				return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
			}
			if _, err := time.Parse("15:04", window.End); err != nil {
				return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
			}
			if window.Start >= window.End || window.Start < previousEnd {
				return nil, bookingHTTP(http.StatusConflict, "invalid weekly hours")
			}
			previousEnd = window.End
		}
	}
	return p, nil
}

func bookingLocalTime(day time.Time, clock string, zone *time.Location) (time.Time, bool) {
	parts, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, false
	}
	instant := time.Date(day.Year(), day.Month(), day.Day(), parts.Hour(), parts.Minute(), 0, 0, zone)
	local := instant.In(zone)
	if local.Year() != day.Year() || local.Month() != day.Month() || local.Day() != day.Day() || local.Hour() != parts.Hour() || local.Minute() != parts.Minute() {
		return time.Time{}, false
	}
	_, offset := instant.Zone()
	for _, neighbor := range []time.Time{instant.Add(-24 * time.Hour), instant.Add(24 * time.Hour)} {
		_, otherOffset := neighbor.In(zone).Zone()
		if otherOffset == offset {
			continue
		}
		other := instant.Add(time.Duration(offset-otherOffset) * time.Second).In(zone)
		if other.Year() == day.Year() && other.Month() == day.Month() && other.Day() == day.Day() && other.Hour() == parts.Hour() && other.Minute() == parts.Minute() {
			return time.Time{}, false // an ambiguous wall clock time needs an unambiguous offset
		}
	}
	return instant, true
}

func bookingDay(value string, zone *time.Location) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, bookingHTTP(http.StatusBadRequest, "date must be YYYY-MM-DD")
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, zone), nil
}

func bookingTime(value string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, bookingHTTP(http.StatusBadRequest, "start must include a time zone offset")
	}
	return instant.UTC(), nil
}

func bookingStoredTime(value interface{}) (time.Time, error) {
	if instant, ok := value.(time.Time); ok {
		return instant.UTC(), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if instant, err := time.Parse(layout, StringOrEmpty(value)); err == nil {
			return instant.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid stored booking time")
}

func (a *bookableAction) bookingRows(page map[string]interface{}, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	id, err := ResourceRowInt64(page["id"])
	if err != nil {
		return nil, err
	}
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("booking", tx, 10001, goqu.Ex{"bookable_id": id})
	if err != nil {
		return nil, err
	}
	if len(refs) > 10000 {
		return nil, bookingHTTP(http.StatusConflict, "bookable has too many bookings")
	}
	rows := make([]map[string]interface{}, 0, len(refs))
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, err
	}
	for offset := 0; offset < len(refs); offset += 200 {
		end := offset + 200
		if end > len(refs) {
			end = len(refs)
		}
		batch, err := a.cruds["booking"].readByReferenceIDsAfterAuthorizationWithTransaction(refs[offset:end], nil,
			a.request(http.MethodGet, "/api/booking", owner), tx)
		if err != nil {
			return nil, err
		}
		if len(batch) != end-offset {
			return nil, errors.New("booking reservation rows are incomplete")
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}

func (a *bookableAction) busy(page map[string]interface{}, calendars *bookingCalendars, bookings []map[string]interface{}, day time.Time, p *bookingPolicy, tx *sqlx.Tx) ([]calendarBusyInterval, error) {
	ignored := make(map[daptinid.DaptinReferenceId]bool)
	for _, booking := range bookings {
		if StringOrEmpty(booking["state"]) != "confirmed" {
			continue
		}
		ref, err := bookingLinkedRef("calendar", booking["calendar_id"], tx)
		if err != nil {
			return nil, err
		}
		ignored[ref] = true
	}
	start := day.Add(-48 * time.Hour)
	end := day.Add(72 * time.Hour)
	instances := 0
	candidates := 0
	intervals := make([]calendarBusyInterval, 0)
	for _, collection := range calendars.sources {
		id, err := bookingLinkedID("collection", collection["id"], tx)
		if err != nil {
			return nil, err
		}
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("calendar", tx, uint(calendarBusyMaxCandidates-candidates+1), goqu.Ex{"collection_id": id})
		if err != nil {
			return nil, err
		}
		candidates += len(refs)
		if candidates > calendarBusyMaxCandidates {
			return nil, bookingHTTP(http.StatusConflict, "required calendars exceed availability limit")
		}
		authorized := make([]daptinid.DaptinReferenceId, 0, len(refs))
		crud := a.cruds["calendar"]
		for _, ref := range refs {
			if ignored[ref] {
				continue
			}
			grant := GetObjectPermissionByReferenceIdWithTransaction("calendar", ref, tx)
			if !grant.CanPeek(calendars.owner.UserReferenceId, calendars.owner.Groups, crud.AdministratorGroupId) &&
				!grant.CanRead(calendars.owner.UserReferenceId, calendars.owner.Groups, crud.AdministratorGroupId) {
				return nil, bookingHTTP(http.StatusConflict, "required calendar contains an unreadable event")
			}
			authorized = append(authorized, ref)
		}
		for offset := 0; offset < len(authorized); offset += 200 {
			limit := offset + 200
			if limit > len(authorized) {
				limit = len(authorized)
			}
			rows, err := crud.readByReferenceIDsAfterAuthorizationWithTransaction(authorized[offset:limit], map[string]bool{"content": true},
				a.request(http.MethodGet, "/api/calendar", calendars.owner), tx)
			if err != nil {
				return nil, err
			}
			if len(rows) != limit-offset {
				return nil, errors.New("calendar availability rows are incomplete")
			}
			for _, row := range rows {
				data, err := calendars.backend.contentBytes("calendar", row)
				if err != nil {
					return nil, err
				}
				stored, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
				if err != nil {
					return nil, err
				}
				part, err := calendarBusyIntervals(stored, start, end, &instances)
				if err != nil {
					return nil, err
				}
				intervals = append(intervals, part...)
			}
		}
	}
	return intervals, nil
}

func bookingOverlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

func bookingWeek(day time.Time) string {
	year, week := day.ISOWeek()
	return fmt.Sprintf("%04d-%02d", year, week)
}

func bookingWindowStarts(day time.Time, window bookingWindow, p *bookingPolicy) []time.Time {
	opening, _ := time.Parse("15:04", window.Start) // validated by parseBookingPolicy
	closing, _ := time.Parse("15:04", window.End)
	from := opening.Hour()*60 + opening.Minute()
	until := closing.Hour()*60 + closing.Minute()
	starts := make([]time.Time, 0)
	for minute := from; minute < until; minute += p.increment {
		start, valid := bookingLocalTime(day, fmt.Sprintf("%02d:%02d", minute/60, minute%60), p.zone)
		if !valid {
			continue
		}
		end := start.Add(time.Duration(p.duration) * time.Minute).In(p.zone)
		if end.Year() != day.Year() || end.Month() != day.Month() || end.Day() != day.Day() ||
			end.Hour()*60+end.Minute() > until {
			continue
		}
		if _, valid := bookingLocalTime(day, end.Format("15:04"), p.zone); valid {
			starts = append(starts, start)
		}
	}
	return starts
}

func (a *bookableAction) available(page map[string]interface{}, calendars *bookingCalendars, bookings []map[string]interface{}, p *bookingPolicy, day time.Time, skip daptinid.DaptinReferenceId, tx *sqlx.Tx) ([]time.Time, error) {
	now := time.Now().In(p.zone)
	if day.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, p.zone)) || day.After(now.AddDate(0, 0, p.horizon)) {
		return []time.Time{}, nil
	}
	busy, err := a.busy(page, calendars, bookings, day, p, tx)
	if err != nil {
		return nil, err
	}
	active := make([]struct{ start, end time.Time }, 0, len(bookings))
	for _, booking := range bookings {
		if StringOrEmpty(booking["state"]) != "confirmed" {
			continue
		}
		if bookingRef(booking["reference_id"]) == skip {
			continue
		}
		start, err := bookingStoredTime(booking["starts_at"])
		if err != nil {
			return nil, err
		}
		end, err := bookingStoredTime(booking["ends_at"])
		if err != nil {
			return nil, err
		}
		active = append(active, struct{ start, end time.Time }{start.In(p.zone), end.In(p.zone)})
	}
	result := make([]time.Time, 0)
	key := strings.ToLower(day.Weekday().String()[:3])
	for _, window := range p.hours[key] {
		for _, start := range bookingWindowStarts(day, window, p) {
			local := start.In(p.zone)
			if start.Before(now.Add(time.Duration(p.notice) * time.Minute)) {
				continue
			}
			end := start.Add(time.Duration(p.duration) * time.Minute)
			blocked := false
			for _, interval := range busy {
				if bookingOverlaps(start.Add(-time.Duration(p.before)*time.Minute), end.Add(time.Duration(p.after)*time.Minute), interval.start, interval.end) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			slotCount, dailyCount, weeklyCount := 0, 0, 0
			for _, previous := range active {
				if previous.start.Equal(start) {
					slotCount++
				} else if bookingOverlaps(start.Add(-time.Duration(p.before)*time.Minute), end.Add(time.Duration(p.after)*time.Minute),
					previous.start.Add(-time.Duration(p.before)*time.Minute), previous.end.Add(time.Duration(p.after)*time.Minute)) {
					blocked = true
				}
				if previous.start.Format("2006-01-02") == local.Format("2006-01-02") {
					dailyCount++
				}
				if bookingWeek(previous.start) == bookingWeek(local) {
					weeklyCount++
				}
			}
			if blocked || slotCount >= p.capacity || (p.daily > 0 && dailyCount >= p.daily) || (p.weekly > 0 && weeklyCount >= p.weekly) {
				continue
			}
			result = append(result, start.UTC())
		}
	}
	return result, nil
}

func (a *bookableAction) slots(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	page, err := a.loadPage(bookingRef(fields["bookable_ref"]), false, true, tx)
	if err != nil {
		return nil, err
	}
	p, err := parseBookingPolicy(page)
	if err != nil {
		return nil, err
	}
	day, err := bookingDay(StringOrEmpty(fields["date"]), p.zone)
	if err != nil {
		return nil, err
	}
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, err
	}
	active, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || active.UserReferenceId != owner.UserReferenceId {
		return nil, bookingHTTP(http.StatusForbidden, "bookable owner context is required")
	}
	calendars, err := a.calendars(page, owner, tx)
	if err != nil {
		return nil, err
	}
	bookings, err := a.bookingRows(page, tx)
	if err != nil {
		return nil, err
	}
	instants, err := a.available(page, calendars, bookings, p, day, daptinid.NullReferenceId, tx)
	if err != nil {
		return nil, err
	}
	slots := make([]string, len(instants))
	for i, instant := range instants {
		slots[i] = instant.Format(time.RFC3339)
	}
	questions, err := bookingQuestions(page)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"title": page["title"], "date": day.Format("2006-01-02"), "time_zone": page["time_zone"], "duration_minutes": p.duration, "questions": questions, "slots": slots}, nil
}

func bookingHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (a *bookableAction) token(ref daptinid.DaptinReferenceId) string {
	if len(a.cruds["bookable"].EncryptionSecret) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, a.cruds["bookable"].EncryptionSecret)
	mac.Write([]byte("booking-token-v1\x00" + ref.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *bookableAction) bookingResult(row map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	ref := bookingRef(row["reference_id"])
	token := a.token(ref)
	if token == "" {
		return nil, bookingHTTP(http.StatusServiceUnavailable, "booking token secret is unavailable")
	}
	start, err := bookingStoredTime(row["starts_at"])
	if err != nil {
		return nil, err
	}
	end, err := bookingStoredTime(row["ends_at"])
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"booking_ref": ref.String(), "state": row["state"], "start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "token": token}, nil
}

func (a *bookableAction) request(method, route string, user *auth.SessionUser) api2go.Request {
	httpRequest := (&http.Request{Method: method, URL: &url.URL{Path: route}}).WithContext(context.WithValue(context.Background(), "user", user))
	return api2go.Request{PlainRequest: httpRequest}
}

func (a *bookableAction) existingAttempt(hash string, tx *sqlx.Tx) (map[string]interface{}, error) {
	refs, err := GetReferenceIdByWhereClauseWithTransaction("booking", tx, goqu.Ex{"attempt_hash": hash})
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}
	if len(refs) != 1 {
		return nil, errors.New("booking attempt is not unique")
	}
	row, _, err := a.cruds["booking"].GetSingleRowByReferenceIdWithTransaction("booking", refs[0], nil, tx)
	return row, err
}

func (a *bookableAction) eventCalendar(page, collection map[string]interface{}, guestName, guestEmail string, start, end time.Time, uid string, tx *sqlx.Tx) (*ical.Calendar, string, error) {
	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, "-//Daptin//Booking//EN")
	event := ical.NewComponent(ical.CompEvent)
	event.Props.SetText(ical.PropUID, uid)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	event.Props.SetDateTime(ical.PropDateTimeStart, start.UTC())
	event.Props.SetDateTime(ical.PropDateTimeEnd, end.UTC())
	event.Props.SetText(ical.PropSummary, "Appointment")
	setBookingProp(event, "SEQUENCE", "0")
	eventClass := strings.ToUpper(strings.TrimSpace(StringOrEmpty(page["event_class"])))
	if eventClass != "PRIVATE" && eventClass != "PUBLIC" && eventClass != "CONFIDENTIAL" {
		return nil, "", bookingHTTP(http.StatusConflict, "invalid bookable event class")
	}
	setBookingProp(event, "CLASS", eventClass)
	organizer := ""
	accountRef := bookingRef(collection["scheduling_mail_account_id"])
	if accountRef != daptinid.NullReferenceId {
		account, _, err := a.cruds["mail_account"].GetSingleRowByReferenceIdWithTransaction("mail_account", accountRef, nil, tx)
		if err != nil {
			return nil, "", err
		}
		organizer = strings.ToLower(strings.TrimSpace(StringOrEmpty(account["username"])))
		if organizer == "" {
			return nil, "", bookingHTTP(http.StatusConflict, "calendar mail account has no sender address")
		}
		setBookingProp(event, "ORGANIZER", "mailto:"+organizer)
		attendee := ical.NewProp("ATTENDEE")
		attendee.Value = "mailto:" + guestEmail
		attendee.Params.Set("CN", guestName)
		attendee.Params.Set("PARTSTAT", "NEEDS-ACTION")
		attendee.Params.Set("ROLE", "REQ-PARTICIPANT")
		event.Props.Add(attendee)
	}
	calendar.Children = append(calendar.Children, event)
	return calendar, organizer, nil
}

func setBookingProp(component *ical.Component, name, value string) {
	property := ical.NewProp(name)
	property.Value = value
	component.Props.Set(property)
}

func bookingEncode(calendar *ical.Calendar) ([]byte, error) {
	var buffer bytes.Buffer
	if err := ical.NewEncoder(&buffer).Encode(calendar); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func advanceBookingSequence(event ical.Event) error {
	current, _ := event.Props.Text("SEQUENCE")
	if current == "" {
		current = "0"
	}
	value, err := strconv.Atoi(current)
	if err != nil || value < 0 || value >= 1000000000 {
		return bookingHTTP(http.StatusConflict, "booking event has an invalid sequence")
	}
	setBookingProp(event.Component, "SEQUENCE", strconv.Itoa(value+1))
	return nil
}

func (a *bookableAction) destinationPath(collection map[string]interface{}, file string, tx *sqlx.Tx) (string, error) {
	id, err := bookingLinkedID(USER_ACCOUNT_TABLE_NAME, collection["user_account_id"], tx)
	if err != nil {
		return "", err
	}
	ownerRef, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, id, tx)
	if err != nil {
		return "", err
	}
	name := StringOrEmpty(collection["name"])
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid destination calendar name")
	}
	return path.Join("/caldav", ownerRef.String(), "calendars", name, file), nil
}

func (a *bookableAction) reserve(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	pageRef := bookingRef(fields["bookable_ref"])
	page, err := a.loadReservePage(fields, tx)
	if err != nil {
		return nil, err
	}
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, err
	}
	active, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || active.UserReferenceId != owner.UserReferenceId {
		return nil, bookingHTTP(http.StatusForbidden, "bookable owner context is required")
	}
	start, err := bookingTime(StringOrEmpty(fields["start"]))
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(StringOrEmpty(fields["guest_name"]))
	email := strings.ToLower(strings.TrimSpace(StringOrEmpty(fields["guest_email"])))
	if len(name) == 0 || len(name) > 200 || len(email) > 200 {
		return nil, bookingHTTP(http.StatusBadRequest, "invalid guest details")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return nil, bookingHTTP(http.StatusBadRequest, "invalid guest email")
	}
	key, err := uuid.Parse(StringOrEmpty(fields["attempt_key"]))
	if err != nil || key.Version() != 4 {
		return nil, bookingHTTP(http.StatusBadRequest, "attempt_key must be a UUIDv4")
	}
	attemptHash := bookingHash(pageRef.String(), key.String())
	answers, _, err := canonicalBookingAnswers(fields["answers"])
	if err != nil {
		return nil, err
	}
	requestHash := bookingHash(pageRef.String(), start.Format(time.RFC3339Nano), name, email, answers)
	existing, err := a.existingAttempt(attemptHash, tx)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if StringOrEmpty(existing["request_hash"]) != requestHash {
			return nil, bookingHTTP(http.StatusConflict, "attempt key was used for another request")
		}
		return a.bookingResult(existing, tx)
	}
	p, err := parseBookingPolicy(page)
	if err != nil {
		return nil, err
	}
	if _, err = bookingAnswers(page, fields["answers"]); err != nil {
		return nil, err
	}
	calendars, err := a.calendars(page, owner, tx)
	if err != nil {
		return nil, err
	}
	bookings, err := a.bookingRows(page, tx)
	if err != nil {
		return nil, err
	}
	day := time.Date(start.In(p.zone).Year(), start.In(p.zone).Month(), start.In(p.zone).Day(), 0, 0, 0, 0, p.zone)
	available, err := a.available(page, calendars, bookings, p, day, daptinid.NullReferenceId, tx)
	if err != nil {
		return nil, err
	}
	selected := false
	for _, slot := range available {
		if slot.Equal(start) {
			selected = true
			break
		}
	}
	if !selected {
		return nil, bookingHTTP(http.StatusConflict, "slot is no longer available")
	}
	end := start.Add(time.Duration(p.duration) * time.Minute)
	uid := uuid.NewString()
	event, organizer, err := a.eventCalendar(page, calendars.destination, name, email, start, end, uid, tx)
	if err != nil {
		return nil, err
	}
	data, err := bookingEncode(event)
	if err != nil {
		return nil, err
	}
	path, err := a.destinationPath(calendars.destination, uid+".ics", tx)
	if err != nil {
		return nil, err
	}
	b := calendars.backend
	content := b.contentValue("calendar", path, ical.MIMEType, data)
	attrs := map[string]interface{}{"rpath": path, "uid": uid, "organizer_address": organizer, "collection_id": fmt.Sprint(calendars.destination["reference_id"]), "content": content}
	if organizer != "" {
		attrs["schedule_tag"] = uuid.NewString()
	}
	if err := b.calendarCreateEvent(path, calendars.destination, attrs, tx); err != nil {
		return nil, err
	}
	eventRow, err := b.calendarObjectRow(path, calendars.destination, tx)
	if err != nil {
		return nil, err
	}
	eventRef := bookingRef(eventRow["reference_id"])
	if _, err := b.scheduleAndStoreCalendarChange(calendars.destination, eventRow, path, event, nil, data, tx); err != nil {
		return nil, err
	}
	model := api2go.NewApi2GoModelWithData("booking", nil, int64(a.cruds["booking"].TableInfo().DefaultPermission), nil, map[string]interface{}{
		"bookable_id": pageRef.String(), "calendar_id": eventRef.String(), "starts_at": start, "ends_at": end,
		"state": "confirmed", "guest_name": name, "guest_email": email,
		"attempt_hash": attemptHash, "request_hash": requestHash, "answers": answers,
	})
	created, err := a.cruds["booking"].CreateWithTransaction(model, a.request(http.MethodPost, "/action/bookable/reserve", owner), tx)
	if err != nil {
		return nil, err
	}
	createdModel, ok := created.Result().(api2go.Api2GoModel)
	if !ok {
		return nil, errors.New("booking create returned an invalid resource")
	}
	createdRef := bookingRef(createdModel.GetID())
	row, _, err := a.cruds["booking"].GetSingleRowByReferenceIdWithTransaction("booking", createdRef, nil, tx)
	if err != nil {
		return nil, err
	}
	return a.bookingResult(row, tx)
}

func (a *bookableAction) authorizedBooking(fields map[string]interface{}, lock bool, tx *sqlx.Tx) (map[string]interface{}, map[string]interface{}, *auth.SessionUser, error) {
	ref := bookingRef(fields["booking_ref"])
	if ref == daptinid.NullReferenceId {
		return nil, nil, nil, bookingHTTP(http.StatusBadRequest, "booking_ref must be a reference ID")
	}
	if lock {
		if err := a.cruds["booking"].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": ref[:]}); err != nil {
			return nil, nil, nil, err
		}
	}
	row, _, err := a.cruds["booking"].GetSingleRowByReferenceIdWithTransaction("booking", ref, nil, tx)
	if err != nil {
		return nil, nil, nil, bookingHTTP(http.StatusNotFound, "booking not found")
	}
	pageID, err := bookingLinkedID("bookable", row["bookable_id"], tx)
	if err != nil {
		return nil, nil, nil, err
	}
	pageRef, err := GetIdToReferenceIdWithTransaction("bookable", pageID, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	if lock {
		if err := a.cruds["bookable"].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": pageRef[:]}); err != nil {
			return nil, nil, nil, err
		}
	}
	page, _, err := a.cruds["bookable"].GetSingleRowByReferenceIdWithTransaction("bookable", pageRef, nil, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	owner, err := a.owner(page, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	callerIsOwner := caller != nil && caller.UserReferenceId == owner.UserReferenceId
	provided := strings.TrimSpace(StringOrEmpty(fields["token"]))
	expected := a.token(ref)
	if expected == "" {
		return nil, nil, nil, bookingHTTP(http.StatusServiceUnavailable, "booking token secret is unavailable")
	}
	decoded, decodeErr := hex.DecodeString(provided)
	want, _ := hex.DecodeString(expected)
	if !callerIsOwner && (decodeErr != nil || !hmac.Equal(decoded, want)) {
		return nil, nil, nil, bookingHTTP(http.StatusForbidden, "booking access denied")
	}
	return row, page, owner, nil
}

func (a *bookableAction) resolveBooking(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	_, _, owner, err := a.authorizedBooking(fields, true, tx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"owner_ref": owner.UserReferenceId.String()}, nil
}

func (a *bookableAction) delivery(row map[string]interface{}, tx *sqlx.Tx) (map[string]int, error) {
	eventID, err := bookingLinkedID("calendar", row["calendar_id"], tx)
	if err != nil {
		return nil, err
	}
	eventRef, err := GetIdToReferenceIdWithTransaction("calendar", eventID, tx)
	if err != nil {
		return nil, err
	}
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("cal_mail", tx, 1001, goqu.Ex{"event_reference": eventRef.String()})
	if err != nil {
		return nil, err
	}
	if len(refs) > 1000 {
		return nil, bookingHTTP(http.StatusConflict, "too many delivery records")
	}
	states := map[string]int{}
	for _, ref := range refs {
		message, _, err := a.cruds["cal_mail"].GetSingleRowByReferenceIdWithTransaction("cal_mail", ref, nil, tx)
		if err != nil {
			return nil, err
		}
		states[StringOrEmpty(message["state"])]++
	}
	return states, nil
}

func (a *bookableAction) manage(fields map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	mutating := a.name != "status"
	row, page, owner, err := a.authorizedBooking(fields, mutating, tx)
	if err != nil {
		return nil, err
	}
	if !mutating {
		result, err := a.bookingResult(row, tx)
		if err != nil {
			return nil, err
		}
		delivery, err := a.delivery(row, tx)
		if err != nil {
			return nil, err
		}
		result["delivery"] = delivery
		eventRef, err := bookingLinkedRef("calendar", row["calendar_id"], tx)
		if err != nil {
			return nil, err
		}
		eventRecord, _, err := a.cruds["calendar"].GetSingleRowByReferenceIdWithTransaction("calendar", eventRef, nil, tx)
		if err != nil {
			return nil, err
		}
		collectionRef, err := bookingLinkedRef("collection", eventRecord["collection_id"], tx)
		if err != nil {
			return nil, err
		}
		collection, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", collectionRef, nil, tx)
		if err != nil {
			return nil, err
		}
		result["mail_configured"] = bookingRef(collection["scheduling_mail_account_id"]) != daptinid.NullReferenceId
		return result, nil
	}
	active, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || active.UserReferenceId != owner.UserReferenceId {
		return nil, bookingHTTP(http.StatusForbidden, "bookable owner context is required")
	}
	if StringOrEmpty(row["state"]) != "confirmed" {
		if a.name == "cancel" {
			return a.bookingResult(row, tx)
		}
		return nil, bookingHTTP(http.StatusConflict, "booking is not confirmed")
	}
	eventID, err := bookingLinkedID("calendar", row["calendar_id"], tx)
	if err != nil {
		return nil, err
	}
	eventRef, err := GetIdToReferenceIdWithTransaction("calendar", eventID, tx)
	if err != nil {
		return nil, err
	}
	eventRecord, _, err := a.cruds["calendar"].GetSingleRowByReferenceIdWithTransaction("calendar", eventRef, nil, tx)
	if err != nil {
		return nil, err
	}
	eventPath := StringOrEmpty(eventRecord["rpath"])
	eventCollectionRef, err := bookingLinkedRef("collection", eventRecord["collection_id"], tx)
	if err != nil {
		return nil, err
	}
	eventCollection, _, err := a.cruds["collection"].GetSingleRowByReferenceIdWithTransaction("collection", eventCollectionRef, nil, tx)
	if err != nil {
		return nil, err
	}
	backend := NewCalDAVBackend(a.cruds, owner, nil)
	var calendars *bookingCalendars
	if a.name == "reschedule" {
		calendars, err = a.calendars(page, owner, tx)
		if err != nil {
			return nil, err
		}
		if bookingRef(calendars.destination["reference_id"]) != eventCollectionRef {
			return nil, bookingHTTP(http.StatusConflict, "booking destination changed; cancel this booking before moving it")
		}
	} else if err := backend.lockCalendarCollection(eventCollection, tx); err != nil {
		return nil, err
	}
	eventRow, err := backend.calendarObjectRow(eventPath, eventCollection, tx)
	if err != nil {
		return nil, err
	}
	previous, err := backend.contentBytes("calendar", eventRow)
	if err != nil {
		return nil, err
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(previous)).Decode()
	if err != nil || len(calendar.Events()) != 1 {
		return nil, bookingHTTP(http.StatusConflict, "booking event is invalid")
	}
	event := calendar.Events()[0]
	if err := advanceBookingSequence(event); err != nil {
		return nil, err
	}
	attrs := map[string]interface{}{}
	var current []byte
	if a.name == "cancel" {
		setBookingProp(event.Component, "STATUS", "CANCELLED")
		attrs["state"] = "cancelled"
	} else {
		p, err := parseBookingPolicy(page)
		if err != nil {
			return nil, err
		}
		start, err := bookingTime(StringOrEmpty(fields["start"]))
		if err != nil {
			return nil, err
		}
		oldStart, err := bookingStoredTime(row["starts_at"])
		if err != nil {
			return nil, err
		}
		if oldStart.Equal(start) {
			return a.bookingResult(row, tx)
		}
		bookings, err := a.bookingRows(page, tx)
		if err != nil {
			return nil, err
		}
		local := start.In(p.zone)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, p.zone)
		available, err := a.available(page, calendars, bookings, p, day, bookingRef(row["reference_id"]), tx)
		if err != nil {
			return nil, err
		}
		selected := false
		for _, slot := range available {
			if slot.Equal(start) {
				selected = true
				break
			}
		}
		if !selected {
			return nil, bookingHTTP(http.StatusConflict, "slot is no longer available")
		}
		end := start.Add(time.Duration(p.duration) * time.Minute)
		event.Props.SetDateTime(ical.PropDateTimeStart, start)
		event.Props.SetDateTime(ical.PropDateTimeEnd, end)
		event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
		attrs["starts_at"], attrs["ends_at"] = start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)
	}
	current, err = bookingEncode(calendar)
	if err != nil {
		return nil, err
	}
	eventChanges := map[string]interface{}{"content": backend.contentValue("calendar", eventPath, ical.MIMEType, current)}
	if bookingRef(eventCollection["scheduling_mail_account_id"]) != daptinid.NullReferenceId {
		eventChanges["schedule_tag"] = uuid.NewString()
	}
	if err := backend.calendarUpdate("calendar", eventPath, eventRow, eventChanges, tx); err != nil {
		return nil, err
	}
	persisted, err := backend.calendarObjectRow(eventPath, eventCollection, tx)
	if err != nil {
		return nil, err
	}
	persistedContent, err := backend.contentBytes("calendar", persisted)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(persistedContent, current) {
		return nil, errors.New("booking event update did not persist")
	}
	if a.name == "cancel" {
		if err := backend.scheduleCalendarChange(eventCollection, eventRef, previous, nil, nil, tx); err != nil {
			return nil, err
		}
	} else {
		storedEvent, err := backend.calendarObjectRow(eventPath, eventCollection, tx)
		if err != nil {
			return nil, err
		}
		if _, err := backend.scheduleAndStoreCalendarChange(eventCollection, storedEvent, eventPath, calendar, previous, current, tx); err != nil {
			return nil, err
		}
	}
	bookingRefID := bookingRef(row["reference_id"])
	model := api2go.NewApi2GoModelWithData("booking", nil, 0, nil, row)
	model.SetAttributes(attrs)
	if _, err := a.cruds["booking"].UpdateWithTransaction(model, a.request(http.MethodPatch, "/action/bookable/"+a.name, owner), tx); err != nil {
		return nil, err
	}
	verified, err := a.cruds["booking"].FindOneWithTransaction(bookingRefID, a.request(http.MethodGet, "/api/booking/"+bookingRefID.String(), owner), tx)
	if err != nil {
		return nil, err
	}
	verifiedModel, ok := verified.Result().(api2go.Api2GoModel)
	if !ok || (attrs["state"] != nil && verifiedModel.GetAttributes()["state"] != attrs["state"]) {
		return nil, errors.New("booking update did not persist")
	}
	if value, changed := attrs["starts_at"]; changed {
		persistedStart, err := bookingStoredTime(verifiedModel.GetAttributes()["starts_at"])
		wantedStart, parseErr := bookingTime(value.(string))
		if err != nil || parseErr != nil || !persistedStart.Equal(wantedStart) {
			return nil, errors.New("booking time update did not persist")
		}
	}
	for key, value := range attrs {
		row[key] = value
	}
	return a.bookingResult(row, tx)
}

func (b *DaptinDAVBackend) bookingEventProtected(ref daptinid.DaptinReferenceId, tx *sqlx.Tx) (bool, error) {
	eventID, err := GetReferenceIdToIdWithTransaction("calendar", ref, tx)
	if err != nil {
		return false, err
	}
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("booking", tx, 1, goqu.Ex{"calendar_id": eventID})
	return len(refs) > 0, err
}
