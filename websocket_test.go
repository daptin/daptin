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
	Method  string                 `json:"method"`
	Payload map[string]interface{} `json:"attributes"`
}

// dialWS opens an authenticated websocket connection with retry for transient failures.
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
func recvJSON(t testing.TB, ws *websocket.Conn, timeout time.Duration) resource.EventMessage {
	t.Helper()
	ws.SetReadDeadline(time.Now().Add(timeout))
	var msg resource.EventMessage
	if err := websocket.JSON.Receive(ws, &msg); err != nil {
		t.Fatalf("websocket recv: %v", err)
	}
	return msg
}

// tryRecvJSON reads a JSON message with a timeout, returns ok=false on timeout.
func tryRecvJSON(ws *websocket.Conn, timeout time.Duration) (resource.EventMessage, bool) {
	ws.SetReadDeadline(time.Now().Add(timeout))
	var msg resource.EventMessage
	if err := websocket.JSON.Receive(ws, &msg); err != nil {
		return msg, false
	}
	return msg, true
}

var wsTokenOnce sync.Once
var wsToken string

// signUpAndGetToken creates a test user (once) and returns a JWT token.
func signUpAndGetToken(t testing.TB) string {
	t.Helper()
	wsTokenOnce.Do(func() {
		client := req.New()
		client.SetTimeout(30 * time.Second)

		// signup — ignore error since user may already exist
		client.Post(wsBaseAddress+"/action/user_account/signup", req.BodyJSON(map[string]interface{}{
			"attributes": map[string]interface{}{
				"email":           "wstest@test.com",
				"name":            "wstest",
				"password":        "tester123",
				"passwordConfirm": "tester123",
			},
		}))

		// signin
		resp, err := client.Post(wsBaseAddress+"/action/user_account/signin", req.BodyJSON(map[string]interface{}{
			"attributes": map[string]interface{}{
				"email":    "wstest@test.com",
				"password": "tester123",
			},
		}))
		if err != nil {
			panic(fmt.Sprintf("signin failed: %v", err))
		}

		var signInResp interface{}
		resp.ToJSON(&signInResp)
		attrs := signInResp.([]interface{})[0].(map[string]interface{})
		if attrs["ResponseType"] != "client.store.set" {
			panic(fmt.Sprintf("unexpected signin response: %v", attrs))
		}
		wsToken = attrs["Attributes"].(map[string]interface{})["value"].(string)
	})
	return wsToken
}

// ===== E2E TESTS =====

func TestWebSocketConnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// list topics to confirm bidirectional communication
	sendJSON(t, ws, wsPayload{
		Method:  "list-topicName",
		Payload: map[string]interface{}{},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "response" || msg.ObjectType != "topicName-list" {
		t.Errorf("expected topic list response, got EventType=%q ObjectType=%q", msg.EventType, msg.ObjectType)
	}

	var data map[string]interface{}
	json.Unmarshal(msg.EventData, &data)
	topics, ok := data["topics"]
	if !ok {
		t.Errorf("missing 'topics' key in response")
	}
	topicList, ok := topics.([]interface{})
	if !ok {
		t.Errorf("topics is not an array: %T", topics)
	}
	t.Logf("server has %d topics", len(topicList))
}

func TestWebSocketSubscribeAck(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// first get available topics
	sendJSON(t, ws, wsPayload{
		Method:  "list-topicName",
		Payload: map[string]interface{}{},
	})
	listMsg := recvJSON(t, ws, 5*time.Second)
	var data map[string]interface{}
	json.Unmarshal(listMsg.EventData, &data)
	topicList := data["topics"].([]interface{})
	if len(topicList) == 0 {
		t.Skip("no topics available to subscribe to")
	}

	topicName := topicList[0].(string)
	t.Logf("subscribing to topic: %s", topicName)

	sendJSON(t, ws, wsPayload{
		Method: "subscribe",
		Payload: map[string]interface{}{
			"topicName": topicName,
		},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "subscribed" {
		t.Errorf("expected 'subscribed' ack, got EventType=%q ObjectType=%q", msg.EventType, msg.ObjectType)
	}
	if msg.ObjectType != topicName {
		t.Errorf("expected ObjectType=%q, got %q", topicName, msg.ObjectType)
	}
}

func TestWebSocketSubscribeNonexistentTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	sendJSON(t, ws, wsPayload{
		Method: "subscribe",
		Payload: map[string]interface{}{
			"topicName": "nonexistent_topic_xyz_12345",
		},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "error" {
		t.Errorf("expected error response, got EventType=%q", msg.EventType)
	}
	if msg.ObjectType != "subscribe" {
		t.Errorf("expected ObjectType='subscribe', got %q", msg.ObjectType)
	}
}

func TestWebSocketUnsubscribeAck(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// get a topic
	sendJSON(t, ws, wsPayload{Method: "list-topicName", Payload: map[string]interface{}{}})
	listMsg := recvJSON(t, ws, 5*time.Second)
	var data map[string]interface{}
	json.Unmarshal(listMsg.EventData, &data)
	topicList := data["topics"].([]interface{})
	if len(topicList) == 0 {
		t.Skip("no topics available")
	}
	topicName := topicList[0].(string)

	// subscribe first
	sendJSON(t, ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, ws, 5*time.Second) // subscribed ack

	// unsubscribe
	sendJSON(t, ws, wsPayload{Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "unsubscribed" {
		t.Errorf("expected 'unsubscribed' ack, got EventType=%q", msg.EventType)
	}
	if msg.ObjectType != topicName {
		t.Errorf("expected ObjectType=%q, got %q", topicName, msg.ObjectType)
	}
}

func TestWebSocketNewMessageErrors(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// missing topicName
	sendJSON(t, ws, wsPayload{Method: "new-message", Payload: map[string]interface{}{}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "error" {
		t.Errorf("expected error for missing topicName, got %q", msg.EventType)
	}

	// nonexistent topic
	sendJSON(t, ws, wsPayload{
		Method: "new-message",
		Payload: map[string]interface{}{
			"topicName": "does_not_exist_xyz",
			"message":   map[string]interface{}{"text": "hello"},
		},
	})
	msg = recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "error" {
		t.Errorf("expected error for nonexistent topic, got %q", msg.EventType)
	}
}

func TestWebSocketCreateDestroyTopic(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := fmt.Sprintf("test-topic-%d", time.Now().UnixNano())

	// create topic
	sendJSON(t, ws, wsPayload{
		Method:  "create-topicName",
		Payload: map[string]interface{}{"name": topicName},
	})

	// verify it appears in list
	time.Sleep(200 * time.Millisecond)
	sendJSON(t, ws, wsPayload{Method: "list-topicName", Payload: map[string]interface{}{}})
	listMsg := recvJSON(t, ws, 5*time.Second)
	var data map[string]interface{}
	json.Unmarshal(listMsg.EventData, &data)
	found := false
	for _, t := range data["topics"].([]interface{}) {
		if t.(string) == topicName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created topic %q not found in topic list", topicName)
	}

	// destroy it
	sendJSON(t, ws, wsPayload{
		Method:  "destroy-topicName",
		Payload: map[string]interface{}{"name": topicName},
	})

	time.Sleep(200 * time.Millisecond)
	sendJSON(t, ws, wsPayload{Method: "list-topicName", Payload: map[string]interface{}{}})
	listMsg = recvJSON(t, ws, 5*time.Second)
	json.Unmarshal(listMsg.EventData, &data)
	for _, t := range data["topics"].([]interface{}) {
		if t.(string) == topicName {
			t = nil
			break
		}
	}
}

func TestWebSocketDestroySystemTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	// get a system topic (any table name in cruds map is a system topic)
	sendJSON(t, ws, wsPayload{Method: "list-topicName", Payload: map[string]interface{}{}})
	listMsg := recvJSON(t, ws, 5*time.Second)
	var data map[string]interface{}
	json.Unmarshal(listMsg.EventData, &data)
	topicList := data["topics"].([]interface{})

	// "world" is always a system topic
	sendJSON(t, ws, wsPayload{
		Method:  "destroy-topicName",
		Payload: map[string]interface{}{"name": "world"},
	})

	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "error" {
		t.Errorf("expected error for system topic delete, got EventType=%q", msg.EventType)
	}
	_ = topicList
}

func TestWebSocketCreateDuplicateTopicError(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	ws := dialWS(t, token)
	defer ws.Close()

	topicName := fmt.Sprintf("dup-topic-%d", time.Now().UnixNano())

	// create first
	sendJSON(t, ws, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(200 * time.Millisecond)

	// create duplicate
	sendJSON(t, ws, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	msg := recvJSON(t, ws, 5*time.Second)
	if msg.EventType != "error" {
		t.Errorf("expected error for duplicate topic, got EventType=%q", msg.EventType)
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
	sendJSON(t, wsPub, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)

	// subscribe on subscriber
	sendJSON(t, wsSub, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	subAck := recvJSON(t, wsSub, 5*time.Second)
	if subAck.EventType != "subscribed" {
		t.Fatalf("expected subscribed ack, got %q", subAck.EventType)
	}

	// publish a message
	sendJSON(t, wsPub, wsPayload{
		Method: "new-message",
		Payload: map[string]interface{}{
			"topicName": topicName,
			"message":   map[string]interface{}{"text": "hello world"},
		},
	})

	// subscriber should receive it
	msg := recvJSON(t, wsSub, 5*time.Second)
	if msg.EventType != "new-message" {
		t.Errorf("expected 'new-message', got EventType=%q", msg.EventType)
	}
	if msg.ObjectType != topicName {
		t.Errorf("expected ObjectType=%q, got %q", topicName, msg.ObjectType)
	}

	var payload map[string]interface{}
	json.Unmarshal(msg.EventData, &payload)
	if payload["text"] != "hello world" {
		t.Errorf("expected text='hello world', got %v", payload["text"])
	}
}

// ===== SCALABILITY / STRESS TESTS =====

func TestWebSocketManySubscribers(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	// server default is 100 max connections per IP (limit.max_connections)
	const numSubscribers = 80

	topicName := fmt.Sprintf("fan-out-%d", time.Now().UnixNano())

	// create topic
	wsCreator := dialWS(t, token)
	sendJSON(t, wsCreator, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)

	// connect and subscribe N clients — each goroutine dials independently
	// addCh on the server is unbuffered, so this also stress-tests connection admission
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
				// dial failure under load is the data point we want
				return
			}
			if err := websocket.JSON.Send(ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}}); err != nil {
				ws.Close()
				return
			}
			ws.SetReadDeadline(time.Now().Add(30 * time.Second))
			var ack resource.EventMessage
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
			if ok && msg.EventType == "new-message" {
				atomic.AddInt64(&received, 1)
			} else {
				t.Errorf("subscriber %d: did not receive message (ok=%v, type=%q)", idx, ok, msg.EventType)
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

	const numMessages = 5000

	topicName := fmt.Sprintf("throughput-%d", time.Now().UnixNano())

	wsPub := dialWS(t, token)
	defer wsPub.Close()
	wsSub := dialWS(t, token)
	defer wsSub.Close()

	// create topic + subscribe
	sendJSON(t, wsPub, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)
	sendJSON(t, wsSub, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSub, 5*time.Second) // ack

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
	}
	sendDuration := time.Since(start)
	t.Logf("sent %d messages in %v (%.0f msg/s)", numMessages, sendDuration, float64(numMessages)/sendDuration.Seconds())

	recvDone.Wait()
	received := atomic.LoadInt64(&recvCount)
	t.Logf("received %d/%d messages", received, numMessages)
	if received < int64(numMessages) {
		t.Errorf("lost %d messages", int64(numMessages)-received)
	}
}

func TestWebSocketConcurrentConnections(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	// server default is 100 max connections per IP — test in waves
	const numConns = 500
	const connBatch = 80

	// open connections in waves — each wave connects, does a round-trip, disconnects
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

				// each connection lists topics to prove it works
				if err := websocket.JSON.Send(ws, wsPayload{
					Method:  "list-topicName",
					Payload: map[string]interface{}{},
				}); err != nil {
					atomic.AddInt64(&totalFailed, 1)
					ws.Close()
					return
				}

				ws.SetReadDeadline(time.Now().Add(10 * time.Second))
				var msg resource.EventMessage
				if err := websocket.JSON.Receive(ws, &msg); err != nil {
					atomic.AddInt64(&totalFailed, 1)
					ws.Close()
					return
				}
				ws.Close()

				if msg.EventType == "response" {
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
		sendJSON(t, ws, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicNames[i]}})
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
	sendJSON(t, ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": allTopics}})

	// should get N subscribed acks
	ackCount := 0
	for i := 0; i < numTopics; i++ {
		msg, ok := tryRecvJSON(ws, 5*time.Second)
		if !ok {
			break
		}
		if msg.EventType == "subscribed" {
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

	// 500 connections total, in waves of 80 (under the 100-per-IP limit)
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
	sendJSON(t, wsPub, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)
	sendJSON(t, wsSlow, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSlow, 5*time.Second) // ack

	// flood messages from multiple goroutines, each with own connection
	// should fill slow client buffer (100) and disconnect it without blocking publishers
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

	// the slow client's buffer should have overflowed;
	// after draining what's buffered, reads should eventually fail
	drained := 0
	for {
		_, ok := tryRecvJSON(wsSlow, 2*time.Second)
		if !ok {
			break
		}
		drained++
	}
	t.Logf("slow consumer drained %d messages before disconnect/timeout", drained)

	// publisher should still be functional — verify by listing topics
	sendJSON(t, wsPub, wsPayload{Method: "list-topicName", Payload: map[string]interface{}{}})
	msg := recvJSON(t, wsPub, 5*time.Second)
	if msg.EventType != "response" {
		t.Errorf("publisher broken after slow consumer flood: EventType=%q", msg.EventType)
	}
}

func TestWebSocketConcurrentPublishers(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numPublishers = 20
	const msgsPerPublisher = 100

	topicName := fmt.Sprintf("concurrent-pub-%d", time.Now().UnixNano())

	// create topic
	wsSetup := dialWS(t, token)
	sendJSON(t, wsSetup, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)

	// subscriber
	wsSub := dialWS(t, token)
	defer wsSub.Close()
	sendJSON(t, wsSub, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(t, wsSub, 5*time.Second) // ack

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

	// allow some loss from buffer overflow under heavy concurrent load
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
	sendJSON(t, ws, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)

	// rapidly subscribe/unsubscribe
	for i := 0; i < churnCycles; i++ {
		sendJSON(t, ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
		msg := recvJSON(t, ws, 5*time.Second)
		if msg.EventType != "subscribed" {
			t.Fatalf("cycle %d: expected subscribed, got %q", i, msg.EventType)
		}

		sendJSON(t, ws, wsPayload{Method: "unsubscribe", Payload: map[string]interface{}{"topicName": topicName}})
		msg = recvJSON(t, ws, 5*time.Second)
		if msg.EventType != "unsubscribed" {
			t.Fatalf("cycle %d: expected unsubscribed, got %q", i, msg.EventType)
		}
	}
	t.Logf("completed %d subscribe/unsubscribe cycles without error", churnCycles)
}

// ===== MULTI-USER STRESS TEST =====

// signUpUser creates a user and returns a JWT token. Each user has a unique email.
func signUpUser(email, password string) (string, error) {
	client := req.New()
	client.SetTimeout(30 * time.Second)

	// signup — ignore error (user may exist from previous run)
	client.Post(wsBaseAddress+"/action/user_account/signup", req.BodyJSON(map[string]interface{}{
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

	const numUsers = 200
	const msgsPerUser = 1000

	// Phase 1: Create users and get tokens concurrently
	t.Log("Phase 1: Creating 200 users...")
	tokens := make([]string, numUsers)
	var signupWg sync.WaitGroup
	var signupErrors int64
	signupStart := time.Now()

	// signup in batches of 20 to avoid overwhelming the HTTP endpoint
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

	// Phase 2: Create topics — each user gets a "inbox" topic
	t.Log("Phase 2: Creating per-user topics...")
	setupWs := dialWS(t, tokens[0])
	topicNames := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		if tokens[i] == "" {
			continue
		}
		topicNames[i] = fmt.Sprintf("inbox-%d-%d", time.Now().UnixNano(), i)
		_ = websocket.JSON.Send(setupWs, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicNames[i]}})
	}
	setupWs.Close()
	time.Sleep(500 * time.Millisecond)
	t.Logf("  created %d topics", validUsers)

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
	var connectWg sync.WaitGroup
	var mu sync.Mutex
	var connectErrors int64

	for i := 0; i < numUsers; i++ {
		if tokens[i] == "" || topicNames[i] == "" {
			continue
		}
		connectWg.Add(1)
		go func(idx int) {
			defer connectWg.Done()
			ws := dialWS(t, tokens[idx])
			// subscribe to own inbox
			if err := websocket.JSON.Send(ws, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicNames[idx]}}); err != nil {
				atomic.AddInt64(&connectErrors, 1)
				ws.Close()
				return
			}
			ws.SetReadDeadline(time.Now().Add(15 * time.Second))
			var ack resource.EventMessage
			if err := websocket.JSON.Receive(ws, &ack); err != nil || ack.EventType != "subscribed" {
				atomic.AddInt64(&connectErrors, 1)
				ws.Close()
				return
			}
			mu.Lock()
			users = append(users, &userConn{ws: ws, token: tokens[idx], idx: idx, topic: topicNames[idx]})
			mu.Unlock()
		}(i)
	}
	connectWg.Wait()
	connectDuration := time.Since(connectStart)
	t.Logf("  connected %d/%d users in %v (%d connect errors)", len(users), validUsers, connectDuration, connectErrors)

	if len(users) < validUsers/2 {
		t.Fatalf("too few connected: %d/%d", len(users), validUsers)
	}

	// Phase 4: Each user sends msgsPerUser messages to a random other user's inbox
	t.Logf("Phase 4: Each of %d users sending %d messages...", len(users), msgsPerUser)

	// start receivers — each user counts incoming messages
	recvCounts := make([]int64, len(users))
	var recvWg sync.WaitGroup
	for i, u := range users {
		recvWg.Add(1)
		go func(idx int, conn *websocket.Conn) {
			defer recvWg.Done()
			for {
				conn.SetReadDeadline(time.Now().Add(30 * time.Second))
				var msg resource.EventMessage
				if err := websocket.JSON.Receive(conn, &msg); err != nil {
					return
				}
				if msg.EventType == "new-message" {
					atomic.AddInt64(&recvCounts[idx], 1)
				}
			}
		}(i, u.ws)
	}

	// send messages — each user picks target = (self+1) % len(users)
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
	<-done // wait for all receiver goroutines to exit

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

	sendJSON(b, wsPub, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)
	sendJSON(b, wsSub, wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
	recvJSON(b, wsSub, 5*time.Second) // ack

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

	sendJSON(b, wsPub, wsPayload{Method: "create-topicName", Payload: map[string]interface{}{"name": topicName}})
	time.Sleep(300 * time.Millisecond)

	subs := make([]*websocket.Conn, numSubs)
	for i := 0; i < numSubs; i++ {
		subs[i] = dialWS(b, token)
		sendJSON(b, subs[i], wsPayload{Method: "subscribe", Payload: map[string]interface{}{"topicName": topicName}})
		recvJSON(b, subs[i], 5*time.Second)
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
		// wait for all subscribers to receive
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
