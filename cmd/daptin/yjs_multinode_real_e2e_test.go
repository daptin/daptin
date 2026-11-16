package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestYjsMultinodeJournalRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") == "" {
		t.Skip("set DAPTIN_E2E_MAIL_POSTGRES_BIN to run a private PostgreSQL cluster")
	}
	options := mailProtocolDatabase(t)
	used := make(map[int]bool)
	portA, httpsA := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
	portB, httpsB := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
	cacheA, cacheB := freeTransportE2EPortPair(t, used), freeTransportE2EPortPair(t, used)
	baseA, baseB := fmt.Sprintf("http://127.0.0.1:%d", portA), fmt.Sprintf("http://127.0.0.1:%d", portB)
	options.olricPort = cacheA
	processA := startTransportE2EDaptin(t, portA, httpsA, baseA, options)
	client := &http.Client{Timeout: 20 * time.Second}
	token := transportE2ESignupSigninAdmin(t, client, baseA)
	options.olricPort, options.olricPeers = cacheB, fmt.Sprintf("127.0.0.1:%d", cacheA+1)
	startTransportE2EDaptin(t, portB, httpsB, baseB, options)
	waitForLLME2ECluster(t, client, baseA, baseB, token)
	doc := accessGroupsE2ECreateRecord(t, client, baseA, token, "document", map[string]interface{}{
		"document_name": "YJS cluster fixture", "document_path": "/", "document_extension": "txt", "mime_type": "text/plain",
		"document_content": []interface{}{},
	})
	room := "/yjs/document." + doc + ".document_content"
	open := func(base string) *websocket.Conn {
		conn := dialYJS(t, "ws"+strings.TrimPrefix(base, "http")+room, token)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	read := func(conn *websocket.Conn, want []byte) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			kind, got, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("YJS peer never received %x: %v", want, err)
			}
			if kind == websocket.BinaryMessage && bytes.Equal(want, got) {
				return
			}
		}
	}
	a, b := open(baseA), open(baseB)
	fixture := [][]string{
		{"0101a19c01000401057369646530046f6e652000", "0101a19c010484a19c01030474776f2000", "0101a19c010884a19c01070674687265652000", "0101a19c010e84a19c010d05666f75722000"},
		{"0101a29c01000401057369646531046f6e652000", "0101a29c010484a29c01030474776f2000", "0101a29c010884a29c01070674687265652000", "0101a29c010e84a29c010d05666f75722000"},
	}
	pending := make(map[string]bool)
	for i := range fixture[0] {
		// Independent Yjs clients update the same resource on different
		// nodes. Both commits must be visible to the other peer and survive.
		frames := make([][]byte, 2)
		for side := range frames {
			payload, err := hex.DecodeString(fixture[side][i])
			if err != nil {
				t.Fatal(err)
			}
			frames[side] = buildYjsUpdate(payload)
			pending[string(frames[side])] = true
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		for side, peer := range []*websocket.Conn{a, b} {
			go func() {
				<-start
				results <- peer.WriteMessage(websocket.BinaryMessage, frames[side])
			}()
		}
		close(start)
		for range frames {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		read(a, frames[1])
		read(b, frames[0])
	}
	awareness := []byte{1, 6, 1, 1, 1, 2, '{', '}'}
	if err := b.WriteMessage(websocket.BinaryMessage, awareness); err != nil {
		t.Fatal(err)
	}
	read(a, awareness)
	_ = a.Close()
	_ = b.Close()
	processA.stopProcess()
	// B must recover every committed update after A exits. Presence
	// belongs only to connected peers and is not part of the replay.
	reopened := open(baseB)
	_ = reopened.SetReadDeadline(time.Now().Add(5 * time.Second))
	for len(pending) > 0 {
		kind, frame, err := reopened.ReadMessage()
		if err != nil {
			t.Fatalf("replay missed %d frames: %v", len(pending), err)
		}
		if kind != websocket.BinaryMessage {
			continue
		}
		if bytes.Equal(frame, awareness) {
			t.Fatal("replayed ephemeral presence")
		}
		delete(pending, string(frame))
	}
}
