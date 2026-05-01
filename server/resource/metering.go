package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

const meteringInternalContextKey = "metering_internal"

var meteringSystemTables = map[string]bool{
	"api_plan":   true,
	"api_member": true,
	"api_usage":  true,
	"api_quota":  true,
}

type MeteringService struct {
	cruds *map[string]*DbResource
}

type MeteringContext struct {
	Request       *http.Request
	User          *auth.SessionUser
	Endpoint      string
	Method        string
	EntityType    string
	ActionName    string
	RequestType   string
	StatusCode    int
	LatencyMS     int
	RequestBytes  int
	ResponseBytes int
	Metering      *table_info.MeteringConfig
	Metadata      map[string]interface{}
	Response      map[string]interface{}
}

type MeteringDecision struct {
	Enabled      bool
	Allowed      bool
	EnforceMode  string
	MeterType    string
	CostExpr     string
	PlanID       int64
	MemberID     int64
	QuotaID      int64
	Plan         map[string]interface{}
	Member       map[string]interface{}
	Quota        map[string]interface{}
	ErrorMessage string
}

func NewMeteringService(cruds *map[string]*DbResource) *MeteringService {
	return &MeteringService{cruds: cruds}
}

func IsMeteringInternalRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	val, _ := req.Context().Value(meteringInternalContextKey).(bool)
	return val
}

func WithMeteringInternal(ctx context.Context) context.Context {
	return context.WithValue(ctx, meteringInternalContextKey, true)
}

func IsMeteringSystemTable(tableName string) bool {
	return meteringSystemTables[tableName]
}

func (m *MeteringService) Preflight(ctx MeteringContext, tx *sqlx.Tx) (*MeteringDecision, error) {
	return m.preflight(ctx, tx, true)
}

func (m *MeteringService) preflight(ctx MeteringContext, tx *sqlx.Tx, consumeRateLimit bool) (*MeteringDecision, error) {
	decision := &MeteringDecision{Allowed: true}
	cfg := normalizeMeteringConfig(ctx.Metering)
	if cfg == nil || !cfg.Enabled {
		return decision, nil
	}
	decision.Enabled = true
	decision.EnforceMode = cfg.EnforceMode
	decision.MeterType = cfg.MeterType
	decision.CostExpr = cfg.CostExpr

	if ctx.User == nil || ctx.User.UserId == 0 {
		return decision, nil
	}

	member, err := m.findActiveMember(ctx.User.UserId, tx)
	if err != nil {
		log.Debugf("[metering] no active api_member for user %d: %v", ctx.User.UserId, err)
		return decision, nil
	}
	decision.Member = member
	decision.MemberID = toInt64(member["id"])
	decision.PlanID = toInt64(member["api_plan_id"])
	if decision.PlanID == 0 {
		return decision, nil
	}

	plan, err := m.findPlan(decision.PlanID, tx)
	if err != nil {
		return decision, err
	}
	decision.Plan = plan
	quota, err := m.ensureQuota(ctx.User.UserId, decision.PlanID, decision.MemberID, member, tx)
	if err != nil {
		return decision, err
	}
	decision.Quota = quota
	decision.QuotaID = toInt64(quota["id"])

	allowed, message := checkMeteringQuota(plan, quota, decision.MeterType)
	decision.Allowed = allowed
	decision.ErrorMessage = message
	if !allowed && decision.EnforceMode == "hard" {
		return decision, api2go.NewHTTPError(errors.New(message), "insufficient_quota", 402)
	}
	if consumeRateLimit {
		allowed, message = checkMeteringRateLimit(ctx, plan)
		decision.Allowed = allowed
		decision.ErrorMessage = message
		if !allowed && decision.EnforceMode == "hard" {
			return decision, api2go.NewHTTPError(errors.New(message), "rate_limit_exceeded", 429)
		}
	}
	return decision, nil
}

func (m *MeteringService) Record(ctx MeteringContext, decision *MeteringDecision, tx *sqlx.Tx) error {
	cfg := normalizeMeteringConfig(ctx.Metering)
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if ctx.User == nil || ctx.User.UserId == 0 {
		return nil
	}
	if decision == nil || !decision.Enabled {
		var err error
		decision, err = m.preflight(ctx, tx, false)
		if err != nil {
			return err
		}
	}
	if decision.MemberID == 0 || decision.PlanID == 0 {
		return nil
	}

	costUnits, evalErr := EvaluateMeteringCost(decision.CostExpr, map[string]interface{}{
		"request":  requestEnv(ctx),
		"response": ctx.Response,
		"metadata": ctx.Metadata,
		"user":     userEnv(ctx.User),
		"plan":     decision.Plan,
	})
	errorMessage := ""
	if evalErr != nil {
		errorMessage = evalErr.Error()
		costUnits = 0
	}
	if costUnits < 0 {
		costUnits = 0
	}

	costMicros := costUnits * toInt64(decision.Plan["overage_price_micros"])
	usageRef, _ := uuid.NewV7()
	now := time.Now()
	metadata := ToJson(ctx.Metadata)
	if metadata == "" || metadata == "null" {
		metadata = "{}"
	}

	insert := statementbuilder.Squirrel.Insert("api_usage").Prepared(true).
		Cols("user_account_id", "api_plan_id", "api_member_id", "endpoint", "method", "entity_type", "action_name",
			"request_type", "status_code", "latency_ms", "request_bytes", "response_bytes", "cost_units",
			"cost_micros", "meter_type", "metadata", "error_message", "reference_id", "permission", "created_at", "updated_at").
		Vals([]interface{}{ctx.User.UserId, decision.PlanID, decision.MemberID, ctx.Endpoint, ctx.Method, nullableString(ctx.EntityType),
			nullableString(ctx.ActionName), nullableString(ctx.RequestType), ctx.StatusCode, ctx.LatencyMS, ctx.RequestBytes,
			ctx.ResponseBytes, costUnits, costMicros, decision.MeterType, metadata, nullableString(errorMessage), usageRef[:],
			auth.DEFAULT_PERMISSION, now, now})
	query, args, err := insert.ToSQL()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(query, args...); err != nil {
		return err
	}

	usageID, err := GetReferenceIdToIdWithTransaction("api_usage", daptinid.DaptinReferenceId(usageRef), tx)
	if err != nil {
		log.Warnf("[metering] failed to resolve api_usage id: %v", err)
	}
	if err = m.incrementQuota(decision.QuotaID, decision.MeterType, costUnits, ctx.ResponseBytes, tx); err != nil {
		return err
	}
	if cfg.PostMeteringAction != "" {
		m.invokePostMeteringAction(cfg.PostMeteringAction, ctx, decision, usageID, costUnits, costMicros, tx)
	}
	return nil
}

func normalizeMeteringConfig(cfg *table_info.MeteringConfig) *table_info.MeteringConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	if normalized.CostExpr == "" {
		normalized.CostExpr = "1"
	}
	if normalized.MeterType == "" {
		normalized.MeterType = "requests"
	}
	if normalized.EnforceMode == "" {
		normalized.EnforceMode = "hard"
	}
	return &normalized
}

func meteringConfigForAction(cfg *table_info.MeteringConfig, actionName string) *table_info.MeteringConfig {
	if cfg == nil {
		return nil
	}
	if cfg.OnActions != nil {
		if actionCfg, ok := cfg.OnActions[actionName]; ok {
			if actionCfg.CostExpr == "" {
				actionCfg.CostExpr = cfg.CostExpr
			}
			if actionCfg.MeterType == "" {
				actionCfg.MeterType = cfg.MeterType
			}
			if actionCfg.EnforceMode == "" {
				actionCfg.EnforceMode = cfg.EnforceMode
			}
			if actionCfg.PostMeteringAction == "" {
				actionCfg.PostMeteringAction = cfg.PostMeteringAction
			}
			return &actionCfg
		}
	}
	return cfg
}

func checkMeteringQuota(plan map[string]interface{}, quota map[string]interface{}, meterType string) (bool, string) {
	requestLimit := toInt64(plan["requests_per_period"])
	if requestLimit >= 0 && toInt64(quota["request_count"])+1 > requestLimit {
		return false, "request quota exceeded"
	}
	if meterType == "compute_units" {
		computeLimit := toInt64(plan["compute_units_per_period"])
		if computeLimit >= 0 && toInt64(quota["compute_units"]) >= computeLimit {
			return false, "compute quota exceeded"
		}
	}
	return true, ""
}

func checkMeteringRateLimit(ctx MeteringContext, plan map[string]interface{}) (bool, string) {
	limit := toInt64(plan["rate_limit_per_minute"])
	if limit < 0 || ctx.User == nil || ctx.User.UserId == 0 {
		return true, ""
	}
	if OlricCache == nil {
		log.Warnf("[metering] rate_limit_per_minute configured but Olric cache is not initialized")
		return true, ""
	}
	windowStart := time.Now().UTC().Truncate(time.Minute)
	key := fmt.Sprintf("api-rate-limit:%d:%d:%s", ctx.User.UserId, toInt64(plan["id"]), windowStart.Format("200601021504"))
	cacheCtx := context.Background()
	err := OlricCache.Put(cacheCtx, key, 0, olric.EX(time.Minute), olric.NX())
	if err != nil && !errors.Is(err, olric.ErrKeyFound) {
		log.Warnf("[metering] failed to initialize rate limit key %s: %v", key, err)
		return true, ""
	}
	count, err := OlricCache.Incr(cacheCtx, key, 1)
	if err != nil {
		log.Warnf("[metering] failed to increment rate limit key %s: %v", key, err)
		return true, ""
	}
	if int64(count) > limit {
		return false, "rate limit exceeded"
	}
	return true, ""
}

func (m *MeteringService) findActiveMember(userID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, args, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From("api_member").
		Where(goqu.Ex{"user_account_id": userID, "status": "active"}).
		Order(goqu.I("id").Desc()).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMap(tx, query, args...)
}

func (m *MeteringService) findPlan(planID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, args, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From("api_plan").
		Where(goqu.Ex{"id": planID}).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMap(tx, query, args...)
}

func (m *MeteringService) ensureQuota(userID, planID, memberID int64, member map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	periodStart := toTime(member["period_start"])
	if periodStart.IsZero() {
		periodStart = time.Now()
	}
	query, args, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From("api_quota").
		Where(goqu.Ex{"user_account_id": userID, "api_plan_id": planID, "api_member_id": memberID, "period_start": periodStart}).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	quota, err := querySingleMap(tx, query, args...)
	if err == nil {
		return quota, nil
	}

	ref, _ := uuid.NewV7()
	now := time.Now()
	insert := statementbuilder.Squirrel.Insert("api_quota").Prepared(true).
		Cols("user_account_id", "api_plan_id", "api_member_id", "period_start", "period_end", "request_count",
			"compute_units", "bytes_used", "reference_id", "permission", "created_at", "updated_at").
		Vals([]interface{}{userID, planID, memberID, periodStart, member["period_end"], 0, 0, 0, ref[:], auth.DEFAULT_PERMISSION, now, now})
	query, args, err = insert.ToSQL()
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(query, args...); err != nil {
		return nil, err
	}
	query, args, err = statementbuilder.Squirrel.Select("*").Prepared(true).
		From("api_quota").Where(goqu.Ex{"reference_id": ref[:]}).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMap(tx, query, args...)
}

func (m *MeteringService) incrementQuota(quotaID int64, meterType string, costUnits int64, bytesUsed int, tx *sqlx.Tx) error {
	if quotaID == 0 {
		return nil
	}
	selectSQL, selectArgs, err := statementbuilder.Squirrel.Select("*").Prepared(true).
		From("api_quota").Where(goqu.Ex{"id": quotaID}).Limit(1).ToSQL()
	if err != nil {
		return err
	}
	existing, err := querySingleMap(tx, selectSQL, selectArgs...)
	if err != nil {
		return err
	}
	record := goqu.Record{
		"request_count": toInt64(existing["request_count"]) + 1,
		"bytes_used":    toInt64(existing["bytes_used"]) + int64(bytesUsed),
		"updated_at":    time.Now(),
	}
	if meterType == "compute_units" {
		record["compute_units"] = toInt64(existing["compute_units"]) + costUnits
	}
	query, args, err := statementbuilder.Squirrel.Update("api_quota").Prepared(true).
		Set(record).Where(goqu.Ex{"id": quotaID}).ToSQL()
	if err != nil {
		return err
	}
	_, err = tx.Exec(query, args...)
	return err
}

func (m *MeteringService) invokePostMeteringAction(actionName string, ctx MeteringContext, decision *MeteringDecision, usageID int64, costUnits, costMicros int64, tx *sqlx.Tx) {
	parts := strings.SplitN(actionName, ":", 2)
	if len(parts) != 2 {
		log.Warnf("[metering] invalid post_metering_action: %s", actionName)
		return
	}
	cruds := *m.cruds
	crud, ok := cruds[parts[0]]
	if !ok {
		log.Warnf("[metering] post_metering_action entity not found: %s", parts[0])
		return
	}
	if ctx.Request == nil {
		return
	}
	req := ctx.Request.WithContext(WithMeteringInternal(ctx.Request.Context()))
	actionReq := actionresponse.ActionRequest{
		Type:   parts[0],
		Action: parts[1],
		Attributes: map[string]interface{}{
			"user_account_id":   ctx.User.UserReferenceId.String(),
			"api_usage_id":      usageID,
			"api_plan_id":       decision.PlanID,
			"api_member_id":     decision.MemberID,
			"cost_units":        costUnits,
			"cost_micros":       costMicros,
			"meter_type":        decision.MeterType,
			"endpoint":          ctx.Endpoint,
			"entity_type":       ctx.EntityType,
			"action_name":       ctx.ActionName,
			"metadata":          ctx.Metadata,
			"metering_internal": true,
		},
	}
	_, err := crud.HandleActionRequest(actionReq, api2go.Request{PlainRequest: req}, tx)
	if err != nil {
		log.Errorf("[metering] post_metering_action failed: %v", err)
	}
}

func querySingleMap(tx *sqlx.Tx, query string, args ...interface{}) (map[string]interface{}, error) {
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("no rows")
	}
	row := map[string]interface{}{}
	if err = rows.MapScan(row); err != nil {
		return nil, err
	}
	return row, nil
}

func requestEnv(ctx MeteringContext) map[string]interface{} {
	return map[string]interface{}{
		"endpoint":       ctx.Endpoint,
		"method":         ctx.Method,
		"entity_type":    ctx.EntityType,
		"action_name":    ctx.ActionName,
		"request_type":   ctx.RequestType,
		"status_code":    ctx.StatusCode,
		"latency_ms":     ctx.LatencyMS,
		"request_bytes":  ctx.RequestBytes,
		"response_bytes": ctx.ResponseBytes,
	}
}

func userEnv(user *auth.SessionUser) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":           user.UserId,
		"reference_id": user.UserReferenceId.String(),
	}
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case uint64:
		return int64(v)
	case uint:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case []byte:
		var out int64
		fmt.Sscanf(string(v), "%d", &out)
		return out
	case string:
		var out int64
		fmt.Sscanf(v, "%d", &out)
		return out
	case nil:
		return 0
	default:
		return 0
	}
}

func toTime(value interface{}) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v
	case string:
		t, _ := time.Parse(time.RFC3339, v)
		return t
	case []byte:
		t, _ := time.Parse(time.RFC3339, string(v))
		return t
	default:
		return time.Time{}
	}
}
