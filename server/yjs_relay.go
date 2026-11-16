package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type yjsMessage int

const (
	yjsSyncRequest yjsMessage = iota
	yjsUpdate
	yjsPresence
)

const yjsFrameLimit = 8 << 20

// Inspect only the y-websocket envelope. Clients own the CRDT and its codec;
// the server preserves opaque, complete updates from both sync reply and update.
func classifyYjsFrame(frame []byte) (yjsMessage, error) {
	outer, prefix := binary.Uvarint(frame)
	if prefix <= 0 {
		return 0, errors.New("invalid YJS message type")
	}
	body := frame[prefix:]
	var kind yjsMessage
	switch outer {
	case 0:
		subtype, n := binary.Uvarint(body)
		if n <= 0 || subtype > 2 {
			return 0, errors.New("invalid YJS sync message")
		}
		kind = yjsUpdate
		if subtype == 0 {
			kind = yjsSyncRequest
		}
		body = body[n:]
	case 1:
		kind = yjsPresence
	case 3:
		if len(body) != 0 {
			return 0, errors.New("invalid YJS awareness query")
		}
		return yjsPresence, nil
	default:
		return 0, errors.New("unsupported YJS message type")
	}
	size, n := binary.Uvarint(body)
	if n <= 0 || size != uint64(len(body)-n) {
		return 0, errors.New("invalid YJS message length")
	}
	return kind, nil
}

type yjsRelay struct {
	ctx       context.Context
	cancel    context.CancelFunc
	store     yjsStore
	broadcast *olricYjsBroadcaster
	next      atomic.Uint64
	upgrader  websocket.Upgrader
}

func (r *yjsRelay) serve(w http.ResponseWriter, req *http.Request, room string, readOnly bool) {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	session := r.next.Add(1)
	// Subscribe before reading history to avoid an update gap during reconnect.
	// Yjs safely deduplicates any frame present in both the replay and the queue.
	peer, err := r.broadcast.Subscribe(ctx, room, session)
	if err != nil {
		http.Error(w, "YJS room unavailable", http.StatusServiceUnavailable)
		return
	}
	defer r.broadcast.Unsubscribe(room, session)
	replay, err := prepareYjsRoom(r.store, room)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errInvalidYjsRoom) {
			status = http.StatusBadRequest
		}
		log.WithError(err).Warn("Could not read YJS room")
		http.Error(w, "YJS room unavailable", status)
		return
	}

	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(yjsFrameLimit)
	_ = conn.SetReadDeadline(time.Now().Add(time.Minute))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(time.Minute)) })

	// The reader handles only the client's input. One writer owns all data
	// writes, including the replay, and a closed peer cancels the connection.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer cancel()
		for {
			messageType, frame, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage {
				return
			}
			kind, err := classifyYjsFrame(frame)
			if err != nil {
				return
			}
			if readOnly && kind != yjsPresence {
				continue
			}
			if kind == yjsUpdate {
				if err := r.store.appendUpdate(ctx, room, yjsJournalRecord(frame)); err != nil {
					log.WithError(err).Warn("Could not persist YJS update")
					return
				}
			}
			if err := r.broadcast.Publish(ctx, room, session, frame); err != nil {
				log.WithError(err).Warn("Could not relay YJS message")
				return
			}
		}
	}()
	defer func() {
		cancel()
		_ = conn.Close()
		<-readerDone
	}()

	write := func(op int, data []byte) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-peer.done:
			return fmt.Errorf("YJS peer fell behind or subscription closed")
		default:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			return conn.WriteMessage(op, data)
		}
	}
	for _, frame := range replay {
		if write(websocket.BinaryMessage, frame) != nil {
			return
		}
	}
	// Empty V1 state vector asks an editable client for offline changes even
	// when it reconnects alone. Readers must never be asked to upload state.
	if !readOnly && write(websocket.BinaryMessage, []byte{0, 0, 1, 0}) != nil {
		return
	}
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-peer.done:
			return
		case <-ping.C:
			if write(websocket.PingMessage, nil) != nil {
				return
			}
		case frame := <-peer.frames:
			if write(websocket.BinaryMessage, frame) != nil {
				return
			}
		}
	}
}
