package resource

import (
	"bytes"
	"context"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
	"github.com/daptin/go-webdav/carddav"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	calendarCollectionTable = "collection"
	calendarObjectTable     = "calendar"
	addressBookTable        = "address_book"
	addressObjectTable      = "contact"
)

var errDAVNotFound = errors.New("DAV resource not found")

var (
	_ caldav.Backend                       = (*DaptinDAVBackend)(nil)
	_ caldav.CalendarPropertyBackend       = (*DaptinDAVBackend)(nil)
	_ caldav.CalendarObjectTransferBackend = (*DaptinDAVBackend)(nil)
	_ carddav.Backend                      = (*DaptinDAVBackend)(nil)
	_ carddav.AddressBookPropertyBackend   = (*DaptinDAVBackend)(nil)
)

// DaptinDAVBackend adapts the protocol backends to canonical Daptin resources.
// The endpoint authenticates first; CalDAV paths identify the calendar owner.
type DaptinDAVBackend struct {
	cruds       map[string]*DbResource
	sessionUser *auth.SessionUser
	prefix      string
	home        string
	headers     http.Header
}

func NewCalDAVBackend(cruds map[string]*DbResource, user *auth.SessionUser, headers http.Header) *DaptinDAVBackend {
	return &DaptinDAVBackend{cruds: cruds, sessionUser: user, prefix: "/caldav", home: "calendars", headers: headers}
}

func NewCardDAVBackend(cruds map[string]*DbResource, user *auth.SessionUser, headers http.Header) *DaptinDAVBackend {
	return &DaptinDAVBackend{cruds: cruds, sessionUser: user, prefix: "/carddav", home: "addressbooks", headers: headers}
}

func (b *DaptinDAVBackend) CurrentUserPrincipal(context.Context) (string, error) {
	if b.sessionUser == nil || b.sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, errors.New("DAV authentication required"))
	}
	return b.prefix + "/" + b.sessionUser.UserReferenceId.String() + "/", nil
}

func (b *DaptinDAVBackend) homePath() string {
	return b.prefix + "/" + b.sessionUser.UserReferenceId.String() + "/" + b.home + "/"
}

func (b *DaptinDAVBackend) calendarHomeOwner(requestPath string) (daptinid.DaptinReferenceId, error) {
	parts := strings.Split(strings.Trim(path.Clean(requestPath), "/"), "/")
	if (len(parts) != 2 && len(parts) != 3) || parts[0] != "caldav" || (len(parts) == 3 && parts[2] != "calendars") {
		return daptinid.NullReferenceId, webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid calendar home path"))
	}
	owner := daptinid.InterfaceToDIR(parts[1])
	if owner == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid calendar principal"))
	}
	return owner, nil
}

func (b *DaptinDAVBackend) CalendarHomeSetPath(ctx context.Context, requestedPath string) (string, error) {
	if b.sessionUser == nil || b.sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, errors.New("DAV authentication required"))
	}
	owner, err := b.calendarHomeOwner(requestedPath)
	if err != nil {
		return "", err
	}
	if owner != b.sessionUser.UserReferenceId {
		visible, err := b.ListCalendars(ctx, requestedPath)
		if err != nil {
			return "", err
		}
		if len(visible) == 0 {
			return "", webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar home access denied"))
		}
	}
	return b.prefix + "/" + owner.String() + "/calendars/", nil
}
func (b *DaptinDAVBackend) AddressBookHomeSetPath(context.Context) (string, error) {
	return b.homePath(), nil
}

func (b *DaptinDAVBackend) collectionName(requestPath string, object bool) (string, error) {
	clean := path.Clean(requestPath)
	home := strings.TrimSuffix(b.homePath(), "/")
	if clean == home || !strings.HasPrefix(clean, home+"/") {
		return "", webdav.NewHTTPError(http.StatusForbidden, errors.New("DAV path is outside the authenticated principal"))
	}
	parts := strings.Split(strings.TrimPrefix(clean, home+"/"), "/")
	want := 1
	if object {
		want = 2
	}
	if len(parts) != want || parts[0] == "" || (object && parts[1] == "") {
		return "", webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid DAV resource path"))
	}
	return parts[0], nil
}

func (b *DaptinDAVBackend) addressBookPath(requestPath string, object bool) (daptinid.DaptinReferenceId, string, error) {
	if b.sessionUser == nil || b.sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusUnauthorized, errors.New("DAV authentication required"))
	}
	parts := strings.Split(strings.Trim(path.Clean(requestPath), "/"), "/")
	want := 4
	if object {
		want = 5
	}
	if len(parts) != want || parts[0] != "carddav" || parts[2] != "addressbooks" || parts[3] == "" || (object && parts[4] == "") {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid CardDAV resource path"))
	}
	owner := daptinid.InterfaceToDIR(parts[1])
	if owner == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid CardDAV principal"))
	}
	return owner, parts[3], nil
}

func (b *DaptinDAVBackend) calendarPath(requestPath string, object bool) (daptinid.DaptinReferenceId, string, error) {
	if b.sessionUser == nil || b.sessionUser.UserReferenceId == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusUnauthorized, errors.New("DAV authentication required"))
	}
	parts := strings.Split(strings.Trim(path.Clean(requestPath), "/"), "/")
	want := 4
	if object {
		want = 5
	}
	if len(parts) != want || parts[0] != "caldav" || parts[2] != "calendars" || parts[3] == "" || (object && parts[4] == "") {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid CalDAV resource path"))
	}
	owner := daptinid.InterfaceToDIR(parts[1])
	if owner == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, "", webdav.NewHTTPError(http.StatusNotFound, errors.New("invalid CalDAV principal"))
	}
	return owner, parts[3], nil
}

// calendarRows enters the same list path as JSON:API, including its SQL read
// filter, table checks, row checks, conversion, and metering.
func (b *DaptinDAVBackend) calendarRows(table, requestPath string, filters ...Query) ([]map[string]interface{}, error) {
	crud := b.cruds[table]
	if crud == nil {
		return nil, fmt.Errorf("DAV resource %s is not configured", table)
	}
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := b.calendarRowsWithTransaction(table, requestPath, tx, filters...)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows, nil
}

func (b *DaptinDAVBackend) calendarRowsWithTransaction(table, requestPath string, tx *sqlx.Tx, filters ...Query) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0)
	err := b.eachDAVRowWithTransaction(table, requestPath, tx, func(row map[string]interface{}) error {
		rows = append(rows, row)
		return nil
	}, filters...)
	return rows, err
}

func (b *DaptinDAVBackend) eachDAVRowWithTransaction(table, requestPath string, tx *sqlx.Tx, visit func(map[string]interface{}) error, filters ...Query) error {
	crud := b.cruds[table]
	if crud == nil {
		return fmt.Errorf("DAV resource %s is not configured", table)
	}
	query, err := encodingjson.Marshal(filters)
	if err != nil {
		return err
	}
	for page := 1; ; page++ {
		req := b.request(http.MethodGet, requestPath)
		req.QueryParams = map[string][]string{
			"query":        {string(query)},
			"page[size]":   {"1000"},
			"page[number]": {fmt.Sprint(page)},
		}
		if table == calendarObjectTable || table == addressObjectTable {
			req.QueryParams["included_relations"] = []string{"content"}
			req.QueryParams["sort"] = []string{"rpath"}
		}
		_, response, err := crud.PaginatedFindAllWithTransaction(req, tx)
		if err != nil {
			return davResourceError(err)
		}
		models, ok := response.Result().([]api2go.Api2GoModel)
		if !ok {
			return errors.New("DAV resource list returned an invalid result")
		}
		for _, model := range models {
			row := model.GetAttributes()
			row["reference_id"] = model.GetID()
			if err := visit(row); err != nil {
				return err
			}
		}
		if len(models) < 1000 {
			return nil
		}
	}
}

func (b *DaptinDAVBackend) calendarCollection(owner daptinid.DaptinReferenceId, name, requestPath string, tx *sqlx.Tx) (map[string]interface{}, error) {
	filters := []Query{{ColumnName: "user_account_id", Operator: "=", Value: owner.String()}, {ColumnName: "name", Operator: "=", Value: name}}
	var rows []map[string]interface{}
	var err error
	if tx == nil {
		rows, err = b.calendarRows(calendarCollectionTable, requestPath, filters...)
	} else {
		rows, err = b.calendarRowsWithTransaction(calendarCollectionTable, requestPath, tx, filters...)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if owner != b.sessionUser.UserReferenceId {
			return nil, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar access denied"))
		}
		return nil, webdav.NewHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	return rows[0], nil
}

func (b *DaptinDAVBackend) calendarObjectRow(requestPath string, collection map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	filters := []Query{{ColumnName: "rpath", Operator: "=", Value: path.Clean(requestPath)},
		{ColumnName: "collection_id", Operator: "=", Value: fmt.Sprint(collection["reference_id"])}}
	var rows []map[string]interface{}
	var err error
	if tx == nil {
		rows, err = b.calendarRows(calendarObjectTable, requestPath, filters...)
	} else {
		rows, err = b.calendarRowsWithTransaction(calendarObjectTable, requestPath, tx, filters...)
	}
	return firstDAVObjectRow(rows, err)
}

func (b *DaptinDAVBackend) calendarEditAllowed(collection map[string]interface{}, tx *sqlx.Tx, create bool) bool {
	if b.sessionUser == nil {
		return false
	}
	crud := b.cruds[calendarCollectionTable]
	permission := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable,
		daptinid.InterfaceToDIR(collection["reference_id"]), tx)
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId
	if create {
		return permission.CanCreate(user, groups, admin) && permission.CanRefer(user, groups, admin)
	}
	return permission.CanUpdate(user, groups, admin)
}

func (b *DaptinDAVBackend) lockCalendarCollection(collection map[string]interface{}, tx *sqlx.Tx) error {
	referenceID := daptinid.InterfaceToDIR(collection["reference_id"])
	return b.cruds[calendarCollectionTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": referenceID[:]})
}

func (b *DaptinDAVBackend) calendarCreate(table, requestPath string, attrs map[string]interface{}, tx *sqlx.Tx) error {
	crud := b.cruds[table]
	_, err := crud.CreateWithTransaction(
		api2go.NewApi2GoModelWithData(table, nil, int64(crud.TableInfo().DefaultPermission), nil, attrs),
		b.request(http.MethodPost, requestPath), tx)
	return davResourceError(err)
}

// A DAV event inherits the collection's persisted group links while the
// collection row is locked. The active account still owns the event row.
func (b *DaptinDAVBackend) calendarCreateEvent(requestPath string, collection map[string]interface{}, attrs map[string]interface{}, tx *sqlx.Tx) error {
	crud := b.cruds[calendarObjectTable]
	if !IsAdminWithTransaction(b.sessionUser, tx) {
		tablePermission := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
		if !tablePermission.CanCreate(b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId) {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar table create access denied"))
		}
	}
	response, err := crud.createAfterAuthorizationWithTransaction(
		api2go.NewApi2GoModelWithData(calendarObjectTable, nil, int64(crud.TableInfo().DefaultPermission), nil, attrs),
		b.request(http.MethodPost, requestPath), tx)
	if err != nil {
		return davResourceError(err)
	}
	model, ok := response.Result().(api2go.Api2GoModel)
	if !ok {
		return errors.New("calendar create returned an invalid resource")
	}
	eventRef := daptinid.InterfaceToDIR(model.GetID())
	if eventRef == daptinid.NullReferenceId {
		return errors.New("calendar create returned no reference ID")
	}
	grants, err := b.calendarCollectionGroupGrants(collection, tx)
	if err != nil {
		return err
	}
	share := calendarShareAction{cruds: b.cruds}
	for groupRef, grant := range grants {
		if err := share.setLink(calendarObjectTable, eventRef, groupRef, grant, requestPath, b.sessionUser, tx); err != nil {
			return err
		}
	}
	return b.appendCalendarChange(daptinid.InterfaceToDIR(collection["reference_id"]), path.Clean(requestPath), nil, tx)
}

func (b *DaptinDAVBackend) calendarUpdate(table, requestPath string, row, attrs map[string]interface{}, tx *sqlx.Tx) error {
	oldPath := StringOrEmpty(row["rpath"])
	oldCollection := daptinid.InterfaceToDIR(row["collection_id"])
	newPath, newCollection := oldPath, oldCollection
	if table == calendarObjectTable {
		if value, ok := attrs["rpath"]; ok {
			newPath = StringOrEmpty(value)
		}
		if value, ok := attrs["collection_id"]; ok {
			newCollection = daptinid.InterfaceToDIR(value)
		}
	}
	var oldPermission permission.PermissionInstance
	if table == calendarObjectTable && (oldPath != newPath || oldCollection != newCollection) {
		oldPermission = GetObjectPermissionByReferenceIdWithTransaction(table, daptinid.InterfaceToDIR(row["reference_id"]), tx)
	}
	updated := make(map[string]interface{}, len(row)+len(attrs))
	for key, value := range row {
		updated[key] = value
	}
	for key, value := range attrs {
		updated[key] = value
	}
	model := api2go.NewApi2GoModelWithData(table, nil, 0, nil, updated)
	_, err := b.cruds[table].UpdateWithTransaction(model, b.request(http.MethodPatch, requestPath), tx)
	if err != nil {
		return davResourceError(err)
	}
	if table != calendarObjectTable {
		return nil
	}
	if oldPath != newPath || oldCollection != newCollection {
		if err := b.appendCalendarChange(oldCollection, oldPath, &oldPermission, tx); err != nil {
			return err
		}
	}
	return b.appendCalendarChange(newCollection, newPath, nil, tx)
}

func (b *DaptinDAVBackend) calendarDelete(table, requestPath string, row map[string]interface{}, tx *sqlx.Tx) error {
	var oldPermission permission.PermissionInstance
	if table == calendarObjectTable {
		oldPermission = GetObjectPermissionByReferenceIdWithTransaction(table, daptinid.InterfaceToDIR(row["reference_id"]), tx)
	}
	_, err := b.cruds[table].DeleteWithTransaction(daptinid.InterfaceToDIR(row["reference_id"]), b.request(http.MethodDelete, requestPath), tx)
	if err != nil {
		return davResourceError(err)
	}
	if table != calendarObjectTable {
		return nil
	}
	return b.appendCalendarChange(daptinid.InterfaceToDIR(row["collection_id"]), StringOrEmpty(row["rpath"]), &oldPermission, tx)
}

func davResourceError(err error) error {
	var resourceError api2go.HTTPError
	if errors.As(err, &resourceError) {
		return webdav.NewHTTPError(resourceError.Status(), err)
	}
	return err
}

func (b *DaptinDAVBackend) request(method, requestPath string) api2go.Request {
	r := (&http.Request{Method: method, URL: &url.URL{Path: requestPath}}).
		WithContext(context.WithValue(context.Background(), "user", b.sessionUser))
	return api2go.Request{PlainRequest: r}
}

func (b *DaptinDAVBackend) rows(table string, where ...goqu.Ex) ([]map[string]interface{}, error) {
	crud := b.cruds[table]
	if crud == nil {
		return nil, fmt.Errorf("DAV resource %s is not configured", table)
	}
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := b.rowsWithTransaction(table, tx, where...)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows, nil
}

func (b *DaptinDAVBackend) rowsWithTransaction(table string, tx *sqlx.Tx, where ...goqu.Ex) ([]map[string]interface{}, error) {
	crud := b.cruds[table]
	if crud == nil {
		return nil, fmt.Errorf("DAV resource %s is not configured", table)
	}
	referenceIDs, err := GetReferenceIdByWhereClauseWithTransaction(table, tx, where...)
	if err != nil {
		return nil, err
	}
	var includedRelations map[string]bool
	if table == calendarObjectTable || table == addressObjectTable {
		includedRelations = map[string]bool{"content": true}
	}
	rows, err := crud.readByReferenceIDsAfterAuthorizationWithTransaction(
		referenceIDs, includedRelations, b.request(http.MethodGet, b.prefix+"/"), tx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (b *DaptinDAVBackend) ownedCollection(table, name string) (map[string]interface{}, error) {
	rows, err := b.rows(table, goqu.Ex{"name": name, "user_account_id": b.sessionUser.UserId})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	return rows[0], nil
}

func (b *DaptinDAVBackend) create(table, requestPath string, attrs map[string]interface{}) (map[string]interface{}, error) {
	crud := b.cruds[table]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	resp, err := crud.createAfterAuthorizationWithTransaction(
		api2go.NewApi2GoModelWithData(table, nil, int64(crud.TableInfo().DefaultPermission), nil, attrs),
		b.request(http.MethodPost, requestPath), tx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	model, ok := resp.Result().(api2go.Api2GoModel)
	if !ok {
		return nil, errors.New("DAV resource create returned an invalid result")
	}
	return model.GetAttributes(), nil
}

func (b *DaptinDAVBackend) update(table, requestPath string, row, attrs map[string]interface{}) error {
	crud := b.cruds[table]
	model := api2go.NewApi2GoModelWithData(table, nil, 0, nil, row)
	updated := make(map[string]interface{}, len(row)+len(attrs))
	for key, value := range row {
		updated[key] = value
	}
	for key, value := range attrs {
		updated[key] = value
	}
	model.SetAttributes(updated)
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = crud.updateAfterAuthorizationWithTransaction(model, b.request(http.MethodPatch, requestPath), tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) delete(table, requestPath string, row map[string]interface{}) error {
	crud := b.cruds[table]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = crud.deleteAfterAuthorizationWithTransaction(daptinid.InterfaceToDIR(row["reference_id"]), b.request(http.MethodDelete, requestPath), tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func davTime(row map[string]interface{}) time.Time {
	for _, key := range []string{"updated_at", "created_at"} {
		switch value := row[key].(type) {
		case time.Time:
			return value
		case string:
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
				if parsed, err := time.Parse(layout, value); err == nil {
					return parsed
				}
			}
		}
	}
	return time.Time{}
}

func checkDAVConditions(exists bool, etag string, ifMatch, ifNoneMatch webdav.ConditionalMatch) error {
	if ifMatch.IsSet() {
		matched, err := ifMatch.MatchETag(etag)
		if err != nil {
			return webdav.NewHTTPError(http.StatusBadRequest, err)
		}
		if !exists || !matched {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("If-Match precondition failed"))
		}
	}
	if ifNoneMatch.IsSet() && exists {
		matched, err := ifNoneMatch.MatchETag(etag)
		if err != nil {
			return webdav.NewHTTPError(http.StatusBadRequest, err)
		}
		if matched {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("If-None-Match precondition failed"))
		}
	}
	return nil
}

func (b *DaptinDAVBackend) putFailure(table, requestPath string, ifMatch, ifNoneMatch webdav.ConditionalMatch, writeErr error) error {
	if !ifMatch.IsSet() && !ifNoneMatch.IsSet() {
		return writeErr
	}
	if !errors.Is(writeErr, ErrVersionConflict) {
		var constraint api2go.HTTPError
		if !errors.As(writeErr, &constraint) || constraint.Status() != http.StatusConflict {
			return writeErr
		}
	}
	row, err := b.objectRow(table, requestPath)
	if errors.Is(err, errDAVNotFound) {
		if ifMatch.IsSet() {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("If-Match precondition failed"))
		}
		return writeErr
	}
	if err != nil {
		return writeErr
	}
	current, err := b.contentBytes(table, row)
	if err != nil {
		return writeErr
	}
	if conditionErr := checkDAVConditions(true, GetMD5Hash(current), ifMatch, ifNoneMatch); conditionErr != nil {
		return conditionErr
	}
	return writeErr
}

func (b *DaptinDAVBackend) deleteObject(table, requestPath string) error {
	crud := b.cruds[table]
	if crud == nil {
		return fmt.Errorf("DAV resource %s is not configured", table)
	}
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ifMatch := webdav.ConditionalMatch(b.headers.Get("If-Match"))
	ifNoneMatch := webdav.ConditionalMatch(b.headers.Get("If-None-Match"))
	conditional := ifMatch.IsSet() || ifNoneMatch.IsSet()
	if conditional {
		if err := crud.lockRowByWhereWithTransaction(tx, goqu.Ex{
			"rpath": path.Clean(requestPath), "user_account_id": b.sessionUser.UserId,
		}); err != nil {
			return err
		}
	}
	row, err := b.objectRowWithTransaction(table, requestPath, tx)
	if err != nil {
		if errors.Is(err, errDAVNotFound) {
			if conditionErr := checkDAVConditions(false, "", ifMatch, ifNoneMatch); conditionErr != nil {
				return conditionErr
			}
		}
		return err
	}
	if conditional {
		current, err := b.contentBytes(table, row)
		if err != nil {
			return err
		}
		if err := checkDAVConditions(true, GetMD5Hash(current), ifMatch, ifNoneMatch); err != nil {
			return err
		}
	}
	_, err = crud.deleteAfterAuthorizationWithTransaction(daptinid.InterfaceToDIR(row["reference_id"]), b.request(http.MethodDelete, requestPath), tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) objectRow(table, requestPath string) (map[string]interface{}, error) {
	filter := goqu.Ex{"rpath": path.Clean(requestPath)}
	if table != addressObjectTable {
		filter["user_account_id"] = b.sessionUser.UserId
	}
	rows, err := b.rows(table, filter)
	return firstDAVObjectRow(rows, err)
}

func (b *DaptinDAVBackend) objectRowWithTransaction(table, requestPath string, tx *sqlx.Tx) (map[string]interface{}, error) {
	filter := goqu.Ex{"rpath": path.Clean(requestPath)}
	if table != addressObjectTable {
		filter["user_account_id"] = b.sessionUser.UserId
	}
	rows, err := b.rowsWithTransaction(table, tx, filter)
	return firstDAVObjectRow(rows, err)
}

func firstDAVObjectRow(rows []map[string]interface{}, err error) (map[string]interface{}, error) {
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errDAVNotFound)
	}
	return rows[0], nil
}

func (b *DaptinDAVBackend) contentBytes(table string, row map[string]interface{}) ([]byte, error) {
	return b.cruds[table].binaryColumnBytes(table, "content", row["content"])
}

func (b *DaptinDAVBackend) contentValue(table, name, contentType string, data []byte) interface{} {
	return b.cruds[table].binaryColumnValueForStorage(table, "content", data, name, contentType)
}

func (b *DaptinDAVBackend) CreateCalendar(_ context.Context, calendar *caldav.Calendar) error {
	owner, name, err := b.calendarPath(calendar.Path, false)
	if err != nil {
		return err
	}
	if owner != b.sessionUser.UserReferenceId {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("cannot create another principal's calendar"))
	}
	crud := b.cruds[calendarCollectionTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := b.calendarCollection(owner, name, calendar.Path, tx); err == nil {
		return webdav.NewHTTPError(http.StatusMethodNotAllowed, errors.New("calendar already exists"))
	} else if !errors.Is(err, errDAVNotFound) {
		return err
	}
	if err := b.calendarCreate(calendarCollectionTable, calendar.Path, map[string]interface{}{"name": name, "display_name": calendar.DisplayName, "description": calendar.Description}, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) ListCalendars(_ context.Context, requestedPath string) ([]caldav.Calendar, error) {
	owner, err := b.calendarHomeOwner(requestedPath)
	if err != nil {
		return nil, err
	}
	home := b.prefix + "/" + owner.String() + "/calendars/"
	rows, err := b.calendarRows(calendarCollectionTable, home, Query{ColumnName: "user_account_id", Operator: "=", Value: owner.String()})
	if err != nil {
		return nil, err
	}
	result := make([]caldav.Calendar, 0, len(rows))
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collectionTable, eventTable := b.calendarTablePermissions(tx)
	for _, row := range rows {
		name := fmt.Sprint(row["name"])
		calendar := caldav.Calendar{Path: home + name + "/", Name: name, DisplayName: StringOrEmpty(row["display_name"]), Description: fmt.Sprint(row["description"]), SupportedComponentSet: []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}}
		b.calendarPrivileges(&calendar, row, collectionTable, eventTable, tx)
		result = append(result, calendar)
	}
	return result, nil
}

func (b *DaptinDAVBackend) GetCalendar(_ context.Context, requestPath string) (*caldav.Calendar, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return nil, err
	}
	row, err := b.calendarCollection(owner, name, requestPath, nil)
	if err != nil {
		return nil, err
	}
	calendar := &caldav.Calendar{Path: b.prefix + "/" + owner.String() + "/calendars/" + name + "/", Name: name, DisplayName: StringOrEmpty(row["display_name"]), Description: fmt.Sprint(row["description"]), SupportedComponentSet: []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collectionTable, eventTable := b.calendarTablePermissions(tx)
	b.calendarPrivileges(calendar, row, collectionTable, eventTable, tx)
	return calendar, nil
}

func (b *DaptinDAVBackend) calendarTablePermissions(tx *sqlx.Tx) (permission.PermissionInstance, permission.PermissionInstance) {
	collectionTable := b.cruds[calendarCollectionTable].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	eventTable := b.cruds[calendarObjectTable].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	return collectionTable, eventTable
}

func (b *DaptinDAVBackend) calendarPrivileges(calendar *caldav.Calendar, row map[string]interface{}, collectionTable, eventTable permission.PermissionInstance, tx *sqlx.Tx) {
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	collection := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, ref, tx)
	admin := b.cruds[calendarCollectionTable].AdministratorGroupId
	user, groups := b.sessionUser.UserReferenceId, b.sessionUser.Groups
	calendar.Read = collectionTable.CanRead(user, groups, admin) && collection.CanRead(user, groups, admin)
	calendar.ReadFreeBusy = (collectionTable.CanPeek(user, groups, admin) || collectionTable.CanRead(user, groups, admin)) &&
		(collection.CanPeek(user, groups, admin) || collection.CanRead(user, groups, admin)) &&
		(eventTable.CanPeek(user, groups, admin) || eventTable.CanRead(user, groups, admin))
	calendar.Bind = calendar.Read && collection.CanCreate(user, groups, admin) && collection.CanRefer(user, groups, admin) &&
		eventTable.CanCreate(user, groups, admin)
	calendar.WriteContent = calendar.Read && collection.CanUpdate(user, groups, admin) &&
		eventTable.CanUpdate(user, groups, admin)
	calendar.WriteProperties = collectionTable.CanUpdate(user, groups, admin) && collection.CanUpdate(user, groups, admin)
	// Deleting an event also checks that event row's Delete grant. Only an
	// administrator can be certain of that grant for every child row here.
	for _, group := range groups {
		if group.GroupReferenceId == admin {
			calendar.Unbind = calendar.Read && collection.CanUpdate(user, groups, admin) &&
				eventTable.CanDelete(user, groups, admin)
			break
		}
	}
}

func (b *DaptinDAVBackend) calendarObject(row, collection map[string]interface{}) (caldav.CalendarObject, error) {
	data, err := b.contentBytes(calendarObjectTable, row)
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	object := caldav.CalendarObject{Path: fmt.Sprint(row["rpath"]), ModTime: davTime(row), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Data: calendar}
	if daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"]) != daptinid.NullReferenceId && StringOrEmpty(row["organizer_address"]) != "" {
		object.ScheduleTag = calendarScheduleTag(row, data)
	}
	return object, nil
}

func (b *DaptinDAVBackend) GetCalendarObject(_ context.Context, requestPath string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	owner, name, err := b.calendarPath(requestPath, true)
	if err != nil {
		return nil, err
	}
	collection, err := b.calendarCollection(owner, name, requestPath, nil)
	if err != nil {
		return nil, err
	}
	row, err := b.calendarObjectRow(requestPath, collection, nil)
	if err != nil {
		return nil, err
	}
	object, err := b.calendarObject(row, collection)
	return &object, err
}

func (b *DaptinDAVBackend) ListCalendarObjects(_ context.Context, requestPath string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return nil, err
	}
	collection, err := b.calendarCollection(owner, name, requestPath, nil)
	if err != nil {
		return nil, err
	}
	rows, err := b.calendarRows(calendarObjectTable, requestPath,
		Query{ColumnName: "collection_id", Operator: "=", Value: fmt.Sprint(collection["reference_id"])})
	if err != nil {
		return nil, err
	}
	result := make([]caldav.CalendarObject, 0, len(rows))
	for _, row := range rows {
		object, err := b.calendarObject(row, collection)
		if err != nil {
			return nil, err
		}
		result = append(result, object)
	}
	return result, nil
}

func (b *DaptinDAVBackend) QueryCalendarObjects(ctx context.Context, requestPath string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	objects, err := b.ListCalendarObjects(ctx, requestPath, &query.CompRequest)
	if err != nil {
		return nil, err
	}
	return caldav.Filter(query, objects)
}

func (b *DaptinDAVBackend) PutCalendarObject(_ context.Context, requestPath string, calendar *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	owner, name, err := b.calendarPath(requestPath, true)
	if err != nil {
		return nil, err
	}
	crud := b.cruds[calendarObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return nil, err
	}
	if !b.calendarEditAllowed(collection, tx, true) && !b.calendarEditAllowed(collection, tx, false) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar edit access denied"))
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}
	data := encoded.Bytes()
	uid, organizer := itipCalendarIdentity(calendar)
	existing, findErr := b.calendarObjectRow(requestPath, collection, tx)
	exists := findErr == nil
	if findErr != nil && !errors.Is(findErr, errDAVNotFound) {
		return nil, findErr
	}
	if !b.calendarEditAllowed(collection, tx, !exists) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar edit access denied"))
	}
	etag := ""
	var previous []byte
	if exists {
		current, err := b.contentBytes(calendarObjectTable, existing)
		if err != nil {
			return nil, err
		}
		previous = current
		etag = GetMD5Hash(current)
	}
	if err := checkScheduleTagCondition(b.headers.Get("If-Schedule-Tag-Match"), exists, existing, previous); err != nil {
		return nil, err
	}
	if err := checkDAVConditions(exists, etag, opts.IfMatch, opts.IfNoneMatch); err != nil {
		return nil, err
	}
	collectionAccount := daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"])
	if exists && collectionAccount != daptinid.NullReferenceId {
		account, _, err := b.cruds["mail_account"].GetSingleRowByReferenceIdWithTransaction("mail_account", collectionAccount, nil, tx)
		if err != nil {
			return nil, err
		}
		sender := strings.ToLower(strings.TrimSpace(StringOrEmpty(account["username"])))
		if b.headers.Get("If-Schedule-Tag-Match") != "" {
			if err := mergeScheduleAttendees(previous, calendar, sender); err != nil {
				return nil, err
			}
		}
		if err := resetRescheduledAttendees(previous, calendar, sender); err != nil {
			return nil, err
		}
	}
	if collectionAccount != daptinid.NullReferenceId {
		if err := preserveServerScheduleStatus(previous, calendar); err != nil {
			return nil, err
		}
		encoded.Reset()
		if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
			return nil, err
		}
		data = encoded.Bytes()
	}
	value := b.contentValue(calendarObjectTable, requestPath, ical.MIMEType, data)
	tag := ""
	if uid != "" && organizer != "" && daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"]) != daptinid.NullReferenceId {
		tag = uuid.NewString()
	}
	if exists {
		err = b.calendarUpdate(calendarObjectTable, requestPath, existing, map[string]interface{}{"content": value, "uid": uid, "organizer_address": organizer, "schedule_tag": tag}, tx)
	} else {
		err = b.calendarCreateEvent(requestPath, collection, map[string]interface{}{"rpath": path.Clean(requestPath), "content": value, "uid": uid, "organizer_address": organizer, "schedule_tag": tag, "collection_id": fmt.Sprint(collection["reference_id"])}, tx)
	}
	if err != nil {
		return nil, err
	}
	event, err := b.calendarObjectRow(requestPath, collection, tx)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]map[string]string)
	if err := b.scheduleCalendarChange(collection, daptinid.InterfaceToDIR(event["reference_id"]), previous, data, statuses, tx); err != nil {
		return nil, err
	}
	if setOutgoingScheduleStatus(calendar, statuses) {
		encoded.Reset()
		if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
			return nil, err
		}
		data = encoded.Bytes()
		delete(event, "version")
		if err := b.calendarUpdate(calendarObjectTable, requestPath, event, map[string]interface{}{
			"content": b.contentValue(calendarObjectTable, requestPath, ical.MIMEType, data),
		}, tx); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{Path: path.Clean(requestPath), ModTime: time.Now(), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), ScheduleTag: tag, Data: calendar}, nil
}

func (b *DaptinDAVBackend) DeleteCalendarObject(_ context.Context, requestPath string) error {
	crud := b.cruds[calendarObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if owner, name, err := b.calendarPath(requestPath, false); err == nil {
		collection, err := b.calendarCollection(owner, name, requestPath, tx)
		if err != nil {
			return err
		}
		if err := b.deleteCalendarCollection(requestPath, collection, tx); err != nil {
			return err
		}
		return tx.Commit()
	}
	owner, name, err := b.calendarPath(requestPath, true)
	if err != nil {
		return err
	}
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return err
	}
	if !b.calendarEditAllowed(collection, tx, false) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar edit access denied"))
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return err
	}
	row, err := b.calendarObjectRow(requestPath, collection, tx)
	if err != nil {
		if errors.Is(err, errDAVNotFound) {
			if conditionErr := checkDAVConditions(false, "", webdav.ConditionalMatch(b.headers.Get("If-Match")), webdav.ConditionalMatch(b.headers.Get("If-None-Match"))); conditionErr != nil {
				return conditionErr
			}
		}
		return err
	}
	ifMatch := webdav.ConditionalMatch(b.headers.Get("If-Match"))
	ifNoneMatch := webdav.ConditionalMatch(b.headers.Get("If-None-Match"))
	previous, err := b.contentBytes(calendarObjectTable, row)
	if err != nil {
		return err
	}
	if err := checkScheduleTagCondition(b.headers.Get("If-Schedule-Tag-Match"), true, row, previous); err != nil {
		return err
	}
	if ifMatch.IsSet() || ifNoneMatch.IsSet() {
		if err := checkDAVConditions(true, GetMD5Hash(previous), ifMatch, ifNoneMatch); err != nil {
			return err
		}
	}
	if err := b.calendarDelete(calendarObjectTable, requestPath, row, tx); err != nil {
		return err
	}
	if err := b.scheduleCalendarChange(collection, daptinid.InterfaceToDIR(row["reference_id"]), previous, nil, nil, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) CreateAddressBook(_ context.Context, addressBook *carddav.AddressBook) error {
	name, err := b.collectionName(addressBook.Path, false)
	if err != nil {
		return err
	}
	if _, err := b.ownedCollection(addressBookTable, name); err == nil {
		return webdav.NewHTTPError(http.StatusMethodNotAllowed, errors.New("address book already exists"))
	} else if !errors.Is(err, errDAVNotFound) {
		return err
	}
	_, err = b.create(addressBookTable, addressBook.Path, map[string]interface{}{"name": name, "display_name": addressBook.DisplayName, "description": addressBook.Description})
	return err
}

func (b *DaptinDAVBackend) ListAddressBooks(context.Context) ([]carddav.AddressBook, error) {
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := b.calendarRowsWithTransaction(addressBookTable, b.homePath(), tx,
		Query{ColumnName: "user_account_id", Operator: "=", Value: b.sessionUser.UserReferenceId.String()})
	if err != nil {
		return nil, err
	}
	result := make([]carddav.AddressBook, 0, len(rows))
	for _, row := range rows {
		name := fmt.Sprint(row["name"])
		result = append(result, b.addressBookInfo(row, b.homePath()+name+"/", tx))
	}
	return result, tx.Commit()
}

func (b *DaptinDAVBackend) GetAddressBook(_ context.Context, requestPath string) (*carddav.AddressBook, error) {
	if _, _, err := b.addressBookPath(requestPath, false); err != nil {
		return nil, err
	}
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, err := b.addressBookWithTransaction(requestPath, false, tx)
	if err != nil {
		return nil, err
	}
	book := b.addressBookInfo(row, requestPath, tx)
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (b *DaptinDAVBackend) addressBookInfo(row map[string]interface{}, requestPath string, tx *sqlx.Tx) carddav.AddressBook {
	bookCRUD := b.cruds[addressBookTable]
	contactCRUD := b.cruds[addressObjectTable]
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, bookCRUD.AdministratorGroupId
	bookTable := bookCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", addressBookTable, tx)
	contactTable := contactCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", addressObjectTable, tx)
	bookRow := GetObjectPermissionByReferenceIdWithTransaction(addressBookTable, daptinid.InterfaceToDIR(row["reference_id"]), tx)
	canUpdate := bookRow.CanUpdate(user, groups, admin)
	return carddav.AddressBook{
		Path: requestPath, Name: StringOrEmpty(row["name"]), DisplayName: StringOrEmpty(row["display_name"]), Description: StringOrEmpty(row["description"]),
		Read:            bookTable.CanRead(user, groups, admin) && bookRow.CanRead(user, groups, admin),
		WriteProperties: bookTable.CanUpdate(user, groups, admin) && canUpdate,
		WriteContent:    canUpdate && contactTable.CanUpdate(user, groups, admin),
		Bind:            bookRow.CanCreate(user, groups, admin) && bookRow.CanRefer(user, groups, admin) && contactTable.CanCreate(user, groups, admin),
		Unbind:          IsAdminWithTransaction(b.sessionUser, tx) && canUpdate && contactTable.CanDelete(user, groups, admin),
	}
}

func (b *DaptinDAVBackend) DeleteAddressBook(_ context.Context, requestPath string) error {
	if _, _, err := b.addressBookPath(requestPath, false); err != nil {
		return err
	}
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	collection, err := b.addressBookWithTransaction(requestPath, false, tx)
	if err != nil {
		return err
	}
	if err := b.lockAddressBook(collection, tx); err != nil {
		return err
	}
	bookRef := daptinid.InterfaceToDIR(collection["reference_id"])
	bookID, err := GetReferenceIdToIdWithTransaction(addressBookTable, bookRef, tx)
	if err != nil {
		return err
	}
	objects, err := b.rowsWithTransaction(addressObjectTable, tx,
		goqu.Ex{"address_book_id": bookID})
	if err != nil {
		return err
	}
	for _, object := range objects {
		if _, err := b.cruds[addressObjectTable].deleteAfterAuthorizationWithTransaction(
			daptinid.InterfaceToDIR(object["reference_id"]), b.request(http.MethodDelete, fmt.Sprint(object["rpath"])), tx); err != nil {
			return err
		}
	}
	if err := b.deleteDAVSyncState(bookRef, tx); err != nil {
		return err
	}
	if err := b.deleteDAVLogState(bookRef, tx); err != nil {
		return err
	}
	if err := b.deleteDAVClock(bookRef, tx); err != nil {
		return err
	}
	if _, err := b.cruds[addressBookTable].deleteAfterAuthorizationWithTransaction(bookRef,
		b.request(http.MethodDelete, requestPath), tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) addressObject(row map[string]interface{}) (carddav.AddressObject, error) {
	data, err := b.contentBytes(addressObjectTable, row)
	if err != nil {
		return carddav.AddressObject{}, err
	}
	card, err := vcard.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return carddav.AddressObject{}, err
	}
	return carddav.AddressObject{Path: fmt.Sprint(row["rpath"]), ModTime: davTime(row), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Card: card}, nil
}

func (b *DaptinDAVBackend) GetAddressObject(ctx context.Context, requestPath string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	if _, _, err := b.addressBookPath(requestPath, true); err != nil {
		return nil, err
	}
	if _, err := b.GetAddressBook(ctx, path.Dir(requestPath)); err != nil {
		return nil, err
	}
	row, err := b.objectRow(addressObjectTable, requestPath)
	if err != nil {
		return nil, err
	}
	object, err := b.addressObject(row)
	return &object, err
}

func (b *DaptinDAVBackend) ListAddressObjects(_ context.Context, requestPath string, _ *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	owner, name, err := b.addressBookPath(requestPath, false)
	if err != nil {
		return nil, err
	}
	books, err := b.calendarRows(addressBookTable, requestPath,
		Query{ColumnName: "name", Operator: "=", Value: name},
		Query{ColumnName: "user_account_id", Operator: "=", Value: owner.String()})
	if err != nil {
		return nil, err
	}
	book, err := firstDAVObjectRow(books, nil)
	if err != nil {
		return nil, err
	}
	rows, err := b.calendarRows(addressObjectTable, requestPath,
		Query{ColumnName: "address_book_id", Operator: "=", Value: daptinid.InterfaceToDIR(book["reference_id"]).String()})
	if err != nil {
		return nil, err
	}
	result := make([]carddav.AddressObject, 0, len(rows))
	for _, row := range rows {
		object, err := b.addressObject(row)
		if err != nil {
			return nil, err
		}
		result = append(result, object)
	}
	return result, nil
}

func (b *DaptinDAVBackend) QueryAddressObjects(ctx context.Context, requestPath string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	objects, err := b.ListAddressObjects(ctx, requestPath, &query.DataRequest)
	if err != nil {
		return nil, err
	}
	return carddav.Filter(query, objects)
}

func (b *DaptinDAVBackend) PutAddressObject(_ context.Context, requestPath string, card vcard.Card, opts *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	if _, _, err := b.addressBookPath(requestPath, true); err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := vcard.NewEncoder(&encoded).Encode(card); err != nil {
		return nil, err
	}
	data := encoded.Bytes()
	crud := b.cruds[addressObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collection, err := b.addressBookWithTransaction(requestPath, true, tx)
	if err != nil {
		return nil, err
	}
	if err := b.lockAddressBook(collection, tx); err != nil {
		return nil, err
	}
	existing, findErr := b.objectRowWithTransaction(addressObjectTable, requestPath, tx)
	exists := findErr == nil
	if findErr != nil && !errors.Is(findErr, errDAVNotFound) {
		return nil, findErr
	}
	if !b.addressBookEditAllowed(collection, tx, !exists) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, errors.New("address book write denied"))
	}
	etag := ""
	if exists {
		current, err := b.contentBytes(addressObjectTable, existing)
		if err != nil {
			return nil, err
		}
		etag = GetMD5Hash(current)
	}
	if err := checkDAVConditions(exists, etag, opts.IfMatch, opts.IfNoneMatch); err != nil {
		return nil, err
	}
	value := b.contentValue(addressObjectTable, requestPath, vcard.MIMEType, data)
	if exists {
		model := api2go.NewApi2GoModelWithData(addressObjectTable, nil, 0, nil, existing)
		updated := make(map[string]interface{}, len(existing)+1)
		for key, old := range existing {
			updated[key] = old
		}
		updated["content"] = value
		model.SetAttributes(updated)
		_, err = crud.updateAfterAuthorizationWithTransaction(model, b.request(http.MethodPatch, requestPath), tx)
	} else {
		model := api2go.NewApi2GoModelWithData(addressObjectTable, nil, int64(crud.TableInfo().DefaultPermission), nil,
			map[string]interface{}{"rpath": path.Clean(requestPath), "content": value,
				"address_book_id": daptinid.InterfaceToDIR(collection["reference_id"]).String()})
		var response api2go.Responder
		response, err = crud.createAfterAuthorizationWithTransaction(model, b.request(http.MethodPost, requestPath), tx)
		if err == nil {
			created, ok := response.Result().(api2go.Api2GoModel)
			if !ok {
				return nil, errors.New("contact create returned an invalid resource")
			}
			contactRef := daptinid.InterfaceToDIR(created.GetID())
			grants, grantErr := b.addressBookGroupGrants(collection, tx)
			if grantErr != nil {
				return nil, grantErr
			}
			share := (&addressBookShareAction{cruds: b.cruds}).base()
			for groupRef, grant := range grants {
				if err := share.setLink(addressObjectTable, contactRef, groupRef, grant, requestPath, b.sessionUser, tx); err != nil {
					return nil, err
				}
			}
		}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, b.putFailure(addressObjectTable, requestPath, opts.IfMatch, opts.IfNoneMatch, err)
	}
	if err := b.appendDAVChange(addressBookTable, daptinid.InterfaceToDIR(collection["reference_id"]), requestPath, nil, tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &carddav.AddressObject{Path: path.Clean(requestPath), ModTime: time.Now(), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Card: card}, nil
}

func (b *DaptinDAVBackend) DeleteAddressObject(_ context.Context, requestPath string) error {
	if _, _, err := b.addressBookPath(requestPath, true); err != nil {
		return err
	}
	crud := b.cruds[addressObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	book, err := b.addressBookWithTransaction(requestPath, true, tx)
	if err != nil {
		return err
	}
	if err := b.lockAddressBook(book, tx); err != nil {
		return err
	}
	if !b.addressBookEditAllowed(book, tx, false) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("address book write denied"))
	}
	ifMatch := webdav.ConditionalMatch(b.headers.Get("If-Match"))
	ifNoneMatch := webdav.ConditionalMatch(b.headers.Get("If-None-Match"))
	row, err := b.objectRowWithTransaction(addressObjectTable, requestPath, tx)
	if err != nil {
		if errors.Is(err, errDAVNotFound) {
			if conditionErr := checkDAVConditions(false, "", ifMatch, ifNoneMatch); conditionErr != nil {
				return conditionErr
			}
		}
		return err
	}
	if ifMatch.IsSet() || ifNoneMatch.IsSet() {
		current, err := b.contentBytes(addressObjectTable, row)
		if err != nil {
			return err
		}
		if err := checkDAVConditions(true, GetMD5Hash(current), ifMatch, ifNoneMatch); err != nil {
			return err
		}
	}
	grant := GetObjectPermissionByReferenceIdWithTransaction(addressObjectTable, daptinid.InterfaceToDIR(row["reference_id"]), tx)
	if _, err := crud.deleteAfterAuthorizationWithTransaction(daptinid.InterfaceToDIR(row["reference_id"]), b.request(http.MethodDelete, requestPath), tx); err != nil {
		return err
	}
	if err := b.appendDAVChange(addressBookTable, daptinid.InterfaceToDIR(book["reference_id"]), requestPath, &grant, tx); err != nil {
		return err
	}
	return tx.Commit()
}
