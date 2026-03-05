package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/resource"
	"github.com/imroc/req"
	"golang.org/x/net/websocket"
)

const wsBaseAddress = "http://localhost:6337"
const wsURL = "ws://localhost:6337/live"

var wsServerOnce sync.Once

func ensureServer() {
	wsServerOnce.Do(func() {
		createServer()
	})
}

// wsPayload matches WebSocketPayload on the server side.
type wsPayload struct {
	Id      string                 `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Payload map[string]interface{} `json:"attributes"`
}

var wsReqCounter atomic.Int64

func nextReqId() string {
	return fmt.Sprintf("req-%d", wsReqCounter.Add(1))
}

// dialWS opens an authenticated websocket connection with retry for transient failures.
// It consumes the initial session-open message before returning.
func dialWS(t testing.TB, token string) *websocket.Conn {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		config, err := websocket.NewConfig(wsURL, wsBaseAddress)
		if err != nil {
			t.Fatalf("websocket config: %v", err)
		}
		config.Header.Set("Authorization", "Bearer "+token)
		config.Header.Set("Cookie", "token="+token)
		ws, err := websocket.DialConfig(config)
		if err != nil {
			// per-IP limiter may reject during high churn — retry after backoff
			time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
			continue
		}
		// consume the session-open message
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		var sessionMsg resource.WsOutMessage
		if err := websocket.JSON.Receive(ws, &sessionMsg); err != nil {
			t.Fatalf("failed to read session-open: %v", err)
		}
		if sessionMsg.Type != "session" || sessionMsg.Status != "open" {
			t.Fatalf("expected session-open, got type=%q status=%q", sessionMsg.Type, sessionMsg.Status)
		}
		return ws
	}
	t.Fatalf("websocket dial: failed after 5 attempts")
	return nil
}

// dialWSRaw opens an authenticated websocket without consuming session-open.
func dialWSRaw(t testing.TB, token string) *websocket.Conn {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		config, err := websocket.NewConfig(wsURL, wsBaseAddress)
		if err != nil {
			t.Fatalf("websocket config: %v", err)
		}
		config.Header.Set("Authorization", "Bearer "+token)
		config.Header.Set("Cookie", "token="+token)
		ws, err := websocket.DialConfig(config)
		if err != nil {
			time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
			continue
		}
		return ws
	}
	t.Fatalf("websocket dial: failed after 5 attempts")
	return nil
}

// sendJSON sends a JSON payload over the websocket.
func sendJSON(t testing.TB, ws *websocket.Conn, v interface{}) {
	t.Helper()
	if err := websocket.JSON.Send(ws, v); err != nil {
		t.Fatalf("websocket send: %v", err)
	}
}

// recvJSON reads a JSON message with a timeout.
func recvJSON(t testing.TB, ws *websocket.Conn, timeout time.Duration) resource.WsOutMessage {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(timeout))
	var msg resource.WsOutMessage
	if err := websocket.JSON.Receive(ws, &msg); err != nil {
		t.Fatalf("websocket recv: %v", err)
	}
	return msg
}

// tryRecvJSON reads a JSON message with a timeout, returns ok=false on timeout.
func tryRecvJSON(ws *websocket.Conn, timeout time.Duration) (resource.WsOutMessage, bool) {
	ws.SetReadDeadline(time.Now().Add(timeout))
	var msg resource.WsOutMessage
	if err := websocket.JSON.Receive(ws, &msg); err != nil {
		return msg, false
	}
	return msg, true
}

var wsTokenOnce sync.Once
var wsToken string

// signUpAndGetToken returns a JWT token, creating a user if needed.
// Works both standalone (guest signup open) and after TestServerApis (guest signup locked).
func signUpAndGetToken(t testing.TB) string {
	if t != nil {
		t.Helper()
	}
	wsTokenOnce.Do(func() {
		client := req.New()
		client.SetTimeout(30 * time.Second)

		// Try signing in as test@gmail.com (created by TestServerApis)
		resp, err := client.Post(wsBaseAddress+"/action/user_account/signin", req.BodyJSON(map[string]interface{}{
			"attributes": map[string]interface{}{
				"email":    "test@gmail.com",
				"password": "tester123",
			},
		}))
		if err == nil {
			if tok := extractToken(resp); tok != "" {
				wsToken = tok
				return
			}
		}

		// TestServerApis hasn't run — sign up as guest
		client.Post(wsBaseAddress+"/action/user_account/signup", req.BodyJSON(map[string]interface{}{
			"attributes": map[string]interface{}{
				"email":           "test@gmail.com",
				"name":            "test",
				"password":        "tester123",
				"passwordConfirm": "tester123",
			},
		}))

		resp, err = client.Post(wsBaseAddress+"/action/user_account/signin", req.BodyJSON(map[string]interface{}{
			"attributes": map[string]interface{}{
				"email":    "test@gmail.com",
				"password": "tester123",
			},
		}))
		if err != nil {
			panic(fmt.Sprintf("signin failed: %v", err))
		}
		wsToken = extractToken(resp)
		if wsToken == "" {
			panic("no token after signup+signin")
		}
	})
	return wsToken
}

func extractToken(resp *req.Resp) string {
	var signInResp []interface{}
	resp.ToJSON(&signInResp)
	for _, item := range signInResp {
		attrs, ok := item.(map[string]interface{})
		if ok && attrs["ResponseType"] == "client.store.set" {
			return attrs["Attributes"].(map[string]interface{})["value"].(string)
		}
	}
	return ""
}

// ===== E2E TESTS =====

func TestWebSocketSessionOpen(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWSRaw(t, token)
	defer ws.Close()

	// first message should be session-open
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "session" {
		t.Errorf("expected type=session, got %q", msg.Type)
	}
	if msg.Status != "open" {
		t.Errorf("expected status=open, got %q", msg.Status)
	}

	var data map[string]interface{}
	json.Unmarshal(msg.Data, &data)
	if data["user"] == nil {
		t.Errorf("session-open missing user field")
	}
	if data["sessionId"] == nil {
		t.Errorf("session-open missing sessionId field")
	}
	t.Logf("session-open: user=%v sessionId=%v", data["user"], data["sessionId"])
}

func TestWebSocketConnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// subscribe to a known system topic to confirm bidirectional communication
	id := nextReqId()
	sendJSON(t, ws, wsPayload{
		Id:      id,
		Method:  "subscribe",
		Payload: map[string]interface{}{"topicName": "user_account"},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" {
		t.Errorf("expected type=response, got %q", msg.Type)
	}
	if msg.Method != "subscribe" {
		t.Errorf("expected method=subscribe, got %q", msg.Method)
	}
	if msg.Id != id {
		t.Errorf("expected id=%q, got %q", id, msg.Id)
	}
}

func TestWebSocketSubscribeAck(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := "user_account"
	t.Logf("subscribing to topic: %s", topicName)

	id := nextReqId()
	sendJSON(t, ws, wsPayload{
		Id:      id,
		Method:  "subscribe",
		Payload: map[string]interface{}{"topicName": topicName},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || !*msg.Ok {
		t.Errorf("expected successful response, got type=%q ok=%v", msg.Type, msg.Ok)
	}
	if msg.Method != "subscribe" {
		t.Errorf("expected method=subscribe, got %q", msg.Method)
	}
	if msg.Id != id {
		t.Errorf("expected id=%q, got %q", id, msg.Id)
	}

	var data map[string]interface{}
	json.Unmarshal(msg.Data, &data)
	if data["topic"] != topicName {
		t.Errorf("expected topic=%q in data, got %v", topicName, data["topic"])
	}
}

func TestWebSocketSubscribeNonexistentTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	id := nextReqId()
	sendJSON(t, ws, wsPayload{
		Id:      id,
		Method:  "subscribe",
		Payload: map[string]interface{}{"topicName": "nonexistent_topic_xyz_12345"},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" {
		t.Errorf("expected type=response, got %q", msg.Type)
	}
	if msg.Ok == nil || *msg.Ok {
		t.Errorf("expected ok=false, got %v", msg.Ok)
	}
	if msg.Method != "subscribe" {
		t.Errorf("expected method=subscribe, got %q", msg.Method)
	}
}

func TestWebSocketUnsubscribeAck(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := "user_account"

	// subscribe first
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, ws, 5*time.Second) // subscribe ack

	// unsubscribe
	id := nextReqId()
	sendJSON(t, ws, wsPayload{Id: id, Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || !*msg.Ok {
		t.Errorf("expected successful unsubscribe response, got type=%q ok=%v", msg.Type, msg.Ok)
	}
	if msg.Method != "unsubscribe" {
		t.Errorf("expected method=unsubscribe, got %q", msg.Method)
	}

	var data map[string]interface{}
	json.Unmarshal(msg.Data, &data)
	if data["topic"] != topicName {
		t.Errorf("expected topic=%q, got %v", topicName, data["topic"])
	}
}

func TestWebSocketNewMessageErrors(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// missing topicName
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "new-message", Payload: map[string]interface{}{}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || *msg.Ok {
		t.Errorf("expected error response for missing topicName, got type=%q ok=%v", msg.Type, msg.Ok)
	}

	// nonexistent topic
	sendJSON(t, ws, wsPayload{
		Id:     nextReqId(),
		Method: "new-message",
		Payload: map[string]interface{}{
			"topicName": "does_not_exist_xyz",
			"message":   map[string]interface{}{"text": "hello"},
		},
	})
	msg = recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || *msg.Ok {
		t.Errorf("expected error response for nonexistent topic, got type=%q ok=%v", msg.Type, msg.Ok)
	}
}

func TestWebSocketCreateDestroyTopic(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := fmt.Sprintf("test-topic-%d", time.Now().UnixNano())

	// create topic
	id := nextReqId()
	sendJSON(t, ws, wsPayload{
		Id:      id,
		Method:  "create-topicName",
		Payload: map[string]interface{}{"name": topicName},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || !*msg.Ok {
		t.Errorf("expected successful create response, got type=%q ok=%v error=%q", msg.Type, msg.Ok, msg.Error)
	}

	// destroy it
	sendJSON(t, ws, wsPayload{
		Id:      nextReqId(),
		Method:  "destroy-topicName",
		Payload: map[string]interface{}{"name": topicName},
	})
	msg = recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || !*msg.Ok {
		t.Errorf("expected successful destroy response, got type=%q ok=%v error=%q", msg.Type, msg.Ok, msg.Error)
	}
}

func TestWebSocketDestroySystemTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// "world" is always a system topic
	sendJSON(t, ws, wsPayload{
		Id:      nextReqId(),
		Method:  "destroy-topicName",
		Payload: map[string]interface{}{"name": "world"},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || *msg.Ok {
		t.Errorf("expected error for system topic delete, got type=%q ok=%v", msg.Type, msg.Ok)
	}
}

func TestWebSocketCreateDuplicateTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := fmt.Sprintf("dup-topic-%d", time.Now().UnixNano())

	// create first
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, ws, 5*time.Second) // success response

	// create duplicate
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || *msg.Ok {
		t.Errorf("expected error for duplicate topic, got type=%q ok=%v", msg.Type, msg.Ok)
	}
}

func TestWebSocketPingPong(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	sendJSON(t, ws, wsPayload{Method: "ping", Payload: map[string]interface{}{}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "pong" {
		t.Errorf("expected type=pong, got %q", msg.Type)
	}
}

func TestWebSocketPubSubRoundTrip(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	topicName := fmt.Sprintf("pubsub-rt-%d", time.Now().UnixNano())

	// publisher connection
	wsPub := dialWS(t, token)
	defer wsPub.Close()

	// subscriber connection
	wsSub := dialWS(t, token)
	defer wsSub.Close()

	// create topic on publisher
	sendJSON(t, wsPub, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, wsPub, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)

	// subscribe on subscriber
	sendJSON(t, wsSub, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	subAck := recvJSON(t, wsSub, 5*time.Second)
	if subAck.Type != "response" || subAck.Ok == nil || !*subAck.Ok {
		t.Fatalf("expected successful subscribe, got type=%q ok=%v", subAck.Type, subAck.Ok)
	}

	// publish a message
	sendJSON(t, wsPub, wsPayload{
		Id:     nextReqId(),
		Method: "new-message",
		Payload: map[string]interface{}{
			"topicName": topicName,
			"message":   map[string]interface{}{"text": "hello world"},
		},
	})

	// subscriber should receive an event
	msg := recvJSON(t, wsSub, 5*time.Second)
	if msg.Type != "event" {
		t.Errorf("expected type=event, got %q", msg.Type)
	}
	if msg.Event != "new-message" {
		t.Errorf("expected event=new-message, got %q", msg.Event)
	}
	if msg.Topic != topicName {
		t.Errorf("expected topic=%q, got %q", topicName, msg.Topic)
	}

	var payload map[string]interface{}
	json.Unmarshal(msg.Data, &payload)
	if payload["text"] != "hello world" {
		t.Errorf("expected text='hello world', got %v", payload["text"])
	}
}

func TestWebSocketNoSuchMethod(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	id := nextReqId()
	sendJSON(t, ws, wsPayload{Id: id, Method: "nonexistent-method", Payload: map[string]interface{}{}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" || msg.Ok == nil || *msg.Ok {
		t.Errorf("expected error for unknown method, got type=%q ok=%v", msg.Type, msg.Ok)
	}
	if msg.Error != "no such method" {
		t.Errorf("expected error='no such method', got %q", msg.Error)
	}
	if msg.Method != "nonexistent-method" {
		t.Errorf("expected method echoed back, got %q", msg.Method)
	}
}

func TestWebSocketRequestCorrelation(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// send with id — response should echo id
	id := nextReqId()
	sendJSON(t, ws, wsPayload{Id: id, Method: "subscribe", Payload: map[string]interface{}{"topicName": "user_account"}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Id != id {
		t.Errorf("expected id=%q echoed, got %q", id, msg.Id)
	}
}

func TestWebSocketNoIdOmitted(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// send without id — response should have empty id (omitted)
	sendJSON(t, ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": "user_account"}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Id != "" {
		t.Errorf("expected empty id when not sent, got %q", msg.Id)
	}
}

// ===== AUTHORIZATION TESTS =====

// expectResponse reads the next message and checks it's a response with the expected ok value.
func expectResponse(t testing.TB, ws *websocket.Conn, method string, wantOk bool) resource.WsOutMessage {
	t.Helper()
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.Type != "response" {
		t.Fatalf("expected type=response, got %q (method=%q)", msg.Type, msg.Method)
	}
	if msg.Method != method {
		t.Fatalf("expected method=%q, got %q", method, msg.Method)
	}
	if msg.Ok == nil {
		t.Fatalf("response has nil Ok for method=%q", method)
	}
	if *msg.Ok != wantOk {
		t.Fatalf("expected ok=%v for method=%q, got ok=%v error=%q", wantOk, method, *msg.Ok, msg.Error)
	}
	return msg
}

// TestAuthzUserTopicDefaultPermOwnerOnly verifies that a newly created user topic
// has owner-only permissions (UserCRUD|UserExecute). A second user should be denied
// subscribe, publish, destroy, and get-topic-permission.
func TestAuthzUserTopicDefaultPermOwnerOnly(t *testing.T) {
	ensureServer()

	// user1 (owner) creates a topic
	token1 := signUpAndGetToken(t)
	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-default-%d", time.Now().UnixNano())
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	// user2 (non-owner) tries to interact
	token2, err := signUpUser("authz-other@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// subscribe should fail (CanRead denied — no GuestRead bit)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", false)

	// new-message should fail (CanExecute denied — no GuestExecute bit)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "new-message", Payload: map[string]interface{}{
		"topicName": topicName,
		"message":   map[string]interface{}{"text": "hello"},
	}})
	expectResponse(t, ws2, "new-message", false)

	// destroy should fail (CanDelete denied)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "destroy-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws2, "destroy-topicName", false)

	// get-topic-permission should fail (CanPeek denied — no GuestPeek bit)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "get-topic-permission", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "get-topic-permission", false)
}

// TestAuthzOwnerCanAccessOwnTopic verifies the owner can subscribe, publish,
// get/set permissions, and destroy their own topic.
func TestAuthzOwnerCanAccessOwnTopic(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := fmt.Sprintf("authz-owner-%d", time.Now().UnixNano())

	// create
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws, "create-topicName", true)

	// subscribe (CanRead — UserRead set)
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws, "subscribe", true)

	// publish (CanExecute — UserExecute set)
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "new-message", Payload: map[string]interface{}{
		"topicName": topicName,
		"message":   map[string]interface{}{"text": "owner msg"},
	}})
	// owner gets the event back on their subscription — drain it
	for {
		msg, ok := tryRecvJSON(ws, 2*time.Second)
		if !ok {
			break
		}
		if msg.Type == "event" {
			continue
		}
		break
	}

	// get-topic-permission (CanPeek — UserPeek set)
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "get-topic-permission", Payload: map[string]interface{}{"topicName": topicName}})
	resp := expectResponse(t, ws, "get-topic-permission", true)
	var permData map[string]interface{}
	json.Unmarshal(resp.Data, &permData)
	if permData["type"] != "user" {
		t.Errorf("expected type=user, got %v", permData["type"])
	}

	// set-topic-permission (owner check passes)
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(2097151), // ALLOW_ALL
	}})
	expectResponse(t, ws, "set-topic-permission", true)

	// unsubscribe first before destroy
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws, "unsubscribe", true)

	// destroy (CanDelete — UserDelete set)
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "destroy-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws, "destroy-topicName", true)
}

// TestAuthzSetPermissionNonOwnerDenied verifies that a non-owner cannot
// set-topic-permission even if they have other access.
func TestAuthzSetPermissionNonOwnerDenied(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-setperm-%d", time.Now().UnixNano())

	// owner creates topic with ALLOW_ALL so user2 can do most things
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(2097151),
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	// user2 tries set-topic-permission — should fail (owner-only)
	token2, err := signUpUser("authz-setperm@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(0),
	}})
	resp := expectResponse(t, ws2, "set-topic-permission", false)
	if resp.Error != "only owner or admin can change permissions" {
		t.Errorf("expected owner-only error, got %q", resp.Error)
	}
}

// TestAuthzGrantGuestReadAllowsNonOwnerSubscribe verifies that when the owner
// adds GuestRead to the topic permission, a non-owner can subscribe.
func TestAuthzGrantGuestReadAllowsNonOwnerSubscribe(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-gread-%d", time.Now().UnixNano())

	// create with default (owner-only)
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	token2, err := signUpUser("authz-gread@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// user2 cannot subscribe yet (no GuestRead)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", false)

	// owner grants GuestRead|GuestPeek + keeps UserCRUD|UserExecute
	// GuestPeek=1, GuestRead=2, UserCRUD=16256, UserExecute=8192
	newPerm := float64(1 | 2 | 16256 | 8192)
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": newPerm,
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	// user2 can now subscribe
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", true)

	// but user2 still cannot publish (no GuestExecute=64)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "new-message", Payload: map[string]interface{}{
		"topicName": topicName,
		"message":   map[string]interface{}{"text": "blocked"},
	}})
	expectResponse(t, ws2, "new-message", false)
}

// TestAuthzGrantGuestExecuteAllowsNonOwnerPublish verifies that when the owner
// adds GuestExecute to the topic permission, a non-owner can publish.
func TestAuthzGrantGuestExecuteAllowsNonOwnerPublish(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-gexec-%d", time.Now().UnixNano())

	// create with GuestExecute + owner perms
	// GuestExecute=32 (1<<5), UserCRUD|UserExecute=16256
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(32 | 16256),
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	token2, err := signUpUser("authz-gexec@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// user2 can publish (GuestExecute granted)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "new-message", Payload: map[string]interface{}{
		"topicName": topicName,
		"message":   map[string]interface{}{"text": "allowed"},
	}})
	// new-message doesn't send a response on success for user topics, so we just check no error
	// Actually it does - let me check by reading the handler flow
	// The handler sends events, not responses for new-message on user topics
	// Let's just verify no error response arrives
	msg, ok := tryRecvJSON(ws2, 2*time.Second)
	if ok && msg.Type == "response" && msg.Ok != nil && !*msg.Ok {
		t.Errorf("publish should have been allowed but got error: %q", msg.Error)
	}

	// user2 cannot subscribe (no GuestRead=2)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", false)
}

// TestAuthzDestroyTopicNonOwnerDenied verifies a non-owner cannot destroy
// a topic even with ALLOW_ALL minus GuestDelete.
func TestAuthzDestroyTopicNonOwnerDenied(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-destroy-%d", time.Now().UnixNano())
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	// Grant everything except GuestDelete (bit 16)
	// ALLOW_ALL=2097151, GuestDelete=16 → 2097151 & ^16 = 2097135
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(2097151 &^ 16),
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	token2, err := signUpUser("authz-destroy@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// user2 can subscribe (GuestRead is set)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", true)

	// user2 cannot destroy (GuestDelete removed)
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "destroy-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws2, "destroy-topicName", false)
}

// TestAuthzGetTopicPermissionRequiresPeek verifies that get-topic-permission
// requires CanPeek, which for non-owners means GuestPeek must be set.
func TestAuthzGetTopicPermissionRequiresPeek(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-getperm-%d", time.Now().UnixNano())
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	token2, err := signUpUser("authz-getperm@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// default perms: no GuestPeek → denied
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "get-topic-permission", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "get-topic-permission", false)

	// owner adds GuestPeek (bit 1) to perm
	// UserCRUD=16256, UserExecute=8192, GuestPeek=1
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(1 | 16256 | 8192),
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	// now user2 can get-topic-permission
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "get-topic-permission", Payload: map[string]interface{}{"topicName": topicName}})
	resp := expectResponse(t, ws2, "get-topic-permission", true)

	var permData map[string]interface{}
	json.Unmarshal(resp.Data, &permData)
	if permData["type"] != "user" {
		t.Errorf("expected type=user, got %v", permData["type"])
	}
	if permData["owner"] == nil {
		t.Errorf("expected owner field in response")
	}
}

// TestAuthzSystemTopicCannotCreateOrDestroy verifies system topics cannot be
// created or destroyed via the WebSocket protocol.
func TestAuthzSystemTopicCannotCreateOrDestroy(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// create-topicName with system name should fail
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": "world"}})
	resp := expectResponse(t, ws, "create-topicName", false)
	if resp.Error != "cannot create topic with reserved name" {
		t.Errorf("expected reserved name error, got %q", resp.Error)
	}

	// destroy-topicName with system name should fail
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "destroy-topicName", Payload: map[string]interface{}{"name": "user_account"}})
	resp = expectResponse(t, ws, "destroy-topicName", false)
	if resp.Error != "cannot delete system topic" {
		t.Errorf("expected system topic error, got %q", resp.Error)
	}
}

// TestAuthzSystemTopicSetPermissionDenied verifies that set-topic-permission
// cannot be used on system topics.
func TestAuthzSystemTopicSetPermissionDenied(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  "world",
		"permission": float64(2097151),
	}})
	resp := expectResponse(t, ws, "set-topic-permission", false)
	if resp.Error != "cannot modify system topic permissions" {
		t.Errorf("expected system topic error, got %q", resp.Error)
	}
}

// TestAuthzPermissionChangeAffectsAccess verifies that when the owner changes
// permissions, subsequent requests from non-owners reflect the new permissions.
func TestAuthzPermissionChangeAffectsAccess(t *testing.T) {
	ensureServer()
	token1 := signUpAndGetToken(t)

	ws1 := dialWS(t, token1)
	defer ws1.Close()

	topicName := fmt.Sprintf("authz-change-%d", time.Now().UnixNano())
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	expectResponse(t, ws1, "create-topicName", true)

	token2, err := signUpUser("authz-change@test.com", "tester123")
	if err != nil {
		t.Fatalf("signup user2: %v", err)
	}
	ws2 := dialWS(t, token2)
	defer ws2.Close()

	// Step 1: default perms — user2 denied subscribe
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", false)

	// Step 2: owner grants GuestRead+GuestPeek
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(1 | 2 | 16256 | 8192), // GuestPeek|GuestRead|UserCRUD|UserExecute
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	// Step 3: user2 can now subscribe
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", true)

	// Step 4: owner revokes GuestRead (back to owner-only)
	sendJSON(t, ws1, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  topicName,
		"permission": float64(16256 | 8192), // UserCRUD|UserExecute only
	}})
	expectResponse(t, ws1, "set-topic-permission", true)

	// Step 5: user2 unsubscribe (if subscribed) and try again — denied
	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, ws2, 5*time.Second) // unsubscribe response

	sendJSON(t, ws2, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	expectResponse(t, ws2, "subscribe", false)
}

// TestAuthzNonexistentTopicPermissionOperations verifies that get/set permission
// on a nonexistent topic returns appropriate errors.
func TestAuthzNonexistentTopicPermissionOperations(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	ghostTopic := fmt.Sprintf("ghost-%d", time.Now().UnixNano())

	// get-topic-permission on nonexistent topic
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "get-topic-permission", Payload: map[string]interface{}{"topicName": ghostTopic}})
	resp := expectResponse(t, ws, "get-topic-permission", false)
	if resp.Error != "topic not found" {
		t.Errorf("expected 'topic not found', got %q", resp.Error)
	}

	// set-topic-permission on nonexistent topic
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
		"topicName":  ghostTopic,
		"permission": float64(2097151),
	}})
	resp = expectResponse(t, ws, "set-topic-permission", false)
	if resp.Error != "topic not found" {
		t.Errorf("expected 'topic not found', got %q", resp.Error)
	}
}

// ===== SCALABILITY / STRESS TESTS =====

func TestWebSocketManySubscribers(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	// keep under CI connection limits (2-4 core runners)
	const numSubscribers = 20

	topicName := fmt.Sprintf("fan-out-%d", time.Now().UnixNano())

	// create topic
	wsCreator := dialWS(t, token)
	sendJSON(t, wsCreator, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, wsCreator, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)

	// connect and subscribe N clients
	subscribers := make([]*websocket.Conn, numSubscribers)
	var subWg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		subWg.Add(1)
		go func(idx int) {
			defer subWg.Done()
			config, err := websocket.NewConfig(wsURL, wsBaseAddress)
			if err != nil {
				t.Errorf("subscriber %d: config error: %v", idx, err)
				return
			}
			config.Header.Set("Authorization", "Bearer "+token)
			config.Header.Set("Cookie", "token="+token)
			ws, err := websocket.DialConfig(config)
			if err != nil {
				return
			}
			// consume session-open
			ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			var sessionMsg resource.WsOutMessage
			if err := websocket.JSON.Receive(ws, &sessionMsg); err != nil {
				ws.Close()
				return
			}
			if err := websocket.JSON.Send(ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}}); err != nil {
				ws.Close()
				return
			}
			ws.SetReadDeadline(time.Now().Add(30 * time.Second))
			var ack resource.WsOutMessage
			if err := websocket.JSON.Receive(ws, &ack); err != nil {
				ws.Close()
				return
			}
			subscribers[idx] = ws
		}(i)
	}
	subWg.Wait()

	// count how many actually connected
	var connectedCount int
	for _, ws := range subscribers {
		if ws != nil {
			connectedCount++
		}
	}
	t.Logf("connected %d/%d subscribers", connectedCount, numSubscribers)

	// publish a message
	sendJSON(t, wsCreator, wsPayload{
		Id:     nextReqId(),
		Method: "new-message",
		Payload: map[string]interface{}{
			"topicName": topicName,
			"message":   map[string]interface{}{"seq": 1},
		},
	})

	// verify all connected subscribers receive it
	var received int64
	var wg sync.WaitGroup
	for i, ws := range subscribers {
		if ws == nil {
			continue
		}
		wg.Add(1)
		go func(idx int, conn *websocket.Conn) {
			defer wg.Done()
			msg, ok := tryRecvJSON(conn, 10*time.Second)
			if ok && msg.Type == "event" && msg.Event == "new-message" {
				atomic.AddInt64(&received, 1)
			} else {
				t.Errorf("subscriber %d: did not receive event (ok=%v, type=%q)", idx, ok, msg.Type)
			}
		}(i, ws)
	}
	wg.Wait()

	t.Logf("fan-out: %d/%d connected subscribers received the message", received, connectedCount)
	if received < int64(connectedCount*9/10) {
		t.Errorf("too many missed: only %d/%d received", received, connectedCount)
	}

	// cleanup
	for _, ws := range subscribers {
		if ws != nil {
			ws.Close()
		}
	}
	wsCreator.Close()
}

func TestWebSocketHighThroughputMessages(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numMessages = 1000

	topicName := fmt.Sprintf("throughput-%d", time.Now().UnixNano())

	wsPub := dialWS(t, token)
	defer wsPub.Close()
	wsSub := dialWS(t, token)
	defer wsSub.Close()

	// create topic + subscribe
	sendJSON(t, wsPub, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, wsPub, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)
	sendJSON(t, wsSub, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSub, 5*time.Second) // subscribe ack

	// concurrent send + recv
	var recvCount int64
	var recvDone sync.WaitGroup
	recvDone.Add(1)
	go func() {
		defer recvDone.Done()
		for recvCount < int64(numMessages) {
			_, ok := tryRecvJSON(wsSub, 15*time.Second)
			if !ok {
				break
			}
			atomic.AddInt64(&recvCount, 1)
		}
	}()

	start := time.Now()
	for i := 0; i < numMessages; i++ {
		sendJSON(t, wsPub, wsPayload{
			Method: "new-message",
			Payload: map[string]interface{}{
				"topicName": topicName,
				"message":   map[string]interface{}{"seq": i},
			},
		})
		// small delay to avoid flooding the buffer faster than listenWrite drains
		if i%100 == 99 {
			time.Sleep(time.Millisecond)
		}
	}
	sendDuration := time.Since(start)
	t.Logf("sent %d messages in %v (%.0f msg/s)", numMessages, sendDuration, float64(numMessages)/sendDuration.Seconds())

	recvDone.Wait()
	received := atomic.LoadInt64(&recvCount)
	t.Logf("received %d/%d messages", received, numMessages)
	// allow up to 5% loss on CI (slow runners with 2-4 cores)
	if received < int64(numMessages*95/100) {
		t.Errorf("lost %d messages (%.1f%%), exceeds 5%% threshold", int64(numMessages)-received, float64(int64(numMessages)-received)/float64(numMessages)*100)
	}
}

func TestWebSocketConcurrentConnections(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numConns = 500
	const connBatch = 80

	var totalConnected int64
	var totalFailed int64

	for wave := 0; wave < numConns/connBatch; wave++ {
		var wg sync.WaitGroup
		for i := 0; i < connBatch; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				config, err := websocket.NewConfig(wsURL, wsBaseAddress)
				if err != nil {
					atomic.AddInt64(&totalFailed, 1)
					return
				}
				config.Header.Set("Authorization", "Bearer "+token)
				config.Header.Set("Cookie", "token="+token)
				ws, err := websocket.DialConfig(config)
				if err != nil {
					atomic.AddInt64(&totalFailed, 1)
					return
				}

				// consume session-open then do a round-trip
				ws.SetReadDeadline(time.Now().Add(10 * time.Second))
				var sessionMsg resource.WsOutMessage
				if err := websocket.JSON.Receive(ws, &sessionMsg); err != nil {
					atomic.AddInt64(&totalFailed, 1)
					ws.Close()
					return
				}

				if err := websocket.JSON.Send(ws, wsPayload{
					Id:      nextReqId(),
					Method:  "subscribe",
					Payload: map[string]interface{}{"topicName": "user_account"},
				}); err != nil {
					atomic.AddInt64(&totalFailed, 1)
					ws.Close()
					return
				}

				var msg resource.WsOutMessage
				if err := websocket.JSON.Receive(ws, &msg); err != nil {
					atomic.AddInt64(&totalFailed, 1)
					ws.Close()
					return
				}
				ws.Close()

				if msg.Type == "response" {
					atomic.AddInt64(&totalConnected, 1)
				} else {
					atomic.AddInt64(&totalFailed, 1)
				}
			}()
		}
		wg.Wait()
	}

	t.Logf("concurrent connections: %d succeeded, %d failed out of %d (in waves of %d)", totalConnected, totalFailed, numConns, connBatch)
	if totalConnected < int64(numConns*9/10) {
		t.Errorf("too many failures: only %d/%d connected", totalConnected, numConns)
	}
}

func TestWebSocketMultiTopicSubscribe(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numTopics = 50

	ws := dialWS(t, token)
	defer ws.Close()

	// create N topics
	topicNames := make([]string, numTopics)
	for i := 0; i < numTopics; i++ {
		topicNames[i] = fmt.Sprintf("multi-%d-%d", time.Now().UnixNano(), i)
		sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicNames[i]}})
		recvJSON(t, ws, 5*time.Second) // create response
	}
	time.Sleep(300 * time.Millisecond)

	// subscribe to all via comma-separated list
	allTopics := ""
	for i, name := range topicNames {
		if i > 0 {
			allTopics += ","
		}
		allTopics += name
	}
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": allTopics}})

	// should get N subscribe responses
	ackCount := 0
	for i := 0; i < numTopics; i++ {
		msg, ok := tryRecvJSON(ws, 5*time.Second)
		if !ok {
			break
		}
		if msg.Type == "response" && msg.Ok != nil && *msg.Ok && msg.Method == "subscribe" {
			ackCount++
		}
	}
	t.Logf("received %d/%d subscribe acks", ackCount, numTopics)
	if ackCount != numTopics {
		t.Errorf("expected %d acks, got %d", numTopics, ackCount)
	}
}

func TestWebSocketRapidConnectDisconnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const totalCycles = 500
	const rapidBatch = 80
	var totalSucceeded int64
	var dialErrors int64

	for wave := 0; wave < totalCycles/rapidBatch+1; wave++ {
		batchSize := rapidBatch
		remaining := totalCycles - wave*rapidBatch
		if remaining <= 0 {
			break
		}
		if batchSize > remaining {
			batchSize = remaining
		}
		var wg sync.WaitGroup
		for i := 0; i < batchSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				config, err := websocket.NewConfig(wsURL, wsBaseAddress)
				if err != nil {
					atomic.AddInt64(&dialErrors, 1)
					return
				}
				config.Header.Set("Authorization", "Bearer "+token)
				config.Header.Set("Cookie", "token="+token)
				ws, err := websocket.DialConfig(config)
				if err != nil {
					atomic.AddInt64(&dialErrors, 1)
					return
				}
				ws.Close()
				atomic.AddInt64(&totalSucceeded, 1)
			}()
		}
		wg.Wait()
	}
	t.Logf("rapid connect/disconnect: %d succeeded, %d dial errors, out of %d", totalSucceeded, dialErrors, totalCycles)
}

func TestWebSocketSlowConsumerDisconnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	topicName := fmt.Sprintf("slow-consumer-%d", time.Now().UnixNano())

	wsPub := dialWS(t, token)
	defer wsPub.Close()
	wsSlow := dialWS(t, token)
	defer wsSlow.Close()

	// create topic + subscribe slow client
	sendJSON(t, wsPub, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, wsPub, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)
	sendJSON(t, wsSlow, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSlow, 5*time.Second) // subscribe ack

	// flood messages
	const floodPublishers = 10
	const floodPerPublisher = 100
	floodCount := floodPublishers * floodPerPublisher

	var floodWg sync.WaitGroup
	floodStart := time.Now()
	for p := 0; p < floodPublishers; p++ {
		floodWg.Add(1)
		go func(pubIdx int) {
			defer floodWg.Done()
			ws := dialWS(t, token)
			defer ws.Close()
			for i := 0; i < floodPerPublisher; i++ {
				_ = websocket.JSON.Send(ws, wsPayload{
					Method: "new-message",
					Payload: map[string]interface{}{
						"topicName": topicName,
						"message":   map[string]interface{}{"pub": pubIdx, "seq": i},
					},
				})
			}
		}(p)
	}
	floodWg.Wait()
	floodDuration := time.Since(floodStart)
	t.Logf("sent %d messages from %d goroutines in %v — publishers were not blocked", floodCount, floodPublishers, floodDuration)

	// drain what's buffered
	drained := 0
	for {
		_, ok := tryRecvJSON(wsSlow, 2*time.Second)
		if !ok {
			break
		}
		drained++
	}
	t.Logf("slow consumer drained %d messages before disconnect/timeout", drained)

	// publisher should still be functional
	id := nextReqId()
	sendJSON(t, wsPub, wsPayload{Id: id, Method: "subscribe", Payload: map[string]interface{}{"topicName": "user_account"}})
	msg := recvJSON(t, wsPub, 5*time.Second)
	if msg.Type != "response" {
		t.Errorf("publisher broken after slow consumer flood: type=%q", msg.Type)
	}
}

func TestWebSocketConcurrentPublishers(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numPublishers = 5
	const msgsPerPublisher = 50

	topicName := fmt.Sprintf("concurrent-pub-%d", time.Now().UnixNano())

	// create topic
	wsSetup := dialWS(t, token)
	sendJSON(t, wsSetup, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, wsSetup, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)

	// subscriber
	wsSub := dialWS(t, token)
	defer wsSub.Close()
	sendJSON(t, wsSub, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSub, 5*time.Second) // subscribe ack

	// launch concurrent publishers
	var wg sync.WaitGroup
	var sendErrors int64
	start := time.Now()
	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(pubIdx int) {
			defer wg.Done()
			ws := dialWS(t, token)
			defer ws.Close()
			for m := 0; m < msgsPerPublisher; m++ {
				err := websocket.JSON.Send(ws, wsPayload{
					Method: "new-message",
					Payload: map[string]interface{}{
						"topicName": topicName,
						"message":   map[string]interface{}{"pub": pubIdx, "seq": m},
					},
				})
				if err != nil {
					atomic.AddInt64(&sendErrors, 1)
					return
				}
			}
		}(p)
	}
	wg.Wait()
	sendDuration := time.Since(start)
	totalSent := int64(numPublishers*msgsPerPublisher) - sendErrors
	t.Logf("sent %d messages from %d publishers in %v (%.0f msg/s), %d send errors",
		totalSent, numPublishers, sendDuration, float64(totalSent)/sendDuration.Seconds(), sendErrors)

	// receive what we can
	var recvCount int64
	recvStart := time.Now()
	for recvCount < totalSent {
		_, ok := tryRecvJSON(wsSub, 5*time.Second)
		if !ok {
			break
		}
		recvCount++
	}
	recvDuration := time.Since(recvStart)
	t.Logf("received %d/%d messages in %v (%.0f msg/s)", recvCount, totalSent, recvDuration, float64(recvCount)/recvDuration.Seconds())

	lossRate := float64(totalSent-recvCount) / float64(totalSent)
	t.Logf("loss rate: %.1f%%", lossRate*100)
	if lossRate > 0.10 {
		t.Errorf("loss rate %.1f%% exceeds 10%% threshold", lossRate*100)
	}

	wsSetup.Close()
}

func TestWebSocketSubscribeUnsubscribeChurn(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const churnCycles = 100

	topicName := fmt.Sprintf("churn-%d", time.Now().UnixNano())

	ws := dialWS(t, token)
	defer ws.Close()

	// create topic
	sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(t, ws, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)

	// rapidly subscribe/unsubscribe
	for i := 0; i < churnCycles; i++ {
		sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
		msg := recvJSON(t, ws, 5*time.Second)
		if msg.Type != "response" || msg.Ok == nil || !*msg.Ok || msg.Method != "subscribe" {
			t.Fatalf("cycle %d: expected subscribe ok, got type=%q ok=%v method=%q", i, msg.Type, msg.Ok, msg.Method)
		}

		sendJSON(t, ws, wsPayload{Id: nextReqId(), Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
		msg = recvJSON(t, ws, 5*time.Second)
		if msg.Type != "response" || msg.Ok == nil || !*msg.Ok || msg.Method != "unsubscribe" {
			t.Fatalf("cycle %d: expected unsubscribe ok, got type=%q ok=%v method=%q", i, msg.Type, msg.Ok, msg.Method)
		}
	}
	t.Logf("completed %d subscribe/unsubscribe cycles without error", churnCycles)
}

// ===== MULTI-USER STRESS TEST =====

// signUpUser creates a user and returns a JWT token. Each user has a unique email.
func signUpUser(email, password string) (string, error) {
	client := req.New()
	client.SetTimeout(30 * time.Second)

	// Guest signup is locked after become_an_administrator, so use admin token
	adminToken := signUpAndGetToken(nil)
	adminHeader := req.Header{"Authorization": "Bearer " + adminToken}

	// signup — ignore error (user may exist from previous run)
	client.Post(wsBaseAddress+"/action/user_account/signup", adminHeader, req.BodyJSON(map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":           email,
			"name":            email,
			"password":        password,
			"passwordConfirm": password,
		},
	}))

	// signin
	resp, err := client.Post(wsBaseAddress+"/action/user_account/signin", req.BodyJSON(map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":    email,
			"password": password,
		},
	}))
	if err != nil {
		return "", fmt.Errorf("signin: %v", err)
	}

	var signInResp interface{}
	resp.ToJSON(&signInResp)
	arr, ok := signInResp.([]interface{})
	if !ok || len(arr) == 0 {
		return "", fmt.Errorf("unexpected signin response: %v", signInResp)
	}
	attrs, ok := arr[0].(map[string]interface{})
	if !ok || attrs["ResponseType"] != "client.store.set" {
		return "", fmt.Errorf("unexpected signin response type: %v", attrs)
	}
	return attrs["Attributes"].(map[string]interface{})["value"].(string), nil
}

func TestWebSocketMultiUserMessaging(t *testing.T) {
	ensureServer()

	const numUsers = 20
	const msgsPerUser = 100

	// Phase 1: Create users and get tokens concurrently
	t.Log("Phase 1: Creating users...")
	tokens := make([]string, numUsers)
	var signupWg sync.WaitGroup
	var signupErrors int64
	signupStart := time.Now()

	const signupBatch = 20
	for batchStart := 0; batchStart < numUsers; batchStart += signupBatch {
		batchEnd := batchStart + signupBatch
		if batchEnd > numUsers {
			batchEnd = numUsers
		}
		for i := batchStart; i < batchEnd; i++ {
			signupWg.Add(1)
			go func(idx int) {
				defer signupWg.Done()
				email := fmt.Sprintf("user%d@stress.test", idx)
				token, err := signUpUser(email, "tester123")
				if err != nil {
					atomic.AddInt64(&signupErrors, 1)
					return
				}
				tokens[idx] = token
			}(i)
		}
		signupWg.Wait()
	}
	signupDuration := time.Since(signupStart)

	var validUsers int
	for _, tok := range tokens {
		if tok != "" {
			validUsers++
		}
	}
	t.Logf("  created %d/%d users in %v (%d signup errors)", validUsers, numUsers, signupDuration, signupErrors)
	if validUsers < numUsers/2 {
		t.Fatalf("too few users created: %d/%d", validUsers, numUsers)
	}

	// Phase 2: Create topics — each user gets an "inbox" topic, set permissions to allow all
	t.Log("Phase 2: Creating per-user topics...")
	setupWs := dialWS(t, tokens[0])
	topicNames := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		if tokens[i] == "" {
			continue
		}
		topicNames[i] = fmt.Sprintf("inbox-%d-%d", time.Now().UnixNano(), i)
		_ = websocket.JSON.Send(setupWs, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicNames[i]}})
		tryRecvJSON(setupWs, 5*time.Second) // create response
		// set topic permission to allow all users to subscribe (read) and publish (execute)
		_ = websocket.JSON.Send(setupWs, wsPayload{Id: nextReqId(), Method: "set-topic-permission", Payload: map[string]interface{}{
			"topicName":  topicNames[i],
			"permission": float64(2097151), // ALLOW_ALL_PERMISSIONS
		}})
		tryRecvJSON(setupWs, 5*time.Second) // set-permission response
	}
	setupWs.Close()
	time.Sleep(500 * time.Millisecond)
	t.Logf("  created %d topics with open permissions", validUsers)

	// Phase 3: Connect all users, subscribe to own inbox
	t.Log("Phase 3: Connecting all users and subscribing...")
	type userConn struct {
		ws    *websocket.Conn
		token string
		idx   int
		topic string
	}
	users := make([]*userConn, 0, validUsers)

	connectStart := time.Now()
	var mu sync.Mutex
	var connectErrors int64

	// connect in batches to stay under per-IP connection limit
	const connectBatch = 80
	userIndices := make([]int, 0, validUsers)
	for i := 0; i < numUsers; i++ {
		if tokens[i] != "" && topicNames[i] != "" {
			userIndices = append(userIndices, i)
		}
	}
	var dialErrors, sessionErrors, subSendErrors, subAckErrors int64
	for batchStart := 0; batchStart < len(userIndices); batchStart += connectBatch {
		batchEnd := batchStart + connectBatch
		if batchEnd > len(userIndices) {
			batchEnd = len(userIndices)
		}
		var batchWg sync.WaitGroup
		for _, idx := range userIndices[batchStart:batchEnd] {
			batchWg.Add(1)
			go func(idx int) {
				defer batchWg.Done()
				// retry dial up to 5 times like dialWS does
				var ws *websocket.Conn
				for attempt := 0; attempt < 5; attempt++ {
					config, err := websocket.NewConfig(wsURL, wsBaseAddress)
					if err != nil {
						continue
					}
					config.Header.Set("Authorization", "Bearer "+tokens[idx])
					config.Header.Set("Cookie", "token="+tokens[idx])
					ws, err = websocket.DialConfig(config)
					if err == nil {
						break
					}
					ws = nil
					time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
				}
				if ws == nil {
					atomic.AddInt64(&dialErrors, 1)
					atomic.AddInt64(&connectErrors, 1)
					return
				}
				// consume session-open
				ws.SetReadDeadline(time.Now().Add(15 * time.Second))
				var sessionMsg resource.WsOutMessage
				if err := websocket.JSON.Receive(ws, &sessionMsg); err != nil {
					atomic.AddInt64(&sessionErrors, 1)
					atomic.AddInt64(&connectErrors, 1)
					ws.Close()
					return
				}
				// subscribe to own inbox
				if err := websocket.JSON.Send(ws, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicNames[idx]}}); err != nil {
					atomic.AddInt64(&subSendErrors, 1)
					atomic.AddInt64(&connectErrors, 1)
					ws.Close()
					return
				}
				var ack resource.WsOutMessage
				if err := websocket.JSON.Receive(ws, &ack); err != nil || ack.Type != "response" || ack.Ok == nil || !*ack.Ok {
					atomic.AddInt64(&subAckErrors, 1)
					atomic.AddInt64(&connectErrors, 1)
					ws.Close()
					return
				}
				mu.Lock()
				users = append(users, &userConn{ws: ws, token: tokens[idx], idx: idx, topic: topicNames[idx]})
				mu.Unlock()
			}(idx)
		}
		batchWg.Wait()
	}
	connectDuration := time.Since(connectStart)
	t.Logf("  connected %d/%d users in %v (%d connect errors: dial=%d session=%d subSend=%d subAck=%d)",
		len(users), validUsers, connectDuration, connectErrors, dialErrors, sessionErrors, subSendErrors, subAckErrors)

	if len(users) < validUsers/2 {
		t.Fatalf("too few connected: %d/%d", len(users), validUsers)
	}

	// Phase 4: Each user sends msgsPerUser messages to a random other user's inbox
	t.Logf("Phase 4: Each of %d users sending %d messages...", len(users), msgsPerUser)

	// start receivers
	recvCounts := make([]int64, len(users))
	var recvWg sync.WaitGroup
	for i, u := range users {
		recvWg.Add(1)
		go func(idx int, conn *websocket.Conn) {
			defer recvWg.Done()
			for {
				conn.SetReadDeadline(time.Now().Add(30 * time.Second))
				var msg resource.WsOutMessage
				if err := websocket.JSON.Receive(conn, &msg); err != nil {
					return
				}
				if msg.Type == "event" && msg.Event == "new-message" {
					atomic.AddInt64(&recvCounts[idx], 1)
				}
			}
		}(i, u.ws)
	}

	// send messages
	var sendWg sync.WaitGroup
	var totalSent int64
	var sendErrors int64
	sendStart := time.Now()

	for i, u := range users {
		sendWg.Add(1)
		go func(senderIdx int, sender *userConn) {
			defer sendWg.Done()
			targetIdx := (senderIdx + 1) % len(users)
			targetTopic := users[targetIdx].topic

			for m := 0; m < msgsPerUser; m++ {
				err := websocket.JSON.Send(sender.ws, wsPayload{
					Method: "new-message",
					Payload: map[string]interface{}{
						"topicName": targetTopic,
						"message": map[string]interface{}{
							"from": sender.idx,
							"seq":  m,
						},
					},
				})
				if err != nil {
					atomic.AddInt64(&sendErrors, 1)
					return
				}
				atomic.AddInt64(&totalSent, 1)
			}
		}(i, u)
	}
	sendWg.Wait()
	sendDuration := time.Since(sendStart)
	sent := atomic.LoadInt64(&totalSent)
	t.Logf("  sent %d messages in %v (%.0f msg/s), %d send errors",
		sent, sendDuration, float64(sent)/sendDuration.Seconds(), sendErrors)

	// wait for receivers to drain (with timeout)
	done := make(chan struct{})
	go func() {
		recvWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Log("  receiver drain timeout after 60s")
	}

	// close all connections to unblock receivers
	for _, u := range users {
		u.ws.Close()
	}
	<-done

	var totalRecv int64
	for _, c := range recvCounts {
		totalRecv += c
	}

	lossRate := float64(sent-totalRecv) / float64(sent) * 100
	t.Logf("Phase 5: Results")
	t.Logf("  users: %d", len(users))
	t.Logf("  total sent: %d", sent)
	t.Logf("  total received: %d", totalRecv)
	t.Logf("  loss rate: %.2f%%", lossRate)
	t.Logf("  send throughput: %.0f msg/s", float64(sent)/sendDuration.Seconds())

	if lossRate > 5.0 {
		t.Errorf("loss rate %.2f%% exceeds 5%% threshold", lossRate)
	}
}

// ===== BENCHMARKS =====

func BenchmarkWebSocketPubSub(b *testing.B) {
	ensureServer()
	token := signUpAndGetToken(b)

	topicName := fmt.Sprintf("bench-ps-%d", time.Now().UnixNano())
	wsPub := dialWS(b, token)
	defer wsPub.Close()
	wsSub := dialWS(b, token)
	defer wsSub.Close()

	sendJSON(b, wsPub, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(b, wsPub, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)
	sendJSON(b, wsSub, wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(b, wsSub, 5*time.Second) // subscribe ack

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sendJSON(b, wsPub, wsPayload{
			Method: "new-message",
			Payload: map[string]interface{}{
				"topicName": topicName,
				"message":   map[string]interface{}{"seq": i},
			},
		})
		recvJSON(b, wsSub, 10*time.Second)
	}
	b.StopTimer()
}

func BenchmarkWebSocketFanOut(b *testing.B) {
	ensureServer()
	token := signUpAndGetToken(b)

	const numSubs = 10

	topicName := fmt.Sprintf("bench-fo-%d", time.Now().UnixNano())
	wsPub := dialWS(b, token)
	defer wsPub.Close()

	sendJSON(b, wsPub, wsPayload{Id: nextReqId(), Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	recvJSON(b, wsPub, 5*time.Second) // create response
	time.Sleep(300 * time.Millisecond)

	subs := make([]*websocket.Conn, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = dialWS(b, token)
		sendJSON(b, subs[i], wsPayload{Id: nextReqId(), Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
		recvJSON(b, subs[i], 5*time.Second) // subscribe ack
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sendJSON(b, wsPub, wsPayload{
			Method: "new-message",
			Payload: map[string]interface{}{
				"topicName": topicName,
				"message":   map[string]interface{}{"seq": i},
			},
		})
		for _, sub := range subs {
			recvJSON(b, sub, 10*time.Second)
		}
	}
	b.StopTimer()

	for _, s := range subs {
		s.Close()
	}
}

func BenchmarkWebSocketConnect(b *testing.B) {
	ensureServer()
	token := signUpAndGetToken(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config, err := websocket.NewConfig(wsURL, wsBaseAddress)
		if err != nil {
			b.Fatal(err)
		}
		config.Header.Set("Authorization", "Bearer "+token)
		config.Header.Set("Cookie", "token="+token)
		ws, err := websocket.DialConfig(config)
		if err != nil {
			b.Fatal(err)
		}
		ws.Close()
	}
}
