package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/daptin/daptin/server/assetcachepojo"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

const calendarAttachmentColumn = "attachments"
const calendarAttachmentMaxBytes = 10 << 20

type calendarMailAttachment struct {
	id          string
	filename    string
	contentType string
	data        []byte
}

// prepareITIPAttachments reads bounded attachment bytes before the calendar
// mutation opens its transaction. Invitation MIME is assembled later from
// this request-local data; no cloud operation is made while queueing mail.
func (b *DaptinDAVBackend) prepareITIPAttachments(ctx context.Context, requestPath string, calendar *ical.Calendar) error {
	b.itipAttachments = nil
	requested := make(map[string]string)
	for _, component := range itipComponents(calendar) {
		for _, attach := range component.Props.Values("ATTACH") {
			id := attach.Params.Get("MANAGED-ID")
			if id == "" {
				continue
			}
			if _, err := uuid.Parse(id); err != nil {
				return webdav.NewHTTPError(http.StatusBadRequest, errors.New("invalid managed attachment ID"))
			}
			link, err := url.Parse(attach.Value)
			if err != nil || link.Path != requestPath || link.Query().Get("attachment") != id {
				return webdav.NewHTTPError(http.StatusBadRequest, errors.New("managed attachment URI does not match the event"))
			}
			requested[id] = attach.Params.Get("FILENAME")
		}
	}
	if len(requested) == 0 {
		return nil
	}
	store, err := b.attachmentStore()
	if err != nil {
		return webdav.NewHTTPError(http.StatusConflict, err)
	}
	crud := b.cruds[calendarObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return err
	}
	_, event, _, err := b.attachmentEvent(requestPath, tx, true)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	eventRef := daptinid.InterfaceToDIR(event["reference_id"])
	files, err := crud.GetAssetColumnFiles(eventRef, calendarAttachmentColumn, tx)
	_ = tx.Rollback()
	if err != nil {
		return err
	}
	metadata := make(map[string]map[string]interface{}, len(files))
	for _, file := range files {
		name, _ := file["name"].(string)
		metadata[name] = file
	}
	b.itipAttachments = make(map[string]calendarMailAttachment, len(requested))
	var total int64
	for id, filename := range requested {
		key := eventRef.String() + "/" + id
		file := metadata[key]
		if file == nil || file["status"] != "completed" {
			return webdav.NewHTTPError(http.StatusConflict, errors.New("managed calendar attachment is unavailable"))
		}
		fileType, _ := file["type"].(string)
		if filename == "" || fileType == "" {
			return webdav.NewHTTPError(http.StatusBadRequest, errors.New("managed calendar attachment has incomplete metadata"))
		}
		reader, err := store.GetFileByNameContext(ctx, key)
		if err != nil {
			return webdav.NewHTTPError(http.StatusConflict, errors.New("managed calendar attachment bytes are unavailable"))
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, calendarAttachmentMaxBytes+1))
		_ = reader.Close()
		if readErr != nil || len(data) > calendarAttachmentMaxBytes {
			return webdav.NewHTTPError(http.StatusRequestEntityTooLarge, errors.New("managed calendar attachment exceeds the size limit"))
		}
		total += int64(len(data))
		if total > calendarAttachmentMaxBytes {
			return webdav.NewHTTPError(http.StatusRequestEntityTooLarge, errors.New("calendar invitation attachments exceed the size limit"))
		}
		b.itipAttachments[id] = calendarMailAttachment{id: id, filename: filename, contentType: fileType, data: data}
	}
	return nil
}

// ServeManagedAttachment handles the CalDAV object request, leaving ordinary
// event PUT/GET and scheduling URLs with their existing handlers.
func (b *DaptinDAVBackend) ServeManagedAttachment(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodOptions && b.serveManagedAttachmentOptions(w, r) {
		return true
	}
	action := r.URL.Query().Get("action")
	attachment := r.URL.Query().Get("attachment")
	if r.Method == http.MethodPost && action == "attachment-add" {
		b.addManagedAttachment(w, r)
		return true
	}
	if r.Method == http.MethodPost && (action == "attachment-update" || action == "attachment-remove") {
		b.changeManagedAttachment(w, r, action)
		return true
	}
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && attachment != "" {
		b.getManagedAttachment(w, r, attachment)
		return true
	}
	return false
}

func (b *DaptinDAVBackend) serveManagedAttachmentOptions(w http.ResponseWriter, r *http.Request) bool {
	if _, err := b.attachmentStore(); err != nil {
		return false
	}
	if b.sessionUser == nil {
		return false
	}
	principalPath := "/caldav/" + b.sessionUser.UserReferenceId.String() + "/"
	requestPath := path.Clean(r.URL.Path)
	allow := "OPTIONS, PROPFIND"
	switch requestPath {
	case path.Clean("/caldav/"), path.Clean(principalPath):
	case path.Clean(b.homePath()):
		allow = "OPTIONS, PROPFIND, MKCOL, MKCALENDAR"
	default:
		return false
	}
	active, err := b.autoSchedulingEnabled(r.Context(), principalPath)
	if err != nil {
		writeSchedulingError(w, err, http.StatusInternalServerError)
		return true
	}
	caps := "1, 3, calendar-access, calendar-managed-attachments, calendar-managed-attachments-no-recurrence"
	if active {
		caps += ", calendar-auto-schedule"
	}
	w.Header().Set("DAV", caps)
	w.Header().Set("Allow", allow)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (b *DaptinDAVBackend) attachmentStore() (*assetcachepojo.AssetFolderCache, error) {
	calendar := b.cruds[calendarObjectTable]
	world := b.cruds["world"]
	if calendar == nil || world == nil {
		return nil, errors.New("calendar attachment resources unavailable")
	}
	column, ok := calendar.TableInfo().GetColumnByName(calendarAttachmentColumn)
	if !ok || !column.IsForeignKey || column.ForeignKeyData.DataSource != "cloud_store" {
		return nil, errors.New("calendar attachments require an administrator-configured cloud_store asset column")
	}
	store := world.AssetFolderCache[calendarObjectTable][calendarAttachmentColumn]
	if store == nil {
		return nil, errors.New("calendar attachment cloud store unavailable")
	}
	return store, nil
}

func (b *DaptinDAVBackend) attachmentEvent(requestPath string, tx *sqlx.Tx, edit bool) (map[string]interface{}, map[string]interface{}, []byte, error) {
	owner, name, err := b.calendarPath(requestPath, true)
	if err != nil {
		return nil, nil, nil, err
	}
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	event, err := b.calendarObjectRow(requestPath, collection, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	if edit {
		crud := b.cruds[calendarObjectTable]
		user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId
		table := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
		row := GetObjectPermissionByReferenceIdWithTransaction(calendarObjectTable, daptinid.InterfaceToDIR(event["reference_id"]), tx)
		if !b.calendarEditAllowed(collection, tx, false) || !table.CanUpdate(user, groups, admin) || !row.CanUpdate(user, groups, admin) {
			return nil, nil, nil, errors.New("calendar event edit access denied")
		}
	}
	content, err := b.contentBytes(calendarObjectTable, event)
	return collection, event, content, err
}

func managedAttachmentFilename(header string) (string, error) {
	if header == "" {
		return "", errors.New("attachment filename is required")
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return "", errors.New("invalid attachment disposition")
	}
	name := path.Base(strings.ReplaceAll(params["filename"], "\\", "/"))
	if name == "" || name == "." || name == "/" {
		return "", errors.New("attachment filename is required")
	}
	if strings.ContainsAny(name, "\r\n\x00") {
		return "", errors.New("invalid attachment filename")
	}
	return name, nil
}

func managedAttachmentURL(r *http.Request, id string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path, RawQuery: url.Values{"attachment": {id}}.Encode()}).String()
}

func addManagedAttachmentProps(calendar *ical.Calendar, id, filename, contentType, uri string, size int64) error {
	components := itipComponents(calendar)
	if len(components) == 0 {
		return errors.New("calendar object has no event or task")
	}
	for _, component := range components {
		prop := ical.NewProp("ATTACH")
		prop.Value = uri
		prop.Params.Set("MANAGED-ID", id)
		prop.Params.Set("FILENAME", filename)
		prop.Params.Set("FMTTYPE", contentType)
		prop.Params.Set("SIZE", strconv.FormatInt(size, 10))
		component.Props.Add(prop)
	}
	return nil
}

func (b *DaptinDAVBackend) addManagedAttachment(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("rid") != "" || r.URL.Query().Get("managed-id") != "" {
		http.Error(w, "unsupported attachment selector", http.StatusBadRequest)
		return
	}
	store, err := b.attachmentStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	filename, err := managedAttachmentFilename(r.Header.Get("Content-Disposition"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType == "" {
		http.Error(w, "attachment Content-Type is required", http.StatusBadRequest)
		return
	}
	if r.ContentLength > calendarAttachmentMaxBytes {
		http.Error(w, "calendar attachment is too large", http.StatusRequestEntityTooLarge)
		return
	}
	crud := b.cruds[calendarObjectTable]
	preflight, err := crud.Connection().Beginx()
	if err != nil {
		http.Error(w, "calendar unavailable", http.StatusInternalServerError)
		return
	}
	_, event, previous, err := b.attachmentEvent(r.URL.Path, preflight, true)
	_ = preflight.Rollback()
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := checkDAVConditions(true, GetMD5Hash(previous), webdav.ConditionalMatch(r.Header.Get("If-Match")), webdav.ConditionalMatch(r.Header.Get("If-None-Match"))); err != nil {
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
		return
	}
	currentCalendar, err := ical.NewDecoder(bytes.NewReader(previous)).Decode()
	if err != nil {
		http.Error(w, "invalid calendar event", http.StatusInternalServerError)
		return
	}
	if err := b.prepareITIPAttachments(r.Context(), r.URL.Path, currentCalendar); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, calendarAttachmentMaxBytes))
	if err != nil {
		http.Error(w, "calendar attachment is too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}
	var existingBytes int
	for _, file := range b.itipAttachments {
		existingBytes += len(file.data)
	}
	if existingBytes+len(body) > calendarAttachmentMaxBytes {
		http.Error(w, "calendar invitation attachments exceed the size limit", http.StatusRequestEntityTooLarge)
		return
	}
	id := uuid.NewString()
	eventRef := daptinid.InterfaceToDIR(event["reference_id"])
	storedName := eventRef.String() + "/" + id
	size, err := store.PutStoredFile(r.Context(), storedName, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "calendar attachment storage failed", http.StatusBadGateway)
		return
	}
	if size != int64(len(body)) {
		b.removeStoredAttachments(r.Context(), []string{storedName})
		http.Error(w, "calendar attachment storage was incomplete", http.StatusBadGateway)
		return
	}
	committed := false
	defer func() {
		if !committed {
			b.removeStoredAttachments(r.Context(), []string{storedName})
		}
	}()
	newFile := calendarMailAttachment{id: id, filename: filename, contentType: contentType, data: body}
	if b.itipAttachments == nil {
		b.itipAttachments = make(map[string]calendarMailAttachment)
	}
	b.itipAttachments[id] = newFile
	data, tag, err := b.commitManagedAttachmentChange(r, eventRef, previous, "", &newFile)
	if err != nil {
		writeSchedulingError(w, err, http.StatusConflict)
		return
	}
	committed = true
	writeManagedAttachmentResponse(w, r, http.StatusCreated, id, tag, data)
}

func removeManagedAttachmentProps(calendar *ical.Calendar, id string) bool {
	found := false
	for _, component := range itipComponents(calendar) {
		remaining := component.Props["ATTACH"][:0]
		for _, attach := range component.Props["ATTACH"] {
			if attach.Params.Get("MANAGED-ID") == id {
				found = true
				continue
			}
			remaining = append(remaining, attach)
		}
		component.Props["ATTACH"] = remaining
	}
	return found
}

func managedAttachmentIDs(calendar *ical.Calendar) map[string]bool {
	ids := make(map[string]bool)
	for _, component := range itipComponents(calendar) {
		for _, attach := range component.Props.Values("ATTACH") {
			if id := attach.Params.Get("MANAGED-ID"); id != "" {
				ids[id] = true
			}
		}
	}
	return ids
}

func rewriteManagedAttachmentURLs(calendar *ical.Calendar, destinationPath string) error {
	for _, component := range itipComponents(calendar) {
		for i := range component.Props["ATTACH"] {
			attach := &component.Props["ATTACH"][i]
			if attach.Params.Get("MANAGED-ID") == "" {
				continue
			}
			link, err := url.Parse(attach.Value)
			if err != nil || link.Query().Get("attachment") != attach.Params.Get("MANAGED-ID") {
				return errors.New("invalid managed attachment URI")
			}
			link.Path = destinationPath
			attach.Value = link.String()
		}
	}
	return nil
}

func (b *DaptinDAVBackend) removeUnreferencedAttachments(eventRef daptinid.DaptinReferenceId, previous []byte, current *ical.Calendar, tx *sqlx.Tx) ([]string, error) {
	if len(previous) == 0 {
		return nil, nil
	}
	oldCalendar, err := ical.NewDecoder(bytes.NewReader(previous)).Decode()
	if err != nil {
		return nil, err
	}
	oldIDs := managedAttachmentIDs(oldCalendar)
	for id := range managedAttachmentIDs(current) {
		delete(oldIDs, id)
	}
	if len(oldIDs) == 0 {
		return nil, nil
	}
	crud := b.cruds[calendarObjectTable]
	var removed []string
	for id := range oldIDs {
		key := eventRef.String() + "/" + id
		files, err := crud.GetAssetColumnFiles(eventRef, calendarAttachmentColumn, tx)
		if err != nil {
			return nil, err
		}
		for i, file := range files {
			if file["name"] != key {
				continue
			}
			filePath, _ := file["path"].(string)
			if err := crud.RemoveAssetColumnFile(eventRef, calendarAttachmentColumn, i, key, filePath, tx); err != nil {
				return nil, err
			}
			removed = append(removed, key)
			break
		}
	}
	return removed, nil
}

func (b *DaptinDAVBackend) removeStoredAttachments(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	store, err := b.attachmentStore()
	if err != nil {
		return
	}
	for _, key := range keys {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		if err := store.RemoveStoredFile(cleanupCtx, key); err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).WithField("attachment", key).Warn("Failed to remove calendar attachment")
		}
		cancel()
	}
}

func (b *DaptinDAVBackend) commitManagedAttachmentChange(r *http.Request, eventRef daptinid.DaptinReferenceId, previous []byte, oldID string, newFile *calendarMailAttachment) ([]byte, string, error) {
	crud := b.cruds[calendarObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	owner, name, err := b.calendarPath(r.URL.Path, true)
	if err != nil {
		return nil, "", err
	}
	collection, err := b.calendarCollection(owner, name, r.URL.Path, tx)
	if err != nil {
		return nil, "", err
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return nil, "", err
	}
	collection, event, current, err := b.attachmentEvent(r.URL.Path, tx, true)
	if err != nil {
		return nil, "", err
	}
	if daptinid.InterfaceToDIR(event["reference_id"]) != eventRef || !bytes.Equal(current, previous) {
		return nil, "", newSchedulingHTTPError(http.StatusPreconditionFailed, errors.New("calendar event changed during attachment upload"))
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(current)).Decode()
	if err != nil {
		return nil, "", err
	}
	if oldID != "" && !removeManagedAttachmentProps(calendar, oldID) {
		return nil, "", newSchedulingHTTPError(http.StatusNotFound, errors.New("managed attachment not found"))
	}
	if newFile != nil {
		if err := addManagedAttachmentProps(calendar, newFile.id, newFile.filename, newFile.contentType,
			managedAttachmentURL(r, newFile.id), int64(len(newFile.data))); err != nil {
			return nil, "", newSchedulingHTTPError(http.StatusBadRequest, err)
		}
	}
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		return nil, "", err
	}
	data := encoded.Bytes()
	tag := ""
	if StringOrEmpty(event["organizer_address"]) != "" && daptinid.InterfaceToDIR(collection["scheduling_mail_account_id"]) != daptinid.NullReferenceId {
		tag = uuid.NewString()
	}
	if err := b.calendarUpdate(calendarObjectTable, r.URL.Path, event, map[string]interface{}{
		"content": b.contentValue(calendarObjectTable, r.URL.Path, ical.MIMEType, data), "schedule_tag": tag,
	}, tx); err != nil {
		return nil, "", err
	}
	if oldID != "" {
		files, err := crud.GetAssetColumnFiles(eventRef, calendarAttachmentColumn, tx)
		if err != nil {
			return nil, "", err
		}
		oldName := eventRef.String() + "/" + oldID
		found := false
		for i, file := range files {
			if file["name"] != oldName {
				continue
			}
			filePath, _ := file["path"].(string)
			if err := crud.RemoveAssetColumnFile(eventRef, calendarAttachmentColumn, i, oldName, filePath, tx); err != nil {
				return nil, "", err
			}
			found = true
			break
		}
		if !found {
			return nil, "", newSchedulingHTTPError(http.StatusNotFound, errors.New("managed attachment metadata not found"))
		}
	}
	if newFile != nil {
		if err := crud.UpdateAssetColumnWithFile(calendarAttachmentColumn, eventRef.String()+"/"+newFile.id,
			eventRef, int64(len(newFile.data)), newFile.contentType, tx); err != nil {
			return nil, "", err
		}
	}
	event, err = b.calendarObjectRow(r.URL.Path, collection, tx)
	if err != nil {
		return nil, "", err
	}
	data, err = b.scheduleAndStoreCalendarChange(collection, event, r.URL.Path, calendar, previous, data, tx)
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return data, tag, nil
}

func writeManagedAttachmentResponse(w http.ResponseWriter, r *http.Request, status int, id, tag string, data []byte) {
	if id != "" {
		w.Header().Set("Cal-Managed-ID", id)
	}
	w.Header().Set("ETag", strconv.Quote(GetMD5Hash(data)))
	if tag != "" {
		w.Header().Set("Schedule-Tag", strconv.Quote(tag))
	}
	if status != http.StatusNoContent && strings.Contains(r.Header.Get("Prefer"), "return=representation") {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(data)
		return
	}
	w.WriteHeader(status)
}

func (b *DaptinDAVBackend) changeManagedAttachment(w http.ResponseWriter, r *http.Request, action string) {
	oldID := r.URL.Query().Get("managed-id")
	if _, err := uuid.Parse(oldID); err != nil || r.URL.Query().Get("rid") != "" {
		http.Error(w, "valid managed-id is required", http.StatusBadRequest)
		return
	}
	store, err := b.attachmentStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	crud := b.cruds[calendarObjectTable]
	preflight, err := crud.Connection().Beginx()
	if err != nil {
		http.Error(w, "calendar unavailable", http.StatusInternalServerError)
		return
	}
	_, event, previous, err := b.attachmentEvent(r.URL.Path, preflight, true)
	_ = preflight.Rollback()
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := checkDAVConditions(true, GetMD5Hash(previous), webdav.ConditionalMatch(r.Header.Get("If-Match")), webdav.ConditionalMatch(r.Header.Get("If-None-Match"))); err != nil {
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
		return
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(previous)).Decode()
	if err != nil {
		http.Error(w, "invalid calendar event", http.StatusInternalServerError)
		return
	}
	if err := b.prepareITIPAttachments(r.Context(), r.URL.Path, calendar); err != nil {
		writeSchedulingError(w, err, http.StatusConflict)
		return
	}
	if _, ok := b.itipAttachments[oldID]; !ok {
		http.NotFound(w, r)
		return
	}
	delete(b.itipAttachments, oldID)
	eventRef := daptinid.InterfaceToDIR(event["reference_id"])
	var newFile *calendarMailAttachment
	if action == "attachment-update" {
		filename, err := managedAttachmentFilename(r.Header.Get("Content-Disposition"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType == "" {
			http.Error(w, "attachment Content-Type is required", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, calendarAttachmentMaxBytes))
		if err != nil {
			http.Error(w, "calendar attachment is too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		var existingBytes int
		for _, file := range b.itipAttachments {
			existingBytes += len(file.data)
		}
		if existingBytes+len(body) > calendarAttachmentMaxBytes {
			http.Error(w, "calendar invitation attachments exceed the size limit", http.StatusRequestEntityTooLarge)
			return
		}
		newFile = &calendarMailAttachment{id: uuid.NewString(), filename: filename, contentType: contentType, data: body}
		if _, err := store.PutStoredFile(r.Context(), eventRef.String()+"/"+newFile.id, bytes.NewReader(body)); err != nil {
			http.Error(w, "calendar attachment storage failed", http.StatusBadGateway)
			return
		}
		b.itipAttachments[newFile.id] = *newFile
	}
	committed := false
	defer func() {
		if !committed && newFile != nil {
			b.removeStoredAttachments(r.Context(), []string{eventRef.String() + "/" + newFile.id})
		}
	}()
	data, tag, err := b.commitManagedAttachmentChange(r, eventRef, previous, oldID, newFile)
	if err != nil {
		writeSchedulingError(w, err, http.StatusConflict)
		return
	}
	committed = true
	b.removeStoredAttachments(r.Context(), []string{eventRef.String() + "/" + oldID})
	if action == "attachment-remove" {
		writeManagedAttachmentResponse(w, r, http.StatusNoContent, "", tag, data)
		return
	}
	writeManagedAttachmentResponse(w, r, http.StatusOK, newFile.id, tag, data)
}

func (b *DaptinDAVBackend) getManagedAttachment(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := uuid.Parse(id); err != nil {
		http.NotFound(w, r)
		return
	}
	store, err := b.attachmentStore()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	crud := b.cruds[calendarObjectTable]
	tx, err := crud.Connection().Beginx()
	if err != nil {
		http.Error(w, "calendar unavailable", http.StatusInternalServerError)
		return
	}
	_, event, content, err := b.attachmentEvent(r.URL.Path, tx, false)
	if err != nil {
		_ = tx.Rollback()
		http.NotFound(w, r)
		return
	}
	eventRef := daptinid.InterfaceToDIR(event["reference_id"])
	storedName := eventRef.String() + "/" + id
	files, err := crud.GetAssetColumnFiles(eventRef, calendarAttachmentColumn, tx)
	_ = tx.Rollback()
	if err != nil {
		http.Error(w, "calendar attachment unavailable", http.StatusInternalServerError)
		return
	}
	var fileType, filename string
	for _, file := range files {
		if file["name"] == storedName && file["status"] == "completed" {
			fileType, _ = file["type"].(string)
			break
		}
	}
	if fileType == "" {
		http.NotFound(w, r)
		return
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
	if err != nil {
		http.Error(w, "invalid calendar event", http.StatusInternalServerError)
		return
	}
	for _, component := range itipComponents(calendar) {
		for _, attach := range component.Props.Values("ATTACH") {
			if attach.Params.Get("MANAGED-ID") == id {
				filename = attach.Params.Get("FILENAME")
			}
		}
	}
	if filename == "" {
		http.NotFound(w, r)
		return
	}
	file, err := store.GetFileByNameContext(r.Context(), storedName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", fileType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method == http.MethodGet {
		_, _ = io.Copy(w, file)
	}
}
