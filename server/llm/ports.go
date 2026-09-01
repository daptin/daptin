package llm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/catalog"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const olricCoordinationTimeout = 5 * time.Second

type daptinSecrets struct {
	cruds map[string]*resource.DbResource
}

func (resolver daptinSecrets) ResolveSecret(ctx context.Context, reference string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	referenceID := daptinid.InterfaceToDIR(reference)
	if referenceID == daptinid.NullReferenceId {
		return nil, errors.New("invalid credential reference")
	}
	transaction, err := resolver.cruds["credential"].Connection().Beginx()
	if err != nil {
		return nil, fmt.Errorf("begin LLM credential read: %w", err)
	}
	defer transaction.Rollback()
	credential, err := resolver.cruds["credential"].GetCredentialByReferenceId(referenceID, transaction)
	if err != nil {
		return nil, fmt.Errorf("resolve LLM credential: %w", err)
	}
	apiKey, ok := credential.DataMap["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, errors.New("LLM credential has no api_key")
	}
	if err := transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit LLM credential read: %w", err)
	}
	return []byte(apiKey), nil
}

type daptinAuthorizer struct {
	cruds map[string]*resource.DbResource
}

func (authorizer daptinAuthorizer) Authorize(ctx context.Context, _ contract.Principal, model catalog.Model) error {
	user, err := daptinSessionUser(ctx)
	if err != nil {
		return err
	}
	modelReference := daptinid.InterfaceToDIR(string(model.ID))
	if modelReference == daptinid.NullReferenceId {
		return errors.New("LLM model has an invalid reference")
	}
	transaction, err := authorizer.cruds["llm_model"].Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin LLM authorization read: %w", err)
	}
	defer transaction.Rollback()
	permission := authorizer.cruds["llm_model"].GetObjectPermissionByReferenceId("llm_model", modelReference, transaction)
	if !permission.CanRead(user.UserReferenceId, user.Groups, authorizer.cruds["llm_model"].AdministratorGroupId) {
		return errors.New("LLM model is not available to this user")
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit LLM authorization read: %w", err)
	}
	return nil
}

type daptinMetering struct {
	cruds   map[string]*resource.DbResource
	service *resource.MeteringService
}

func (metering daptinMetering) Admit(ctx context.Context, admission contract.Admission) (contract.ReservationToken, error) {
	user, err := daptinSessionUser(ctx)
	if err != nil {
		return contract.ReservationToken{}, err
	}
	transaction, err := metering.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return contract.ReservationToken{}, fmt.Errorf("begin metering admission: %w", err)
	}
	defer transaction.Rollback()
	config, err := llmMeteringConfig(metering.cruds["world"].ConfigStore, transaction)
	if err != nil {
		return contract.ReservationToken{}, err
	}
	decision, err := metering.service.Admit(resource.MeteringContext{
		RequestID: string(admission.RequestID), User: user, Endpoint: "/v1/" + string(admission.Operation), Method: "POST",
		EntityType: "llm_model", RequestType: "llm_" + string(admission.Operation),
		EstimatedMeasures: gatewayUsageMeasures(admission.EstimatedUsage), Metering: config,
		Metadata: map[string]interface{}{"model_id": admission.ModelID, "operation": admission.Operation},
	}, transaction)
	if err != nil {
		var httpError api2go.HTTPError
		if errors.As(err, &httpError) && httpError.Status() == 402 {
			return contract.ReservationToken{}, fmt.Errorf("%w: %v", gateway.ErrAdmissionDenied, err)
		}
		return contract.ReservationToken{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.ReservationToken{}, fmt.Errorf("commit metering admission: %w", err)
	}
	return contract.ReservationToken{RequestID: admission.RequestID, Opaque: decision.ReservationToken}, nil
}

func (metering daptinMetering) Complete(ctx context.Context, completion contract.Completion) error {
	return metering.terminalize(ctx, completion.Token, "", completion.HTTPStatus, completion.Usage, map[string]interface{}{
		"status": completion.Status, "error_code": completion.ErrorCode, "cache_status": completion.CacheStatus,
		"attempts": completion.Attempts, "first_byte_at": completion.FirstByteAt, "ended_at": completion.EndedAt,
	}, true)
}

func (metering daptinMetering) Cancel(ctx context.Context, cancellation contract.Cancellation) error {
	return metering.terminalize(ctx, cancellation.Token, cancellation.Reason, 499, cancellation.Usage, map[string]interface{}{
		"status": "cancelled", "attempts": cancellation.Attempts, "ended_at": cancellation.EndedAt,
	}, false)
}

func (metering daptinMetering) terminalize(ctx context.Context, token contract.ReservationToken, reason string, status int, usage contract.Usage, metadata map[string]interface{}, complete bool) error {
	if token.Opaque == "" {
		return nil
	}
	user, err := daptinSessionUser(ctx)
	if err != nil {
		return err
	}
	transaction, err := metering.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin metering terminalization: %w", err)
	}
	defer transaction.Rollback()
	config, err := llmMeteringConfig(metering.cruds["world"].ConfigStore, transaction)
	if err != nil {
		return err
	}
	decision := &resource.MeteringDecision{Enabled: true, RequestID: string(token.RequestID), ReservationToken: token.Opaque}
	meteringContext := resource.MeteringContext{
		RequestID: string(token.RequestID), User: user, StatusCode: status, Measures: gatewayUsageMeasures(usage),
		Metering: config, ErrorMessage: reason,
		Metadata: metadata,
	}
	if complete {
		err = metering.service.Complete(meteringContext, decision, transaction)
	} else {
		err = metering.service.Cancel(meteringContext, decision, transaction)
	}
	if err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit metering terminalization: %w", err)
	}
	return nil
}

func gatewayUsageMeasures(usage contract.Usage) map[string]int64 {
	return map[string]int64{
		"input_tokens": usage.InputTokens, "output_tokens": usage.OutputTokens,
		"cache_read_tokens": usage.CacheReadTokens, "cache_write_tokens": usage.CacheWriteTokens,
		"reasoning_tokens": usage.ReasoningTokens, "total_tokens": usage.TotalTokens, "cost_micros": usage.CostMicros,
	}
}

func daptinSessionUser(ctx context.Context) (*auth.SessionUser, error) {
	user, _ := ctx.Value("user").(*auth.SessionUser)
	if user == nil || user.UserId == 0 || user.UserReferenceId == daptinid.NullReferenceId {
		return nil, errors.New("authenticated Daptin session is required")
	}
	return user, nil
}

func llmMeteringConfig(configStore *resource.ConfigStore, transaction *sqlx.Tx) (*table_info.MeteringConfig, error) {
	config := &table_info.MeteringConfig{Enabled: true, CostExpr: "1", MeterType: "requests"}
	if configStore == nil || transaction == nil {
		return nil, errors.New("LLM metering requires Daptin's config store and transaction")
	}
	enabled, err := configStore.GetConfigValueFor("metering.llm.enabled", "backend", transaction)
	if err == nil {
		config.Enabled, err = strconv.ParseBool(strings.TrimSpace(enabled))
		if err != nil {
			return nil, fmt.Errorf("invalid metering.llm.enabled: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read metering.llm.enabled: %w", err)
	}
	if costExpression, err := configStore.GetConfigValueFor("metering.llm.cost_expr", "backend", transaction); err == nil && strings.TrimSpace(costExpression) != "" {
		config.CostExpr = costExpression
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read metering.llm.cost_expr: %w", err)
	}
	if meterType, err := configStore.GetConfigValueFor("metering.llm.meter_type", "backend", transaction); err == nil && strings.TrimSpace(meterType) != "" {
		config.MeterType = meterType
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read metering.llm.meter_type: %w", err)
	}
	if postAction, err := configStore.GetConfigValueFor("metering.llm.post_metering_action", "backend", transaction); err == nil && strings.TrimSpace(postAction) != "" {
		config.PostMeteringAction = postAction
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read metering.llm.post_metering_action: %w", err)
	}
	return config, nil
}

type olricCounterStore struct {
	values olric.DMap
	leases olric.DMap
}

func (store olricCounterStore) Add(ctx context.Context, key string, amount int64, ttl time.Duration) (int64, error) {
	if amount > int64(math.MaxInt) || amount < int64(math.MinInt) {
		return 0, errors.New("counter amount exceeds platform integer range")
	}
	lock, err := store.values.LockWithTimeout(ctx, "lock:"+key, olricCoordinationTimeout, olricCoordinationTimeout)
	if err != nil {
		return 0, err
	}
	defer releaseOlricLock(ctx, lock)
	return store.addLocked(ctx, key, amount, ttl)
}

func (store olricCounterStore) addLocked(ctx context.Context, key string, amount int64, ttl time.Duration) (int64, error) {
	if err := store.values.Put(ctx, key, 0, olric.EX(ttl), olric.NX()); err != nil && !errors.Is(err, olric.ErrKeyFound) {
		return 0, err
	}
	value, err := store.values.Incr(ctx, key, int(amount))
	return int64(value), err
}

func (store olricCounterStore) Get(ctx context.Context, key string) (int64, bool, error) {
	response, err := store.values.Get(ctx, key)
	if errors.Is(err, olric.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	value, err := response.Int64()
	return value, err == nil, err
}

func (store olricCounterStore) Acquire(ctx context.Context, key string, maximum int64, ttl time.Duration) (string, error) {
	if maximum <= 0 {
		return "", gateway.ErrCounterLimit
	}
	lock, err := store.values.LockWithTimeout(ctx, "lock:"+key, olricCoordinationTimeout, olricCoordinationTimeout)
	if err != nil {
		return "", err
	}
	defer releaseOlricLock(ctx, lock)
	value, _, err := store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if value >= maximum {
		return "", gateway.ErrCounterLimit
	}
	value, err = store.addLocked(ctx, key, 1, ttl)
	if err != nil {
		return "", err
	}
	if err := store.values.Expire(ctx, key, ttl); err != nil {
		store.rollbackIncrement(ctx, key, value)
		return "", err
	}
	token := uuid.NewString()
	if err := store.leases.Put(ctx, token, key, olric.EX(ttl), olric.NX()); err != nil {
		store.rollbackIncrement(ctx, key, value)
		return "", err
	}
	return token, nil
}

func (store olricCounterStore) Release(ctx context.Context, token string) error {
	response, err := store.leases.Get(ctx, token)
	if errors.Is(err, olric.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	key, err := response.String()
	if err != nil {
		return err
	}
	lock, err := store.values.LockWithTimeout(ctx, "lock:"+key, olricCoordinationTimeout, olricCoordinationTimeout)
	if err != nil {
		return err
	}
	defer releaseOlricLock(ctx, lock)
	if _, err := store.leases.Get(ctx, token); errors.Is(err, olric.ErrKeyNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := store.leases.Delete(ctx, token); err != nil {
		return err
	}
	value, found, err := store.Get(ctx, key)
	if err != nil || !found {
		return err
	}
	if value <= 1 {
		_, err = store.values.Delete(ctx, key)
		return err
	}
	_, err = store.values.Decr(ctx, key, 1)
	return err
}

func (store olricCounterStore) rollbackIncrement(parent context.Context, key string, value int64) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), olricCoordinationTimeout)
	defer cancel()
	if value <= 1 {
		_, _ = store.values.Delete(ctx, key)
		return
	}
	_, _ = store.values.Decr(ctx, key, 1)
}

func releaseOlricLock(parent context.Context, lock olric.LockContext) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), olricCoordinationTimeout)
	defer cancel()
	_ = lock.Unlock(ctx)
}

func (store olricCounterStore) Delete(ctx context.Context, key string) error {
	_, err := store.values.Delete(ctx, key)
	return err
}

type olricResponseCache struct {
	values olric.DMap
}

func (cache olricResponseCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	response, err := cache.values.Get(ctx, key)
	if errors.Is(err, olric.ErrKeyNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	payload, err := response.Byte()
	return payload, err == nil, err
}

func (cache olricResponseCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return cache.values.Put(ctx, key, value, olric.EX(ttl))
}

func (cache olricResponseCache) Delete(ctx context.Context, key string) error {
	_, err := cache.values.Delete(ctx, key)
	return err
}
