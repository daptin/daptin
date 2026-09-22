package server

import (
	"context"
	stdjson "encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/artpar/ydb"
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

type yjsRoomSubscription struct {
	pubsub *redis.PubSub
	refs   int
}

type olricYjsBroadcaster struct {
	ctx    context.Context
	pubsub *olric.PubSub
	nodeID string
	local  ydb.Broadcaster

	mu    sync.Mutex
	rooms map[ydb.YjsRoomName]*yjsRoomSubscription
}

func newOlricYjsBroadcaster(ctx context.Context, pubsub *olric.PubSub) (*olricYjsBroadcaster, error) {
	if pubsub == nil {
		return nil, fmt.Errorf("YJS requires Olric PubSub")
	}
	return &olricYjsBroadcaster{
		ctx:    ctx,
		pubsub: pubsub,
		nodeID: uuid.NewString(),
		local:  ydb.NewLocalBroadcaster(64),
		rooms:  make(map[ydb.YjsRoomName]*yjsRoomSubscription),
	}, nil
}

func (b *olricYjsBroadcaster) Publish(room ydb.YjsRoomName, senderSessionID uint64, data []byte) {
	payload, err := stdjson.Marshal(yjsBroadcastEnvelope{
		NodeID:    b.nodeID,
		SessionID: senderSessionID,
		Data:      data,
	})
	if err != nil {
		log.Errorf("failed to encode YJS broadcast for room %s: %v", room, err)
		return
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	if _, err := b.pubsub.Publish(ctx, yjsTopicPrefix+string(room), payload); err != nil {
		log.Errorf("failed to publish YJS message for room %s: %v", room, err)
	}
}

func (b *olricYjsBroadcaster) Subscribe(room ydb.YjsRoomName, sessionID uint64) (<-chan []byte, error) {
	localChannel, err := b.local.Subscribe(room, sessionID)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	if subscription := b.rooms[room]; subscription != nil {
		subscription.refs++
		b.mu.Unlock()
		return localChannel, nil
	}

	subscription := b.pubsub.Subscribe(b.ctx, yjsTopicPrefix+string(room))
	receiveCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	_, err = subscription.Receive(receiveCtx)
	cancel()
	if err != nil {
		b.mu.Unlock()
		b.local.Unsubscribe(room, sessionID)
		_ = subscription.Close()
		return nil, err
	}
	b.rooms[room] = &yjsRoomSubscription{pubsub: subscription, refs: 1}
	b.mu.Unlock()

	go b.relay(room, subscription)
	return localChannel, nil
}

func (b *olricYjsBroadcaster) relay(room ydb.YjsRoomName, subscription *redis.PubSub) {
	for message := range subscription.Channel() {
		var envelope yjsBroadcastEnvelope
		if err := stdjson.Unmarshal([]byte(message.Payload), &envelope); err != nil {
			log.Warnf("failed to decode YJS broadcast for room %s: %v", room, err)
			continue
		}
		senderSessionID := uint64(0)
		if envelope.NodeID == b.nodeID {
			senderSessionID = envelope.SessionID
		}
		b.local.Publish(room, senderSessionID, envelope.Data)
	}
}

func (b *olricYjsBroadcaster) Unsubscribe(room ydb.YjsRoomName, sessionID uint64) {
	b.local.Unsubscribe(room, sessionID)
	b.mu.Lock()
	defer b.mu.Unlock()
	subscription := b.rooms[room]
	if subscription == nil {
		return
	}
	subscription.refs--
	if subscription.refs == 0 {
		delete(b.rooms, room)
		_ = subscription.pubsub.Close()
	}
}

func (b *olricYjsBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for room, subscription := range b.rooms {
		_ = subscription.pubsub.Close()
		delete(b.rooms, room)
	}
}
