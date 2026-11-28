package agent

import (
	"encoding/json"
	"errors"
	"fmt"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/jmoiron/sqlx"
)

func executionFields(table string) []string {
	fields := []string{"id", "reference_id", "permission", "user_account_id", "version", "created_at", "updated_at"}
	switch table {
	case "agentrun":
		return append(fields, "agent_id", "status", "state", "auth_version", "deadline_at", "claim", "lease_until", "wake_at", "current_step_id", "active_try_id", "source_step_id")
	case "agentstp":
		return append(fields, "agentrun_id", "number", "kind", "operation", "call_id", "source_step_id", "accepted_try_id")
	case "agenttry":
		return append(fields, "agentrun_id", "agentstp_id", "number", "status")
	}
	return fields
}

func refValue(value interface{}) string {
	ref := daptinid.InterfaceToDIR(value)
	if ref == daptinid.NullReferenceId {
		return ""
	}
	return ref.String()
}

func nullableRef(ref string) interface{} {
	if ref == "" {
		return nil
	}
	return ref
}

func readState(row map[string]interface{}, config Config) (runState, error) {
	var state runState
	err := decode(row["state"], &state)
	state.Config = config
	state.Active = refValue(row["active_try_id"])
	state.Current = refValue(row["current_step_id"])
	state.Source = refValue(row["source_step_id"])
	return state, err
}

func (s store) snapshot(ref string, tx *sqlx.Tx) (Config, error) {
	var config Config
	row, err := s.cruds["agentrun"].AgentExecutionRow(daptinid.InterfaceToDIR(ref), append(executionFields("agentrun"), "config"), tx)
	if err != nil {
		return config, err
	}
	err = decode(row["config"], &config)
	return config, err
}

func (s store) result(ref string, tx *sqlx.Tx) (map[string]interface{}, error) {
	return s.cruds["agenttry"].AgentExecutionRow(daptinid.InterfaceToDIR(ref), append(executionFields("agenttry"), "result"), tx)
}

func (s store) stepCalls(run, source string, tx *sqlx.Tx) ([]contract.ToolCall, error) {
	step, err := s.load("agentstp", source, tx)
	if err != nil {
		return nil, err
	}
	if refValue(step["agentrun_id"]) != run || step["kind"] != "model" || refValue(step["accepted_try_id"]) == "" {
		return nil, errors.New("invalid source model step")
	}
	attempt, err := s.result(refValue(step["accepted_try_id"]), tx)
	if err != nil {
		return nil, err
	}
	if attempt["status"] != "completed" || refValue(attempt["agentstp_id"]) != source || refValue(attempt["agentrun_id"]) != run {
		return nil, errors.New("invalid source model attempt")
	}
	message, _, err := modelMessage(attempt["result"])
	return message.ToolCalls, err
}

func (s store) operation(step, attempt map[string]interface{}, tx *sqlx.Tx) (intent, error) {
	number, err := resource.ResourceRowInt64(attempt["number"])
	if err != nil {
		return intent{}, err
	}
	operation := intent{Kind: resource.StringOrEmpty(step["kind"]), Operation: resource.StringOrEmpty(step["operation"]), Step: reference(step), Number: int(number), RequestID: reference(attempt)}
	if refValue(attempt["agentstp_id"]) != operation.Step || refValue(attempt["agentrun_id"]) != refValue(step["agentrun_id"]) {
		return intent{}, errors.New("attempt belongs to a different operation")
	}
	if operation.Kind == "tool" {
		calls, err := s.stepCalls(refValue(step["agentrun_id"]), refValue(step["source_step_id"]), tx)
		if err != nil {
			return intent{}, err
		}
		for _, call := range calls {
			if call.ID == resource.StringOrEmpty(step["call_id"]) {
				operation.Call = call
				return operation, nil
			}
		}
		return intent{}, errors.New("source tool call not found")
	}
	if operation.Kind != "model" {
		return intent{}, errors.New("invalid step kind")
	}
	return operation, nil
}

func modelMessage(result interface{}) (contract.Message, string, error) {
	var response contract.Response
	if err := decode(result, &response); err != nil {
		return contract.Message{}, "", err
	}
	if response.Chat == nil || len(response.Chat.Choices) != 1 {
		return contract.Message{}, "", errors.New("model must return one chat choice")
	}
	choice := response.Chat.Choices[0]
	if choice.Message.Role != "assistant" {
		return contract.Message{}, "", errors.New("model returned invalid message role")
	}
	seen := map[string]bool{}
	for _, call := range choice.Message.ToolCalls {
		if call.ID == "" || call.Type != "function" || seen[call.ID] {
			return contract.Message{}, "", errors.New("invalid or duplicate tool call identity")
		}
		seen[call.ID] = true
	}
	return choice.Message, choice.FinishReason, nil
}

// transcript is an ephemeral request builder. Results remain owned by attempts;
// only explicitly accepted steps contribute to the conversation.
type transcript struct {
	messages []contract.Message
	pending  []contract.ToolCall
	source   string
	sequence int
}

func (t *transcript) append(row map[string]interface{}) error {
	number, err := resource.ResourceRowInt64(row["number"])
	if err != nil || int(number) != t.sequence+1 || refValue(row["accepted_reference_id"]) == "" || row["status"] != "completed" {
		return errors.New("incomplete accepted agent history")
	}
	switch row["kind"] {
	case "model":
		if len(t.pending) != 0 {
			return errors.New("model step precedes pending tool results")
		}
		message, _, err := modelMessage(row["result"])
		if err != nil {
			return err
		}
		t.messages = append(t.messages, message)
		t.pending, t.source = message.ToolCalls, reference(row)
	case "tool":
		if len(t.pending) == 0 || t.pending[0].ID != row["call_id"] || refValue(row["source_reference_id"]) != t.source {
			return errors.New("tool result does not match its source call")
		}
		body, err := resultText(row["result"])
		if err != nil {
			return err
		}
		t.messages = append(t.messages, contract.Message{Role: "tool", ToolCallID: t.pending[0].ID, Content: []contract.ContentPart{{Type: "text", Text: body}}})
		t.pending = t.pending[1:]
	default:
		return errors.New("unknown history step kind")
	}
	t.sequence++
	return nil
}

func (s store) conversation(ref string, state *runState, tx *sqlx.Tx) ([]contract.Message, error) {
	input, err := s.cruds["agentrun"].AgentExecutionRow(daptinid.InterfaceToDIR(ref), append(executionFields("agentrun"), "input"), tx)
	if err != nil {
		return nil, err
	}
	t := transcript{}
	if state.Config.Instructions != "" {
		t.messages = append(t.messages, contract.Message{Role: "system", Content: []contract.ContentPart{{Type: "text", Text: state.Config.Instructions}}})
	}
	var initial []contract.Message
	if err := decode(input["input"], &initial); err != nil {
		return nil, err
	}
	t.messages = append(t.messages, initial...)
	for t.sequence < state.Accepted {
		rows, err := s.cruds["agentrun"].AgentHistoryPage(daptinid.InterfaceToDIR(ref), t.sequence, state.Accepted, tx)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, errors.New("accepted agent history is missing")
		}
		for _, row := range rows {
			if err := t.append(row); err != nil {
				return nil, fmt.Errorf("reconstruct step %d: %w", t.sequence+1, err)
			}
		}
	}
	if len(t.pending) != 0 {
		return nil, errors.New("agent history has unresolved tool calls")
	}
	return t.messages, nil
}

// Tool messages use the stored JSON bytes, retaining large integer values and
// exact result content rather than decoding through floating-point interfaces.
func resultText(value interface{}) (string, error) {
	var body string
	switch v := value.(type) {
	case string:
		body = v
	case []byte:
		body = string(v)
	default:
		var err error
		body, err = encode(v)
		if err != nil {
			return "", err
		}
	}
	if !json.Valid([]byte(body)) {
		return "", errors.New("invalid stored tool result")
	}
	return body, nil
}
