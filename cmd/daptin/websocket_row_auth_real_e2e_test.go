package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

const websocketRowAuthSchema = `Tables:
  - TableName: ws_acl_probe
    Permission: 63
    DefaultPermission: 16256
    IsStateTrackingEnabled: true
    Columns:
      - Name: label
        ColumnType: label
        DataType: varchar(100)
      - Name: password
        ColumnType: password
        DataType: varchar(200)
      - Name: secret
        ColumnType: label
        DataType: varchar(100)
        ExcludeFromApi: true
Actions:
  - Name: change_label
    OnType: ws_acl_probe
    InstanceOptional: true
    # Keep this action explicitly executable after administrator onboarding.
    Permission: 32
    InFields:
      - Name: record_id
        ColumnName: record_id
        ColumnType: label
      - Name: label
        ColumnName: label
        ColumnType: label
    OutFields:
      - Type: ws_acl_probe
        Method: PATCH
        Attributes:
          reference_id: ~record_id
          label: ~label
StateMachineDescriptions:
  - Name: ws_probe_workflow
    InitialState: pending
    Events:
      - Name: confirm
        Src: [pending]
        Dst: confirmed
`

func websocketRowAuthDial(t *testing.T, base, token, topic string) *websocket.Conn {
	t.Helper()
	config, err := websocket.NewConfig("ws"+strings.TrimPrefix(base, "http")+"/live", base)
	if err != nil {
		t.Fatal(err)
	}
	config.Header.Set("Authorization", "Bearer "+token)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	if message := recvJSON(t, ws, 5*time.Second); message.Type != "session" {
		t.Fatalf("expected authenticated session, got %#v", message)
	}
	sendJSON(t, ws, wsPayload{Id: "subscribe", Method: "subscribe", Payload: map[string]interface{}{"topicName": topic}})
	message := recvJSON(t, ws, 5*time.Second)
	if message.Method != "subscribe" || message.Ok == nil || !*message.Ok {
		t.Fatalf("subscribe failed: %#v", message)
	}
	return ws
}

func websocketRowAuthAssertEvent(t *testing.T, event resource.WsOutMessage, kind, id string) {
	t.Helper()
	var row map[string]interface{}
	if err := json.Unmarshal(event.Data, &row); err != nil {
		t.Fatal(err)
	}
	if event.Type != "event" || event.Event != kind || row["reference_id"] != id {
		t.Fatalf("unexpected event: kind=%s data=%s", event.Event, event.Data)
	}
	for _, key := range []string{"id", "__permission", "secret"} {
		if _, exists := row[key]; exists {
			t.Fatalf("internal/excluded field %s reached subscriber", key)
		}
	}
	if value, exists := row["password"]; exists && value != "" {
		t.Fatal("stored password reached subscriber")
	}
}

func TestWebSocketRowAuthorizationRealE2E(t *testing.T) {
	requireRealE2E(t)
	ports := map[int]bool{}
	port, httpsPort := freeTransportE2EPort(t, ports), freeTransportE2EPort(t, ports)
	olricPort := freeTransportE2EPortPair(t, ports)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	// The explicit Olric port lets the test inject malformed internal events.
	process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{schema: websocketRowAuthSchema, olricPort: olricPort})
	defer process.stopProcess()
	client := &http.Client{Timeout: 20 * time.Second}
	admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
	owner := accessGroupsE2ESignupSigninUser(t, client, base, admin, "ws-owner")
	other := accessGroupsE2ESignupSigninUser(t, client, base, admin, "ws-other")
	reader := accessGroupsE2ESignupSigninUser(t, client, base, admin, "ws-reader")
	otherID := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "ws-other@test.local")
	group := accessGroupsE2ECreateUsergroup(t, client, base, admin, "ws-readers")
	readerID := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "ws-reader@test.local")
	accessGroupsE2ECreateJoin(t, client, base, admin, "user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{"user_account_id": readerID, "usergroup_id": group}, 0)
	const topic = "ws_acl_probe"
	sockets := map[string]*websocket.Conn{
		"owner": websocketRowAuthDial(t, base, owner, topic), "other": websocketRowAuthDial(t, base, other, topic),
		"reader": websocketRowAuthDial(t, base, reader, topic), "admin": websocketRowAuthDial(t, base, admin, topic),
	}
	// A marker on the same PubSub channel bounds denial assertions without
	// treating a socket timeout (or a disconnected socket) as proof of denial.
	barrier := func(kind, id string, allowed map[string]bool, ownerConsumed bool) {
		t.Helper()
		marker := uuid.NewString()
		sendJSON(t, sockets["admin"], wsPayload{Method: "new-message", Payload: map[string]interface{}{"topicName": topic, "message": map[string]interface{}{"marker": marker}}})
		for name, ws := range sockets {
			count := 0
			if name == "owner" && ownerConsumed {
				count = 1
			}
			for {
				event := recvJSON(t, ws, 5*time.Second)
				if event.Event == "new-message" && strings.Contains(string(event.Data), marker) {
					break
				}
				if !allowed[name] {
					t.Fatalf("%s received a denied event: %s %s", name, event.Event, event.Data)
				}
				websocketRowAuthAssertEvent(t, event, kind, id)
				count++
			}
			want := 0
			if allowed[name] {
				want = 1
			}
			if count != want {
				t.Fatalf("%s received %d %s events, want %d", name, count, kind, want)
			}
		}
	}
	phase := func(kind, id string, groupRead bool) {
		t.Helper()
		websocketRowAuthAssertEvent(t, recvJSON(t, sockets["owner"], 5*time.Second), kind, id)
		barrier(kind, id, map[string]bool{"owner": true, "admin": true, "reader": groupRead}, true)
	}
	id := accessGroupsE2ECreateRecord(t, client, base, owner, topic, map[string]interface{}{"label": "initial", "password": "SyntheticPass123!", "secret": "private"})
	phase("create", id, false)
	patch := func(label string) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/"+topic+"/"+id, owner, accessGroupsE2ERecordPayload(topic, id, map[string]interface{}{"label": label}), http.StatusOK)
	}
	patch("rest")
	phase("update", id, false)
	accessGroupsE2EAssertStatus(t, client, http.MethodGet, base+"/api/"+topic+"/"+id, other, nil, http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/"+topic+"/change_label", owner, map[string]interface{}{"attributes": map[string]interface{}{"record_id": id, "label": "action"}}, http.StatusOK)
	phase("update", id, false)
	joinEntity := topic + "_" + topic + "_id_has_usergroup_usergroup_id"
	share := func() string {
		return accessGroupsE2ECreateJoin(t, client, base, admin, joinEntity, map[string]interface{}{topic + "_id": id, "usergroup_id": group}, int64(auth.GroupRead))
	}
	join := share()
	patch("shared")
	phase("update", id, true)
	accessGroupsE2EAssertStatus(t, client, http.MethodDelete, base+"/api/"+joinEntity+"/"+join, admin, nil, http.StatusOK)
	patch("revoked")
	phase("update", id, false)

	cacheClient, err := olric.NewClusterClient([]string{fmt.Sprintf("127.0.0.1:%d", olricPort)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cacheClient.Close(ctx)
	}()
	pubsub, err := cacheClient.NewPubSub()
	if err != nil {
		t.Fatal(err)
	}
	publish := func(event resource.WsOutMessage) {
		t.Helper()
		if _, err := pubsub.Publish(context.Background(), topic, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, payload := range []string{
		`{`, `null`, `{}`, `{"reference_id":"` + id + `"}`, `{"__type":"user_account","reference_id":"` + id + `"}`,
		`{"__type":"ws_acl_probe","reference_id":"invalid"}`,
		`{"__type":"ws_acl_probe","reference_id":"` + uuid.NewString() + `"}`,
	} {
		publish(resource.WsOutMessage{Type: "event", Topic: topic, Event: "update", Source: "database", Data: []byte(payload)})
	}
	for _, snapshot := range []string{`null`, `{}`, `"invalid"`, `{"UserId":null,"Permission":2097151}`} {
		publish(resource.WsOutMessage{Type: "event", Topic: topic, Event: "delete", Source: "database", Data: []byte(`{"__type":"ws_acl_probe","reference_id":"` + id + `","__permission":` + snapshot + `}`)})
	}
	for _, data := range [][]byte{[]byte("invalid"), {255, 255, 255, 255}, {127, 255, 255, 255}} {
		if _, err := pubsub.Publish(context.Background(), topic, data); err != nil {
			t.Fatal(err)
		}
	}
	barrier("", "", nil, false)
	// Supplied ACL/owner attributes cannot override current persisted access.
	publish(resource.WsOutMessage{Type: "event", Topic: topic, Event: "update", Source: "database", Data: []byte(`{"__type":"ws_acl_probe","reference_id":"` + id + `","user_account_id":"` + otherID + `","permission":2097151,"password":"synthetic-hash","secret":"private","id":1}`)})
	phase("update", id, false)

	// Both public custom-message entry points still use the same delivery path.
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/world/publish_to_topic", admin, map[string]interface{}{"attributes": map[string]interface{}{"topicName": topic, "message": map[string]interface{}{"__type": "user_account", "text": "custom"}}}, http.StatusOK)
	for _, ws := range sockets {
		event := recvJSON(t, ws, 5*time.Second)
		if event.Event != "new-message" || !strings.Contains(string(event.Data), "custom") {
			t.Fatalf("custom action message missing: %#v", event)
		}
	}
	share()
	patch("shared-for-delete")
	phase("update", id, true)
	accessGroupsE2EAssertStatus(t, client, http.MethodDelete, base+"/api/"+topic+"/"+id, owner, nil, http.StatusOK)
	phase("delete", id, true)
}

func TestWebSocketStateTransitionAuthorizationRealE2E(t *testing.T) {
	f := startDaptinE2E(t, transportE2EDaptinOptions{schema: websocketRowAuthSchema})
	defer f.Process.stopProcess()
	admin := f.SignupAdmin(t)
	other := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "ws-state-other")
	id := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "ws_acl_probe", map[string]interface{}{"label": "state", "password": "SyntheticPass123!", "secret": "private"})
	smd := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "smd", "name", "ws_probe_workflow")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/track/start/"+smd, admin, map[string]interface{}{"typeName": "ws_acl_probe", "referenceId": id}, http.StatusOK)
	stateID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "ws_acl_probe_state", "current_state", "pending")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/ws_acl_probe_state/"+stateID, admin, accessGroupsE2ERecordPayload("ws_acl_probe_state", stateID, map[string]interface{}{"permission": int64(auth.UserRead)}), http.StatusOK)
	ownerWS := websocketRowAuthDial(t, f.URL, admin, "ws_acl_probe_state")
	otherWS := websocketRowAuthDial(t, f.URL, other, "ws_acl_probe_state")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, f.URL+"/track/event/ws_acl_probe/"+stateID+"/confirm", admin, map[string]interface{}{}, http.StatusOK)
	websocketRowAuthAssertEvent(t, recvJSON(t, ownerWS, 5*time.Second), "update", stateID)
	sendJSON(t, ownerWS, wsPayload{Method: "new-message", Payload: map[string]interface{}{"topicName": "ws_acl_probe_state", "message": map[string]interface{}{"marker": "done"}}})
	if event := recvJSON(t, otherWS, 5*time.Second); event.Event != "new-message" {
		t.Fatalf("unrelated user received private state transition: %#v", event)
	}
}
