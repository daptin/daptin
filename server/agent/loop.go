package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func (s *Service) tick(ctx context.Context, ref, token string) (bool, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return true, err
	}
	var state runState
	if err := decode(row["state"], &state); err != nil {
		return true, err
	}
	user, err := s.authorizeRun(ctx, row, tx)
	if err != nil {
		return true, s.fail(ctx, row, &state, err, tx)
	}
	now, err := s.clock(tx)
	if err != nil {
		return true, err
	}
	ctx = userContext(ctx, user)
	state.finishActive(now)
	remaining, _, limitErr := state.remaining(now)
	if limitErr != nil {
		return true, s.fail(ctx, row, &state, limitErr, tx)
	}
	var operation intent
	var attempt map[string]interface{}
	if state.Active != "" {
		attempt, err = s.load("agenttry", state.Active, tx)
		if err != nil {
			return true, err
		}
		if err := decode(attempt["intent"], &operation); err != nil {
			return true, err
		}
		switch attempt["status"] {
		case "completed":
			return s.accept(ctx, row, &state, operation, attempt["result"], tx)
		case "waiting":
			// Only timer waits are runnable without a submitted decision.
			if operation.Call.Function.Name != "agent_wait" {
				return true, errors.New("decision has no submitted result")
			}
			if _, err = s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "completed", "result": map[string]interface{}{"resumed": true}}, tx); err != nil {
				return true, err
			}
			return false, tx.Commit()

		case "unknown":
			state.Error = "tool effect is unknown; reconcile this attempt before continuing"
			if err := s.save(ctx, row, &state, "unknown", tx); err != nil {
				return true, err
			}
			return true, tx.Commit()
		case "failed":
			return true, s.fail(ctx, row, &state, errors.New("saved attempt failed"), tx)
		case "dispatched":
			if operation.Kind == "tool" && !repeatable(state.Config, operation.Call.Function.Name) {
				if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "unknown"}, tx); err != nil {
					return true, err
				}
				state.Error = "tool effect is unknown; reconcile this attempt before continuing"
				if err := s.save(ctx, row, &state, "unknown", tx); err != nil {
					return true, err
				}
				return true, tx.Commit()
			}
			if operation.Number >= state.Config.Limits.Attempts {
				return true, s.fail(ctx, row, &state, &executionLimit{"attempts_exceeded", "attempt limit reached"}, tx)
			}
			if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "interrupted"}, tx); err != nil {
				return true, err
			}
			operation.Number++
		default:
			return true, errors.New("unresolved attempt state")
		}
	} else {
		operation = intent{Operation: uuid.NewString(), Number: 1}
		if len(state.Pending) > 0 {
			if state.Calls >= state.Config.Limits.ToolCalls {
				return true, s.fail(ctx, row, &state, &executionLimit{"tool_calls_exceeded", "tool call limit reached"}, tx)
			}
			operation.Kind = "tool"
			operation.Call = state.Pending[0]
			state.Calls++
		} else {
			if state.Turns >= state.Config.Limits.Turns {
				return true, s.fail(ctx, row, &state, &executionLimit{"turns_exceeded", "model turn limit reached"}, tx)
			}
			operation.Kind = "model"
			state.Turns++
		}
		state.Sequence++
		step, err := s.create(userContext(ctx, user), "agentstp", map[string]interface{}{"agentrun_id": ref, "number": state.Sequence, "kind": operation.Kind, "payload": operation}, tx)
		if err != nil {
			return true, err
		}
		operation.Step = reference(step)
	}
	attemptRef := uuid.NewString()
	operation.RequestID = attemptRef
	attempt, err = s.create(userContext(ctx, user), "agenttry", map[string]interface{}{"reference_id": attemptRef, "agentrun_id": ref, "agentstp_id": operation.Step, "status": "dispatched", "intent": operation}, tx)
	if err != nil {
		return true, err
	}
	state.Active = reference(attempt)
	if operation.Kind == "tool" && (operation.Call.Function.Name == "agent_decision" || operation.Call.Function.Name == "agent_wait") {
		args, err := validateCall(state.Config, operation.Call)
		if err != nil {
			return s.reject(ctx, row, &state, attempt, err, tx)
		}
		status := "waiting_decision"
		if operation.Call.Function.Name == "agent_wait" {
			until, err := time.Parse(time.RFC3339, args["until"].(string))
			if err != nil || !until.After(now) || until.After(state.Started.Add(time.Duration(state.Config.Limits.Seconds)*time.Second)) {
				return s.reject(ctx, row, &state, attempt, errors.New("wait time must precede the run deadline and follow current time"), tx)
			}
			status = "queued"
			if _, err := s.update(ctx, "agentrun", row, map[string]interface{}{"wake_at": until}, tx); err != nil {
				return true, err
			}
			row, err = s.load("agentrun", ref, tx)
			if err != nil {
				return true, err
			}
		}
		if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "waiting"}, tx); err != nil {
			return true, err
		}
		if err := s.save(ctx, row, &state, status, tx); err != nil {
			return true, err
		}
		return true, tx.Commit()
	}
	state.ActiveSince = now
	if err := s.save(ctx, row, &state, "running", tx); err != nil {
		return true, err
	}
	if err := tx.Commit(); err != nil {
		return true, err
	}
	// The intent is durable. No checkpoint transaction spans inference or tool I/O.
	// Revalidate identity, ownership and remaining time immediately before I/O.
	user, remaining, timeCode, err := s.beforeDispatch(ctx, ref, token)
	if err != nil {
		return true, s.terminal(ctx, ref, token, "failed", err)
	}
	work, cancel := context.WithTimeout(userContext(ctx, user), remaining)
	defer cancel()
	if state.Config.Limits.Tokens > 0 || state.Config.Limits.CostMicros > 0 {
		work = resource.WithMeteringBudget(work, state.budget(ref, token))
	}
	result, err := s.execute(work, user, &state, operation)
	if err != nil && errors.Is(work.Err(), context.DeadlineExceeded) {
		err = &executionLimit{timeCode, "execution time limit reached"}
	}
	var rejected *toolRejection
	if errors.As(err, &rejected) {
		result = map[string]interface{}{"error": map[string]interface{}{"message": err.Error()}}
		err = nil
	}
	// Persist late evidence with a fresh bounded context, even after cancellation
	// or ownership loss. It never authorizes advancement of the run.
	evidenceCtx, evidenceCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer evidenceCancel()
	if err != nil {
		if ctx.Err() != nil && (operation.Kind == "model" || repeatable(state.Config, operation.Call.Function.Name)) {
			if saveErr := s.evidence(evidenceCtx, state.Active, "dispatched", map[string]interface{}{"interrupted": true, "error": err.Error()}); saveErr != nil {
				return true, saveErr
			}
			return true, ctx.Err()
		}
		var gatewayError *contract.Error
		if operation.Kind == "model" && errors.As(err, &gatewayError) && gatewayError.Retryable && operation.Number < state.Config.Limits.Attempts {
			return true, s.retry(evidenceCtx, ref, token, state.Active, gatewayError, operation.Number)
		}
		if operation.Kind == "tool" && !repeatable(state.Config, operation.Call.Function.Name) {
			if saveErr := s.evidence(evidenceCtx, state.Active, "unknown", map[string]interface{}{"error": err.Error()}); saveErr != nil {
				return true, saveErr
			}
			return true, s.terminal(evidenceCtx, ref, token, "unknown", err)
		}
		if saveErr := s.evidence(evidenceCtx, state.Active, "failed", map[string]interface{}{"error": err.Error()}); saveErr != nil {
			return true, saveErr
		}
		return true, s.terminal(evidenceCtx, ref, token, "failed", err)
	}
	if _, encodeErr := encodeBounded(result, maxPayload); encodeErr != nil {
		if operation.Kind == "tool" && repeatable(state.Config, operation.Call.Function.Name) {
			result = map[string]interface{}{"error": map[string]interface{}{"message": encodeErr.Error()}}
		} else {
			status := "failed"
			if operation.Kind == "tool" {
				status = "unknown"
			}
			if saveErr := s.evidence(evidenceCtx, state.Active, status, map[string]interface{}{"error": encodeErr.Error()}); saveErr != nil {
				return true, saveErr
			}
			return true, s.terminal(evidenceCtx, ref, token, status, encodeErr)
		}
	}

	if err := s.evidence(evidenceCtx, state.Active, "completed", result); err != nil {
		return true, err
	}
	return false, nil
}
func (s *Service) beforeDispatch(ctx context.Context, ref, token string) (*auth.SessionUser, time.Duration, string, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, 0, "", err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return nil, 0, "", err
	}
	user, err := s.authorizeRun(ctx, row, tx)
	if err != nil {
		return nil, 0, "", err
	}
	var state runState
	if err := decode(row["state"], &state); err != nil {
		return nil, 0, "", err
	}
	now, err := s.clock(tx)
	if err != nil {
		return nil, 0, "", err
	}
	remaining, code, err := state.remaining(now)
	return user, remaining, code, err
}

func (s *Service) evidence(ctx context.Context, ref, status string, result interface{}) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := s.load("agenttry", ref, tx)
	if err != nil {
		return err
	}
	// A reconciler's accepted result cannot be overwritten by an old worker.
	if row["status"] == "completed" {
		return nil
	}
	if _, err = s.update(ctx, "agenttry", row, map[string]interface{}{"status": status, "result": result}, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) terminal(ctx context.Context, ref, token, status string, cause error) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return err
	}
	var state runState
	if err := decode(row["state"], &state); err != nil {
		return err
	}
	state.Error = cause.Error()
	state.ErrorCode = errorCode(cause)
	if err := s.save(ctx, row, &state, status, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) accept(ctx context.Context, row map[string]interface{}, state *runState, operation intent, result interface{}, tx *sqlx.Tx) (bool, error) {
	status := "running"
	if operation.Kind == "model" {
		var response contract.Response
		if err := decode(result, &response); err != nil {
			return true, s.fail(ctx, row, state, err, tx)
		}
		if response.Chat == nil || len(response.Chat.Choices) != 1 {
			return true, s.fail(ctx, row, state, errors.New("model must return one chat choice"), tx)
		}
		choice := response.Chat.Choices[0]
		message := choice.Message
		if message.Role != "assistant" {
			return true, s.fail(ctx, row, state, errors.New("model returned invalid message role"), tx)
		}
		seen := map[string]bool{}
		if state.Calls+len(message.ToolCalls) > state.Config.Limits.ToolCalls {
			return true, s.fail(ctx, row, state, &executionLimit{"tool_calls_exceeded", "tool call limit reached"}, tx)
		}
		for _, call := range message.ToolCalls {
			if seen[call.ID] {
				return true, s.fail(ctx, row, state, errors.New("duplicate tool call identity"), tx)
			}
			seen[call.ID] = true
			if call.ID == "" || call.Type != "function" {
				return true, s.fail(ctx, row, state, errors.New("invalid tool call identity or type"), tx)
			}
		}
		state.Messages = append(state.Messages, message)
		state.Pending = message.ToolCalls
		if len(message.ToolCalls) == 0 {
			if choice.FinishReason != "stop" {
				return true, s.fail(ctx, row, state, fmt.Errorf("model stopped with %s", choice.FinishReason), tx)
			}
			status = "completed"
			for _, part := range message.Content {
				if part.Type == "text" || part.Type == "output_text" {
					state.Result += part.Text
				}
			}
		}
	} else {
		var value interface{}
		if err := decode(result, &value); err != nil {
			return true, err
		}
		encoded, err := encode(value)
		if err != nil {
			return true, err
		}
		state.Messages = append(state.Messages, contract.Message{Role: "tool", ToolCallID: operation.Call.ID, Content: []contract.ContentPart{{Type: "text", Text: encoded}}})
		if len(state.Pending) == 0 || state.Pending[0].ID != operation.Call.ID {
			return true, errors.New("tool result does not match pending operation")
		}
		state.Pending = state.Pending[1:]
	}
	step, err := s.load("agentstp", operation.Step, tx)
	if err != nil {
		return true, err
	}
	if _, err := s.update(ctx, "agentstp", step, map[string]interface{}{"payload": map[string]interface{}{"operation": operation, "result": result}}, tx); err != nil {
		return true, err
	}
	state.Active = ""
	state.Error = ""
	state.ErrorCode = ""
	if err := s.save(ctx, row, state, status, tx); err != nil {
		return true, err
	}
	return status != "running", tx.Commit()
}

// The run owns retry admission. Only the gateway's explicit retryable contract
// admits another inference attempt, with a durable wake and bounded attempts.
func (s *Service) retry(ctx context.Context, ref, token, attemptRef string, cause *contract.Error, number int) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return err
	}
	attempt, err := s.load("agenttry", attemptRef, tx)
	if err != nil {
		return err
	}
	if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"result": map[string]interface{}{"retryable": true, "error": cause.Error()}}, tx); err != nil {
		return err
	}
	var state runState
	if err := decode(row["state"], &state); err != nil {
		return err
	}
	state.Error = cause.Error()
	now, err := s.clock(tx)
	if err != nil {
		return err
	}
	delay := time.Duration(5*(1<<uint(number-1))) * time.Second
	if cause.RetryAfter > delay {
		delay = cause.RetryAfter
	}
	state.finishActive(now)
	wake := now.Add(delay)
	deadline := state.Started.Add(time.Duration(state.Config.Limits.Seconds) * time.Second)
	// Wake at the deadline only to terminalize; never dispatch sooner than a
	// provider's RetryAfter when it lies beyond the permitted run lifetime.
	if wake.After(deadline) {
		wake = deadline
	}
	row, err = s.update(ctx, "agentrun", row, map[string]interface{}{"wake_at": wake}, tx)
	if err != nil {
		return err
	}
	if err := s.save(ctx, row, &state, "queued", tx); err != nil {
		return err
	}
	return tx.Commit()
}

// A performer failure is confined to this attempt, just as HTTP recovery confines
// it to one request. A mutation panic remains an uncertain effect.
func (s *Service) execute(ctx context.Context, user *auth.SessionUser, state *runState, operation intent) (result interface{}, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("execution panic: %v", value)
		}
	}()
	if operation.Kind == "model" {
		return s.gateway.Invoke(ctx, user, contract.Request{ID: contract.ID(operation.RequestID), Operation: contract.OperationChat, PublicModel: state.Config.Model, MaxOutputTokens: state.Config.Limits.OutputTokens, Chat: &contract.ChatRequest{Messages: state.Messages, Tools: tools(state.Config), N: 1, MaxCompletionTokens: state.Config.Limits.OutputTokens}})
	}
	return s.dispatch(ctx, state, operation.Operation, operation.Call)
}

func (s *Service) reject(ctx context.Context, row map[string]interface{}, state *runState, attempt map[string]interface{}, cause error, tx *sqlx.Tx) (bool, error) {
	if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "completed", "result": map[string]interface{}{"error": map[string]interface{}{"message": cause.Error()}}}, tx); err != nil {
		return true, err
	}
	if err := s.save(ctx, row, state, "running", tx); err != nil {
		return true, err
	}
	return false, tx.Commit()
}
