package websockets

import (
	"encoding/json"
	"testing"

	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
)

func TestSystemEventData(t *testing.T) {
	row := map[string]interface{}{"__type": "document", "reference_id": uuid.NewString()}
	for _, kind := range []string{"create", "update", "delete"} {
		t.Run(kind, func(t *testing.T) {
			data, _ := json.Marshal(row)
			event := resource.WsOutMessage{Type: "event", Topic: "document", Event: kind, Source: "database", Data: data}
			if _, ok := systemEventData("document", event); !ok {
				t.Fatal("valid database event rejected")
			}
			for name, change := range map[string]func(*resource.WsOutMessage){
				"wrong envelope type": func(e *resource.WsOutMessage) { e.Type = "response" },
				"wrong topic":         func(e *resource.WsOutMessage) { e.Topic = "user_account" },
				"wrong source":        func(e *resource.WsOutMessage) { e.Source = uuid.NewString() },
				"unknown event":       func(e *resource.WsOutMessage) { e.Event = "unknown" },
			} {
				t.Run(name, func(t *testing.T) {
					invalid := event
					change(&invalid)
					if _, ok := systemEventData("document", invalid); ok {
						t.Fatal("invalid database event accepted")
					}
				})
			}
			for _, payload := range []string{
				`{`, `null`, `[]`, `{}`, `{"reference_id":"` + row["reference_id"].(string) + `"}`,
				`{"__type":"user_account","reference_id":"` + row["reference_id"].(string) + `"}`,
				`{"__type":{},"reference_id":"` + row["reference_id"].(string) + `"}`,
				`{"__type":"document"}`, `{"__type":"document","reference_id":"invalid"}`,
				`{"__type":"document","reference_id":"00000000-0000-0000-0000-000000000000"}`,
			} {
				invalid := event
				invalid.Data = []byte(payload)
				if _, ok := systemEventData("document", invalid); ok {
					t.Fatalf("invalid row accepted: %s", payload)
				}
			}
		})
	}
}

func TestSystemCustomMessagesDoNotSelectRowAuthorization(t *testing.T) {
	for _, payload := range []string{`{"__type":"user_account","permission":2097151}`, `[1,2]`, `"hello"`, `null`} {
		event := resource.WsOutMessage{Type: "event", Topic: "document", Event: "new-message", Source: uuid.NewString(), Data: []byte(payload)}
		if _, ok := systemEventData("document", event); !ok {
			t.Fatalf("custom message rejected: %s", payload)
		}
		event.Source = "database"
		if _, ok := systemEventData("document", event); ok {
			t.Fatal("database event disguised as custom message accepted")
		}
	}
}
