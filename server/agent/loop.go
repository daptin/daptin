package agent

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func (s *Service) tick(ctx context.Context, ref, token string, config Config) (bool, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	if err := s.restart.CheckTransaction(tx); err != nil {
		return true, err
	}
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return true, err
	}
	state, err := readState(row, config)
	if err != nil {
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
		step, err := s.load("agentstp", refValue(attempt["agentstp_id"]), tx)
		if err != nil {
			return true, err
		}
		operation, err = s.operation(step, attempt, tx)
		if err != nil {
			return true, err
		}
		switch attempt["status"] {
		case "completed":
			saved, err := s.result(state.Active, tx)
			if err != nil {
				return true, err
			}
			return s.accept(ctx, row, &state, operation, saved["result"], tx)
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
		if state.Source != "" {
			if state.Calls >= state.Config.Limits.ToolCalls {
				return true, s.fail(ctx, row, &state, &executionLimit{"tool_calls_exceeded", "tool call limit reached"}, tx)
			}
			operation.Kind = "tool"
			calls, err := s.stepCalls(ref, state.Source, tx)
			if err != nil {
				return true, err
			}
			if state.NextCall < 0 || state.NextCall >= len(calls) {
				return true, errors.New("invalid pending tool position")
			}
			operation.Call = calls[state.NextCall]
			state.Calls++
		} else {
			if state.Turns >= state.Config.Limits.Turns {
				return true, s.fail(ctx, row, &state, &executionLimit{"turns_exceeded", "model turn limit reached"}, tx)
			}
			operation.Kind = "model"
			state.Turns++
		}
		state.Sequence++
		step, err := s.create(userContext(ctx, user), "agentstp", map[string]interface{}{"agentrun_id": ref, "number": state.Sequence, "kind": operation.Kind, "operation": operation.Operation, "source_step_id": nullableRef(state.Source), "call_id": operation.Call.ID}, tx)
		if err != nil {
			return true, err
		}
		operation.Step = reference(step)
		state.Current = operation.Step
	}
	if operation.Kind == "model" {
		state.Messages, err = s.conversation(ref, &state, tx)
		if err != nil {
			return true, s.fail(ctx, row, &state, err, tx)
		}
	}
	attemptRef := uuid.NewString()
	operation.RequestID = attemptRef
	attempt, err = s.create(userContext(ctx, user), "agenttry", map[string]interface{}{"reference_id": attemptRef, "agentrun_id": ref, "agentstp_id": operation.Step, "status": "dispatched", "number": operation.Number}, tx)
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
	user, remaining, timeCode, err := s.beforeDispatch(ctx, ref, token, config)
	if err != nil {
		return true, s.terminal(ctx, ref, token, "failed", err, config)
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
			return true, s.retry(evidenceCtx, ref, token, state.Active, gatewayError, operation.Number, config)
		}
		if operation.Kind == "tool" && !repeatable(state.Config, operation.Call.Function.Name) {
			if saveErr := s.evidence(evidenceCtx, state.Active, "unknown", map[string]interface{}{"error": err.Error()}); saveErr != nil {
				return true, saveErr
			}
			return true, s.terminal(evidenceCtx, ref, token, "unknown", err, config)
		}
		if saveErr := s.evidence(evidenceCtx, state.Active, "failed", map[string]interface{}{"error": err.Error()}); saveErr != nil {
			return true, saveErr
		}
		return true, s.terminal(evidenceCtx, ref, token, "failed", err, config)
	}
	if _, encodeErr := encode(result); encodeErr != nil {
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
			return true, s.terminal(evidenceCtx, ref, token, status, encodeErr, config)
		}
	}

	if err := s.evidence(evidenceCtx, state.Active, "completed", result); err != nil {
		return true, err
	}
	return false, nil
}
func (s *Service) beforeDispatch(ctx context.Context, ref, token string, config Config) (*auth.SessionUser, time.Duration, string, error) {
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
	state, err := readState(row, config)
	if err != nil {
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
func (s *Service) terminal(ctx context.Context, ref, token, status string, cause error, config Config) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return err
	}
	state, err := readState(row, config)
	if err != nil {
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
	step, err := s.load("agentstp", operation.Step, tx)
	if err != nil {
		return true, err
	}
	number, err := resource.ResourceRowInt64(step["number"])
	if err != nil {
		return true, err
	}
	if refValue(step["agentrun_id"]) != reference(row) || operation.Step != state.Current {
		return true, errors.New("step belongs to a different execution")
	}
	if selected := refValue(step["accepted_try_id"]); selected != "" {
		if selected != state.Active || int(number) != state.Accepted {
			return true, errors.New("step already has a different accepted attempt")
		}
		state.Active = ""
		if err := s.save(ctx, row, state, resource.StringOrEmpty(row["status"]), tx); err != nil {
			return true, err
		}
		return row["status"] != "running", tx.Commit()
	}
	if int(number) != state.Accepted+1 {
		return true, errors.New("step acceptance is out of order")
	}
	status := "running"
	if operation.Kind == "model" {
		message, finish, err := modelMessage(result)
		if err != nil {
			return true, s.fail(ctx, row, state, err, tx)
		}
		if state.Calls+len(message.ToolCalls) > state.Config.Limits.ToolCalls {
			return true, s.fail(ctx, row, state, &executionLimit{"tool_calls_exceeded", "tool call limit reached"}, tx)
		}
		state.NextCall = 0
		if len(message.ToolCalls) > 0 {
			state.Source = operation.Step
		} else {
			if finish != "stop" {
				return true, s.fail(ctx, row, state, fmt.Errorf("model stopped with %s", finish), tx)
			}
			state.Source = ""
			status = "completed"
		}
	} else {
		if state.Source == "" || state.Source != refValue(step["source_step_id"]) {
			return true, errors.New("tool result has a different source")
		}
		calls, err := s.stepCalls(reference(row), state.Source, tx)
		if err != nil {
			return true, err
		}
		if state.NextCall >= len(calls) || calls[state.NextCall].ID != operation.Call.ID {
			return true, errors.New("tool result does not match pending operation")
		}
		state.NextCall++
		if state.NextCall == len(calls) {
			state.Source = ""
			state.NextCall = 0
		}
	}
	if _, err := s.update(ctx, "agentstp", step, map[string]interface{}{"accepted_try_id": state.Active}, tx); err != nil {
		return true, err
	}
	state.Accepted = int(number)
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
func (s *Service) retry(ctx context.Context, ref, token, attemptRef string, cause *contract.Error, number int, config Config) error {
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
	state, err := readState(row, config)
	if err != nil {
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
			log.WithError(err).WithField("stack", string(debug.Stack())).Error("[agent] execution panic")
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
