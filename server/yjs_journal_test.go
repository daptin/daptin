package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestYjsReadsCapturedJournals(t *testing.T) {
	// Captured from real JavaScript Yjs peers over the replaced server.
	const updates = "1e00021b0101b96000040104626f64790d53686172656420636166c3a92000140002110101b9600c84b9600b06e69db1e4baac00"
	const request = "0400000100"
	for _, tc := range []string{updates + request, request + updates + request} {
		raw, err := hex.DecodeString(tc)
		if err != nil {
			t.Fatal(err)
		}
		got, err := yjsReplay(raw)
		if err != nil || len(got) != 2 {
			t.Fatalf("replay = %x, %v", got, err)
		}
		encoded := append(yjsJournalRecord(got[0]), yjsJournalRecord(got[1])...)
		if hex.EncodeToString(encoded) != updates {
			t.Fatalf("replay changed frame bytes: %x", encoded)
		}
		if frames, err := yjsReplay(raw[:len(raw)-1]); err == nil || frames != nil {
			t.Fatal("accepted truncated journal")
		}
	}
}

func TestYjsRoomJournalCannotEscapeStorage(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	rooms := filepath.Join(root, "rooms")
	if err := os.Mkdir(rooms, 0700); err != nil {
		t.Fatal(err)
	}
	store := diskYjsStore{dir: rooms}
	frame := yjsJournalRecord([]byte{0, 2, 2, 0, 0})
	for _, name := range []string{"", "..", "../outside", "/outside", "nested\\room"} {
		if _, err := store.read(name); err == nil {
			t.Fatalf("read accepted %q", name)
		}
		if err := store.appendUpdate(context.Background(), name, frame); err == nil {
			t.Fatalf("append accepted %q", name)
		}
	}
	if err := os.Symlink(outside, filepath.Join(rooms, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.read("escape"); err == nil {
		t.Fatal("read followed escaping link")
	}
	if err := store.appendUpdate(context.Background(), "escape", frame); err == nil {
		t.Fatal("append followed escaping link")
	}
	if err := store.appendUpdate(context.Background(), "team.notes", frame); err != nil {
		t.Fatal(err)
	}
	got, err := store.read("team.notes")
	if err != nil || !bytes.Equal(got, frame) {
		t.Fatalf("room history = %x, %v", got, err)
	}
}

func TestYjsFanoutDisconnectsOnlySlowPeer(t *testing.T) {
	sender := &yjsPeer{frames: make(chan []byte, 1), done: make(chan struct{})}
	slow := &yjsPeer{frames: make(chan []byte, 1), done: make(chan struct{})}
	active := &yjsPeer{frames: make(chan []byte, 1), done: make(chan struct{})}
	subscription := &yjsRoomSubscription{peers: map[uint64]*yjsPeer{1: sender, 2: slow, 3: active}}
	b := &olricYjsBroadcaster{nodeID: "local", rooms: map[string]*yjsRoomSubscription{"room": subscription}}
	slow.frames <- []byte{99}
	b.fanout("room", subscription, yjsBroadcastEnvelope{"local", 1, []byte{31}})
	select {
	case <-slow.done:
	default:
		t.Fatal("overloaded peer remained live")
	}
	select {
	case <-sender.frames:
		t.Fatal("message echoed to local sender")
	default:
	}
	select {
	case got := <-active.frames:
		if !bytes.Equal(got, []byte{31}) {
			t.Fatalf("wrong frame: %x", got)
		}
	default:
		t.Fatal("slow peer prevented delivery")
	}
	// The same session number from a different node is not the same peer.
	b.fanout("room", subscription, yjsBroadcastEnvelope{"remote", 1, []byte{42}})
	select {
	case got := <-sender.frames:
		if !bytes.Equal(got, []byte{42}) {
			t.Fatalf("wrong cross-node frame: %x", got)
		}
	default:
		t.Fatal("remote frame suppressed")
	}
}
