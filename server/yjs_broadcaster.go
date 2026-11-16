package server

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/buraksezer/olric"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const yjsTopicPrefix = "yjs:"

type yjsBroadcastEnvelope struct {
	NodeID    string `json:"node_id"`
	SessionID uint64 `json:"session_id"`
	Data      []byte `json:"data"`
}

type yjsPeer struct {
	frames chan []byte
	done   chan struct{}
}

func (p *yjsPeer) close() {
	select {
	case <-p.done:
	default:
		close(p.done)
	}
}

type yjsRoomSubscription struct {
	pubsub *redis.PubSub
	peers  map[uint64]*yjsPeer
}

type olricYjsBroadcaster struct {
	ctx    context.Context
	pubsub *olric.PubSub
	nodeID string

	mu     sync.Mutex
	rooms  map[string]*yjsRoomSubscription
	closed bool
}

func newOlricYjsBroadcaster(ctx context.Context, pubsub *olric.PubSub) (*olricYjsBroadcaster, error) {
	if pubsub == nil {
		return nil, fmt.Errorf("YJS requires Olric PubSub")
	}
	return &olricYjsBroadcaster{
		ctx: ctx, pubsub: pubsub, nodeID: uuid.NewString(),
		rooms: make(map[string]*yjsRoomSubscription),
	}, nil
}

func (b *olricYjsBroadcaster) Publish(ctx context.Context, room string, sender uint64, data []byte) error {
	payload, err := stdjson.Marshal(yjsBroadcastEnvelope{NodeID: b.nodeID, SessionID: sender, Data: data})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = b.pubsub.Publish(ctx, yjsTopicPrefix+room, payload)
	return err
}

func (b *olricYjsBroadcaster) Subscribe(ctx context.Context, room string, session uint64) (*yjsPeer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("YJS broadcaster closed")
	}
	current := b.rooms[room]
	if current == nil {
		subscription := b.pubsub.Subscribe(b.ctx, yjsTopicPrefix+room)
		receiveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := subscription.Receive(receiveCtx)
		cancel()
		if err != nil {
			_ = subscription.Close()
			return nil, err
		}
		current = &yjsRoomSubscription{pubsub: subscription, peers: make(map[uint64]*yjsPeer)}
		b.rooms[room] = current
		go b.relay(room, current)
	}
	peer := &yjsPeer{frames: make(chan []byte, 64), done: make(chan struct{})}
	current.peers[session] = peer
	return peer, nil
}

func (b *olricYjsBroadcaster) relay(room string, subscription *yjsRoomSubscription) {
	for message := range subscription.pubsub.Channel() {
		var envelope yjsBroadcastEnvelope
		if err := stdjson.Unmarshal([]byte(message.Payload), &envelope); err != nil {
			log.Warnf("failed to decode YJS broadcast for room %s: %v", room, err)
			continue
		}
		b.fanout(room, subscription, envelope)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.rooms[room] == subscription {
		for _, peer := range subscription.peers {
			peer.close()
		}
		delete(b.rooms, room)
	}
}

func (b *olricYjsBroadcaster) fanout(room string, subscription *yjsRoomSubscription, envelope yjsBroadcastEnvelope) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.rooms[room] != subscription {
		return
	}
	for session, peer := range subscription.peers {
		if envelope.NodeID == b.nodeID && envelope.SessionID == session {
			continue
		}
		select {
		case <-peer.done:
			continue
		default:
		}
		select {
		case peer.frames <- envelope.Data:
		default:
			// A client which falls behind must reconnect and replay from the
			// journal; dropping an arbitrary update would leave it out of sync.
			peer.close()
		}
	}
}

func (b *olricYjsBroadcaster) Unsubscribe(room string, session uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subscription := b.rooms[room]
	if subscription == nil {
		return
	}
	if peer := subscription.peers[session]; peer != nil {
		peer.close()
		delete(subscription.peers, session)
	}
	if len(subscription.peers) == 0 {
		delete(b.rooms, room)
		_ = subscription.pubsub.Close()
	}
}

func (b *olricYjsBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for room, subscription := range b.rooms {
		for _, peer := range subscription.peers {
			peer.close()
		}
		delete(b.rooms, room)
		_ = subscription.pubsub.Close()
	}
}
