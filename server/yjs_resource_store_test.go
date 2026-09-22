package server

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"testing"
)

func TestYjsStateEntryRequiresAtMostOneStateAsset(t *testing.T) {
	ordinary := map[string]interface{}{"name": "note.txt", "type": "text/plain"}
	state := map[string]interface{}{"name": "content.yjs", "type": yjsStateMediaType}

	entry, err := yjsStateEntry([]map[string]interface{}{ordinary, state})
	if err != nil || !reflect.DeepEqual(entry, state) {
		t.Fatalf("entry=%v err=%v", entry, err)
	}
	if _, err := yjsStateEntry([]map[string]interface{}{state, state}); err == nil {
		t.Fatal("duplicate YJS state assets must be rejected")
	}
}

func TestDecodeYjsStateAcceptsOnlyRawBase64(t *testing.T) {
	want := []byte("framed-yjs-history")
	encoded := base64.StdEncoding.EncodeToString(want)
	got, err := decodeYjsState(encoded)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("decoded=%q err=%v", got, err)
	}
	for _, invalid := range []string{"", "x-crdt/yjs," + encoded, "data:application/octet-stream;base64," + encoded, "@@@"} {
		if _, err := decodeYjsState(invalid); err == nil {
			t.Fatalf("decodeYjsState(%q) succeeded", invalid)
		}
	}
}

func TestWithYjsStatePreservesOrdinaryFilesAndReplacesState(t *testing.T) {
	ordinary := map[string]interface{}{"name": "note.txt", "type": "text/plain"}
	oldState := map[string]interface{}{"name": "old.yjs", "type": yjsStateMediaType, "contents": "b2xk"}
	state := []byte("new")

	updated := withYjsState([]map[string]interface{}{ordinary, oldState}, "content", state)
	if len(updated) != 2 || !reflect.DeepEqual(updated[0], ordinary) {
		t.Fatalf("updated files = %#v", updated)
	}
	entry := updated[1].(map[string]interface{})
	if entry["name"] != "content.yjs" || entry["type"] != yjsStateMediaType || entry["contents"] != base64.StdEncoding.EncodeToString(state) {
		t.Fatalf("state entry = %#v", entry)
	}
}

func TestYjsColumnFilesAcceptsResourceRepresentations(t *testing.T) {
	want := []map[string]interface{}{{"name": "content.yjs", "type": yjsStateMediaType}}
	for _, value := range []interface{}{
		`[{"name":"content.yjs","type":"x-crdt/yjs"}]`,
		want,
		[]interface{}{want[0]},
	} {
		got, err := yjsColumnFiles(value)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("yjsColumnFiles(%T) = %#v, %v", value, got, err)
		}
	}
}
