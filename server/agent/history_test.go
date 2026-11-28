package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
)

func historyRow(number int, kind, step, source, call string, result interface{}) map[string]interface{} {
	return map[string]interface{}{"number": int64(number), "kind": kind, "reference_id": step,
		"accepted_reference_id": uuid.NewString(), "source_reference_id": source, "call_id": call, "status": "completed", "result": result}
}

func responseWith(message contract.Message) contract.Response {
	return contract.Response{Chat: &contract.ChatResponse{Choices: []contract.ChatChoice{{Message: message, FinishReason: "stop"}}}}
}

func TestTranscriptPreservesToolOrderMetadataAndLargeResults(t *testing.T) {
	step := uuid.NewString()
	message := contract.Message{Role: "assistant", Content: []contract.ContentPart{{Type: "text", Text: "inspect"}}, ToolCalls: []contract.ToolCall{
		{ID: "one", Type: "function", Function: contract.FunctionCall{Name: "lookup", Arguments: `{"key":"a"}`}, ExtraContent: json.RawMessage(`{"google":{"thought_signature":"opaque/+=="}}`)},
		{ID: "two", Type: "function", Function: contract.FunctionCall{Name: "lookup", Arguments: `{"key":"b"}`}},
	}}
	timeline := transcript{}
	if err := timeline.append(historyRow(1, "model", step, "", "", responseWith(message))); err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("result", 1<<20)
	first, _ := encode(map[string]interface{}{"body": large})
	if err := timeline.append(historyRow(2, "tool", uuid.NewString(), step, "one", first)); err != nil {
		t.Fatal(err)
	}
	if err := timeline.append(historyRow(3, "tool", uuid.NewString(), step, "two", `{"approved":true}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := encode(timeline.messages[0])
	want, _ := encode(message)
	if got != want || timeline.messages[1].ToolCallID != "one" || timeline.messages[2].ToolCallID != "two" || len(timeline.pending) != 0 {
		t.Fatalf("conversation order or metadata changed: got %#v want %#v", timeline.messages[0], message)
	}
	var retained map[string]string
	if err := json.Unmarshal([]byte(timeline.messages[1].Content[0].Text), &retained); err != nil || retained["body"] != large {
		t.Fatal("large result changed", err)
	}
}

func TestTranscriptRejectsMissingOrMismatchedAcceptedEvidence(t *testing.T) {
	step := uuid.NewString()
	message := contract.Message{Role: "assistant", ToolCalls: []contract.ToolCall{{ID: "call", Type: "function", Function: contract.FunctionCall{Name: "read", Arguments: "{}"}}}}
	for _, change := range []func(map[string]interface{}){
		func(row map[string]interface{}) { row["accepted_reference_id"] = nil },
		func(row map[string]interface{}) { row["status"] = "failed" },
		func(row map[string]interface{}) { row["number"] = int64(4) },
		func(row map[string]interface{}) { row["source_reference_id"] = uuid.NewString() },
		func(row map[string]interface{}) { row["call_id"] = "other" },
	} {
		timeline := transcript{}
		if err := timeline.append(historyRow(1, "model", step, "", "", responseWith(message))); err != nil {
			t.Fatal(err)
		}
		row := historyRow(2, "tool", uuid.NewString(), step, "call", `{"ok":true}`)
		change(row)
		if timeline.append(row) == nil {
			t.Fatal("incomplete or unrelated evidence entered the conversation")
		}
	}
}

func TestCheckpointSizeDoesNotDependOnConversationOrConfiguration(t *testing.T) {
	state := runState{Accepted: 10, Sequence: 10, Config: Config{Model: "test"}}
	small, _ := encode(state)
	state.Config.Instructions = strings.Repeat("x", 5<<20)
	state.Messages = []contract.Message{{Role: "assistant", Content: []contract.ContentPart{{Type: "text", Text: strings.Repeat("y", 5<<20)}}}}
	large, err := encode(state)
	if err != nil || small != large {
		t.Fatalf("checkpoint grew with history: %d -> %d, %v", len(small), len(large), err)
	}
}

func TestToolResultTextRetainsExactJSONAndIntegerPrecision(t *testing.T) {
	body := `{ "version":9007199254740993,"value":"exact" }`
	actual, err := resultText(body)
	if err != nil || actual != body {
		t.Fatalf("stored result changed: %s %v", actual, err)
	}
}

func BenchmarkAgentCheckpointEncoding(b *testing.B) {
	for _, turns := range []int{10, 100, 200} {
		b.Run(fmt.Sprint(turns), func(b *testing.B) {
			state := runState{Config: Config{Model: "model"}, Accepted: turns, Sequence: turns}
			for i := 0; i < turns; i++ {
				state.Messages = append(state.Messages, contract.Message{Role: "assistant", Content: []contract.ContentPart{{Type: "text", Text: strings.Repeat("x", 64<<10)}}})
			}
			sample, _ := encode(state)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := encode(state); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(sample)), "checkpoint-bytes")
			b.ReportMetric(float64(turns*(64<<10)), "history-bytes")
		})
	}
}
