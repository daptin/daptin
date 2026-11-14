package resource

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWsOutMessageRejectsInvalidLengths(t *testing.T) {
	message := WsOutMessage{Type: "event", Topic: "document", Event: "update", Source: "database", Data: []byte(`{"reference_id":"test"}`)}
	encoded, err := message.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var decoded WsOutMessage
	if err := decoded.UnmarshalBinary(encoded); err != nil || !bytes.Equal(decoded.Data, message.Data) {
		t.Fatalf("round trip failed: %v", err)
	}
	for _, size := range []int32{-1, 1 << 30} {
		var invalid bytes.Buffer
		if err := binary.Write(&invalid, binary.BigEndian, size); err != nil {
			t.Fatal(err)
		}
		if err := decoded.UnmarshalBinary(invalid.Bytes()); err == nil {
			t.Fatalf("accepted invalid length %d", size)
		}
	}
	for n := 0; n < len(encoded); n++ {
		if err := decoded.UnmarshalBinary(encoded[:n]); err == nil {
			t.Fatalf("accepted truncated event of length %d", n)
		}
	}
}
