package resource

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
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

const (
	meteringInternalContextKey = "metering_internal"
	defaultReservationTTL      = 5 * time.Minute
	meteringStateHeld          = "held"
	meteringStateCompleted     = "completed"
	meteringStateCancelled     = "cancelled"
	meteringStateExpired       = "expired"
)

var (
	errMeteringRequestTerminal = errors.New("metering request is already terminal")
	errMeteringRowNotFound     = errors.New("metering row not found")
)

var meteringSystemTables = map[string]bool{
	"api_plan": true, "api_member": true, "api_usage": true, "api_quota": true,
}

type MeteringService struct {
	cruds *map[string]*DbResource
	now   func() time.Time
}

type MeteringContext struct {
	Request           *http.Request
	RequestID         string
	APIKeyID          string
	User              *auth.SessionUser
	Endpoint          string
	Method            string
	EntityType        string
	ActionName        string
	RequestType       string
	StatusCode        int
	LatencyMS         int
	RequestBytes      int
	ResponseBytes     int
	EstimatedMeasures map[string]int64
	Measures          map[string]int64
	ReservationTTL    time.Duration
	Metering          *table_info.MeteringConfig
	Metadata          map[string]interface{}
	Response          map[string]interface{}
	ErrorMessage      string
}

type MeteringDecision struct {
	Enabled          bool
	UsageID          int64
	RequestID        string
	ReservationToken string
	PlanID           int64
	MemberID         int64
	Plan             map[string]interface{}
	Member           map[string]interface{}
	ReservedMeasures map[string]int64
	reservation      map[string]meteringReservation
	config           *table_info.MeteringConfig
}

type meteringLimit struct {
	Metric  string `json:"metric"`
	Window  string `json:"window"`
	Maximum int64  `json:"maximum"`
	Mode    string `json:"mode"`
}

type meteringReservation struct {
	BucketKey string `json:"bucket_key"`
	Metric    string `json:"metric"`
	Amount    int64  `json:"amount"`
}

func NewMeteringService(cruds *map[string]*DbResource) *MeteringService {
	return &MeteringService{cruds: cruds, now: time.Now}
}

func (m *MeteringService) internalUser(user *auth.SessionUser) *auth.SessionUser {
	copyOfUser := *user
	groups := append(auth.GroupPermissionList(nil), user.Groups...)
	administrator := (*m.cruds)["api_usage"].AdministratorGroupId
	for _, group := range groups {
		if group.GroupReferenceId == administrator {
			copyOfUser.Groups = groups
			return &copyOfUser
		}
	}
	copyOfUser.Groups = append(groups, auth.GroupPermission{GroupReferenceId: administrator})
	return &copyOfUser
}

func IsMeteringInternalRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	value, _ := req.Context().Value(meteringInternalContextKey).(bool)
	return value
}

func WithMeteringInternal(ctx context.Context) context.Context {
	return context.WithValue(ctx, meteringInternalContextKey, true)
}

func IsMeteringSystemTable(tableName string) bool {
	return meteringSystemTables[tableName]
}

func (m *MeteringService) Admit(ctx MeteringContext, tx *sqlx.Tx) (*MeteringDecision, error) {
	decision := &MeteringDecision{}
	config := normalizeMeteringConfig(ctx.Metering)
	if config == nil || !config.Enabled || ctx.User == nil || ctx.User.UserId == 0 {
		return decision, nil
	}
	if tx == nil {
		return nil, errors.New("metering admission requires a transaction")
	}
	decision.Enabled = true
	decision.config = config
	if err := m.lockMeteringUser(ctx.User.UserId, tx); err != nil {
		return nil, err
	}

	member, memberErr := m.findActiveMember(ctx.User.UserId, tx)
	if memberErr == nil {
		decision.Member = member
		memberID, err := ResourceRowInt64(member["id"])
		if err != nil {
			return nil, fmt.Errorf("invalid api_member id: %w", err)
		}
		if memberID <= 0 {
			return nil, errors.New("invalid api_member id: must be positive")
		}
		planID, err := ResourceRowInt64(member["api_plan_id"])
		if err != nil {
			return nil, fmt.Errorf("invalid api_member api_plan_id: %w", err)
		}
		if planID <= 0 {
			return nil, errors.New("invalid api_member api_plan_id: must be positive")
		}
		decision.MemberID = memberID
		decision.PlanID = planID
		plan, err := m.findPlan(planID, tx)
		if err != nil {
			return nil, err
		}
		decision.Plan = plan
	} else if !errors.Is(memberErr, errMeteringRowNotFound) {
		return nil, memberErr
	}

	estimated, err := admissionMeasures(ctx, config)
	if err != nil {
		return nil, err
	}
	decision.ReservedMeasures = estimated
	limits, err := meteringLimits(decision.Plan)
	if err != nil {
		return nil, err
	}
	requestID := strings.TrimSpace(ctx.RequestID)
	if requestID == "" && ctx.Request != nil {
		requestID = strings.TrimSpace(ctx.Request.Header.Get("X-Request-ID"))
	}
	if requestID == "" {
		requestID = uuid.Must(uuid.NewV7()).String()
	}
	if len(requestID) > 128 {
		return nil, errors.New("metering request_id exceeds 128 characters")
	}
	decision.RequestID = requestID
	decision.ReservationToken = uuid.Must(uuid.NewV7()).String()
	decision.reservation = make(map[string]meteringReservation, len(limits))

	existing, err := m.findUsageByRequestID(requestID, tx)
	if err == nil {
		existingUserID, conversionErr := ResourceRowInt64(existing["user_account_id"])
		if conversionErr != nil {
			return nil, fmt.Errorf("invalid api_usage user_account_id: %w", conversionErr)
		}
		if existingUserID <= 0 {
			return nil, errors.New("invalid api_usage user_account_id: must be positive")
		}
		if existingUserID != ctx.User.UserId {
			return nil, errors.New("metering request_id belongs to another user")
		}
		return m.existingDecision(existing, config, tx)
	}
	if !errors.Is(err, errMeteringRowNotFound) {
		return nil, err
	}
	for _, limit := range limits {
		reservation, reserveErr := m.reserve(decision, limit, estimated[limit.Metric], ctx.User, tx)
		if reserveErr != nil {
			return nil, reserveErr
		}
		decision.reservation[reservation.BucketKey] = reservation
	}
	reservationJSON, err := json.MarshalToString(decision.reservation)
	if err != nil {
		return nil, err
	}
	usage, err := m.insertAdmission(ctx, decision, reservationJSON, tx)
	if err != nil {
		return nil, err
	}
	decision.UsageID, err = ResourceRowInt64(usage["id"])
	if err != nil {
		return nil, fmt.Errorf("invalid api_usage id: %w", err)
	}
	if decision.UsageID <= 0 {
		return nil, errors.New("invalid api_usage id: must be positive")
	}
	return decision, nil
}

func (m *MeteringService) Complete(ctx MeteringContext, decision *MeteringDecision, tx *sqlx.Tx) error {
	return m.terminalize(ctx, decision, meteringStateCompleted, nil, tx)
}

func (m *MeteringService) Cancel(ctx MeteringContext, decision *MeteringDecision, tx *sqlx.Tx) error {
	return m.terminalize(ctx, decision, meteringStateCancelled, nil, tx)
}

func (m *MeteringService) ExpireReservations(now time.Time, limit uint, tx *sqlx.Tx) (int, error) {
	if tx == nil {
		return 0, errors.New("metering expiry recovery requires a transaction")
	}
	if limit == 0 {
		return 0, errors.New("metering expiry recovery limit must be positive")
	}
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_usage").
		Where(goqu.Ex{"state": meteringStateHeld, "reservation_expires_at": goqu.Op{"lte": now}}).
		Order(goqu.I("reservation_expires_at").Asc()).Limit(limit).ToSQL()
	if err != nil {
		return 0, err
	}
	rows, err := tx.Queryx(query, arguments...)
	if err != nil {
		return 0, err
	}
	usages, err := RowsToMap(rows, "api_usage")
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, err
	}
	released := 0
	for _, usage := range usages {
		decision, err := m.decisionFromUsage(usage, nil, tx)
		if err != nil {
			return released, err
		}
		if err := m.terminalize(
			MeteringContext{ErrorMessage: "reservation expired"},
			decision, meteringStateExpired, map[string]int64{}, tx,
		); err != nil {
			if errors.Is(err, errMeteringRequestTerminal) {
				continue
			}
			return released, err
		}
		released++
	}
	return released, nil
}

func (m *MeteringService) recoverExpiredReservations(now time.Time, limit uint) (int, error) {
	transaction, err := (*m.cruds)["api_usage"].Connection().Beginx()
	if err != nil {
		return 0, err
	}
	defer transaction.Rollback()
	expired, err := m.ExpireReservations(now, limit, transaction)
	if err != nil {
		return 0, err
	}
	if err := transaction.Commit(); err != nil {
		return 0, err
	}
	return expired, nil
}

func (m *MeteringService) terminalize(ctx MeteringContext, decision *MeteringDecision, terminalState string, measures map[string]int64, tx *sqlx.Tx) error {
	if decision == nil || !decision.Enabled {
		return nil
	}
	if tx == nil {
		return errors.New("metering terminalization requires a transaction")
	}
	usage, err := m.findUsageByToken(decision.ReservationToken, tx)
	if err != nil {
		return err
	}
	usageID, err := ResourceRowInt64(usage["id"])
	if err != nil {
		return fmt.Errorf("invalid api_usage id: %w", err)
	}
	if usageID <= 0 {
		return errors.New("invalid api_usage id: must be positive")
	}
	usageUserID, err := ResourceRowInt64(usage["user_account_id"])
	if err != nil {
		return fmt.Errorf("invalid api_usage user_account_id: %w", err)
	}
	if usageUserID <= 0 {
		return errors.New("invalid api_usage user_account_id: must be positive")
	}
	if ctx.User != nil && usageUserID != ctx.User.UserId {
		return errors.New("metering reservation belongs to another user")
	}
	usageRequestID := StringOrEmpty(usage["request_id"])
	if decision.RequestID != "" && decision.RequestID != usageRequestID {
		return errors.New("metering reservation request_id does not match its usage record")
	}
	state := StringOrEmpty(usage["state"])
	if state == terminalState {
		return nil
	}
	if state == meteringStateCompleted || state == meteringStateCancelled || state == meteringStateExpired {
		return fmt.Errorf("%w: current=%s requested=%s", errMeteringRequestTerminal, state, terminalState)
	}
	if len(decision.reservation) == 0 {
		if err := json.UnmarshalFromString(StringOrEmpty(usage["reservation_buckets"]), &decision.reservation); err != nil {
			return fmt.Errorf("decode metering reservation buckets: %w", err)
		}
	}
	config := decision.config
	if config == nil {
		config = normalizeMeteringConfig(ctx.Metering)
	}
	if measures == nil {
		measures, err = completionMeasures(ctx, config)
		if err != nil {
			return err
		}
	}
	bucketKeys := make([]string, 0, len(decision.reservation))
	for bucketKey := range decision.reservation {
		bucketKeys = append(bucketKeys, bucketKey)
	}
	sort.Strings(bucketKeys)
	for _, bucketKey := range bucketKeys {
		reservation := decision.reservation[bucketKey]
		actual := measures[reservation.Metric]
		quota, err := m.findQuota(reservation.BucketKey, tx)
		if err != nil {
			return err
		}
		reserved, err := ResourceRowInt64(quota["reserved"])
		if err != nil {
			return fmt.Errorf("invalid api_quota reserved: %w", err)
		}
		consumed, err := ResourceRowInt64(quota["consumed"])
		if err != nil {
			return fmt.Errorf("invalid api_quota consumed: %w", err)
		}
		if reserved < reservation.Amount {
			return fmt.Errorf("metering quota terminalization failed for metric %s", reservation.Metric)
		}
		updatedConsumed, overflow := addInt64(consumed, actual)
		if overflow {
			return fmt.Errorf("metering quota consumption overflow for metric %s", reservation.Metric)
		}
		quotaModel := api2go.NewApi2GoModelWithData("api_quota", (*m.cruds)["api_quota"].TableInfo().Columns,
			int64((*m.cruds)["api_quota"].TableInfo().DefaultPermission), (*m.cruds)["api_quota"].TableInfo().Relations, quota)
		quotaModel.SetAttributes(map[string]interface{}{"reserved": reserved - reservation.Amount, "consumed": updatedConsumed})
		request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch,
			URL: &url.URL{Path: "/api_quota/" + daptinid.InterfaceToDIR(quota["reference_id"]).String()}}).
			WithContext(context.WithValue(WithMeteringInternal(context.Background()), "user", ctx.User))}
		if _, err := (*m.cruds)["api_quota"].UpdateWithoutFilters(quotaModel, request, tx); err != nil {
			return fmt.Errorf("terminalize metering quota: %w", err)
		}
	}

	measuresJSON, err := json.MarshalToString(measures)
	if err != nil {
		return err
	}
	metadata := ToJson(ctx.Metadata)
	if metadata == "" || metadata == "null" {
		metadata = "{}"
	}
	usageModel := api2go.NewApi2GoModelWithData("api_usage", (*m.cruds)["api_usage"].TableInfo().Columns,
		int64((*m.cruds)["api_usage"].TableInfo().DefaultPermission), (*m.cruds)["api_usage"].TableInfo().Relations, usage)
	usageModel.SetAttributes(map[string]interface{}{
		"state": terminalState, "status_code": ctx.StatusCode, "latency_ms": ctx.LatencyMS,
		"request_bytes": ctx.RequestBytes, "response_bytes": ctx.ResponseBytes, "measures": measuresJSON,
		"metadata": metadata, "error_message": nullableString(ctx.ErrorMessage), "terminal_at": m.now(),
	})
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch,
		URL: &url.URL{Path: "/api_usage/" + daptinid.InterfaceToDIR(usage["reference_id"]).String()}}).
		WithContext(context.WithValue(WithMeteringInternal(context.Background()), "user", ctx.User))}
	if _, err := (*m.cruds)["api_usage"].UpdateWithoutFilters(usageModel, request, tx); err != nil {
		return fmt.Errorf("terminalize metering usage: %w", err)
	}
	if config != nil && config.PostMeteringAction != "" {
		m.invokePostMeteringAction(config.PostMeteringAction, ctx, decision, usageID, measures, tx)
	}
	return nil
}

func (m *MeteringService) insertAdmission(ctx MeteringContext, decision *MeteringDecision, reservationJSON string, tx *sqlx.Tx) (map[string]interface{}, error) {
	now := m.now()
	lease := ctx.ReservationTTL
	if lease <= 0 {
		lease = defaultReservationTTL
	}
	reservedJSON, err := json.MarshalToString(decision.ReservedMeasures)
	if err != nil {
		return nil, err
	}
	metadata := ToJson(ctx.Metadata)
	if metadata == "" || metadata == "null" {
		metadata = "{}"
	}
	record := map[string]interface{}{
		"request_id": decision.RequestID, "reservation_token": decision.ReservationToken, "api_key_id": nullableString(ctx.APIKeyID),
		"endpoint": ctx.Endpoint, "method": ctx.Method, "entity_type": nullableString(ctx.EntityType),
		"action_name": nullableString(ctx.ActionName), "request_type": nullableString(ctx.RequestType),
		"status_code": 0, "latency_ms": 0, "request_bytes": ctx.RequestBytes, "response_bytes": 0,
		"state": meteringStateHeld, "reservation_expires_at": now.Add(lease),
		"reserved_measures": reservedJSON, "reservation_buckets": reservationJSON, "measures": "{}", "metadata": metadata,
	}
	if decision.Plan != nil {
		record["api_plan_id"] = daptinid.InterfaceToDIR(decision.Plan["reference_id"]).String()
	}
	if decision.Member != nil {
		record["api_member_id"] = daptinid.InterfaceToDIR(decision.Member["reference_id"]).String()
	}
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/api_usage"}}).
		WithContext(context.WithValue(WithMeteringInternal(context.Background()), "user", m.internalUser(ctx.User)))}
	_, err = (*m.cruds)["api_usage"].CreateWithoutFilter(
		api2go.NewApi2GoModelWithData("api_usage", nil, 0, nil, record), request, tx)
	if err != nil {
		return nil, fmt.Errorf("create metering admission: %w", err)
	}
	return m.findUsageByRequestID(decision.RequestID, tx)
}

func (m *MeteringService) reserve(decision *MeteringDecision, limit meteringLimit, amount int64, user *auth.SessionUser, tx *sqlx.Tx) (meteringReservation, error) {
	windowStart, windowEnd, err := meteringWindow(limit.Window, decision.Member, m.now())
	if err != nil {
		return meteringReservation{}, err
	}
	bucketKey := fmt.Sprintf("%d:%d:%s:%s:%d", decision.PlanID, decision.MemberID, limit.Metric, limit.Window, windowStart.Unix())
	memberUserID, err := ResourceRowInt64(decision.Member["user_account_id"])
	if err != nil {
		return meteringReservation{}, fmt.Errorf("invalid api_member user_account_id: %w", err)
	}
	if memberUserID <= 0 {
		return meteringReservation{}, errors.New("invalid api_member user_account_id: must be positive")
	}
	quota, err := m.findQuota(bucketKey, tx)
	if errors.Is(err, errMeteringRowNotFound) {
		quota = map[string]interface{}{"bucket_key": bucketKey, "metric": limit.Metric, "window_start": windowStart,
			"window_end": windowEnd, "maximum": limit.Maximum, "reserved": int64(0), "consumed": int64(0),
			"api_plan_id":   daptinid.InterfaceToDIR(decision.Plan["reference_id"]).String(),
			"api_member_id": daptinid.InterfaceToDIR(decision.Member["reference_id"]).String()}
	} else if err != nil {
		return meteringReservation{}, err
	}
	maximum, err := ResourceRowInt64(quota["maximum"])
	if err != nil {
		return meteringReservation{}, fmt.Errorf("invalid api_quota maximum: %w", err)
	}
	reserved, err := ResourceRowInt64(quota["reserved"])
	if err != nil {
		return meteringReservation{}, fmt.Errorf("invalid api_quota reserved: %w", err)
	}
	consumed, err := ResourceRowInt64(quota["consumed"])
	if err != nil {
		return meteringReservation{}, fmt.Errorf("invalid api_quota consumed: %w", err)
	}
	updatedReserved, overflow := addInt64(reserved, amount)
	if overflow {
		return meteringReservation{}, errors.New("metering quota reservation overflow")
	}
	if limit.Mode == "hard" && maximum >= 0 && (consumed > maximum || updatedReserved > maximum-consumed) {
		message := fmt.Sprintf("%s limit exceeded", limit.Metric)
		return meteringReservation{}, api2go.NewHTTPError(errors.New(message), "insufficient_quota", http.StatusPaymentRequired)
	}
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/api_quota"}}).
		WithContext(context.WithValue(WithMeteringInternal(context.Background()), "user", m.internalUser(user)))}
	if quota["reference_id"] == nil {
		quota["reserved"] = updatedReserved
		if _, err := (*m.cruds)["api_quota"].CreateWithoutFilter(
			api2go.NewApi2GoModelWithData("api_quota", nil, 0, nil, quota), request, tx); err != nil {
			return meteringReservation{}, fmt.Errorf("create metering quota: %w", err)
		}
	} else {
		quotaModel := api2go.NewApi2GoModelWithData("api_quota", (*m.cruds)["api_quota"].TableInfo().Columns,
			int64((*m.cruds)["api_quota"].TableInfo().DefaultPermission), (*m.cruds)["api_quota"].TableInfo().Relations, quota)
		quotaModel.SetAttributes(map[string]interface{}{"reserved": updatedReserved})
		request.PlainRequest.Method = http.MethodPatch
		request.PlainRequest.URL.Path += "/" + daptinid.InterfaceToDIR(quota["reference_id"]).String()
		if _, err := (*m.cruds)["api_quota"].UpdateWithoutFilters(quotaModel, request, tx); err != nil {
			return meteringReservation{}, fmt.Errorf("update metering quota: %w", err)
		}
	}
	return meteringReservation{BucketKey: bucketKey, Metric: limit.Metric, Amount: amount}, nil
}

func (m *MeteringService) existingDecision(usage map[string]interface{}, config *table_info.MeteringConfig, tx *sqlx.Tx) (*MeteringDecision, error) {
	state := StringOrEmpty(usage["state"])
	if state == meteringStateCompleted || state == meteringStateCancelled || state == meteringStateExpired {
		return nil, fmt.Errorf("%w: %s", errMeteringRequestTerminal, state)
	}
	if state != meteringStateHeld {
		return nil, fmt.Errorf("metering request is in non-reusable state %s", state)
	}
	return m.decisionFromUsage(usage, config, tx)
}

func (m *MeteringService) decisionFromUsage(usage map[string]interface{}, config *table_info.MeteringConfig, tx *sqlx.Tx) (*MeteringDecision, error) {
	usageID, err := ResourceRowInt64(usage["id"])
	if err != nil {
		return nil, fmt.Errorf("invalid api_usage id: %w", err)
	}
	if usageID <= 0 {
		return nil, errors.New("invalid api_usage id: must be positive")
	}
	var planID int64
	if usage["api_plan_id"] != nil {
		planID, err = ResourceRowInt64(usage["api_plan_id"])
		if err != nil {
			return nil, fmt.Errorf("invalid api_usage api_plan_id: %w", err)
		}
		if planID <= 0 {
			return nil, errors.New("invalid api_usage api_plan_id: must be positive")
		}
	}
	var memberID int64
	if usage["api_member_id"] != nil {
		memberID, err = ResourceRowInt64(usage["api_member_id"])
		if err != nil {
			return nil, fmt.Errorf("invalid api_usage api_member_id: %w", err)
		}
		if memberID <= 0 {
			return nil, errors.New("invalid api_usage api_member_id: must be positive")
		}
	}
	decision := &MeteringDecision{
		Enabled: true, UsageID: usageID, RequestID: StringOrEmpty(usage["request_id"]),
		ReservationToken: StringOrEmpty(usage["reservation_token"]), PlanID: planID,
		MemberID: memberID, config: config,
	}
	if err := json.UnmarshalFromString(StringOrEmpty(usage["reserved_measures"]), &decision.ReservedMeasures); err != nil {
		return nil, fmt.Errorf("decode reserved metering measures: %w", err)
	}
	if err := json.UnmarshalFromString(StringOrEmpty(usage["reservation_buckets"]), &decision.reservation); err != nil {
		return nil, fmt.Errorf("decode metering reservation buckets: %w", err)
	}
	if decision.PlanID != 0 {
		plan, err := m.findPlan(decision.PlanID, tx)
		if err != nil {
			return nil, err
		}
		decision.Plan = plan
	}
	if decision.MemberID != 0 {
		member, err := m.findMember(decision.MemberID, tx)
		if err != nil {
			return nil, err
		}
		decision.Member = member
	}
	return decision, nil
}

func normalizeMeteringConfig(config *table_info.MeteringConfig) *table_info.MeteringConfig {
	if config == nil {
		return nil
	}
	normalized := *config
	if normalized.CostExpr == "" {
		normalized.CostExpr = "1"
	}
	if normalized.MeterType == "" {
		normalized.MeterType = "requests"
	}
	return &normalized
}

// MeteringConfigForAction resolves an action override while inheriting the
// resource defaults. Non-CRUD adapters use the same metering configuration
// owner as Daptin actions.
func MeteringConfigForAction(config *table_info.MeteringConfig, actionName string) *table_info.MeteringConfig {
	if config == nil {
		return nil
	}
	if actionConfig, ok := config.OnActions[actionName]; ok {
		if actionConfig.CostExpr == "" {
			actionConfig.CostExpr = config.CostExpr
		}
		if actionConfig.MeterType == "" {
			actionConfig.MeterType = config.MeterType
		}
		if actionConfig.PostMeteringAction == "" {
			actionConfig.PostMeteringAction = config.PostMeteringAction
		}
		return &actionConfig
	}
	return config
}

func meteringLimits(plan map[string]interface{}) ([]meteringLimit, error) {
	if plan == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(StringOrEmpty(plan["limits"]))
	if raw == "" {
		return nil, nil
	}
	limits := make([]meteringLimit, 0)
	if err := json.UnmarshalFromString(raw, &limits); err != nil {
		return nil, fmt.Errorf("decode api_plan limits: %w", err)
	}
	seen := make(map[string]bool, len(limits))
	for index := range limits {
		limit := &limits[index]
		limit.Metric = strings.TrimSpace(limit.Metric)
		limit.Window = strings.TrimSpace(limit.Window)
		limit.Mode = strings.ToLower(strings.TrimSpace(limit.Mode))
		if limit.Mode == "" {
			limit.Mode = "hard"
		}
		identity := limit.Metric + "\x00" + limit.Window
		if !validMeteringMetric(limit.Metric) || seen[identity] {
			return nil, fmt.Errorf("api_plan contains invalid or duplicate metric/window %q/%q", limit.Metric, limit.Window)
		}
		seen[identity] = true
		if limit.Maximum < 0 {
			return nil, fmt.Errorf("api_plan metric %q has a negative maximum", limit.Metric)
		}
		if limit.Mode != "hard" && limit.Mode != "soft" {
			return nil, fmt.Errorf("api_plan metric %q has unsupported mode %q", limit.Metric, limit.Mode)
		}
		if _, _, err := meteringWindow(limit.Window, nil, time.Now()); err != nil && limit.Window != "member_period" {
			return nil, fmt.Errorf("api_plan metric %q: %w", limit.Metric, err)
		}
	}
	sort.Slice(limits, func(i, j int) bool {
		if limits[i].Metric == limits[j].Metric {
			return limits[i].Window < limits[j].Window
		}
		return limits[i].Metric < limits[j].Metric
	})
	return limits, nil
}

func meteringWindow(window string, member map[string]interface{}, now time.Time) (time.Time, time.Time, error) {
	now = now.UTC()
	switch window {
	case "minute":
		start := now.Truncate(time.Minute)
		return start, start.Add(time.Minute), nil
	case "hour":
		start := now.Truncate(time.Hour)
		return start, start.Add(time.Hour), nil
	case "day":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 1), nil
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0), nil
	case "member_period":
		start := toTime(member["period_start"])
		end := toTime(member["period_end"])
		if start.IsZero() || end.IsZero() || !end.After(start) {
			return time.Time{}, time.Time{}, errors.New("member_period requires valid member period_start and period_end")
		}
		return start.UTC(), end.UTC(), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported metering window %q", window)
	}
}

func admissionMeasures(ctx MeteringContext, config *table_info.MeteringConfig) (map[string]int64, error) {
	measures, err := normalizedMeasures(ctx.EstimatedMeasures)
	if err != nil {
		return nil, err
	}
	if _, exists := measures["requests"]; !exists {
		measures["requests"] = 1
	}
	if _, exists := measures[config.MeterType]; !exists {
		measures[config.MeterType] = 1
	}
	return measures, nil
}

func completionMeasures(ctx MeteringContext, config *table_info.MeteringConfig) (map[string]int64, error) {
	measures, err := normalizedMeasures(ctx.Measures)
	if err != nil {
		return nil, err
	}
	if _, exists := measures["requests"]; !exists {
		measures["requests"] = 1
	}
	if _, exists := measures["bytes"]; !exists {
		bytes, overflow := addInt64(int64(ctx.RequestBytes), int64(ctx.ResponseBytes))
		if overflow {
			return nil, errors.New("metering byte measure overflow")
		}
		measures["bytes"] = bytes
	}
	if config != nil && config.MeterType != "" {
		if _, supplied := measures[config.MeterType]; supplied {
			return measures, nil
		}
		cost, evalErr := EvaluateMeteringCost(config.CostExpr, map[string]interface{}{
			"request": requestEnv(ctx), "response": ctx.Response, "metadata": ctx.Metadata, "user": userEnv(ctx.User),
		})
		if evalErr != nil {
			return nil, evalErr
		}
		if cost < 0 {
			return nil, errors.New("metering expression returned a negative measure")
		}
		measures[config.MeterType] = cost
	}
	return measures, nil
}

func normalizedMeasures(input map[string]int64) (map[string]int64, error) {
	measures := make(map[string]int64, len(input)+2)
	for metric, amount := range input {
		metric = strings.TrimSpace(metric)
		if !validMeteringMetric(metric) {
			return nil, fmt.Errorf("invalid metering metric %q", metric)
		}
		if amount < 0 {
			return nil, fmt.Errorf("metering metric %q cannot be negative", metric)
		}
		measures[metric] = amount
	}
	return measures, nil
}

func validMeteringMetric(metric string) bool {
	if metric == "" || len(metric) > 100 {
		return false
	}
	for _, character := range metric {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') ||
			character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func addInt64(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, true
	}
	return left + right, false
}

func (m *MeteringService) lockMeteringUser(userID int64, tx *sqlx.Tx) error {
	query, arguments, err := statementbuilder.Squirrel.Select("id").Prepared(true).From(USER_ACCOUNT_TABLE_NAME).
		Where(goqu.Ex{"id": userID}).Limit(1).ForUpdate(goqu.Wait).ToSQL()
	if err != nil {
		return err
	}
	if _, err := querySingleMeteringRow(USER_ACCOUNT_TABLE_NAME, tx, query, arguments...); err != nil {
		return fmt.Errorf("lock metering user: %w", err)
	}
	return nil
}

func (m *MeteringService) findActiveMember(userID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_member").
		Where(goqu.Ex{"user_account_id": userID, "status": "active"}).Order(goqu.I("id").Desc()).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_member", tx, query, arguments...)
}

func (m *MeteringService) findMember(memberID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_member").
		Where(goqu.Ex{"id": memberID}).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_member", tx, query, arguments...)
}

func (m *MeteringService) findPlan(planID int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_plan").
		Where(goqu.Ex{"id": planID}).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_plan", tx, query, arguments...)
}

func (m *MeteringService) findUsageByRequestID(requestID string, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_usage").
		Where(goqu.Ex{"request_id": requestID}).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_usage", tx, query, arguments...)
}

func (m *MeteringService) findUsageByToken(token string, tx *sqlx.Tx) (map[string]interface{}, error) {
	if token == "" {
		return nil, errors.New("metering reservation token is required")
	}
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_usage").
		Where(goqu.Ex{"reservation_token": token}).Limit(1).ForUpdate(goqu.Wait).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_usage", tx, query, arguments...)
}

func (m *MeteringService) findQuota(bucketKey string, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, arguments, err := statementbuilder.Squirrel.Select("*").Prepared(true).From("api_quota").
		Where(goqu.Ex{"bucket_key": bucketKey}).Limit(1).ForUpdate(goqu.Wait).ToSQL()
	if err != nil {
		return nil, err
	}
	return querySingleMeteringRow("api_quota", tx, query, arguments...)
}

func querySingleMeteringRow(resourceName string, tx *sqlx.Tx, query string, arguments ...interface{}) (map[string]interface{}, error) {
	rows, err := tx.Queryx(query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result, err := RowsToMap(rows, resourceName)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errMeteringRowNotFound
	}
	return result[0], nil
}

func (m *MeteringService) invokePostMeteringAction(actionName string, ctx MeteringContext, decision *MeteringDecision, usageID int64, measures map[string]int64, tx *sqlx.Tx) {
	parts := strings.SplitN(actionName, ":", 2)
	if len(parts) != 2 || m.cruds == nil {
		log.Warnf("[metering] invalid post_metering_action: %s", actionName)
		return
	}
	crud, ok := (*m.cruds)[parts[0]]
	if !ok || ctx.Request == nil {
		log.Warnf("[metering] post_metering_action entity not found: %s", parts[0])
		return
	}
	req := ctx.Request.WithContext(WithMeteringInternal(ctx.Request.Context()))
	actionReq := actionresponse.ActionRequest{
		Type: parts[0], Action: parts[1],
		Attributes: map[string]interface{}{
			"user_account_id": ctx.User.UserReferenceId.String(), "api_usage_id": usageID,
			"api_plan_id": decision.PlanID, "api_member_id": decision.MemberID, "measures": measures,
			"endpoint": ctx.Endpoint, "entity_type": ctx.EntityType, "action_name": ctx.ActionName,
			"metadata": ctx.Metadata, "metering_internal": true,
		},
	}
	if _, err := crud.HandleActionRequest(actionReq, api2go.Request{PlainRequest: req}, tx); err != nil {
		log.Errorf("[metering] post_metering_action failed: %v", err)
	}
}

func requestEnv(ctx MeteringContext) map[string]interface{} {
	return map[string]interface{}{
		"endpoint": ctx.Endpoint, "method": ctx.Method, "entity_type": ctx.EntityType,
		"action_name": ctx.ActionName, "request_type": ctx.RequestType, "status_code": ctx.StatusCode,
		"latency_ms": ctx.LatencyMS, "request_bytes": ctx.RequestBytes, "response_bytes": ctx.ResponseBytes,
	}
}

func userEnv(user *auth.SessionUser) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{"id": user.UserId, "reference_id": user.UserReferenceId.String()}
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func toTime(value interface{}) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		timestamp, _ := time.Parse(time.RFC3339, typed)
		return timestamp
	case []byte:
		timestamp, _ := time.Parse(time.RFC3339, string(typed))
		return timestamp
	default:
		return time.Time{}
	}
}
