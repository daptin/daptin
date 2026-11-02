package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"golang.org/x/net/websocket"
)

const topicPublishSwitchSchema = `
Actions:
  - Name: publish_as_selected_account
    OnType: world
    InstanceOptional: true
    Permission: 0
    InFields:
      - Name: user_reference_id
        ColumnType: label
      - Name: topicName
        ColumnType: label
      - Name: message
        ColumnType: content
    OutFields:
      - Method: SWITCH_USER
        Type: __as_user
        Attributes:
          user_reference_id: ~user_reference_id
      - Method: EXECUTE
        Type: __publish_to_topic
        Attributes:
          topicName: ~topicName
          message: ~message
`

func TestTopicPublishUsesPersistedPermissionsRealE2E(t *testing.T) {
	options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "topics.db"), schema: topicPublishSwitchSchema}
	f := startDaptinE2E(t, options)
	admin := accessGroupsE2ESignupSigninAdmin(t, f.Client, f.URL)
	f.Process.stopProcess()
	f = startDaptinE2E(t, options)
	user := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "topic-owner")
	other := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "topic-other")
	userID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "topic-owner@test.local")
	otherID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "topic-other@test.local")
	adminID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "admin@test.local")
	endpoint := f.URL + "/action/world/publish_to_topic"
	payload := func(topic, marker string) map[string]interface{} {
		return map[string]interface{}{"attributes": map[string]interface{}{"topicName": topic, "message": map[string]interface{}{"marker": marker}}}
	}
	for _, token := range []string{"", user} {
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, token, payload("world", "wrapper-denied"), http.StatusForbidden)
	}
	actionID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "action", "action_name", "publish_to_topic")
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/action/"+actionID, admin,
		accessGroupsE2ERecordPayload("action", actionID, map[string]interface{}{"permission": int64(auth.GuestExecute)}), http.StatusOK)

	dial := func(token string) *websocket.Conn {
		t.Helper()
		config, err := websocket.NewConfig("ws"+strings.TrimPrefix(f.URL, "http")+"/live", f.URL)
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
			t.Fatalf("missing user session: %#v", message)
		}
		return ws
	}
	observer := websocketRowAuthDial(t, f.URL, admin, "world")
	expectEvent := func(topic, marker, source string) {
		t.Helper()
		event := recvJSON(t, observer, 5*time.Second)
		var message map[string]interface{}
		if err := json.Unmarshal(event.Data, &message); err != nil || event.Topic != topic || event.Event != "new-message" || message["marker"] != marker || event.Source != source {
			t.Fatalf("unexpected event: %#v message=%#v error=%v", event, message, err)
		}
	}
	publish := func(token, topic, marker, source string) {
		t.Helper()
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost, endpoint, token, payload(topic, marker), http.StatusOK)
		expectEvent(topic, marker, source)
	}
	deny := func(token, topic string) {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost, endpoint, token, payload(topic, "must-not-publish"), http.StatusInternalServerError)
		encoded, _ := json.Marshal(response)
		if !strings.Contains(string(encoded), "permission denied") {
			t.Fatalf("expected topic denial: %s", encoded)
		}
		// A permitted message is a barrier: a denied message must not precede it.
		publish(admin, "world", "denial-barrier", adminID)
	}
	direct := func(token, topic, marker, source string, allowed bool) {
		t.Helper()
		ws := dial(token)
		defer ws.Close()
		sendJSON(t, ws, wsPayload{Id: "direct-publish", Method: "new-message", Payload: map[string]interface{}{
			"topicName": topic, "message": map[string]interface{}{"marker": marker},
		}})
		if allowed {
			expectEvent(topic, marker, source)
		} else {
			message := recvJSON(t, ws, 5*time.Second)
			if message.Ok == nil || *message.Ok || !strings.Contains(message.Error, "permission denied") {
				t.Fatalf("direct publish not denied: %#v", message)
			}
			publish(admin, "world", "direct-denial-barrier", adminID)
		}
	}
	switchPublish := func(targetID string, allowed bool) {
		t.Helper()
		body := payload("world", "switched-publisher")
		body["attributes"].(map[string]interface{})["user_reference_id"] = targetID
		want := http.StatusInternalServerError
		if allowed {
			want = http.StatusOK
		}
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost,
			f.URL+"/action/world/publish_as_selected_account", admin, body, want)
		if allowed {
			expectEvent("world", "switched-publisher", targetID)
		} else {
			encoded, _ := json.Marshal(response)
			if !strings.Contains(string(encoded), "permission denied") {
				t.Fatalf("expected switched-user denial: %s", encoded)
			}
			publish(admin, "world", "switch-denial-barrier", adminID)
		}
	}

	t.Run("system topic", func(t *testing.T) {
		deny(user, "world")
		deny("", "world")
		direct(user, "world", "direct-denied", userID, false)
		publish(admin, "world", "real-admin", adminID)
		direct(admin, "world", "direct-admin", adminID, true)
		switchPublish(userID, false)

		groupID := accessGroupsE2ECreateUsergroup(t, f.Client, f.URL, admin, "topic-publishers")
		accessGroupsE2ECreateJoin(t, f.Client, f.URL, admin, "user_account_user_account_id_has_usergroup_usergroup_id",
			map[string]interface{}{"user_account_id": userID, "usergroup_id": groupID}, 0)
		worldID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "world", "table_name", "world")
		const grants = "world_world_id_has_usergroup_usergroup_id"
		grantID := accessGroupsE2ECreateJoin(t, f.Client, f.URL, admin, grants,
			map[string]interface{}{"world_id": worldID, "usergroup_id": groupID}, int64(auth.GroupCreate))
		publish(user, "world", "explicit-grant", userID)
		direct(user, "world", "direct-granted", userID, true)
		switchPublish(userID, true)
		switchPublish(otherID, false)

		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/"+grants+"/"+grantID, admin,
			accessGroupsE2ERecordPayload(grants, grantID, map[string]interface{}{"permission": 0}), http.StatusOK)
		deny(user, "world")
		direct(user, "world", "direct-revoked", userID, false)
		switchPublish(userID, false)
		t.Log("Wrapper execution does not grant topic access; explicit membership grants, revocation, administrators and SWITCH_USER verified")
	})

	t.Run("custom topic", func(t *testing.T) {
		const topic = "owned-topic-probe"
		ownerWS := dial(user)
		sendJSON(t, ownerWS, wsPayload{Id: "create-owned", Method: "create-topicName", Payload: map[string]interface{}{"name": topic}})
		if message := recvJSON(t, ownerWS, 5*time.Second); message.Ok == nil || !*message.Ok {
			t.Fatalf("topic creation failed: %#v", message)
		}
		sendJSON(t, observer, wsPayload{Id: "observe-owned", Method: "subscribe", Payload: map[string]interface{}{"topicName": topic}})
		if message := recvJSON(t, observer, 5*time.Second); message.Ok == nil || !*message.Ok {
			t.Fatalf("topic subscription failed: %#v", message)
		}
		publish(user, topic, "topic-owner", userID)
		direct(user, topic, "direct-owner", userID, true)
		publish(admin, topic, "admin-custom", adminID)
		deny(other, topic)
		deny("", topic)
		direct(other, topic, "direct-nonowner", otherID, false)

		setPermission := func(permission auth.AuthPermission) {
			sendJSON(t, ownerWS, wsPayload{Id: "topic-policy", Method: "set-topic-permission", Payload: map[string]interface{}{"topicName": topic, "permission": int64(permission)}})
			if message := recvJSON(t, ownerWS, 5*time.Second); message.Ok == nil || !*message.Ok {
				t.Fatalf("topic permission update failed: %#v", message)
			}
		}
		setPermission(auth.UserCRUD | auth.UserExecute | auth.GuestExecute)
		publish("", topic, "explicit-guest", "00000000-0000-0000-0000-000000000000")
		direct(other, topic, "direct-public", otherID, true)
		setPermission(auth.UserCRUD | auth.UserExecute)
		deny("", topic)
		t.Log("Custom-topic ownership, administrator access, explicit guest execution and revocation verified")
	})
}
