package resource

import (
	"bytes"
	"context"
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
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

const (
	calendarCollectionTable = "collection"
	calendarObjectTable     = "calendar"
	addressBookTable        = "address_book"
	addressObjectTable      = "contact"
)

var errDAVNotFound = errors.New("DAV resource not found")

var (
	_ caldav.Backend  = (*DaptinDAVBackend)(nil)
	_ carddav.Backend = (*DaptinDAVBackend)(nil)
)

// DaptinDAVBackend adapts the protocol backends to canonical Daptin resources.
// The endpoint authenticates first; every lookup is then scoped to that owner.
type DaptinDAVBackend struct {
	cruds       map[string]*DbResource
	sessionUser *auth.SessionUser
	prefix      string
	home        string
}

func NewCalDAVBackend(cruds map[string]*DbResource, user *auth.SessionUser) *DaptinDAVBackend {
	return &DaptinDAVBackend{cruds: cruds, sessionUser: user, prefix: "/caldav", home: "calendars"}
}

func NewCardDAVBackend(cruds map[string]*DbResource, user *auth.SessionUser) *DaptinDAVBackend {
	return &DaptinDAVBackend{cruds: cruds, sessionUser: user, prefix: "/carddav", home: "addressbooks"}
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

func (b *DaptinDAVBackend) CalendarHomeSetPath(context.Context) (string, error) {
	return b.homePath(), nil
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
	if err := tx.Commit(); err != nil {
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

func (b *DaptinDAVBackend) objectRows(table, relationColumn string, collection map[string]interface{}) ([]map[string]interface{}, error) {
	collectionID, ok := collection["id"]
	if !ok {
		return nil, errors.New("DAV collection has no internal identity")
	}
	return b.rows(table, goqu.Ex{relationColumn: collectionID, "user_account_id": b.sessionUser.UserId})
}

func (b *DaptinDAVBackend) objectRow(table, requestPath string) (map[string]interface{}, error) {
	rows, err := b.rows(table, goqu.Ex{"rpath": path.Clean(requestPath), "user_account_id": b.sessionUser.UserId})
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
	name, err := b.collectionName(calendar.Path, false)
	if err != nil {
		return err
	}
	if _, err := b.ownedCollection(calendarCollectionTable, name); err == nil {
		return webdav.NewHTTPError(http.StatusMethodNotAllowed, errors.New("calendar already exists"))
	} else if !errors.Is(err, errDAVNotFound) {
		return err
	}
	_, err = b.create(calendarCollectionTable, calendar.Path, map[string]interface{}{"name": name, "description": calendar.Description})
	return err
}

func (b *DaptinDAVBackend) ListCalendars(context.Context) ([]caldav.Calendar, error) {
	rows, err := b.rows(calendarCollectionTable, goqu.Ex{"user_account_id": b.sessionUser.UserId})
	if err != nil {
		return nil, err
	}
	result := make([]caldav.Calendar, 0, len(rows))
	for _, row := range rows {
		name := fmt.Sprint(row["name"])
		result = append(result, caldav.Calendar{Path: b.homePath() + url.PathEscape(name) + "/", Name: name, Description: fmt.Sprint(row["description"]), SupportedComponentSet: []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}})
	}
	return result, nil
}

func (b *DaptinDAVBackend) GetCalendar(_ context.Context, requestPath string) (*caldav.Calendar, error) {
	name, err := b.collectionName(requestPath, false)
	if err != nil {
		return nil, err
	}
	row, err := b.ownedCollection(calendarCollectionTable, name)
	if err != nil {
		return nil, err
	}
	return &caldav.Calendar{Path: b.homePath() + url.PathEscape(name) + "/", Name: name, Description: fmt.Sprint(row["description"]), SupportedComponentSet: []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}}, nil
}

func (b *DaptinDAVBackend) calendarObject(row map[string]interface{}) (caldav.CalendarObject, error) {
	data, err := b.contentBytes(calendarObjectTable, row)
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return caldav.CalendarObject{}, err
	}
	return caldav.CalendarObject{Path: fmt.Sprint(row["rpath"]), ModTime: davTime(row), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Data: calendar}, nil
}

func (b *DaptinDAVBackend) GetCalendarObject(_ context.Context, requestPath string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	if _, err := b.collectionName(requestPath, true); err != nil {
		return nil, err
	}
	row, err := b.objectRow(calendarObjectTable, requestPath)
	if err != nil {
		return nil, err
	}
	object, err := b.calendarObject(row)
	return &object, err
}

func (b *DaptinDAVBackend) ListCalendarObjects(_ context.Context, requestPath string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	name, err := b.collectionName(requestPath, false)
	if err != nil {
		return nil, err
	}
	collection, err := b.ownedCollection(calendarCollectionTable, name)
	if err != nil {
		return nil, err
	}
	rows, err := b.objectRows(calendarObjectTable, "collection_id", collection)
	if err != nil {
		return nil, err
	}
	result := make([]caldav.CalendarObject, 0, len(rows))
	for _, row := range rows {
		object, err := b.calendarObject(row)
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
	name, err := b.collectionName(requestPath, true)
	if err != nil {
		return nil, err
	}
	collection, err := b.ownedCollection(calendarCollectionTable, name)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		return nil, err
	}
	data := encoded.Bytes()
	existing, findErr := b.objectRow(calendarObjectTable, requestPath)
	exists := findErr == nil
	if findErr != nil && !errors.Is(findErr, errDAVNotFound) {
		return nil, findErr
	}
	etag := ""
	if exists {
		current, err := b.contentBytes(calendarObjectTable, existing)
		if err != nil {
			return nil, err
		}
		etag = GetMD5Hash(current)
	}
	if err := checkDAVConditions(exists, etag, opts.IfMatch, opts.IfNoneMatch); err != nil {
		return nil, err
	}
	value := b.contentValue(calendarObjectTable, requestPath, ical.MIMEType, data)
	if exists {
		err = b.update(calendarObjectTable, requestPath, existing, map[string]interface{}{"content": value})
	} else {
		_, err = b.create(calendarObjectTable, requestPath, map[string]interface{}{"rpath": path.Clean(requestPath), "content": value, "collection_id": daptinid.InterfaceToDIR(collection["reference_id"]).String()})
	}
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{Path: path.Clean(requestPath), ModTime: time.Now(), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Data: calendar}, nil
}

func (b *DaptinDAVBackend) DeleteCalendarObject(_ context.Context, requestPath string) error {
	if name, err := b.collectionName(requestPath, false); err == nil {
		collection, err := b.ownedCollection(calendarCollectionTable, name)
		if err != nil {
			return err
		}
		objects, err := b.objectRows(calendarObjectTable, "collection_id", collection)
		if err != nil {
			return err
		}
		for _, object := range objects {
			if err := b.delete(calendarObjectTable, fmt.Sprint(object["rpath"]), object); err != nil {
				return err
			}
		}
		return b.delete(calendarCollectionTable, requestPath, collection)
	}
	if _, err := b.collectionName(requestPath, true); err != nil {
		return err
	}
	row, err := b.objectRow(calendarObjectTable, requestPath)
	if err != nil {
		return err
	}
	return b.delete(calendarObjectTable, requestPath, row)
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
	_, err = b.create(addressBookTable, addressBook.Path, map[string]interface{}{"name": name, "description": addressBook.Description})
	return err
}

func (b *DaptinDAVBackend) ListAddressBooks(context.Context) ([]carddav.AddressBook, error) {
	rows, err := b.rows(addressBookTable, goqu.Ex{"user_account_id": b.sessionUser.UserId})
	if err != nil {
		return nil, err
	}
	result := make([]carddav.AddressBook, 0, len(rows))
	for _, row := range rows {
		name := fmt.Sprint(row["name"])
		result = append(result, carddav.AddressBook{Path: b.homePath() + url.PathEscape(name) + "/", Name: name, Description: fmt.Sprint(row["description"])})
	}
	return result, nil
}

func (b *DaptinDAVBackend) GetAddressBook(_ context.Context, requestPath string) (*carddav.AddressBook, error) {
	name, err := b.collectionName(requestPath, false)
	if err != nil {
		return nil, err
	}
	row, err := b.ownedCollection(addressBookTable, name)
	if err != nil {
		return nil, err
	}
	return &carddav.AddressBook{Path: b.homePath() + url.PathEscape(name) + "/", Name: name, Description: fmt.Sprint(row["description"])}, nil
}

func (b *DaptinDAVBackend) DeleteAddressBook(_ context.Context, requestPath string) error {
	name, err := b.collectionName(requestPath, false)
	if err != nil {
		return err
	}
	collection, err := b.ownedCollection(addressBookTable, name)
	if err != nil {
		return err
	}
	objects, err := b.objectRows(addressObjectTable, "address_book_id", collection)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := b.delete(addressObjectTable, fmt.Sprint(object["rpath"]), object); err != nil {
			return err
		}
	}
	return b.delete(addressBookTable, requestPath, collection)
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

func (b *DaptinDAVBackend) GetAddressObject(_ context.Context, requestPath string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	if _, err := b.collectionName(requestPath, true); err != nil {
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
	name, err := b.collectionName(requestPath, false)
	if err != nil {
		return nil, err
	}
	collection, err := b.ownedCollection(addressBookTable, name)
	if err != nil {
		return nil, err
	}
	rows, err := b.objectRows(addressObjectTable, "address_book_id", collection)
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
	name, err := b.collectionName(requestPath, true)
	if err != nil {
		return nil, err
	}
	collection, err := b.ownedCollection(addressBookTable, name)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := vcard.NewEncoder(&encoded).Encode(card); err != nil {
		return nil, err
	}
	data := encoded.Bytes()
	existing, findErr := b.objectRow(addressObjectTable, requestPath)
	exists := findErr == nil
	if findErr != nil && !errors.Is(findErr, errDAVNotFound) {
		return nil, findErr
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
		err = b.update(addressObjectTable, requestPath, existing, map[string]interface{}{"content": value})
	} else {
		_, err = b.create(addressObjectTable, requestPath, map[string]interface{}{"rpath": path.Clean(requestPath), "content": value, "address_book_id": daptinid.InterfaceToDIR(collection["reference_id"]).String()})
	}
	if err != nil {
		return nil, err
	}
	return &carddav.AddressObject{Path: path.Clean(requestPath), ModTime: time.Now(), ContentLength: int64(len(data)), ETag: GetMD5Hash(data), Card: card}, nil
}

func (b *DaptinDAVBackend) DeleteAddressObject(_ context.Context, requestPath string) error {
	if _, err := b.collectionName(requestPath, true); err != nil {
		return err
	}
	row, err := b.objectRow(addressObjectTable, requestPath)
	if err != nil {
		return err
	}
	return b.delete(addressObjectTable, requestPath, row)
}
