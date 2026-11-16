package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func TestMailGroupReadsRealE2E(t *testing.T) {
	requireRealE2E(t)
	options := mailProtocolDatabase(t)
	f := startDaptinE2E(t, options)
	driver := options.databaseType
	if driver == "" {
		driver = "sqlite3"
	}
	db, err := sqlx.Connect(driver, options.connectionString)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	admin := f.SignupAdmin(t)
	member := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "mail-groups-member")
	user := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "mail-groups-member@test.local")
	other := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "mail-groups-other")
	patch := func(table, ref string, permission int64) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/"+table+"/"+ref, admin,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"permission": permission}), http.StatusOK)
	}
	seed := func(table string, data map[string]interface{}) string {
		t.Helper()
		ref := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, table, data)
		patch(table, ref, 0)
		return ref
	}
	server := seed("mail_server", map[string]interface{}{"hostname": "group-mail.test", "is_enabled": false, "listen_interface": "127.0.0.1:0", "max_size": 10000, "max_clients": 1, "xclient_on": false, "always_on_tls": false, "authentication_required": true})
	const mailbox = "shared@group-mail.test"
	account := seed("mail_account", map[string]interface{}{"username": mailbox, "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": server})
	box := seed("mail_box", map[string]interface{}{"name": "INBOX", "subscribed": true, "uidvalidity": 1, "nextuid": 3, "attributes": "", "flags": "", "permanent_flags": "\\Seen", "mail_account_id": account})
	addMail := func(sequence int) string {
		name := fmt.Sprintf("group-%d", sequence)
		id := "<" + name + "@group-mail.test>"
		raw := "From: sender@example.test\r\nTo: " + mailbox + "\r\nSubject: " + name + "\r\nMessage-ID: " + id + "\r\n\r\nShared fixture body\r\n"
		return seed("mail", map[string]interface{}{"mail_box_id": box, "mail": base64.StdEncoding.EncodeToString([]byte(raw)), "message_id": id, "mail_id": name,
			"from_address": "sender@example.test", "internal_date": time.Date(2026, 10, sequence, 12, 0, 0, 0, time.UTC).Format(time.RFC3339), "to_address": mailbox, "reply_to_address": "sender@example.test", "sender_address": "sender@example.test",
			"subject": name, "body": "Shared fixture body", "spam_score": 0, "hash": name, "content_type": "text/plain", "recipient": mailbox, "has_attachment": false,
			"ip_addr": "127.0.0.1", "return_path": "sender@example.test", "is_tls": false, "seen": sequence == 1, "recent": sequence == 2, "deleted": false, "uid": sequence, "spam": false, "size": len(raw), "flags": ""})
	}
	first, second := addMail(1), addMail(2)
	groupA := accessGroupsE2ECreateUsergroup(t, f.Client, f.URL, admin, "group_mail_a")
	groupB := accessGroupsE2ECreateUsergroup(t, f.Client, f.URL, admin, "group_mail_b")
	const membership = "user_account_user_account_id_has_usergroup_usergroup_id"
	// Internal join resources are not exposed by the Office package. Seed
	// their grants in the disposable database and verify every read over HTTP.
	link := func(table, ref, group string, permission int64) uuid.UUID {
		t.Helper()
		dest := uuid.New()
		src, gid := uuid.MustParse(ref), uuid.MustParse(group)
		joinTable := table + "_" + table + "_id_has_usergroup_usergroup_id"
		query := fmt.Sprintf("INSERT INTO %s (%s_id, usergroup_id, reference_id, permission) SELECT r.id, g.id, ?, ? FROM %s r, usergroup g WHERE r.reference_id = ? AND g.reference_id = ?", joinTable, table, table)
		result, err := db.Exec(db.Rebind(query), dest[:], permission, src[:], gid[:])
		if err != nil {
			t.Fatal(err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			t.Fatalf("seed fixture grant: count=%d err=%v", count, err)
		}
		return dest
	}
	unlink := func(ref uuid.UUID) {
		t.Helper()
		if _, err := db.Exec(db.Rebind("DELETE FROM "+membership+" WHERE reference_id = ?"), ref[:]); err != nil {
			t.Fatal(err)
		}
	}
	joinA, joinB := link("user_account", user, groupA, 0), link("user_account", user, groupB, 0)
	grant := func(table, ref, group string) {
		link(table, ref, group, int64(auth.GroupPeek|auth.GroupRead))
	}
	for _, group := range []string{groupA, groupB} {
		grant("mail_account", account, group)
		grant("mail_box", box, group)
		grant("mail", first, group)
	}
	grant("mail", second, groupB)
	list := func(token string, page, total int, want string) {
		t.Helper()
		filters, _ := json.Marshal([]map[string]string{{"column": "mail_box_id", "operator": "is", "value": box}, {"column": "subject", "operator": "contains_literal", "value": "group-"}})
		query := url.Values{"page[number]": {fmt.Sprint(page)}, "page[size]": {"1"}, "sort": {"uid"}, "fields": {"reference_id,subject"}, "query": {string(filters)}}
		value := accessGroupsE2ERequestJSON(t, f.Client, http.MethodGet, f.URL+"/api/mail?"+query.Encode(), token, nil, http.StatusOK)
		rows := accessGroupsE2EDataArray(t, value)
		count := value.(map[string]interface{})["links"].(map[string]interface{})["total"]
		var got string
		if len(rows) == 1 {
			got = rows[0].(map[string]interface{})["id"].(string)
		}
		if got != want || len(rows) > 1 || count != float64(total) {
			t.Errorf("group Mail page %d: got id=%q count=%v, want id=%q count=%d", page, got, count, want, total)
		}
	}
	read := func(ref string, status int) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodGet, f.URL+"/api/mail/"+ref, member, nil, status)
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/action/mail/unpack", member,
			map[string]interface{}{"attributes": map[string]interface{}{"message_ref": ref}}, status)
	}
	status := func(token string, total, unread, recent int, latest string) {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, f.URL+"/action/mail_box/status", token,
			map[string]interface{}{"attributes": map[string]interface{}{"mail_account_id": account}}, http.StatusOK)
		rows := response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})["data"].([]interface{})
		if len(rows) != 1 {
			t.Fatalf("expected one readable status row, got %d", len(rows))
		}
		row := rows[0].(map[string]interface{})
		var gotLatest string
		if message, ok := row["latest_message"].(map[string]interface{}); ok {
			gotLatest, _ = message["reference_id"].(string)
		}
		if row["messages"] != float64(total) || row["unseen"] != float64(unread) || row["recent"] != float64(recent) || gotLatest != latest {
			t.Errorf("Mail status: got messages=%v unseen=%v recent=%v latest=%q; want %d/%d/%d/latest=%q",
				row["messages"], row["unseen"], row["recent"], gotLatest, total, unread, recent, latest)
		}
		if row["uidnext"] != float64(3) || row["uidvalidity"] != float64(1) {
			t.Errorf("mailbox UID state changed: %#v", row)
		}
	}
	list(other, 1, 0, "")
	list(member, 1, 2, first)
	list(member, 2, 2, second)
	read(first, http.StatusOK)
	status(member, 2, 1, 1, second)
	unlink(joinB)
	list(member, 1, 1, first)
	list(member, 2, 1, "")
	status(member, 1, 0, 0, first)
	// Keep parents readable so no ancestor check can mask a stale message grant.
	patch("mail_account", account, int64(auth.GuestRead))
	patch("mail_box", box, int64(auth.GuestRead))
	status(other, 0, 0, 0, "")
	read(second, http.StatusForbidden)
	unlink(joinA)
	list(member, 1, 0, "")
	read(first, http.StatusForbidden)
	status(member, 0, 0, 0, "")
	link("user_account", user, groupB, 0)
	list(member, 1, 2, first)
	list(member, 2, 2, second)
	read(second, http.StatusOK)
	status(member, 2, 1, 1, second)
	if !t.Failed() {
		t.Log("Overlapping group grants occupy one row per message; the same bearer session reflects removal and restoration.")
		t.Log("Mailbox status counts and latest message follow readable rows; UID state stays fixed.")
	}
}
