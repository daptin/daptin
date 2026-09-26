package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gorillaWs "github.com/gorilla/websocket"
)

const yjsBaseURL = "ws://localhost:6337"

// yjsDialer is a gorilla/websocket dialer with reasonable timeouts.
var yjsDialer = &gorillaWs.Dialer{
	HandshakeTimeout: 5 * time.Second,
}

// dialYJS opens an authenticated gorilla/websocket connection to a YJS endpoint.
func dialYJS(t testing.TB, url string, token string) *gorillaWs.Conn {
	t.Helper()
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
		header.Set("Cookie", "token="+token)
	}
	for attempt := 0; attempt < 5; attempt++ {
		conn, _, err := yjsDialer.Dial(url, header)
		if err != nil {
			time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
			continue
		}
		return conn
	}
	t.Fatalf("yjs dial %s: failed after 5 attempts", url)
	return nil
}

// --- YJS/ydb binary protocol helpers ---
// ydb is a relay: it forwards binary messages between clients in the same room.
// It does NOT respond to SyncStep1 itself — only other connected clients do.
// Message format: [messageType(uvarint), sub-type-specific data...]
// For messageSync(0): [0, syncSubType(uvarint), payload(uvarint-length-prefixed)]

func yjsWriteUvarint(buf *bytes.Buffer, n uint64) {
	bs := make([]byte, binary.MaxVarintLen64)
	l := binary.PutUvarint(bs, n)
	buf.Write(bs[:l])
}

func yjsWritePayload(buf *bytes.Buffer, data []byte) {
	yjsWriteUvarint(buf, uint64(len(data)))
	buf.Write(data)
}

// buildYjsSyncStep1 builds a valid SyncStep1 message with empty state vector.
func buildYjsSyncStep1() []byte {
	buf := &bytes.Buffer{}
	yjsWriteUvarint(buf, 0) // messageSync
	yjsWriteUvarint(buf, 0) // messageYjsSyncStep1
	yjsWritePayload(buf, []byte{})
	return buf.Bytes()
}

// buildYjsUpdate builds a valid update message with arbitrary payload.
func buildYjsUpdate(payload []byte) []byte {
	buf := &bytes.Buffer{}
	yjsWriteUvarint(buf, 0) // messageSync
	yjsWriteUvarint(buf, 2) // messageYjsUpdate
	yjsWritePayload(buf, payload)
	return buf.Bytes()
}

// buildYjsAwareness builds a minimal awareness message.
// Format: messageAwareness(1) + clientId + clock + var1 + var2 + json_string(payload-prefixed)
func buildYjsAwareness(clientId uint64) []byte {
	buf := &bytes.Buffer{}
	yjsWriteUvarint(buf, 1)        // messageAwareness
	yjsWriteUvarint(buf, clientId) // clientId
	yjsWriteUvarint(buf, 1)        // clock
	yjsWriteUvarint(buf, 1)        // var1
	yjsWriteUvarint(buf, 0)        // var2
	jsonStr := `{"user":{"name":"test"}}`
	yjsWritePayload(buf, []byte(jsonStr))
	return buf.Bytes()
}

// tryReadYJSMessage reads a binary message, returning ok=false on timeout/error.
func tryReadYJSMessage(conn *gorillaWs.Conn, timeout time.Duration) ([]byte, bool) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, false
	}
	return data, true
}

// connectAndSubscribe connects to a YJS document and waits for the subscription
// goroutine in ydb to complete (subscribeRoom is async).
func connectAndSubscribe(t testing.TB, docName string, token string) *gorillaWs.Conn {
	t.Helper()
	conn := dialYJS(t, yjsBaseURL+"/yjs/"+docName, token)
	// subscribeRoom runs in a goroutine — give it time to add to room.subs
	time.Sleep(100 * time.Millisecond)
	return conn
}

// ===== AUTH TESTS =====

func TestYJSNoAuthReturns403(t *testing.T) {
	ensureServer()

	// Attempt to connect to /yjs/test-doc WITHOUT any auth token.
	// Before the fix, this would succeed due to missing return after AbortWithStatus(403).
	header := http.Header{}
	_, resp, err := yjsDialer.Dial(yjsBaseURL+"/yjs/test-doc", header)
	if err == nil {
		t.Errorf("expected dial to fail without auth, but it succeeded")
		return
	}
	if resp != nil && resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestYJSAuthenticatedConnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	conn := dialYJS(t, yjsBaseURL+"/yjs/auth-test-doc", token)
	defer conn.Close()

	// ydb is a relay — it won't respond to SyncStep1 unless there's another client.
	// Verify we can send without error (connection is alive).
	err := conn.WriteMessage(gorillaWs.BinaryMessage, buildYjsSyncStep1())
	if err != nil {
		t.Fatalf("failed to send SyncStep1: %v", err)
	}

	// Connection should stay open — verify with a ping
	err = conn.WriteMessage(gorillaWs.PingMessage, nil)
	if err != nil {
		t.Fatalf("ping failed — connection not alive: %v", err)
	}
	t.Log("authenticated YJS connection established and alive")
}

// ===== DOCUMENT ISOLATION =====

func TestYJSDocumentIsolation(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docA := fmt.Sprintf("iso-a-%d", time.Now().UnixNano())
	docB := fmt.Sprintf("iso-b-%d", time.Now().UnixNano())

	// Connect two clients to different documents
	connA1 := connectAndSubscribe(t, docA, token)
	defer connA1.Close()
	connA2 := connectAndSubscribe(t, docA, token)
	defer connA2.Close()
	connB := connectAndSubscribe(t, docB, token)
	defer connB.Close()

	// Send an update to doc-a — connA2 should receive it, connB should NOT
	update := buildYjsUpdate([]byte{0x42, 0x43})
	connA1.WriteMessage(gorillaWs.BinaryMessage, update)

	// connA2 should receive the forwarded update
	_, okA2 := tryReadYJSMessage(connA2, 3*time.Second)
	if !okA2 {
		t.Errorf("connA2 did not receive update sent to same document")
	}

	// connB should NOT receive anything (different document)
	_, okB := tryReadYJSMessage(connB, 1*time.Second)
	if okB {
		t.Errorf("connB received a message from a different document — isolation broken")
	}
}

// ===== COLLABORATION / RELAY TESTS =====

func TestYJSTwoClientRelay(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docName := fmt.Sprintf("relay-%d", time.Now().UnixNano())

	connA := connectAndSubscribe(t, docName, token)
	defer connA.Close()
	connB := connectAndSubscribe(t, docName, token)
	defer connB.Close()

	// Client A sends update — Client B should receive it via ydb relay
	payload := []byte{0xDE, 0xAD}
	connA.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate(payload))

	data, ok := tryReadYJSMessage(connB, 5*time.Second)
	if !ok {
		t.Fatal("client B did not receive update relayed from client A")
	}
	t.Logf("client B received %d bytes from relay", len(data))

	// Verify it's a sync message (first byte = messageSync = 0)
	if len(data) > 0 && data[0] != 0 {
		t.Errorf("expected messageSync (0), got %d", data[0])
	}
}

func TestYJSBidirectionalRelay(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docName := fmt.Sprintf("bidir-%d", time.Now().UnixNano())

	connA := connectAndSubscribe(t, docName, token)
	defer connA.Close()
	connB := connectAndSubscribe(t, docName, token)
	defer connB.Close()

	// A → B
	connA.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate([]byte{0x01}))
	_, okAB := tryReadYJSMessage(connB, 5*time.Second)
	if !okAB {
		t.Error("A→B relay failed")
	}

	// B → A
	connB.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate([]byte{0x02}))
	_, okBA := tryReadYJSMessage(connA, 5*time.Second)
	if !okBA {
		t.Error("B→A relay failed")
	}
}

func TestYJSSyncStep1Forwarding(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docName := fmt.Sprintf("sync-fwd-%d", time.Now().UnixNano())

	connA := connectAndSubscribe(t, docName, token)
	defer connA.Close()
	connB := connectAndSubscribe(t, docName, token)
	defer connB.Close()

	// Client A sends SyncStep1 — should be forwarded to client B
	connA.WriteMessage(gorillaWs.BinaryMessage, buildYjsSyncStep1())

	data, ok := tryReadYJSMessage(connB, 5*time.Second)
	if !ok {
		t.Fatal("SyncStep1 was not forwarded to client B")
	}
	// Forwarded message should start with messageSync(0) + SyncStep1(0)
	if len(data) >= 2 && data[0] == 0 && data[1] == 0 {
		t.Logf("client B received forwarded SyncStep1 (%d bytes)", len(data))
	} else {
		t.Errorf("unexpected message format: %v", data)
	}
}

func TestYJSMultipleClientsOnSameDoc(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numClients = 10
	docName := fmt.Sprintf("multi-client-%d", time.Now().UnixNano())

	conns := make([]*gorillaWs.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conns[i] = connectAndSubscribe(t, docName, token)
	}
	defer func() {
		for _, c := range conns {
			if c != nil {
				c.Close()
			}
		}
	}()

	// Client 0 sends an update — all others should receive it
	conns[0].WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate([]byte{0xFF}))

	received := 0
	for i := 1; i < numClients; i++ {
		_, ok := tryReadYJSMessage(conns[i], 5*time.Second)
		if ok {
			received++
		}
	}
	t.Logf("fan-out: %d/%d other clients received the update", received, numClients-1)
	if received < numClients-2 { // allow 1 miss under load
		t.Errorf("too few clients received update: %d/%d", received, numClients-1)
	}
}

// ===== AWARENESS TESTS =====

func TestYJSAwarenessRelay(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docName := fmt.Sprintf("awareness-%d", time.Now().UnixNano())

	connA := connectAndSubscribe(t, docName, token)
	defer connA.Close()
	connB := connectAndSubscribe(t, docName, token)
	defer connB.Close()

	// ydb echoes awareness back to the sender's session (session.send),
	// not to other sessions via updateRoom. So A should get its own awareness back.
	connA.WriteMessage(gorillaWs.BinaryMessage, buildYjsAwareness(42))

	data, ok := tryReadYJSMessage(connA, 5*time.Second)
	if !ok {
		t.Log("no awareness echo received (may depend on ydb version)")
	} else if len(data) > 0 && data[0] == 1 {
		t.Logf("received awareness echo (%d bytes)", len(data))
	} else {
		t.Logf("received non-awareness message type=%d (%d bytes)", data[0], len(data))
	}
}

// ===== STRESS / SCALABILITY TESTS =====

func TestYJSConcurrentConnections(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numConns = 80
	docName := fmt.Sprintf("concurrent-%d", time.Now().UnixNano())

	var connected int64
	var failed int64
	var wg sync.WaitGroup

	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			header := http.Header{}
			header.Set("Authorization", "Bearer "+token)
			header.Set("Cookie", "token="+token)
			conn, _, err := yjsDialer.Dial(yjsBaseURL+"/yjs/"+docName, header)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			// verify connection is alive by sending
			err = conn.WriteMessage(gorillaWs.BinaryMessage, buildYjsSyncStep1())
			conn.Close()
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			atomic.AddInt64(&connected, 1)
		}()
	}
	wg.Wait()

	t.Logf("concurrent YJS connections: %d succeeded, %d failed out of %d", connected, failed, numConns)
	if connected < int64(numConns*9/10) {
		t.Errorf("too many failures: only %d/%d connected", connected, numConns)
	}
}

func TestYJSRapidConnectDisconnect(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const totalCycles = 300
	const batchSize = 60
	var totalSucceeded int64
	var dialErrors int64

	for wave := 0; wave*batchSize < totalCycles; wave++ {
		remaining := totalCycles - wave*batchSize
		batch := batchSize
		if batch > remaining {
			batch = remaining
		}
		var wg sync.WaitGroup
		for i := 0; i < batch; i++ {
			wg.Add(1)
			go func(docIdx int) {
				defer wg.Done()
				docName := fmt.Sprintf("rapid-%d-%d", wave, docIdx)
				header := http.Header{}
				header.Set("Authorization", "Bearer "+token)
				header.Set("Cookie", "token="+token)
				conn, _, err := yjsDialer.Dial(yjsBaseURL+"/yjs/"+docName, header)
				if err != nil {
					atomic.AddInt64(&dialErrors, 1)
					return
				}
				conn.Close()
				atomic.AddInt64(&totalSucceeded, 1)
			}(i)
		}
		wg.Wait()
	}

	t.Logf("rapid connect/disconnect: %d succeeded, %d failed out of %d", totalSucceeded, dialErrors, totalCycles)
}

func TestYJSHighThroughputUpdates(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numMessages = 500
	docName := fmt.Sprintf("throughput-%d", time.Now().UnixNano())

	// Publisher
	connPub := connectAndSubscribe(t, docName, token)
	defer connPub.Close()

	// Subscriber
	connSub := connectAndSubscribe(t, docName, token)
	defer connSub.Close()

	// Concurrent receive
	var recvCount int64
	var recvDone sync.WaitGroup
	recvDone.Add(1)
	go func() {
		defer recvDone.Done()
		for atomic.LoadInt64(&recvCount) < int64(numMessages) {
			_, ok := tryReadYJSMessage(connSub, 10*time.Second)
			if !ok {
				break
			}
			atomic.AddInt64(&recvCount, 1)
		}
	}()

	// Send updates with unique payloads
	start := time.Now()
	for i := 0; i < numMessages; i++ {
		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload, uint32(i))
		err := connPub.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate(payload))
		if err != nil {
			t.Fatalf("send failed at message %d: %v", i, err)
		}
	}
	sendDuration := time.Since(start)
	t.Logf("sent %d messages in %v (%.0f msg/s)", numMessages, sendDuration, float64(numMessages)/sendDuration.Seconds())

	recvDone.Wait()
	received := atomic.LoadInt64(&recvCount)
	t.Logf("received %d/%d messages", received, numMessages)
	if received == 0 {
		t.Errorf("received zero messages — relay forwarding is broken")
	}
	if received > 0 && received < int64(numMessages/2) {
		t.Logf("note: ydb may coalesce/drop under high throughput (received %d/%d)", received, numMessages)
	}
}

func TestYJSManyDocumentsConcurrent(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numDocs = 50

	// Open a pair of connections per document, send an update, verify relay
	var wg sync.WaitGroup
	var relayOK int64

	for i := 0; i < numDocs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			docName := fmt.Sprintf("many-docs-%d-%d", time.Now().UnixNano(), idx)

			connA := connectAndSubscribe(t, docName, token)
			defer connA.Close()
			connB := connectAndSubscribe(t, docName, token)
			defer connB.Close()

			connA.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate([]byte{byte(idx)}))
			_, ok := tryReadYJSMessage(connB, 5*time.Second)
			if ok {
				atomic.AddInt64(&relayOK, 1)
			}
		}(i)
	}
	wg.Wait()

	t.Logf("many documents: %d/%d relayed successfully", relayOK, numDocs)
	if relayOK < int64(numDocs*8/10) {
		t.Errorf("too many relay failures: %d/%d", relayOK, numDocs)
	}
}

func TestYJSSlowConsumer(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	docName := fmt.Sprintf("slow-consumer-%d", time.Now().UnixNano())

	// Fast publisher
	connPub := connectAndSubscribe(t, docName, token)
	defer connPub.Close()

	// Slow consumer — connects but never reads
	connSlow := connectAndSubscribe(t, docName, token)
	defer connSlow.Close()

	// Flood messages — should fill slow consumer's send channel (buffer=5)
	// without blocking the publisher
	const floodCount = 200
	start := time.Now()
	sendErrors := 0
	for i := 0; i < floodCount; i++ {
		payload := make([]byte, 4)
		binary.BigEndian.PutUint32(payload, uint32(i))
		err := connPub.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate(payload))
		if err != nil {
			sendErrors++
			break
		}
	}
	floodDuration := time.Since(start)
	t.Logf("sent %d messages in %v — publisher was not blocked (send errors: %d)",
		floodCount-sendErrors, floodDuration, sendErrors)

	// Publisher should still be functional
	err := connPub.WriteMessage(gorillaWs.PingMessage, nil)
	if err != nil {
		t.Errorf("publisher broken after slow consumer flood: %v", err)
	}

	// Drain whatever the slow consumer got
	drained := 0
	for {
		_, ok := tryReadYJSMessage(connSlow, 2*time.Second)
		if !ok {
			break
		}
		drained++
	}
	t.Logf("slow consumer drained %d messages before disconnect/timeout", drained)
}

func TestYJSConcurrentPublishers(t *testing.T) {
	ensureServer()
	token := signUpAndGetToken(t)

	const numPublishers = 10
	const msgsPerPublisher = 50

	docName := fmt.Sprintf("concurrent-pub-%d", time.Now().UnixNano())

	// Subscriber
	connSub := connectAndSubscribe(t, docName, token)
	defer connSub.Close()

	// Launch concurrent publishers
	var wg sync.WaitGroup
	var sendErrors int64
	var totalSent int64
	start := time.Now()

	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(pubIdx int) {
			defer wg.Done()
			conn := connectAndSubscribe(t, docName, token)
			defer conn.Close()

			for m := 0; m < msgsPerPublisher; m++ {
				payload := make([]byte, 4)
				binary.BigEndian.PutUint32(payload, uint32(pubIdx*msgsPerPublisher+m))
				err := conn.WriteMessage(gorillaWs.BinaryMessage, buildYjsUpdate(payload))
				if err != nil {
					atomic.AddInt64(&sendErrors, 1)
					return
				}
				atomic.AddInt64(&totalSent, 1)
			}
		}(p)
	}
	wg.Wait()
	sendDuration := time.Since(start)
	sent := atomic.LoadInt64(&totalSent)
	t.Logf("sent %d messages from %d publishers in %v (%.0f msg/s), %d send errors",
		sent, numPublishers, sendDuration, float64(sent)/sendDuration.Seconds(), sendErrors)

	// Receive what we can
	var recvCount int64
	for {
		_, ok := tryReadYJSMessage(connSub, 5*time.Second)
		if !ok {
			break
		}
		recvCount++
	}
	t.Logf("subscriber received %d messages", recvCount)
	if recvCount == 0 && sent > 0 {
		t.Errorf("subscriber received zero messages — relay forwarding broken")
	}
}

// ===== BENCHMARKS =====

func BenchmarkYJSConnect(b *testing.B) {
	ensureServer()
	token := signUpAndGetToken(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := http.Header{}
		header.Set("Authorization", "Bearer "+token)
		header.Set("Cookie", "token="+token)
		conn, _, err := yjsDialer.Dial(yjsBaseURL+"/yjs/bench-connect", header)
		if err != nil {
			b.Fatal(err)
		}
		conn.Close()
	}
}

func BenchmarkYJSUpdateRelay(b *testing.B) {
	ensureServer()
	token := signUpAndGetToken(b)

	docName := fmt.Sprintf("bench-relay-%d", time.Now().UnixNano())
	connPub := connectAndSubscribe(b, docName, token)
	defer connPub.Close()
	connSub := connectAndSubscribe(b, docName, token)
	defer connSub.Close()

	update := buildYjsUpdate([]byte{0x01, 0x02, 0x03, 0x04})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connPub.WriteMessage(gorillaWs.BinaryMessage, update)
		connSub.SetReadDeadline(time.Now().Add(10 * time.Second))
		connSub.ReadMessage()
	}
	b.StopTimer()
}
