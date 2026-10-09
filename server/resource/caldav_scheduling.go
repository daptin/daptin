package resource

import (
	"bytes"
	"context"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav/caldav"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/jmoiron/sqlx"
)

// schedulingPrincipal describes a calendar owner's configured mail identity.
type schedulingPrincipal struct {
	Addresses  []string
	InboxPath  string
	OutboxPath string
}

type schedulingResponse struct {
	Recipient     string
	RequestStatus string
	Calendar      *ical.Calendar
}

const calendarInboxMaxMessages = 500

func (b *DaptinDAVBackend) schedulingOwner(requestPath string) (daptinid.DaptinReferenceId, error) {
	parts := strings.Split(strings.Trim(path.Clean(requestPath), "/"), "/")
	if len(parts) < 2 || parts[0] != "caldav" {
		return daptinid.NullReferenceId, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	owner := daptinid.InterfaceToDIR(parts[1])
	if owner == daptinid.NullReferenceId || b.sessionUser == nil || owner != b.sessionUser.UserReferenceId {
		return daptinid.NullReferenceId, newSchedulingHTTPError(http.StatusForbidden, errors.New("scheduling principal access denied"))
	}
	return owner, nil
}

func (b *DaptinDAVBackend) schedulingPrincipal(_ context.Context, principalPath string) (*schedulingPrincipal, error) {
	owner, err := b.schedulingOwner(principalPath)
	if err != nil {
		return nil, err
	}
	if path.Clean(principalPath) != path.Clean("/caldav/"+owner.String()+"/") {
		return nil, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	collections, err := b.calendarRows(calendarCollectionTable, principalPath,
		Query{ColumnName: "user_account_id", Operator: "=", Value: owner.String()})
	if err != nil {
		return nil, err
	}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	addresses := make(map[string]bool)
	accounts := make(map[daptinid.DaptinReferenceId]bool)
	for _, collection := range collections {
		accountRef := daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"])
		if accountRef == daptinid.NullReferenceId || accounts[accountRef] {
			continue
		}
		accounts[accountRef] = true
		account, _, err := b.cruds["mail_account"].GetSingleRowByReferenceIdWithTransaction("mail_account", accountRef, nil, tx)
		if err != nil {
			return nil, err
		}
		address := strings.ToLower(strings.TrimSpace(fmt.Sprint(account["username"])))
		parsed, err := mail.ParseAddress(address)
		if err != nil || parsed.Address != address {
			return nil, newSchedulingHTTPError(http.StatusNotFound, errors.New("scheduling account has no valid address"))
		}
		accountID, err := ResourceRowInt64(account["id"])
		if err != nil {
			return nil, err
		}
		connected, err := GetReferenceIdByWhereClauseWithTransaction(calendarCollectionTable, tx, goqu.Ex{"scheduling_mail_account_id": accountID})
		if err != nil {
			return nil, err
		}
		uniqueOwner := true
		for _, collectionRef := range connected {
			linked, _, err := b.cruds[calendarCollectionTable].GetSingleRowByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, nil, tx)
			if err != nil {
				return nil, err
			}
			if daptinid.InterfaceToDIR(linked["user_account_id"]) != owner {
				uniqueOwner = false
			}
		}
		if uniqueOwner {
			addresses["mailto:"+address] = true
		}
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	result := &schedulingPrincipal{
		InboxPath:  "/caldav/" + owner.String() + "/schedule-inbox/",
		OutboxPath: "/caldav/" + owner.String() + "/schedule-outbox/",
	}
	for address := range addresses {
		result.Addresses = append(result.Addresses, address)
	}
	sort.Strings(result.Addresses)
	return result, nil
}

func (b *DaptinDAVBackend) schedulingInboxRows(ctx context.Context, requestPath string) ([]map[string]interface{}, error) {
	owner, err := b.schedulingOwner(requestPath)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path.Clean(requestPath), "/caldav/"+owner.String()+"/schedule-inbox") {
		return nil, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	principal, err := b.schedulingPrincipal(ctx, "/caldav/"+owner.String()+"/")
	if err != nil || principal == nil {
		return nil, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	crud := b.cruds["cal_mail"]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	filters := []Query{
		{ColumnName: "direction", Operator: "=", Value: "inbound"},
		{ColumnName: "user_account_id", Operator: "=", Value: owner.String()},
		{ColumnName: "acknowledged_at", Operator: "=", Value: nil},
	}
	query, err := encodingjson.Marshal(filters)
	if err != nil {
		return nil, err
	}
	req := b.request(http.MethodGet, requestPath)
	req.QueryParams = map[string][]string{
		"query": {string(query)}, "page[size]": {fmt.Sprint(calendarInboxMaxMessages + 1)}, "page[number]": {"1"},
	}
	_, response, err := crud.PaginatedFindAllWithTransaction(req, tx)
	if err != nil {
		return nil, davResourceError(err)
	}
	models, ok := response.Result().([]api2go.Api2GoModel)
	if !ok {
		return nil, errors.New("scheduling inbox returned invalid rows")
	}
	if len(models) > calendarInboxMaxMessages {
		return nil, newSchedulingHTTPError(http.StatusInsufficientStorage, errors.New("scheduling inbox exceeds page limit"))
	}
	rows := make([]map[string]interface{}, 0, len(models))
	for _, model := range models {
		row := model.GetAttributes()
		row["reference_id"] = model.GetID()
		rows = append(rows, row)
	}
	return rows, tx.Commit()
}

func (b *DaptinDAVBackend) deleteSchedulingInboxObject(_ context.Context, requestPath string) error {
	owner, err := b.schedulingOwner(requestPath)
	if err != nil {
		return err
	}
	inbox := "/caldav/" + owner.String() + "/schedule-inbox/"
	if !strings.HasPrefix(requestPath, inbox) || path.Dir(requestPath) != path.Clean(inbox) || !strings.HasSuffix(requestPath, ".ics") {
		return newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	messageRef := daptinid.InterfaceToDIR(strings.TrimSuffix(strings.TrimPrefix(requestPath, inbox), ".ics"))
	if messageRef == daptinid.NullReferenceId {
		return newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	crud := b.cruds["cal_mail"]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	request := actionresponse.ActionRequest{Type: "cal_mail", Action: "acknowledge",
		Attributes: map[string]interface{}{"cal_mail_id": messageRef.String()}}
	if _, err := crud.HandleActionRequest(request, b.request(http.MethodPost, requestPath), tx); err != nil {
		return davResourceError(err)
	}
	return tx.Commit()
}

type itipAcknowledgeAction struct{ cruds map[string]*DbResource }

func NewITIPAcknowledgeAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &itipAcknowledgeAction{cruds: cruds}
}
func (*itipAcknowledgeAction) Name() string { return "itip.acknowledge" }

func (a *itipAcknowledgeAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	ref := daptinid.InterfaceToDIR(fields["message_id"])
	if caller == nil || tx == nil || ref == daptinid.NullReferenceId {
		return nil, nil, []error{errors.New("invalid scheduling acknowledgment")}
	}
	crud := a.cruds["cal_mail"]
	row, _, err := crud.GetSingleRowByReferenceIdWithTransaction("cal_mail", ref, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if row["direction"] != "inbound" || daptinid.InterfaceToDIR(row["user_account_id"]) != caller.UserReferenceId && !IsAdminWithTransaction(caller, tx) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("inbox message unavailable"), "inbox message unavailable", http.StatusForbidden)}
	}
	if row["acknowledged_at"] != nil {
		return nil, []actionresponse.ActionResponse{NewActionResponse("itip.acknowledge", map[string]interface{}{"message_id": ref.String()})}, nil
	}
	model := api2go.NewApi2GoModelWithData("cal_mail", nil, 0, nil, map[string]interface{}{
		"reference_id": ref.String(), "acknowledged_at": time.Now().UTC(),
	})
	b := NewCalDAVBackend(a.cruds, caller, nil)
	if _, err := crud.updateAfterAuthorizationWithTransaction(model, b.request(http.MethodPatch, "/api/cal_mail/"+ref.String()), tx); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("itip.acknowledge", map[string]interface{}{"message_id": ref.String()})}, nil
}

type itipDeliveryStatusAction struct{ cruds map[string]*DbResource }

func NewITIPDeliveryStatusAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &itipDeliveryStatusAction{cruds: cruds}
}
func (*itipDeliveryStatusAction) Name() string { return "itip.delivery_status" }

func (a *itipDeliveryStatusAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	messageRef := daptinid.InterfaceToDIR(fields["message_id"])
	if caller == nil || tx == nil || collectionRef == daptinid.NullReferenceId || messageRef == daptinid.NullReferenceId {
		return nil, nil, []error{errors.New("invalid scheduling status request")}
	}
	message, _, err := a.cruds["cal_mail"].GetSingleRowByReferenceIdWithTransaction("cal_mail", messageRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if message["direction"] != "outbound" || daptinid.InterfaceToDIR(message["collection_id"]) != collectionRef {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("scheduling message unavailable"), "scheduling message unavailable", http.StatusNotFound)}
	}
	user, groups := caller.UserReferenceId, caller.Groups
	denied := func() (api2go.Responder, []actionresponse.ActionResponse, []error) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("scheduling status denied"), "scheduling status denied", http.StatusForbidden)}
	}
	collectionCRUD := a.cruds[calendarCollectionTable]
	collectionTablePermission := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	collectionPermission := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if !collectionTablePermission.CanRead(user, groups, collectionCRUD.AdministratorGroupId) || !collectionPermission.CanRead(user, groups, collectionCRUD.AdministratorGroupId) {
		return denied()
	}
	collection, _, err := collectionCRUD.GetSingleRowByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	eventRef := daptinid.InterfaceToDIR(message["event_reference"])
	if eventRef == daptinid.NullReferenceId {
		return denied()
	}
	eventRows, err := GetSingleColumnValueByReferenceIdWithTransaction(calendarObjectTable, []interface{}{"reference_id"}, "reference_id", []string{eventRef.String()}, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if len(eventRows) == 0 {
		if user != daptinid.InterfaceToDIR(collection["user_account_id"]) && user != daptinid.InterfaceToDIR(message["user_account_id"]) && !IsAdminWithTransaction(caller, tx) {
			return denied()
		}
	} else {
		eventCRUD := a.cruds[calendarObjectTable]
		eventTablePermission := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
		eventPermission := GetObjectPermissionByReferenceIdWithTransaction(calendarObjectTable, eventRef, tx)
		if !eventTablePermission.CanRead(user, groups, eventCRUD.AdministratorGroupId) || !eventPermission.CanRead(user, groups, eventCRUD.AdministratorGroupId) {
			return denied()
		}
		event, _, err := eventCRUD.GetSingleRowByReferenceIdWithTransaction(calendarObjectTable, eventRef, nil, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		if daptinid.InterfaceToDIR(event["collection_id"]) != collectionRef {
			return denied()
		}
	}
	outboxRef := daptinid.InterfaceToDIR(message["outbox_id"])
	outbox, _, err := a.cruds["outbox"].GetSingleRowByReferenceIdWithTransaction("outbox", outboxRef, nil, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	lastError := strings.TrimSpace(fmt.Sprint(outbox["last_error"]))
	if outbox["last_error"] == nil {
		lastError = ""
	}
	if len(lastError) > 200 {
		lastError = lastError[:200]
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("itip.delivery_status", map[string]interface{}{
		"message_id": messageRef.String(), "sent": outbox["sent"], "retry_count": outbox["retry_count"],
		"next_retry_at": outbox["next_retry_at"], "last_error": lastError,
	})}, nil
}

func (b *DaptinDAVBackend) schedulingInboxObject(owner daptinid.DaptinReferenceId, row map[string]interface{}) (caldav.CalendarObject, error) {
	data := []byte(fmt.Sprint(row["icalendar"]))
	calendar, err := ical.NewDecoder(strings.NewReader(string(data))).Decode()
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		return caldav.CalendarObject{}, err
	}
	return caldav.CalendarObject{
		Path:    "/caldav/" + owner.String() + "/schedule-inbox/" + fmt.Sprint(row["reference_id"]) + ".ics",
		ModTime: davTime(row), ContentLength: int64(encoded.Len()), ETag: GetMD5Hash(encoded.Bytes()), Data: calendar,
	}, nil
}

func (b *DaptinDAVBackend) listSchedulingInbox(ctx context.Context, requestPath string) ([]caldav.CalendarObject, error) {
	owner, err := b.schedulingOwner(requestPath)
	if err != nil {
		return nil, err
	}
	rows, err := b.schedulingInboxRows(ctx, requestPath)
	if err != nil {
		return nil, err
	}
	objects := make([]caldav.CalendarObject, 0, len(rows))
	for _, row := range rows {
		object, err := b.schedulingInboxObject(owner, row)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func (b *DaptinDAVBackend) getSchedulingInboxObject(ctx context.Context, requestPath string) (*caldav.CalendarObject, error) {
	objects, err := b.listSchedulingInbox(ctx, path.Dir(requestPath)+"/")
	if err != nil {
		return nil, err
	}
	for _, object := range objects {
		if path.Clean(object.Path) == path.Clean(requestPath) {
			return &object, nil
		}
	}
	return nil, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
}

func (b *DaptinDAVBackend) querySchedulingInbox(ctx context.Context, requestPath string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	objects, err := b.listSchedulingInbox(ctx, requestPath)
	if err != nil {
		return nil, err
	}
	if query == nil {
		return objects, nil
	}
	result := make([]caldav.CalendarObject, 0, len(objects))
	for _, object := range objects {
		filter := query.CompFilter
		for _, component := range object.Data.Children {
			if component.Name == ical.CompEvent && component.Props.Get(ical.PropDateTimeStart) == nil {
				// iTIP replies may omit DTSTART. RFC 6638 requires those
				// inbox messages to match time-range calendar queries.
				filter = inboxFilterWithoutEventRange(filter)
				break
			}
		}
		matches, err := caldav.Match(filter, &object)
		if err != nil {
			return nil, err
		}
		if matches {
			result = append(result, object)
		}
	}
	return result, nil
}

func inboxFilterWithoutEventRange(filter caldav.CompFilter) caldav.CompFilter {
	if filter.Name == ical.CompEvent {
		filter.Start = time.Time{}
		filter.End = time.Time{}
	}
	if len(filter.Comps) != 0 {
		children := make([]caldav.CompFilter, len(filter.Comps))
		for i, child := range filter.Comps {
			children[i] = inboxFilterWithoutEventRange(child)
		}
		filter.Comps = children
	}
	return filter
}

func (b *DaptinDAVBackend) schedulingFreeBusy(ctx context.Context, outboxPath string, request *ical.Calendar) ([]schedulingResponse, error) {
	owner, err := b.schedulingOwner(outboxPath)
	if err != nil {
		return nil, err
	}
	principal, err := b.schedulingPrincipal(ctx, "/caldav/"+owner.String()+"/")
	if err != nil || principal == nil || path.Clean(outboxPath) != path.Clean(principal.OutboxPath) {
		return nil, newSchedulingHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	if request == nil || request.Props.Get("METHOD") == nil || !strings.EqualFold(request.Props.Get("METHOD").Value, "REQUEST") {
		return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("free/busy request requires METHOD:REQUEST"))
	}
	var component *ical.Component
	for _, child := range request.Children {
		if child.Name != ical.CompFreeBusy || component != nil {
			return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("free/busy request requires one VFREEBUSY"))
		}
		component = child
	}
	if component == nil {
		return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("free/busy request has no VFREEBUSY"))
	}
	requestUID := itipProp(component.Props.Get(ical.PropUID))
	if requestUID == "" {
		return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("free/busy request has no UID"))
	}
	organizer := "mailto:" + itipAddress(component.Props.Get("ORGANIZER"))
	validOrganizer := false
	for _, address := range principal.Addresses {
		if address == organizer {
			validOrganizer = true
			break
		}
	}
	if !validOrganizer {
		return nil, newSchedulingHTTPError(http.StatusForbidden, errors.New("free/busy organizer does not match outbox"))
	}
	start, startErr := component.Props.DateTime(ical.PropDateTimeStart, time.UTC)
	end, endErr := component.Props.DateTime(ical.PropDateTimeEnd, time.UTC)
	if startErr != nil || endErr != nil || !start.Before(end) || end.Sub(start) > 366*24*time.Hour {
		return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("invalid free/busy range"))
	}
	attendees := component.Props.Values("ATTENDEE")
	if len(attendees) == 0 || len(attendees) > 100 {
		return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("free/busy request requires 1 to 100 attendees"))
	}
	responses := make([]schedulingResponse, 0, len(attendees))
	for _, attendee := range attendees {
		address := itipAddress(&attendee)
		if address == "" {
			return nil, newSchedulingHTTPError(http.StatusBadRequest, errors.New("invalid free/busy attendee"))
		}
		busy, err := b.schedulingRecipientBusy(ctx, address, organizer, requestUID, start, end)
		if err != nil {
			responses = append(responses, schedulingResponse{Recipient: "mailto:" + address, RequestStatus: "3.7;Invalid Calendar User"})
			continue
		}
		responses = append(responses, schedulingResponse{Recipient: "mailto:" + address, RequestStatus: "2.0;Success", Calendar: busy})
	}
	return responses, nil
}

func (b *DaptinDAVBackend) schedulingRecipientBusy(ctx context.Context, address, organizer, requestUID string, start, end time.Time) (*ical.Calendar, error) {
	tx, err := b.cruds["mail_account"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	accounts, _, err := b.cruds["mail_account"].GetRowsByWhereClauseWithTransaction("mail_account", nil, tx, goqu.Ex{"username": address})
	if err != nil || len(accounts) != 1 {
		return nil, errors.New("free/busy recipient unavailable")
	}
	accountID, err := ResourceRowInt64(accounts[0]["id"])
	if err != nil {
		return nil, err
	}
	collectionRefs, err := GetReferenceIdByWhereClauseWithTransaction(calendarCollectionTable, tx, goqu.Ex{"scheduling_mail_account_id": accountID})
	if err != nil || len(collectionRefs) == 0 {
		return nil, errors.New("free/busy recipient unavailable")
	}
	var owner daptinid.DaptinReferenceId
	paths := make([]string, 0, len(collectionRefs))
	for _, ref := range collectionRefs {
		collection, _, err := b.cruds[calendarCollectionTable].GetSingleRowByReferenceIdWithTransaction(calendarCollectionTable, ref, nil, tx)
		if err != nil {
			return nil, err
		}
		candidate := daptinid.InterfaceToDIR(collection["user_account_id"])
		if candidate == daptinid.NullReferenceId || owner != daptinid.NullReferenceId && candidate != owner {
			return nil, errors.New("free/busy recipient is ambiguous")
		}
		owner = candidate
		paths = append(paths, "/caldav/"+owner.String()+"/calendars/"+fmt.Sprint(collection["name"])+"/")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := ical.NewCalendar()
	result.Props.SetText(ical.PropVersion, "2.0")
	result.Props.SetText(ical.PropProductID, "-//Daptin//CalDAV Scheduling//EN")
	result.Props.SetText("METHOD", "REPLY")
	busy := ical.NewComponent(ical.CompFreeBusy)
	busy.Props.SetText(ical.PropUID, requestUID)
	busy.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	busy.Props.SetDateTime(ical.PropDateTimeStart, start.UTC())
	busy.Props.SetDateTime(ical.PropDateTimeEnd, end.UTC())
	busy.Props.SetText("ORGANIZER", organizer)
	busy.Props.SetText("ATTENDEE", "mailto:"+address)
	result.Children = append(result.Children, busy)
	for _, calendarPath := range paths {
		calendar, err := b.FreeBusy(ctx, calendarPath, start, end, true)
		if err != nil {
			return nil, err
		}
		for _, child := range calendar.Children {
			if child.Name == ical.CompFreeBusy {
				for _, prop := range child.Props.Values(ical.PropFreeBusy) {
					busy.Props.Add(&prop)
				}
			}
		}
	}
	return result, nil
}
