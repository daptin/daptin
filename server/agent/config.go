// Package agent composes durable runs from Daptin resources, actions and its LLM gateway.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/daptin/llmgateway/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	leaseDuration     = 30 * time.Second
	heartbeatInterval = 5 * time.Second
	maxPayload        = 1 << 20
	maxHistory        = 4 << 20
)

type Limits struct {
	Turns        int   `json:"turns"`
	ToolCalls    int   `json:"tool_calls"`
	Seconds      int   `json:"seconds"`
	OutputTokens int64 `json:"output_tokens"`
	Attempts     int   `json:"attempts"`
}
type Binding struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Entity      string          `json:"entity"`
	Action      string          `json:"action"`
	Parameters  json.RawMessage `json:"parameters"`
	// A maintainer promises that this action implements repeatable reads or
	// durable deduplication using the supplied logical operation reference.
	Repeatable       bool   `json:"repeatable"`
	IdempotencyInput string `json:"idempotency_input,omitempty"`
}
type Toolset struct {
	Resources  []string  `json:"resources"`
	Operations []string  `json:"operations"`
	Actions    []string  `json:"actions"`
	Tools      []Binding `json:"tools"`
}
type Config struct {
	Instructions string  `json:"instructions"`
	Model        string  `json:"model"`
	Toolset      Toolset `json:"toolset"`
	Limits       Limits  `json:"limits"`
}
type runState struct {
	Config   Config              `json:"config"`
	Messages []contract.Message  `json:"messages"`
	Started  time.Time           `json:"started"`
	Turns    int                 `json:"turns"`
	Calls    int                 `json:"calls"`
	Sequence int                 `json:"sequence"`
	Active   string              `json:"active,omitempty"`
	Pending  []contract.ToolCall `json:"pending,omitempty"`
	Error    string              `json:"error,omitempty"`
	Result   string              `json:"result,omitempty"`
}
type intent struct {
	Kind      string            `json:"kind"`
	Operation string            `json:"operation"`
	Step      string            `json:"step"`
	Call      contract.ToolCall `json:"call,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Number    int               `json:"number"`
}

func decode(value interface{}, target interface{}) error {
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}
	// Terminal metadata may add a small error envelope to an already full history.
	if len(data) > maxHistory+1024 {
		return errors.New("agent payload exceeds history limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return errors.New("expected a single JSON value")
	}
	return nil
}
func encode(value interface{}) string { data, _ := json.Marshal(value); return string(data) }
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == "*" || v == value {
			return true
		}
	}
	return false
}
func (c *Config) validate() error {
	if c.Model == "" {
		return errors.New("agent model is required")
	}
	if c.Limits.Turns == 0 {
		c.Limits.Turns = 20
	}
	if c.Limits.ToolCalls == 0 {
		c.Limits.ToolCalls = 50
	}
	if c.Limits.Seconds == 0 {
		c.Limits.Seconds = 900
	}
	if c.Limits.OutputTokens == 0 {
		c.Limits.OutputTokens = 2048
	}
	if c.Limits.Attempts == 0 {
		c.Limits.Attempts = 3
	}
	if c.Limits.Turns < 1 || c.Limits.Turns > 100 || c.Limits.ToolCalls < 1 || c.Limits.ToolCalls > 1000 || c.Limits.Seconds < 1 || c.Limits.Seconds > 86400 || c.Limits.OutputTokens < 1 || c.Limits.OutputTokens > 32768 || c.Limits.Attempts < 1 || c.Limits.Attempts > 5 {
		return errors.New("invalid agent execution limits")
	}
	for _, op := range c.Toolset.Operations {
		if !contains([]string{"discover", "list", "read", "create", "update", "delete", "relations"}, op) || op == "*" {
			return fmt.Errorf("unknown resource operation %q", op)
		}
	}
	if len(c.Toolset.Tools) > 64 || len(c.Toolset.Resources) > 500 || len(c.Toolset.Actions) > 500 {
		return errors.New("agent toolset exceeds configuration limits")
	}
	names := map[string]bool{}
	for _, b := range c.Toolset.Tools {
		if b.Name == "" || strings.HasPrefix(b.Name, "daptin_") || strings.HasPrefix(b.Name, "agent_") || names[b.Name] || b.Entity == "" || b.Action == "" {
			return errors.New("invalid or duplicate named tool")
		}
		names[b.Name] = true
		if schema, err := argumentSchema(b.Parameters); err != nil {
			return fmt.Errorf("tool %s: %w", b.Name, err)
		} else if schema.Type != "object" {
			return errors.New("tool parameters must describe an object")
		}
	}
	return nil
}

// The supported dialect is the existing OpenAPI 3 schema validator's structural
// subset. Reject unsupported keywords rather than silently ignoring constraints.
func argumentSchema(raw json.RawMessage) (*openapi3.Schema, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, errors.New("tool parameters must be an object schema")
	}
	allowed := []string{"type", "description", "properties", "required", "additionalProperties", "items", "enum", "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems", "pattern", "nullable"}
	for key, value := range object {
		if !contains(allowed, key) {
			return nil, fmt.Errorf("unsupported schema keyword %s", key)
		}
		switch key {
		case "properties":
			var properties map[string]json.RawMessage
			if err := json.Unmarshal(value, &properties); err != nil {
				return nil, err
			}
			for _, v := range properties {
				if _, err := argumentSchema(v); err != nil {
					return nil, err
				}
			}
		case "items":
			if _, err := argumentSchema(value); err != nil {
				return nil, err
			}
		case "additionalProperties":
			if string(value) != "true" && string(value) != "false" {
				if _, err := argumentSchema(value); err != nil {
					return nil, err
				}
			}
		}
	}
	var schema openapi3.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	if err := schema.Validate(context.Background()); err != nil {
		return nil, err
	}
	return &schema, nil
}
func validateArguments(raw string, schema json.RawMessage) (map[string]interface{}, error) {
	if len(raw) > maxPayload {
		return nil, errors.New("tool arguments exceed limit")
	}
	var args map[string]interface{}
	if err := decode(raw, &args); err != nil || args == nil {
		return nil, errors.New("tool arguments must be a JSON object")
	}
	compiled, err := argumentSchema(schema)
	if err != nil {
		return nil, err
	}
	if err := compiled.VisitJSON(args); err != nil {
		return nil, fmt.Errorf("invalid tool arguments: %w", err)
	}
	return args, nil
}
