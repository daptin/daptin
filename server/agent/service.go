package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	store
	gateway  *llm.Gateway
	locks    olric.DMap
	cancel   context.CancelFunc
	done     chan struct{}
	stopping atomic.Bool
}

func New(cruds map[string]*resource.DbResource, gateway *llm.Gateway, client *olric.ClusterClient) (*Service, error) {
	if gateway == nil || client == nil {
		return nil, errors.New("agent service requires gateway and Olric")
	}
	locks, err := client.NewDMap("agent-runs")
	if err != nil {
		return nil, err
	}
	return &Service{store: store{cruds}, gateway: gateway, locks: locks}, nil
}
func (s *Service) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var events <-chan *redis.Message
		if s.cruds["world"].PubSub != nil {
			subscription := s.cruds["world"].PubSub.Subscribe(ctx, "agent-wake")
			defer subscription.Close()
			events = subscription.Channel()
		}

		slots := make(chan struct{}, 4)
		var workers sync.WaitGroup
		defer workers.Wait()
		for {
			select {
			case <-ctx.Done():
				return
			case _, open := <-events:
				if !open {
					events = nil
					continue
				}
				// Notifications are hints; the next bounded database scan decides eligibility.
				ticker.Reset(time.Millisecond)
				continue
			case <-ticker.C:
				ticker.Reset(time.Second)
				if s.stopping.Load() {
					continue
				}
				capacity := cap(slots) - len(slots)
				if capacity == 0 {
					continue
				}
				rows, err := s.candidates(ctx, capacity)
				if err != nil {
					if ctx.Err() == nil {
						log.WithError(err).Error("[agent] reconcile runs")
					}
					continue
				}
				for _, row := range rows {
					select {
					case slots <- struct{}{}:
						workers.Add(1)
						go func(ref string) {
							defer workers.Done()
							defer func() {
								if value := recover(); value != nil {
									log.WithField("run", ref).Errorf("[agent] worker panic: %v", value)
								}
							}()
							defer func() { <-slots }()
							if err := s.advance(ctx, ref); err != nil && ctx.Err() == nil {
								log.WithError(err).WithField("run", ref).Error("[agent] advance interrupted")
							}
						}(reference(row))
					default:
					}
				}
			}
		}
	}()
}
func (s *Service) Quiesce() {
	s.stopping.Store(true)
	if s.cancel != nil {
		s.cancel()
	}
}
func (s *Service) Drain(ctx context.Context) error {
	s.Quiesce()
	if s.done == nil {
		return nil
	}
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Service) Performers() []actionresponse.ActionPerformerInterface {
	var result []actionresponse.ActionPerformerInterface
	for _, name := range []string{"start", "cancel", "decide", "reconcile"} {
		result = append(result, &control{s: s, name: name})
	}
	return result
}

type control struct {
	s    *Service
	name string
}

func (c *control) Name() string { return "agent." + c.name }
func (c *control) DoAction(outcome actionresponse.Outcome, input map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	result, err := c.s.control(c.name, input, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	return api2go.Response{Res: api2go.NewApi2GoModelWithData("agent.result", nil, 0, nil, result)}, []actionresponse.ActionResponse{{ResponseType: "agent.result", Attributes: result}}, nil
}
func (s *Service) control(name string, input map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	defer func() {
		if pubsub := s.cruds["world"].PubSub; pubsub != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = pubsub.Publish(ctx, "agent-wake", "reconcile")
		}
	}()

	session, _ := input["sessionUser"].(*auth.SessionUser)
	if session == nil || session.UserReferenceId == daptinid.NullReferenceId {
		return nil, errors.New("signed-in execution account required")
	}
	// Action outcomes have transient privileges. Reload only persisted membership.
	user, err := s.user(session.UserReferenceId.String(), tx)
	if err != nil {
		return nil, err
	}
	ctx := userContext(context.Background(), user)
	subject, ok := input["subject"].(map[string]interface{})
	if !ok {
		return nil, errors.New("action subject is required")
	}
	if name == "start" {
		definition, err := s.read(ctx, "agent", reference(subject), tx)
		if err != nil {
			return nil, err
		}
		permission := s.cruds["agent"].GetObjectPermissionByReferenceId("agent", daptinid.InterfaceToDIR(definition["reference_id"]), tx)
		if !permission.CanExecute(user.UserReferenceId, user.Groups, s.cruds["agent"].AdministratorGroupId) || !permission.CanRefer(user.UserReferenceId, user.Groups, s.cruds["agent"].AdministratorGroupId) {
			return nil, errors.New("agent execution is forbidden")
		}
		var config Config
		if err := decode(definition["config"], &config); err != nil {
			return nil, err
		}
		if err := config.validate(); err != nil {
			return nil, err
		}
		// The untrusted caller supplies conversation content, not system instructions.
		var messages []map[string]interface{}
		if err := decode(input["messages"], &messages); err != nil {
			return nil, err
		}
		if len(messages) == 0 || len(messages) > 100 {
			return nil, errors.New("one to 100 user messages required")
		}
		for _, message := range messages {
			if message["role"] != "user" {
				return nil, errors.New("initial messages must have user role")
			}
		}
		encoded, err := encode(map[string]interface{}{"model": config.Model, "messages": messages, "max_tokens": config.Limits.OutputTokens})
		if err != nil {
			return nil, err
		}
		payload := []byte(encoded)
		canonical, err := openai.DecodeChatRequest(contract.ID(uuid.NewString()), payload)
		if err != nil {
			return nil, err
		}
		now, err := s.clock(tx)
		if err != nil {
			return nil, err
		}
		state := runState{Config: config, Started: now}
		row, err := s.create(ctx, "agentrun", map[string]interface{}{"agent_id": reference(definition), "status": "queued", "state": state, "config": config, "input": canonical.Chat.Messages, "auth_version": user.AuthVersion, "deadline_at": now.Add(time.Duration(config.Limits.Seconds) * time.Second)}, tx)
		if err != nil {
			return nil, err
		}
		return controlResult(reference(row), "queued"), nil
	}
	row, err := s.controlRun(ctx, reference(subject), tx)
	if err != nil {
		return nil, err
	}
	permission := s.cruds["agentrun"].GetObjectPermissionByReferenceId("agentrun", daptinid.InterfaceToDIR(row["reference_id"]), tx)
	if !permission.CanRead(user.UserReferenceId, user.Groups, s.cruds["agentrun"].AdministratorGroupId) || !permission.CanExecute(user.UserReferenceId, user.Groups, s.cruds["agentrun"].AdministratorGroupId) {
		return nil, errors.New("run control is forbidden")
	}
	status := resource.StringOrEmpty(row["status"])
	var config Config
	if err := decode(row["config"], &config); err != nil {
		return nil, err
	}
	state, err := readState(row, config)
	if err != nil {
		return nil, err
	}
	switch name {
	case "cancel":
		if status == "completed" || status == "failed" || status == "cancelled" {
			return controlResult(reference(row), status), nil
		}
		status = "cancelled"
	case "decide", "reconcile":
		expected := "waiting_decision"
		if name == "reconcile" {
			expected = "unknown"
		}
		if status != expected || state.Active == "" {
			return nil, errors.New("run is not waiting for this control")
		}
		attempt, err := s.load("agenttry", state.Active, tx)
		if err != nil {
			return nil, err
		}
		value := input["decision"]
		if name == "reconcile" {
			if input["attempt"] != state.Active {
				return nil, errors.New("attempt does not match unresolved operation")
			}
			value = input["result"]
		}
		var decoded interface{}
		if err := decode(value, &decoded); err != nil {
			return nil, err
		}
		if _, err := encode(decoded); err != nil {
			return nil, err
		}
		if attempt["status"] != "completed" {
			if _, err := s.update(ctx, "agenttry", attempt, map[string]interface{}{"status": "completed", "result": decoded}, tx); err != nil {
				return nil, err
			}
		}
		status = "queued"
	default:
		return nil, errors.New("unknown control")
	}
	_, err = s.update(ctx, "agentrun", row, map[string]interface{}{"status": status, "claim": nil, "lease_until": nil, "wake_at": nil}, tx)
	return controlResult(reference(row), status), err
}
func (s *Service) candidates(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	return s.cruds["agentrun"].AgentRunCandidates(ctx, limit)
}
func (s *Service) advance(parent context.Context, ref string) error {
	if s.stopping.Load() {
		return nil
	}
	lockCtx, cancelLock := context.WithTimeout(parent, time.Second)
	lock, err := s.locks.LockWithTimeout(lockCtx, ref, leaseDuration, time.Second)
	cancelLock()
	if err != nil {
		return nil
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = lock.Unlock(ctx)
	}()
	token := uuid.NewString()
	row, err := s.claim(parent, ref, token)
	if err != nil || row == nil {
		return err
	}
	configTx, err := s.begin(parent)
	if err != nil {
		s.release(ref, token, Config{})
		return err
	}
	config, err := s.snapshot(ref, configTx)
	configTx.Rollback()
	if err != nil {
		s.release(ref, token, Config{})
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	heartbeatDone := make(chan struct{})
	stopHeartbeat := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := lock.Lease(ctx, leaseDuration); err != nil {
					cancel()
					return
				}
				if err := s.heartbeat(ctx, ref, token); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { close(stopHeartbeat); <-heartbeatDone; s.release(ref, token, config) }()
	for turns := 0; turns < 200; turns++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.stopping.Load() {
			return nil
		}
		stop, err := s.tick(ctx, ref, token, config)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
	return nil
}
func (s *Service) claim(ctx context.Context, ref, token string) (map[string]interface{}, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, err := s.load("agentrun", ref, tx)
	if err != nil {
		return nil, err
	}
	status := resource.StringOrEmpty(row["status"])
	if status != "queued" && status != "running" && status != "waiting_decision" && status != "unknown" {
		return nil, nil
	}
	now, err := s.clock(tx)
	if err != nil {
		return nil, err
	}
	if status == "waiting_decision" || status == "unknown" {
		deadline, ok := row["deadline_at"].(time.Time)
		if !ok || deadline.After(now) {
			return nil, nil
		}
	}
	if expiry, ok := row["lease_until"].(time.Time); ok && expiry.After(now) {
		return nil, nil
	}
	if wake, ok := row["wake_at"].(time.Time); ok && wake.After(now) && row["deadline_at"].(time.Time).After(now) {
		return nil, nil
	}
	row, err = s.update(ctx, "agentrun", row, map[string]interface{}{"status": "running", "claim": token, "lease_until": now.Add(leaseDuration)}, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}
func (s *Service) heartbeat(ctx context.Context, ref, token string) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return err
	}
	user, err := s.authorizeRun(ctx, row, tx)
	if err != nil {
		return err
	}
	ctx = userContext(ctx, user)
	now, err := s.clock(tx)
	if err != nil {
		return err
	}
	if _, err = s.update(ctx, "agentrun", row, map[string]interface{}{"lease_until": now.Add(leaseDuration)}, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) owned(ref, token string, tx *sqlx.Tx) (map[string]interface{}, error) {
	row, err := s.load("agentrun", ref, tx)
	if err != nil {
		return nil, err
	}
	if row["claim"] != token || row["status"] != "running" {
		return nil, errors.New("run ownership lost")
	}
	now, err := s.clock(tx)
	if err != nil {
		return nil, err
	}
	expiry, ok := row["lease_until"].(time.Time)
	if !ok || !expiry.After(now) {
		return nil, errors.New("run ownership expired")
	}
	return row, nil
}
func (s *Service) save(ctx context.Context, row map[string]interface{}, state *runState, status string, tx *sqlx.Tx) error {
	if status != "running" {
		now, err := s.clock(tx)
		if err != nil {
			return err
		}
		state.finishActive(now)
	}
	if err := s.progress(row, state, tx); err != nil {
		return err
	}
	attrs := map[string]interface{}{"state": state, "status": status, "active_try_id": nullableRef(state.Active), "current_step_id": nullableRef(state.Current), "source_step_id": nullableRef(state.Source)}
	if status != "running" {
		attrs["claim"] = nil
		attrs["lease_until"] = nil
	}
	_, err := s.update(ctx, "agentrun", row, attrs, tx)
	return err
}

func (s *Service) progress(row map[string]interface{}, state *runState, tx *sqlx.Tx) error {
	now, err := s.clock(tx)
	if err != nil {
		return err
	}
	state.ProgressAt = now
	if seconds := state.Config.Limits.ActiveSeconds; seconds > 0 {
		used := state.ActiveMS
		if !state.ActiveSince.IsZero() && now.After(state.ActiveSince) {
			used += now.Sub(state.ActiveSince).Milliseconds()
		}
		remaining := seconds*1000 - used
		if remaining < 0 {
			remaining = 0
		}
		state.ActiveRemainingMS = &remaining
	}
	if state.Config.Limits.Tokens > 0 || state.Config.Limits.CostMicros > 0 {
		state.Budget, err = resource.NewMeteringService(&s.cruds).BudgetProgress(state.budget(reference(row), ""), tx)
	}
	return err
}

func (s *Service) fail(ctx context.Context, row map[string]interface{}, state *runState, cause error, tx *sqlx.Tx) error {
	state.Error = cause.Error()
	state.ErrorCode = errorCode(cause)
	if err := s.save(ctx, row, state, "failed", tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Yield only our own live claim after owned work has stopped. A different node
// can recover immediately on graceful shutdown or after a bounded work slice.
func (s *Service) release(ref, token string, config Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback()
	row, err := s.owned(ref, token, tx)
	if err != nil {
		return
	}
	state, err := readState(row, config)
	if err != nil {
		return
	}
	if err := s.save(ctx, row, &state, "queued", tx); err == nil {
		_ = tx.Commit()
	}
}

// The existing GraphQL action response exposes value/message. Keep the public
// run reference available there without extending the shared response schema.
func controlResult(ref, status string) map[string]interface{} {
	return map[string]interface{}{"reference_id": ref, "status": status, "value": ref, "message": status}
}
