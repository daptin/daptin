package resource

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	exchangeExecutionPending         = "pending"
	exchangeExecutionRunning         = "running"
	exchangeExecutionRetryableFailed = "retryable_failed"
	exchangeExecutionSucceeded       = "succeeded"
	exchangeExecutionTerminalFailed  = "terminal_failed"

	exchangeExecutionMaxAttempts = int64(5)
	exchangeExecutionLease       = 45 * time.Second
	exchangeExecutionRetention   = 30 * 24 * time.Hour
	exchangeExecutionCleanupSize = uint(100)
)

// ExchangeExecutionService owns the durable exchange lifecycle. Mutation
// middleware only enqueues through the normal resource path; this service is
// the sole place that claims, executes, and terminalizes queued work.
type ExchangeExecutionService struct {
	cruds *map[string]*DbResource
	now   func() time.Time
}

func NewExchangeExecutionService(_ *CmsConfig, cruds *map[string]*DbResource) *ExchangeExecutionService {
	return &ExchangeExecutionService{cruds: cruds, now: time.Now}
}

func (service *ExchangeExecutionService) Enqueue(exchange ExchangeContract, method string,
	row map[string]interface{}, request api2go.Request, transaction *sqlx.Tx) error {
	queue, err := service.queueResource()
	if err != nil {
		return err
	}
	if transaction == nil {
		return fmt.Errorf("enqueue data exchange execution without a transaction")
	}

	exchangeReference := daptinid.InterfaceToDIR(exchange.ReferenceId)
	if exchangeReference == daptinid.NullReferenceId {
		return fmt.Errorf("data exchange [%s] has no reference_id", exchange.Name)
	}
	sourceType := StringOrEmpty(row["__type"])
	sourceReference := exchangeSourceReference(row["reference_id"])
	if sourceType == "" || sourceReference == "" {
		return fmt.Errorf("data exchange [%s] source identity is incomplete", exchange.Name)
	}
	sourceReferenceID := daptinid.InterfaceToDIR(sourceReference)
	if sourceReferenceID == daptinid.NullReferenceId {
		return fmt.Errorf("data exchange [%s] source reference_id is invalid", exchange.Name)
	}
	sourceVersion := int64(0)
	if row["version"] != nil {
		sourceVersion, err = exchangeSourceVersion(row["version"])
		if err != nil {
			return fmt.Errorf("invalid source version for data exchange [%s]: %w", exchange.Name, err)
		}
	}

	executionReference := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
		exchangeReference.String(), sourceType, sourceReference, strings.ToLower(method), fmt.Sprint(sourceVersion),
	}, "\x00")))
	if existing, _, findErr := queue.GetSingleRowByReferenceIdWithTransaction(
		"data_exchange_execution", daptinid.DaptinReferenceId(executionReference), nil, transaction); findErr == nil && existing != nil {
		return nil
	}

	executionUser, err := exchangeSessionUser(*service.cruds, exchange.AsUserId, transaction)
	if err != nil {
		return fmt.Errorf("resolve data exchange execution user: %w", err)
	}
	sourceResource := (*service.cruds)[sourceType]
	if sourceResource == nil {
		return fmt.Errorf("data exchange [%s] source resource [%s] is unavailable", exchange.Name, sourceType)
	}
	sourcePermission, hasCapturedPermission := row["__permission"].(permission.PermissionInstance)
	if !hasCapturedPermission || strings.ToLower(method) != "delete" {
		sourcePermission = sourceResource.GetRowPermissionWithTransaction(row, transaction)
	}
	if !sourcePermission.CanRead(executionUser.UserReferenceId, executionUser.Groups,
		sourceResource.AdministratorGroupId) {
		return fmt.Errorf("data exchange user cannot read source [%s][%s]", sourceType, sourceReference)
	}
	adminID, _ := GetAdminUserIdAndUserGroupId(transaction)
	if adminID <= 0 {
		return fmt.Errorf("administrators group has no execution owner")
	}
	adminUser, err := exchangeSessionUser(*service.cruds, adminID, transaction)
	if err != nil {
		return fmt.Errorf("resolve data exchange execution owner: %w", err)
	}

	attributes := map[string]interface{}{
		"reference_id":        executionReference.String(),
		"data_exchange_id":    exchangeReference.String(),
		"as_user_id":          executionUser.UserReferenceId.String(),
		"source_type":         sourceType,
		"source_reference_id": sourceReferenceID[:],
		"source_method":       strings.ToLower(method),
		"source_version":      sourceVersion,
		"state":               exchangeExecutionPending,
		"attempt_count":       int64(0),
		"max_attempts":        exchangeExecutionMaxAttempts,
		"next_attempt_at":     service.now().UTC(),
	}
	request.PlainRequest = request.PlainRequest.WithContext(
		context.WithValue(request.PlainRequest.Context(), "user", adminUser))
	model := api2go.NewApi2GoModelWithData("data_exchange_execution", nil, 0, nil, attributes)
	if _, err = queue.CreateWithoutFilter(model, request, transaction); err != nil {
		return fmt.Errorf("create data exchange execution: %w", err)
	}
	return nil
}

func exchangeSourceVersion(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case float64:
		if typed != math.Trunc(typed) || typed < math.MinInt64 || typed > math.MaxInt64 {
			return 0, fmt.Errorf("invalid floating-point version [%v]", typed)
		}
		return int64(typed), nil
	case float32:
		asFloat64 := float64(typed)
		if asFloat64 != math.Trunc(asFloat64) {
			return 0, fmt.Errorf("invalid floating-point version [%v]", typed)
		}
		return int64(typed), nil
	default:
		return ResourceRowInt64(value)
	}
}

func exchangeSourceReference(value interface{}) string {
	if value == nil {
		return ""
	}
	if reference := daptinid.InterfaceToDIR(value); reference != daptinid.NullReferenceId {
		return reference.String()
	}
	if reference := StringOrEmpty(value); reference != "" {
		return reference
	}
	if reference := fmt.Sprint(value); reference != "<nil>" {
		return reference
	}
	return ""
}

// ProcessPending rotates the action transaction after claiming a row. This is
// the established outbox boundary: no SQL transaction remains open while an
// external provider is running.
func (service *ExchangeExecutionService) ProcessPending(transaction *sqlx.Tx) error {
	queue, err := service.queueResource()
	if err != nil {
		return err
	}
	if transaction == nil {
		return fmt.Errorf("process data exchange executions without a transaction")
	}
	if err := service.terminalizeExhaustedLeases(transaction, service.now().UTC()); err != nil {
		return err
	}
	if err := service.cleanupCompleted(transaction, service.now().UTC()); err != nil {
		return err
	}

	claim, err := service.claimNext(transaction, service.now().UTC())
	if err != nil || claim == nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit data exchange execution claim: %w", err)
	}

	attemptTransaction, err := queue.Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin data exchange execution attempt: %w", err)
	}
	*transaction = *attemptTransaction

	_, exchange, source, terminalCode, err := service.loadAttempt(claim.id, transaction)
	if err != nil || terminalCode != "" {
		_ = transaction.Rollback()
		failureTransaction, beginErr := queue.Connection().Beginx()
		if beginErr != nil {
			return beginErr
		}
		*transaction = *failureTransaction
		if terminalCode == "" {
			terminalCode = "execution_context_unavailable"
		}
		return service.markFailure(claim, terminalCode, err, true, transaction)
	}

	if exchange.TargetType != "action" {
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit data exchange execution context: %w", err)
		}
		_, executeErr := NewExchangeExecution(exchange, service.cruds).Execute([]map[string]interface{}{source}, nil)
		terminalTransaction, beginErr := queue.Connection().Beginx()
		if beginErr != nil {
			return beginErr
		}
		*transaction = *terminalTransaction
		if executeErr != nil {
			return service.markFailure(claim, "target_unavailable", executeErr, false, transaction)
		}
		return service.markSucceeded(claim, transaction)
	}

	if _, err := NewExchangeExecution(exchange, service.cruds).Execute([]map[string]interface{}{source}, transaction); err != nil {
		_ = transaction.Rollback()
		failureTransaction, beginErr := queue.Connection().Beginx()
		if beginErr != nil {
			return beginErr
		}
		*transaction = *failureTransaction
		return service.markFailure(claim, "target_unavailable", err, false, transaction)
	}
	return service.markSucceeded(claim, transaction)
}

func (service *ExchangeExecutionService) terminalizeExhaustedLeases(transaction *sqlx.Tx, now time.Time) error {
	query, args, err := statementbuilder.Squirrel.Select("id").Prepared(true).
		From("data_exchange_execution").Where(
		goqu.Ex{"state": exchangeExecutionRunning},
		goqu.Ex{"lease_expires_at": goqu.Op{"lte": now}},
		goqu.I("attempt_count").Gte(goqu.I("max_attempts")),
	).Order(goqu.C("lease_expires_at").Asc()).Limit(exchangeExecutionCleanupSize).ToSQL()
	if err != nil {
		return err
	}
	rows, err := transaction.Queryx(query, args...)
	if err != nil {
		return fmt.Errorf("list exhausted data exchange leases: %w", err)
	}
	ids := make([]int64, 0, exchangeExecutionCleanupSize)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	updateSQL, updateArgs, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":              exchangeExecutionTerminalFailed,
			"completed_at":       now,
			"lease_token":        nil,
			"lease_expires_at":   nil,
			"last_error_code":    "lease_expired",
			"last_error_summary": "attempt lease expired after the retry limit",
			"updated_at":         now,
		}).Where(
		goqu.Ex{"id": goqu.Op{"in": ids}},
		goqu.Ex{"state": exchangeExecutionRunning},
		goqu.Ex{"lease_expires_at": goqu.Op{"lte": now}},
	).ToSQL()
	if err != nil {
		return err
	}
	if _, err := transaction.Exec(updateSQL, updateArgs...); err != nil {
		return fmt.Errorf("terminalize exhausted data exchange leases: %w", err)
	}
	return nil
}

func (service *ExchangeExecutionService) cleanupCompleted(transaction *sqlx.Tx, now time.Time) error {
	query, args, err := statementbuilder.Squirrel.Select("id").Prepared(true).
		From("data_exchange_execution").Where(
		goqu.Ex{"state": goqu.Op{"in": []string{exchangeExecutionSucceeded, exchangeExecutionTerminalFailed}}},
		goqu.Ex{"completed_at": goqu.Op{"lt": now.Add(-exchangeExecutionRetention)}},
	).Order(goqu.C("completed_at").Asc()).Limit(exchangeExecutionCleanupSize).ToSQL()
	if err != nil {
		return err
	}
	rows, err := transaction.Queryx(query, args...)
	if err != nil {
		return fmt.Errorf("list expired data exchange executions: %w", err)
	}
	ids := make([]int64, 0, exchangeExecutionCleanupSize)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	deleteSQL, deleteArgs, err := statementbuilder.Squirrel.Delete("data_exchange_execution").Prepared(true).
		Where(
			goqu.Ex{"id": goqu.Op{"in": ids}},
			goqu.Ex{"state": goqu.Op{"in": []string{exchangeExecutionSucceeded, exchangeExecutionTerminalFailed}}},
			goqu.Ex{"completed_at": goqu.Op{"lt": now.Add(-exchangeExecutionRetention)}},
		).ToSQL()
	if err != nil {
		return err
	}
	if _, err := transaction.Exec(deleteSQL, deleteArgs...); err != nil {
		return fmt.Errorf("delete expired data exchange executions: %w", err)
	}
	return nil
}

type exchangeExecutionClaim struct {
	id          int64
	attempts    int64
	maxAttempts int64
	leaseToken  string
}

func (service *ExchangeExecutionService) claimNext(transaction *sqlx.Tx, now time.Time) (*exchangeExecutionClaim, error) {
	query, args, err := statementbuilder.Squirrel.Select("id", "attempt_count", "max_attempts").
		Prepared(true).From("data_exchange_execution").Where(exchangeExecutionEligible(now)).
		Order(goqu.C("created_at").Asc()).Limit(1).ToSQL()
	if err != nil {
		return nil, err
	}
	var claim exchangeExecutionClaim
	if err := transaction.QueryRowx(query, args...).Scan(&claim.id, &claim.attempts, &claim.maxAttempts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("list pending data exchange executions: %w", err)
	}
	claim.leaseToken = uuid.NewString()
	claim.attempts++
	claimSQL, claimArgs, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":            exchangeExecutionRunning,
			"attempt_count":    claim.attempts,
			"lease_token":      claim.leaseToken,
			"lease_expires_at": now.Add(exchangeExecutionLease),
			"updated_at":       now,
		}).Where(goqu.Ex{"id": claim.id}, exchangeExecutionEligible(now)).ToSQL()
	if err != nil {
		return nil, err
	}
	result, err := transaction.Exec(claimSQL, claimArgs...)
	if err != nil {
		return nil, fmt.Errorf("claim data exchange execution [%d]: %w", claim.id, err)
	}
	claimed, err := result.RowsAffected()
	if err != nil || claimed == 0 {
		return nil, err
	}
	return &claim, nil
}

func (service *ExchangeExecutionService) loadAttempt(id int64, transaction *sqlx.Tx) (
	map[string]interface{}, ExchangeContract, map[string]interface{}, string, error) {
	row := make(map[string]interface{})
	rows, err := transaction.Queryx(transaction.Rebind(`select id, data_exchange_id, as_user_id,
		source_type, source_reference_id, source_method, source_version from data_exchange_execution where id = ?`), id)
	if err != nil {
		return nil, ExchangeContract{}, nil, "execution_not_found", err
	}
	if !rows.Next() {
		_ = rows.Close()
		return nil, ExchangeContract{}, nil, "execution_not_found", fmt.Errorf("data exchange execution [%d] is unavailable", id)
	}
	if err := rows.MapScan(row); err != nil {
		_ = rows.Close()
		return nil, ExchangeContract{}, nil, "execution_not_found", err
	}
	if err := rows.Close(); err != nil {
		return nil, ExchangeContract{}, nil, "execution_not_found", err
	}

	exchangeID, err := ResourceRowInt64(row["data_exchange_id"])
	if err != nil {
		return row, ExchangeContract{}, nil, "exchange_not_found", err
	}
	exchange, err := service.loadExchange(exchangeID, transaction)
	if err != nil {
		return row, ExchangeContract{}, nil, "exchange_not_found", err
	}
	asUserID, err := ResourceRowInt64(row["as_user_id"])
	if err != nil {
		return row, exchange, nil, "execution_user_not_found", err
	}
	executionUser, err := exchangeSessionUser(*service.cruds, asUserID, transaction)
	if err != nil {
		return row, exchange, nil, "execution_user_not_found", err
	}
	exchange.AsUserId = asUserID

	sourceType := StringOrEmpty(row["source_type"])
	sourceReference := daptinid.InterfaceToDIR(exchangeSourceReference(row["source_reference_id"]))
	if sourceType == "" || sourceReference == daptinid.NullReferenceId || (*service.cruds)[sourceType] == nil {
		return row, exchange, nil, "source_not_found", fmt.Errorf("data exchange source identity is unavailable")
	}
	if StringOrEmpty(row["source_method"]) == "delete" {
		return row, exchange, map[string]interface{}{
			"__type": sourceType, "reference_id": sourceReference.String(),
		}, "", nil
	}
	source, _, err := (*service.cruds)[sourceType].GetSingleRowByReferenceIdWithTransaction(
		sourceType, sourceReference, nil, transaction)
	if err != nil {
		return row, exchange, nil, "source_not_found", err
	}
	sourcePermission := (*service.cruds)[sourceType].GetObjectPermissionByReferenceId(sourceType, sourceReference, transaction)
	if !sourcePermission.CanRead(executionUser.UserReferenceId, executionUser.Groups,
		(*service.cruds)[sourceType].AdministratorGroupId) {
		return row, exchange, nil, "source_read_denied", fmt.Errorf("data exchange user cannot read source")
	}
	source["__type"] = sourceType
	recordedVersion, err := exchangeSourceVersion(row["source_version"])
	if err != nil {
		return row, exchange, nil, "source_version_invalid", err
	}
	currentVersion, err := exchangeSourceVersion(source["version"])
	if err != nil {
		return row, exchange, nil, "source_version_invalid", err
	}
	if recordedVersion > 0 && currentVersion != recordedVersion {
		return row, exchange, nil, "source_version_changed", fmt.Errorf("source changed after exchange enqueue")
	}
	return row, exchange, source, "", nil
}

func (service *ExchangeExecutionService) loadExchange(id int64, transaction *sqlx.Tx) (ExchangeContract, error) {
	var exchange ExchangeContract
	var referenceID []byte
	var sourceAttributes, targetAttributes, attributes, options []byte
	var asUserID *int64
	err := transaction.QueryRowx(transaction.Rebind(`select reference_id, name, source_attributes, source_type,
		target_attributes, target_type, attributes, options, as_user_id from data_exchange where id = ?`), id).
		Scan(&referenceID, &exchange.Name, &sourceAttributes, &exchange.SourceType,
			&targetAttributes, &exchange.TargetType, &attributes, &options, &asUserID)
	if err != nil {
		return exchange, err
	}
	if asUserID == nil {
		return exchange, fmt.Errorf("data exchange [%s] has no as_user_id", exchange.Name)
	}
	exchange.ReferenceId = daptinid.InterfaceToDIR(referenceID).String()
	exchange.AsUserId = *asUserID
	if err := json.Unmarshal(sourceAttributes, &exchange.SourceAttributes); err != nil {
		return exchange, err
	}
	if err := json.Unmarshal(targetAttributes, &exchange.TargetAttributes); err != nil {
		return exchange, err
	}
	if err := json.Unmarshal(attributes, &exchange.Attributes); err != nil {
		return exchange, err
	}
	if len(options) > 0 {
		if err := json.Unmarshal(options, &exchange.Options); err != nil {
			return exchange, err
		}
	}
	return exchange, nil
}

func (service *ExchangeExecutionService) markSucceeded(claim *exchangeExecutionClaim, transaction *sqlx.Tx) error {
	now := service.now().UTC()
	return service.updateClaim(claim, goqu.Record{
		"state": exchangeExecutionSucceeded, "completed_at": now,
		"lease_token": nil, "lease_expires_at": nil,
		"last_error_code": nil, "last_error_summary": nil, "updated_at": now,
	}, transaction)
}

func (service *ExchangeExecutionService) markFailure(claim *exchangeExecutionClaim, code string,
	executionErr error, terminal bool, transaction *sqlx.Tx) error {
	now := service.now().UTC()
	state := exchangeExecutionRetryableFailed
	completedAt := interface{}(nil)
	nextAttempt := interface{}(now.Add(exchangeRetryDelay(claim.attempts)))
	if terminal || claim.attempts >= claim.maxAttempts {
		state = exchangeExecutionTerminalFailed
		completedAt = now
		nextAttempt = nil
	}
	summary := code
	if executionErr != nil {
		summary = fmt.Sprintf("%s (%T)", code, executionErr)
	}
	return service.updateClaim(claim, goqu.Record{
		"state": state, "next_attempt_at": nextAttempt, "completed_at": completedAt,
		"lease_token": nil, "lease_expires_at": nil,
		"last_error_code": code, "last_error_summary": summary, "updated_at": now,
	}, transaction)
}

func (service *ExchangeExecutionService) updateClaim(claim *exchangeExecutionClaim, values goqu.Record, transaction *sqlx.Tx) error {
	query, args, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(values).Where(goqu.Ex{
		"id": claim.id, "state": exchangeExecutionRunning, "lease_token": claim.leaseToken,
	}).ToSQL()
	if err != nil {
		return err
	}
	result, err := transaction.Exec(query, args...)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("data exchange execution [%d] lease was lost", claim.id)
	}
	return nil
}

func exchangeExecutionEligible(now time.Time) exp.Expression {
	return goqu.Or(
		goqu.And(
			goqu.Ex{"state": goqu.Op{"in": []string{exchangeExecutionPending, exchangeExecutionRetryableFailed}}},
			goqu.Or(goqu.Ex{"next_attempt_at": nil}, goqu.Ex{"next_attempt_at": goqu.Op{"lte": now}}),
			goqu.I("attempt_count").Lt(goqu.I("max_attempts")),
		),
		goqu.And(
			goqu.Ex{"state": exchangeExecutionRunning},
			goqu.Ex{"lease_expires_at": goqu.Op{"lte": now}},
			goqu.I("attempt_count").Lt(goqu.I("max_attempts")),
		),
	)
}

func exchangeRetryDelay(attempts int64) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 12 {
		attempts = 12
	}
	delay := time.Duration(1<<attempts) * time.Second
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func (service *ExchangeExecutionService) queueResource() (*DbResource, error) {
	if service == nil || service.cruds == nil || (*service.cruds)["data_exchange_execution"] == nil {
		return nil, fmt.Errorf("data_exchange_execution resource is unavailable")
	}
	return (*service.cruds)["data_exchange_execution"], nil
}

// Retry returns a terminal execution to the same scheduled claim path. It does
// not execute work inline and it preserves the total attempt count.
func (service *ExchangeExecutionService) Retry(referenceID daptinid.DaptinReferenceId, transaction *sqlx.Tx) error {
	if _, err := service.queueResource(); err != nil {
		return err
	}
	if referenceID == daptinid.NullReferenceId || transaction == nil {
		return fmt.Errorf("retry data exchange execution without a valid identity or transaction")
	}
	now := service.now().UTC()
	query, args, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":              exchangeExecutionPending,
			"max_attempts":       goqu.L("attempt_count + ?", exchangeExecutionMaxAttempts),
			"next_attempt_at":    now,
			"lease_token":        nil,
			"lease_expires_at":   nil,
			"last_error_code":    nil,
			"last_error_summary": nil,
			"completed_at":       nil,
			"updated_at":         now,
		}).Where(goqu.Ex{
		"reference_id": referenceID[:],
		"state":        exchangeExecutionTerminalFailed,
	}).ToSQL()
	if err != nil {
		return err
	}
	result, err := transaction.Exec(query, args...)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("data exchange execution is not terminal or does not exist")
	}
	return nil
}
